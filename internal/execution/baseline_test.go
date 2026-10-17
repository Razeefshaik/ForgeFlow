package execution

import (
	"context"
	"forgeflow/internal/domain"
	"reflect"
	"strings"
	"testing"
)

func TestFailedTestNamesAcrossRunners(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		want         []string
	}{
		{"go", "=== RUN   TestA\n--- FAIL: TestA (0.00s)\n    a_test.go:3: boom\n--- FAIL: TestB/sub (0.01s)\nFAIL\nFAIL\texample.com/m\t0.2s\n", []string{"go:TestA", "go:TestB/sub"}},
		{"pytest", "=========================== short test summary info ============================\nFAILED tests/test_a.py::test_x - AssertionError: no\nERROR tests/test_b.py::test_y - fixture missing\n", []string{"pytest:tests/test_a.py::test_x", "pytest:tests/test_b.py::test_y"}},
		{"unittest", "FAIL: test_x (pkg.tests.TestCase.test_x)\nERROR: test_y (pkg.tests.TestCase)\n", []string{"unittest:pkg.tests.TestCase.test_x", "unittest:pkg.tests.TestCase.test_y"}},
		{"gradle", "com.example.FooTest > handlesNull() FAILED\n    java.lang.AssertionError at FooTest.java:12\n> Task :test FAILED\n", []string{"junit:com.example.FooTest > handlesNull()"}},
		{"surefire3", "[ERROR] com.example.BarTest.parses -- Time elapsed: 0.012 s <<< FAILURE!\n", []string{"junit:com.example.BarTest.parses"}},
		{"surefire2", "[ERROR] parses(com.example.BarTest)  Time elapsed: 0.01 s  <<< ERROR!\n", []string{"junit:com.example.BarTest.parses"}},
		{"cargo", "test parser::tests::empty ... FAILED\n", []string{"cargo:parser::tests::empty"}},
		{"jest", "  ● Parser › rejects empty input\n", []string{"jest:Parser › rejects empty input"}},
		{"tap", "not ok 3 - rejects empty input\n", []string{"tap:rejects empty input"}},
	} {
		got, ok := failedTestNames(tc.output)
		if !ok || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v %v, want %v", tc.name, got, ok, tc.want)
		}
	}
	if names, ok := failedTestNames("BUILD FAILED\nerror: cannot find symbol"); ok || len(names) != 0 {
		t.Fatal("build failure parsed as test failures", names)
	}
}

func TestPreexistingReasonRequiresProof(t *testing.T) {
	run := func(exit int, output string) domain.TestRun { return domain.TestRun{ExitCode: exit, Output: output} }
	baseOut := "--- FAIL: TestFlaky (0.00s)\n--- FAIL: TestNetwork (0.00s)\n"
	base := domain.BaselineCommand{ExitCode: 1, FailedTests: []string{"go:TestFlaky", "go:TestNetwork"}}
	if preexistingReason(base, run(1, baseOut), run(1, "--- FAIL: TestFlaky (0.00s)\n")) == "" {
		t.Fatal("subset of base failures rejected")
	}
	if preexistingReason(base, run(1, baseOut), run(1, "--- FAIL: TestFlaky (0.00s)\n--- FAIL: TestNew (0.00s)\n")) != "" {
		t.Fatal("a new failing test was accepted")
	}
	if preexistingReason(base, run(1, baseOut), run(2, "panic: build exploded")) != "" {
		t.Fatal("unidentifiable post-change failure accepted against identified base failures")
	}
	if preexistingReason(domain.BaselineCommand{Passed: true}, run(0, ""), run(1, "--- FAIL: TestFlaky")) != "" {
		t.Fatal("failure accepted although the base passed")
	}
	noTests := domain.BaselineCommand{ExitCode: 5, NoTests: true}
	if preexistingReason(noTests, run(5, "collected 0 items\nno tests ran in 0.01s"), run(5, "collected 0 items\nno tests ran in 0.02s")) == "" {
		t.Fatal("identical empty collection rejected")
	}
	lint := "src/a.js:10:5 error 'x' is defined but never used no-unused-vars\n1 problem (1 error, 0 warnings)\n"
	unparsed := domain.BaselineCommand{ExitCode: 1}
	if preexistingReason(unparsed, run(1, lint+"Done in 1.2s\n"), run(1, lint+"Done in 3.4s\n")) == "" {
		t.Fatal("identical lint debt rejected")
	}
	if preexistingReason(unparsed, run(1, lint), run(1, lint+"src/b.js:2:1 error 'y' is not defined no-undef\n")) != "" {
		t.Fatal("a new lint error was accepted")
	}
	compile := "src/Foo.java:3: error: cannot find symbol\nBUILD FAILED in 2s\n"
	if preexistingReason(unparsed, run(1, compile), run(1, compile)) != "" {
		t.Fatal("a build that never reached its checks was accepted as verified")
	}
}

