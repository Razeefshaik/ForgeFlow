package discovery

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
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
	return fixtureFiles(t, map[string]string{"CONTRIBUTING.md": "Run go test ./... before contributing.\n", "AGENTS.md": "Run required verification: go test ./...\n", "go.mod": "module example/backend\n\ngo 1.27.0\n", ".github/workflows/ci.yml": "name: tests\njobs:\n  test:\n    steps:\n      - run: go test ./...\n"})
}

// ciRuns is a successful four-minute default-branch verification run, the
// evidence a contributable repository provides.
func ciRuns() string {
	end := time.Now().UTC().Add(-time.Hour)
	return fmt.Sprintf(`{"workflow_runs":[{"name":"tests","path":".github/workflows/ci.yml","head_branch":"main","event":"push","status":"completed","conclusion":"success","run_started_at":%q,"updated_at":%q}]}`, end.Add(-4*time.Minute).Format(time.RFC3339), end.Format(time.RFC3339))
}

// quietTimeline has a maintainer comment, an outside comment, a closed pull
// request and an open issue that mention the issue: none is competing work.
const quietTimeline = `[{"event":"commented","author_association":"MEMBER"},{"event":"commented","author_association":"NONE"},{"event":"cross-referenced","source":{"issue":{"number":40,"state":"closed","html_url":"https://github.com/example/backend/pull/40","pull_request":{"url":"x"}}}},{"event":"cross-referenced","source":{"issue":{"number":41,"state":"open","html_url":"https://github.com/example/backend/issues/41"}}}]`

const issueBody = "Reproduction steps for database bug: open two writers on the same key and observe that the second write is lost."

func fixtureFiles(t *testing.T, files map[string]string) (*httptest.Server, *atomic.Int64) {
	return fixtureTimeline(t, files, quietTimeline)
}

