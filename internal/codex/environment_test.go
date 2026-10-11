package codex

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestGradleHomeIsWritableAndIsolatedPerWorkspace(t *testing.T) {
	t.Setenv("GRADLE_USER_HOME", filepath.Join(t.TempDir(), "global-gradle"))
	a, b := t.TempDir(), t.TempDir()
	for _, dir := range []string{a, b} {
		count := 0
		expected := filepath.Join(dir, ".forgeflow-runtime", "gradle")
		for _, entry := range workspaceEnvironment(dir) {
			key, value, _ := strings.Cut(entry, "=")
			if strings.EqualFold(key, "GRADLE_USER_HOME") {
				count++
				if value != expected {
					t.Fatalf("shared cache escaped workspace: %s", value)
				}
			}
		}
		if count != 1 {
			t.Fatalf("duplicate or missing Gradle override: %d", count)
		}
		if err := os.WriteFile(filepath.Join(expected, "wrapper-lock-probe"), []byte("local"), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWindowsWrapperUsesExplicitLocalPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows wrapper resolution")
	}
	for _, name := range []string{"./gradlew", "./mvnw"} {
		cmd, err := resolveCommand(domain.VerificationCommand{Program: name, Arguments: []string{"--version"}}, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Program != "cmd.exe" || !strings.HasPrefix(cmd.Arguments[2], `.\`) {
			t.Fatalf("wrapper could resolve outside workspace: %+v", cmd)
		}
	}
}

func TestGoTemporaryDirectoryIsWritableAndIsolated(t *testing.T) {
	t.Setenv("GOTMPDIR", filepath.Join(t.TempDir(), "inherited-unwritable"))
	for _, dir := range []string{t.TempDir(), t.TempDir()} {
		expected := filepath.Join(dir, ".forgeflow-runtime", "tmp")
		count := 0
		env, err := workspaceEnvironmentForNetwork(dir, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range env {
			key, value, _ := strings.Cut(entry, "=")
			if strings.EqualFold(key, "GOTMPDIR") {
				count++
				if value != expected {
					t.Fatalf("Go temporary directory escaped workspace: %s", value)
				}
				probe, err := os.MkdirTemp(value, "go-build")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(probe); err != nil {
					t.Fatal(err)
				}
			}
		}
		if count != 1 {
			t.Fatalf("duplicate or missing GOTMPDIR override: %d", count)
		}
	}
}

func TestGoTestFilterAllowsAlternationOnlyAsFilter(t *testing.T) {
	for _, tc := range []struct {
		command domain.VerificationCommand
		allowed bool
	}{
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./internal/needs", "-run", "TestOne|TestTwo"}}, true},
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-run=TestOne|TestTwo", "./..."}}, true},
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-bench", "BenchmarkOne|BenchmarkTwo"}}, true},
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-skip=TestOne|TestTwo"}}, true},
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-run", "[broken"}}, false},
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-run"}}, false},
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "TestOne|TestTwo"}}, false},
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-run", "TestOne|TestTwo&other"}}, false},
		{domain.VerificationCommand{Program: "gradle", Arguments: []string{"-run", "TestOne|TestTwo"}}, false},
	} {
		if err := ValidateCommand(tc.command); (err == nil) != tc.allowed {
			t.Fatalf("ValidateCommand(%+v) = %v, allowed %v", tc.command, err, tc.allowed)
		}
	}
}

func TestGoUsesWorkspaceTemporaryDirectoryAndNativeFilter(t *testing.T) {
	repo := t.TempDir()
	// Inheriting a file as GOTMPDIR demonstrably breaks Go before tests. The
	// workspace environment must replace it, and alternation must reach Go intact.
	wrong := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(wrong, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTMPDIR", wrong)
	t.Setenv("GOTOOLCHAIN", "local")
	for name, body := range map[string]string{
		"go.mod":          "module example.com/runtimeprobe\n\ngo 1.23\n",
		"runtime_test.go": "package runtimeprobe\nimport (\"testing\";\"os\";\"path/filepath\")\nfunc TestOne(t *testing.T) { p := os.Getenv(\"GOTMPDIR\"); if filepath.Base(p)!=\"tmp\" || filepath.Base(filepath.Dir(p))!=\".forgeflow-runtime\" {t.Fatal(p)} }; func TestTwo(t *testing.T){}; func TestExcluded(t *testing.T){t.Fatal(\"filter did not reach Go\")}\n",
	} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	env, err := workspaceEnvironmentForNetwork(repo, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-run", "TestOne|TestTwo", "./...")
	cmd.Dir, cmd.Env = repo, env
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "PASS: TestOne") || !strings.Contains(string(output), "PASS: TestTwo") || strings.Contains(string(output), "RUN   TestExcluded") {
		t.Fatalf("real Go runtime/filter execution: %v\n%s", err, output)
	}
}

func TestCommandValidationReturnsActionableArgument(t *testing.T) {
	err := ValidateCommand(domain.VerificationCommand{Program: "go", Arguments: []string{"test", "|"}})
	var invalid *CommandValidationError
	if !errors.As(err, &invalid) || invalid.Argument != 1 || invalid.Program != "go" {
		t.Fatalf("missing validation detail: %v", err)
	}
}

func TestCodexErrorEventsPreserveTransportEvidence(t *testing.T) {
	for _, raw := range []string{`{"type":"turn.failed","error":{"message":"HTTP 429 Too Many Requests"}}`, `{"type":"error","message":"HTTP 429 Too Many Requests"}`, `{"type":"error","error":"HTTP 429 Too Many Requests"}`} {
		if !strings.Contains(eventFailure([]byte(raw)), "HTTP 429") {
			t.Fatal("lost service error", raw)
		}
	}
}
