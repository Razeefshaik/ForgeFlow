package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"forgeflow/internal/config"
	"forgeflow/internal/discovery"
	"forgeflow/internal/github"
	"forgeflow/internal/seed"
	"forgeflow/internal/storage"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	s, e := storage.Open(context.Background(), ":memory:", true)
	if e != nil {
		t.Fatal(e)
	}
	if e = seed.Load(context.Background(), s); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer((Server{Store: s}).Handler())
	t.Cleanup(func() { server.Close(); s.Close() })
	return server
}

func TestDiscoveryHTTPControlsAndExecutionBoundary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store, e := storage.Open(ctx, ":memory:", false)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	entered := make(chan struct{}, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-r.Context().Done() }))
	defer upstream.Close()
	c := github.New("")
	c.BaseURL = upstream.URL
	d := discovery.New(ctx, store, c, "public unauthenticated", time.Minute)
	server := httptest.NewServer((Server{Store: store, Discovery: d}).Handler())
	defer server.Close()
	for _, path := range []string{"/api/discovery/pause", "/api/discovery/resume"} {
		r := post(t, server, path, "{}", server.URL)
		if r.StatusCode != 200 {
			t.Fatal(path, r.StatusCode)
		}
		r.Body.Close()
	}
	r := post(t, server, "/api/discovery/run", "{}", server.URL)
	if r.StatusCode != 202 {
		t.Fatal(r.StatusCode)
	}
	r.Body.Close()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("scan not started")
	}
	r = post(t, server, "/api/discovery/run", "{}", server.URL)
	if r.StatusCode != 409 {
		t.Fatal("duplicate run allowed", r.StatusCode)
	}
	r.Body.Close()
	r = post(t, server, "/api/discovery/cancel", "{}", server.URL)
	if r.StatusCode != 202 {
		t.Fatal(r.StatusCode)
	}
	r.Body.Close()
	d.Wait()
	runs, e := store.DiscoveryRuns(ctx)
	if e != nil || len(runs) != 1 || runs[0].Status != "CANCELLED" {
		t.Fatal(runs, e)
	}
	cs, _ := store.Contributions(ctx)
	if len(cs) != 0 {
		t.Fatal("discovery started a contribution")
	}
	events, _ := store.Events(ctx, 0, "", 100)
	found := false
	for _, ev := range events {
		if ev.Type == "DiscoveryCancellationRequested" {
			found = true
		}
	}
	if !found {
		t.Fatal("cancellation unaudited")
	}
}
func post(t *testing.T, server *httptest.Server, path, body, origin string) *http.Response {
	t.Helper()
	r, e := http.NewRequest("POST", server.URL+path, strings.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	r.Header.Set("Content-Type", "application/json")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	resp, e := server.Client().Do(r)
	if e != nil {
		t.Fatal(e)
	}
	return resp
}
func TestAPIReadsAndHumanConfigApply(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/health", "/api/overview", "/api/opportunities", "/api/contributions", "/api/config/history", "/api/events", "/api/agents", "/api/usage"} {
		r, e := s.Client().Get(s.URL + path)
		if e != nil {
			t.Fatal(e)
		}
		if r.StatusCode != 200 {
			t.Fatal(path, r.StatusCode)
		}
		r.Body.Close()
	}
	c := config.Default()
	c.Profile.Languages["Rust"] = 0.8
	body, e := json.Marshal(map[string]any{"base_version": 1, "config": c, "reason": "Include Rust"})
	if e != nil {
		t.Fatal(e)
	}
	r := post(t, s, "/api/config/proposals", string(body), s.URL)
	if r.StatusCode != 201 {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("proposal %d %s", r.StatusCode, b)
	}
	var p struct {
		ID string `json:"id"`
	}
	if e = json.NewDecoder(r.Body).Decode(&p); e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	r = post(t, s, "/api/config/proposals/"+p.ID+"/apply", "{}", s.URL)
	if r.StatusCode != 200 {
		t.Fatal(r.StatusCode)
	}
	r.Body.Close()
	r = post(t, s, "/api/opportunities/demo-1/proceed", "{}", s.URL)
	if r.StatusCode != 501 {
		t.Fatal("execution unexpectedly available")
	}
	r.Body.Close()
}
func TestAPIRejectsCrossOriginAndMalformedBodies(t *testing.T) {
	s := testServer(t)
	for _, test := range []struct {
		body, origin string
		status       int
	}{{"{}", "https://evil.example", 403}, {"{} {}", "", 400}, {`{"message":"hello","shell":"rm"}`, "", 400}, {strings.Repeat("x", 70000), "", 400}} {
		r := post(t, s, "/api/operator/chat", test.body, test.origin)
		if r.StatusCode != test.status {
			t.Fatalf("got %d want %d", r.StatusCode, test.status)
		}
		r.Body.Close()
	}
	req, _ := http.NewRequest("GET", s.URL+"/api/overview", nil)
	req.Host = "evil.example"
	r, e := s.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	if r.StatusCode != 403 {
		t.Fatal("nonlocal host accepted")
	}
	r.Body.Close()
}
func TestSSEReplaysAfterCursor(t *testing.T) {
	s := testServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, "GET", s.URL+"/api/events/stream", nil)
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Last-Event-ID", "1")
	r, e := s.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Body.Close()
	if r.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal(r.Header)
	}
	scanner := bufio.NewScanner(r.Body)
	var out bytes.Buffer
	for scanner.Scan() {
		out.WriteString(scanner.Text() + "\n")
		if strings.HasPrefix(scanner.Text(), "data:") {
			break
		}
	}
	if !strings.Contains(out.String(), "id: 2") || !strings.Contains(out.String(), "IssueRanked") {
		t.Fatal(out.String())
	}
}