func fixtureTimeline(t *testing.T, files map[string]string, timeline string) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	calls := &atomic.Int64{}
	head := strings.Repeat("a", 40)
	blobs := map[string]string{}
	nodes := []map[string]any{}
	for p, body := range files {
		sum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(body), body)))
		sha := fmt.Sprintf("%x", sum)
		blobs[sha] = body
		nodes = append(nodes, map[string]any{"path": p, "type": "blob", "mode": "100644", "sha": sha, "size": len(body)})
	}
	nodes = append(nodes, map[string]any{"path": "db_test.go", "type": "blob"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		now := time.Now().UTC().Format(time.RFC3339)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search/repositories":
			if q := r.URL.Query(); q.Get("sort") != "stars" || !strings.Contains(q.Get("q"), "-issues:>0") || !strings.Contains(q.Get("q"), "pushed:>=") {
				t.Errorf("repository search is not the invited-issues search by stars: %s", r.URL.RawQuery)
			}
			fmt.Fprintf(w, `{"total_count":1,"items":[{"full_name":"example/backend","language":"Go","description":"database backend","stargazers_count":500,"forks_count":4,"default_branch":"main","pushed_at":%q}]}`, now)
		case "/repos/example/backend/commits":
			fmt.Fprintf(w, `[{"sha":%q,"commit":{"committer":{"date":%q}}}]`, head, now)
		case "/repos/example/backend/git/trees/" + head:
			json.NewEncoder(w).Encode(map[string]any{"tree": nodes})
		case "/repos/example/backend/actions/runs":
			fmt.Fprint(w, ciRuns())
		case "/repos/example/backend/releases":
			fmt.Fprintf(w, `[{"published_at":%q}]`, now)
		case "/repos/example/backend/pulls":
			fmt.Fprintf(w, `[{"merged_at":%q,"author_association":"CONTRIBUTOR"}]`, now)
		case "/repos/example/backend/issues":
			if r.URL.Query().Get("assignee") != "none" || r.URL.Query().Get("state") != "open" {
				t.Errorf("issue list is not limited to open unassigned issues: %s", r.URL.RawQuery)
			}
			fmt.Fprintf(w, `[{"number":12,"title":"Fix storage race","body":%q,"state":"open","labels":[{"name":"kind/bug"}],"comments":2,"created_at":%q,"updated_at":%q},{"number":13,"title":"A pull request","body":%q,"state":"open","labels":[{"name":"bug"}],"pull_request":{"url":"x"}}]`, issueBody, now, now, issueBody)
		case "/repos/example/backend/issues/12/timeline":
			fmt.Fprint(w, timeline)
		default:
			if strings.HasPrefix(r.URL.Path, "/repos/example/backend/git/blobs/") {
				sha := strings.TrimPrefix(r.URL.Path, "/repos/example/backend/git/blobs/")
				body, ok := blobs[sha]
				if !ok {
					http.NotFound(w, r)
					return
				}
				json.NewEncoder(w).Encode(github.Blob{SHA: sha, Size: len(body), Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte(body))})
				return
			}
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
	// PRs referencing #123 or issues/123 must not count as competition for #12.
	if e == nil || e.MaintainerComments == nil || *e.MaintainerComments != 1 || len(e.CompetingPRs) != 0 || !e.CompetitionChecked || e.Guidelines == nil || !*e.Guidelines {
		t.Fatal("evidence missing or PR number/URL prefix matched", e)
	}
	if e.Prerequisites == nil || e.Prerequisites.CI == nil || e.Prerequisites.CI.MedianSeconds != 240 {
		t.Fatal("CI timing evidence was not recorded", e.Prerequisites)
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
	if len(events) != 9 {
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

func TestAbandonedContributionDoesNotDriveNewRepositoryScreening(t *testing.T) {
	server, _ := fixture(t)
	service, store := setup(t, server, false)
	ctx := context.Background()
	cfg, err := store.CurrentConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	initial := domain.DiscoveryRun{ID: "initial", Status: "RUNNING", ConfigVersion: cfg.Version, StartedAt: time.Now().UTC()}
	if err = store.StartDiscovery(ctx, initial); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC()
	initial.Status, initial.FinishedAt = "SUCCEEDED", &end
	o := domain.Opportunity{ID: "old-issue", Repository: "example/backend", Evidence: &domain.Evidence{RunID: initial.ID, ConfigVersion: cfg.Version}}
	if err = store.FinishDiscovery(ctx, initial, []domain.Opportunity{o}); err != nil {
		t.Fatal(err)
	}
	c := domain.Contribution{ID: "old", OpportunityID: o.ID, Repository: o.Repository, State: "SELECTED"}
	if err = store.ApproveWorkspace(ctx, c, domain.WorkspaceApproval{Config: cfg}); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveExecution(ctx, domain.ExecutionRecord{ContributionID: c.ID, Status: "ABANDONED", Incident: &domain.ExecutionIncident{Kind: "environment_blocked", Summary: "Historical failure"}}, "fixture", "old incident retained"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Start(); err != nil {
		t.Fatal(err)
	}
	service.Wait()
	runs, err := store.DiscoveryRuns(ctx)
	if err != nil || runs[0].Accepted != 1 || len(runs[0].ExcludedRepositories) != 0 {
		t.Fatal("old contribution determined eligibility", runs, err)
	}
	exclusions, err := store.RepositoryExclusions(ctx)
	if err != nil || len(exclusions) != 0 {
		t.Fatal(exclusions, err)
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

// An open PR already solving the issue, or an assignee, means a ForgeFlow PR
// would race existing work; such issues are not offered.
func TestCompetingOrAssignedIssuesAreNotOffered(t *testing.T) {
	server, _ := fixtureTimeline(t, map[string]string{"CONTRIBUTING.md": "Run go test ./... before contributing.\n", "go.mod": "module example/backend\n\ngo 1.27.0\n"}, `[{"event":"cross-referenced","source":{"issue":{"number":55,"state":"open","html_url":"https://github.com/example/backend/pull/55","pull_request":{"url":"x"}}}}]`)
	s, store := setup(t, server, false)
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	opps, err := store.Opportunities(context.Background())
	if err != nil || len(opps) != 0 {
		t.Fatal("issue with a competing open PR was offered", opps, err)
	}
	// Nothing to offer is a completed scan, not a failed one.
	runs, err := store.DiscoveryRuns(context.Background())
	if err != nil || runs[0].Status != "SUCCEEDED" {
		t.Fatal("a scan that found nothing to offer was not reported as succeeded", runs, err)
	}
}
