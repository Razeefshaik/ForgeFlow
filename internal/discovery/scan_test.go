package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func labelled(names ...string) github.Issue {
	i := github.Issue{Number: 1, State: "open", Body: issueBody, CreatedAt: time.Now().Add(-48 * time.Hour)}
	for _, n := range names {
		i.Labels = append(i.Labels, github.Label{Name: n})
	}
	return i
}

func TestRepositoryLabelsMatchPreferredLabels(t *testing.T) {
	for label, want := range map[string]bool{"bug": true, "kind/bug": true, "type: bug": true, "Bug 🐛": true, "debug": false, "bugfix-needed": false} {
		if labelMatches(label, "bug") != want {
			t.Errorf("label %q against bug: want %v", label, want)
		}
	}
	for label, want := range map[string]bool{"good first issue": true, "good-first-issue": true, "Good First Issue": true, "good first": false, "first issue": false} {
		if labelMatches(label, "good first issue") != want {
			t.Errorf("label %q against good first issue: want %v", label, want)
		}
	}
}

func TestOnlyActionableIssuesAreOffered(t *testing.T) {
	cfg := domain.Config{}
	cfg.Issues.PreferredLabels = []string{"good first issue", "help wanted", "bug", "enhancement"}
	cfg.Issues.Difficulty = []string{"easy", "medium"}
	cfg.Profile.Exclude = []string{"documentation-only"}
	old := func(i github.Issue) github.Issue { i.CreatedAt = time.Now().AddDate(-3, 0, 0); return i }
	short := labelled("bug")
	short.Body = "It crashes."
	assigned := labelled("bug")
	assigned.Assignees = []github.User{{Login: "someone"}}
	pull := labelled("bug")
	pull.PullRequest = json.RawMessage(`{"url":"x"}`)
	locked := labelled("bug")
	locked.Locked = true
	for name, tc := range map[string]struct {
		issue  github.Issue
		ok     bool
		reason string
	}{
		"recent bug":                 {labelled("kind/bug"), true, ""},
		"good first issue":           {labelled("good-first-issue"), true, ""},
		"old invited issue":          {func() github.Issue { i := labelled("help wanted", "enhancement"); i.CreatedAt = time.Now().AddDate(-2, 0, 0); return i }(), true, ""},
		"ancient invited issue":      {old(old(labelled("good first issue"))), false, "more than three years"},
		"standing thread":            {func() github.Issue { i := labelled("help wanted"); i.Comments = 120; return i }(), false, "long discussion"},
		"pending triage":             {labelled("bug", "pending"), false, "not ready"},
		"old uninvited bug":          {old(labelled("bug")), false, "older than a year"},
		"no preferred label":         {labelled("refactor"), false, ""},
		"pull request":               {pull, false, ""},
		"assigned":                   {assigned, false, "assigned"},
		"locked":                     {locked, false, "locked"},
		"needs triage":               {labelled("bug", "needs-triage"), false, "not ready"},
		"question":                   {labelled("bug", "question"), false, "not ready"},
		"blocked":                    {labelled("good first issue", "status: blocked"), false, "not ready"},
		"documentation":              {labelled("documentation", "good first issue"), false, "excluded by profile"},
		"hard":                       {labelled("bug", "hard"), false, "difficulty"},
		"no description to plan on":  {short, false, "too little description"},
		"debug is not the bug label": {labelled("debug"), false, ""},
	} {
		ok, reason := offerable(tc.issue, cfg)
		if ok != tc.ok || !strings.Contains(reason, tc.reason) || tc.reason == "" && reason != "" {
			t.Errorf("%s: offerable=%v reason=%q", name, ok, reason)
		}
	}
}

func TestListsAndTutorialsAreNotSoftware(t *testing.T) {
	for name, want := range map[string]bool{"avelino/awesome-go": true, "EbookFoundation/free-programming-books": true, "x/python-tutorial": true, "jwasham/coding-interview-university": true, "helm/helm": false, "scikit-learn/scikit-learn": false, "pallets/flask": false, "x/machine-learning-lib": false} {
		if notSoftware(github.Repository{FullName: name}) != want {
			t.Errorf("%s: want notSoftware=%v", name, want)
		}
	}
	if !notSoftware(github.Repository{FullName: "x/resources", Topics: []string{"awesome-list"}}) {
		t.Error("a repository with the awesome-list topic was treated as software")
	}
}

// bigFixture serves three repositories: one whose tree exceeds the read limit,
// one whose tree GitHub truncates, and one that 404s. None is a failed scan.
func bigFixture(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	screened := 0
	head := strings.Repeat("a", 40)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/search/repositories":
			if strings.Contains(r.URL.Query().Get("q"), "help-wanted-issues") {
				fmt.Fprint(w, `{"total_count":0,"items":[]}`)
				return
			}
			items := []github.Repository{}
			for _, n := range []string{"big/huge", "big/truncated", "big/gone"} {
				items = append(items, github.Repository{FullName: n, Language: "Go", DefaultBranch: "main", PushedAt: time.Now().UTC()})
			}
			json.NewEncoder(w).Encode(github.RepoSearch{Total: 3, Items: items})
		case strings.HasSuffix(r.URL.Path, "/commits"):
			screened++
			if strings.Contains(r.URL.Path, "big/gone") {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode([]github.Commit{{SHA: head}})
		case strings.Contains(r.URL.Path, "/big/huge/git/trees/"):
			fmt.Fprint(w, `{"tree":[{"path":"`+strings.Repeat("x", 4<<20)+`","type":"blob"}]}`)
		case strings.Contains(r.URL.Path, "/big/truncated/git/trees/"):
			fmt.Fprint(w, `{"truncated":true,"tree":[{"path":"go.mod","type":"blob"}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &screened
}

func TestOversizedRepositoriesAreExcludedNotScanFailures(t *testing.T) {
	server, screened := bigFixture(t)
	s, store := setup(t, server, false)
	onlyGo(t, store)
	run := completedScan(t, s) // requires SUCCEEDED
	kinds := map[string]string{}
	for _, x := range run.ExcludedRepositories {
		kinds[x.Repository] = x.Status + " " + x.Kind
	}
	if kinds["big/huge"] != "excluded repository_too_large" || kinds["big/truncated"] != "excluded repository_too_large" {
		t.Fatalf("oversized repositories were not excluded for their size: %v", kinds)
	}
	if kinds["big/gone"] != "needs_assessment needs_assessment" {
		t.Fatalf("an unreadable repository was not withheld: %v", kinds)
	}
	// A decision made under the same rules stands: excluded repositories are
	// not screened again, and the withheld one is not either for a day.
	before := *screened
	next := completedScan(t, s)
	if *screened != before || len(next.RepositoryAssessments) != 0 {
		t.Fatalf("known exclusions were screened again: %d reads, %+v", *screened-before, next.RepositoryAssessments)
	}
	found := false
	for _, w := range next.Warnings {
		found = found || strings.Contains(w, "not screened again")
	}
	if !found {
		t.Fatalf("skipping known exclusions was not disclosed: %v", next.Warnings)
	}
	if opps, err := store.Opportunities(context.Background()); err != nil || len(opps) != 0 {
		t.Fatal(opps, err)
	}
}
