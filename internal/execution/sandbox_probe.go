package execution

import (
	"context"
	"forgeflow/internal/domain"
	"time"
)

type gitShellProber interface {
	ProbeGitShell(context.Context, string) (domain.TestRun, error)
}

// Use seconds-long capability probes instead of repeatedly rebuilding a large
// repository while its required Git subprocess cannot even start.
func (s *Service) recoverGitShell(ctx context.Context, repo string, r *domain.ExecutionRecord, d *domain.ExecutionIncident) error {
	prober, ok := s.Runner.(gitShellProber)
	if !ok {
		return nil
	}
	for attempt := 0; ; attempt++ {
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		run, err := prober.ProbeGitShell(probeCtx, repo)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			run.Error = err.Error()
			if run.ExitCode == 0 {
				run.ExitCode = -1
			}
		}
		// A capability probe is audit evidence, never a required repository test.
		if e := s.Store.WorkspaceEvent(ctx, r.ContributionID, "SandboxCapabilityChecked", "Checked Git shell startup in the contribution sandbox", run); e != nil {
			return e
		}
		if err == nil && run.ExitCode == 0 {
			return s.Store.WorkspaceEvent(ctx, r.ContributionID, "SandboxCapabilityRestored", "Git shell starts; required repository verification must now run", nil)
		}
		d.Evidence = "The required Git subprocess failed in an isolated capability probe.\n" + promptExcerpt(run.Output+"\n"+run.Error, 2048)
		d.NextAction = "Git's shell cannot start in this Windows sandbox. Configure a compatible Codex Windows sandbox (MXC on supported devices, or a provisioned elevated sandbox) and Git installation, then retry. No credential change or source fix is justified by this failure."
		if e := s.incident(ctx, r, d); e != nil {
			return e
		}
		if attempt >= maxTransientRetries || r.RecoveryAttempts >= maxRecoveryActions {
			d.Status, d.Retryable = "needs_input", false
			return setupFailure(d)
		}
		if e := s.waitForRecovery(ctx, r, d, attempt, "Rechecking Git shell startup before rerunning expensive repository tests"); e != nil {
			return e
		}
	}
}
