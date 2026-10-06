package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"forgeflow/internal/storage"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	calls := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		now := time.Now().UTC().Format(time.RFC3339)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search/issues":
			if strings.Contains(r.URL.Query().Get("q"), "is:pr") {
				fmt.Fprint(w, `{"total_count":2,"items":[{"number":55,"body":"Fixes #12"},{"number":56,"body":"https://github.com/example/backend/issues/123"}]}`)
			} else {
				fmt.Fprintf(w, `{"total_count":1,"items":[{"number":12,"repository_url":"https://api.github.com/repos/example/backend"}]}`)
			}
		case "/repos/example/backend":
			fmt.Fprintf(w, `{"full_name":"example/backend","language":"Go","description":"database backend","stargazers_count":500,"forks_count":4,"default_branch":"main","pushed_at":%q}`, now)
		case "/repos/example/backend/commits":
			fmt.Fprintf(w, `[{"sha":"abc123","commit":{"committer":{"date":%q}}}]`, now)
		case "/repos/example/backend/git/trees/abc123":
			fmt.Fprint(w, `{"tree":[{"path":"CONTRIBUTING.md","type":"blob"},{"path":"AGENTS.md","type":"blob"},{"path":"go.mod","type":"blob"},{"path":".github/workflows/ci.yml","type":"blob"},{"path":"db_test.go","type":"blob"}]}`)
		case "/repos/example/backend/releases":
			fmt.Fprintf(w, `[{"published_at":%q}]`, now)
		case "/repos/example/backend/pulls":
			fmt.Fprintf(w, `[{"merged_at":%q,"author_association":"CONTRIBUTOR"}]`, now)
		case "/repos/example/backend/issues/12":
			fmt.Fprintf(w, `{"number":12,"title":"Fix storage race","body":"Reproduction steps for database bug","state":"open","labels":[{"name":"bug"}],"comments":2,"created_at":%q,"updated_at":%q}`, now, now)
		case "/repos/example/backend/issues/12/comments":
			fmt.Fprint(w, `[{"author_association":"MEMBER"},{"author_association":"NONE"}]`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	return server, calls
}
func setup(t *testing.T, server *httptest.Server, demo bool) (*Service, *storage.Store) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	store, err := storage.Open(ctx, ":memory:", demo)
	if err != nil {
		t.Fatal(err)
	}
	c := github.New("")
	c.BaseURL = server.URL
	s := New(ctx, store, c, "public unauthenticated", 0)
	t.Cleanup(func() { cancel(); s.Cancel(); s.Wait(); store.Close() })
	return s, store
}
func TestScanPersistsEvidenceAndReusesCacheWithoutErasingHistory(t *testing.T) {
	server, calls := fixture(t)
	s, store := setup(t, server, false)
	ctx := context.Background()
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	opps, err := store.Opportunities(ctx)
	if err != nil || len(opps) != 1 {
		t.Fatal(opps, err)
	}
	e := opps[0].Evidence
	if e == nil || e.MaintainerComments == nil || *e.MaintainerComments != 1 || len(e.CompetingPRs) != 1 || e.CompetingPRs[0] != "https://github.com/example/backend/pull/55" || e.Guidelines == nil || !*e.Guidelines {
		t.Fatal("evidence missing or PR URL prefix matched", e)
	}
	runs, _ := store.DiscoveryRuns(ctx)
	if len(runs) != 1 || runs[0].Status != "SUCCEEDED" || runs[0].Accepted != 1 {
		t.Fatal(runs)
	}
	initial := calls.Load()
	if _, err = s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	if calls.Load() != initial {
		t.Fatal("unchanged data repeatedly fetched")
	}
	runs, _ = store.DiscoveryRuns(ctx)
	if len(runs) != 2 || runs[0].Requests != 0 {
		t.Fatal("scan history/cache accounting incorrect", runs)
	}
	events, _ := store.Events(ctx, 0, "", 100)
	if len(events) != 7 {
		t.Fatal("audit history lost", len(events))
	}
	cs, _ := store.Contributions(ctx)
	if len(cs) != 0 {
		t.Fatal("discovery executed contribution")
	}
}
func TestSingleFlightCancellationAndDemoIsolation(t *testing.T) {
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { entered <- struct{}{}; <-r.Context().Done() }))
	defer server.Close()
	s, store := setup(t, server, false)
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("scan did not start")
	}
	if _, err := s.Start(); !errors.Is(err, ErrBusy) {
		t.Fatal("duplicate scan allowed", err)
	}
	s.Cancel()
	s.Wait()
	runs, _ := store.DiscoveryRuns(context.Background())
	if len(runs) != 1 || runs[0].Status != "CANCELLED" {
		t.Fatal(runs)
	}
	demo, _ := setup(t, server, true)
	if _, err := demo.Start(); err == nil {
		t.Fatal("demo performed live discovery")
	}
}
func TestRateLimitProducesFailedScanAndBackoff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(429)
		fmt.Fprint(w, `{"message":"sensitive"}`)
	}))
	defer server.Close()
	s, store := setup(t, server, false)
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	status, err := s.Status(context.Background())
	if err != nil || status.RetryAt == nil || status.LastRun.Status != "FAILED" {
		t.Fatal(status, err)
	}
	if _, err = s.Start(); err == nil {
		t.Fatal("scan ignored backoff")
	}
	b, _ := json.Marshal(status)
	if strings.Contains(string(b), "sensitive") {
		t.Fatal("GitHub error payload leaked")
	}
	opps, _ := store.Opportunities(context.Background())
	if len(opps) != 0 {
		t.Fatal("fabricated opportunities after failure")
	}
}
func TestRepositoryURLsCannotRedirectAdapter(t *testing.T) {
	for _, raw := range []string{"https://evil.example/repos/a/b", "https://api.github.com/repos/a/b/issues/1", "https://api.github.com/repos/../x", "http://api.github.com/repos/a/b"} {
		if RepositoryName(raw) != "" {
			t.Fatal(raw)
		}
	}
	if RepositoryName("https://api.github.com/repos/a/b") != "a/b" {
		t.Fatal("valid repo rejected")
	}
}

