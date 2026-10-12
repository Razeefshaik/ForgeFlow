package domain

import "time"

type VerificationCommand struct {
	Program        string   `json:"program"`
	Arguments      []string `json:"arguments"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"`
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
	Model          string           `json:"model,omitempty"`
	Status         string           `json:"status"`
	SessionID      string           `json:"session_id"`
	StartedAt      time.Time        `json:"started_at"`
	FinishedAt     *time.Time       `json:"finished_at"`
	Output         string           `json:"output"`
	Error          string           `json:"error,omitempty"`
	Usage          map[string]int64 `json:"usage,omitempty"`
}
type TestRun struct {
	ID              string               `json:"id"`
	ContributionID  string               `json:"contribution_id"`
	Command         VerificationCommand  `json:"command"`
	Directory       string               `json:"directory"`
	StartedAt       time.Time            `json:"started_at"`
	FinishedAt      time.Time            `json:"finished_at"`
	ExitCode        int                  `json:"exit_code"`
	Output          string               `json:"output"`
	Truncated       bool                 `json:"truncated"`
	FailureKind     string               `json:"failure_kind,omitempty"`
	RecoveryOf      string               `json:"recovery_of,omitempty"`
	Error           string               `json:"error,omitempty"`
	ExecutedCommand *VerificationCommand `json:"executed_command,omitempty"`
}

// ExecutionIncident is the current diagnosis. Historical diagnoses and actions
// live in immutable events; a resolved incident is removed from current status.
type ExecutionIncident struct {
	ID               string               `json:"id"`
	Kind             string               `json:"kind"`
	Source           string               `json:"source,omitempty"`
	Summary          string               `json:"summary"`
	Evidence         string               `json:"evidence"`
	NextAction       string               `json:"next_action"`
	Status           string               `json:"status"`
	TestRunID        string               `json:"test_run_id,omitempty"`
	Command          *VerificationCommand `json:"command,omitempty"`
	Attempt          int                  `json:"attempt"`
	CanRecover       bool                 `json:"can_recover"`
	RetryAt          *time.Time           `json:"retry_at,omitempty"`
	Retryable        bool                 `json:"retryable,omitempty"`
	DetectedAt       time.Time            `json:"detected_at"`
	DiagnosisVersion int                  `json:"diagnosis_version,omitempty"`
	Path             string               `json:"path,omitempty"`
	SecondaryKinds   []string             `json:"secondary_kinds,omitempty"`
	FailedTests      []string             `json:"failed_tests,omitempty"`
}
type ExecutionRecord struct {
	IssueDiscussion  []IssueComment        `json:"issue_discussion,omitempty"`
	PlanContextHash  string                `json:"plan_context_hash,omitempty"`
	PlanInvalidated  bool                  `json:"plan_invalidated,omitempty"`
	ReviewPatchHash  string                `json:"review_patch_hash,omitempty"`
	Environment      *ExecutionEnvironment `json:"environment,omitempty"`
	EnvironmentPlan  *EnvironmentPlan      `json:"environment_plan,omitempty"`
	ContributionID   string                `json:"contribution_id"`
	Status           string                `json:"status"`
	Phase            string                `json:"phase"`
	Plan             *Plan                 `json:"plan,omitempty"`
	Review           *Review               `json:"review,omitempty"`
	Summary          string                `json:"summary"`
	Message          string                `json:"message,omitempty"`
	FixIterations    int                   `json:"fix_iterations"`
	ReviewCycles     int                   `json:"review_cycles"`
	PlanApproved     bool                  `json:"plan_approved"`
	Network          bool                  `json:"network"`
	Incident         *ExecutionIncident    `json:"incident,omitempty"`
	RecoveryAttempts int                   `json:"recovery_attempts"`
	Constraints      string                `json:"constraints"`
	UpdatedAt        time.Time             `json:"updated_at"`
	ChangedFiles     []string              `json:"changed_files"`
	Diff             string                `json:"diff"`
	Report           string                `json:"report"`
	PRTitle          string                `json:"pr_title"`
	PRRevision       string                `json:"pr_revision"`
	PRBody           string                `json:"pr_body"`
	PRURL            string                `json:"pr_url"`
	HeadCommit       string                `json:"head_commit"`
	SubmissionToken  string                `json:"submission_token"`
}

// ExecutionEnvironment is observed control-plane evidence, not an agent's claim.
type ExecutionEnvironment struct {
	Status       string            `json:"status"`
	Fingerprint  string            `json:"fingerprint,omitempty"`
	BaseImage    string            `json:"base_image,omitempty"`
	Image        string            `json:"image,omitempty"`
	Tools        map[string]string `json:"tools,omitempty"`
	Network      bool              `json:"downloads_approved"`
	Capabilities []string          `json:"capabilities"`
	Checks       []TestRun         `json:"checks,omitempty"`
	Actions      []string          `json:"actions,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// EnvironmentPlan is a declarative request. Commands execute only in isolated
// image builds or the contribution's verification container, never on the host.
type EnvironmentPlan struct {
	Summary           string                `json:"summary"`
	BaseImage         string                `json:"base_image"`
	Files             []string              `json:"files"`
	ImageCommands     []VerificationCommand `json:"image_commands"`
	WorkspaceCommands []VerificationCommand `json:"workspace_commands"`
	HookCommands      []VerificationCommand `json:"hook_commands,omitempty"`
	CompletionGates   []CompletionGate      `json:"completion_gates,omitempty"`
	Checks            []VerificationCommand `json:"checks"`
	Environment       []EnvironmentVariable `json:"environment"`
	Requirements      []string              `json:"requirements"`
	Blockers          []string              `json:"blockers"`
}

// CompletionGate is a public verification of an externally satisfied requirement.
// Its private credentials must never be supplied to contribution processes.
type CompletionGate struct {
	Requirement string                `json:"requirement"`
	Source      string                `json:"source"`
	Action      string                `json:"action"`
	Binding     string                `json:"binding"`
	Checks      []VerificationCommand `json:"checks"`
}

type IssueComment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	URL       string    `json:"html_url"`
	UpdatedAt time.Time `json:"updated_at"`
}

type EnvironmentVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
