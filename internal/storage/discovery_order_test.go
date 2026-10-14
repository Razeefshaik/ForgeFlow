package storage

import (
	"context"
	"forgeflow/internal/domain"
	"testing"
	"time"
)

func TestDiscoveryRunsWithSameTimestampReturnNewestFirst(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, ":memory:", false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	before, err := s.Events(ctx, 0, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"z-first", "a-second"} {
		run := domain.DiscoveryRun{ID: id, ConfigVersion: 1, Status: "RUNNING", StartedAt: stamp}
		if err = s.StartDiscovery(ctx, run); err != nil {
			t.Fatal(err)
		}
		run.Status, run.FinishedAt = "SUCCEEDED", &stamp
		if err = s.FinishDiscovery(ctx, run, nil); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := s.DiscoveryRuns(ctx)
	if err != nil || len(runs) != 2 || runs[0].ID != "a-second" || runs[1].ID != "z-first" {
		t.Fatalf("newest scan must win equal timestamps: %+v %v", runs, err)
	}
	events, err := s.Events(ctx, 0, "", 100)
	if err != nil || len(events) != len(before)+4 {
		t.Fatal("scan audit history changed", events, err)
	}
}
