package execution

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/domain"
	"strings"
	"testing"
	"time"
)

// recipeRunner models the osh-core failure: an offline first attempt blocks
// on download approval, the resumed attempt's generated script fails, and a
// corrected recipe (a different base image here) succeeds.
type recipeRunner struct {
	fixtureRunner
	planners, preparations int
	prompts                []string
	failure                string
	failRecipe             string
	offlineError           string
}

func (f *recipeRunner) EnvironmentBase() string { return "fixture:1" }
func (f *recipeRunner) EnvironmentFingerprint(_ string, p domain.EnvironmentPlan) (string, error) {
	return p.BaseImage, nil
}
func (f *recipeRunner) PrepareEnvironment(_ context.Context, _ string, network bool, p domain.EnvironmentPlan) (domain.ExecutionEnvironment, error) {
	f.preparations++
	env := domain.ExecutionEnvironment{Status: "ready", Image: "image-" + p.BaseImage, Fingerprint: p.BaseImage, Network: network}
	if len(p.Blockers) > 0 {
		env.Status, env.Error = "blocked", "environment requirements need operator input: "+strings.Join(p.Blockers, "; ")
		return env, errors.New(env.Error)
	}
	if !network && f.offlineError != "" {
		env.Status, env.Error = "blocked", f.offlineError
		return env, errors.New(env.Error)
	}
	if p.BaseImage == f.failRecipe {
		env.Status, env.Error = "blocked", f.failure
		env.Checks = []domain.TestRun{{Command: domain.VerificationCommand{Program: "sh", Arguments: []string{"-c", "gradle forgeflowResolveDependencies"}}, ExitCode: 1, Output: "Could not get unknown property 'allprojects' for task ':forgeflowResolveDependencies'", StartedAt: time.Now(), FinishedAt: time.Now()}}
		return env, errors.New(env.Error)
	}
	return env, nil
}
func (f *recipeRunner) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	if r.Role != "environment-planner" {
		return f.fixtureRunner.Run(ctx, r, event)
	}
	f.planners++
	f.prompts = append(f.prompts, r.Prompt)
	p := domain.EnvironmentPlan{Summary: "Temurin and Gradle", BaseImage: "fixture:" + string(rune('0'+f.planners)), Checks: []domain.VerificationCommand{{Program: "gradle", Arguments: []string{"--version"}}}}
	if strings.Contains(r.Prompt, "Downloads approved: false") {
		p.Blockers = []string{"The JDK image and Gradle distribution must be downloaded; downloads are not approved."}
	}
	b, _ := json.Marshal(p)
	return codex.Result{Output: string(b)}, nil
}

func TestResumedBlockedEnvironmentStillGetsOneAutomaticRepair(t *testing.T) {
	runner := &recipeRunner{
		failRecipe: "fixture:2",
		failure:    "environment prerequisite failed: sh -c gradle forgeflowResolveDependencies: exit 1\nOutput:\nCould not get unknown property 'allprojects' for task ':forgeflowResolveDependencies'",
	}
	s, c := buildFixture(t, runner)
	repo, meta, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r := domain.ExecutionRecord{ContributionID: c.ID}
	if err = s.prepareEnvironment(ctx, c, &r, repo, meta, false); err == nil {
		t.Fatal("offline preparation should block on download approval")
	}
	// The human resumes with downloads approved. The planner was told downloads
	// were unavailable, so it replans; that replan's failure must still get its
	// one evidence-driven correction instead of pausing immediately.
	r.Network = true
	if err = s.prepareEnvironment(ctx, c, &r, repo, meta, false); err != nil {
		t.Fatalf("automatic recipe correction did not run: %v (planners=%d preparations=%d)", err, runner.planners, runner.preparations)
	}
	if runner.planners != 3 || r.Environment.Status != "ready" || r.EnvironmentPlan.BaseImage != "fixture:3" {
		t.Fatalf("unexpected repair path: planners=%d env=%+v plan=%+v", runner.planners, r.Environment, r.EnvironmentPlan)
	}
	if !strings.Contains(runner.prompts[2], "unknown property 'allprojects'") {
		t.Fatal("the correction prompt did not contain the actual failure")
	}
	if r.Incident != nil {
		t.Fatalf("resolved repair left an incident: %+v", r.Incident)
	}
}

