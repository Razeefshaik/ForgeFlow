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
	deadline := 10 * time.Minute
	if command.Program == "go" {
		for i, arg := range command.Arguments {
			value := ""
			if strings.HasPrefix(arg, "-timeout=") {
				value = strings.TrimPrefix(arg, "-timeout=")
			} else if arg == "-timeout" && i+1 < len(command.Arguments) {
				value = command.Arguments[i+1]
			}
			if d, err := time.ParseDuration(value); err == nil && d > 0 && d+2*time.Minute > deadline {
				deadline = d + 2*time.Minute
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
		return s.incident(ctx, r, d)
	}
	r.RecoveryAttempts++
	d.Status, d.Attempt = "recovering", r.RecoveryAttempts
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
	retried, installed := false, false
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
		budget := r.RecoveryAttempts < maxRecoveryActions
		if d.CanRecover && budget && !retried && (d.Kind == "dependency_network" || d.Kind == "network_transport") {
			retried, recoveryOf = true, run.ID
			if e := s.recoveryAction(ctx, r, d, "Issue reactor is retrying dependency access before changing code"); e != nil {
				return false, e
			}
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false, ctx.Err()
			case <-timer.C:
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
		if retried {
			d.NextAction = "The automatic retry still failed. Inspect the dependency service or sandbox transport, then request fresh verification."
		}
		if !budget {
			d.NextAction = "Automatic recovery reached its session limit. Inspect the latest evidence before requesting a fresh recovery."
		}
		d.Status = "blocked"
		if e := s.Store.SaveExecution(ctx, *r, "reactor", "Automatic recovery stopped with evidence preserved"); e != nil {
			return false, e
		}
		return false, setupFailure(d)
	}
}
