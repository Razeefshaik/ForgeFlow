package codex

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func environmentFixturePlan() domain.EnvironmentPlan {
	return domain.EnvironmentPlan{Summary: "Use repository toolchains", BaseImage: "fixture:1", Files: []string{"requirements.txt"}, Checks: []domain.VerificationCommand{{Program: "python3", Arguments: []string{"--version"}}}}
}

func TestEnvironmentInputsAndRecipeBoundaries(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("example==1\n"), 0600)
	c := &CLI{}
	plan := environmentFixturePlan()
	one, err := c.EnvironmentFingerprint(dir, plan)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("example==2\n"), 0600)
	two, err := c.EnvironmentFingerprint(dir, plan)
	if err != nil || one == two {
		t.Fatal("dependency change did not invalidate recipe", err)
	}
	for _, name := range []string{"../outside", "/outside", "C:/outside", ".git/config", ".env", "nested/.npmrc", "x/../requirements.txt", "requirements.txt\nFROM bad"} {
		bad := plan
		bad.Files = []string{name}
		if _, err = c.EnvironmentFingerprint(dir, bad); err == nil {
			t.Fatal("accepted input", name)
		}
	}
	bad := plan
	bad.BaseImage = "fixture\nRUN evil"
	if ValidateEnvironmentPlan(bad) == nil {
		t.Fatal("accepted image injection")
	}
	bad = plan
	bad.Environment = []domain.EnvironmentVariable{{Name: "DOCKER_HOST", Value: "unix:///var/run/docker.sock"}}
	if ValidateEnvironmentPlan(bad) == nil {
		t.Fatal("accepted host capability")
	}
	plan.ImageCommands = []domain.VerificationCommand{{Program: "sh", Arguments: []string{"-c", "echo 'quoted'\nFROM malicious"}}}
	recipe, err := environmentDockerfile("sha256:"+strings.Repeat("a", 64), plan)
	if err != nil || strings.Count(recipe, "\nFROM ") != 0 || !strings.Contains(recipe, `\nFROM malicious`) {
		t.Fatal("argument escaped Dockerfile instruction", recipe, err)
	}
	if !strings.Contains(recipe, "USER 1000:1000") || strings.Contains(recipe, "--mount") {
		t.Fatal(recipe)
	}
}

func TestEnvironmentContainerCommandsSupportMultipleLanguages(t *testing.T) {
	for _, command := range []domain.VerificationCommand{{Program: "cargo", Arguments: []string{"test", "--locked"}}, {Program: "dotnet", Arguments: []string{"test"}}, {Program: "bundle", Arguments: []string{"exec", "rspec"}}, {Program: "cmake", Arguments: []string{"--build", "build"}}, {Program: "./custom-language-test", Arguments: []string{"--verify"}}} {
		if err := ValidateContainerCommand(command); err != nil {
			t.Fatal(command, err)
		}
	}
	if ValidateContainerCommand(domain.VerificationCommand{Program: "cargo test"}) == nil {
		t.Fatal("accepted combined program/args")
	}
}

func TestEnvironmentSelectionsRemainWorkspaceSpecific(t *testing.T) {
	c := &CLI{VerificationImage: "base:1", environments: map[string]domain.ExecutionEnvironment{"one": {Status: "ready", Image: "python:3"}, "two": {Status: "ready", Image: "node:22"}}}
	if c.workspaceImage("one") != "python:3" || c.workspaceImage("two") != "node:22" || c.workspaceImage("three") != "base:1" {
		t.Fatal("image selection crossed workspace")
	}
}

