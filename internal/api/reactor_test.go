package api

import (
	"context"
	"encoding/json"
	"forgeflow/internal/domain"
	"forgeflow/internal/execution"
	"forgeflow/internal/seed"
	"forgeflow/internal/storage"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReactorDiagnosisIsReadOnlyAndRecoveryRequiresApproval(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, ":memory:", true)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = seed.Load(ctx, store); err != nil {
		t.Fatal(err)
	}
	const id = "demo-etcd"
	if err = store.SaveExecution(ctx, domain.ExecutionRecord{ContributionID: id, Status: "BLOCKED", Summary: "dependency download failed"}, "fixture", "synthetic blocked execution"); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveTest(ctx, domain.TestRun{ID: "failed-download", ContributionID: id, Command: domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, ExitCode: 1, Output: "proxyconnect tcp: dial tcp 127.0.0.1:9: connection refused", FinishedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	before, _ := store.Events(ctx, 0, id, 100)
	service := execution.New(ctx, store, nil, nil, t.TempDir(), t.TempDir())
	server := httptest.NewServer((Server{Store: store, Execution: service}).Handler())
	defer server.Close()
	response, err := http.Get(server.URL + "/api/contributions/" + id + "/diagnosis")
	if err != nil {
		t.Fatal(err)
	}
	var diagnosis domain.ExecutionIncident
	err = json.NewDecoder(response.Body).Decode(&diagnosis)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || diagnosis.Kind != "network_transport" || diagnosis.CanRecover || diagnosis.TestRunID != "failed-download" {
		t.Fatalf("incorrect diagnosis: %+v %v", diagnosis, err)
	}
	response, err = http.Post(server.URL+"/api/contributions/"+id+"/recover", "application/json", strings.NewReader(`{"approved":false,"network":true}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatal("unapproved recovery accepted", response.StatusCode)
	}
	after, _ := store.Events(ctx, 0, id, 100)
	if len(after) != len(before) {
		t.Fatal("diagnosis or unapproved recovery mutated execution")
	}
}
