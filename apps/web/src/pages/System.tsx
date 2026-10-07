import { useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Bot, GitBranch, ShieldCheck } from "lucide-react";
import { request, useAPI } from "../api";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import type { Contribution, Event, Opportunity } from "../types";
import {
  Badge,
  Empty,
  EventRows,
  SectionHeader,
  StateBadge,
} from "../components/primitives";
import { Dialog } from "../components/ui/dialog";
import ExecutionPanel from "../components/ExecutionPanel";
function CommandOutput({ event }: { event: Event }) {
  const record = event.data as { arguments: string[]; exit_code: number; output: string; truncated: boolean };
  return <details className="command-result">
    <summary>Git command · exit {record.exit_code} · {new Date(event.created_at).toLocaleTimeString()}</summary>
    <pre className="issue-body">{record.arguments.join(" ")}</pre>
    <pre className="issue-body">{record.output || "No command output."}</pre>
    {record.truncated && <p className="muted">Output truncated at 128 KiB.</p>}
  </details>;
}
export function Contributions() {
  const cs = useQuery({queryKey:["/contributions"],queryFn:()=>request<Contribution[]>("/contributions"),refetchInterval:3000});
  const events = useAPI<Event[]>("/events");
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedId = searchParams.get("contribution");
  const current = cs.data?.find(c => c.id === selectedId) ?? null;
  const selectContribution = (id: string | null) => {
    setSearchParams(previous => {
      const next = new URLSearchParams(previous);
      if (id) next.set("contribution", id); else next.delete("contribution");
      return next;
    });
  };
  const timelinePath="/events?entity="+encodeURIComponent(current?.id ?? "");
  const timeline = useQuery({queryKey:[timelinePath],queryFn:()=>request<Event[]>(timelinePath),enabled:!!current,refetchInterval:3000});
  // Merge the global live tail so already-running older servers also show fresh activity.
  const liveTimeline = [...new Map([...(timeline.data ?? []), ...(events.data ?? []).filter(e => e.entity_id === current?.id)].map(e => [e.id,e])).values()].sort((a,b)=>a.id-b.id).slice(-100);
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
        {cs.error ? (
          <p className="error">{cs.error.message}</p>
        ) : !cs.data?.length ? (
          <Empty title="No contribution workspaces">
            Inspect a live opportunity and approve preparation to clone its repository into an isolated workspace.
          </Empty>
        ) : (
          cs.data.map((c) => (
            <button
              className="contribution-row contribution-button"
              onClick={() => selectContribution(c.id)}
              key={c.id}
            >
              <span className="branch-icon">
                <GitBranch size={18} />
              </span>
              <div>
                <strong>{c.repository}</strong>
                <p>{c.title}</p>
              </div>
              <StateBadge state={c.state} />
            </button>
          ))
        )}
      </section>
      <Dialog
        open={!!current}
        onOpenChange={(v) => {
          if (!v) selectContribution(null);
        }}
        title={current?.repository ?? "Contribution"}
        description="Your contribution workspace · Plan, verification and review"
        wide
        workspace
      >
        {current && (
          <>
            <StateBadge state={current.state} />
            <p className="summary">{current.title}</p>
            {current.message && <p className="note">{current.message}</p>}
            {current.demo && (
              <div className="note">
                Illustrative demo state. No repository was cloned, no agent ran
                and no contribution tests or reviews were performed.
              </div>
            )}
            <ol className="lifecycle" aria-label="Contribution stages">
              {[{label:"Plan",states:["SELECTED","CLONING","PLANNING"]},{label:"Code",states:["CODING"]},{label:"Test",states:["TESTING"]},{label:"Fix",states:["FIXING"]},{label:"Review",states:["REVIEWING"]},{label:"Human review",states:["READY","PR_PREPARED","PR_OPENED"]}].map((stage,index) => <li key={stage.label} className={stage.states.includes(current.state) ? "current" : ""} aria-current={stage.states.includes(current.state) ? "step" : undefined}><span>{index + 1}</span>{stage.label}</li>)}
            </ol>
            <details className="workspace-inspector"><summary>Workspace details · Branch, location and configuration</summary>
            <dl className="key-values">
              <div>
                <dt>Contribution ID</dt>
                <dd>{current.id}</dd>
              </div>
              <div>
                <dt>{current.demo ? "Branch (illustrative)" : "Branch"}</dt>
                <dd>{current.branch}</dd>
              </div>
              <div>
                <dt>Configuration snapshot</dt>
                <dd>v{current.config_version}</dd>
              </div>
              {current.workspace && <div><dt>Workspace</dt><dd><code>{current.workspace}</code></dd></div>}
              {current.base_commit && <div><dt>Base commit</dt><dd><code>{current.base_commit}</code></dd></div>}
            </dl>
            </details>
            {!current.demo && <ExecutionPanel contribution={current} />}
            <h3>Persisted timeline</h3>
            <EventRows
              events={liveTimeline.filter(
                (e) => e.entity_id === current.id,
              )}
            />
            {!current.demo && <>
              <h3>Git command output</h3>
              {liveTimeline.filter(e => e.type === "CommandFinished").map(e => <CommandOutput key={e.id} event={e} />)}
            </>}
            <p className="muted">
              Git command output and snapshots are saved in this workspace's .autopilot directory. Contribution execution requires approval; PR submission has its own approval.
            </p>
          </>
        )}
      </Dialog>
    </>
  );
}
export function Activity() {
  const events = useAPI<Event[]>("/events");
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">PERSISTED · ORDERED · REPLAYABLE</span>
          <h1>Activity</h1>
          <p>
            The latest 100 events. Configuration and system history stay intact.
          </p>
        </div>
        <Badge>Append-only audit</Badge>
      </div>
      <section className="surface">
        <SectionHeader title="Event timeline" />
        {events.error ? (
          <p className="error">{events.error.message}</p>
        ) : (
          <EventRows events={events.data ?? []} />
        )}
      </section>
    </>
  );
}
export function Agents() {
  const cache = useQueryClient();
  const [controlError, setControlError] = useState("");
  const agents = useAPI<{id: string; contribution_id: string; role: string; status: string; session_id: string; started_at: string; finished_at: string | null; output: string}[]>("/agents");
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">EXECUTION VISIBILITY</span>
          <h1>Agents</h1>
          <p>
            Contributor and reviewer activity will originate from real execution
            events.
          </p>
        </div>
      </div>
      <section className="surface">
        <SectionHeader title="Connected agents" extra={<Bot size={17} />} />
        {controlError && <p className="error" role="alert">{controlError}</p>}
        {agents.error ? (
          <p className="error">{agents.error.message}</p>
        ) : agents.data?.length ? agents.data.map(a => <details key={a.id} className="command-result"><summary>{a.role} · {a.status} · {new Date(a.started_at).toLocaleString()}</summary><p>Contribution: {a.contribution_id}</p>{a.status === "RUNNING" && <button onClick={async () => { try { await request("/contributions/"+encodeURIComponent(a.contribution_id)+"/stop",{}); await cache.invalidateQueries(); } catch(e) { setControlError((e as Error).message); } }}>Stop this agent</button>}<p>Codex session: {a.session_id || "Starting"}</p><pre className="issue-body">{a.output || "Live activity is available in the contribution timeline."}</pre></details>) : (
          <Empty title="No Codex sessions yet">
            Approved contributions create real planner, contributor and independent reviewer sessions. Demo records do not launch agents.
          </Empty>
        )}
      </section>
    </>
  );
}
export function Usage() {
	const [manualAllowance, setManualAllowance] = useState(() => localStorage.getItem("forgeflow-manual-allowance") || "");
  const usage = useAPI<{
    sessions: number;
    allowance_remaining: number | null;
    message: string;
    observed_usage: Record<string,number>;
    duration_seconds: number;
  }>("/usage");
  const opportunities = useAPI<Opportunity[]>("/opportunities");
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">ESTIMATES, CLEARLY SEPARATED</span>
          <h1>Codex effort & usage</h1>
          <p>
            Plan allowance is unavailable. Heuristic effort never changes
            opportunity ranking.
          </p>
        </div>
      </div>
      <div className="usage-summary"><div><span className="eyebrow">OBSERVED INPUT TOKENS</span><strong>{usage.data?.observed_usage?.input_tokens ?? 0}</strong></div><div><span className="eyebrow">OBSERVED OUTPUT TOKENS</span><strong>{usage.data?.observed_usage?.output_tokens ?? 0}</strong></div><div><span className="eyebrow">SESSION DURATION</span><strong>{Math.round((usage.data?.duration_seconds ?? 0)/60)}m</strong></div></div>
      <div className="usage-summary">
        <div>
          <span className="eyebrow">OBSERVED LOCAL SESSIONS</span>
          <strong>{usage.data?.sessions ?? "—"}</strong>
        </div>
        <div>
          <span className="eyebrow">OFFICIAL REMAINING ALLOWANCE</span>
          <strong>Unavailable</strong>
          <p>No guessed percentages.</p>
		  <label>User supplied remaining allowance<input aria-label="User supplied remaining allowance" value={manualAllowance} maxLength={200} placeholder="For example: 30%, checked at 7 PM" onChange={e => {setManualAllowance(e.target.value); localStorage.setItem("forgeflow-manual-allowance", e.target.value);}} /></label><p className="muted">Stored in this browser. Self reported; never used for ranking.</p>
        </div>
      </div>
      {usage.error && <p className="error">{usage.error.message}</p>}
      <section className="surface">
        <SectionHeader title="Opportunity estimates" />
        {opportunities.error ? (
          <p className="error">{opportunities.error.message}</p>
        ) : opportunities.data?.length ? (
          opportunities.data.map((o) => (
            <div className="estimate-row" key={o.id}>
              <div>
                <strong>{o.repository}</strong>
                <p>
                  {o.estimate.files.join("–")} relevant files ·{" "}
                  {o.estimate.iterations.join("–")} coding iterations
                </p>
              </div>
              <Badge>
                {o.estimate.category.replaceAll("_", " ").toLowerCase()}
              </Badge>
              <span className="muted">
                {Math.round(o.estimate.confidence * 100)}% confidence
              </span>
            </div>
          ))
        ) : (
          <Empty title="No estimates yet">
            Estimates appear alongside analyzed opportunities.
          </Empty>
        )}
      </section>
    </>
  );
}
export function Settings({
  theme,
  setTheme,
}: {
  theme: string;
  setTheme: (v: string) => void;
}) {
  const runtime = useAPI<{contributions_root: string; codex_available: boolean; codex_model: string}>("/runtime");
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">YOUR LOCAL WORKBENCH</span>
          <h1>Settings</h1>
          <p>
            Personalize the interface. Operational settings live in
            Configuration.
          </p>
        </div>
      </div>
      <section className="surface">
        <SectionHeader title="Appearance" />
        <div className="settings-row">
          <label htmlFor="theme-select">Color theme</label>
          <select
            id="theme-select"
            value={theme}
            onChange={(e) => setTheme(e.target.value)}
          >
            <option value="dark">Dark</option>
            <option value="light">Light</option>
          </select>
        </div>
      </section>
      <section className="surface settings-safety">
        <SectionHeader title="Local runtime" />
        <dl className="key-values"><div><dt>Contribution root</dt><dd>{runtime.data?.contributions_root ?? "Loading…"}</dd></div><div><dt>Codex CLI</dt><dd>{runtime.data?.codex_available ? "Installed · uses local Codex authentication" : "Unavailable · run codex login after installing"}</dd></div><div><dt>Model</dt><dd>{runtime.data?.codex_model || "Codex CLI default"}</dd></div></dl>
        <p className="muted">Change the workspace root in configs/local.json or FORGEFLOW_CONTRIBUTIONS_DIR, then restart the server.</p>
      </section>
      <section className="surface settings-safety">
        <SectionHeader
          title="Approval boundaries"
          extra={<ShieldCheck size={17} />}
        />
        <p>
          Contribution execution requires Proceed to Contribute. Completed work
          stops at READY FOR HUMAN REVIEW. PR submission requires a separate
          human action. Automatic merging is forbidden.
        </p>
      </section>
    </>
  );
}
