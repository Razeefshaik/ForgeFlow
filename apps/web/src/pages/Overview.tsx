import {
  ArrowRight,
  GitBranch,
  ShieldCheck,
  CircleCheck,
  CirclePause,
  ArrowUpRight,
  Bot,
  Terminal,
} from "../components/icons";
import { Link } from "react-router-dom";
import { lazy, Suspense } from "react";
import {
  AnimatedNumber,
  RepositoryAvatar,
  ProgressMeter,
} from "../components/Visuals";
import { useAPI } from "../api";
import type { Contribution, Event, Overview as OverviewType } from "../types";
import { EventRows, SectionHeader, StateBadge } from "../components/primitives";
import LiveExecutionProgress from "../components/LiveExecutionProgress";
const Opportunities = lazy(() => import("./Opportunities"));

export default function Overview() {
  const overview = useAPI<OverviewType>("/overview");
  const contributions = useAPI<Contribution[]>("/contributions");
  const events = useAPI<Event[]>("/events");
  const agents = useAPI<{ status: string }[]>("/agents");
  const o = overview.data;
  const all = contributions.data ?? [];
  const attention = all.filter((c) =>
    ["BLOCKED", "PAUSED", "READY", "PR_PREPARED"].includes(c.state),
  );
  const active = all.filter(
    (c) =>
      ![
        "BLOCKED",
        "PAUSED",
        "READY",
        "PR_PREPARED",
        "PR_OPENED",
        "ABANDONED",
        "FAILED",
      ].includes(c.state),
  );
  const visible = [...attention, ...active].slice(0, 5);
  const focused = active[0] ?? attention[0];
  return (
    <>
      <div className="page-intro overview-intro">
        <div>
          <span className="eyebrow">YOUR WORKSPACE, AT A GLANCE</span>
          <h1>Good work starts here.</h1>
          <p>Your agents, contribution evidence, and next decisions in one place.</p>
        </div>
        <Link className="btn btn-primary" to="/opportunities">
          Find an opportunity <ArrowUpRight size={15} />
        </Link>
      </div>
      {overview.error && (
        <p className="error" role="alert">
          {overview.error.message}
        </p>
      )}
      <div className="metric-grid">
        {[
          {
            label: "Running agents",
            value: agents.data?.filter((a) => a.status === "RUNNING").length,
            icon: GitBranch,
            detail: "Real local Codex sessions",
          },
          {
            label: "Needs attention",
            value: contributions.data ? attention.length : undefined,
            icon: CirclePause,
            detail: "Approvals, paused work and blockers",
          },
          {
            label: "Ready for review",
            value: o
              ? (o.states.READY ?? 0) + (o.states.PR_PREPARED ?? 0)
              : undefined,
            icon: ShieldCheck,
            detail: "Waiting for your eyes",
          },
        ].map((m, index) => (
          <div className="metric" data-tone={index === 0 ? "indigo" : index === 1 ? "rose" : "jade"} key={m.label}>
            <div>
              <span>{m.label}</span>
              <m.icon size={22} />
            </div>
            <AnimatedNumber value={m.value} />
            <p>{m.detail}</p>
          </div>
        ))}
      </div>
      <div className="workspace-grid">
        <section className="surface workspace-work">
          <SectionHeader
            title="Active work"
            extra={
              <Link className="text-link" to="/contributions">
                View all <ArrowRight size={14} />
              </Link>
            }
          />
          {focused && !contributions.error && <div className="overview-focus">
            <div className="focus-kicker"><span><Terminal size={15} />Contribution in focus</span><StateBadge state={focused.state} /></div>
            <Link className="focus-issue" to={`/contributions/${encodeURIComponent(focused.id)}`}><span className="repo-name">{focused.repository}</span><h2>{focused.title}</h2><ArrowUpRight size={22} /></Link>
            <div className="focus-context"><span><Bot size={15} />{focused.codex_model || "Model not recorded"}</span><code>{focused.branch || "Branch not recorded"}</code></div>
            {!focused.demo && ["PLANNING", "CODING", "TESTING", "FIXING", "REVIEWING"].includes(focused.state) && <LiveExecutionProgress id={focused.id} phase={focused.state} />}
            <Link className="text-link" to={`/contributions/${encodeURIComponent(focused.id)}`}>Inspect execution and evidence <ArrowRight size={15} /></Link>
          </div>}
          {contributions.error ? (
            <p className="error">{contributions.error.message}</p>
          ) : contributions.isPending ? (
            <div className="skeleton" aria-label="Loading contributions" />
          ) : visible.length ? (
            visible.map((c) => (
              <Link
                className="contribution-row"
                key={c.id}
                to={`/contributions/${encodeURIComponent(c.id)}`}
              >
                <RepositoryAvatar repository={c.repository} />
                <div>
                  <strong>{c.repository}</strong>
                  <p>{c.title}</p>
                  <span className="work-status">
                    {["READY", "PR_PREPARED"].includes(c.state)
                      ? "Ready for your review"
                      : c.state === "BLOCKED"
                        ? "Needs your attention"
                        : c.state === "PAUSED"
                          ? "Paused by request"
                          : "Open to inspect execution"}
                  </span>
                </div>
                <div className="work-card-status">
                  <StateBadge state={c.state} />
                  {["CODING", "TESTING", "FIXING", "REVIEWING"].includes(
                    c.state,
                  ) &&
                    !c.demo && (
                      <ProgressMeter
                        indeterminate
                        label={`${c.repository}: execution ongoing`}
                      />
                    )}
                </div>
              </Link>
            ))
          ) : (
            <div className="workspace-empty">
              <span className="empty-workspace-symbol"><GitBranch size={32} /></span>
              <h3>Your next contribution starts here.</h3>
              <p>
                Choose an issue that matters to you. ForgeFlow keeps the plan,
                tests and review together.
              </p>
              <Link className="text-link" to="/opportunities">
                Explore opportunities <ArrowRight size={14} />
              </Link>
            </div>
          )}
        </section>
        <aside className="attention-card surface">
          <span className="attention-icon">
            {attention.length ? (
              <CirclePause size={22} />
            ) : (
              <CircleCheck size={22} />
            )}
          </span>
          <span className="eyebrow">NEEDS YOUR ATTENTION</span>
          <h2>
            {contributions.isPending
              ? "Checking your workspace…"
              : contributions.error
                ? "Unable to check"
                : attention.length
                  ? `${attention.length} contribution${attention.length === 1 ? "" : "s"} to look at.`
                  : "You're all caught up."}
          </h2>
          <p>
            {contributions.error
              ? "Reconnect to see what needs your attention."
              : attention.length
                ? "Review completed work, resume a paused contribution, or inspect a blocker."
                : "Approvals and blockers appear here when there's something for you to do."}
          </p>
          <div className="attention-items">
            {attention.slice(0, 3).map((c) => (
              <Link
                key={c.id}
                to={`/contributions/${encodeURIComponent(c.id)}`}
              >
                <span className="attention-item-dot" />
                <div>
                  <strong>{c.repository}</strong>
                  <small>
                    {["READY", "PR_PREPARED"].includes(c.state)
                      ? "Review completed work"
                      : c.state === "BLOCKED"
                        ? "Inspect blocker"
                        : "Resume paused work"}
                  </small>
                </div>
                <ArrowUpRight size={16} />
              </Link>
            ))}
          </div>
          <Link className="btn btn-secondary" to="/contributions">
            Open contributions <ArrowRight size={14} />
          </Link>
          <div className="approval-note">
            <ShieldCheck size={16} />
            <span>
              You choose when work starts.
              <br />
              You approve before a PR is submitted.
            </span>
          </div>
        </aside>
      </div>
      <div className="overview-bento">
      <section className="surface overview-opportunities">
        <Suspense
          fallback={
            <div className="skeleton" aria-label="Loading opportunities" />
          }
        >
          <Opportunities compact />
        </Suspense>
      </section>
      <div className="overview-lower">
        <section className="surface">
          <SectionHeader
            title="Recent activity"
            extra={
              <Link className="text-link" to="/activity">
                All events <ArrowRight size={14} />
              </Link>
            }
          />
          {events.error ? (
            <p className="error">{events.error.message}</p>
          ) : (
            <EventRows events={events.data ?? []} compact />
          )}
        </section>
      </div>
      </div>
      <div className="bottom-note">
        <ShieldCheck size={14} /> Ranking ignores Codex effort. Contribution
        execution and PR submission require human approval.
      </div>
    </>
  );
}
