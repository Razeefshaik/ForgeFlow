package execution

import (
	"context"
	"encoding/json"
	"errors"
	"forgeflow/internal/codex"
	"forgeflow/internal/config"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixtureRunner struct {
	contributors, reviews int
	roles                 []string
	models                []string
	alwaysFail            bool
	blockedVerification   bool
}

func (f *fixtureRunner) Check(context.Context, string, string) error { return nil }
func (f *fixtureRunner) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	f.roles = append(f.roles, r.Role)
	f.models = append(f.models, r.Model)
	if event != nil {
		if e := event(json.RawMessage(`{"type":"turn.started"}`)); e != nil {
			return codex.Result{}, e
		}
	}
	var value any
	switch r.Role {
	case "planner":
		value = domain.Plan{Summary: "Correct addition and add regression tests", RootCause: "Subtracting the second operand", Files: []string{"sum.go", "sum_test.go"}, Strategy: "Use addition and test positive and negative operands", Tests: []domain.VerificationCommand{{Program: "go", Arguments: []string{"test", "./..."}}}, Risks: []string{}, Unknowns: []string{}}
	case "contributor":
		f.contributors++
		if f.contributors == 1 {
			os.WriteFile(filepath.Join(r.Directory, "sum.go"), []byte("package fixture\n// Addition fix in progress.\nfunc Add(a,b int) int {return a-b}\n"), 0600)
		} else {
			os.WriteFile(filepath.Join(r.Directory, "sum.go"), []byte("package fixture\nfunc Add(a,b int) int {return a+b}\n"), 0600)
		}
		if f.contributors >= 3 {
			os.WriteFile(filepath.Join(r.Directory, "negative_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestNegative(t *testing.T){if Add(-2,-3)!=-5 {t.Fatal(\"negative sum\")}}\n"), 0600)
		}
		value = map[string]string{"status": "IMPLEMENTED", "summary": "Fixed addition with regression tests"}
		if f.blockedVerification && f.contributors >= 2 {
			value = map[string]string{"status": "BLOCKED", "summary": "Fixed the patch. Agent testing stalled on downloads and Docker is unavailable; required verification remains blocked."}
		}
	case "reviewer":
		f.reviews++
		review := domain.Review{Verdict: "APPROVE", Summary: "Correct minimal fix with positive and negative regression coverage", Findings: []domain.Finding{}}
		if f.reviews == 1 {
			review.Verdict = "REQUEST_CHANGES"
			review.Findings = []domain.Finding{{Severity: "medium", File: "sum_test.go", Line: 3, Explanation: "Missing negative operand coverage", Fix: "Add a negative operand regression test"}}
		}
		value = review
	}
	b, _ := json.Marshal(value)
	return codex.Result{SessionID: storage.ID(), Output: string(b), Usage: map[string]int64{"input_tokens": 10, "output_tokens": 5}}, nil
}
func (f *fixtureRunner) Command(ctx context.Context, dir string, c domain.VerificationCommand, network bool) (domain.TestRun, error) {
	rec := domain.TestRun{Command: c, Directory: dir, StartedAt: time.Now().UTC(), ExitCode: -1}
	cmd := exec.CommandContext(ctx, c.Program, c.Arguments...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	rec.FinishedAt = time.Now().UTC()
	rec.Output = string(out)
	if cmd.ProcessState != nil {
		rec.ExitCode = cmd.ProcessState.ExitCode()
	}
	if f.alwaysFail {
		rec.ExitCode = 1
		err = errors.New("synthetic repeated failure")
	}
	return rec, err
}
func buildFixture(t *testing.T, runner codex.Runner) (*Service, domain.Contribution) {
	return buildFixtureWithModel(t, runner, "")
}
func buildFixtureWithModel(t *testing.T, runner codex.Runner, model string) (*Service, domain.Contribution) {
	return buildFixtureWithApproval(t, runner, model, false)
}
func buildFixtureWithApproval(t *testing.T, runner codex.Runner, model string, requirePlanApproval bool) (*Service, domain.Contribution) {
	t.Helper()
	return buildFixtureWithFiles(t, runner, model, requirePlanApproval, nil)
}

// buildFixtureWithFiles commits extra repository files into the approved base.
func buildFixtureWithFiles(t *testing.T, runner codex.Runner, model string, requirePlanApproval bool, extra map[string]string) (*Service, domain.Contribution) {
	t.Helper()
	return buildFixtureSpec(t, runner, model, requirePlanApproval, fixtureSpec{Files: extra})
}

// fixtureSpec describes a disposable repository and its approved issue.
// Replace drops the default Go module so other ecosystems can be modeled.
type fixtureSpec struct {
	Files       map[string]string
	Replace     bool
	Title, Body string
	CI          *domain.CIEvidence
}

func buildFixtureSpec(t *testing.T, runner codex.Runner, model string, requirePlanApproval bool, spec fixtureSpec) (*Service, domain.Contribution) {
	t.Helper()
	extra := spec.Files
	if spec.Title == "" {
		spec.Title = "Correct Add"
	}
	if spec.Body == "" {
		spec.Body = "The Add function subtracts instead of adding. Fix addition and add positive and negative regression coverage."
	}
	ctx := context.Background()
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, ".cache"), 0700)
	base := filepath.Join(root, "external")
	repo := filepath.Join(base, "fixture", "repo")
	meta := filepath.Join(base, "fixture", ".autopilot")
	for _, path := range []string{repo, meta, filepath.Join(meta, "empty-hooks"), filepath.Join(base, "fixture", "artifacts")} {
		if e := os.MkdirAll(path, 0700); e != nil {
			t.Fatal(e)
		}
	}
	files := map[string]string{"go.mod": "module example.com/fixture\n\ngo 1.26\n", "sum.go": "package fixture\nfunc Add(a,b int) int {return a-b}\n", "sum_test.go": "package fixture\nimport \"testing\"\nfunc TestAdd(t *testing.T){if Add(2,3)!=5 {t.Fatal(\"addition incorrect\")}}\n"}
	if spec.Replace {
		files = map[string]string{}
	}
	for name, body := range extra {
		files[name] = body
	}
	for name, body := range files {
		path := filepath.Join(repo, filepath.FromSlash(name))
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(path, []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = gitEnv()
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("fixture Git: %v %s", e, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	git("add", ".")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "initial")
	sha := git("rev-parse", "HEAD")
	git("checkout", "-b", "autopilot/issue-1")
	store, e := storage.Open(ctx, filepath.Join(root, "test.db"), false)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { store.Close() })
	start := time.Now().UTC()
	run := domain.DiscoveryRun{ID: "fixture-scan", Status: "RUNNING", ConfigVersion: 1, StartedAt: start}
	if e = store.StartDiscovery(ctx, run); e != nil {
		t.Fatal(e)
	}
	end := time.Now().UTC()
	run.Status = "SUCCEEDED"
	run.FinishedAt = &end
	o := domain.Opportunity{ID: "fixture-issue", Repository: "example/fixture", Number: 1, Title: spec.Title, Evidence: &domain.Evidence{RunID: run.ID, ConfigVersion: 1}}
	if e = store.FinishDiscovery(ctx, run, []domain.Opportunity{o}); e != nil {
		t.Fatal(e)
	}
	c := domain.Contribution{ID: "fixture", OpportunityID: o.ID, Repository: o.Repository, Title: o.Title, State: "SELECTED", Branch: "autopilot/issue-1", BaseCommit: sha, Workspace: filepath.ToSlash(repo), ConfigVersion: 1, CodexModel: model}
	cfg := domain.ConfigVersion{Version: 1, Config: config.Default()}
	cfg.Config.Codex.MaxFixIterations = 3
	cfg.Config.Codex.MaxReviewCycles = 2
	cfg.Config.Contributions.RequirePlanApproval = requirePlanApproval
	issue, _ := json.Marshal(map[string]any{"number": 1, "state": "open", "title": spec.Title, "body": spec.Body})
	approved := o
	if spec.CI != nil {
		evidence := *o.Evidence
		evidence.Prerequisites = &domain.RepositoryAssessment{Repository: o.Repository, HeadSHA: sha, Status: "eligible", CI: spec.CI}
		approved.Evidence = &evidence
	}
	a := domain.WorkspaceApproval{Token: "fixture-approval", Config: cfg, Opportunity: approved, Issue: issue, Repository: json.RawMessage(`{"full_name":"example/fixture","default_branch":"main"}`)}
	if e = store.ApproveWorkspace(ctx, c, a); e != nil {
		t.Fatal(e)
	}
	for _, state := range []string{"PREPARING", "ANALYZING_REPOSITORY", "PLANNING", "BLOCKED"} {
		if e = store.Transition(ctx, c.ID, state); e != nil {
			t.Fatal(e)
		}
	}
	return New(ctx, store, runner, nil, root, base), c
}
func TestWorkflowRealCommandsFixIndependentReviewAndPRGate(t *testing.T) {
	runner := &fixtureRunner{}
	s, c := buildFixtureWithModel(t, runner, "model-a")
	ctx := context.Background()
	if _, e := s.Start(ctx, c.ID, false, false); e == nil {
		t.Fatal("execution started without approval")
	}
	if _, e := s.Start(ctx, c.ID, true, false); e != nil {
		t.Fatal(e)
	}
	s.Wait()
	current, _ := s.Store.Contribution(ctx, c.ID)
	r, e := s.Store.Execution(ctx, c.ID)
	if e != nil || current.State != "READY" {
		t.Fatalf("workflow did not finish: %+v %+v %v", current, r, e)
	}
	if r.FixIterations != 2 || r.ReviewCycles != 2 || r.Review.Verdict != "APPROVE" {
		t.Fatalf("missing bounded loops: %+v", r)
	}
	tests, _ := s.Store.Tests(ctx, c.ID)
	failed := false
	for _, run := range tests {
		if run.ExitCode != 0 {
			failed = true
		}
	}
	if !failed || tests[len(tests)-1].ExitCode != 0 {
		t.Fatal("real failure and recovery evidence missing")
	}
	agents, _ := s.Store.Agents(ctx)
	sessions := map[string]bool{}
	for _, a := range agents {
		if a.Status != "SUCCEEDED" || sessions[a.SessionID] || a.Model != "model-a" {
			t.Fatal("contexts not independent")
		}
		sessions[a.SessionID] = true
	}
	for _, model := range runner.models {
		if model != "model-a" {
			t.Fatalf("agent received wrong model: %q", model)
		}
	}
	if !strings.Contains(r.Diff, "negative_test.go") || r.Report == "" {
		t.Fatal("new file or report missing")
	}
	if _, e = s.SubmitPR(ctx, c.ID, "", false); e == nil {
		t.Fatal("PR submitted without approval")
	}
	prepared, e := s.PreparePR(ctx, c.ID)
	if e != nil || prepared.HeadCommit == c.BaseCommit || prepared.SubmissionToken == "" {
		t.Fatalf("local preparation failed: %+v %v", prepared, e)
	}
	repo, _, _ := s.Paths(c)
	committedDiff, _, e := s.diff(ctx, repo, c.BaseCommit)
	if e != nil || committedDiff != r.Diff {
		t.Fatal("reviewed diff changed when new files were committed", e)
	}
	if _, e = s.SubmitPR(ctx, c.ID, prepared.SubmissionToken, false); e == nil {
		t.Fatal("submission gate bypassed")
	}
	current, _ = s.Store.Contribution(ctx, c.ID)
	if current.State != "PR_PREPARED" {
		t.Fatal("PR opened unexpectedly")
	}
}
func TestRepeatedFailuresStopAtConfiguredBound(t *testing.T) {
	f := &fixtureRunner{alwaysFail: true}
	s, c := buildFixture(t, f)
	ctx := context.Background()
	if _, e := s.Start(ctx, c.ID, true, false); e != nil {
		t.Fatal(e)
	}
	s.Wait()
	r, _ := s.Store.Execution(ctx, c.ID)
	if r.Status != "BLOCKED" || r.FixIterations != 3 || !strings.Contains(r.Summary, "maximum fix") {
		t.Fatalf("retry bound not enforced: %+v", r)
	}
}
func TestRealCodexWorkflow(t *testing.T) {
	if os.Getenv("FORGEFLOW_REAL_CODEX_TEST") != "1" {
		t.Skip("opt in to actual authenticated Codex sessions")
	}
	cli := codex.Resolve("", "")
	cli.VerificationImage = os.Getenv("FORGEFLOW_TEST_CONTAINER_IMAGE")
	if mode := os.Getenv("FORGEFLOW_TEST_SANDBOX"); mode != "" {
		if err := cli.SetSandboxMode(mode); err != nil {
			t.Fatal(err)
		}
	}
	if base := os.Getenv("FORGEFLOW_TEST_CONTRIBUTIONS_ROOT"); base != "" {
		if !filepath.IsAbs(base) {
			t.Fatal("absolute disposable fixture parent required")
		}
		t.Setenv("TEMP", base)
		t.Setenv("TMP", base)
	}
	spec, ok := realWorkflowFixtures[os.Getenv("FORGEFLOW_TEST_FIXTURE")]
	if !ok {
		t.Fatal("unknown FORGEFLOW_TEST_FIXTURE; use go, python or gradle")
	}
	s, c := buildFixtureSpec(t, cli, os.Getenv("FORGEFLOW_TEST_MODEL"), false, spec)
	ctx := context.Background()
	if cli.VerificationImage != "" {
		repo, _, err := s.Paths(c)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := cli.RemoveRuntimeVolume(cleanup, repo); err != nil {
				t.Error(err)
			}
		})
	}
	if evidence := os.Getenv("FORGEFLOW_TEST_EVIDENCE"); evidence != "" {
		if err := os.MkdirAll(evidence, 0700); err != nil {
			t.Fatal(err)
		}
		defer func() {
			tests, testErr := s.Store.Tests(ctx, c.ID)
			agents, agentErr := s.Store.Agents(ctx)
			events, eventErr := s.Store.Events(ctx, 0, c.ID, 1000)
			record, recordErr := s.Store.Execution(ctx, c.ID)
			if err := s.exportEvents(ctx, c.ID, evidence); err != nil {
				t.Error(err)
			}
			repo, _, _ := s.Paths(c)
			if err := writeJSON(filepath.Join(evidence, "workflow.json"), map[string]any{"workspace": repo, "execution": record, "tests": tests, "agents": agents, "events": events, "errors": []error{testErr, agentErr, eventErr, recordErr}}); err != nil {
				t.Error(err)
			}
		}()
	}
	// Non-Go fixtures need public toolchains and packages, so they run with
	// explicit download approval exactly as a human would grant it.
	network := os.Getenv("FORGEFLOW_TEST_FIXTURE") != "" && os.Getenv("FORGEFLOW_TEST_FIXTURE") != "go"
	if _, e := s.Start(ctx, c.ID, true, network); e != nil {
		t.Fatal(e)
	}
	s.Wait()
	r, e := s.Store.Execution(ctx, c.ID)
	if e != nil || r.Status != "READY" {
		t.Fatalf("actual Codex workflow: %+v %v", r, e)
	}
	tests, _ := s.Store.Tests(ctx, c.ID)
	if len(tests) == 0 {
		t.Fatal("no real verification")
	}
	if r.Baseline == nil || r.Baseline.Status == "" || r.Baseline.Status == "unavailable" {
		t.Fatalf("baseline was not measured: %+v", r.Baseline)
	}
	if want := os.Getenv("FORGEFLOW_TEST_FIXTURE") == "python"; want != (len(r.Preexisting) > 0) {
		t.Fatalf("pre-existing failure handling: want disclosure=%v, got %+v", want, r.Preexisting)
	}
	agents, _ := s.Store.Agents(ctx)
	roles := map[string]int{}
	var input, output int64
	for _, a := range agents {
		if a.ContributionID == c.ID {
			roles[a.Role]++
			input += a.Usage["input_tokens"]
			output += a.Usage["output_tokens"]
		}
	}
	t.Logf("actual Codex READY: %d commands, %d fixes, %d reviews, baseline %s, %d pre-existing; sessions %v; observed tokens in/out %d/%d", len(tests), r.FixIterations, r.ReviewCycles, r.Baseline.Status, len(r.Preexisting), roles, input, output)
}

