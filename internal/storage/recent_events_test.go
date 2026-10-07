package storage

import (
	"context"
	"fmt"
	"testing"
)

func TestRecentEntityEventsReturnsNewestPageAndPreservesReplay(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, ":memory:", false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < 140; i++ {
		if err = s.WorkspaceEvent(ctx, "active", "Progress", fmt.Sprintf("active-%d", i), nil); err != nil {
			t.Fatal(err)
		}
		if err = s.WorkspaceEvent(ctx, "other", "Progress", "interleaved unrelated event", nil); err != nil {
			t.Fatal(err)
		}
	}
	recent, err := s.RecentEntityEvents(ctx, "active")
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 100 || recent[0].Message != "active-40" || recent[99].Message != "active-139" {
		t.Fatalf("timeline did not advance past first page: %+v", recent)
	}
	replay, err := s.Events(ctx, 0, "active", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(replay) != 100 || replay[0].Message != "active-0" || replay[99].Message != "active-99" {
		t.Fatal("cursor replay was changed")
	}
}
