package execution

import (
	"context"
	"encoding/json"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiscussionInvalidatesLegacyPlanAndPreservesEvidence(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	ctx := context.Background()
	a, _ := s.Store.Approval(ctx, c.ID)
	if err := s.Store.Transition(ctx, c.ID, "PLANNING"); err != nil {
		t.Fatal(err)
	}
	body := "Root fix landed, but nested modules and missing packages remain."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("per_page") != "100" {
			t.Error("unbounded comment request")
		}
		json.NewEncoder(w).Encode([]domain.IssueComment{{ID: 1, Body: body, URL: "https://example.invalid/comment/1"}})
	}))
	defer server.Close()
	s.Client = github.New("")
	s.Client.BaseURL = server.URL
	r := domain.ExecutionRecord{ContributionID: c.ID, Phase: "PLANNING", Plan: &domain.Plan{Summary: "already fixed"}, PlanApproved: true}
	if err := s.refreshIssueContext(ctx, a, &r); err != nil {
		t.Fatal(err)
	}
	if r.Plan != nil || r.PlanApproved || !r.PlanInvalidated || len(r.IssueDiscussion) != 1 {
		t.Fatal("stale plan survived", r)
	}
	repo, _, _ := s.Paths(c)
	prompt, err := issueDiscussionPrompt(repo, &r)
	if err != nil || !strings.Contains(prompt, "complete issue discussion") {
		t.Fatal(prompt, err)
	}
	r.Plan = &domain.Plan{Summary: "new plan"}
	r.PlanInvalidated = false
	if err := s.refreshIssueContext(ctx, a, &r); err != nil || r.Plan == nil {
		t.Fatal("unchanged discussion wasted a plan", err)
	}
	body = "A newly verified acceptance criterion."
	if err := s.refreshIssueContext(ctx, a, &r); err != nil || r.Plan != nil {
		t.Fatal("edited comment did not invalidate", err)
	}
}

func TestDiscussionBudgetFailsBeforeReplacingSnapshot(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		json.NewEncoder(w).Encode([]domain.IssueComment{{Body: strings.Repeat("x", 257<<10)}})
	}))
	defer server.Close()
	s.Client = github.New("")
	s.Client.BaseURL = server.URL
	a, _ := s.Store.Approval(context.Background(), c.ID)
	r := domain.ExecutionRecord{ContributionID: c.ID, Plan: &domain.Plan{Summary: "preserved"}}
	if err := s.refreshIssueContext(context.Background(), a, &r); err == nil || r.Plan == nil {
		t.Fatal("oversized scope accepted")
	}
}
