package execution

import (
	"encoding/json"
	"forgeflow/internal/domain"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPromptEvidencePreservesLargeHistoryAndUsesLatestOutcomes(t *testing.T) {
	repo := t.TempDir()
	tests := []domain.TestRun{}
	for i := 0; i < 24; i++ {
		tests = append(tests, domain.TestRun{ID: strings.Repeat("x", i+1), Command: domain.VerificationCommand{Program: "./gradlew", Arguments: []string{"JUnitQuick"}}, ExitCode: 1, Output: "STALE_FAILURE\n" + strings.Repeat("<>&\n", 12000)})
	}
	tests = append(tests, domain.TestRun{ID: "current", Command: tests[0].Command, ExitCode: 0, Output: "PASSED_NEWEST\n" + strings.Repeat("<>&\n", 12000) + "\nBUILD SUCCESSFUL", Truncated: true})
	prompt, err := testPromptEvidence(repo, tests)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompt) > maxTestEvidenceBytes || strings.Contains(prompt, "STALE_FAILURE") {
		t.Fatal("historical logs leaked into inline evidence")
	}
	var summary struct {
		Snapshot string              `json:"saved_records_path"`
		Total    int                 `json:"total_runs"`
		Latest   []testEvidenceEntry `json:"latest_per_command_newest_first"`
	}
	if err := json.Unmarshal([]byte(prompt), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Total != 25 || len(summary.Latest) != 1 || summary.Latest[0].ID != "current" || summary.Latest[0].ExitCode != 0 || summary.Latest[0].PriorRuns != 24 || !summary.Latest[0].SourceTruncated || !strings.Contains(summary.Latest[0].Excerpt, "BUILD SUCCESSFUL") {
		t.Fatalf("latest status or original truncation metadata lost: %+v", summary)
	}
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(summary.Snapshot)))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 1<<20 {
		t.Fatal("fixture did not reproduce oversized JSON evidence")
	}
	var saved []domain.TestRun
	if err := json.Unmarshal(data, &saved); err != nil || !reflect.DeepEqual(saved, tests) {
		t.Fatal("saved evidence was lost or modified")
	}
}

func TestWholePromptBudgetPreservesUnicodeAndCompleteRequest(t *testing.T) {
	repo := t.TempDir()
	request := "IMPORTANT START\n" + strings.Repeat("😀<>&\n", 100000) + "\nIMPORTANT END"
	inline, snapshot, err := boundedAgentPrompt(repo, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(inline) > maxAgentPromptBytes || !utf8.ValidString(inline) || !strings.Contains(inline, "IMPORTANT START") || !strings.Contains(inline, "IMPORTANT END") || !strings.Contains(inline, snapshot) {
		t.Fatal("prompt limit lost instructions, file reference or Unicode boundaries")
	}
	saved, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(snapshot)))
	if err != nil || string(saved) != request {
		t.Fatal("complete original request was not preserved")
	}
	inline, snapshot, err = boundedAgentPrompt(repo, "small unchanged prompt")
	if err != nil || snapshot != "" || inline != "small unchanged prompt" {
		t.Fatal("small requests changed unnecessarily")
	}
}

func TestEvidenceBudgetIncludesJSONEscapingAndCommandMetadata(t *testing.T) {
	tests := []domain.TestRun{}
	for i := 0; i < 30; i++ {
		tests = append(tests, domain.TestRun{ID: strings.Repeat("n", i+1), Command: domain.VerificationCommand{Program: "./gradlew", Arguments: []string{strings.Repeat("<&>", 3000), strings.Repeat("a", i+1)}}, ExitCode: 1, Output: strings.Repeat("<&>\n", 10000)})
	}
	inline, err := testPromptEvidence(t.TempDir(), tests)
	if err != nil || len(inline) > maxTestEvidenceBytes || !json.Valid([]byte(inline)) {
		t.Fatalf("escaped evidence exceeds budget: bytes=%d err=%v", len(inline), err)
	}
	var summary struct {
		Omitted int                 `json:"omitted_commands"`
		Latest  []testEvidenceEntry `json:"latest_per_command_newest_first"`
	}
	json.Unmarshal([]byte(inline), &summary)
	if len(summary.Latest) == 0 || summary.Latest[0].ID != tests[len(tests)-1].ID || summary.Omitted+len(summary.Latest) != 30 {
		t.Fatal("newest evidence or omitted-history accounting was lost")
	}
}
