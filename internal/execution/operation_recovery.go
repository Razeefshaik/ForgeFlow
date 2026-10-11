package execution

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"net"
	"path/filepath"
	"strings"
	"time"
)

func inputRequired(kind string) bool {
	switch kind {
	case "environment_blocked", "toolchain_mismatch", "usage_limit":
		return true
	case "authentication", "runtime_unavailable", "workspace_permissions", "tool_unavailable", "disk_capacity", "dependency_unavailable", "verification_timeout", "test_failure_timeout", "dependency_network", "network_transport", "rate_limit", "dependencies_missing", "resource_busy", "workspace_runtime", "plan_invalid", "agent_output", "input_too_large", "sandbox_process", "generated_path_access", "no_changes":
		return true
	default:
		return false
	}
}

// recoverOperation handles freshness and agent transport failures.
// It never retries an unknown result or an isolation/scope failure.
func (s *Service) recoverOperation(ctx context.Context, r *domain.ExecutionRecord, name string, operation func() error) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := operation()
		if err == nil {
			if attempt > 0 {
				if err := s.Store.WorkspaceEvent(ctx, r.ContributionID, "RecoveryActionCompleted", "The original operation completed after recovery: "+name, nil); err != nil {
					return err
				}
				return s.resolveIncident(ctx, r)
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Network approval controls repository processes, not authenticated
		// control-plane GitHub/Codex API connections used for this operation.
		d := diagnoseCommand(domain.VerificationCommand{}, "", err, true)
		d.Command = nil
		d.Source = "execution"
		if strings.Contains(name, "GitHub") {
			d.Source = "github"
			var transport net.Error
			if errors.As(err, &transport) && transport.Timeout() {
				d.Kind, d.CanRecover, d.Retryable = "dependency_network", true, true
				d.Evidence, d.NextAction = "The authenticated GitHub read timed out.", "Retry the fresh issue read with bounded backoff before any repository execution."
			}
		} else if strings.HasPrefix(name, "Codex ") {
			d.Source = "codex"
		}
		if d.Kind == "dependency_network" {
			d.Summary = name + " encountered a transient service connection failure."
		}
		if d.Kind == "code_failure" {
			return err
		}
		if e := s.incident(ctx, r, d); e != nil {
			return e
		}
		if !d.Retryable {
			return err
		}
		if e := s.waitForRecovery(ctx, r, d, attempt, "Issue reactor scheduled a retry: "+name); e != nil {
			return e
		}
	}
}

func (s *Service) recoveringAgent(ctx context.Context, c domain.Contribution, r *domain.ExecutionRecord, repo, meta, role, prompt, schema string) (string, error) {
	environment, err := environmentPrompt(repo, r)
	if err != nil {
		return "", err
	}
	prompt += environment
	var output string
	err = s.recoverOperation(ctx, r, "Codex "+role, func() error {
		if role == "contributor" {
			if err := s.verifyApprovedGit(ctx, repo, c.BaseCommit, c.Branch); err != nil {
				return err
			}
		}
		var err error
		output, err = s.agent(ctx, c.ID, repo, meta, role, prompt+"\nIf an earlier attempt left files, inspect and preserve its partial work before continuing. Do not reset Git or redo unrelated changes.", schema, c.CodexModel, r.Network)
		return err
	})
	return output, err
}

func validatePlan(output string) (domain.Plan, error) {
	return validatePlanWith(output, codex.ValidateCommand)
}

