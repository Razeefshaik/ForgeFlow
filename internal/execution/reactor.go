package execution

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"os/exec"
	"strings"
	"time"
)

const maxRecoveryActions = 3

func hasAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}

// Classify known infrastructure failures before allowing the code fixer to act.
// Unknown repository failures still go through Codex's existing bounded fix loop.
func diagnoseCommand(command domain.VerificationCommand, output string, err error, network bool) *domain.ExecutionIncident {
	text := strings.ToLower(output)
	if err != nil {
		text += "\n" + strings.ToLower(err.Error())
	}
	d := &domain.ExecutionIncident{Status: "blocked", Command: &command, DetectedAt: time.Now().UTC()}
	switch {
	case hasAny(text, "sandbox isolation", "outside active workspace", "writes outside", "git configuration changed", "changed git history", "left its approved branch", "crosses a link boundary", "must be an independent directory"):
		d.Kind, d.Summary = "isolation", "Workspace isolation or approved Git identity could not be verified."
		d.Evidence, d.NextAction = "The execution boundary rejected this operation.", "Inspect the workspace and restore its approved boundary before resuming."
	case hasAny(text, "cannot connect to the docker daemon", "docker daemon is not running", "linux daemon is unavailable", "dockerdesktoplinuxengine", "pipe/docker_engine", "pipe\\docker_engine", "wsl access is denied", "wsl_e", "wsl/service", "wsl access denied"):
		d.Kind, d.Summary = "runtime_unavailable", "Required Docker or WSL verification is unavailable."
		d.Evidence, d.NextAction = "The command reported that the required runtime could not be reached.", "Restore the required runtime or use an approved environment that can run the same checks, then recover verification."
	case hasAny(text, "401 unauthorized", "403 forbidden", "authentication required", "could not read username", "invalid credentials"):
		d.Kind, d.Summary = "authentication", "A required service rejected authentication."
		d.Evidence, d.NextAction = "The service rejected access; retries cannot supply credentials.", "Configure the required service authentication outside the agent, then recover verification."
	case hasAny(text, "access is denied", "permission denied", "read-only file system", "eacces", "cache is not writable"):
		d.Kind, d.Summary = "workspace_permissions", "Verification setup failed because a required path is not writable."
		d.Evidence, d.NextAction = "The process reported a filesystem permission error.", "Inspect the command output and workspace cache permissions, then recover verification."
	case errors.Is(err, exec.ErrNotFound) || hasAny(text, "executable file not found", "could not be resolved", "node is unavailable", "createprocessasuserw failed: 2"):
		d.Kind, d.Summary = "tool_unavailable", "A required verification tool is unavailable."
		d.Evidence, d.NextAction = "The runner could not launch the required executable.", "Install or configure the required tool, then recover verification."
	case hasAny(text, "proxyconnect tcp", "127.0.0.1:9:", "localhost:9:", "[::1]:9:"):
		d.Kind, d.Summary = "network_transport", "Dependency setup could not reach its configured proxy."
		d.Evidence = "The command failed before verification while connecting through a proxy."
		d.CanRecover = network
		d.NextAction = "Allow dependency downloads when resuming to retry with the approved network policy."
		if network {
			d.NextAction = "Retry the same check with approved network access. Inherited loopback discard proxies are removed only from the child environment."
		}
	case hasAny(text, "sockettimeoutexception", "unknownhostexception", "tls handshake timeout", "i/o timeout", "connection reset by peer", "temporary failure in name resolution", "eai_again", "econnreset", "502 bad gateway", "503 service unavailable", "network is unreachable", "could not resolve host", "enotfound", "connection refused", "actively refused") || (errors.Is(err, context.DeadlineExceeded) && hasAny(text, "downloading", "download", "fetching")):
		d.Kind, d.Summary = "dependency_network", "Dependency access failed before verification could complete."
		d.Evidence = "The command reported a network or dependency download failure."
		d.CanRecover = network
		d.NextAction = "Allow dependency downloads when resuming; the same required check will run again."
		if network {
			d.NextAction = "Retry the same required check once with backoff; stop if the dependency service remains unavailable."
		}
	case hasAny(text, "404 not found", "410 gone", "no matching version found", "etarget") && hasAny(text, "toolchain", "download", "module", "npm"):
		d.Kind, d.Summary = "dependency_unavailable", "A required dependency or toolchain version is unavailable."
		d.Evidence, d.NextAction = "The dependency service rejected the requested version; this is not a passing test or a source-code failure.", "Inspect the requested version and repository requirements. Restore the required dependency before retrying tests."
	case (command.Program == "npm" || command.Program == "node") && hasAny(text, "cannot find module", "err_module_not_found", "node_modules missing", "module_not_found", "is not recognized as an internal or external command", "command not found"):
		d.Kind, d.Summary = "dependencies_missing", "Workspace dependencies may not be installed."
		d.Evidence = "The JavaScript command could not find a required dependency."
		d.CanRecover = network
		d.NextAction = "With download approval and an existing package lock, install workspace dependencies and rerun the original check."
	case errors.Is(err, context.DeadlineExceeded):
		d.Kind, d.Summary = "verification_timeout", "The verification command exceeded its allowed duration."
		d.Evidence, d.NextAction = "The command deadline expired; silence alone is not evidence of a stalled process.", "Inspect the saved output and required environment before recovering verification."
	default:
		d.Kind, d.Summary = "code_failure", "A repository check failed."
		d.Status, d.CanRecover = "fixing", true
		d.Evidence, d.NextAction = "The actual command exited unsuccessfully.", "Codex will investigate the latest failure, make a focused fix, and rerun required checks."
	}
	return d
}

