package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"forgeflow/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureStore(t *testing.T) *storage.Store {
	t.Helper()
	ctx := context.Background()
	s, err := storage.Open(ctx, filepath.Join(t.TempDir(), "test.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	run := domain.DiscoveryRun{ID: "scan", Status: "RUNNING", ConfigVersion: 1, StartedAt: time.Now().UTC()}
	if err = s.StartDiscovery(ctx, run); err != nil {
		t.Fatal(err)
	}
	end := time.Now().UTC()
	run.Status = "SUCCEEDED"
	run.FinishedAt = &end
	o := domain.Opportunity{ID: "issue", Repository: "example/project", Number: 42, Title: "Fix parser", Evidence: &domain.Evidence{RunID: "scan", ConfigVersion: 1}}
	if err = s.FinishDiscovery(ctx, run, []domain.Opportunity{o}); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestFreshPreviewApprovalAndStaleInputs(t *testing.T) {
	s := fixtureStore(t)
	ctx := context.Background()
	closed := false
	calls := 0
	head := strings.Repeat("a", 40)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/repos/example/project":
			json.NewEncoder(w).Encode(github.Repository{FullName: "example/project", DefaultBranch: "main"})
		case "/repos/example/project/issues/42":
			state := "open"
			if closed {
				state = "closed"
			}
			json.NewEncoder(w).Encode(github.Issue{Number: 42, State: state, Title: "Fix parser", HTMLURL: "https://github.com/example/project/issues/42"})
		case "/repos/example/project/commits":
			json.NewEncoder(w).Encode([]github.Commit{{SHA: head}})
		case "/search/issues":
			json.NewEncoder(w).Encode(github.IssueSearch{})
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	client := github.New("")
	client.BaseURL = upstream.URL
	manager := New(ctx, s, client, t.TempDir())
	manager.Git = "not-executed"
	preview, err := manager.Preview(ctx, "issue")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Proceed(ctx, "issue", preview.Token, false); err == nil {
		t.Fatal("missing human approval accepted")
	}
	if calls != 4 {
		t.Fatal("unapproved request fetched or executed work")
	}
	head = strings.Repeat("b", 40)
	if _, err = manager.Proceed(ctx, "issue", preview.Token, true); !errors.Is(err, ErrChanged) {
		t.Fatalf("stale head accepted: %v", err)
	}
	closed = true
	if _, err = manager.Preview(ctx, "issue"); err == nil {
		t.Fatal("cached open issue accepted")
	}
	cs, _ := s.Contributions(ctx)
	if len(cs) != 0 {
		t.Fatal("rejected approval created contribution")
	}
	entries, _ := os.ReadDir(manager.Root)
	if len(entries) != 0 {
		t.Fatal("preview created workspace")
	}
}
func TestExclusiveWorkspaceAndLinkBoundary(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "contributions", "one")
	if err := createIsolated(root, parent); err != nil {
		t.Fatal(err)
	}
	if err := createIsolated(root, parent); err == nil {
		t.Fatal("existing workspace reused")
	}
	if err := createIsolated(root, filepath.Join(root, "elsewhere", "one")); err == nil {
		t.Fatal("outside path accepted")
	}
	linkedRoot := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(linkedRoot, "contributions")); err != nil {
		t.Log("symlink privilege unavailable; exclusive-path checks completed")
		return
	}
	if err := createIsolated(linkedRoot, filepath.Join(linkedRoot, "contributions", "escape")); err == nil {
		t.Fatal("linked contributions accepted")
	}
	entries, _ := os.ReadDir(target)
	if len(entries) != 0 {
		t.Fatal("wrote through symlink")
	}
}
func TestRealGitCloneBranchAndRecordedFailure(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("Git required for workspace integration test")
	}
	ctx := context.Background()
	s := fixtureStore(t)
	root := t.TempDir()
	manager := New(ctx, s, github.New(""), root)
	cfg, _ := s.CurrentConfig(ctx)
	c := domain.Contribution{ID: "one", OpportunityID: "issue", State: "SELECTED", ConfigVersion: 1}
	a := domain.WorkspaceApproval{Token: "fixture", Config: cfg, Opportunity: domain.Opportunity{ID: "issue"}}
	if err := s.ApproveWorkspace(ctx, c, a); err != nil {
		t.Fatal(err)
	}
	origin := filepath.Join(t.TempDir(), "origin")
	if err := os.Mkdir(origin, 0700); err != nil {
		t.Fatal(err)
	}
	git := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(manager.Git, args...)
		cmd.Dir = dir
		cmd.Env = gitEnvironment("")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Git: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git(origin, "init")
	os.WriteFile(filepath.Join(origin, "README.md"), []byte("fixture"), 0600)
	git(origin, "add", "README.md")
	git(origin, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "fixture")
	sha := git(origin, "rev-parse", "HEAD")
	parent := filepath.Join(root, "contributions", "one")
	if err := createIsolated(root, parent); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(parent, ".autopilot")
	os.Mkdir(meta, 0700)
	repo := filepath.Join(parent, "repo")
	if _, err := manager.command(ctx, "one", meta, parent, []string{"clone", "--no-checkout", "--no-local", "--", origin, repo}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "README.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("clone unexpectedly checked out files")
	}
	if _, err := manager.command(ctx, "one", meta, repo, []string{"checkout", "-b", "autopilot/issue-42", sha, "--"}); err != nil {
		t.Fatal(err)
	}
	if git(repo, "rev-parse", "HEAD") != sha || git(repo, "branch", "--show-current") != "autopilot/issue-42" {
		t.Fatal("wrong commit or branch")
	}
	result, err := manager.command(ctx, "one", meta, repo, []string{"rev-parse", "--verify", "definitely-missing-ref"})
	if err == nil || result.ExitCode == 0 {
		t.Fatal("real Git failure reported success")
	}
	events, _ := s.Events(ctx, 0, "one", 100)
	finished := 0
	for _, e := range events {
		if e.Type == "CommandFinished" {
			finished++
		}
	}
	if finished != 3 {
		t.Fatal("command outcomes not audited")
	}
	log, err := os.ReadFile(filepath.Join(meta, "commands.jsonl"))
	if err != nil || len(strings.Split(strings.TrimSpace(string(log)), "\n")) != 3 {
		t.Fatal("command log incomplete")
	}
}
func TestApprovalIdempotencyAndRestartRecovery(t *testing.T) {
	s := fixtureStore(t)
	ctx := context.Background()
	cfg, _ := s.CurrentConfig(ctx)
	c := domain.Contribution{ID: "one", OpportunityID: "issue", State: "SELECTED", ConfigVersion: 1}
	a := domain.WorkspaceApproval{Token: strings.Repeat("f", 64), Config: cfg, Opportunity: domain.Opportunity{ID: "issue"}}
	if err := s.ApproveWorkspace(ctx, c, a); err != nil {
		t.Fatal(err)
	}
	manager := New(ctx, s, github.New(""), t.TempDir())
	got, err := manager.Proceed(ctx, "issue", a.Token, true)
	if err != nil || got.ID != c.ID {
		t.Fatalf("idempotent approval: %+v %v", got, err)
	}
	if err = s.RecoverWorkspaces(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = s.Contribution(ctx, "one")
	if err != nil || got.State != "BLOCKED" || got.PreviousState != "SELECTED" {
		t.Fatal("interrupted preparation not blocked")
	}
	if _, err = s.ApprovedContribution(ctx, a.Token); err != nil {
		t.Fatal("approval history lost")
	}
}
