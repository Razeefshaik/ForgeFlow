package execution

import (
	"encoding/json"
	"errors"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const maxTestEvidenceBytes = 64 << 10
const maxAgentPromptBytes = 256 << 10

type testEvidenceEntry struct {
	ID              string    `json:"id"`
	Command         string    `json:"command"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	ExitCode        int       `json:"exit_code"`
	PriorRuns       int       `json:"prior_runs"`
	OutputBytes     int       `json:"saved_output_bytes"`
	SourceTruncated bool      `json:"source_output_truncated"`
	Excerpt         string    `json:"output_excerpt"`
}

func testPromptEvidence(repo string, tests []domain.TestRun) (string, error) {
	data, err := json.Marshal(tests)
	if err != nil {
		return "", err
	}
	snapshot, err := writePromptArtifact(repo, "tests-", ".json", data)
	if err != nil {
		return "", err
	}
	counts := map[string]int{}
	for _, run := range tests {
		key, _ := json.Marshal(run.Command)
		counts[string(key)]++
	}
	seen := map[string]bool{}
	entries := []testEvidenceEntry{}
	for i := len(tests) - 1; i >= 0; i-- {
		run := tests[i]
		key, _ := json.Marshal(run.Command)
		if seen[string(key)] {
			continue
		}
		seen[string(key)] = true
		if len(entries) == 12 {
			continue
		}
		limit := 1536
		if run.ExitCode != 0 {
			limit = 4096
		}
		entries = append(entries, testEvidenceEntry{ID: promptExcerpt(run.ID, 128), Command: promptExcerpt(string(key), 2048), StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, ExitCode: run.ExitCode, PriorRuns: counts[string(key)] - 1, OutputBytes: len(run.Output), SourceTruncated: run.Truncated, Excerpt: promptExcerpt(run.Output, limit)})
	}
	for {
		summary := struct {
			Snapshot        string              `json:"saved_records_path"`
			Note            string              `json:"note"`
			TotalRuns       int                 `json:"total_runs"`
			OmittedCommands int                 `json:"omitted_commands"`
			Latest          []testEvidenceEntry `json:"latest_per_command_newest_first"`
		}{snapshot, "Excerpts only. All saved historical records are preserved in the workspace snapshot and audit store. Filter the snapshot by run ID and read relevant output in bounded chunks before approving; do not dump the entire history. source_output_truncated means the original runner did not retain all output. Older failures do not describe the latest run of the same command.", len(tests), len(counts) - len(entries), entries}
		encoded, e := json.Marshal(summary)
		if e != nil {
			return "", e
		}
		if len(encoded) <= maxTestEvidenceBytes {
			return string(encoded), nil
		}
		if len(entries) == 0 {
			return "", errors.New("test evidence metadata exceeds inline budget")
		}
		entries = entries[:len(entries)-1]
	}
}

func boundedAgentPrompt(repo, prompt string) (string, string, error) {
	if len(prompt) <= maxAgentPromptBytes {
		return prompt, "", nil
	}
	snapshot, err := writePromptArtifact(repo, "prompt-", ".txt", []byte(prompt))
	if err != nil {
		return "", "", err
	}
	header := "The complete task input is saved at " + snapshot + ". This inline request is an excerpt. Read the complete saved task before acting; do not infer missing instructions or verification.\n\n"
	return header + promptExcerpt(prompt, maxAgentPromptBytes-len(header)), snapshot, nil
}

// Keep the beginning and end, mark omissions, and never split a UTF-8 character.
func promptExcerpt(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	marker := "\n...[excerpt omitted; consult saved evidence]...\n"
	if limit <= len(marker) {
		return utf8Prefix(value, limit)
	}
	remaining := limit - len(marker)
	head := utf8Prefix(value, remaining/3)
	start := len(value) - (remaining - remaining/3)
	for start < len(value) && !utf8.RuneStart(value[start]) {
		start++
	}
	return head + marker + value[start:]
}

func utf8Prefix(value string, limit int) string {
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}

func writePromptArtifact(repo, prefix, suffix string, data []byte) (string, error) {
	dir := repo
	for _, part := range []string{".forgeflow-runtime", "evidence"} {
		dir = filepath.Join(dir, part)
		if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("prompt evidence directory must stay inside the workspace")
		}
	}
	name := prefix + storage.ID() + suffix
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	_, err = f.Write(data)
	closeErr := f.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return strings.Join([]string{".forgeflow-runtime", "evidence", name}, "/"), nil
}