func (s *Service) incident(ctx context.Context, r *domain.ExecutionRecord, d *domain.ExecutionIncident) error {
	if d.ID == "" {
		d.ID = storage.ID()
	}
	d.Attempt = r.RecoveryAttempts
	r.Incident = d
	if err := s.Store.WorkspaceEvent(ctx, r.ContributionID, "IssueDetected", d.Summary, d); err != nil {
		return err
	}
	return s.Store.SaveExecution(ctx, *r, "reactor", d.Summary)
}

func (s *Service) resolveIncident(ctx context.Context, r *domain.ExecutionRecord) error {
	if r.Incident == nil {
		return nil
	}
	resolved := *r.Incident
	resolved.Status = "resolved"
	if err := s.Store.WorkspaceEvent(ctx, r.ContributionID, "IssueResolved", "Fresh verification resolved the current execution issue", resolved); err != nil {
		return err
	}
	r.Incident = nil
	return s.Store.SaveExecution(ctx, *r, "reactor", "Current execution issue cleared by fresh verification")
}

func verificationOnlyBlock(summary string) bool {
	text := strings.ToLower(summary)
	if hasAny(text, "ambiguous", "already solved", "issue is closed", "issue closed", "scope changed", "cannot implement", "unable to implement", "not implemented", "partially implemented", "implementation incomplete", "fix incomplete", "cannot fix", "could not fix", "need clarification", "cannot determine", "missing requirements", "requirements are unclear", "acceptance criteria are unclear", "out of scope") {
		return false
	}
	return hasAny(text, "verification", "testing", "tests", "test execution", "downloads", "dependencies", "docker", "wsl")
}

// A BLOCKED response may contain a completed patch. Only verification-related
// blocks with an actual patch can proceed to the control plane's required tests.
func (s *Service) acceptImplementation(ctx context.Context, c domain.Contribution, r *domain.ExecutionRecord, repo, output string) error {
	var result struct{ Status, Summary string }
	if err := json.Unmarshal([]byte(output), &result); err != nil || strings.TrimSpace(result.Summary) == "" {
		return errors.New("contributor returned an invalid implementation result; inspect saved agent output")
	}
	if result.Status == "IMPLEMENTED" {
		r.Summary = result.Summary
		return nil
	}
	if result.Status != "BLOCKED" || !verificationOnlyBlock(result.Summary) {
		return errors.New("contributor blocked: " + result.Summary)
	}
	if err := s.verifyApprovedGit(ctx, repo, c.BaseCommit, c.Branch); err != nil {
		return err
	}
	_, files, err := s.diff(ctx, repo, c.BaseCommit)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("contributor blocked before producing a patch: " + result.Summary)
	}
	r.Summary = result.Summary
	return s.incident(ctx, r, &domain.ExecutionIncident{Kind: "agent_verification", Summary: "The contributor left a patch but could not complete its own verification.", Evidence: "A verification-related BLOCKED response and workspace changes are saved. The agent's summary is not a test result.", NextAction: "ForgeFlow is running the required checks independently before deciding whether to fix or review the patch.", Status: "verifying", CanRecover: true, DetectedAt: time.Now().UTC()})
}

