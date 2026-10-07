package domain

import "time"

type VerificationCommand struct {
	Program   string   `json:"program"`
	Arguments []string `json:"arguments"`
}
type Plan struct {
	Summary   string                `json:"summary"`
	RootCause string                `json:"root_cause"`
	Files     []string              `json:"files"`
	Strategy  string                `json:"strategy"`
	Tests     []VerificationCommand `json:"tests"`
	Risks     []string              `json:"risks"`
	Unknowns  []string              `json:"unknowns"`
}
type Finding struct {
	Severity    string `json:"severity"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Explanation string `json:"explanation"`
	Fix         string `json:"recommended_fix"`
}
type Review struct {
	Verdict  string    `json:"verdict"`
	Summary  string    `json:"summary"`
	Findings []Finding `json:"findings"`
}
type AgentRun struct {
	ID             string           `json:"id"`
	ContributionID string           `json:"contribution_id"`
	Role           string           `json:"role"`
	Status         string           `json:"status"`
	SessionID      string           `json:"session_id"`
	StartedAt      time.Time        `json:"started_at"`
	FinishedAt     *time.Time       `json:"finished_at"`
	Output         string           `json:"output"`
	Usage          map[string]int64 `json:"usage,omitempty"`
}
type TestRun struct {
	ID             string              `json:"id"`
	ContributionID string              `json:"contribution_id"`
	Command        VerificationCommand `json:"command"`
	Directory      string              `json:"directory"`
	StartedAt      time.Time           `json:"started_at"`
	FinishedAt     time.Time           `json:"finished_at"`
	ExitCode       int                 `json:"exit_code"`
	Output         string              `json:"output"`
	Truncated      bool                `json:"truncated"`
}
type ExecutionRecord struct {
	ContributionID  string    `json:"contribution_id"`
	Status          string    `json:"status"`
	Phase           string    `json:"phase"`
	Plan            *Plan     `json:"plan,omitempty"`
	Review          *Review   `json:"review,omitempty"`
	Summary         string    `json:"summary"`
	Message         string    `json:"message,omitempty"`
	FixIterations   int       `json:"fix_iterations"`
	ReviewCycles    int       `json:"review_cycles"`
	PlanApproved    bool      `json:"plan_approved"`
	Network         bool      `json:"network"`
	Constraints     string    `json:"constraints"`
	UpdatedAt       time.Time `json:"updated_at"`
	ChangedFiles    []string  `json:"changed_files"`
	Diff            string    `json:"diff"`
	Report          string    `json:"report"`
	PRTitle         string    `json:"pr_title"`
	PRBody          string    `json:"pr_body"`
	PRURL           string    `json:"pr_url"`
	HeadCommit      string    `json:"head_commit"`
	SubmissionToken string    `json:"submission_token"`
}
