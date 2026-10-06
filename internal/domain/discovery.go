package domain

import "time"

// Evidence records observations, not guarantees about future acceptance or execution.
type Evidence struct {
	IssueURL           string     `json:"issue_url"`
	RepositoryURL      string     `json:"repository_url"`
	Description        string     `json:"description"`
	Topics             []string   `json:"topics"`
	Stars              int        `json:"stars"`
	Forks              int        `json:"forks"`
	Archived           bool       `json:"archived"`
	DefaultBranch      string     `json:"default_branch"`
	HeadSHA            string     `json:"head_sha"`
	LastCommit         *time.Time `json:"last_commit"`
	LatestRelease      *time.Time `json:"latest_release"`
	IssueCreatedAt     time.Time  `json:"issue_created_at"`
	IssueUpdatedAt     time.Time  `json:"issue_updated_at"`
	IssueState         string     `json:"issue_state"`
	Assignees          []string   `json:"assignees"`
	CommentCount       int        `json:"comment_count"`
	MaintainerComments *int       `json:"maintainer_comments"`
	CommentsSampled    int        `json:"comments_sampled"`
	CompetingPRs       []string   `json:"competing_prs"`
	CompetitionChecked bool       `json:"competition_checked"`
	MergedPRsSampled   int        `json:"merged_prs_sampled"`
	ExternalPRsMerged  *int       `json:"external_prs_merged"`
	Guidelines         *bool      `json:"guidelines"`
	AgentRules         *bool      `json:"agent_rules"`
	CodeOfConduct      *bool      `json:"code_of_conduct"`
	CI                 *bool      `json:"ci"`
	Tests              *bool      `json:"tests"`
	BuildFiles         []string   `json:"build_files"`
	TreeComplete       bool       `json:"tree_complete"`
	Warnings           []string   `json:"warnings"`
	ObservedAt         time.Time  `json:"observed_at"`
	ConfigVersion      int64      `json:"config_version"`
	RunID              string     `json:"run_id"`
}

type DiscoveryRun struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"`
	ConfigVersion int64      `json:"config_version"`
	StartedAt     time.Time  `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	Candidates    int        `json:"candidates"`
	Accepted      int        `json:"accepted"`
	Requests      int        `json:"requests"`
	Warnings      []string   `json:"warnings"`
}