var realWorkflowFixtures = map[string]fixtureSpec{
	"":   {},
	"go": {},
	// A Python package with a real pytest dependency and an unrelated legacy
	// test that already fails on the base. The issue must be fixed; the legacy
	// failure must be disclosed rather than chased or reported as passing.
	"python": {Replace: true, Title: "add() subtracts its arguments", Body: "calc.arith.add(2, 3) returns -1 instead of 5. Fix add so it returns the sum and add regression tests for positive and negative operands. Tests use pytest (see pyproject.toml).", Files: map[string]string{
		"pyproject.toml":         "[project]\nname = \"calc\"\nversion = \"0.1.0\"\nrequires-python = \">=3.10\"\n\n[project.optional-dependencies]\ntest = [\"pytest==8.3.3\"]\n\n[tool.pytest.ini_options]\ntestpaths = [\"tests\", \"legacy\"]\n",
		"requirements-test.txt":  "pytest==8.3.3\n",
		"calc/__init__.py":       "",
		"calc/arith.py":          "def add(a, b):\n    return a - b\n",
		"tests/test_arith.py":    "from calc.arith import add\n\n\ndef test_add_positive():\n    assert add(2, 3) == 5\n",
		"legacy/test_reports.py": "def render_report():\n    return \"v1\"\n\n\ndef test_report_matches_v2_format():\n    # Known upstream failure tracked separately; unrelated to calc.arith.\n    assert render_report() == \"v2\"\n",
		"CONTRIBUTING.md":        "Install test dependencies with `pip install -r requirements-test.txt` and run `python -m pytest`.\n",
		".gitignore":             "__pycache__/\n.pytest_cache/\n/.forgeflow-runtime/\n",
	}},
	// A Gradle/JUnit project without a wrapper, like repositories that need
	// the environment to provision a JDK and a pinned Gradle distribution.
	"gradle": {Replace: true, Title: "Arith.add returns the difference", Body: "calc.Arith.add(2, 3) returns -1 instead of 5. Fix add and add JUnit regression tests. The project builds with Gradle 8.10.2 on Java 17 (see README).", Files: map[string]string{
		"settings.gradle":                   "rootProject.name = 'calc'\n",
		"build.gradle":                      "plugins { id 'java' }\n\njava { sourceCompatibility = JavaVersion.VERSION_17; targetCompatibility = JavaVersion.VERSION_17 }\n\nrepositories { mavenCentral() }\n\ndependencies { testImplementation 'junit:junit:4.13.2' }\n",
		"src/main/java/calc/Arith.java":     "package calc;\n\npublic final class Arith {\n    private Arith() {}\n\n    public static int add(int a, int b) {\n        return a - b;\n    }\n}\n",
		"src/test/java/calc/ArithTest.java": "package calc;\n\nimport static org.junit.Assert.assertEquals;\n\nimport org.junit.Test;\n\npublic class ArithTest {\n    @Test\n    public void addsPositiveNumbers() {\n        assertEquals(5, Arith.add(2, 3));\n    }\n}\n",
		"README.md":                         "Build and test with Gradle 8.10.2 on Java 17: `gradle test`. There is no Gradle wrapper.\n",
		".gitignore":                        "build/\n.gradle/\n/.forgeflow-runtime/\n",
	}},
}

