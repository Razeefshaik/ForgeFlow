package api

import (
	"context"
	"forgeflow/internal/codex"
	"forgeflow/internal/storage"
	"net/http/httptest"
	"testing"
)

func TestEnvironmentAPIRequiresApprovalAndRejectsCrossOriginCommands(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store, err := storage.Open(ctx, ":memory:", false)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m, err := codex.NewEnvironmentManager(ctx, &codex.CLI{Binary: "fixture-codex"}, t.TempDir(), t.TempDir(), func(ctx context.Context, kind, msg string, data any) error {
		return store.WorkspaceEvent(ctx, "execution-environment", kind, msg, data)
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer((Server{Store: store, Environment: m}).Handler())
	defer server.Close()
	for _, tc := range []struct {
		action, body, origin string
		status               int
	}{
		{"select", `{"mode":"elevated","approved":false}`, server.URL, 409},
		{"setup", `{"approved":false}`, server.URL, 409},
		{"select", `{"mode":"danger-full-access","approved":true}`, server.URL, 409},
		{"select", `{"mode":"elevated","approved":true}`, "https://untrusted.example", 403},
		{"setup", `{"approved":true,"command":"anything"}`, server.URL, 400},
	} {
		r := post(t, server, "/api/runtime/environment/"+tc.action, tc.body, tc.origin)
		r.Body.Close()
		if r.StatusCode != tc.status {
			t.Fatal(tc, r.StatusCode)
		}
	}
	if m.Status().Running {
		t.Fatal("rejected request launched setup")
	}
	events, _ := store.Events(ctx, 0, "execution-environment", 100)
	if len(events) != 0 {
		t.Fatal("rejected action changed audit", events)
	}
}
