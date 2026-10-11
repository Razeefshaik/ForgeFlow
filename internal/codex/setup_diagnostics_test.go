package codex

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func runtimeLockLine(at time.Time, path string, code string) string {
	return "[" + at.Format(time.RFC3339Nano) + "] runtime read/execute validation failed: validate runtime read/execute access on " + path + ": open ACL target for root-only update: The process cannot access the file because it is being used by another process. (os error " + code + ")\n"
}

func TestSandboxSetupRuntimeLockRequiresCurrentAttemptEvidence(t *testing.T) {
	now := time.Now().UTC()
	path := `C:\Users\User\Codex\runtime\node_repl.exe`
	for _, tc := range []struct{ name, evidence, want string }{
		{"current", runtimeLockLine(now, path, "32"), path},
		{"stale", runtimeLockLine(now.Add(-time.Hour), path, "32"), ""},
		{"future", runtimeLockLine(now.Add(time.Hour), path, "32"), ""},
		{"different-error", runtimeLockLine(now, path, "5"), ""},
		{"no-timestamp", "runtime read/execute validation failed: " + path + " (os error 32)", ""},
		{"unrelated-command", "[" + now.Format(time.RFC3339Nano) + "] unrelated command: " + path + " (os error 32)\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sandboxRuntimeLockFromLog(strings.NewReader(tc.evidence), now.Add(-time.Second), now.Add(time.Second))
			if got != tc.want {
				t.Fatal(got, tc.want)
			}
		})
	}
}

func TestSandboxSetupRuntimeDiagnosisReadsBoundedPublicLogsOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	now := time.Now().UTC()
	path := `C:\Codex\runtime\node_repl.exe`
	for _, dir := range []string{".sandbox", ".sandbox-secrets"} {
		if err := os.Mkdir(filepath.Join(home, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".sandbox-secrets", "sandbox.log"), []byte(runtimeLockLine(now, path, "32")), 0600); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("helper_unknown_error: setup refresh had errors")
	if got := diagnoseSandboxSetupFailure(cause, now); got != nil {
		t.Fatal("read nonpublic setup state", got)
	}
	data := strings.Repeat("old unrelated output\n", 10000) + runtimeLockLine(now, path, "32")
	if err := os.WriteFile(filepath.Join(home, ".sandbox", "sandbox.2026-10-11.log"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	got := diagnoseSandboxSetupFailure(cause, now)
	var lock *sandboxRuntimeInUseError
	if !errors.As(got, &lock) || lock.Path != path || !errors.Is(got, cause) || !strings.Contains(got.Error(), "retry sandbox setup") {
		t.Fatal("lost current lock diagnosis or recovery", got)
	}
	if got := diagnoseSandboxSetupFailure(errors.New("UAC declined"), now); got != nil {
		t.Fatal("attached unrelated lock to another failure", got)
	}
}

func TestRuntimeLockNeedsInputWithoutAdoptingOrProbingSandbox(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows setup action")
	}
	m, cli := environmentFixture(t)
	m.setup = func(context.Context, *CLI) (string, error) {
		err := &sandboxRuntimeInUseError{Path: `C:\Codex\node_repl.exe`, Cause: errors.New("setup refresh had errors")}
		return err.Error(), err
	}
	probed := false
	m.probe = func(context.Context, *CLI) ([]EnvironmentCheck, error) { probed = true; return nil, nil }
	if _, err := m.Start("setup", "elevated", true); err != nil {
		t.Fatal(err)
	}
	m.Wait()
	s := m.Status()
	if s.Status != "needs_input" || s.Running || cli.SandboxMode() != "auto" || probed || !strings.Contains(s.Error, "node_repl.exe") {
		t.Fatal(s)
	}
	if _, err := os.Stat(m.settingsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("adopted an unverified sandbox")
	}
}