// Diagnose is read-only and also gives older blocked executions a useful cause.
func (s *Service) Diagnose(ctx context.Context, id string) (*domain.ExecutionIncident, error) {
	r, err := s.Store.Execution(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if r.Incident != nil {
		return r.Incident, nil
	}
	if r.Status != "BLOCKED" {
		return nil, nil
	}
	if hasAny(strings.ToLower(r.Summary), "scope changed", "issue is no longer", "issue is closed", "execution stopped;", "interrupted by server restart", "changed git history", "git configuration changed", "left its approved branch", "sandbox isolation") {
		return &domain.ExecutionIncident{Kind: "execution_boundary", Summary: "Execution requires inspection before resuming.", Evidence: "The current stop concerns approval, isolation, or an explicit interruption.", NextAction: "Inspect the current execution message and saved inputs, then resume explicitly.", Status: "blocked", DetectedAt: r.UpdatedAt}, nil
	}
	tests, err := s.Store.Tests(ctx, id)
	if err != nil {
		return nil, err
	}
	var agent struct{ Status, Summary string }
	text := r.Summary
	if i := strings.IndexByte(text, '{'); i >= 0 && strings.HasPrefix(text, "contributor cannot") {
		_ = json.Unmarshal([]byte(text[i:]), &agent)
	}
	if agent.Status == "BLOCKED" && !verificationOnlyBlock(agent.Summary) {
		return &domain.ExecutionIncident{Kind: "agent_blocked", Summary: "The contributor reported an issue that requires inspection.", Evidence: "The saved agent result reports a substantive blocker.", NextAction: "Review the contributor output and issue scope before resuming.", Status: "blocked", DetectedAt: r.UpdatedAt}, nil
	}
	if len(tests) > 0 {
		last := tests[len(tests)-1]
		if last.ExitCode != 0 {
			d := diagnoseCommand(last.Command, last.Output, nil, r.Network)
			d.ID, d.TestRunID, d.DetectedAt = last.ID, last.ID, last.FinishedAt
			d.Status = "blocked"
			return d, nil
		}
	}
	if agent.Status == "BLOCKED" && verificationOnlyBlock(agent.Summary) {
		return &domain.ExecutionIncident{Kind: "agent_verification", Summary: "The agent stopped because its verification could not complete.", Evidence: "The saved agent response reports verification limitations; required checks need a fresh run.", NextAction: "Recover verification to check the saved patch before spending another code-fix iteration.", Status: "blocked", CanRecover: r.Plan != nil, DetectedAt: r.UpdatedAt}, nil
	}
	d := diagnoseCommand(domain.VerificationCommand{}, r.Summary, nil, r.Network)
	d.DetectedAt, d.Status, d.Command = r.UpdatedAt, "blocked", nil
	// Unclassified control-plane failures cannot be automatically resumed as tests.
	if d.Kind == "code_failure" {
		d.Kind, d.Summary, d.CanRecover = "execution_blocked", "Execution stopped and needs its saved evidence inspected.", false
		d.NextAction = "Inspect the execution output before resuming."
	}
	return d, nil
}

func (s *Service) Recover(ctx context.Context, id string, approved, allowDownloads bool) (domain.ExecutionRecord, error) {
	if !approved {
		return domain.ExecutionRecord{}, errors.New("confirm recovery of this stopped contribution")
	}
	d, err := s.Diagnose(ctx, id)
	if err != nil {
		return domain.ExecutionRecord{}, err
	}
	networkIssue := d != nil && (d.Kind == "dependency_network" || d.Kind == "network_transport" || d.Kind == "dependencies_missing")
	if d == nil || (!d.CanRecover && !(allowDownloads && networkIssue)) {
		return domain.ExecutionRecord{}, errors.New("this issue requires its recommended action before automatic recovery")
	}
	// Retry enforces state, approval, workspace and concurrency checks. It keeps
	// saved network approval and reruns required tests before any further fixing.
	if allowDownloads {
		return s.retry(ctx, id, "tests", true, &allowDownloads)
	}
	return s.Retry(ctx, id, "tests", true)
}