type waitingRunner struct {
	fixtureRunner
	started chan struct{}
}

func TestAgentCannotChangeGitConfigForUnsandboxedHostOperations(t *testing.T) {
	s, c := buildFixture(t, &fixtureRunner{})
	repo, _, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.prepareRuntime(repo); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(repo, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("\n[filter \"unsafe\"]\n clean = arbitrary-command\n")
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.git(context.Background(), repo, "status", "--porcelain"); err == nil {
		t.Fatal("modified Git configuration was accepted")
	}
	if err = s.prepareRuntime(repo); err == nil {
		t.Fatal("resume replaced the pinned configuration snapshot")
	}
}

func (f *waitingRunner) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	close(f.started)
	<-ctx.Done()
	return codex.Result{}, ctx.Err()
}

func TestPausePreservesWorkspaceAndRequiresConfirmedResume(t *testing.T) {
	runner := &waitingRunner{started: make(chan struct{})}
	s, c := buildFixture(t, runner)
	ctx := context.Background()
	if err := s.SetConstraints(ctx, c.ID, "Preserve the public API", false); err == nil {
		t.Fatal("unconfirmed constraints accepted")
	}
	if err := s.SetConstraints(ctx, c.ID, "Preserve the public API", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-runner.started:
	case <-time.After(10 * time.Second):
		t.Fatal("agent did not start")
	}
	if err := s.SetConstraints(ctx, c.ID, "Change API", true); err == nil {
		t.Fatal("constraints changed during execution")
	}
	if err := s.Control(ctx, c.ID, "pause", false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	current, err := s.Store.Contribution(ctx, c.ID)
	if err != nil || current.State != "PAUSED" {
		t.Fatal("pause did not persist", current, err)
	}
	record, err := s.Store.Execution(ctx, c.ID)
	if err != nil || record.Constraints != "Preserve the public API" || record.Status != "PAUSED" {
		t.Fatal("constraints/evidence lost", record, err)
	}
	if _, err := s.Start(ctx, c.ID, false, false); err == nil {
		t.Fatal("unconfirmed resume allowed")
	}
	repo, _, err := s.Paths(current)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(repo, "sum.go")); err != nil {
		t.Fatal("workspace lost on pause", err)
	}
	if err = s.Store.SaveExecution(ctx, domain.ExecutionRecord{ContributionID: c.ID, Status: "AWAITING_PLAN_APPROVAL", Phase: "PLANNING", Plan: &domain.Plan{Summary: "Review first"}}, "test", "fixture plan"); err != nil {
		t.Fatal(err)
	}
	if err = s.Control(ctx, c.ID, "approve-plan", false); err == nil {
		t.Fatal("plan approval bypassed")
	}
	if err = s.Control(ctx, c.ID, "approve-plan", true); err != nil {
		t.Fatal(err)
	}
	record, err = s.Store.Execution(ctx, c.ID)
	if err != nil || !record.PlanApproved || record.Status != "PAUSED" {
		t.Fatal("approved plan cannot resume", record, err)
	}
}

