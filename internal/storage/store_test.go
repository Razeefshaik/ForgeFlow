package storage

import (
	"context"
	"forgeflow/internal/config"
	"forgeflow/internal/domain"
	"path/filepath"
	"testing"
)

func TestConfigApprovalConflictRollbackAndDurability(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	s, e := Open(ctx, path, false)
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.CurrentConfig(ctx)
	if e != nil {
		t.Fatal(e)
	}
	c := v.Config
	c.Profile.Languages["Rust"] = 0.8
	p, e := s.Propose(ctx, v.Version, c, "Include Rust")
	if e != nil {
		t.Fatal(e)
	}
	p2, e := s.Propose(ctx, v.Version, c, "Concurrent proposal")
	if e != nil {
		t.Fatal(e)
	}
	before, _ := s.CurrentConfig(ctx)
	if before.Version != v.Version {
		t.Fatal("proposal applied without human action")
	}
	applied, e := s.Apply(ctx, p.ID)
	if e != nil {
		t.Fatal(e)
	}
	if applied.Version != 2 {
		t.Fatal(applied)
	}
	if _, e = s.Apply(ctx, p2.ID); e != ErrConflict {
		t.Fatalf("stale proposal: %v", e)
	}
	if _, e = s.Rollback(ctx, 1, 1); e != ErrConflict {
		t.Fatal("stale rollback accepted")
	}
	restored, e := s.Rollback(ctx, 1, 2)
	if e != nil {
		t.Fatal(e)
	}
	if restored.Version != 3 || restored.Config.Profile.Languages["Rust"] != 0 {
		t.Fatal("rollback didn't restore snapshot")
	}
	es, e := s.Events(ctx, 0, "", 100)
	if e != nil {
		t.Fatal(e)
	}
	if len(es) != 5 {
		t.Fatalf("want 5 events got %d", len(es))
	}
	if _, e = s.db.Exec("DELETE FROM events"); e == nil {
		t.Fatal("audit events were deletable")
	}
	if _, e = s.db.Exec("UPDATE config_versions SET reason='rewritten'"); e == nil {
		t.Fatal("config history was mutable")
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s, e = Open(ctx, path, false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	history, e := s.ConfigHistory(ctx)
	if e != nil || len(history) != 3 {
		t.Fatalf("history lost: %v %v", history, e)
	}
	es, e = s.Events(ctx, es[2].ID, "", 100)
	if e != nil || len(es) != 2 {
		t.Fatal("event replay lost cursor")
	}
}
func TestSafetyGateRejectedWithoutAuditSideEffects(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	c := config.Default()
	c.Contributions.AutoMerge = true
	if _, e = s.Propose(ctx, 1, c, "Enable merging"); e == nil {
		t.Fatal("unsafe config accepted")
	}
	es, e := s.Events(ctx, 0, "", 100)
	if e != nil || len(es) != 1 {
		t.Fatal("rejected proposal wrote audit events")
	}
}
func TestModeIsolationAndAtomicSeed(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "demo.db")
	s, e := Open(ctx, path, true)
	if e != nil {
		t.Fatal(e)
	}
	opps := []domain.Opportunity{{ID: "demo", Repository: "example/repo", Demo: true, Ranking: domain.Ranking{Score: 90}}}
	cs := []domain.Contribution{{ID: "c", OpportunityID: "missing", State: "CODING", Demo: true}}
	if e = s.SeedOnce(ctx, opps, cs); e == nil {
		t.Fatal("invalid foreign key accepted")
	}
	list, _ := s.Opportunities(ctx)
	if len(list) != 0 {
		t.Fatal("partial seed survived rollback")
	}
	cs[0].OpportunityID = "demo"
	if e = s.SeedOnce(ctx, opps, cs); e != nil {
		t.Fatal(e)
	}
	if e = s.SeedOnce(ctx, opps, cs); e != nil {
		t.Fatal(e)
	}
	es, _ := s.Events(ctx, 0, "", 100)
	if len(es) != 3 {
		t.Fatal("seed duplicated events")
	}
	if e = s.Transition(ctx, "c", "READY"); e == nil {
		t.Fatal("skipped tests/review")
	}
	if e = s.Transition(ctx, "c", "PAUSED"); e != nil {
		t.Fatal(e)
	}
	if e = s.Transition(ctx, "c", "TESTING"); e == nil {
		t.Fatal("invalid resume")
	}
	if e = s.Transition(ctx, "c", "CODING"); e != nil {
		t.Fatal(e)
	}
	listCS, _ := s.Contributions(ctx)
	if listCS[0].State != "CODING" || listCS[0].PreviousState != "" {
		t.Fatal(listCS)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	if live, e := Open(ctx, path, false); e == nil {
		live.Close()
		t.Fatal("opened demo database in live mode")
	}
	live, e := Open(ctx, ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	defer live.Close()
	if e = live.SeedOnce(ctx, opps, nil); e == nil {
		t.Fatal("live seed accepted")
	}
}
