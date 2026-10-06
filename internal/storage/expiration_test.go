package storage

import (
	"context"
	"testing"
	"time"
)

func TestTemporaryConfigExpiresWithoutOverwritingNewerChanges(t *testing.T) {
	ctx := context.Background()
	s, e := Open(ctx, ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	v, _ := s.CurrentConfig(ctx)
	v.Config.Profile.Languages["Rust"] = 0.8
	until := time.Now().Add(time.Hour)
	p, e := s.ProposeTimed(ctx, v.Version, v.Config, "Temporary Rust", &until)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Apply(ctx, p.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.ExpireConfigs(ctx, until.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	v, _ = s.CurrentConfig(ctx)
	if v.Version != 3 || v.Config.Profile.Languages["Rust"] != 0 {
		t.Fatal("temporary configuration not restored")
	}
	v.Config.Profile.Languages["Rust"] = 0.8
	p, _ = s.ProposeTimed(ctx, v.Version, v.Config, "Temporary again", &until)
	s.Apply(ctx, p.ID)
	v, _ = s.CurrentConfig(ctx)
	v.Config.Profile.Languages["C++"] = 0.5
	newer, _ := s.Propose(ctx, v.Version, v.Config, "Manual change supersedes temporary profile")
	s.Apply(ctx, newer.ID)
	if e = s.ExpireConfigs(ctx, until.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	v, _ = s.CurrentConfig(ctx)
	if v.Config.Profile.Languages["C++"] != 0.5 || v.Version != 5 {
		t.Fatal("expiry overwrote newer manual changes")
	}
}
