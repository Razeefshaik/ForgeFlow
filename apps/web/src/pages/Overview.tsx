import {
  ArrowRight,
  GitBranch,
  Radar,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { Link } from "react-router-dom";
import { useAPI } from "../api";
import type { Contribution, Event, Overview as OverviewType } from "../types";
import {
  Badge,
  EventRows,
  SectionHeader,
  StateBadge,
} from "../components/primitives";
import { lazy, Suspense } from "react";
const Opportunities = lazy(() => import("./Opportunities"));
export default function Overview() {
  const overview = useAPI<OverviewType>("/overview");
  const contributions = useAPI<Contribution[]>("/contributions");
  const events = useAPI<Event[]>("/events");
  const o = overview.data;
  if (overview.error)
    return (
      <p className="error" role="alert">
        {overview.error.message}
      </p>
    );
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">YOUR CONTRIBUTION CONTROL PLANE</span>
          <h1>Good work starts here.</h1>
          <p>Find the right issue. Keep every contribution under control.</p>
        </div>
        <Badge tone="green">
          <span className="status-dot" /> Local-first
        </Badge>
      </div>
      <div className="metric-grid">
        {[
          {
            label: "Repositories in view",
            value: o?.repositories,
            icon: Radar,
            detail:
              o?.mode === "demo"
                ? "Illustrative repositories"
                : "Persisted repository candidates",
          },
          {
            label: "Ranked opportunities",
            value: o?.opportunities,
            icon: Sparkles,
            detail: (o?.high_quality ?? 0) + " scoring 85 or higher",
          },
          {
            label: "Active contributions",
            value: o?.active_contributions,
            icon: GitBranch,
            detail:
              o?.mode === "demo"
                ? "Illustrative lifecycle states"
                : "Persisted contribution workflows",
          },
          {
            label: "Ready for review",
            value: o?.states.READY ?? 0,
            icon: ShieldCheck,
            detail: "Human approval comes next",
          },
        ].map((m) => (
          <div className="metric" key={m.label}>
            <div>
              <span>{m.label}</span>
              <m.icon size={16} />
            </div>
            <strong>{m.value ?? "—"}</strong>
            <p>{m.detail}</p>
          </div>
        ))}
      </div>
      <div className="pipeline">
        <div>
          <span className="eyebrow">THE WORKFLOW</span>
          <Badge>2 human gates</Badge>
        </div>
        <div className="pipeline-steps">
          {[
            "Discover",
            "Rank",
            "Select",
            "Contribute",
            "Test",
            "Review",
            "Human review",
          ].map((x, i) => (
            <span key={x}>
              <i className={i === 2 || i === 6 ? "gate" : ""}>
                {String(i + 1).padStart(2, "0")}
              </i>
              {x}
              {i < 6 && <ArrowRight size={12} />}
            </span>
          ))}
        </div>
      </div>
      <section className="surface">
        <Suspense
          fallback={
            <div className="skeleton" aria-label="Loading opportunities" />
          }
        >
          <Opportunities compact />
        </Suspense>
      </section>
      <div className="lower-grid">
        <section className="surface">
          <SectionHeader
            title="Contribution pipeline"
            extra={
              <Link className="text-link" to="/contributions">
                Inspect <ArrowRight size={13} />
              </Link>
            }
          />
          {contributions.error ? (
            <p className="error">{contributions.error.message}</p>
          ) : contributions.data?.length ? (
            contributions.data.map((c) => (
              <Link className="contribution-row" key={c.id} to="/contributions">
                <span className="branch-icon">
                  <GitBranch size={17} />
                </span>
                <div>
                  <strong>{c.repository.split("/")[1]}</strong>
                  <p>{c.title}</p>
                </div>
                <StateBadge state={c.state} />
              </Link>
            ))
          ) : (
            <p className="inline-empty">
              No contributions yet. Execution is a later milestone.
            </p>
          )}
        </section>
        <section className="surface">
          <SectionHeader
            title="Recent activity"
            extra={
              <Link className="text-link" to="/activity">
                All events <ArrowRight size={13} />
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
      <div className="bottom-note">
        <ShieldCheck size={14} /> Ranking ignores Codex effort. Contribution
        execution and PR submission require human approval.
      </div>
    </>
  );
}
