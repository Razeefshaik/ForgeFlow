package workspace

import (
	"context"
	"encoding/json"
	"forgeflow/internal/github"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewPrerequisitesRecheckedBeforeContributionApproval(t *testing.T) {
	store := fixtureStore(t)
	ctx := context.Background()
	head := strings.Repeat("a", 40)
	body := "Run go test ./... before contributing."
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/example/project":
			json.NewEncoder(w).Encode(github.Repository{FullName: "example/project", DefaultBranch: "main"})
		case "/repos/example/project/issues/42":
			json.NewEncoder(w).Encode(github.Issue{Number: 42, State: "open", HTMLURL: "https://github.com/example/project/issues/42", Title: "Fix parser"})
		case "/repos/example/project/commits":
			json.NewEncoder(w).Encode([]github.Commit{{SHA: head}})
		case "/search/issues":
			json.NewEncoder(w).Encode(github.IssueSearch{})
		default:
			if !servePrerequisite(t, w, r, "example/project", head, body) {
				http.NotFound(w, r)
			}
		}
	}))
	defer upstream.Close()
	client := github.New("")
	client.BaseURL = upstream.URL
	manager := New(ctx, store, client, t.TempDir())
	manager.Git = "not-executed"
	preview, err := manager.Preview(ctx, "issue")
	if err != nil {
		t.Fatal(err)
	}
	head = strings.Repeat("b", 40)
	body = "Contributors must supply an Ed25519 signed receipt."
	if _, err = manager.Proceed(ctx, "issue", preview.Token, true); err == nil || !strings.Contains(err.Error(), "repository excluded") {
		t.Fatal("changed requirements bypassed screening", err)
	}
	cs, err := store.Contributions(ctx)
	if err != nil || len(cs) != 0 {
		t.Fatal("blocked selection created work", cs, err)
	}
	entries, err := os.ReadDir(manager.Root)
	if err != nil || len(entries) != 0 {
		t.Fatal("screening cloned or edited a repository", entries, err)
	}
	excluded, err := store.RepositoryExclusions(ctx)
	if err != nil || len(excluded) != 1 || excluded[0].HeadSHA != head {
		t.Fatal(excluded, err)
	}
	head = strings.Repeat("c", 40)
	body = "Run go test ./... before contributing."
	if _, err = manager.Preview(ctx, "issue"); err != nil {
		t.Fatal("repaired repository remained blacklisted", err)
	}
	excluded, err = store.RepositoryExclusions(ctx)
	if err != nil || len(excluded) != 0 {
		t.Fatal(excluded, err)
	}
}
