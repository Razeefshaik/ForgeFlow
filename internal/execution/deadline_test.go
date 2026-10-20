package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestVerificationDeadlineUsesCIEvidenceAndBuildTool(t *testing.T) {
	gradlew := domain.VerificationCommand{Program: "./gradlew", Arguments: []string{"test"}}
	for _, tc := range []struct {
		name    string
		command domain.VerificationCommand
		ci      time.Duration
		want    time.Duration
	}{
		// 2026-10-17: ./gradlew test was killed at exactly 10 minutes, twice.
		{"wrapper without evidence", gradlew, 0, 30 * time.Minute},
		{"provisioned gradle path", domain.VerificationCommand{Program: "/opt/gradle-8.10.2/bin/gradle", Arguments: []string{"--offline", "build"}}, 0, 30 * time.Minute},
		{"maven", domain.VerificationCommand{Program: "mvn", Arguments: []string{"verify"}}, 0, 30 * time.Minute},
		{"maven wrapper", domain.VerificationCommand{Program: "./mvnw", Arguments: []string{"test"}}, 0, 30 * time.Minute},
		{"other program", domain.VerificationCommand{Program: "python3", Arguments: []string{"-m", "pytest"}}, 0, 10 * time.Minute},
		// osh-core: a 6.4 minute CI median ran over 20 minutes locally.
		{"observed CI median", gradlew, 384 * time.Second, 4*384*time.Second + 10*time.Minute},
		{"fast CI never lowers the build default", gradlew, time.Minute, 30 * time.Minute},
		{"fast CI raises a short default", domain.VerificationCommand{Program: "python3", Arguments: []string{"-m", "pytest"}}, time.Minute, 14 * time.Minute},
		{"slow CI is capped", gradlew, time.Hour, 2 * time.Hour},
		{"go timeout still applies", domain.VerificationCommand{Program: "go", Arguments: []string{"test", "-timeout", "20m", "./..."}}, time.Minute, 22 * time.Minute},
		{"planner budget wins", domain.VerificationCommand{Program: "./gradlew", Arguments: []string{"test"}, TimeoutSeconds: 900}, time.Hour, 15 * time.Minute},
	} {
		if got := verificationDeadline(tc.command, tc.ci); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

type deadlineRunner struct {
	fixtureRunner
	remaining []time.Duration
}

func (f *deadlineRunner) Command(ctx context.Context, dir string, c domain.VerificationCommand, network bool) (domain.TestRun, error) {
	deadline, _ := ctx.Deadline()
	f.remaining = append(f.remaining, time.Until(deadline))
	now := time.Now().UTC()
	return domain.TestRun{Command: c, Directory: dir, StartedAt: now, FinishedAt: now, Output: "BUILD SUCCESSFUL"}, nil
}

func TestRecordedCommandDeadlineComesFromApprovedCIEvidence(t *testing.T) {
	command := domain.VerificationCommand{Program: "./gradlew", Arguments: []string{"test"}}
	for _, tc := range []struct {
		ci   *domain.CIEvidence
		want time.Duration
	}{
		{nil, 30 * time.Minute},
		{&domain.CIEvidence{Workflows: []string{"ci.yml"}, SuccessfulRuns: 5, MedianSeconds: 384}, 4*384*time.Second + 10*time.Minute},
	} {
		runner := &deadlineRunner{}
		s, c := buildFixtureSpec(t, runner, "", false, fixtureSpec{CI: tc.ci})
		repo, _, err := s.Paths(c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.recordedCommand(context.Background(), c.ID, repo, command, false, ""); err != nil {
			t.Fatal(err)
		}
		if len(runner.remaining) != 1 || runner.remaining[0] > tc.want || runner.remaining[0] < tc.want-time.Minute {
			t.Fatalf("command deadline %v, want %s", runner.remaining, tc.want)
		}
		events, err := s.Store.Events(context.Background(), 0, c.ID, 100)
		if err != nil {
			t.Fatal(err)
		}
		recorded := false
		for _, event := range events {
			if event.Type == "TestRunStarted" && strings.Contains(event.Message, tc.want.String()+" deadline") {
				recorded = true
			}
		}
		if !recorded {
			t.Fatal("deadline missing from the audit event", events)
		}
	}
}

func TestTimedOutDownloadNeedsDownloadsAtTheEnd(t *testing.T) {
	command := domain.VerificationCommand{Program: "./gradlew", Arguments: []string{"test"}}
	wrapper := "Downloading https://services.gradle.org/distributions/gradle-8.10.2-bin.zip\n.............10%.............20%.............100%\nWelcome to Gradle 8.10.2!\n"
	progress := strings.Repeat("> Task :core:test\nOshCoreTest > parsesHandbook() PASSED\n", 60)
	d := diagnoseCommand(command, wrapper+progress, context.DeadlineExceeded, true)
	if d.Kind != "verification_timeout" || d.Retryable {
		t.Fatalf("a passing suite past its deadline was diagnosed as a download stall: %s", d.Kind)
	}
	stalled := wrapper + progress + "> Task :core:compileJava\nDownloading https://repo.maven.apache.org/maven2/org/example/big/1.0/big-1.0.jar\n"
	if d = diagnoseCommand(command, stalled, context.DeadlineExceeded, true); d.Kind != "dependency_network" || !d.Retryable {
		t.Fatalf("a download in progress at the deadline was not retried: %s", d.Kind)
	}
	if d = diagnoseCommand(command, stalled, errors.New("exit status 1"), true); d.Kind == "dependency_network" {
		t.Fatal("an ordinary failure was treated as a download stall")
	}
}
