package domain

import "time"

// RepositoryExclusion records discovery-time requirements, not a quality score
// or a diagnosis inferred from earlier contributions.
type RepositoryExclusion struct {
	Repository     string                `json:"repository"`
	ContributionID string                `json:"contribution_id,omitempty"`
	Kind           string                `json:"kind"`
	Reason         string                `json:"reason"`
	NextAction     string                `json:"next_action"`
	DetectedAt     time.Time             `json:"detected_at"`
	HeadSHA        string                `json:"head_sha,omitempty"`
	Status         string                `json:"status,omitempty"`
	Checks         []PrerequisiteFinding `json:"checks,omitempty"`
}

type PrerequisiteFinding struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// RepositoryAssessment is a bounded static assessment, never a test result.
type RepositoryAssessment struct {
	Repository   string                `json:"repository"`
	HeadSHA      string                `json:"head_sha"`
	Status       string                `json:"status"`
	Profile      string                `json:"profile"`
	CheckedAt    time.Time             `json:"checked_at"`
	Files        []string              `json:"files"`
	Findings     []PrerequisiteFinding `json:"findings"`
	Requirements []PrerequisiteFinding `json:"requirements,omitempty"`
	CI           *CIEvidence           `json:"ci,omitempty"`
}

// CIEvidence summarizes recent default-branch GitHub Actions runs of the
// repository's verification workflows. It is observed timing, never a test
// result: discovery uses it to skip builds that cannot fit ForgeFlow's budget,
// and execution uses it to size command deadlines.
type CIEvidence struct {
	Workflows        []string  `json:"workflows"`
	SuccessfulRuns   int       `json:"successful_runs"`
	MedianSeconds    int       `json:"median_seconds"`
	SlowestWorkflow  string    `json:"slowest_workflow,omitempty"`
	LatestConclusion string    `json:"latest_conclusion,omitempty"`
	ObservedAt       time.Time `json:"observed_at"`
}
