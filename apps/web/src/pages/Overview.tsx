import { ArrowRight, GitBranch, Radar, ShieldCheck, Sparkles, CircleCheck, CirclePause, ArrowUpRight } from "lucide-react";
import { Link } from "react-router-dom";
import { lazy, Suspense } from "react";
import { useAPI } from "../api";
import type { Contribution, Event, Overview as OverviewType } from "../types";
import { Badge, EventRows, SectionHeader, StateBadge } from "../components/primitives";
const Opportunities = lazy(() => import("./Opportunities"));

export default function Overview() {
  const overview = useAPI<OverviewType>("/overview");
  const contributions = useAPI<Contribution[]>("/contributions");
  const events = useAPI<Event[]>("/events");
  const o = overview.data;
  const all = contributions.data ?? [];
  const attention = all.filter(c => ["BLOCKED", "PAUSED", "READY", "PR_PREPARED"].includes(c.state));
  const active = all.filter(c => !["BLOCKED", "PAUSED", "READY", "PR_PREPARED", "PR_OPENED", "ABANDONED", "FAILED"].includes(c.state));
  const visible = [...attention, ...active].slice(0, 5);
  return (
    <>
      <div className="page-intro overview-intro">
        <div>
          <span className="eyebrow">YOUR WORKSPACE, AT A GLANCE</span>
          <h1>Good work starts here.</h1>
          <p>A little clarity. Your next contribution.</p>
        </div>
        <Link className="btn btn-primary" to="/opportunities">Find an opportunity <ArrowUpRight size={15} /></Link>
      </div>
      {overview.error && <p className="error" role="alert">{overview.error.message}</p>}
      <div className="metric-grid">
        {[
          { label: "Repositories", value: o?.repositories, icon: Radar, detail: o?.mode === "demo" ? "Illustrative candidates" : "In your discovery pool" },
          { label: "Opportunities", value: o?.opportunities, icon: Sparkles, detail: o ? `${o.high_quality} scoring 85 or higher` : "Loading ranked issues" },
          { label: "Active contributions", value: o?.active_contributions, icon: GitBranch, detail: "Isolated workspaces" },
          { label: "Ready for review", value: o ? o.states.READY ?? 0 : undefined, icon: ShieldCheck, detail: "Waiting for your eyes" },
        ].map(m => <div className="metric" key={m.label}><div><span>{m.label}</span><m.icon size={17} /></div><strong>{m.value ?? "—"}</strong><p>{m.detail}</p></div>)}
      </div>
      <div className="workspace-grid">
        <section className="surface workspace-work">
          <SectionHeader title="Your contributions" extra={<Link className="text-link" to="/contributions">View all <ArrowRight size={14} /></Link>} />
          {contributions.error ? <p className="error">{contributions.error.message}</p> : contributions.isPending ? <div className="skeleton" aria-label="Loading contributions" /> : visible.length ? visible.map(c => (
            <Link className="contribution-row" key={c.id} to={`/contributions?contribution=${encodeURIComponent(c.id)}`}>
              <span className="branch-icon"><GitBranch size={18} /></span>
              <div><strong>{c.repository}</strong><p>{c.title}</p><span className="work-status">{["READY", "PR_PREPARED"].includes(c.state) ? "Ready for your review" : c.state === "BLOCKED" ? "Needs your attention" : c.state === "PAUSED" ? "Paused by request" : "Open to inspect execution"}</span></div>
              <StateBadge state={c.state} />
            </Link>
          )) : <div className="workspace-empty"><span className="empty-orbit"><GitBranch size={27} /></span><h3>Your next contribution starts here.</h3><p>Choose an issue that matters to you. ForgeFlow keeps the plan, tests and review together.</p><Link className="text-link" to="/opportunities">Explore opportunities <ArrowRight size={14} /></Link></div>}
        </section>
        <aside className="attention-card surface">
          <span className="attention-icon">{attention.length ? <CirclePause size={22} /> : <CircleCheck size={22} />}</span>
          <span className="eyebrow">NEEDS YOUR ATTENTION</span>
          <h2>{contributions.isPending ? "Checking your workspace…" : contributions.error ? "Unable to check" : attention.length ? `${attention.length} contribution${attention.length === 1 ? "" : "s"} to look at.` : "You're all caught up."}</h2>
          <p>{contributions.error ? "Reconnect to see what needs your attention." : attention.length ? "Review completed work, resume a paused contribution, or inspect a blocker." : "Approvals and blockers appear here when there's something for you to do."}</p>
          <Link className="btn btn-secondary" to="/contributions">Open contributions <ArrowRight size={14} /></Link>
          <div className="approval-note"><ShieldCheck size={16} /><span>You choose when work starts.<br />You approve before a PR is submitted.</span></div>
        </aside>
      </div>
      <section className="surface overview-opportunities"><Suspense fallback={<div className="skeleton" aria-label="Loading opportunities" />}><Opportunities compact /></Suspense></section>
      <div className="lower-grid overview-lower">
        <section className="surface"><SectionHeader title="Recent activity" extra={<Link className="text-link" to="/activity">All events <ArrowRight size={14} /></Link>} />{events.error ? <p className="error">{events.error.message}</p> : <EventRows events={events.data ?? []} compact />}</section>
        <section className="workflow-card surface"><span className="eyebrow">FROM ISSUE TO CONTRIBUTION</span><h2>A thoughtful workflow.<br />With you in control.</h2><p>Codex implements. Real commands verify. An independent reviewer checks the work.</p><ol className="workflow-list">{["Find the right issue", "Approve the contribution", "Plan, build and verify", "Review before submitting"].map((step, i) => <li key={step}><span className="workflow-number">{i + 1}</span><span className="workflow-label">{step}</span>{(i === 1 || i === 3) && <Badge>Your approval</Badge>}</li>)}</ol></section>
      </div>
      <div className="bottom-note"><ShieldCheck size={14} /> Ranking ignores Codex effort. Contribution execution and PR submission require human approval.</div>
    </>
  );
}
