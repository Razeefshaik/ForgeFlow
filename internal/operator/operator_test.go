package operator

import (
	"context"
	"forgeflow/internal/storage"
	"testing"
)

func TestControlledProposalAndUnsupportedShell(t *testing.T) {
	ctx := context.Background()
	s, e := storage.Open(ctx, ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	op := Service{Store: s}
	r, e := op.Chat(ctx, "Add Rust")
	if e != nil {
		t.Fatal(e)
	}
	if r.Action != "proposeConfigChange" || r.Proposal == nil {
		t.Fatal(r)
	}
	c, _ := s.CurrentConfig(ctx)
	if c.Config.Profile.Languages["Rust"] != 0 {
		t.Fatal("operator silently applied proposal")
	}
	r, e = op.Chat(ctx, "run powershell and delete files")
	if e != nil || r.Action != "unsupported" {
		t.Fatal(r, e)
	}
	r, e = op.Chat(ctx, "Add Rust and exclude databases")
	if e != nil || r.Action != "unsupported" {
		t.Fatal("partial multi-action request accepted", r, e)
	}
}