func TestRelatedFailuresAreNeverAccepted(t *testing.T) {
	if !relatedToPatch([]string{"go:TestParseHeader"}, "", "Issue: TestParseHeader fails on empty input", "", nil) {
		t.Fatal("failure named by the issue was treated as unrelated")
	}
	if relatedToPatch([]string{"pytest:legacy/test_report.py::test_parse"}, "", "+def test_parse_empty():\n+    assert parse('') == []", "", []string{"calc/parse.py"}) {
		t.Fatal("a longer new test name matched an unrelated failing test")
	}
	output := "--- FAIL: TestOther (0.00s)\nFAIL\texample.com/m/internal/parse\t0.1s\n"
	if !relatedToPatch([]string{"go:TestOther"}, output, "", "example.com/m", []string{"internal/parse/header.go"}) {
		t.Fatal("failure in the edited Go package was treated as unrelated")
	}
	if relatedToPatch([]string{"go:TestOther"}, output, "", "example.com/m", []string{"cmd/tool/main.go"}) {
		t.Fatal("failure in an untouched package was treated as related")
	}
	if !relatedToPatch([]string{"junit:com.example.parse.HeaderTest > rejects()"}, "", "", "", []string{"src/main/java/com/example/parse/Header.java"}) {
		t.Fatal("failure in the edited Java package was treated as unrelated")
	}
	if !relatedToPatch([]string{"pytest:pkg/test_header.py::test_empty"}, "", "", "", []string{"pkg/header.py"}) {
		t.Fatal("failure beside the edited Python module was treated as unrelated")
	}
}

// An unrelated upstream failure used to send the contributor into fix loops
// until the iteration limit. With a measured baseline, the issue is fixed,
// reviewed and disclosed instead; the failure is never reported as a pass.
func TestUnrelatedPreexistingFailureReachesReadyWithDisclosure(t *testing.T) {
	runner := &fixtureRunner{}
	s, c := buildFixtureWithFiles(t, runner, "", false, map[string]string{
		"other/other.go":      "package other\n",
		"other/other_test.go": "package other\nimport \"testing\"\nfunc TestUpstreamFlaky(t *testing.T){t.Fatal(\"upstream failure unrelated to Add\")}\n",
	})
	ctx := context.Background()
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	current, _ := s.Store.Contribution(ctx, c.ID)
	r, err := s.Store.Execution(ctx, c.ID)
	if err != nil || current.State != "READY" {
		t.Fatalf("contribution did not reach READY: %s %+v %v", current.State, r, err)
	}
	if r.Baseline == nil || r.Baseline.Status != "preexisting_failures" || len(r.Baseline.Commands) != 2 {
		t.Fatalf("baseline not recorded: %+v", r.Baseline)
	}
	if len(r.Preexisting) != 1 || !reflect.DeepEqual(r.Preexisting[0].FailedTests, []string{"go:TestUpstreamFlaky"}) {
		t.Fatalf("pre-existing failure not disclosed: %+v", r.Preexisting)
	}
	tests, _ := s.Store.Tests(ctx, c.ID)
	baselineRuns, lastGoTest := 0, domain.TestRun{}
	for _, run := range tests {
		if run.Baseline {
			baselineRuns++
		} else if run.Command.Program == "go" && run.Command.Arguments[0] == "test" {
			lastGoTest = run
		}
	}
	if baselineRuns != 2 || lastGoTest.ExitCode == 0 {
		t.Fatalf("real exit codes were not preserved: baseline=%d last=%+v", baselineRuns, lastGoTest)
	}
	// The issue's own failing test (TestAdd, in the edited package) still had
	// to be fixed through the normal loop.
	if r.FixIterations < 1 || !strings.Contains(r.Diff, "a+b") {
		t.Fatalf("issue-related failure was accepted instead of fixed: fixes=%d diff=%s", r.FixIterations, r.Diff)
	}
	for _, text := range []string{r.Report, r.PRBody} {
		if !strings.Contains(text, "TestUpstreamFlaky") {
			t.Fatalf("disclosure missing:\n%s", text)
		}
	}
	if !strings.Contains(r.PRBody, "unmodified base commit") || !strings.Contains(r.Report, "Pre-existing failures (not counted as passes)") {
		t.Fatalf("disclosure wording missing:\nPR:\n%s\nReport:\n%s", r.PRBody, r.Report)
	}
}

func TestIssueFailureOnBaseIsStillFixed(t *testing.T) {
	// TestAdd fails on the base and is exactly what the issue is about; it sits
	// in the edited package, so acceptance must be refused.
	runner := &fixtureRunner{}
	s, c := buildFixture(t, runner)
	ctx := context.Background()
	if _, err := s.Start(ctx, c.ID, true, false); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	r, _ := s.Store.Execution(ctx, c.ID)
	if r.Status != "READY" || len(r.Preexisting) != 0 || r.FixIterations != 2 {
		t.Fatalf("issue failure accepted as pre-existing: status=%s preexisting=%+v fixes=%d", r.Status, r.Preexisting, r.FixIterations)
	}
	events, _ := s.Store.Events(ctx, 0, c.ID, 1000)
	rejected := false
	for _, event := range events {
		if event.Type == "PreexistingFailureRejected" {
			rejected = true
		}
	}
	if !rejected {
		t.Fatal("rejection of a related failure was not audited")
	}
}
