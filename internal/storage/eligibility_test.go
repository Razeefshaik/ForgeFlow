package storage

import (
	"context"
	"encoding/json"
	"forgeflow/internal/domain"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func eligibilityFixture(t *testing.T, s *Store, id, repo string) {
	t.Helper()
	o, err := json.Marshal(domain.Opportunity{ID: id, Repository: repo})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO opportunities VALUES (?,?,?) ON CONFLICT(id) DO NOTHING", id, string(o), 0); err != nil {
		t.Fatal(err)
	}
	c := domain.Contribution{ID: id, OpportunityID: id, Repository: repo, State: "PAUSED"}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO contributions VALUES (?,?,?)", id, id, string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestEligibilityPreservesScoresHistoryAndRestoresAfterRecovery(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "eligibility.db")
	s, err := Open(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	run := domain.DiscoveryRun{ID: "scan", Status: "RUNNING", ConfigVersion: 1, StartedAt: time.Now().UTC()}
	if err = s.StartDiscovery(ctx, run); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC()
	run.Status, run.FinishedAt = "SUCCEEDED", &end
	opps := []domain.Opportunity{
		{ID: "blocked", Repository: "Example/Blocked", Ranking: domain.Ranking{Score: 95}, Evidence: &domain.Evidence{RunID: run.ID, ConfigVersion: 1}},
		{ID: "eligible", Repository: "Example/Eligible", Ranking: domain.Ranking{Score: 80}, Evidence: &domain.Evidence{RunID: run.ID, ConfigVersion: 1}},
	}
	if err = s.FinishDiscovery(ctx, run, opps); err != nil {
		t.Fatal(err)
	}
	eligibilityFixture(t, s, "old", "example/blocked")
	r := domain.ExecutionRecord{ContributionID: "old", Status: "NEEDS_INPUT", Incident: &domain.ExecutionIncident{Kind: "verification_timeout", Source: "environment", Summary: "Required Git provenance check exceeds deadline", NextAction: "Repair Git metadata access", DetectedAt: end}}
	if err = s.SaveExecution(ctx, r, "fixture", "real diagnosis fixture"); err != nil {
		t.Fatal(err)
	}
	a := domain.RepositoryAssessment{Repository: "example/blocked", HeadSHA: strings.Repeat("a", 40), Status: "excluded", Profile: "fixture-v1", CheckedAt: end, Findings: []domain.PrerequisiteFinding{{Kind: "trusted_signing", Source: "CONTRIBUTING.md", Line: 1, Reason: "Required trusted signer is unavailable"}}}
	if err = s.SaveRepositoryAssessment(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	all, err := s.Opportunities(ctx)
	if err != nil || len(all) != 1 || all[0].ID != "eligible" || all[0].Ranking.Score != 80 {
		t.Fatal(all, err)
	}
	original, err := s.Opportunity(ctx, "blocked")
	if err != nil || original.Ranking.Score != 95 {
		t.Fatal("historical score lost", original, err)
	}
	if err = s.RequireRepositoryEligible(ctx, "EXAMPLE/BLOCKED"); err == nil {
		t.Fatal("case variant bypassed exclusion")
	}
	cfg, err := s.CurrentConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// A preview obtained before the blocker was saved must not create new work.
	if err = s.ApproveWorkspace(ctx, domain.Contribution{ID: "new", OpportunityID: "other-issue", Repository: "Example/Blocked"}, domain.WorkspaceApproval{Config: cfg}); err == nil {
		t.Fatal("approval transaction bypassed exclusion")
	}
	if _, err = s.Contribution(ctx, "new"); err != ErrNotFound {
		t.Fatal("rejected contribution was saved", err)
	}
	exclusions, err := s.RepositoryExclusions(ctx)
	if err != nil || len(exclusions) != 1 || exclusions[0].HeadSHA != a.HeadSHA || exclusions[0].Reason != a.Findings[0].Reason {
		t.Fatal(exclusions, err)
	}
	r.Incident, r.Status = nil, "READY"
	if err = s.SaveExecution(ctx, r, "fixture", "fresh verification resolved incident"); err != nil {
		t.Fatal(err)
	}
	a.Status, a.Findings, a.CheckedAt = "eligible", nil, time.Now().UTC()
	if err = s.SaveRepositoryAssessment(ctx, a); err != nil {
		t.Fatal(err)
	}
	all, err = s.Opportunities(ctx)
	if err != nil || len(all) != 3 || all[0].Ranking.Score != 95 {
		t.Fatal("recovery did not restore ranking", all, err)
	}
	events, err := s.Events(ctx, 0, "old", 100)
	if err != nil || len(events) != 2 {
		t.Fatal("diagnosis history erased", events, err)
	}
	if err = s.RequireRepositoryEligible(ctx, "example/blocked"); err != nil {
		t.Fatal(err)
	}
}

func TestAbandonedAndFailedExecutionsDoNotCreateSearchExclusions(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, ":memory:", false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	eligibilityFixture(t, s, "c", "owner/repo")
	for _, status := range []string{"ABANDONED", "FAILED", "NEEDS_INPUT", "BLOCKED"} {
		r := domain.ExecutionRecord{ContributionID: "c", Status: status, Incident: &domain.ExecutionIncident{Kind: "environment_blocked", Source: "environment", Summary: "Old failure"}}
		if err = s.SaveExecution(ctx, r, "fixture", "retained old outcome"); err != nil {
			t.Fatal(err)
		}
		all, err := s.RepositoryExclusions(ctx)
		if err != nil || len(all) != 0 {
			t.Fatal("historical contribution determined new repository eligibility", all, err)
		}
	}
}

func TestCurrentRunnerProfileRequiresFreshAssessmentForRanking(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, ":memory:", false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	eligibilityFixture(t, s, "o", "new/repo")
	s.AssessmentProfile = "current-profile"
	list, err := s.Opportunities(ctx)
	if err != nil || len(list) != 0 {
		t.Fatal("unassessed historical opportunity entered current ranking", list, err)
	}
	a := domain.RepositoryAssessment{Repository: "new/repo", HeadSHA: strings.Repeat("a", 40), Status: "eligible", Profile: s.AssessmentProfile, CheckedAt: time.Now().UTC()}
	if err = s.SaveRepositoryAssessment(ctx, a); err != nil {
		t.Fatal(err)
	}
	list, err = s.Opportunities(ctx)
	if err != nil || len(list) != 1 {
		t.Fatal(list, err)
	}
	// A stale scan cannot erase fresher prerequisite evidence.
	older := a
	older.CheckedAt = a.CheckedAt.Add(-time.Minute)
	older.Status = "excluded"
	older.Findings = []domain.PrerequisiteFinding{{Kind: "trusted_signing", Reason: "obsolete", Source: "CONTRIBUTING.md"}}
	if err = s.SaveRepositoryAssessment(ctx, older); err != nil {
		t.Fatal(err)
	}
	list, err = s.Opportunities(ctx)
	if err != nil || len(list) != 1 {
		t.Fatal("obsolete scan replaced a newer assessment", list, err)
	}
	s.AssessmentProfile = "different-runner"
	list, err = s.Opportunities(ctx)
	if err != nil || len(list) != 0 {
		t.Fatal("old runner assessment reused", list, err)
	}
	x, err := s.RepositoryExclusions(ctx)
	if err != nil || len(x) != 1 || x[0].Status != "needs_assessment" {
		t.Fatal(x, err)
	}
}
