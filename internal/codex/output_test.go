package codex

import (
	"strings"
	"testing"
)

func TestBoundedOutputPreservesFinalFailureAfterVerboseLogs(t *testing.T) {
	out := &bounded{limit: 1024}
	_, _ = out.Write([]byte("command started\n" + strings.Repeat("build output\n", 300)))
	_, _ = out.Write([]byte("--- FAIL: TestRegression\npanic: test timed out after 10m0s\n"))
	got := out.String()
	if !strings.HasPrefix(got, "command started") || !strings.Contains(got, "--- FAIL: TestRegression") || !strings.Contains(got, "panic: test timed out") || !out.truncated || len(got) > 1100 {
		t.Fatal("lost bounded causal evidence", len(got), got)
	}
}
