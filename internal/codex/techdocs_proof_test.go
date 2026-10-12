package codex

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in proof of public documentation dependencies, using a disposable copy
// of the actual handbook. No live contribution is executed or modified.
func TestRealTechDocsEnvironment(t *testing.T) {
	if os.Getenv("FORGEFLOW_TEST_TECHDOCS") != "1" {
		t.Skip("explicit Docker/download opt-in required")
	}
	base, source, evidence := os.Getenv("FORGEFLOW_TEST_CONTAINER_IMAGE"), os.Getenv("FORGEFLOW_TEST_DOCS_SOURCE"), os.Getenv("FORGEFLOW_TEST_EVIDENCE")
	if base == "" || source == "" || evidence == "" {
		t.Fatal("image, read-only documentation source and evidence directory required")
	}
	dir := t.TempDir()
	err := filepath.WalkDir(source, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			t.Fatalf("linked source rejected: %s", rel)
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(dir, rel), 0700)
		}
		// The handbook needs only its config and docs, not generated app code.
		if rel != "mkdocs.yml" && !strings.HasPrefix(rel, "docs"+string(filepath.Separator)) {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), body, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	init := exec.Command("git", "init", dir)
	if output, err := init.CombinedOutput(); err != nil {
		t.Fatal(err, string(output))
	}
	if err = os.MkdirAll(evidence, 0700); err != nil {
		t.Fatal(err)
	}
	log, err := os.OpenFile(filepath.Join(evidence, "techdocs-environment.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	record := func(v any) {
		t.Helper()
		if err := json.NewEncoder(log).Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	ctx, stop := context.WithTimeout(context.Background(), 12*time.Minute)
	defer stop()
	c := &CLI{VerificationImage: base}
	plan := domain.EnvironmentPlan{
		Summary:   "Pinned public MkDocs and TechDocs plugin for the handbook, without cloud services",
		BaseImage: base, Files: []string{"mkdocs.yml"},
		ImageCommands: []domain.VerificationCommand{
			{Program: "sh", Arguments: []string{"-c", "apt-get update && apt-get install -y --no-install-recommends python3-venv && rm -rf /var/lib/apt/lists/*"}},
			{Program: "python3", Arguments: []string{"-m", "venv", "/opt/techdocs"}},
			{Program: "/opt/techdocs/bin/pip", Arguments: []string{"install", "--disable-pip-version-check", "mkdocs==1.6.1", "mkdocs-techdocs-core==1.7.1"}},
			{Program: "/opt/techdocs/bin/pip", Arguments: []string{"freeze", "--all"}},
		},
		Checks: []domain.VerificationCommand{{Program: "/opt/techdocs/bin/python", Arguments: []string{"-c", "import importlib.metadata as m; assert m.version('mkdocs')=='1.6.1'; assert m.version('mkdocs-techdocs-core')=='1.7.1'; assert any(e.name=='techdocs-core' for e in m.entry_points(group='mkdocs.plugins'))"}}},
	}
	report, err := c.PrepareEnvironment(ctx, dir, true, plan)
	record(report)
	if err != nil {
		t.Fatal(err)
	}
	build := domain.VerificationCommand{Program: "/opt/techdocs/bin/mkdocs", Arguments: []string{"build", "--strict", "--site-dir", ".forgeflow-runtime/site"}}
	run, err := c.Command(ctx, dir, build, false)
	record(run)
	if err != nil || run.ExitCode != 0 {
		t.Fatal("real offline handbook build failed", err, run.Output)
	}
	page, err := os.ReadFile(filepath.Join(dir, ".forgeflow-runtime", "site", "index.html"))
	if err != nil || !strings.Contains(string(page), "Handbook") {
		t.Fatal("handbook output missing", err)
	}
	cached, err := c.PrepareEnvironment(ctx, dir, false, plan)
	record(cached)
	if err != nil || cached.Image != report.Image {
		t.Fatal("offline prepared-image reuse failed", err)
	}
	// A documentation edit must use the same prepared dependencies and rebuild.
	index := filepath.Join(dir, "docs", "index.md")
	f, err := os.OpenFile(index, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("\nForgeFlow disposable documentation edit proof.\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	run, err = c.Command(ctx, dir, build, false)
	record(run)
	if err != nil || run.ExitCode != 0 {
		t.Fatal("edited handbook build failed", err, run.Output)
	}
	page, err = os.ReadFile(filepath.Join(dir, ".forgeflow-runtime", "site", "index.html"))
	if err != nil || !strings.Contains(string(page), "ForgeFlow disposable documentation edit proof.") {
		t.Fatal("edited content absent", err)
	}
	// Cancel an actual container command with a descendant that would write
	// later if runtime cleanup left it alive.
	running, cancelCommand := context.WithCancel(ctx)
	t.Cleanup(cancelCommand)
	type completion struct {
		run domain.TestRun
		err error
	}
	done := make(chan completion, 1)
	command := domain.VerificationCommand{Program: "/opt/techdocs/bin/python", Arguments: []string{"-c", "import subprocess,sys,time,pathlib; subprocess.Popen([sys.executable,'-c',\"import time,pathlib; time.sleep(6); pathlib.Path('escaped-child').write_text('bad')\"]); pathlib.Path('cancel-ready').write_text('ready'); time.sleep(120)"}}
	go func() { run, err := c.Command(running, dir, command, false); done <- completion{run, err} }()
	deadline := time.Now().Add(60 * time.Second)
	for statError(filepath.Join(dir, "cancel-ready")) != nil && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	ready := statError(filepath.Join(dir, "cancel-ready")) == nil
	cancelCommand()
	completed := <-done
	record(completed.run)
	if !ready || !errors.Is(completed.err, context.Canceled) {
		t.Fatal("container cancellation failed", ready, completed.err)
	}
	time.Sleep(7 * time.Second)
	if !errors.Is(statError(filepath.Join(dir, "escaped-child")), os.ErrNotExist) {
		t.Fatal("descendant survived container cleanup")
	}
	record(map[string]any{"result": "PASS", "fixture": dir, "source": source, "cleanup": "Go test removes owned temporary repository; prepared public dependency image remains reusable"})
}
