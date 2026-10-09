import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  Bot,
  ShieldCheck,
  Sun,
  Folder,
  Terminal,
  Github,
  SlidersHorizontal,
} from "../components/icons";
import { request, useAPI } from "../api";
import { useQueryClient } from "@tanstack/react-query";
import type { Event, Opportunity } from "../types";
import {
  Badge,
  Empty,
  EventRows,
  SectionHeader,
} from "../components/primitives";
import { ProgressMeter, CopyValue } from "../components/Visuals";
import { Button } from "../components/ui/button";

export function Activity() {
  const events = useAPI<Event[]>("/events");
  const [search, setSearch] = useState("");
  const [type, setType] = useState("");
  const filtered = (events.data ?? []).filter(
    (e) =>
      (!type || e.type === type) &&
      `${e.message} ${e.type} ${e.entity_id} ${e.actor}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">YOUR WORKSPACE HISTORY</span>
          <h1>Activity</h1>
          <p>
            Every decision, command and state change. The latest 100 persisted
            events.
          </p>
        </div>
        <Badge>Append-only audit</Badge>
      </div>
      <section className="surface">
        <SectionHeader
          title="Event timeline"
          extra={<Badge>{filtered.length} events</Badge>}
        />
        <div className="activity-filters">
          <input
            aria-label="Search activity"
            placeholder="Search events, workspaces or commands..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <select
            aria-label="Filter event type"
            value={type}
            onChange={(e) => setType(e.target.value)}
          >
            <option value="">All event types</option>
            {[...new Set((events.data ?? []).map((e) => e.type))]
              .sort()
              .map((name) => (
                <option key={name} value={name}>
                  {name.replace(/([a-z])([A-Z])/g, "$1 $2")}
                </option>
              ))}
          </select>
        </div>
        {events.error ? (
          <p className="error" role="alert">
            {events.error.message}
          </p>
        ) : events.isPending ? (
          <div className="skeleton" aria-label="Loading activity" />
        ) : filtered.length ? (
          <EventRows events={filtered} />
        ) : (
          <Empty title="No matching events">
            Try another search or event type. Your persisted history is
            unchanged.
          </Empty>
        )}
      </section>
    </>
  );
}

export function Agents() {
  const cache = useQueryClient();
  const [controlError, setControlError] = useState("");
  const [busy, setBusy] = useState("");
  const [filter, setFilter] = useState("All");
  const [now, setNow] = useState(Date.now());
  useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 30_000); return () => clearInterval(timer); }, []);
  const agents =
    useAPI<
      {
        id: string;
        contribution_id: string;
        role: string;
        model?: string;
        status: string;
        session_id: string;
        started_at: string;
        finished_at: string | null;
        output: string;
      }[]
    >("/agents");
  const filtered = (agents.data ?? []).filter(
    (a) =>
      filter === "All" ||
      (filter === "Running" ? a.status === "RUNNING" : a.status !== "RUNNING"),
  );
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">EXECUTION VISIBILITY</span>
          <h1>Agents</h1>
          <p>Real Codex sessions, their workspace and recorded output.</p>
        </div>
        <div className="segmented-control">
          {["All", "Running", "Finished"].map((name) => (
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
      <div className="agent-summary" aria-label="Agent run summary">
        <div><span>Running sessions</span><strong>{agents.data ? agents.data.filter(a => a.status === "RUNNING").length : "—"}</strong><p>Active processes in isolated contribution workspaces.</p></div>
        <div><span>Historical runs</span><strong>{agents.data ? agents.data.filter(a => a.status !== "RUNNING").length : "—"}</strong><p>Completed and stopped runs remain available for inspection.</p></div>
      </div>
      {controlError && (
        <p className="error" role="alert">
          {controlError}
        </p>
      )}
      {agents.error ? (
        <p className="error" role="alert">
          {agents.error.message}
        </p>
      ) : agents.isPending ? (
        <div className="skeleton" aria-label="Loading agents" />
      ) : filtered.length ? (
        <div className="agent-grid">
          {filtered.map((a) => (
            <article key={a.id} className="surface agent-card">
              <div className="agent-card-heading">
                <span className="branch-icon">
                  <Bot size={23} />
                </span>
                <div>
                  <h2>{a.role}</h2>
                  <p>{new Date(a.started_at).toLocaleString()}</p>
                  {a.model && <p>Model: {a.model}</p>}
                  {Number.isFinite(Date.parse(a.started_at)) && <span className="agent-elapsed">{a.finished_at && Number.isFinite(Date.parse(a.finished_at)) ? Math.max(0, Math.floor((Date.parse(a.finished_at) - Date.parse(a.started_at)) / 60_000)) + "m recorded" : a.status === "RUNNING" ? Math.max(0, Math.floor((now - Date.parse(a.started_at)) / 60_000)) + "m elapsed" : "Duration not recorded"}</span>}
                </div>
                <Badge tone={a.status === "RUNNING" ? "violet" : "neutral"}>
                  {a.status}
                </Badge>
              </div>
              {a.status === "RUNNING" && (
                <>
                  <ProgressMeter indeterminate label="Agent process running" />
                  <p style={{ marginTop: 12 }}>
                    Process marked running. Progress is recorded in the
                    workspace timeline.
                  </p>
                </>
              )}
              <Link
                className="text-link"
                to={`/contributions/${encodeURIComponent(a.contribution_id)}`}
              >
                Open contribution workspace
              </Link>
              <details>
                <summary>Session details & output</summary>
                <p>Session: {a.session_id || "Starting"}</p>
                <p>Contribution: {a.contribution_id}</p>
                <pre className="issue-body">
                  {a.output ||
                    "Live activity is available in the contribution timeline."}
                </pre>
              </details>
              {a.status === "RUNNING" && (
                <Button
                  variant="secondary"
                  disabled={!!busy}
                  onClick={async () => {
                    setBusy(a.id);
                    setControlError("");
                    try {
                      await request(
                        `/contributions/${encodeURIComponent(a.contribution_id)}/stop`,
                        {},
                      );
                      await cache.invalidateQueries();
                    } catch (e) {
                      setControlError((e as Error).message);
                    } finally {
                      setBusy("");
                    }
                  }}
                >
                  Stop this agent
                </Button>
              )}
            </article>
          ))}
        </div>
      ) : (
        <section className="surface">
          <Empty title="No Codex sessions yet">
            Approved contributions create planner, contributor and independent
            reviewer sessions. Demo records do not launch agents.
          </Empty>
        </section>
      )}
    </>
  );
}
export function Usage() {
  const [manualAllowance, setManualAllowance] = useState(
    () => localStorage.getItem("forgeflow-manual-allowance") || "",
  );
  const usage = useAPI<{
    sessions: number;
    allowance_remaining: number | null;
    message: string;
    observed_usage: Record<string, number>;
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
      <div className="usage-summary usage-observed">
        <div>
          <span className="eyebrow">OBSERVED INPUT TOKENS</span>
          <strong>{usage.data?.observed_usage?.input_tokens ?? "—"}</strong>
        </div>
        <div>
          <span className="eyebrow">OBSERVED OUTPUT TOKENS</span>
          <strong>{usage.data?.observed_usage?.output_tokens ?? "—"}</strong>
        </div>
        <div>
          <span className="eyebrow">SESSION DURATION</span>
          <strong>
            {usage.data
              ? `${Math.round(usage.data.duration_seconds / 60)}m`
              : "—"}
          </strong>
        </div>
      </div>
      <div className="usage-summary usage-allowance">
        <div>
          <span className="eyebrow">OBSERVED LOCAL SESSIONS</span>
          <strong>{usage.data?.sessions ?? "—"}</strong>
        </div>
        <div>
          <span className="eyebrow">OFFICIAL REMAINING ALLOWANCE</span>
          <strong>Unavailable</strong>
          <p>No guessed percentages.</p>
          <label>
            User supplied remaining allowance
            <input
              aria-label="User supplied remaining allowance"
              value={manualAllowance}
              maxLength={200}
              placeholder="For example: 30%, checked at 7 PM"
              onChange={(e) => {
                setManualAllowance(e.target.value);
                localStorage.setItem(
                  "forgeflow-manual-allowance",
                  e.target.value,
                );
              }}
            />
          </label>
          <p className="muted">
            Stored in this browser. Self reported; never used for ranking.
          </p>
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
  const runtime = useAPI<{
    contributions_root: string;
    codex_available: boolean;
    codex_model: string;
  }>("/runtime");
  const account = useAPI<{ connected: boolean; login: string }>("/auth/github");
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">MAKE IT YOURS</span>
          <h1>Settings</h1>
          <p>Your interface, local workspace and connected account.</p>
        </div>
      </div>
      <div className="settings-layout">
        <nav className="settings-sections" aria-label="Settings sections">
          <a href="#appearance">
            <Sun size={18} />
            Appearance
          </a>
          <a href="#workspace">
            <Folder size={18} />
            Workspace
          </a>
          <a href="#execution">
            <Terminal size={18} />
            Execution
          </a>
          <a href="#github">
            <Github size={18} />
            GitHub
          </a>
          <a href="#approval">
            <ShieldCheck size={18} />
            Approval
          </a>
        </nav>
        <div className="settings-content">
          <section className="surface" id="appearance">
            <SectionHeader title="Appearance" />
            <div className="settings-row">
              <label htmlFor="theme-select">Color theme</label>
              <select
                id="theme-select"
                value={theme}
                onChange={(e) => setTheme(e.target.value)}
              >
                <option value="light">Light</option>
                <option value="dark">Dark</option>
                <option value="system">System</option>
              </select>
            </div>
            <div className="theme-options">
              {["light", "dark", "system"].map((name) => (
                <button
                  className="theme-option"
                  key={name}
                  aria-label={`Use ${name} theme`}
                  aria-pressed={theme === name}
                  onClick={() => setTheme(name)}
                >
                  <span className={`theme-preview theme-preview-${name}`} />
                  <span className="capitalize">{name}</span>
                </button>
              ))}
            </div>
          </section>
          <section className="surface" id="workspace">
            <SectionHeader
              title="Workspace location"
              extra={<Folder size={18} />}
            />
            <dl className="key-values">
              <div>
                <dt>Effective contribution root</dt>
                <dd>
                  {runtime.data ? (
                    <CopyValue value={runtime.data.contributions_root} />
                  ) : (
                    "Unavailable"
                  )}
                </dd>
              </div>
            </dl>
            <p className="muted">
              Each contribution uses its own &lt;id&gt;/repo directory. Change
              the root in configs/local.json or FORGEFLOW_CONTRIBUTIONS_DIR,
              then restart the server.
            </p>
            {runtime.error && (
              <p className="error" role="alert">
                {runtime.error.message}
              </p>
            )}
          </section>
          <section className="surface" id="execution">
            <SectionHeader
              title="Local execution"
              extra={<Terminal size={18} />}
            />
            <dl className="key-values">
              <div>
                <dt>Codex CLI</dt>
                <dd>
                  {runtime.data
                    ? runtime.data.codex_available
                      ? "Installed · local Codex authentication"
                      : "Unavailable · install Codex and run codex login"
                    : "Checking runtime..."}
                </dd>
              </div>
              <div>
                <dt>Default model for new contributions</dt>
                <dd>{runtime.data?.codex_model || "Codex CLI default"}</dd>
              </div>
            </dl>
            <p className="muted">Choose a model for each contribution when reviewing its approval preview. That choice is saved with the contribution.</p>
            <p>
              <Link className="text-link" to="/configuration">
                <SlidersHorizontal size={15} />
                Manage execution limits in Configuration
              </Link>
            </p>
          </section>
          <section className="surface" id="github">
            <SectionHeader
              title="GitHub connection"
              extra={<Github size={18} />}
            />
            <p>
              {account.data
                ? account.data.connected
                  ? `Connected${account.data.login ? ` as ${account.data.login}` : ""}`
                  : "No GitHub account connected"
                : "Checking account?"}
            </p>
            <p>
              <Link className="text-link" to="/login">
                Manage GitHub account
              </Link>
            </p>
          </section>
          <section className="surface" id="approval">
            <SectionHeader
              title="Approval boundaries"
              extra={<ShieldCheck size={18} />}
            />
            <p>
              You select each contribution before coding starts. Completed work
              stops for human review. Publishing a PR requires separate
              approval. Automatic merging is forbidden.
            </p>
          </section>
        </div>
      </div>
    </>
  );
}
