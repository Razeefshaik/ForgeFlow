package discovery

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"forgeflow/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func progressFixture(t *testing.T, rejected int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/search/issues" {
			if strings.Contains(r.URL.Query().Get("q"), "is:pr") {
				json.NewEncoder(w).Encode(github.IssueSearch{})
				return
			}
			page, err := strconv.Atoi(r.URL.Query().Get("page"))
			if err != nil || page < 1 || page > 40 || r.URL.Query().Get("per_page") != "25" {
				t.Errorf("unbounded/invalid search %s", r.URL.RawQuery)
				http.Error(w, "invalid page", 400)
				return
			}
			items := []github.Issue{}
			if page <= 2 {
				for i := 1; i <= 25; i++ {
					n := (page-1)*25 + i
					items = append(items, github.Issue{Number: n, RepositoryURL: fmt.Sprintf("https://api.github.com/repos/fresh/repo%d", n)})
				}
			}
			json.NewEncoder(w).Encode(github.IssueSearch{Total: 50, Items: items})
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 {
			http.NotFound(w, r)
			return
		}
		n, _ := strconv.Atoi(strings.TrimPrefix(parts[2], "repo"))
		repo := "fresh/" + parts[2]
		body := "Run go test ./... before contributing."
		if n <= rejected {
			body = "Contributors must supply an Ed25519 signed receipt."
		}
		sum := sha1.Sum([]byte(fmt.Sprintf("blob %d\x00%s", len(body), body)))
		blobSHA := fmt.Sprintf("%x", sum)
		if len(parts) == 3 {
			json.NewEncoder(w).Encode(github.Repository{FullName: repo, Language: "Go", DefaultBranch: "main", PushedAt: time.Now().UTC()})
			return
		}
		switch parts[3] {
		case "commits":
			json.NewEncoder(w).Encode([]github.Commit{{SHA: strings.Repeat("a", 40)}})
		case "git":
			if parts[4] == "trees" {
				json.NewEncoder(w).Encode(map[string]any{"tree": []map[string]any{{"path": "CONTRIBUTING.md", "type": "blob", "mode": "100644", "sha": blobSHA, "size": len(body)}}})
			} else {
				json.NewEncoder(w).Encode(github.Blob{SHA: blobSHA, Size: len(body), Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte(body))})
			}
		case "issues":
			json.NewEncoder(w).Encode(github.Issue{Number: n, Title: "Fix backend behavior", State: "open", Body: "A reproduction for a backend defect", Labels: []github.Label{{Name: "bug"}}})
		case "releases", "pulls":
			fmt.Fprint(w, "[]")
		default:
			t.Errorf("unexpected fixture request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func onlyGo(t *testing.T, store *storage.Store) {
	t.Helper()
	ctx := context.Background()
	cfg, err := store.CurrentConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Config.Profile.Languages = map[string]float64{"Go": 1}
	p, err := store.Propose(ctx, cfg.Version, cfg.Config, "Fixture search profile")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Apply(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
}

func completedScan(t *testing.T, s *Service) domain.DiscoveryRun {
	t.Helper()
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	runs, err := s.Store.DiscoveryRuns(context.Background())
	if err != nil || len(runs) == 0 {
		t.Fatal(runs, err)
	}
	if runs[0].Status != "SUCCEEDED" {
		t.Fatal(runs[0])
	}
	return runs[0]
}

func TestSearchContinuesPastPartialPageAndPersistsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	server := progressFixture(t, 0)
	path := filepath.Join(t.TempDir(), "discovery.db")
	store, err := storage.Open(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { store.Close() }()
	onlyGo(t, store)
	client := github.New("")
	client.BaseURL = server.URL
	s := New(ctx, store, client, "public", 0)
	first := completedScan(t, s)
	if first.Accepted != 2 || first.NextSearchCursors["Go"].Page != 1 || first.NextSearchCursors["Go"].Offset != 2 || first.NewOpportunities == nil || *first.NewOpportunities != 2 {
		t.Fatal(first)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = storage.Open(ctx, path, false)
	if err != nil {
		t.Fatal(err)
	}
	s = New(ctx, store, client, "public", 0)
	second := completedScan(t, s)
	if second.SearchCursors["Go"].Offset != 2 || second.NextSearchCursors["Go"].Offset != 4 || *second.NewOpportunities != 2 || *second.RefreshedOpportunities != 0 {
		t.Fatal("restart did not continue discovery", second)
	}
	for i := 0; i < 11; i++ {
		completedScan(t, s)
	}
	pageTwo := completedScan(t, s)
	if pageTwo.SearchCursors["Go"].Page != 2 || *pageTwo.NewOpportunities != 2 {
		t.Fatal("later pages not searched", pageTwo)
	}
	opps, err := store.Opportunities(ctx)
	if err != nil || len(opps) != 27 {
		t.Fatal("new repositories skipped or old observations erased", len(opps), err)
	}
	cs, err := store.Contributions(ctx)
	if err != nil || len(cs) != 0 {
		t.Fatal("discovery started contribution execution", cs, err)
	}
}

func TestRejectedRepositoriesDoNotConsumeEligibleTarget(t *testing.T) {
	server := progressFixture(t, 4)
	s, store := setup(t, server, false)
	onlyGo(t, store)
	run := completedScan(t, s)
	if run.Accepted != 2 || len(run.ExcludedRepositories) != 4 || len(run.RepositoryAssessments) != 6 || run.NextSearchCursors["Go"].Offset != 6 {
		t.Fatal("rejected repositories exhausted opportunity target", run)
	}
}

func TestFailedSearchKeepsItsCursorForExplicitLaterAttempt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "60"); w.WriteHeader(429) }))
	defer server.Close()
	s, store := setup(t, server, false)
	onlyGo(t, store)
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	runs, err := store.DiscoveryRuns(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c, err := store.DiscoveryCursors(context.Background(), runs[0].SearchScope)
	if err != nil || c["Go"].Page != 1 || c["Go"].Offset != 0 {
		t.Fatal("failed search advanced progress", c, err)
	}
	if _, err = s.Start(); err == nil {
		t.Fatal("rate-limited search retried without backoff")
	}
}