type untestableModifiedRunner struct {
	fixtureRunner
}

func (u *untestableModifiedRunner) Run(ctx context.Context, r codex.Request, event func(json.RawMessage) error) (codex.Result, error) {
	u.roles = append(u.roles, r.Role)
	if r.Role == "reviewer" {
		u.reviews++
		b, _ := json.Marshal(domain.Review{Verdict: "APPROVE", Summary: "Approved"})
		return codex.Result{SessionID: storage.ID(), Output: string(b), Usage: map[string]int64{"input_tokens": 5, "output_tokens": 5}}, nil
	}
	return u.fixtureRunner.Run(ctx, r, event)
}

func TestCodeChangedAfterPassingTestsMustRetestBeforeReady(t *testing.T) {
	runner := &untestableModifiedRunner{}
	s, c := buildFixture(t, runner)
	ctx := context.Background()
	repo, _, err := s.Paths(c)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(repo, "sum.go"), []byte("package fixture\nfunc Add(a,b int) int {return a+b}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.Store.SaveTest(ctx, domain.TestRun{
		ID:             storage.ID(),
		ContributionID: c.ID,
		Command:        domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./..."}},
		Directory:      repo,
		ExitCode:       0,
		Output:         "PASS",
		StartedAt:      time.Now().UTC(),
		FinishedAt:     time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err = s.Store.SaveExecution(ctx, domain.ExecutionRecord{
		ContributionID: c.ID,
		Status:         "PAUSED",
		Phase:          "REVIEWING",
		PlanApproved:   true,
		Plan: &domain.Plan{
			Summary: "Fix Add",
			Tests:   []domain.VerificationCommand{{Program: "go", Arguments: []string{"test", "./..."}}},
		},
	}, "test", "paused before review finish"); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"PLANNING", "CODING", "TESTING", "REVIEWING", "PAUSED"} {
		if err = s.Store.Transition(ctx, c.ID, state); err != nil {
			t.Fatal(state, err)
		}
	}
	// Introduce broken code after the prior test pass.
	if err = os.WriteFile(filepath.Join(repo, "sum.go"), []byte("package fixture\nfunc Add(a,b int) int {return a-b}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runner.alwaysFail = true
	if _, err = s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, err := s.Store.Execution(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status == "READY" {
		t.Fatalf("modified broken code reached READY without passing tests: %+v", r)
	}
}

func TestPlannedSingleCheckPreservesDetectedTestLintAndBuild(t *testing.T) {
	repo := t.TempDir()
	pkg := `{"name":"fixture","scripts":{"test":"vitest run","lint":"eslint .","build":"tsc -b","typecheck":"tsc --noEmit"}}`
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(pkg), 0600); err != nil {
		t.Fatal(err)
	}
	plan := &domain.Plan{
		Summary: "Typecheck and fix",
		Tests:   []domain.VerificationCommand{{Program: "npm", Arguments: []string{"run", "typecheck"}}},
	}
	cmds, err := verificationCommands(repo, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 4 {
		t.Fatalf("expected all 4 verification commands to be preserved, got %d: %+v", len(cmds), cmds)
	}
	want := map[string]bool{
		"npm run typecheck": true,
		"npm run test":      true,
		"npm run lint":      true,
		"npm run build":     true,
	}
	for _, cmd := range cmds {
		key := strings.TrimSpace(cmd.Program + " " + strings.Join(cmd.Arguments, " "))
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing verification commands: %v (got %+v)", want, cmds)
	}
}

func TestConstraintUpdateInvalidatesPlanApproval(t *testing.T) {
	s, c := buildFixtureWithApproval(t, &fixtureRunner{}, "", true)
	ctx := context.Background()
	if err := s.Store.SaveExecution(ctx, domain.ExecutionRecord{
		ContributionID: c.ID,
		Status:         "PAUSED",
		Phase:          "CODING",
		Constraints:    "Initial constraint",
		PlanApproved:   true,
		Plan:           &domain.Plan{Summary: "Initial plan"},
	}, "test", "approved initial plan"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetConstraints(ctx, c.ID, "Updated constraint requiring new plan", true); err != nil {
		t.Fatal(err)
	}
	r, err := s.Store.Execution(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !r.PlanInvalidated || r.PlanApproved {
		t.Fatalf("expected PlanInvalidated=true and PlanApproved=false after constraint update, got %+v", r)
	}
}

type mutatingTestRunner struct {
	fixtureRunner
	runs int
}

func (m *mutatingTestRunner) Command(ctx context.Context, dir string, c domain.VerificationCommand, network bool) (domain.TestRun, error) {
	m.runs++
	_ = os.WriteFile(filepath.Join(dir, "sum.go"), []byte("package fixture\n// mutated during test\nfunc Add(a,b int) int {return a+b+"+strings.Repeat("0+", m.runs)+"0}\n"), 0600)
	return domain.TestRun{
		Command:    c,
		Directory:  dir,
		StartedAt:  time.Now().UTC(),
		FinishedAt: time.Now().UTC(),
		ExitCode:   0,
		Output:     "PASS",
	}, nil
}

func TestRepeatedTestMutationsStopInsteadOfLoopingForever(t *testing.T) {
	runner := &mutatingTestRunner{}
	s, c := buildFixture(t, runner)
	ctx := context.Background()
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, err := s.Store.Execution(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "BLOCKED" || r.Incident == nil || r.Incident.Kind != "verification_mutated_workspace" || r.MutatedTestRetries != 1 {
		t.Fatalf("expected bounded stop on repeated test mutations, got %+v", r)
	}
}

func TestFailedBatchSignatureIncludesAllFailedCommandsAndNormalizesDurations(t *testing.T) {
	batch1 := []domain.TestRun{
		{
			Command:     domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./pkg/a"}},
			ExitCode:    1,
			FailureKind: "code_failure",
			Output:      "--- FAIL: TestA (0.58s)\nFAIL\tpkg/a\t5.213s\n",
		},
		{
			Command:  domain.VerificationCommand{Program: "go", Arguments: []string{"vet", "./..."}},
			ExitCode: 0,
			Output:   "ok",
		},
	}
	batch2 := []domain.TestRun{
		{
			Command:     domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./pkg/a"}},
			ExitCode:    1,
			FailureKind: "code_failure",
			Output:      "--- FAIL: TestA (1.42s)\nFAIL\tpkg/a\t8.004s\n",
		},
		{
			Command:  domain.VerificationCommand{Program: "go", Arguments: []string{"vet", "./..."}},
			ExitCode: 0,
			Output:   "ok",
		},
	}
	batch3 := []domain.TestRun{
		{
			Command:     domain.VerificationCommand{Program: "go", Arguments: []string{"test", "./pkg/b"}},
			ExitCode:    1,
			FailureKind: "code_failure",
			Output:      "--- FAIL: TestA (0.58s)\nFAIL\tpkg/b\t5.213s\n",
		},
		{
			Command:  domain.VerificationCommand{Program: "go", Arguments: []string{"vet", "./..."}},
			ExitCode: 0,
			Output:   "ok",
		},
	}
	sig1 := failedBatchSignature(batch1)
	sig2 := failedBatchSignature(batch2)
	sig3 := failedBatchSignature(batch3)
	if sig1 == "" || sig1 != sig2 {
		t.Fatalf("expected normalized durations to produce identical batch signature: %q vs %q", sig1, sig2)
	}
	if sig1 == sig3 {
		t.Fatalf("expected different failed command arguments/output to produce distinct batch signatures: %q vs %q", sig1, sig3)
	}
}

func TestUnquotedDependencyFailureInFailingTestClassifiesAsDependenciesMissing(t *testing.T) {
	cmd := domain.VerificationCommand{Program: "python3", Arguments: []string{"-m", "unittest"}}
	d := diagnoseCommand(cmd, "--- FAIL: TestImport (0.01s)\nModuleNotFoundError: No module named 'pytest'", errors.New("exit status 1"), true)
	if d.Kind != "dependencies_missing" {
		t.Fatalf("expected dependencies_missing for unquoted ModuleNotFoundError inside test, got %+v", d)
	}
	quoted := diagnoseCommand(cmd, "--- FAIL: TestGit: unexpected fatal: detected dubious ownership in repository", errors.New("exit status 1"), true)
	if quoted.Kind != "code_failure" {
		t.Fatalf("expected code_failure for quoted assertion error, got %+v", quoted)
	}
}
