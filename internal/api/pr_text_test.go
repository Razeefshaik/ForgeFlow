package api

import (
	"bytes"
	"context"
	"encoding/json"
	"forgeflow/internal/domain"
	"forgeflow/internal/execution"
	"forgeflow/internal/storage"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPRDraftHTTPRevisionAndConflict(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, ":memory:", true)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.SeedOnce(ctx, []domain.Opportunity{{ID: "o", Demo: true}}, []domain.Contribution{{ID: "c", OpportunityID: "o", State: "PR_PREPARED", Demo: true}}); err != nil {
		t.Fatal(err)
	}
	record := domain.ExecutionRecord{ContributionID: "c", Status: "PR_PREPARED", PRTitle: "Fix addition", PRBody: "Fixes #1", HeadCommit: "fixture-head"}
	if err = store.SaveExecution(ctx, record, "fixture", "draft"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer((Server{Store: store, Execution: execution.New(ctx, store, nil, nil, t.TempDir(), t.TempDir())}).Handler())
	defer server.Close()
	response, err := http.Get(server.URL + "/api/contributions/c/execution")
	if err != nil {
		t.Fatal(err)
	}
	var fetched domain.ExecutionRecord
	if err = json.NewDecoder(response.Body).Decode(&fetched); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || fetched.PRRevision == "" {
		t.Fatal("revision unavailable")
	}
	body, _ := json.Marshal(map[string]string{"title": "Correct addition", "body": "Added regression coverage. Fixes #1", "revision": fetched.PRRevision})
	post := func() *http.Response {
		t.Helper()
		r, e := http.Post(server.URL+"/api/contributions/c/pr-text", "application/json", bytes.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	response = post()
	var edited domain.ExecutionRecord
	if err = json.NewDecoder(response.Body).Decode(&edited); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || edited.PRTitle != "Correct addition" || edited.SubmissionToken == "" || edited.PRRevision == fetched.PRRevision {
		t.Fatalf("draft edit failed: %+v status=%d", edited, response.StatusCode)
	}
	response = post()
	response.Body.Close()
	if response.StatusCode != 409 {
		t.Fatalf("stale HTTP edit accepted: %d", response.StatusCode)
	}
}
