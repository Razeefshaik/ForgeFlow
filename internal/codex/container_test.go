package codex

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContainerBoundaryAndNetworkArguments(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, network := range []bool{false, true} {
		args, err := containerArguments("local-fixture:1", "fixture", repo, domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}}, network)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(args, "\n")
		for _, required := range []string{"--pull=never", "--read-only", "--cap-drop=ALL", "no-new-privileges", "1000:1000", "target=/workspace/.git,readonly", "--entrypoint\ngo\nlocal-fixture:1\ntest\n./..."} {
			if !strings.Contains(joined, required) {
				t.Fatal("missing boundary", required, joined)
			}
		}
		want := "none"
		if network {
			want = "bridge"
		}
		if !strings.Contains(joined, "--network\n"+want) {
			t.Fatal(joined)
		}
		if strings.Count(joined, "type=bind,") != 2 || strings.Contains(joined, "docker.sock") || strings.Contains(joined, "--privileged") {
			t.Fatal(joined)
		}
	}
	for _, image := range []string{"", "--privileged", "image\nflag"} {
		if _, err := containerArguments(image, "fixture", repo, domain.VerificationCommand{Program: "go"}, false); err == nil {
			t.Fatal("accepted image", image)
		}
	}
	if _, err := containerArguments("image", "fixture", t.TempDir(), domain.VerificationCommand{Program: "go"}, false); err == nil {
		t.Fatal("accepted non-repository")
	}
}

func TestContainerPreflightBudgetIncludesCreationAndNativeProbe(t *testing.T) {
	c := &CLI{VerificationImage: "fixture:1"}
	if c.CheckTimeout() < 45*time.Second+containerCreateTimeout+containerProbeTimeout+containerCleanupTimeout {
		t.Fatal("outer deadline can expire during container creation")
	}
	if (&CLI{}).CheckTimeout() != 45*time.Second {
		t.Fatal("native deadline changed")
	}
}

func TestContainerHomeSurvivesCachePreparation(t *testing.T) {
	repo := t.TempDir()
	if err := prepareLinuxContainerCaches(repo); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(repo, ".forgeflow-runtime", "linux", "home", "installed-tool")
	if err := os.WriteFile(marker, []byte("preserve dependency"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareLinuxContainerCaches(repo); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "preserve dependency" {
		t.Fatal("workspace dependency was lost", err)
	}
	other := t.TempDir()
	if err := prepareLinuxContainerCaches(other); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, ".forgeflow-runtime", "linux", "home", "installed-tool")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("dependency leaked into another workspace", err)
	}
}