func validatePlanWith(output string, validate func(domain.VerificationCommand) error) (domain.Plan, error) {
	var plan domain.Plan
	if err := json.Unmarshal([]byte(output), &plan); err != nil || strings.TrimSpace(plan.Summary) == "" {
		return plan, errors.New("planner returned an invalid plan; return valid schema JSON with a nonempty summary")
	}
	if len(plan.Tests) > 12 {
		return plan, errors.New("planner returned an invalid plan; propose at most 12 verification commands")
	}
	for _, command := range plan.Tests {
		if err := validate(command); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

func (s *Service) planning(ctx context.Context, c domain.Contribution, r *domain.ExecutionRecord, repo, meta, instructions string) (domain.Plan, error) {
	prompt := instructions + "Read the repository without editing. Return a concrete plan: root cause, affected files, strategy, real verification commands, risks and unknowns. Each verification program must be a single executable name from go, python, python3, pytest, npm, node, mvn, ./mvnw, gradle, ./gradlew. Put every argument in the arguments array, never inside program. Do not propose shell commands, formatting commands, pipelines or environment assignments as verification. Native Go test -run, -bench and -skip regex alternation is allowed; shell pipelines are not."
	if p, ok := s.Runner.(codex.EnvironmentPreparer); ok && p.EnvironmentBase() != "" {
		prompt = instructions + "Read the repository without editing. Return a concrete plan: root cause, affected files, strategy, real verification commands, risks and unknowns. Container verification supports any installed language's executable (for example cargo, dotnet, bundle, cmake, uv or a repository script). Supply one program and a separate arguments array; all execution stays in the selected container. Use the supplied environment evidence. Do not omit required tests or claim prerequisite checks replace the test suite."
	}
	// A previously rejected legacy command may now validate. Reuse that saved
	// result, but still pass through fresh preflight and human plan approval.
	previous, err := s.Store.LatestAgent(ctx, c.ID, "planner")
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return domain.Plan{}, err
	}
	if err == nil && previous.Status == "SUCCEEDED" && previous.Error == "" && r.Constraints == "" {
		plan, err := validatePlanWith(previous.Output, s.validateCommand)
		if err == nil {
			_, err = verificationCommands(repo, &plan, s.validateCommand)
		}
		if err == nil {
			if err = s.Store.WorkspaceEvent(ctx, c.ID, "PlanRecovered", "Previously saved planner output now validates; plan approval remains required when configured", map[string]string{"agent_run_id": previous.ID}); err != nil {
				return domain.Plan{}, err
			}
			return plan, nil
		}
	}
	for attempt := 0; ; attempt++ {
		output, err := s.recoveringAgent(ctx, c, r, repo, meta, "planner", prompt, planSchema)
		if err != nil {
			return domain.Plan{}, err
		}
		plan, err := validatePlanWith(output, s.validateCommand)
		if err == nil {
			_, err = verificationCommands(repo, &plan, s.validateCommand)
		}
		if err == nil {
			return plan, nil
		}
		// Rejected plans are evidence, never approved plans or test results.
		name := "rejected-plan-" + storage.ID() + ".json"
		if e := writeJSON(filepath.Join(meta, name), map[string]string{"output": output, "validation_error": err.Error()}); e != nil {
			return domain.Plan{}, e
		}
		d := &domain.ExecutionIncident{Kind: "plan_invalid", Source: "planner", Summary: "The planner proposed a plan that did not pass validation.", Evidence: err.Error(), NextAction: "The read-only planner is correcting the rejected plan; no code or tests run during plan correction.", Status: "recovering", CanRecover: true, DetectedAt: time.Now().UTC()}
		if e := s.incident(ctx, r, d); e != nil {
			return domain.Plan{}, e
		}
		if e := s.Store.WorkspaceEvent(ctx, c.ID, "PlanRejected", err.Error(), map[string]string{"artifact": name}); e != nil {
			return domain.Plan{}, e
		}
		if attempt >= maxTransientRetries || r.RecoveryAttempts >= maxRecoveryActions {
			d.NextAction = "The planner could not produce a valid plan within the retry limit. Inspect the rejected plan and provide contribution constraints before resuming."
			return domain.Plan{}, err
		}
		if e := s.recoveryAction(ctx, r, d, "Returning precise command validation feedback to the read-only planner"); e != nil {
			return domain.Plan{}, e
		}
		prompt = instructions + "Correct the rejected plan and return complete schema JSON. Do not edit code, omit necessary verification, or weaken tests. Native argv is required; split shell commands into separate executable/arguments entries. Validation feedback:\n" + err.Error() + "\nRejected output:\n" + output
	}
}
