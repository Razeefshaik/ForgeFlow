export interface Factor {
  key: string;
  label: string;
  score: number;
  weight: number;
  reason: string;
}
export interface Opportunity {
  id: string;
  repository: string;
  number: number;
  title: string;
  summary: string;
  language: string;
  domain: string;
  labels: string[];
  difficulty: string;
  merge_likelihood: string;
  risks: string[];
  demo: boolean;
  evidence?: Evidence;
  ranking: { score: number; factors: Factor[]; version: string };
  estimate: {
    category: string;
    iterations: [number, number];
    files: [number, number];
    context: string;
    test_complexity: string;
    confidence: number;
    explanation: string[];
    allowance_impact: number | null;
  };
}
export interface Evidence {
  issue_url: string;
  repository_url: string;
  description: string;
  topics: string[] | null;
  stars: number;
  forks: number;
  archived: boolean;
  default_branch: string;
  head_sha: string;
  last_commit: string | null;
  latest_release: string | null;
  issue_created_at: string;
  issue_updated_at: string;
  issue_state: string;
  assignees: string[];
  comment_count: number;
  maintainer_comments: number | null;
  comments_sampled: number;
  competing_prs: string[];
  competition_checked: boolean;
  merged_prs_sampled: number;
  external_prs_merged: number | null;
  guidelines: boolean | null;
  agent_rules: boolean | null;
  code_of_conduct: boolean | null;
  ci: boolean | null;
  tests: boolean | null;
  build_files: string[];
  tree_complete: boolean;
  warnings: string[];
  observed_at: string;
  config_version: number;
  run_id: string;
}
export interface DiscoveryRun {
  id: string;
  status: string;
  config_version: number;
  started_at: string;
  finished_at: string | null;
  candidates: number;
  accepted: number;
  requests: number;
  warnings: string[];
}
export interface DiscoveryStatus {
  available: boolean;
  running: boolean;
  automatic: boolean;
  authentication: string;
  interval_seconds: number;
  next_run: string | null;
  retry_at: string | null;
  last_run: DiscoveryRun | null;
  message: string;
}
export interface Contribution {

  workspace?: string;
  base_commit?: string;
  message?: string;
  id: string;
  opportunity_id: string;
  repository: string;
  title: string;
  state: string;
  previous_state?: string;
  branch: string;
  config_version: number;
  codex_model?: string;
  updated_at: string;
  demo: boolean;
}
export interface Event {
  id: number;
  type: string;
  entity_id: string;
  actor: string;
  message: string;
  data: unknown;
  created_at: string;
  demo: boolean;
}
export interface Config {
  profile: {
    languages: Record<string, number>;
    domains: Record<string, number>;
    exclude: string[];
  };
  repositories: { min_stars: number; recent_activity_days: number };
  issues: { preferred_labels: string[]; difficulty: string[] };
  agents: {
    max_scouts: number;
    max_contributors: number;
    max_reviewers: number;
  };
  contributions: {
    require_plan_approval: boolean;
    auto_prepare_pr: boolean;
    auto_create_pr: boolean;
    auto_merge: boolean;
  };
  codex: { max_fix_iterations: number; max_review_cycles: number };
}
export interface ConfigVersion {
  version: number;
  config: Config;
  actor: string;
  reason: string;
  created_at: string;
}
export interface Proposal {
  expires_at?: string;
  id: string;
  base_version: number;
  config: Config;
  reason: string;
  status: string;
  created_at: string;
}
export interface Overview {
  mode: "demo" | "live";
  repositories: number;
  opportunities: number;
  high_quality: number;
  active_contributions: number;
  states: Record<string, number>;
  config_version: number;
  discovery_status: string;
  execution_available: boolean;
}
export interface Reply {
  confirmation?: {action: string; contribution_id: string; label: string; constraints?: string};
  message: string;
  action: string;
  proposal?: Proposal;
}
