package execution

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

// Explicit protocol fixture: no external commands run in this unit test.
type setupFailureRunner struct {
	fixtureRunner
	calls int
}

func (f *setupFailureRunner) Command(_ context.Context, dir string, c domain.VerificationCommand, _ bool) (domain.TestRun, error) {
	f.calls++
	return domain.TestRun{Command: c, Directory: dir, ExitCode: 1, StartedAt: time.Now(), FinishedAt: time.Now(), Output: "java.io.FileNotFoundException: wrapper.zip.lck (Access is denied)\nat org.gradle.wrapper.Install.createDist"}, errors.New("fixture setup failure")
}
func TestGradleSetupFailureStopsBeforeRetryingCodeOrOtherCommands(t *testing.T) {
	runner := &setupFailureRunner{}
	s, c := buildFixture(t, runner)
	repo, meta, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(repo, "go.mod")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"build.gradle", "gradlew"} {
		if err = os.WriteFile(filepath.Join(repo, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	r := domain.ExecutionRecord{ContributionID: c.ID, Plan: &domain.Plan{Tests: []domain.VerificationCommand{{Program: "./gradlew", Arguments: []string{"quickTest"}}}}}
	passed, err := s.tests(context.Background(), c.ID, repo, meta, &r)
	if passed || err == nil || !strings.Contains(err.Error(), "verification setup failed") {
		t.Fatalf("setup problem treated as code failure: %v", err)
	}
	tests, err := s.Store.Tests(context.Background(), c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 || len(tests) != 1 || r.FixIterations != 0 {
		t.Fatal("setup failure retried or lost evidence")
	}
}
