import { useState } from "react";
import { Bot, GitBranch, ShieldCheck } from "lucide-react";
import { useAPI } from "../api";
import type { Contribution, Event, Opportunity } from "../types";
import {
  Badge,
  Empty,
  EventRows,
  SectionHeader,
  StateBadge,
} from "../components/primitives";
import { Dialog } from "../components/ui/dialog";
export function Contributions() {
  const cs = useAPI<Contribution[]>("/contributions");
  const events = useAPI<Event[]>("/events");
  const [selected, setSelected] = useState<Contribution | null>(null);
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
            The workspace manager and contributor runner are later milestones.
          </Empty>
        ) : (
          cs.data.map((c) => (
            <button
              className="contribution-row contribution-button"
              onClick={() => setSelected(c)}
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
        open={!!selected}
        onOpenChange={(v) => {
          if (!v) setSelected(null);
        }}
        title={selected?.repository ?? "Contribution"}
        description="Persisted contribution record · no execution adapter connected"
        wide
      >
        {selected && (
          <>
            <StateBadge state={selected.state} />
            <p className="summary">{selected.title}</p>
            {selected.demo && (
              <div className="note">
                Illustrative demo state. No repository was cloned, no agent ran
                and no contribution tests or reviews were performed.
              </div>
            )}
            <dl className="key-values">
              <div>
                <dt>Contribution ID</dt>
                <dd>{selected.id}</dd>
              </div>
              <div>
                <dt>Branch (illustrative)</dt>
                <dd>{selected.branch}</dd>
              </div>
              <div>
                <dt>Configuration snapshot</dt>
                <dd>v{selected.config_version}</dd>
              </div>
            </dl>
            <h3>Persisted timeline</h3>
            <EventRows
              events={(events.data ?? []).filter(
                (e) => e.entity_id === selected.id,
              )}
            />
            <p className="muted">
              Execution controls will be enabled when the workspace manager and
              runner are available.
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
  const agents = useAPI<unknown[]>("/agents");
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
        {agents.error ? (
          <p className="error">{agents.error.message}</p>
        ) : (
          <Empty title="No execution adapter connected">
            Codex contributor and independent reviewer integration are later
            milestones. Demo contribution states do not represent active agents.
          </Empty>
        )}
      </section>
    </>
  );
}
export function Usage() {
  const usage = useAPI<{
    sessions: number;
    allowance_remaining: number | null;
    message: string;
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
      <div className="usage-summary">
        <div>
          <span className="eyebrow">OBSERVED LOCAL SESSIONS</span>
          <strong>{usage.data?.sessions ?? "—"}</strong>
        </div>
        <div>
          <span className="eyebrow">OFFICIAL REMAINING ALLOWANCE</span>
          <strong>Unavailable</strong>
          <p>No guessed percentages.</p>
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
