package execution

import (
	"context"
	"errors"
	"forgeflow/internal/domain"
	"regexp"
	"strings"
)

const diagnosisVersion = 2

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
