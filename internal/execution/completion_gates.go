package execution

import (
	"context"
	"fmt"
	"forgeflow/internal/domain"
	"time"
)

// Completion gates never trigger source-fix retries or credential requests to
// an agent. Every recovery reruns the public verifier against the current tree.
func (s *Service) verifyCompletionGates(ctx context.Context, id, repo string, r *domain.ExecutionRecord) error {
	if r.EnvironmentPlan == nil {
		return nil
	}
	for _, gate := range r.EnvironmentPlan.CompletionGates {
		// A HEAD-only verifier can accept an old receipt while uncommitted edits
		// remain. Export the reviewed patch for the repository's trusted commit
		// workflow; never present that receipt as proof of this working tree.
		if gate.Binding == "commit" {
			d := &domain.ExecutionIncident{Kind: "completion_gate", Source: "verification", Summary: "The reviewed patch needs the repository's trusted commit and signing workflow.", Evidence: gate.Requirement + " (" + gate.Source + ") certifies a commit. The contribution is an uncommitted reviewed patch; a receipt for its base commit cannot certify these edits.", NextAction: gate.Action + " Use artifacts/final.patch and the saved handoff report in a trusted checkout. ForgeFlow does not currently import externally signed commits; do not repeatedly recover or place private keys here.", Status: "needs_input", DetectedAt: time.Now().UTC(), DiagnosisVersion: diagnosisVersion}
			if err := s.incident(ctx, r, d); err != nil {
				return err
			}
			return fmt.Errorf("trusted commit handoff required: %s", gate.Requirement)
		}
		for _, command := range gate.Checks {
			if err := s.validateCommand(command); err != nil {
				return err
			}
			checkCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			rec, err := s.Runner.Command(checkCtx, repo, command, false)
			cancel()
			auditCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
			e := s.Store.WorkspaceEvent(auditCtx, id, "CompletionGateChecked", gate.Requirement, map[string]any{"gate": gate, "result": rec, "error": fmt.Sprint(err)})
			stop()
			if e != nil {
				return e
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil || rec.ExitCode != 0 {
				d := &domain.ExecutionIncident{Kind: "completion_gate", Source: "verification", Summary: "Code verification and review finished; an external completion requirement remains.", Evidence: gate.Requirement + " (" + gate.Source + ")\n" + rec.Output + "\n" + fmt.Sprint(err), NextAction: gate.Action + " Keep private keys outside the contribution workspace. Recover after supplying the required public evidence; ForgeFlow will verify it before READY.", Status: "needs_input", CanRecover: true, DetectedAt: time.Now().UTC(), DiagnosisVersion: diagnosisVersion}
				if e := s.incident(ctx, r, d); e != nil {
					return e
				}
				return fmt.Errorf("external completion requirement: %s", gate.Requirement)
			}
		}
	}
	return nil
}
