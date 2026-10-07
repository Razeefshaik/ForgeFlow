package execution

import (
	"context"
	"forgeflow/internal/domain"
	"strings"
	"testing"
)

func TestCurrentPhaseMessageReplacesOldFailureWithoutDestroyingSummary(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	ctx := context.Background()
	r := domain.ExecutionRecord{ContributionID: c.ID, Status: "RUNNING", Summary: "Previous Git-history error"}
	if err := s.phase(ctx, &r, "PLANNING"); err != nil {
		t.Fatal(err)
	}
	saved, err := s.Store.Execution(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	current, err := s.Store.Contribution(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Message == "" || strings.Contains(saved.Message, "Git-history") || current.Message != saved.Message {
		t.Fatalf("stale execution message: %+v %+v", saved, current)
	}
	if saved.Summary != r.Summary {
		t.Fatal("phase update destroyed implementation summary")
	}
}
