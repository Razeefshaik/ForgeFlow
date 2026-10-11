package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"regexp"
	"strings"
)

const diagnosisVersion = 3

var goAssertion = regexp.MustCompile(`(?m)^\s*--- FAIL: (Test[A-Za-z0-9_]+)(?:[/ (]|$)`)

func failedGoTests(command domain.VerificationCommand, output string) []string {
	if command.Program != "go" || len(command.Arguments) == 0 || command.Arguments[0] != "test" {
		return nil
	}
	seen := map[string]bool{}
	var tests []string
	for _, match := range goAssertion.FindAllStringSubmatch(output, 12) {
		if !seen[match[1]] {
			tests = append(tests, match[1])
			seen[match[1]] = true
		}
	}
	return tests
}

// Match the Go package loader's own diagnostic, not a path mentioned in a test.
var deniedGoPackage = regexp.MustCompile(`(?mi)^pattern [^\r\n]+: open (.+): (?:Access is denied\.|permission denied)\s*$`)

func deniedPackagePath(command domain.VerificationCommand, output string) string {
	if command.Program != "go" {
		return ""
	}
	m := deniedGoPackage.FindStringSubmatch(output)
	if len(m) != 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func sandboxProcessFailure(text string) bool {
	return hasAny(text, "couldn't create signal pipe", "could not create signal pipe", "cygheap base mismatch", "child_copy: linked dll data write copy failed", "fork: resource temporarily unavailable", "ntcreatedirectoryobject(")
}

func savedCommandError(run domain.TestRun) error {
	if run.Error == context.DeadlineExceeded.Error() {
		return context.DeadlineExceeded
	}
	if run.Error == context.Canceled.Error() {
		return context.Canceled
	}
	if run.Error != "" {
		return errors.New(run.Error)
	}
	return nil
}
