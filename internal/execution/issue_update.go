package execution

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"time"
)

// An upstream issue edited after approval used to block every resume with
// "issue scope changed" and offered no way forward. The edit is now saved for
// review; continuing requires an explicit human approval of the new text and
// a fresh plan. The original approval record is never rewritten.

var errIssueChanged = errors.New("issue scope changed since approval; review the updated issue and approve it to continue")

// effectiveIssue is the issue snapshot agents work from: the original
// approval, with the title and body of a human-approved later revision.
func effectiveIssue(a domain.WorkspaceApproval, r *domain.ExecutionRecord) json.RawMessage {
	if r == nil || r.ApprovedIssue == nil {
		return a.Issue
	}
	issue := map[string]any{}
	if err := json.Unmarshal(a.Issue, &issue); err != nil {
		issue = map[string]any{}
	}
	issue["title"], issue["body"] = r.ApprovedIssue.Title, r.ApprovedIssue.Body
	b, err := json.Marshal(issue)
	if err != nil {
		return a.Issue
	}
	return b
}

func approvedIssueText(a domain.WorkspaceApproval, r *domain.ExecutionRecord) (string, string, error) {
	var issue struct{ Title, Body string }
	if err := json.Unmarshal(effectiveIssue(a, r), &issue); err != nil {
		return "", "", err
	}
	return issue.Title, issue.Body, nil
}

// recordIssueChange saves the fetched edit and a needs-input incident.
func (s *Service) recordIssueChange(ctx context.Context, r *domain.ExecutionRecord, title, body string) error {
	now := time.Now().UTC()
	r.PendingIssue = &domain.IssueRevision{Title: title, Body: body, FetchedAt: now}
	d := &domain.ExecutionIncident{Kind: "issue_changed", Source: "github", Summary: "The upstream issue was edited after you approved this contribution.", Evidence: "The current issue title or body differs from the approved text. Both versions are saved for comparison.", NextAction: "Review the updated issue. Approve it to continue with a fresh plan (existing edits are kept), or abandon the contribution.", Status: "needs_input", DetectedAt: now, DiagnosisVersion: diagnosisVersion}
	if err := s.incident(ctx, r, d); err != nil {
		return err
	}
	return errIssueChanged
}

// approveIssueUpdate runs under s.mu with no active task for the contribution.
func (s *Service) approveIssueUpdate(ctx context.Context, id string) error {
	r, err := s.Store.Execution(ctx, id)
	if err != nil {
		return err
	}
	if r.PendingIssue == nil {
		return errors.New("no issue update is awaiting approval")
	}
	a, err := s.Store.Approval(ctx, id)
	if err != nil {
		return err
	}
	previousTitle, previousBody, err := approvedIssueText(a, &r)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	revision := *r.PendingIssue
	revision.ApprovedAt = &now
	r.ApprovedIssue, r.PendingIssue = &revision, nil
	r.PlanInvalidated, r.PlanApproved = true, false
	r.Incident = nil
	if r.Status == "NEEDS_INPUT" || r.Status == "BLOCKED" {
		r.Status = "PAUSED"
	}
	r.Message = "Updated issue approved. Resume to plan against it; existing edits are kept."
	if err = s.Store.WorkspaceEvent(ctx, id, "IssueUpdateApproved", "Human approved the edited upstream issue; a fresh plan is required", map[string]string{"previous_title": previousTitle, "previous_body": previousBody, "title": revision.Title, "body": revision.Body}); err != nil {
		return err
	}
	return s.Store.SaveExecution(ctx, r, "user", "Human approved the updated issue; a fresh plan is required before coding continues")
}
