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
}
