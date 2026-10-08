import { useState } from "react";
import { Link, Navigate, useParams, useSearchParams } from "react-router-dom";
import { ArrowLeft, ArrowUpRight } from "../components/icons";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { APIError, request } from "../api";
import type { Contribution, Event } from "../types";
import {
  Empty,
  EventRows,
  SectionHeader,
  StateBadge,
} from "../components/primitives";
import ExecutionPanel from "../components/ExecutionPanel";
import { CopyValue, RepositoryAvatar } from "../components/Visuals";

const contributionURL = (id: string) =>
  `/contributions/${encodeURIComponent(id)}`;

export default function Contributions() {
  const [filter, setFilter] = useState("All");
  const [search, setSearch] = useState("");
  const [searchParams] = useSearchParams();
  const legacyId = searchParams.get("contribution");
  const contributions = useQuery({
    queryKey: ["/contributions"],
    queryFn: () => request<Contribution[]>("/contributions"),
    refetchInterval: 3000,
    enabled: !legacyId,
  });
  if (legacyId) return <Navigate to={contributionURL(legacyId)} replace />;
  const visible = (contributions.data ?? []).filter(
    (c) =>
      (c.repository + " " + c.title)
        .toLowerCase()
        .includes(search.toLowerCase()) &&
      (filter === "All" ||
        (filter === "Needs attention"
          ? ["BLOCKED", "PAUSED", "READY", "PR_PREPARED"].includes(c.state)
          : filter === "Completed"
            ? ["PR_OPENED", "ABANDONED", "FAILED"].includes(c.state)
            : [
                "SELECTED",
                "CLONING",
                "PLANNING",
                "CODING",
                "TESTING",
                "FIXING",
                "REVIEWING",
              ].includes(c.state))),
  );
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">ISOLATED WORK · SHARED VISIBILITY</span>
          <h1>Contributions</h1>
          <p>Every issue gets its own branch, workspace and audit trail.</p>
        </div>
      </div>
      <section className="surface">
        <SectionHeader title="Contribution workspaces" />
        <div className="table-toolbar contribution-filter">
          <input
            aria-label="Search contributions"
            placeholder="Search repositories or issues..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <div
            className="segmented-control"
            aria-label="Filter contribution state"
          >
            {["All", "Active", "Needs attention", "Completed"].map((name) => (
              <button
                key={name}
                aria-pressed={filter === name}
                onClick={() => setFilter(name)}
              >
                {name}
              </button>
            ))}
          </div>
        </div>
        {contributions.error ? (
          <p className="error" role="alert">
            {contributions.error.message}
          </p>
        ) : contributions.isPending ? (
          <div className="skeleton" aria-label="Loading contributions" />
        ) : !contributions.data?.length ? (
          <Empty title="No contribution workspaces">
            Inspect a live opportunity and approve preparation to clone its
            repository into an isolated workspace.
          </Empty>
        ) : !visible.length ? (
          <Empty title="No matching workspaces">
            Try another search or state filter.
          </Empty>
        ) : (
          visible.map((c) => (
            <Link
              className="contribution-row contribution-button"
              key={c.id}
              to={contributionURL(c.id)}
            >
              <RepositoryAvatar repository={c.repository} />
              <div>
                <strong>{c.repository}</strong>
                <p>{c.title}</p>
              </div>
              <StateBadge state={c.state} />
              <ArrowUpRight size={16} aria-hidden="true" />
            </Link>
          ))
        )}
      </section>
    </>
  );
}

const stages = [
  { label: "Plan", states: ["SELECTED", "CLONING", "PLANNING"] },
  { label: "Code", states: ["CODING"] },
  { label: "Test", states: ["TESTING"] },
  { label: "Fix", states: ["FIXING"] },
  { label: "Review", states: ["REVIEWING"] },
  { label: "Human review", states: ["READY", "PR_PREPARED", "PR_OPENED"] },
];

function CommandOutput({ event }: { event: Event }) {
  const record = event.data as {
    arguments: string[];
    exit_code: number;
    output: string;
    truncated: boolean;
  };
  return (
    <details className="command-result">
      <summary>
        Git command · exit {record.exit_code} ·{" "}
        {new Date(event.created_at).toLocaleTimeString()}
      </summary>
      <pre className="issue-body">{record.arguments.join(" ")}</pre>
      <pre className="issue-body">{record.output || "No command output."}</pre>
      {record.truncated && (
        <p className="muted">Output truncated at 128 KiB.</p>
      )}
    </details>
  );
}

