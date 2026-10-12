package execution

import (
	"context"
	"errors"
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func verificationDeadline(command domain.VerificationCommand) time.Duration {
	if command.TimeoutSeconds > 0 && command.TimeoutSeconds <= 7200 {
		return time.Duration(command.TimeoutSeconds) * time.Second
	}
	deadline := 10 * time.Minute
	if command.Program == "go" {
		if len(command.Arguments) > 0 && command.Arguments[0] == "test" {
			deadline = 12 * time.Minute
		}
		for i, arg := range command.Arguments {
			value := ""
			if strings.HasPrefix(arg, "-timeout=") {
				value = strings.TrimPrefix(arg, "-timeout=")
			} else if arg == "-timeout" && i+1 < len(command.Arguments) {
				value = command.Arguments[i+1]
			}
			if d, err := time.ParseDuration(value); err == nil && value != "" {
				if d <= 0 {
					deadline = 30 * time.Minute
				} else if d+2*time.Minute > deadline {
					deadline = d + 2*time.Minute
				}
			}
		}
	}
	if deadline > 30*time.Minute {
		deadline = 30 * time.Minute
	}
	return deadline
}

func (s *Service) recordedCommand(ctx context.Context, id, repo string, command domain.VerificationCommand, network bool, recoveryOf string) (domain.TestRun, error) {
	started := time.Now().UTC()
	if err := s.Store.WorkspaceEvent(ctx, id, "TestRunStarted", "Executing sandboxed verification", command); err != nil {
		return domain.TestRun{}, err
	}
	testCtx, cancel := context.WithTimeout(ctx, verificationDeadline(command))
	run, commandErr := s.Runner.Command(testCtx, repo, command, network)
	if commandErr == nil && testCtx.Err() != nil {
		commandErr = testCtx.Err()
	}
	cancel()
	run.ID, run.ContributionID, run.RecoveryOf = storage.ID(), id, recoveryOf
	run.Command, run.Directory = command, repo
	if run.StartedAt.IsZero() {
		run.StartedAt = started
	}
	if run.FinishedAt.IsZero() {
		run.FinishedAt = time.Now().UTC()
		if commandErr != nil {
			run.ExitCode = -1 // An unlaunched command is never reported as passed.
		}
	}
	if commandErr != nil && run.ExitCode == 0 {
		run.ExitCode = -1
	}
	if commandErr != nil {
		run.Error = commandErr.Error()
	}
	if commandErr != nil && strings.TrimSpace(run.Output) == "" {
		run.Output = commandErr.Error()
	}
	if commandErr != nil || run.ExitCode != 0 {
		run.FailureKind = diagnoseCommand(command, run.Output, commandErr, network).Kind
		if ctx.Err() != nil {
			run.FailureKind = "cancelled"
		}
	}
	auditCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	err := s.Store.SaveTest(auditCtx, run)
	stop()
	if err != nil {
		return domain.TestRun{}, fmt.Errorf("cannot persist verification evidence: %w", err)
	}
	return run, commandErr
}

func (s *Service) recoveryAction(ctx context.Context, r *domain.ExecutionRecord, d *domain.ExecutionIncident, action string) error {
	if r.RecoveryAttempts >= maxRecoveryActions {
		d.Status, d.CanRecover = "blocked", false
		d.NextAction = "Automatic recovery reached its session limit. Inspect the latest evidence before requesting a fresh recovery."
		if err := s.incident(ctx, r, d); err != nil {
			return err
		}
		return setupFailure(d)
	}
	r.RecoveryAttempts++
	d.Status, d.Attempt = "recovering", r.RecoveryAttempts
	if d.RetryAt != nil {
		d.Status = "waiting_retry"
	}
	r.Incident = d
	r.Message = action
	if err := s.Store.WorkspaceMessage(ctx, r.ContributionID, action); err != nil {
		return err
	}
	if err := s.Store.WorkspaceEvent(ctx, r.ContributionID, "RecoveryActionStarted", action, d); err != nil {
		return err
	}
	return s.Store.SaveExecution(ctx, *r, "reactor", action)
}

// waitForRecovery persists the schedule before waiting. Pause/stop cancels the
// timer, and neither a long rate-limit window nor an exhausted budget spins.
func (s *Service) waitForRecovery(ctx context.Context, r *domain.ExecutionRecord, d *domain.ExecutionIncident, attempt int, action string) error {
	if attempt >= maxTransientRetries || r.RecoveryAttempts >= maxRecoveryActions {
		d.Retryable = false
		d.NextAction = "Automatic retries reached their limit. Restore the required service or environment, then request fresh recovery."
		return setupFailure(d)
	}
	delay := time.Second * time.Duration(1<<attempt)
	if d.RetryAt != nil {
		delay = time.Until(*d.RetryAt)
	}
	if delay > time.Minute {
		d.Retryable = false
		d.NextAction = "The service retry time is more than one minute away. Resume after that time; no agent is kept running while waiting."
		return setupFailure(d)
	}
	if delay < 0 {
		delay = 0
	}
	at := time.Now().UTC().Add(delay)
	d.RetryAt = &at
	if err := s.recoveryAction(ctx, r, d, action); err != nil {
		return err
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	d.Status, d.RetryAt = "recovering", nil
	if err := s.Store.WorkspaceEvent(ctx, r.ContributionID, "RecoveryRetryStarted", "Scheduled retry is starting", d); err != nil {
		return err
	}
	return s.Store.SaveExecution(ctx, *r, "reactor", "Recovery delay completed; retrying the original operation")
}

func lockedNPMDependencies(repo string) bool {
	for _, name := range []string{"package.json", "package-lock.json"} {
		info, err := os.Lstat(filepath.Join(repo, name))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	_, err := os.Lstat(filepath.Join(repo, "node_modules"))
	return errors.Is(err, os.ErrNotExist)
}

func setupFailure(d *domain.ExecutionIncident) error {
	return fmt.Errorf("verification setup failed: %s %s", d.Summary, d.NextAction)
}

// verifyWithRecovery never changes the required command. Recovery commands and
// failed attempts are recorded separately, then the original command runs again.
func (s *Service) verifyWithRecovery(ctx context.Context, id, repo string, command domain.VerificationCommand, r *domain.ExecutionRecord, all *strings.Builder) (bool, error) {
	recoveryOf := ""
	// A resumed execution retains failed evidence. Probe a known broken shell
	// before spending another full build timeout on exactly the same command.
	if _, ok := s.Runner.(gitShellProber); ok {
		previous, e := s.Store.Tests(ctx, id)
		if e != nil {
			return false, e
		}
		for i := len(previous) - 1; i >= 0; i-- {
			run := previous[i]
			if run.Command.Program != command.Program || strings.Join(run.Command.Arguments, "\x00") != strings.Join(command.Arguments, "\x00") {
				continue
			}
			if run.ExitCode != 0 && sandboxProcessFailure(strings.ToLower(run.Output)) {
				d := diagnoseCommand(command, run.Output, savedCommandError(run), r.Network)
				d.TestRunID = run.ID
				if e = s.recoverGitShell(ctx, repo, r, d); e != nil {
					return false, e
				}
				recoveryOf = run.ID
			}
			break
		}
	}
	retries, installed, runtimePrepared := 0, false, false
	investigatedTimeout := false
	environmentRepaired := false
	quarantined := map[string]bool{}
	saveOutput := func(run domain.TestRun) error {
		fmt.Fprintf(all, "$ %s %s\nrun: %s\nexit: %d\n%s\n", run.Command.Program, strings.Join(run.Command.Arguments, " "), run.ID, run.ExitCode, run.Output)
		return os.WriteFile(filepath.Join(filepath.Dir(repo), "artifacts", "test-output.txt"), []byte(all.String()), 0600)
	}
	for {
		run, err := s.recordedCommand(ctx, id, repo, command, r.Network, recoveryOf)
		if run.ID == "" {
			return false, err
		}
		if e := saveOutput(run); e != nil {
			return false, e
		}
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if err == nil && run.ExitCode == 0 {
			if recoveryOf != "" {
				if e := s.Store.WorkspaceEvent(ctx, id, "RecoveryActionCompleted", "The original verification command passed after recovery", map[string]string{"test_run_id": run.ID, "recovery_of": recoveryOf}); e != nil {
					return false, e
				}
			}
			return true, nil
		}
		d := diagnoseCommand(command, run.Output, err, r.Network)
		d.TestRunID = run.ID
		if e := s.incident(ctx, r, d); e != nil {
			return false, e
		}
		if d.Kind == "code_failure" {
			return false, nil
		}
		if !environmentRepaired && (d.Kind == "toolchain_mismatch" || d.Kind == "tool_unavailable" || d.Kind == "dependencies_missing") {
			environmentRepaired = true
			if repaired, e := s.repairVerificationEnvironment(ctx, id, repo, r, d); e != nil {
				return false, e
			} else if repaired {
				recoveryOf = run.ID
				continue
			}
		}
		if d.Kind == "test_failure_timeout" {
			if investigatedTimeout {
				d.Status, d.Retryable = "needs_input", false
				d.NextAction = "The complete required suite timed out again after focused checks passed. Inspect saved timeout stacks and service/fixture requirements; repeated source fixes are not justified."
				return false, setupFailure(d)
			}
			investigatedTimeout = true
			passed, investigationErr := s.investigateTimedOutTests(ctx, id, repo, command, run, r, saveOutput)
			if errors.Is(investigationErr, retryCompleteVerification) {
				recoveryOf = run.ID
				continue
			}
			return passed, investigationErr
		}
		budget := r.RecoveryAttempts < maxRecoveryActions
		if d.Kind == "sandbox_process" && budget && retries < maxTransientRetries {
			if _, ok := s.Runner.(gitShellProber); ok {
				if e := s.recoverGitShell(ctx, repo, r, d); e != nil {
					return false, e
				}
			}
		}
		if d.Kind == "generated_path_access" && budget && !quarantined[d.Path] {
			quarantined[d.Path] = true
			if e := s.quarantineGeneratedDirectory(ctx, id, repo, r, d); e == nil {
				recoveryOf = run.ID
				continue
			} else {
				d.CanRecover = false
				d.NextAction = "Automatic path repair was refused: " + e.Error() + ". Inspect the recorded path permissions, then retry."
			}
		}
		if d.Retryable && budget && retries < maxTransientRetries && (d.Kind != "workspace_runtime" || !runtimePrepared) {
			recoveryOf = run.ID
			if e := s.waitForRecovery(ctx, r, d, retries, "Issue reactor scheduled a retry of the original verification command"); e != nil {
				return false, e
			}
			retries++
			if d.Kind == "workspace_runtime" {
				runtimePrepared = true
				if e := s.prepareRuntime(repo); e != nil {
					failure := diagnoseCommand(command, "", e, r.Network)
					failure.TestRunID = run.ID
					if e = s.incident(ctx, r, failure); e != nil {
						return false, e
					}
					return false, setupFailure(failure)
				}
			}
			continue
		}
		if d.Kind == "dependencies_missing" && d.CanRecover && budget && !installed && lockedNPMDependencies(repo) {
			installed, recoveryOf = true, run.ID
			if e := s.recoveryAction(ctx, r, d, "Issue reactor is installing locked workspace dependencies before rerunning verification"); e != nil {
				return false, e
			}
			install := domain.VerificationCommand{Program: "npm", Arguments: []string{"ci", "--ignore-scripts", "--no-audit", "--no-fund"}}
			installRun, installErr := s.recordedCommand(ctx, id, repo, install, r.Network, run.ID)
			if installRun.ID == "" {
				return false, installErr
			}
			if e := saveOutput(installRun); e != nil {
				return false, e
			}
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			if installErr == nil && installRun.ExitCode == 0 {
				continue
			}
			d = diagnoseCommand(install, installRun.Output, installErr, r.Network)
			d.Status, d.CanRecover, d.TestRunID = "blocked", false, installRun.ID
			d.Summary = "Locked workspace dependency installation failed. " + d.Summary
			d.NextAction = "Inspect the saved installation output before requesting another recovery."
			if e := s.incident(ctx, r, d); e != nil {
				return false, e
			}
			return false, setupFailure(d)
		}
		if d.Kind == "dependencies_missing" && !lockedNPMDependencies(repo) {
			d.CanRecover = false
			d.NextAction = "Inspect dependency setup. Automatic npm installation requires an existing package lock and an absent node_modules directory."
		}
		if retries > 0 && d.Kind != "sandbox_process" {
			d.NextAction = "Automatic retries still failed. Restore the required service or environment, then request fresh verification."
		}
		if !budget {
			d.NextAction = "Automatic recovery reached its session limit. Inspect the latest evidence before requesting a fresh recovery."
		}
		d.Status, d.Retryable = "needs_input", false
		if e := s.Store.SaveExecution(ctx, *r, "reactor", "Automatic recovery stopped with evidence preserved"); e != nil {
			return false, e
		}
		return false, setupFailure(d)
	}
}
