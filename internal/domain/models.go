package domain

import "time"

type Factor struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Score  float64 `json:"score"`
	Weight float64 `json:"weight"`
	Reason string  `json:"reason"`
}
type Quality struct {
	Factors []Factor `json:"factors"`
}
type Ranking struct {
	Score   float64  `json:"score"`
	Factors []Factor `json:"factors"`
	Version string   `json:"version"`
}
type Estimate struct {
	Category        string   `json:"category"`
	Iterations      [2]int   `json:"iterations"`
	Files           [2]int   `json:"files"`
	Context         string   `json:"context"`
	TestComplexity  string   `json:"test_complexity"`
	Confidence      float64  `json:"confidence"`
	Explanation     []string `json:"explanation"`
	AllowanceImpact *float64 `json:"allowance_impact"`
}
type Opportunity struct {
	ID              string    `json:"id"`
	Repository      string    `json:"repository"`
	Number          int       `json:"number"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	Language        string    `json:"language"`
	Domain          string    `json:"domain"`
	Labels          []string  `json:"labels"`
	Difficulty      string    `json:"difficulty"`
	MergeLikelihood string    `json:"merge_likelihood"`
	Risks           []string  `json:"risks"`
	Ranking         Ranking   `json:"ranking"`
	Estimate        Estimate  `json:"estimate"`
	Demo            bool      `json:"demo"`
	Evidence        *Evidence `json:"evidence,omitempty"`
}
type Contribution struct {
	ID                string    `json:"id"`
	OpportunityID     string    `json:"opportunity_id"`
	Repository        string    `json:"repository"`
	Title             string    `json:"title"`
	State             string    `json:"state"`
	PreviousState     string    `json:"previous_state,omitempty"`
	Branch            string    `json:"branch"`
	ConfigVersion     int64     `json:"config_version"`
	UpdatedAt         time.Time `json:"updated_at"`
	Demo              bool      `json:"demo"`
	Workspace         string    `json:"workspace,omitempty"`
	BaseCommit        string    `json:"base_commit,omitempty"`
	Message           string    `json:"message,omitempty"`
	ExecutionApproved bool      `json:"execution_approved,omitempty"`
}
type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	EntityID  string    `json:"entity_id"`
	Actor     string    `json:"actor"`
	Message   string    `json:"message"`
	Data      any       `json:"data"`
	CreatedAt time.Time `json:"created_at"`
	Demo      bool      `json:"demo"`
}
type Config struct {
	Profile struct {
		Languages map[string]float64 `json:"languages"`
		Domains   map[string]float64 `json:"domains"`
		Exclude   []string           `json:"exclude"`
	} `json:"profile"`
	Repositories struct {
		MinStars           int `json:"min_stars"`
		RecentActivityDays int `json:"recent_activity_days"`
	} `json:"repositories"`
	Issues struct {
		PreferredLabels []string `json:"preferred_labels"`
		Difficulty      []string `json:"difficulty"`
	} `json:"issues"`
	Agents struct {
		MaxScouts       int `json:"max_scouts"`
		MaxContributors int `json:"max_contributors"`
		MaxReviewers    int `json:"max_reviewers"`
	} `json:"agents"`
	Contributions struct {
		RequirePlanApproval bool `json:"require_plan_approval"`
		AutoPreparePR       bool `json:"auto_prepare_pr"`
		AutoCreatePR        bool `json:"auto_create_pr"`
		AutoMerge           bool `json:"auto_merge"`
	} `json:"contributions"`
	Codex struct {
		MaxFixIterations int `json:"max_fix_iterations"`
		MaxReviewCycles  int `json:"max_review_cycles"`
	} `json:"codex"`
}
type ConfigVersion struct {
	Version   int64     `json:"version"`
	Config    Config    `json:"config"`
	Actor     string    `json:"actor"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
type Proposal struct {
	ID          string     `json:"id"`
	BaseVersion int64      `json:"base_version"`
	Config      Config     `json:"config"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}