func TestInfrastructureFailureRerunsSavedRecipeWithoutPlanning(t *testing.T) {
	for _, failure := range []string{
		"local image unavailable: failed to connect to the Docker API at npipe:////./pipe/dockerDesktopLinuxEngine",
		"environment base image is not cached; dependency download approval is required",
	} {
		runner := &recipeRunner{}
		s, c := buildFixture(t, runner)
		repo, meta, err := s.Paths(c)
		if err != nil {
			t.Fatal(err)
		}
		plan := domain.EnvironmentPlan{Summary: "saved", BaseImage: "fixture:9", Checks: []domain.VerificationCommand{{Program: "gradle", Arguments: []string{"--version"}}}}
		r := domain.ExecutionRecord{ContributionID: c.ID, Network: true, EnvironmentPlan: &plan, Environment: &domain.ExecutionEnvironment{Status: "blocked", Fingerprint: "fixture:9", Error: failure}}
		if err = s.prepareEnvironment(context.Background(), c, &r, repo, meta, false); err != nil {
			t.Fatal(failure, err)
		}
		if runner.planners != 0 || runner.preparations != 1 || r.Environment.Status != "ready" {
			t.Fatalf("%s: spent planning on an infrastructure failure: planners=%d preparations=%d", failure, runner.planners, runner.preparations)
		}
	}
}

func TestTimedOutPreparationReplansWithSavedCommandEvidence(t *testing.T) {
	runner := &recipeRunner{}
	s, c := buildFixture(t, runner)
	repo, meta, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	plan := domain.EnvironmentPlan{Summary: "saved", BaseImage: "fixture:9", Checks: []domain.VerificationCommand{{Program: "python3", Arguments: []string{"--version"}}}}
	// Records saved before this change kept only the bare deadline error.
	env := &domain.ExecutionEnvironment{Status: "blocked", Fingerprint: "fixture:9", Error: "context deadline exceeded", Checks: []domain.TestRun{{Command: domain.VerificationCommand{Program: "python3", Arguments: []string{"-m", "pip", "install", "mkdocs-material"}}, ExitCode: 1, Output: "Installing collected packages: mkdocs-material"}}}
	r := domain.ExecutionRecord{ContributionID: c.ID, Network: true, EnvironmentPlan: &plan, Environment: env, Incident: &domain.ExecutionIncident{Kind: "environment_blocked", Evidence: "context deadline exceeded"}}
	if err = s.prepareEnvironment(context.Background(), c, &r, repo, meta, false); err != nil {
		t.Fatal(err)
	}
	if runner.planners != 1 || !strings.Contains(runner.prompts[0], "Installing collected packages: mkdocs-material") || !strings.Contains(runner.prompts[0], codex.ContainerRuntimeDir) {
		t.Fatalf("planner did not see the slow step or the fast runtime location: planners=%d", runner.planners)
	}
}

// Records saved by older pipelines advised supplying prerequisites even for
// recipe defects and slow steps. Diagnosis must describe the recovery that
// will actually run, read-only, without rewriting the saved incident.
func TestLegacyEnvironmentIncidentDescribesActualRecovery(t *testing.T) {
	for _, tc := range []struct{ failure, want string }{
		{"context deadline exceeded", "environment planner for a corrected recipe"},
		{"environment prerequisite failed: Could not get unknown property 'allprojects'", "environment planner for a corrected recipe"},
		{"local image unavailable: failed to connect to the Docker API at npipe:////./pipe/dockerDesktopLinuxEngine", "reruns it unchanged"},
	} {
		s, c := buildFixture(t, &recipeRunner{})
		ctx := context.Background()
		plan := domain.EnvironmentPlan{Summary: "saved", BaseImage: "fixture:9", Checks: []domain.VerificationCommand{{Program: "gradle", Arguments: []string{"--version"}}}}
		saved := &domain.ExecutionIncident{ID: "legacy", Kind: "environment_blocked", Summary: "Repository environment setup is incomplete; coding and tests are paused.", Evidence: tc.failure, NextAction: "Supply the listed prerequisites, then recover explicitly.", Status: "needs_input", DiagnosisVersion: 5}
		r := domain.ExecutionRecord{ContributionID: c.ID, Status: "NEEDS_INPUT", Phase: "PLANNING", Network: true, EnvironmentPlan: &plan, Environment: &domain.ExecutionEnvironment{Status: "blocked", Error: tc.failure}, Incident: saved}
		if err := s.Store.SaveExecution(ctx, r, "test", "legacy record"); err != nil {
			t.Fatal(err)
		}
		d, err := s.Diagnose(ctx, c.ID)
		if err != nil || d == nil || !d.CanRecover || !strings.Contains(d.NextAction, tc.want) || d.DiagnosisVersion != diagnosisVersion {
			t.Fatalf("%s: diagnosis %+v %v", tc.failure, d, err)
		}
		stored, _ := s.Store.Execution(ctx, c.ID)
		if stored.Incident.NextAction != saved.NextAction || stored.Incident.DiagnosisVersion != 5 {
			t.Fatal("diagnosis rewrote the saved incident")
		}
	}
}
