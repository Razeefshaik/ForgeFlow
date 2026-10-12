package codex

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHookPreparationKeepsGitProtected(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	args, err := containerArguments("fixture:1", "fixture", dir, domain.VerificationCommand{Program: "make", Arguments: []string{"hooks"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := hookPreparationArguments(args, dir)
	joined := strings.Join(prepared, "\n")
	if err != nil || !strings.Contains(joined, "target=/workspace/.git,readonly") || !strings.Contains(joined, "target=/workspace/.git/hooks\n") || !strings.Contains(joined, "target=/workspace/.git/info/lefthook.checksum\n") || strings.Count(joined, "type=bind,") != 4 {
		t.Fatal(joined, err)
	}
	if strings.Contains(strings.Join(args, "\n"), "target=/workspace/.git/hooks") {
		t.Fatal("ordinary command gained hook writes")
	}
}

func TestCompletionGateRequiresPublicChecksAndSource(t *testing.T) {
	p := environmentFixturePlan()
	p.CompletionGates = []domain.CompletionGate{{Binding: "working_tree", Requirement: "signed receipt", Source: "requirements.txt", Action: "Ask trusted signer", Checks: []domain.VerificationCommand{{Program: "verify-receipt"}}}}
	if err := ValidateEnvironmentPlan(p); err != nil {
		t.Fatal(err)
	}
	p.CompletionGates[0].Checks = nil
	if ValidateEnvironmentPlan(p) == nil {
		t.Fatal("accepted an unverifiable promise")
	}
	p.CompletionGates[0].Checks = []domain.VerificationCommand{{Program: "verify-receipt"}}
	p.CompletionGates[0].Source = "unread-policy.md"
	if ValidateEnvironmentPlan(p) == nil {
		t.Fatal("accepted unsnapshotted gate source")
	}
}

func TestVerificationBudgetCannotBeUnbounded(t *testing.T) {
	for _, seconds := range []int{-1, 7201} {
		command := domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}, TimeoutSeconds: seconds}
		if ValidateCommand(command) == nil || ValidateContainerCommand(command) == nil {
			t.Fatal("unsafe command duration admitted", seconds)
		}
	}
	if err := ValidateContainerCommand(domain.VerificationCommand{Program: "make", Arguments: []string{"verify-all"}, TimeoutSeconds: 3600}); err != nil {
		t.Fatal(err)
	}
}

func TestRealHookPreparationIsolationCancellation(t *testing.T) {
	if os.Getenv("FORGEFLOW_TEST_HOOKS") != "1" {
		t.Skip("opt in to real Docker hook preparation")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	c := &CLI{VerificationImage: os.Getenv("FORGEFLOW_TEST_CONTAINER_IMAGE")}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	install := domain.VerificationCommand{Program: "sh", Arguments: []string{"-c", `set -eu
test "$(id -u)" = 1000
echo installed > .git/hooks/pre-commit
echo checksum > .git/info/lefthook.checksum
if touch .git/info/exclude 2>/tmp/denied; then exit 1; fi
grep -E 'Read-only file system|Permission denied' /tmp/denied
if touch .git/config 2>/tmp/denied; then exit 1; fi
grep -E 'Read-only file system|Permission denied' /tmp/denied
test ! -e /var/run/docker.sock
echo hook-install-pass`}}
	rec, err := c.containerCommandMode(ctx, c.VerificationImage, dir, install, false, true)
	t.Log(rec.Output)
	if err != nil || rec.ExitCode != 0 {
		t.Fatal(err)
	}
	check := domain.VerificationCommand{Program: "sh", Arguments: []string{"-c", `set -eu
grep installed .git/hooks/pre-commit
grep checksum .git/info/lefthook.checksum
if echo corrupted 2>/tmp/denied > .git/info/lefthook.checksum; then exit 1; fi
grep -E 'Read-only file system|Permission denied' /tmp/denied
if echo corrupted 2>/tmp/denied > .git/hooks/pre-commit; then exit 1; fi
grep -E 'Read-only file system|Permission denied' /tmp/denied
echo hook-readonly-pass`}}
	rec, err = c.containerCommandImage(ctx, c.VerificationImage, dir, check, false)
	t.Log(rec.Output)
	if err != nil || rec.ExitCode != 0 {
		t.Fatal(err)
	}
	cancelCtx, stop := context.WithCancel(ctx)
	ready := filepath.Join(dir, "started")
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, err := os.Stat(ready); err == nil {
				stop()
				return
			}
			select {
			case <-cancelCtx.Done():
				return
			case <-time.After(50 * time.Millisecond):
			}
		}
	}()
	defer stop()
	rec, err = c.containerCommandMode(cancelCtx, c.VerificationImage, dir, domain.VerificationCommand{Program: "sh", Arguments: []string{"-c", "touch started; (sleep 3; touch escaped-child) & wait"}}, false, true)
	<-done
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancel not observed", rec, err)
	}
	time.Sleep(4 * time.Second)
	if _, err := os.Stat(filepath.Join(dir, "escaped-child")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("child survived cleanup", err)
	}
	t.Log("Cancellation removed the container and delayed child; disposable directory is removed by test cleanup")
}
