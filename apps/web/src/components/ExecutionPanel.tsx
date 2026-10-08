import { useState } from "react";
import LiveExecutionProgress from "./LiveExecutionProgress";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { request } from "../api";
import type { Contribution } from "../types";
import { Button } from "./ui/button";
import { CircleCheck, Terminal, Clock, ShieldCheck } from "./icons";
import { ProgressMeter } from "./Visuals";

type Execution = {
  status: string;
  phase?: string;
  message?: string;
  summary: string;
  fix_iterations: number;
  review_cycles: number;
  network: boolean;
  constraints?: string;
  plan?: {
    summary: string;
    root_cause: string;
    strategy: string;
    files: string[];
    risks: string[];
    unknowns: string[];
  };
  review?: {
    verdict: string;
    summary: string;
    findings: {
      severity: string;
      file: string;
      line: number;
      explanation: string;
      recommended_fix: string;
    }[];
  };
  diff: string;
  report: string;
  pr_title: string;
  pr_body: string;
  submission_token: string;
  pr_url: string;
};
type TestRun = {
  id: string;
  command: { program: string; arguments: string[] };
  exit_code: number;
  output: string;
  started_at: string;
  finished_at: string;
  truncated: boolean;
};
export default function ExecutionPanel({
  contribution: c,
}: {
  contribution: Contribution;
}) {
  const prefix = "/contributions/" + encodeURIComponent(c.id);
  const execution = useQuery({
    queryKey: [prefix + "/execution"],
    queryFn: () => request<Execution>(prefix + "/execution"),
    refetchInterval: 3000,
  });
  const tests = useQuery({
    queryKey: [prefix + "/tests"],
    queryFn: () => request<TestRun[]>(prefix + "/tests"),
    refetchInterval: 3000,
  });
  const [tab, setTab] = useState("Plan");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [approved, setApproved] = useState(false);
  const [submitApproved, setSubmitApproved] = useState(false);
  const [network, setNetwork] = useState(false);
  const [constraints, setConstraints] = useState<string | null>(null);
  const cache = useQueryClient();
  const r = execution.data;
  const running = r?.status === "RUNNING";
  async function act(action: string, body: unknown = {}) {
    setBusy(true);
    setError("");
    try {
      await request(prefix + "/" + action, body);
      setApproved(false);
      setSubmitApproved(false);
      await cache.invalidateQueries();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  const recorded = (tests.data ?? []).filter((t) => !!t.finished_at);
  const passed = recorded.filter((t) => t.exit_code === 0).length;
  const failed = recorded.length - passed;
  const duration = recorded.reduce(
    (sum, t) =>
      sum + Math.max(0, Date.parse(t.finished_at) - Date.parse(t.started_at)),
    0,
  );
  return (
    <section className="execution-panel">
      <h3>Contribution execution</h3>
      <p className="muted">
        {r?.message ||
          (running
            ? "Execution running: " + c.state.toLowerCase()
            : r?.summary) ||
          "Start Codex planning, coding, real verification and independent review in this isolated workspace."}
      </p>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {execution.error && <p className="error">{execution.error.message}</p>}
      {!running && ["BLOCKED", "PAUSED", "PLANNING"].includes(c.state) && (
        <>
          <label className="approval-checkbox">
            <input
              type="checkbox"
              checked={approved}
              onChange={(e) => setApproved(e.target.checked)}
            />{" "}
            I approve Codex coding, verification and independent review in this
            workspace.
          </label>
          <label className="approval-checkbox">
            <input
              type="checkbox"
              checked={network}
              onChange={(e) => setNetwork(e.target.checked)}
            />{" "}
            Allow dependency downloads during execution.
          </label>
        </>
      )}
      <div className="execution-controls">
        <Button
          variant="ghost"
          disabled={busy}
          onClick={() => act("open-workspace")}
        >
          Open workspace folder
        </Button>
        {!running && ["BLOCKED", "PAUSED", "PLANNING"].includes(c.state) && (
          <Button
            disabled={
              busy || !approved || r?.status === "AWAITING_PLAN_APPROVAL"
            }
            onClick={() => act("start", { approved: true, network })}
          >
            {r?.status === "NOT_STARTED" ? "Start coding" : "Resume execution"}
          </Button>
        )}
        {r?.status === "AWAITING_PLAN_APPROVAL" && (
          <Button
            disabled={busy}
            onClick={() => act("approve-plan", { approved: true })}
          >
            Approve implementation plan
          </Button>
        )}
        {running && (
          <>
            <Button
              variant="secondary"
              disabled={busy}
              onClick={() => act("pause")}
            >
              Pause contribution
            </Button>
            <Button variant="ghost" disabled={busy} onClick={() => act("stop")}>
              Stop agent
            </Button>
          </>
        )}
        {!running &&
          r?.plan &&
          ["READY", "BLOCKED", "PAUSED"].includes(c.state) && (
            <>
              <Button
                variant="ghost"
                disabled={busy}
                onClick={() => act("tests", { approved: true })}
              >
                Retry tests
              </Button>
              <Button
                variant="ghost"
                disabled={busy}
                onClick={() => act("review", { approved: true })}
              >
                Request independent review
              </Button>
            </>
          )}
        {c.state === "READY" && (
          <Button disabled={busy} onClick={() => act("prepare-pr")}>
            Prepare local commit & PR
          </Button>
        )}
        {!["PR_OPENED", "FAILED", "ABANDONED"].includes(c.state) && (
          <Button
            variant="ghost"
            disabled={busy}
            onClick={() => {
              if (
                window.confirm(
                  "Abandon this contribution? Its workspace and audit history will be preserved.",
                )
              )
                void act("abandon", { approved: true });
            }}
          >
            Abandon contribution
          </Button>
        )}
      </div>
      {!running && ["PLANNING", "PAUSED", "BLOCKED"].includes(c.state) && (
        <details className="command-result">
          <summary>Contribution constraints</summary>
          <label className="settings-row">
            Instructions for this contribution
            <textarea
              aria-label="Contribution constraints"
              maxLength={4000}
              value={constraints ?? r?.constraints ?? ""}
              onChange={(e) => setConstraints(e.target.value)}
              placeholder="Keep the change small; preserve the public API..."
            />
          </label>
          <Button
            disabled={busy}
            onClick={() =>
              act("constraints", {
                approved: true,
                constraints: constraints ?? r?.constraints ?? "",
              })
            }
          >
            Save contribution constraints
          </Button>
        </details>
      )}
      <div
        className="execution-tabs"
        role="tablist"
        aria-label="Contribution evidence"
      >
        {["Plan", "Tests", "Review", "Diff", "Report", "PR"].map((name) => (
          <button
            key={name}
            id={`evidence-tab-${name}`}
            role="tab"
            aria-controls="evidence-panel"
            tabIndex={tab === name ? 0 : -1}
            aria-selected={tab === name}
            onClick={() => setTab(name)}
            onKeyDown={(event) => {
              const names = ["Plan", "Tests", "Review", "Diff", "Report", "PR"];
              const index = names.indexOf(name);
              const next =
                event.key === "ArrowRight"
                  ? names[(index + 1) % names.length]
                  : event.key === "ArrowLeft"
                    ? names[(index + names.length - 1) % names.length]
                    : event.key === "Home"
                      ? names[0]
                      : event.key === "End"
                        ? names[names.length - 1]
                        : null;
              if (next) {
                event.preventDefault();
                setTab(next);
                document.getElementById(`evidence-tab-${next}`)?.focus();
              }
            }}
          >
            {name}
            {name === "Tests" ? " (" + (tests.data?.length ?? 0) + ")" : ""}
          </button>
        ))}
      </div>
      <div className="execution-workbench">
        <div className="execution-evidence">
          {running && (
            <LiveExecutionProgress id={c.id} phase={r.phase ?? c.state} />
          )}
          <div
            className="evidence-content"
            id="evidence-panel"
            role="tabpanel"
            aria-labelledby={`evidence-tab-${tab}`}
            key={tab}
          >
            {tab === "Plan" &&
              (r?.plan ? (
                <>
                  <h4>{r.plan.summary}</h4>
                  <p>{r.plan.root_cause}</p>
                  <p>{r.plan.strategy}</p>
                  <p className="muted">Files: {r.plan.files.join(", ")}</p>
                  <p>Risks: {r.plan.risks.join("; ")}</p>
                  <p>Unknowns: {r.plan.unknowns.join("; ")}</p>
                </>
              ) : (
                <p className="muted">
                  A plan will be saved before code is edited.
                </p>
              ))}
            {tab === "Tests" && (
              <>
                {tests.error && <p className="error">{tests.error.message}</p>}
                {!tests.data?.length && (
                  <p className="muted">
                    No repository verification has run yet.
                  </p>
                )}
                {tests.data?.map((t) => (
                  <details key={t.id} className="command-result">
                    <summary>
                      {t.command.program} {t.command.arguments.join(" ")} · exit{" "}
                      {t.exit_code} ·{" "}
                      {Math.round(
                        (Date.parse(t.finished_at) - Date.parse(t.started_at)) /
                          1000,
                      )}
                      s
                    </summary>
                    <pre className="issue-body">{t.output || "No output"}</pre>
                    {t.truncated && <p>Output truncated at 256 KiB.</p>}
                  </details>
                ))}
              </>
            )}
            {tab === "Review" &&
              (r?.review ? (
                <>
                  <h4>{r.review.verdict}</h4>
                  <p>{r.review.summary}</p>
                  {r.review.findings.map((f, i) => (
                    <div className="review-finding" key={i}>
                      <b>
                        {f.severity} · {f.file}:{f.line}
                      </b>
                      <p>{f.explanation}</p>
                      <p>{f.recommended_fix}</p>
                    </div>
                  ))}
                </>
              ) : (
                <p className="muted">
                  Independent review starts after real verification passes.
                </p>
              ))}
            {tab === "Diff" &&
              (r?.diff ? (
                <pre className="diff-view">
                  {r.diff.split("\n").map((line, i) => (
                    <span
                      key={i}
                      className={
                        line.startsWith("+")
                          ? "diff-add"
                          : line.startsWith("-")
                            ? "diff-remove"
                            : line.startsWith("@@")
                              ? "diff-section"
                              : ""
                      }
                    >
                      {line || " "}
                      {"\n"}
                    </span>
                  ))}
                </pre>
              ) : (
                <p className="muted">The reviewed diff appears here.</p>
              ))}
            {tab === "Report" && (
              <pre className="issue-body">
                {r?.report ||
                  "Final report is generated after tests and independent review."}
              </pre>
            )}
            {tab === "PR" && (
              <>
                {r?.pr_title ? (
                  <>
                    <h4>{r.pr_title}</h4>
                    <pre className="issue-body">{r.pr_body}</pre>
                    {c.state === "PR_PREPARED" && (
                      <>
                        <label className="approval-checkbox">
                          <input
                            type="checkbox"
                            checked={submitApproved}
                            onChange={(e) =>
                              setSubmitApproved(e.target.checked)
                            }
                          />{" "}
                          I approve creating or using my fork, pushing this
                          reviewed branch and submitting this PR to GitHub.
                        </label>
                        <Button
                          disabled={busy || !submitApproved}
                          onClick={() =>
                            act("submit-pr", {
                              approved: true,
                              token: r.submission_token,
                            })
                          }
                        >
                          Submit PR to GitHub
                        </Button>
                      </>
                    )}
                    {r.pr_url && (
                      <a href={r.pr_url} target="_blank" rel="noreferrer">
                        Open submitted PR
                      </a>
                    )}
                  </>
                ) : (
                  <p className="muted">
                    PR title/body are generated with the final report.
                    Submission always requires your explicit approval.
                  </p>
                )}
              </>
            )}
          </div>
        </div>
        <aside
          className="verification-summary surface"
          aria-label="Verification summary"
        >
          <div className="section-header">
            <h2>Verification evidence</h2>
            <Terminal size={18} />
          </div>
          <div className="verification-body">
            <span className="eyebrow">RECORDED COMMAND RUNS</span>
            <div className="verification-total">
              <strong>{tests.data ? recorded.length : "—"}</strong>
              <span>completed</span>
            </div>
            {tests.data && (
              <ProgressMeter
                value={recorded.length ? (passed / recorded.length) * 100 : 0}
                label="Successful recorded command runs"
              />
            )}
            <div className="verification-counts">
              <span>
                <CircleCheck size={17} />
                {tests.data ? passed : "—"} passed
              </span>
              <span className={failed ? "error" : "muted"}>
                {tests.data ? failed : "—"} failed
              </span>
            </div>
            <p className="muted">
              Recorded history, including earlier attempts. A passing command
              does not establish final review approval.
            </p>
            <dl className="verification-facts">
              <div>
                <dt>
                  <Clock size={16} /> Total command time
                </dt>
                <dd>{Math.round(duration / 1000)}s</dd>
              </div>
              <div>
                <dt>
                  <ShieldCheck size={16} /> Independent review
                </dt>
                <dd>{r?.review?.verdict || "Not recorded"}</dd>
              </div>
              <div>
                <dt>Fix iterations</dt>
                <dd>{r?.fix_iterations ?? "—"}</dd>
              </div>
            </dl>
            {tests.error && (
              <p className="error" role="alert">
                Verification evidence unavailable: {tests.error.message}
              </p>
            )}
            {!tests.data && !tests.error && (
              <p className="muted">Loading command history...</p>
            )}
          </div>
        </aside>
      </div>
      {r?.plan && (
        <p className="muted">
          Fix iterations: {r.fix_iterations} · Independent review cycles:{" "}
          {r.review_cycles}
        </p>
      )}
    </section>
  );
}
