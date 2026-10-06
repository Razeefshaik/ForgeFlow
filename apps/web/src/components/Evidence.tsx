import type { Evidence as EvidenceType } from "../types";
function presence(value: boolean | null) {
  return value === null ? "Unknown" : value ? "Detected" : "Not detected";
}
function date(value: string | null) {
  return value ? new Date(value).toLocaleString() : "Unavailable";
}
export default function Evidence({ e }: { e: EvidenceType }) {
  return (
    <section className="evidence-panel" aria-label="GitHub analysis evidence">
      <h3>Observed on GitHub</h3>
      <p className="muted">
        {date(e.observed_at)} · profile v{e.config_version}. These observations
        are a snapshot; confirm current issue status on GitHub before
        contributing.
      </p>
      <div className="evidence-links">
        <a
          className="text-link"
          href={e.issue_url}
          target="_blank"
          rel="noreferrer"
        >
          Open issue on GitHub ↗
        </a>
        <a
          className="text-link"
          href={e.repository_url}
          target="_blank"
          rel="noreferrer"
        >
          Open repository ↗
        </a>
      </div>
      <dl className="key-values">
        <div>
          <dt>Issue status at observation</dt>
          <dd>{e.issue_state}</dd>
        </div>
        <div>
          <dt>Updated</dt>
          <dd>{date(e.issue_updated_at)}</dd>
        </div>
        <div>
          <dt>Created</dt>
          <dd>{date(e.issue_created_at)}</dd>
        </div>
        <div>
          <dt>Assignees</dt>
          <dd>{e.assignees.join(", ") || "None observed"}</dd>
        </div>
        <div>
          <dt>Stars / forks</dt>
          <dd>
            {e.stars.toLocaleString()} / {e.forks.toLocaleString()}
          </dd>
        </div>
        <div>
          <dt>Last default-branch commit</dt>
          <dd>{date(e.last_commit)}</dd>
        </div>
        <div>
          <dt>Stable release (latest five)</dt>
          <dd>{date(e.latest_release)}</dd>
        </div>
        <div>
          <dt>Archived</dt>
          <dd>{e.archived ? "Yes" : "No"}</dd>
        </div>
        <div>
          <dt>Contributor guidelines</dt>
          <dd>{presence(e.guidelines)}</dd>
        </div>
        <div>
          <dt>Agent rules / code of conduct</dt>
          <dd>
            {presence(e.agent_rules)} / {presence(e.code_of_conduct)}
          </dd>
        </div>
        <div>
          <dt>CI / test files</dt>
          <dd>
            {presence(e.ci)} / {presence(e.tests)}
          </dd>
        </div>
        <div>
          <dt>Build manifests</dt>
          <dd>{e.build_files.join(", ") || "None observed"}</dd>
        </div>
        <div>
          <dt>Maintainer participation</dt>
          <dd>
            {e.maintainer_comments === null
              ? "Unknown"
              : `${e.maintainer_comments} of ${e.comments_sampled} sampled comments`}{" "}
            ({e.comment_count} total)
          </dd>
        </div>
        <div>
          <dt>External merges</dt>
          <dd>
            {e.external_prs_merged === null
              ? "Unknown"
              : `${e.external_prs_merged} of ${e.merged_prs_sampled} sampled merged PRs`}
          </dd>
        </div>
      </dl>
      <p className="muted">
        {e.topics?.length ? "Topics: " + e.topics.join(", ") + ". " : ""}File
        presence does not prove that tests or CI pass. Acceptance signals are
        sampled heuristics.
      </p>
      <h3>Possible competing work</h3>
      {e.competing_prs.length ? (
        <ul className="risk-list">
          {e.competing_prs.map((url, i) => (
            <li key={url}>
              <a
                className="text-link"
                href={url}
                target="_blank"
                rel="noreferrer"
              >
                Referencing pull request {i + 1} ↗
              </a>
            </li>
          ))}
        </ul>
      ) : (
        <p className="muted">
          {e.competition_checked
            ? "No referencing open PR found in the bounded search. Unlinked work may exist."
            : "Competition evidence is incomplete or unavailable."}
        </p>
      )}
      <details>
        <summary>Full issue description</summary>
        <pre className="issue-description">
          {e.description || "No description supplied."}
        </pre>
      </details>
    </section>
  );
}
