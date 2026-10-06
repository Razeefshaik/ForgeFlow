package workspace

import (
	"context"
	"encoding/json"
	"forgeflow/internal/github"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The subprocess redirects the one fixed GitHub clone URL to our local fixture,
// then runs real Git. Production URL construction and the public API are unchanged.
func init() {
	if os.Getenv("FORGEFLOW_GIT_FIXTURE_HELPER") != "1" {
		return
	}
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "https://github.com/example/project.git" {
			args[i] = os.Getenv("FORGEFLOW_GIT_FIXTURE_ORIGIN")
		}
	}
	args = append([]string{"-c", "protocol.file.allow=always"}, args...)
	cmd := exec.Command(os.Getenv("FORGEFLOW_GIT_FIXTURE_EXECUTABLE"), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if cmd.ProcessState != nil {
			os.Exit(cmd.ProcessState.ExitCode())
		}
		os.Exit(127)
	}
	os.Exit(0)
}
func TestApprovedPreparationEndToEndWithRealGit(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	origin := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(realGit, args...)
		cmd.Dir = origin
		cmd.Env = gitEnvironment("")
		b, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("fixture Git: %v %s", e, b)
		}
		return strings.TrimSpace(string(b))
	}
	run("init")
	if err = os.WriteFile(filepath.Join(origin, "AGENTS.md"), []byte("Run go test ./... before review."), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "AGENTS.md")
	run("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "initial")
	sha := run("rev-parse", "HEAD")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/example/project":
			json.NewEncoder(w).Encode(github.Repository{FullName: "example/project", DefaultBranch: "main"})
		case "/repos/example/project/issues/42":
			json.NewEncoder(w).Encode(github.Issue{Number: 42, State: "open", Title: "Fix parser", HTMLURL: "https://github.com/example/project/issues/42"})
		case "/repos/example/project/commits":
			json.NewEncoder(w).Encode([]github.Commit{{SHA: sha}})
		case "/search/issues":
			json.NewEncoder(w).Encode(github.IssueSearch{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	s := fixtureStore(t)
	ctx := context.Background()
	root := t.TempDir()
	client := github.New("")
	client.BaseURL = upstream.URL
	manager := New(ctx, s, client, root)
	// Test child remains a native executable on Windows; no shell wrapper is used.
	t.Setenv("FORGEFLOW_GIT_FIXTURE_HELPER", "1")
	t.Setenv("FORGEFLOW_GIT_FIXTURE_ORIGIN", origin)
	t.Setenv("FORGEFLOW_GIT_FIXTURE_EXECUTABLE", realGit)
	manager.Git = os.Args[0]
	preview, err := manager.Preview(ctx, "issue")
	if err != nil {
		t.Fatal(err)
	}
	c, err := manager.Proceed(ctx, "issue", preview.Token, true)
	if err != nil {
		t.Fatal(err)
	}
	manager.Wait()
	got, err := s.Contribution(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "BLOCKED" || got.PreviousState != "PLANNING" {
		events, _ := s.Events(ctx, 0, c.ID, 100)
		t.Fatalf("preparation: %+v events=%+v", got, events)
	}
	repo := filepath.Join(root, filepath.FromSlash(c.Workspace))
	meta := filepath.Join(filepath.Dir(repo), ".autopilot")
	for _, name := range []string{"approval.json", "metadata.json", "issue.json", "repository.json", "configuration-snapshot.json", "inspection.json", "commands.jsonl", "events.jsonl"} {
		if _, err = os.Stat(filepath.Join(meta, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil || !strings.Contains(string(b), "go test") {
		t.Fatal("source or instruction missing")
	}
	cmd := exec.Command(realGit, "rev-parse", "HEAD")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) != sha {
		t.Fatal("approved SHA not checked out")
	}
	duplicate, err := manager.Proceed(ctx, "issue", preview.Token, true)
	if err != nil || duplicate.ID != c.ID {
		t.Fatal("repeat approval created another workspace")
	}
	entries, _ := os.ReadDir(filepath.Join(root, "contributions"))
	if len(entries) != 1 {
		t.Fatal("unexpected extra workspace")
	}
}
