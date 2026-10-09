package execution

import (
	"context"
	"errors"
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"regexp"
	"strings"
)

// Only provenance claims are excluded. Technical subjects such as an LLM API
// regression remain valid PR content. Internal audit reports keep full evidence.
var authorshipClaim = regexp.MustCompile(`(?i)(?:\b(?:generated|written|built|created|implemented|authored|produced|developed|assisted|powered)\b[^\n]{0,60}\b(?:by|with|using)\b[^\n]{0,60}\b(?:an?\s+)?(?:llm|ai|codex|chatgpt|claude|copilot|openai|gpt[- ]?\d*)\b|\b(?:ai|llm)[ -](?:generated|assisted|authored|built)\b|\bcodex model\s*:|\bco-authored-by\s*:[^\n]*(?:bot|codex|openai|copilot))`)

func publicText(text string) string {
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		if !authorshipClaim.MatchString(line) {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func publicPR(a domain.WorkspaceApproval, r domain.ExecutionRecord, tests []domain.TestRun) (string, string) {
	title := fmt.Sprintf("Fix #%d: %s", a.Opportunity.Number, publicText(a.Opportunity.Title))
	title = strings.Join(strings.Fields(title), " ")
	summary := publicText(r.Summary)
	if summary == "" {
		summary = fmt.Sprintf("Addresses #%d.", a.Opportunity.Number)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Summary\n%s\n\n## Changes\n", summary)
	for _, file := range r.ChangedFiles {
		fmt.Fprintf(&b, "- `%s`\n", publicText(file))
	}
	b.WriteString("\n## Testing\n")
	// Report the latest outcome per command; historical failed attempts remain
	// in the internal report and never masquerade as current failures or passes.
	latest := map[string]domain.TestRun{}
	order := []string{}
	for _, run := range tests {
		key := run.Command.Program + "\x00" + strings.Join(run.Command.Arguments, "\x00")
		if _, found := latest[key]; !found {
			order = append(order, key)
		}
		latest[key] = run
	}
	for _, key := range order {
		run := latest[key]
		outcome := fmt.Sprintf("exit %d", run.ExitCode)
		if run.FinishedAt.IsZero() {
			outcome = "not completed"
		}
		fmt.Fprintf(&b, "- `%s`: %s\n", publicText(strings.TrimSpace(run.Command.Program+" "+strings.Join(run.Command.Arguments, " "))), outcome)
	}
	if len(tests) == 0 {
		b.WriteString("- No verification results recorded.\n")
	}
	if r.Plan != nil {
		risks := publicText(strings.Join(append(append([]string{}, r.Plan.Risks...), r.Plan.Unknowns...), "\n"))
		if risks != "" {
			fmt.Fprintf(&b, "\n## Risks and limitations\n%s\n", risks)
		}
	}
	fmt.Fprintf(&b, "\nFixes #%d\n", a.Opportunity.Number)
	return title, b.String()
}

func validatePRText(title, body string) error {
	if strings.TrimSpace(title) == "" || len(title) > 256 || strings.ContainsAny(title, "\r\n") {
		return errors.New("PR title must be a single line of 1–256 bytes")
	}
	if strings.TrimSpace(body) == "" || len(body) > 60000 {
		return errors.New("PR description must contain 1–60000 bytes")
	}
	if authorshipClaim.MatchString(title + "\n" + body) {
		return errors.New("PR text contains generated authorship attribution; review and remove the attribution before saving or submitting")
	}
	return nil
}

// SavePRText is optimistic: polling or another tab cannot silently overwrite
// a newer draft, and changing prepared text invalidates the old approval token.
func (s *Service) SavePRText(ctx context.Context, id, title, body, revision string) (domain.ExecutionRecord, error) {
	if err := s.beginPR(id); err != nil {
		return domain.ExecutionRecord{}, err
	}
	defer s.endPR(id)
	r, err := s.Store.Execution(ctx, id)
	if err != nil {
		return r, err
	}
	c, err := s.Store.Contribution(ctx, id)
	if err != nil {
		return r, err
	}
	if c.State != "READY" && c.State != "PR_PREPARED" {
		return r, errors.New("PR text can be edited only after review and before submission")
	}
	if revision == "" || revision != submissionToken(r) {
		return r, fmt.Errorf("%w: PR draft changed; reload the latest draft before saving", storage.ErrConflict)
	}
	title, body = strings.TrimSpace(title), strings.TrimSpace(body)
	if err = validatePRText(title, body); err != nil {
		return r, err
	}
	r.PRTitle, r.PRBody = title, body
	if c.State == "PR_PREPARED" {
		r.SubmissionToken = submissionToken(r)
	}
	err = s.Store.SaveExecution(ctx, r, "user", "Human edited PR title and description; submission approval must be renewed")
	return r, err
}

// PRRevision binds editing to the exact saved commit and text, including legacy records.
func PRRevision(r domain.ExecutionRecord) string { return submissionToken(r) }
