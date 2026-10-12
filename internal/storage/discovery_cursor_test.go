package storage

import (
	"context"
	"forgeflow/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestDiscoveryProgressAndObservationsCommitTogether(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, ":memory:", false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	scope := strings.Repeat("a", 64)
	r := domain.DiscoveryRun{ID: "scan", ConfigVersion: 1, Status: "RUNNING", StartedAt: time.Now().UTC(), SearchScope: scope, NextSearchCursors: map[string]domain.DiscoveryCursor{"Go": {Page: 1, Offset: 2}}}
	if err = s.StartDiscovery(ctx, r); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC()
	r.Status, r.FinishedAt = "SUCCEEDED", &end
	bad := domain.Opportunity{ID: "bad", Demo: true, Evidence: &domain.Evidence{RunID: r.ID, ConfigVersion: r.ConfigVersion}}
	if err = s.FinishDiscovery(ctx, r, []domain.Opportunity{bad}); err == nil {
		t.Fatal("invalid observation committed")
	}
	c, err := s.DiscoveryCursors(ctx, scope)
	if err != nil || len(c) != 0 {
		t.Fatal("failed scan advanced durable progress", c, err)
	}
	good := domain.Opportunity{ID: "good", Repository: "new/repo", Evidence: &domain.Evidence{RunID: r.ID, ConfigVersion: r.ConfigVersion}}
	if err = s.FinishDiscovery(ctx, r, []domain.Opportunity{good}); err != nil {
		t.Fatal(err)
	}
	c, err = s.DiscoveryCursors(ctx, scope)
	if err != nil || c["Go"].Offset != 2 {
		t.Fatal(c, err)
	}
	r.ID = "refresh"
	r.Status = "RUNNING"
	r.FinishedAt = nil
	r.StartedAt = time.Now().UTC()
	if err = s.StartDiscovery(ctx, r); err != nil {
		t.Fatal(err)
	}
	end = time.Now().UTC()
	r.Status, r.FinishedAt = "SUCCEEDED", &end
	good.Evidence.RunID = r.ID
	if err = s.FinishDiscovery(ctx, r, []domain.Opportunity{good}); err != nil {
		t.Fatal(err)
	}
	runs, err := s.DiscoveryRuns(ctx)
	if err != nil || *runs[0].NewOpportunities != 0 || *runs[0].RefreshedOpportunities != 1 {
		t.Fatal("refresh was counted as a new opportunity", runs, err)
	}
}
