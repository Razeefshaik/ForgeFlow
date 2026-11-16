package execution

import (
	"forgeflow/internal/domain"
	"testing"
)

// A real Python run on 2026-11-16: the plan ran "<venv>/bin/python -m pytest"
// and the detector added "python -m pytest" on the system interpreter, which
// has no pytest. The patch was right and the fix budget was exhausted anyway.
func TestDetectedCommandRunThroughThePreparedEnvironmentIsNotRepeated(t *testing.T) {
	detected := domain.VerificationCommand{Program: "python", Arguments: []string{"-m", "pytest"}}
	for name, tc := range map[string]struct {
		planned domain.VerificationCommand
		want    bool
	}{
		"venv interpreter":    {domain.VerificationCommand{Program: "/workspace/.forgeflow-runtime/linux/venv/bin/python", Arguments: []string{"-m", "pytest"}}, true},
		"versioned python":    {domain.VerificationCommand{Program: "python3.12", Arguments: []string{"-m", "pytest"}}, true},
		"different arguments": {domain.VerificationCommand{Program: "/venv/bin/python", Arguments: []string{"-m", "pytest", "tests/unit"}}, false},
		"different tool":      {domain.VerificationCommand{Program: "/venv/bin/tox", Arguments: []string{"-m", "pytest"}}, false},
		"the same command":    {detected, false},
	} {
		if got := subsumedByPlan(detected, []domain.VerificationCommand{tc.planned}); got != tc.want {
			t.Errorf("%s: subsumed=%v, want %v", name, got, tc.want)
		}
	}
	if !sameTool("/opt/gradle/bin/gradle", "gradle") || sameTool("python", "pypy") {
		t.Fatal("tool identity is wrong")
	}
}

func TestCommandThatCannotStartIsRecognized(t *testing.T) {
	header := "Docker container forgeflow-verify-83c2aa7b; image sha256:47fe; working directory /workspace\n"
	missing := domain.TestRun{ExitCode: 1, Output: header + "/usr/bin/python: No module named pytest\n"}
	same := domain.TestRun{ExitCode: 1, Output: "Docker container forgeflow-verify-6804960510c1; image sha256:47fe; working directory /workspace\n/usr/bin/python: No module named pytest\n"}
	if unrunnable(missing) == "" || unrunnable(missing) != unrunnable(same) {
		t.Fatal("a missing module was not recognized across two runs")
	}
	for name, run := range map[string]domain.TestRun{
		"passing":       {ExitCode: 0, Output: header + "ok"},
		"real failures": {ExitCode: 1, Output: header + "FAILED tests/test_a.py::test_x - assert 1 == 2\nImportError: No module named foo\n1 failed, 3 passed\nmore\n"},
		"plain failure": {ExitCode: 1, Output: header + "assertion failed\n"},
	} {
		if unrunnable(run) != "" {
			t.Errorf("%s was treated as unable to start", name)
		}
	}
	base := domain.BaselineCommand{ExitCode: 1}
	if preexistingReason(base, missing, same) == "" {
		t.Fatal("identical failure to start on base and after the change was not recognized")
	}
	// A command that started on the base but cannot start after the change is a regression.
	other := domain.TestRun{ExitCode: 1, Output: header + "/usr/bin/python: No module named calc\n"}
	if preexistingReason(base, missing, other) != "" {
		t.Fatal("a different failure to start was accepted")
	}
}