func TestCancellationKeepsUnfinishedRepositoryPosition(t *testing.T) {
	fixture := progressFixture(t, 0)
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/fresh/repo1" {
			entered <- struct{}{}
			<-r.Context().Done()
			return
		}
		fixture.Config.Handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	s, store := setup(t, server, false)
	onlyGo(t, store)
	if _, err := s.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("repository request did not start")
	}
	s.Cancel()
	s.Wait()
	runs, err := store.DiscoveryRuns(context.Background())
	if err != nil || runs[0].Status != "CANCELLED" {
		t.Fatal(runs, err)
	}
	c, err := store.DiscoveryCursors(context.Background(), runs[0].SearchScope)
	if err != nil || c["Go"].Page != 1 || c["Go"].Offset != 0 {
		t.Fatal("cancelled repository was skipped", c, err)
	}
}

func TestSearchProgressScopeAndExhaustion(t *testing.T) {
	if c := nextPosition(40, 24, 2000, 25, false); c.Page != 1 || c.Offset != 0 {
		t.Fatal(c)
	}
	if c := nextPosition(2, 4, 30, 5, false); c.Page != 1 || c.Offset != 0 {
		t.Fatal(c)
	}
	if c := nextPosition(1, 24, 50, 25, false); c.Page != 2 || c.Offset != 0 {
		t.Fatal(c)
	}
	s := &Service{}
	cfg := domain.Config{}
	cfg.Profile.Languages = map[string]float64{"Go": 1}
	initial := s.searchScope(cfg)
	cfg.Codex.MaxFixIterations = 20
	if s.searchScope(cfg) != initial {
		t.Fatal("effort/retry setting changed exploration scope")
	}
	cfg.Issues.PreferredLabels = []string{"bug"}
	if s.searchScope(cfg) == initial {
		t.Fatal("changed search filters reused old progress")
	}
}
