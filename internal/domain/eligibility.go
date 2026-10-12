package domain

import "time"

// RepositoryExclusion is current prerequisite evidence, not a quality score.
// Its source contribution remains available for explicit recovery.
type RepositoryExclusion struct {
	Repository     string    `json:"repository"`
	ContributionID string    `json:"contribution_id"`
	Kind           string    `json:"kind"`
	Reason         string    `json:"reason"`
	NextAction     string    `json:"next_action"`
	DetectedAt     time.Time `json:"detected_at"`
}