func TestContainerCacheRefusesExistingFile(t *testing.T) {
	repo := t.TempDir()
	if err := prepareLinuxContainerCaches(repo); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(repo, ".forgeflow-runtime", "linux", "home")
	if err := os.Remove(home); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(home, []byte("repository data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := prepareLinuxContainerCaches(repo); err == nil {
		t.Fatal("accepted a repository file as container home")
	}
	if data, err := os.ReadFile(home); err != nil || string(data) != "repository data" {
		t.Fatal("repository file was changed", err)
	}
}

func TestRealContainerHomePersistence(t *testing.T) {
	if os.Getenv("FORGEFLOW_TEST_PREFLIGHT") != "1" {
		t.Skip("opt in to real Docker workspace persistence; no model session")
	}
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	c := &CLI{VerificationImage: os.Getenv("FORGEFLOW_TEST_CONTAINER_IMAGE")}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	for _, script := range []string{
		`set -eu; printf 'persisted' > "$HOME/fixture-tool"; printf 'cache' > "$XDG_CACHE_HOME/fixture-cache"`,
		`set -eu; test "$(cat "$HOME/fixture-tool")" = persisted; test "$(cat "$XDG_CACHE_HOME/fixture-cache")" = cache; test "$(git config --get core.checkStat)" = minimal; test "$(git config --get safe.directory)" = /workspace; echo 'fresh container preserved workspace HOME and caches: PASS'`,
	} {
		run, err := c.containerCommand(ctx, repo, domain.VerificationCommand{Program: "sh", Arguments: []string{"-c", script}}, false)
		t.Log(run.Output)
		if err != nil || run.ExitCode != 0 {
			t.Fatal("container workspace persistence failed", err, run.Output)
		}
	}
}

func TestRealContainerPreflight(t *testing.T) {
	if os.Getenv("FORGEFLOW_TEST_PREFLIGHT") != "1" {
		t.Skip("opt in to real Docker preflight")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	c := &CLI{VerificationImage: os.Getenv("FORGEFLOW_TEST_CONTAINER_IMAGE")}
	ctx, cancel := context.WithTimeout(context.Background(), c.CheckTimeout())
	defer cancel()
	if err := c.checkContainer(ctx, dir); err != nil {
		t.Fatal(err)
	}
	t.Log("Real workspace write, root/Git write denial, UID 1000, offline networking and absent Docker socket passed; fixture removed by test cleanup")
}

// Explicit opt-in: real native model editing, real Docker tests, and exact
// fixture/container cleanup. Evidence is retained outside the disposable clone.
func TestRealContainerVerification(t *testing.T) {
	if os.Getenv("FORGEFLOW_TEST_CONTAINER") != "1" {
		t.Skip("opt in to installed Docker and a real Codex editing session")
	}
	base := os.Getenv("FORGEFLOW_TEST_CONTRIBUTIONS_ROOT")
	evidence := os.Getenv("FORGEFLOW_TEST_EVIDENCE")
	if !filepath.IsAbs(base) || !filepath.IsAbs(evidence) {
		t.Fatal("absolute disposable parent and evidence directory required")
	}
	dir, err := os.MkdirTemp(base, ".forgeflow-validation-")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	log, err := os.OpenFile(filepath.Join(evidence, "container-validation.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	record := func(v any) {
		t.Helper()
		if err := json.NewEncoder(log).Encode(v); err != nil {
			t.Fatal(err)
		}
		if err := log.Sync(); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		real, e := filepath.EvalSymlinks(dir)
		if e != nil || !strings.EqualFold(filepath.Dir(real), filepath.Clean(base)) || !strings.HasPrefix(filepath.Base(real), ".forgeflow-validation-") {
			t.Error("cleanup boundary invalid")
			return
		}
		e = os.RemoveAll(real)
		record(map[string]any{"cleanup": real, "error": e, "removed": errors.Is(statError(real), os.ErrNotExist)})
		if e != nil {
			t.Error(e)
		}
	}()
	repo := filepath.Join(dir, "repo")
	if err = os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"go.mod":      "module example.com/containerfixture\n\ngo 1.26\n",
		"sum.go":      "package fixture\nfunc Add(a,b int) int {return a-b}\n",
		"sum_test.go": "package fixture\nimport (\"testing\";\"os/exec\")\nfunc TestAdd(t *testing.T){if Add(2,3)!=5 {t.Fatal(\"addition incorrect\")}}\nfunc TestGitShell(t *testing.T){if b,e:=exec.Command(\"git\",\"-c\",\"alias.probe=!exit 0\",\"probe\").CombinedOutput();e!=nil{t.Fatalf(\"%v: %s\",e,b)}}\n",
		"cancel.cjs":  "const {spawn}=require('node:child_process'); const fs=require('node:fs'); spawn(process.execPath,['-e',\"setTimeout(()=>require('node:fs').writeFileSync('/workspace/escaped-child','bad'),8000)\"],{stdio:'inherit'});fs.writeFileSync('cancel-ready','ready');setInterval(()=>{},1000);\n",
		".gitignore":  "/.forgeflow-runtime/\n/cancel-ready\n",
	}
	for name, body := range files {
		if err = os.WriteFile(filepath.Join(repo, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	init := exec.Command("git", "init", "--quiet", repo)
	if out, e := init.CombinedOutput(); e != nil {
		t.Fatalf("git init: %v %s", e, out)
	}
	cli := Resolve("", "")
	cli.VerificationImage = os.Getenv("FORGEFLOW_TEST_CONTAINER_IMAGE")
	if cli.VerificationImage == "" {
		t.Fatal("explicit image required")
	}
	if err = cli.SetSandboxMode("mxc"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	appRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if err = cli.Check(ctx, repo, appRoot); err != nil {
		record(map[string]any{"check_error": err.Error()})
		t.Fatal(err)
	}
	record(map[string]any{"isolation": "native workspace/sibling/control plus container root/git checks passed"})
	tests := domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-v", "./..."}}
	run, err := cli.Command(ctx, repo, tests, false)
	record(run)
	if err == nil || !strings.Contains(run.Output, "addition incorrect") || !strings.Contains(run.Output, "PASS: TestGitShell") {
		t.Fatal("baseline did not prove assertion failure and working Git shell", err, run.Output)
	}
	result, err := cli.Run(ctx, Request{Directory: repo, Role: "contributor", Prompt: "This is an approved disposable validation repository. Fix Add in sum.go so it adds its two integers. Add a negative-number regression test in sum_test.go. Preserve TestGitShell. Edit only sum.go and sum_test.go. Do not commit, create a PR, or run tests; ForgeFlow will run the required Linux container suite after your edit. Report the changed files truthfully.", Network: false}, func(raw json.RawMessage) error {
		return json.NewEncoder(log).Encode(map[string]any{"agent_event": raw})
	})
	record(map[string]any{"agent_result": result, "error": errorText(err)})
	if err != nil {
		t.Fatal(err)
	}
	run, err = cli.Command(ctx, repo, tests, false)
	record(run)
	if err != nil || run.ExitCode != 0 || !strings.Contains(run.Output, "PASS: TestGitShell") {
		t.Fatal("fixed suite failed", err, run.Output)
	}
	// Verify descendant cancellation after an actual readiness marker, not a
	// guessed startup delay. No model session is needed to test process ownership.
	childCtx, stop := context.WithCancel(ctx)
	done := make(chan domain.TestRun, 1)
	failure := make(chan error, 1)
	go func() {
		r, e := cli.Command(childCtx, repo, domain.VerificationCommand{Program: "node", Arguments: []string{"cancel.cjs"}}, false)
		done <- r
		failure <- e
	}()
	deadline := time.Now().Add(30 * time.Second)
	for statError(filepath.Join(repo, "cancel-ready")) != nil && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	ready := statError(filepath.Join(repo, "cancel-ready")) == nil
	stop()
	run = <-done
	err = <-failure
	record(run)
	if !ready || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation was not exercised", ready, err)
	}
	time.Sleep(9 * time.Second)
	if !errors.Is(statError(filepath.Join(repo, "escaped-child")), os.ErrNotExist) {
		t.Fatal("descendant survived cancellation")
	}
	for _, name := range []string{"sum.go", "sum_test.go"} {
		b, e := os.ReadFile(filepath.Join(repo, name))
		if e != nil {
			t.Fatal(e)
		}
		record(map[string]any{"edited_file": name, "content": string(b)})
	}
	record(map[string]any{"result": "PASS", "cancellation": "owned container and descendant stopped"})
}

func statError(p string) error { _, err := os.Stat(p); return err }
func errorText(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
