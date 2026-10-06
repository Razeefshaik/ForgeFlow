package operator

import (
	"context"
	"forgeflow/internal/codex"
	"forgeflow/internal/storage"
	"os"
	"testing"
)

func TestRealOperatorTemporaryMultiSettingProposal(t *testing.T) {
	if os.Getenv("FORGEFLOW_REAL_CODEX_TEST") != "1" {
		t.Skip("opt in to authenticated Codex Operator")
	}
	ctx := context.Background()
	store, err := storage.Open(ctx, ":memory:", false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	op := Service{Store: store, AI: codex.Resolve("", ""), Root: t.TempDir()}
	reply, err := op.Chat(ctx, "Include Rust, prioritize databases and distributed systems with weight 1, and exclude frontend-only work for the next hour. Preserve all other settings. Propose these changes for my review.")
	if err != nil {
		events, _ := store.Events(ctx, 0, "operator", 100)
		for _, event := range events {
			if event.Type == "OperatorModelCompleted" {
				t.Logf("model fixture response: %+v", event.Data)
			}
		}
		t.Fatal(err)
	}
	if reply.Proposal == nil || reply.Proposal.ExpiresAt == nil || reply.Proposal.Config.Profile.Languages["Rust"] <= 0 || reply.Proposal.Config.Profile.Domains["databases"] != 1 || reply.Proposal.Config.Profile.Domains["distributed-systems"] != 1 {
		t.Fatalf("missing proposal fields: %+v", reply)
	}
	cfg, err := store.CurrentConfig(ctx)
	if err != nil || cfg.Version != 1 || cfg.Config.Profile.Languages["Rust"] != 0 {
		t.Fatal("Operator applied a proposal without approval", err)
	}
	if _, err = store.Apply(ctx, reply.Proposal.ID); err != nil {
		t.Fatal(err)
	}
	t.Log("authenticated AI produced a validated, expiring multi-setting proposal; configuration changed only after explicit Apply")
}
