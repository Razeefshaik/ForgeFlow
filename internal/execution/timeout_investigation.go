package execution

import (
	"context"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var failedGoPackage = regexp.MustCompile(`(?m)^FAIL\s+([A-Za-z0-9_./+-]+)\s+[0-9.]+s\s*$`)
var goModuleName = regexp.MustCompile(`(?m)^module\s+([^\s]+)\s*$`)

func focusedGoCommands(repo string, original domain.VerificationCommand, output string) []domain.VerificationCommand {
	tests := failedGoTests(original, output)
	if len(tests) == 0 {
		return nil
	}
	info, err := os.Lstat(filepath.Join(repo, "go.mod"))
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		return nil
	}
	module := goModuleName.FindStringSubmatch(string(b))
	if len(module) != 2 {
		return nil
	}
	var commands []domain.VerificationCommand
	seen := map[string]bool{}
	for _, pkg := range failedGoPackage.FindAllStringSubmatch(output, 3) {
		rel := "."
		if pkg[1] != module[1] {
			if !strings.HasPrefix(pkg[1], module[1]+"/") {
				continue
			}
			rel = strings.TrimPrefix(pkg[1], module[1]+"/")
		}
		if !filepath.IsLocal(rel) || seen[rel] {
			continue
		}
		real, e := filepath.EvalSymlinks(filepath.Join(repo, filepath.FromSlash(rel)))
		if e != nil || !strings.EqualFold(real, filepath.Clean(filepath.Join(repo, filepath.FromSlash(rel)))) {
			continue
		}
		seen[rel] = true
		args := []string{"test", "./" + rel, "-run", "^(" + strings.Join(tests, "|") + ")$", "-count=1", "-timeout=90s"}
		for i, arg := range original.Arguments {
			if arg == "-race" || strings.HasPrefix(arg, "-tags=") {
				args = append(args, arg)
			}
			if arg == "-tags" && i+1 < len(original.Arguments) {
				args = append(args, arg, original.Arguments[i+1])
			}
		}
		command := domain.VerificationCommand{Program: "go", Arguments: args}
		if codex.ValidateCommand(command) == nil {
			commands = append(commands, command)
		}
	}
	return commands
}

// Supplemental reproductions are audit evidence, never replacement verification.
func (s *Service) investigateTimedOutTests(ctx context.Context, id, repo string, command domain.VerificationCommand, original domain.TestRun, r *domain.ExecutionRecord, save func(domain.TestRun) error) (bool, error) {
	d := r.Incident
	commands := focusedGoCommands(repo, command, original.Output)
	if len(commands) == 0 {
		// The assertion itself is sufficient evidence to investigate. Do not
		// invent a package or execute an arbitrary path from command output.
		d.Status = "fixing"
		d.NextAction = "Investigate the recorded assertions and whether they predate this patch. Reproduce them in the active workspace; the incomplete required suite must run again."
		return false, s.incident(ctx, r, d)
	}
	if r.RecoveryAttempts >= maxRecoveryActions {
		return false, setupFailure(d)
	}
	if err := s.recoveryAction(ctx, r, d, "Investigating assertions recorded before the verification timeout"); err != nil {
		return false, err
	}
	confirmed := false
	for _, focused := range commands {
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		run, err := s.recordedCommand(probeCtx, id, repo, focused, r.Network, original.ID)
		cancel()
		if run.ID == "" {
			return false, err
		}
		if e := save(run); e != nil {
			return false, e
		}
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if err == nil && run.ExitCode == 0 {
			continue
		}
		failure := diagnoseCommand(focused, run.Output, err, r.Network)
		failure.TestRunID = run.ID
		if failure.Kind == "code_failure" || failure.Kind == "test_failure_timeout" {
			confirmed = true
			continue
		}
		// A prerequisite failure in the reproduction is still a setup failure.
		if e := s.incident(ctx, r, failure); e != nil {
			return false, e
		}
		return false, setupFailure(failure)
	}
	if confirmed {
		d.Status = "fixing"
		d.NextAction = "A focused command confirmed a repository failure. Codex must check whether it predates the patch, fix only issue-related behavior, and rerun every complete required check."
		return false, s.incident(ctx, r, d)
	}
	// Passing focused tests do not explain the incomplete suite. Preserve both
	// outcomes and avoid sending a code fixer after a nonexistent reproduction.
	d.Kind, d.Status = "verification_timeout", "needs_input"
	d.NextAction = "Focused checks passed, but the required suite remains incomplete. Inspect the saved Go timeout stacks and fixture/service requirements, then rerun the full suite."
	if err := s.incident(ctx, r, d); err != nil {
		return false, err
	}
	return false, errors.New("verification remains incomplete after focused timeout investigation")
}