func TestSchedulePreferenceSurvivesRestartAndPreservesAudit(t *testing.T) {
	server, _ := fixture(t)
	s, store := setup(t, server, false)
	s.Interval = time.Minute
	if err := s.SetAutomatic(false); err != nil {
		t.Fatal(err)
	}
	restarted := New(context.Background(), store, s.Client, "public unauthenticated", time.Minute)
	if err := restarted.RestoreSchedule(); err != nil {
		t.Fatal(err)
	}
	status, err := restarted.Status(context.Background())
	if err != nil || status.Automatic {
		t.Fatal("pause lost on restart", status, err)
	}
	events, _ := store.Events(context.Background(), 0, "", 100)
	if events[len(events)-1].Type != "DiscoveryScheduleChanged" {
		t.Fatal("schedule change unaudited")
	}
}
func TestRecoveredInterruptedRunCannotBeCompletedTwice(t *testing.T) {
	server, _ := fixture(t)
	_, store := setup(t, server, false)
	ctx := context.Background()
	run := domain.DiscoveryRun{ID: "interrupted", Status: "RUNNING", ConfigVersion: 1, StartedAt: time.Now().UTC(), Warnings: []string{}}
	if err := store.StartDiscovery(ctx, run); err != nil {
		t.Fatal(err)
	}
	if err := store.RecoverDiscovery(ctx); err != nil {
		t.Fatal(err)
	}
	runs, _ := store.DiscoveryRuns(ctx)
	if runs[0].Status != "INTERRUPTED" {
		t.Fatal(runs)
	}
	if err := store.FinishDiscovery(ctx, runs[0], nil); err == nil {
		t.Fatal("completed run rewritten")
	}
}
