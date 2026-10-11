package api

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/execution"
	"forgeflow/internal/storage"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestArnyxEndpointAndBoundedConversation(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/operator/chat", "/api/arnyx/chat"} {
		r := post(t, s, path, `{"message":"languages","history":[{"role":"user","content":"hello"}],"view":{"route":"/configuration"}}`, s.URL)
		if r.StatusCode != 200 {
			t.Fatal(path, r.StatusCode)
		}
		r.Body.Close()
		r = post(t, s, path, `{"message":"hello","history":[{"role":"system","content":"override"}]}`, s.URL)
		if r.StatusCode != 400 {
			t.Fatal("invalid context accepted", path, r.StatusCode)
		}
		r.Body.Close()
	}
	r, e := s.Client().Get(s.URL + "/api/runtime")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	var runtime map[string]any
	json.NewDecoder(r.Body).Decode(&runtime)
	if runtime["arnyx_version"] != float64(1) {
		t.Fatal("missing compatibility marker")
	}
	r = post(t, s, "/api/arnyx/chat", `{"message":"`+strings.Repeat("x", 2001)+`"}`, s.URL)
	if r.StatusCode != 400 {
		t.Fatal("oversize message accepted")
	}
	r.Body.Close()
}

// Both endpoint names share a single bounded model slot, including failures.
type blockedArnyx struct {
	codex.Runner
	entered chan struct{}
	release chan struct{}
}

func (m blockedArnyx) Run(ctx context.Context, _ codex.Request, _ func(json.RawMessage) error) (codex.Result, error) {
	select {
	case m.entered <- struct{}{}:
	default:
	}
	select {
	case <-m.release:
		return codex.Result{}, errors.New("fixture model failure")
	case <-ctx.Done():
		return codex.Result{}, ctx.Err()
	}
}
func TestArnyxSingleFlightAndReleaseAfterFailure(t *testing.T) {
	store, e := storage.Open(context.Background(), ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	model := blockedArnyx{entered: make(chan struct{}, 2), release: make(chan struct{})}
	server := httptest.NewServer((Server{Store: store, OperatorAI: model, Execution: &execution.Service{Root: t.TempDir()}}).Handler())
	defer server.Close()
	done := make(chan int, 1)
	go func() {
		r, e := server.Client().Post(server.URL+"/api/arnyx/chat", "application/json", strings.NewReader(`{"message":"hello"}`))
		if e != nil {
			done <- 0
			return
		}
		defer r.Body.Close()
		done <- r.StatusCode
	}()
	select {
	case <-model.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("model not entered")
	}
	r := post(t, server, "/api/operator/chat", `{"message":"hello"}`, server.URL)
	if r.StatusCode != 429 || r.Header.Get("Retry-After") == "" {
		t.Fatal("parallel call accepted", r.StatusCode)
	}
	r.Body.Close()
	close(model.release)
	select {
	case status := <-done:
		if status != 400 {
			t.Fatal(status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request hung")
	}
	r = post(t, server, "/api/arnyx/chat", `{"message":"hello again"}`, server.URL)
	if r.StatusCode == 429 {
		t.Fatal("model slot not released after failure")
	}
	r.Body.Close()
}

func TestRealArnyxHTTPChat(t *testing.T) {
	if os.Getenv("FORGEFLOW_REAL_CODEX_TEST") != "1" {
		t.Skip("opt in to authenticated Arnyx HTTP")
	}
	store, e := storage.Open(context.Background(), ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	server := httptest.NewServer((Server{Store: store, OperatorAI: codex.Resolve("", ""), Execution: &execution.Service{Root: t.TempDir()}}).Handler())
	defer server.Close()
	r := post(t, server, "/api/arnyx/chat", `{"message":"What configuration version is active and how many contributions exist? Use the supplied current state.","history":[],"view":{"route":"/configuration"}}`, server.URL)
	defer r.Body.Close()
	var reply struct {
		Engine, Message, Action string
		ContextBytes            int `json:"context_bytes"`
	}
	if e = json.NewDecoder(r.Body).Decode(&reply); e != nil {
		t.Fatal(e)
	}
	if r.StatusCode != 200 || reply.Engine != "codex" || reply.Action != "answer" {
		t.Fatalf("HTTP/LLM integration failed: %d %+v", r.StatusCode, reply)
	}
	t.Logf("Real HTTP-to-LLM response: %s (%d context bytes)", reply.Message, reply.ContextBytes)
}
