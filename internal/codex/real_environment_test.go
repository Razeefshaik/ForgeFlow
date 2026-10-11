package codex

import (
	"context"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in, no model session and no administrator setup.
func TestRealSandboxGoAssertionAndTimeoutEvidence(t *testing.T) {
	if os.Getenv("FORGEFLOW_TEST_GO_RUNTIME") != "1" {
		t.Skip("opt in to installed sandbox probe; no model or admin setup")
	}
	c := Resolve("", "")
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"go.mod":       "module example.com/timeoutprobe\n\ngo 1.23\n",
		"hang_test.go": "package timeoutprobe\nimport (\"testing\";\"time\")\nfunc TestAssertion(t *testing.T){t.Fatal(\"recorded real assertion\")}\nfunc TestHang(t *testing.T){time.Sleep(time.Minute)}\n",
	} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GOTOOLCHAIN", "local")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	run, err := c.Command(ctx, repo, domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./...", "-timeout=250ms"}}, false)
	if err == nil || run.ExitCode == 0 || !strings.Contains(run.Output, "--- FAIL: TestAssertion") || !strings.Contains(run.Output, "panic: test timed out after") || run.ExecutedCommand == nil {
		t.Fatal("real timeout evidence missing", run, err)
	}
	t.Log(run.Output)
}
