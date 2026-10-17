package execution

import (
	"context"
	"encoding/json"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type promptRunner struct {
	fixtureRunner
	mu      sync.Mutex
	prompts map[string]string
}

func (p *promptRunner) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	p.mu.Lock()
	if p.prompts == nil {
		p.prompts = map[string]string{}
	}
	p.prompts[r.Role] = r.Prompt
	p.mu.Unlock()
	return p.fixtureRunner.Run(ctx, r, event)
}

// An issue edited after approval used to block every resume with "issue scope
// changed" and no way forward. The edit is now saved for review, recovery is
// refused until a human approves it, and the approved text drives a fresh plan.
func TestEditedIssueNeedsApprovalThenContinuesWithFreshPlan(t *testing.T) {
	runner := &promptRunner{}
	s, c := buildFixture(t, runner)
	ctx := context.Background()
	title, body := "Correct Add for all integers", "The Add function subtracts instead of adding. Fix addition, add positive and negative regression coverage, and keep the signature."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/comments"):
			json.NewEncoder(w).Encode([]domain.IssueComment{})
		case strings.HasSuffix(req.URL.Path, "/issues/1"):
			json.NewEncoder(w).Encode(map[string]any{"number": 1, "state": "open", "title": title, "body": body})
		default:
			json.NewEncoder(w).Encode(map[string]any{"full_name": "example/fixture", "default_branch": "main"})
		}
	}))
	defer server.Close()
	s.Client = github.New("")
	s.Client.BaseURL = server.URL
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, _ := s.Store.Execution(ctx, c.ID)
	current, _ := s.Store.Contribution(ctx, c.ID)
	if current.State != "PAUSED" || r.Status != "NEEDS_INPUT" || r.Incident == nil || r.Incident.Kind != "issue_changed" || r.PendingIssue == nil || r.PendingIssue.Body != body {
		t.Fatalf("edited issue was not saved for review: state=%s record=%+v", current.State, r)
	}
	if runner.contributors != 0 {
		t.Fatal("coding ran against an unapproved issue")
	}
	if _, err := s.Recover(ctx, c.ID, true, false); err == nil || !strings.Contains(err.Error(), "approve the updated issue") {
		t.Fatal("recovery bypassed the issue update approval", err)
	}
	if err := s.Control(ctx, c.ID, "approve-issue", false); err == nil {
		t.Fatal("issue update approved without confirmation")
	}
	if err := s.Control(ctx, c.ID, "approve-issue", true); err != nil {
		t.Fatal(err)
	}
	r, _ = s.Store.Execution(ctx, c.ID)
	if r.ApprovedIssue == nil || r.ApprovedIssue.ApprovedAt == nil || r.PendingIssue != nil || !r.PlanInvalidated || r.Incident != nil {
		t.Fatalf("approval not recorded: %+v", r)
	}
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, _ = s.Store.Execution(ctx, c.ID)
	if r.Status != "READY" {
		t.Fatalf("approved update did not continue: %+v", r)
	}
	if !strings.Contains(runner.prompts["planner"], "keep the signature") || !strings.Contains(runner.prompts["contributor"], "keep the signature") {
		t.Fatal("agents did not receive the approved issue text")
	}
	if !strings.Contains(r.PRTitle, title) {
		t.Fatalf("PR title kept the superseded issue title: %q", r.PRTitle)
	}
	approval, _ := s.Store.Approval(ctx, c.ID)
	if !strings.Contains(string(approval.Issue), "Fix addition and add positive and negative regression coverage.") {
		t.Fatal("the original approval record was rewritten")
	}
}