// Real Docker proof without paid model sessions. Uses a disposable clone and a
// real downloaded Node library plus Python verification; never touches live contributions.
func TestRealGenericEnvironment(t *testing.T) {
	if os.Getenv("FORGEFLOW_TEST_ENVIRONMENT") != "1" {
		t.Skip("opt in to real Docker provisioning and dependency downloads")
	}
	base := os.Getenv("FORGEFLOW_TEST_CONTAINER_IMAGE")
	if base == "" {
		t.Fatal("explicit image required")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"requirements.txt": "# Python standard library unittest; Python >=3.11\n",
		"package.json":     `{"private":true,"engines":{"node":">=22"}}`,
		"test_sum.py":      "import unittest\nclass Addition(unittest.TestCase):\n def test_add(self): self.assertEqual(2+3,5)\nif __name__ == '__main__': unittest.main()\n",
		"test-node.cjs":    "const assert=require('node:assert/strict'); const semver=require('/opt/forgeflow-libs/node_modules/semver'); assert(semver.satisfies(process.version,'>=22'));assert.equal(semver.valid('1.2.3'),'1.2.3');console.log('node library test PASS');\n",
	}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	c := &CLI{VerificationImage: base}
	plan := domain.EnvironmentPlan{Summary: "Node and Python with a pinned Node library", BaseImage: base, Files: []string{"package.json", "requirements.txt"}, ImageCommands: []domain.VerificationCommand{{Program: "npm", Arguments: []string{"install", "--prefix", "/opt/forgeflow-libs", "--ignore-scripts", "--no-audit", "--no-fund", "semver@7.7.2"}}}, Checks: []domain.VerificationCommand{
		{Program: "node", Arguments: []string{"-e", `if(!require('/opt/forgeflow-libs/node_modules/semver').satisfies(process.version,'>=22'))process.exit(1)`}},
		{Program: "python3", Arguments: []string{"-c", "import sys; assert sys.version_info >= (3,11)"}},
	}}
	evidence := os.Getenv("FORGEFLOW_TEST_EVIDENCE")
	if evidence == "" {
		t.Fatal("evidence directory required")
	}
	if err := os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	log, err := os.OpenFile(filepath.Join(evidence, "generic-environment.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	record := func(value any) {
		t.Helper()
		if err := json.NewEncoder(log).Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	report, err := c.PrepareEnvironment(ctx, dir, true, plan)
	record(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []domain.VerificationCommand{{Program: "node", Arguments: []string{"test-node.cjs"}}, {Program: "python3", Arguments: []string{"test_sum.py"}}} {
		rec, err := c.Command(ctx, dir, command, false)
		record(rec)
		if err != nil || rec.ExitCode != 0 {
			t.Fatal(err, rec.Output)
		}
	}
	// Identical inputs reuse the image, but actual prerequisite checks still run.
	cached, err := c.PrepareEnvironment(ctx, dir, false, plan)
	record(cached)
	if err != nil || cached.Image != report.Image {
		t.Fatal("offline reuse failed", err)
	}
	// A changed dependency specification invalidates the image. The new proof
	// fails before testing, instead of silently accepting the old runtime.
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"private":true,"engines":{"node":">=999"}}`), 0600)
	plan.BaseImage = report.Image
	plan.ImageCommands = nil
	plan.Checks[0].Arguments = []string{"-e", `if(!require('/opt/forgeflow-libs/node_modules/semver').satisfies(process.version,require('./package.json').engines.node))process.exit(1)`}
	blocked, err := c.PrepareEnvironment(ctx, dir, false, plan)
	record(blocked)
	if err == nil || blocked.Status != "blocked" || blocked.Fingerprint == report.Fingerprint {
		t.Fatal("incompatible version was accepted", err)
	}
	// Unsupported/private services are explicit blockers, without starting builds.
	plan.Blockers = []string{"private test service credentials are unavailable"}
	blocked, err = c.PrepareEnvironment(ctx, dir, false, plan)
	record(blocked)
	if err == nil || len(blocked.Checks) != 0 {
		t.Fatal("service blocker executed commands")
	}
	// Exercise cancellation during a real dependency command, including a child
	// which would write after cancellation if container cleanup were incomplete.
	plan.Blockers = nil
	plan.Checks = []domain.VerificationCommand{{Program: "node", Arguments: []string{"--version"}}}
	plan.WorkspaceCommands = []domain.VerificationCommand{{Program: "node", Arguments: []string{"-e", `const {spawn}=require('node:child_process');spawn(process.execPath,['-e',"setTimeout(()=>require('node:fs').writeFileSync('/workspace/escaped-child','bad'),6000)"],{stdio:'inherit'});require('node:fs').writeFileSync('setup-ready','ready');setInterval(()=>{},1000)`}}}
	setupCtx, stopSetup := context.WithCancel(ctx)
	type completion struct {
		report domain.ExecutionEnvironment
		err    error
	}
	done := make(chan completion, 1)
	go func() { r, e := c.PrepareEnvironment(setupCtx, dir, false, plan); done <- completion{r, e} }()
	deadline := time.Now().Add(90 * time.Second)
	for statError(filepath.Join(dir, "setup-ready")) != nil && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	ready := statError(filepath.Join(dir, "setup-ready")) == nil
	stopSetup()
	completed := <-done
	record(completed.report)
	if !ready || !errors.Is(completed.err, context.Canceled) {
		t.Fatal("real setup cancellation failed", ready, completed.err)
	}
	time.Sleep(7 * time.Second)
	if !errors.Is(statError(filepath.Join(dir, "escaped-child")), os.ErrNotExist) {
		t.Fatal("setup descendant survived cancellation")
	}
	stopped, stop := context.WithCancel(ctx)
	stop()
	_, err = c.PrepareEnvironment(stopped, dir, false, plan)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled setup did not return cancellation", err)
	}
	if _, err = os.Stat(filepath.Join(dir, ".git", "forgeflow-probe")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("Git boundary changed")
	}
	record(map[string]any{"result": "PASS", "fixture": dir, "cleanup": "owned t.TempDir removed by Go test cleanup"})
}
