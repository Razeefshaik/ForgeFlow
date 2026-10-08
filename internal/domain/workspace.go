package domain

import (
	"encoding/json"
	"time"
)

// WorkspaceApproval is an immutable, human-approved execution input.
type WorkspaceApproval struct {
	Token         string          `json:"token"`
	WorkspaceRoot string          `json:"workspace_root"`
	Opportunity   Opportunity     `json:"opportunity"`
	Config        ConfigVersion   `json:"configuration"`
	Issue         json.RawMessage `json:"issue"`
	Repository    json.RawMessage `json:"repository"`
	Competition   json.RawMessage `json:"competition"`
	BaseCommit    string          `json:"base_commit"`
	CodexModel    string          `json:"codex_model,omitempty"`
	Warnings      []string        `json:"warnings"`
	CheckedAt     time.Time       `json:"checked_at"`
}
type CommandRecord struct {
	Arguments  []string  `json:"arguments"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	ExitCode   int       `json:"exit_code"`
	Output     string    `json:"output"`
	Truncated  bool      `json:"truncated"`
}