export function ContributionPage() {
  const { id = "" } = useParams();
  const cache = useQueryClient();
  const path = contributionURL(id);
  const contribution = useQuery({
    queryKey: [path],
    queryFn: () => request<Contribution>(path),
    placeholderData: () =>
      cache
        .getQueryData<Contribution[]>(["/contributions"])
        ?.find((c) => c.id === id),
    refetchInterval: 3000,
    retry: (count, error) =>
      !(error instanceof APIError && error.status === 404) && count < 1,
  });
  const current = contribution.data;
  const timelinePath = "/events?entity=" + encodeURIComponent(id);
  const timeline = useQuery({
    queryKey: [timelinePath],
    queryFn: () => request<Event[]>(timelinePath),
    enabled: !!current,
    refetchInterval: 3000,
  });
  const events = useQuery({
    queryKey: ["/events"],
    queryFn: () => request<Event[]>("/events"),
    enabled: !!current,
    refetchInterval: 3000,
  });
  // Keep the newest live tail alongside persisted entity history.
  const liveTimeline = [
    ...new Map(
      [
        ...(timeline.data ?? []),
        ...(events.data ?? []).filter((e) => e.entity_id === id),
      ].map((e) => [e.id, e]),
    ).values(),
  ]
    .sort((a, b) => a.id - b.id)
    .slice(-100);

  return (
    <section className="contribution-page" aria-label="Contribution workspace">
      <Link className="text-link contribution-back" to="/contributions">
        <ArrowLeft size={15} /> Back to contributions
      </Link>
      {contribution.isPending ? (
        <div className="skeleton" aria-label="Loading contribution" />
      ) : !current ? (
        <Empty
          title={
            contribution.error instanceof APIError &&
            contribution.error.status === 404
              ? "Contribution not found"
              : "Unable to load contribution"
          }
        >
          {contribution.error?.message ?? "This workspace is unavailable."}
        </Empty>
      ) : (
        <>
          <div className="page-intro contribution-page-heading">
            <div className="contribution-title">
              <RepositoryAvatar repository={current.repository} />
              <div>
                <span className="eyebrow">{current.repository}</span>
                <h1>{current.title}</h1>
                <p>
                  Isolated contribution workspace · configuration v
                  {current.config_version}
                </p>
              </div>
            </div>
            <StateBadge state={current.state} />
          </div>
          {contribution.error && (
            <p className="error" role="alert">
              Workspace status could not refresh: {contribution.error.message}
            </p>
          )}
          {current.message && <p className="note">{current.message}</p>}
          {current.demo && (
            <div className="note">
              Illustrative demo state. No repository was cloned, no agent ran
              and no contribution tests or reviews were performed.
            </div>
          )}
          <ol className="lifecycle" aria-label="Contribution stages">
            {stages.map((stage, index) => (
              <li
                key={stage.label}
                className={
                  stage.states.includes(current.state) ? "current" : ""
                }
                aria-current={
                  stage.states.includes(current.state) ? "step" : undefined
                }
              >
                <span>{index + 1}</span>
                {stage.label}
              </li>
            ))}
          </ol>
          {!current.demo && (
            <ExecutionPanel key={current.id} contribution={current} />
          )}
          <details className="workspace-inspector">
            <summary>
              Workspace details · Branch, location and configuration
            </summary>
            <dl className="key-values">
              <div>
                <dt>Contribution ID</dt>
                <dd>
                  <CopyValue value={current.id} />
                </dd>
              </div>
              <div>
                <dt>{current.demo ? "Branch (illustrative)" : "Branch"}</dt>
                <dd>
                  <CopyValue value={current.branch} />
                </dd>
              </div>
              <div>
                <dt>Configuration snapshot</dt>
                <dd>v{current.config_version}</dd>
              </div>
              {current.workspace && (
                <div>
                  <dt>Workspace</dt>
                  <dd>
                    <CopyValue value={current.workspace} />
                  </dd>
                </div>
              )}
              {current.base_commit && (
                <div>
                  <dt>Base commit</dt>
                  <dd>
                    <CopyValue value={current.base_commit} />
                  </dd>
                </div>
              )}
            </dl>
          </details>
          <section
            className="surface contribution-timeline"
            aria-label="Persisted timeline"
          >
            <SectionHeader title="Persisted timeline" />
            {timeline.error && (
              <p className="error" role="alert">
                Timeline could not refresh: {timeline.error.message}
              </p>
            )}
            <EventRows
              events={liveTimeline.filter((e) => e.entity_id === id)}
            />
          </section>
          {!current.demo && (
            <details className="command-result">
              <summary>Git command output</summary>
              {liveTimeline
                .filter((e) => e.type === "CommandFinished")
                .map((e) => (
                  <CommandOutput key={e.id} event={e} />
                ))}
            </details>
          )}
          <p className="muted contribution-audit-note">
            Git command output and snapshots are saved in this workspace's
            .autopilot directory. Contribution execution requires approval; PR
            submission has its own approval.
          </p>
        </>
      )}
    </section>
  );
}
