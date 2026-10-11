package codex

import (
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "TestOne|TestTwo"}}, false},
		{domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-run", "TestOne|TestTwo&other"}}, false},
		{domain.VerificationCommand{Program: "gradle", Arguments: []string{"-run", "TestOne|TestTwo"}}, false},
	} {
		if err := ValidateCommand(tc.command); (err == nil) != tc.allowed {
			t.Fatalf("ValidateCommand(%+v) = %v, allowed %v", tc.command, err, tc.allowed)
		}
	}
}
