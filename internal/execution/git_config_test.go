package execution

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestConfigurationFailureIsNotReportedAsChangedHistory(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	repo, _, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.prepareRuntime(repo); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "config", "--local", "branch.autopilot/issue-1.vscode-merge-base", "origin/main")
	cmd.Dir = repo
	cmd.Env = gitEnv()
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("fixture config: %v %s", e, out)
	}
	err = s.verifyApprovedGit(context.Background(), repo, c.BaseCommit, c.Branch)
	if err == nil || !strings.Contains(err.Error(), "Git configuration changed after approval") || strings.Contains(err.Error(), "changed Git history") {
		t.Fatalf("incorrect failure classification: %v", err)
	}
}

func TestCloneTrackingAndVSCodeSettingsCanBePinned(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	repo, _, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"remote.origin.url": "https://github.com/digitaldrywood/detent.git", "branch.develop.remote": "origin", "branch.develop.merge": "refs/heads/develop", "branch.autopilot/issue-1.vscode-merge-base": "origin/develop"} {
		cmd := exec.Command("git", "config", "--local", key, value)
		cmd.Dir = repo
		cmd.Env = gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture config: %v %s", err, out)
		}
	}
	if err = s.prepareRuntime(repo); err != nil {
		t.Fatal("legitimate clone/editor settings rejected", err)
	}
	if _, err = s.git(context.Background(), repo, "status", "--porcelain"); err != nil {
		t.Fatal("pinned configuration failed verification", err)
	}
}

func TestGitConfigStillRejectsExecutableAndUnrecognizedSettings(t *testing.T) {
	for _, entry := range []string{"filter.evil.clean\narbitrary-command", "include.path\nother-config", "core.sshcommand\narbitrary-command", "branch.develop.mergeoptions\n--unrelated", "branch.develop.remote\n!arbitrary-command", "branch.develop.vscode-merge-base\norigin/develop; arbitrary-command"} {
		if err := validateLocalGitConfig([]byte(entry + "\x00")); err == nil {
			t.Fatalf("unsafe setting accepted: %q", entry)
		}
	}
}
