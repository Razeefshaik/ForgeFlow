import { useEffect, useState } from "react";
import LiveExecutionProgress from "./LiveExecutionProgress";
import IssueReactor, { type ExecutionIncident } from "./IssueReactor";
import ContributionStages from "./ContributionStages";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { request } from "../api";
import type { Contribution } from "../types";
import { Button } from "./ui/button";
import {
  CircleCheck,
  Terminal,
  Clock,
  ShieldCheck,
  Folder,
  CirclePause,
  ChevronRight,
  Search,
} from "./icons";

type Execution = {
  status: string;
  phase?: string;
  message?: string;
  summary: string;
  fix_iterations: number;
  review_cycles: number;
  network: boolean;
  incident?: ExecutionIncident;
  recovery_attempts?: number;
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
  failure_kind?: string;
  recovery_of?: string;
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
  const [inspectedStage, setInspectedStage] = useState<string | null>(null);
  const [evidenceTest, setEvidenceTest] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [approvalOpen, setApprovalOpen] = useState(false);
  const [optionsOpen, setOptionsOpen] = useState(false);
  const [notice, setNotice] = useState("");
  const [testFilter, setTestFilter] = useState("All");
  const [testSearch, setTestSearch] = useState("");
  const [error, setError] = useState("");
  const [approved, setApproved] = useState(false);
  const [submitApproved, setSubmitApproved] = useState(false);
  const [network, setNetwork] = useState(false);
  const [constraints, setConstraints] = useState<string | null>(null);
  const cache = useQueryClient();
  const r = execution.data;
  const running = r?.status === "RUNNING";
  const diagnosis = useQuery({
    queryKey: [prefix + "/diagnosis"],
    queryFn: () => request<ExecutionIncident | null>(prefix + "/diagnosis"),
    enabled: r?.status === "BLOCKED" && !r.incident,
    refetchInterval: r?.status === "BLOCKED" ? 5000 : false,
  });
  const currentIncident =
    r?.incident ?? (r?.status === "BLOCKED" ? diagnosis.data : null);
  useEffect(() => {
    if (tab !== "Tests" || !evidenceTest) return;
    const details = document.getElementById(
      "test-run-" + evidenceTest,
    ) as HTMLDetailsElement | null;
    if (details) {
      details.open = true;
      details.querySelector("summary")?.focus({ preventScroll: true });
      details.scrollIntoView({ block: "nearest" });
      setEvidenceTest(null);
    }
  }, [tab, evidenceTest, tests.data]);
  async function act(action: string, body: unknown = {}) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await request(prefix + "/" + action, body);
      setApproved(false);
      setSubmitApproved(false);
      setApprovalOpen(false);
      setNotice(
        action === "open-workspace"
          ? "Workspace opened."
          : action === "constraints"
            ? "Contribution instructions saved."
            : "Action accepted. Refreshing execution status…",
      );
      await cache.invalidateQueries();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  const recorded = (tests.data ?? []).filter((t) => !!t.finished_at);
  const passed = recorded.filter((t) => t.exit_code === 0).length;
  const setupBlocked = recorded.filter(
    (t) =>
      t.failure_kind && !["code_failure", "cancelled"].includes(t.failure_kind),
  ).length;
  const interrupted = recorded.filter(
    (t) => t.failure_kind === "cancelled",
  ).length;
  const failed = recorded.length - passed - setupBlocked - interrupted;
  const strategy = r?.plan?.strategy ?? "";
  const testStatus = (t: TestRun) =>
    !t.finished_at
      ? "Running"
      : t.failure_kind === "cancelled"
        ? "Interrupted"
        : t.exit_code === 0
          ? "Passed"
          : t.failure_kind && t.failure_kind !== "code_failure"
            ? "Setup blocked"
            : "Failed";
  const visibleTests = [...(tests.data ?? [])]
    .reverse()
    .filter(
      (t) =>
        (testFilter === "All" || testStatus(t) === testFilter) &&
        (
          t.command.program +
          " " +
          t.command.arguments.join(" ") +
          " " +
          t.output
        )
          .toLowerCase()
          .includes(testSearch.toLowerCase()),
    );
  const duration = recorded.reduce(
    (sum, t) =>
      sum + Math.max(0, Date.parse(t.finished_at) - Date.parse(t.started_at)),
    0,
  );
  return (
    <section className="execution-panel">
      <ContributionStages
        contribution={c}
        execution={r}
        inspectedStage={inspectedStage}
        onSelect={(name, stage) => {
          setInspectedStage(stage);
          setTab(name);
          requestAnimationFrame(() =>
            document.getElementById(`evidence-tab-${name}`)?.focus(),
          );
        }}
      />
      <div className="execution-commandbar">
        <div className="execution-task">
          <span className="task-caption">Current task</span>
          <h2>
            {running
              ? ({
                  PLANNING: "Investigating the issue",
                  CODING: "Implementing changes",
                  TESTING: "Running verification",
                  FIXING: "Fixing the latest findings",
                  REVIEWING: "Independent review",
                }[r.phase ?? c.state] ?? "Contribution in progress")
              : r?.status === "BLOCKED"
                ? "Execution needs attention"
                : c.state === "PAUSED"
                  ? "Execution paused"
                  : r?.status === "AWAITING_PLAN_APPROVAL"
                    ? "Review the implementation plan"
                    : c.state === "READY"
                      ? "Ready for your review"
                      : c.state === "PR_OPENED"
                        ? "Pull request submitted"
                        : c.state === "PR_PREPARED"
                          ? "Approve PR submission"
                          : "Contribution workspace"}
          </h2>
          <p>
            {c.state === "PR_OPENED" && !running
              ? "The pull request has been submitted. Open the PR tab to inspect its recorded link and review evidence."
              : r?.status === "BLOCKED" && currentIncident
                ? "Your changes and execution evidence are saved."
                : r?.message ||
                  (running
                    ? "Execution running: " + c.state.toLowerCase()
                    : r?.summary) ||
                  c.message ||
                  "Plan, implement, verify, then review the contribution."}
          </p>
        </div>
        <div className="execution-primary-actions">
          <Button
            variant="secondary"
            disabled={busy}
            onClick={() => act("open-workspace")}
          >
            <Folder size={16} />
            Open workspace
          </Button>
          {!running &&
            ["BLOCKED", "PAUSED", "PLANNING"].includes(c.state) &&
            r?.status !== "AWAITING_PLAN_APPROVAL" && (
              <Button
                variant={currentIncident ? "secondary" : "default"}
                disabled={busy}
                aria-expanded={approvalOpen}
                aria-controls="execution-approval"
                onClick={() => setApprovalOpen(!approvalOpen)}
              >
                {r?.status === "NOT_STARTED"
                  ? "Start coding"
                  : "Resume execution"}
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
            <Button
              variant="secondary"
              disabled={busy}
              onClick={() => act("pause")}
            >
              <CirclePause size={16} />
              Pause contribution
            </Button>
          )}
          {c.state === "READY" && !running && (
            <Button disabled={busy} onClick={() => act("prepare-pr")}>
              Prepare local commit & PR
            </Button>
          )}
          {c.state === "PR_PREPARED" && (
            <Button onClick={() => setTab("PR")}>Review submission</Button>
          )}
          <Button
            variant="ghost"
            aria-expanded={optionsOpen}
            aria-controls="execution-options"
            onClick={() => setOptionsOpen(!optionsOpen)}
          >
            More actions
            <ChevronRight
              size={14}
              className={optionsOpen ? "disclosure-open" : ""}
            />
          </Button>
        </div>
      </div>
      {error && (
        <p className="error action-feedback" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="action-feedback" role="status">
          <CircleCheck size={16} />
          {notice}
        </p>
      )}
      {execution.error && (
        <p className="error" role="alert">
          Execution status could not refresh: {execution.error.message}
        </p>
      )}
      {approvalOpen &&
        !running &&
        ["BLOCKED", "PAUSED", "PLANNING"].includes(c.state) && (
          <section
            className="execution-approval"
            id="execution-approval"
            aria-label="Execution approval"
          >
            <h3>Resume with your approval</h3>
            <p className="approval-context">
              <strong>{c.repository}</strong> · <code>{c.id}</code>
            </p>
            <p>
              Codex can edit this contribution's isolated workspace and run its
              required checks.
            </p>
            <label className="approval-checkbox">
              <input
                type="checkbox"
                checked={approved}
                onChange={(e) => setApproved(e.target.checked)}
              />
              I approve Codex coding, verification and independent review in
              this workspace.
            </label>
            <label className="approval-checkbox">
              <input
                type="checkbox"
                checked={network}
                onChange={(e) => setNetwork(e.target.checked)}
              />
              Allow dependency downloads during execution.
            </label>
            <div className="approval-actions">
              <Button
                disabled={
                  busy || !approved || r?.status === "AWAITING_PLAN_APPROVAL"
                }
                onClick={() => act("start", { approved: true, network })}
              >
                Approve and resume
              </Button>
              <Button variant="ghost" onClick={() => setApprovalOpen(false)}>
                Cancel
              </Button>
            </div>
          </section>
        )}
      {optionsOpen && (
        <section
          className="execution-options"
          id="execution-options"
          aria-label="More contribution actions"
        >
          <div className="execution-controls">
            {running && (
              <Button
                variant="secondary"
                disabled={busy}
                onClick={() => act("stop")}
              >
                Stop agent
              </Button>
            )}
            {!running &&
              r?.plan &&
              ["READY", "BLOCKED", "PAUSED"].includes(c.state) && (
                <>
                  <Button
                    variant="secondary"
                    disabled={busy}
                    onClick={() => act("tests", { approved: true })}
                  >
                    Retry tests
                  </Button>
                  <Button
                    variant="secondary"
                    disabled={busy}
                    onClick={() => act("review", { approved: true })}
                  >
                    Request independent review
                  </Button>
                </>
              )}
            {!["PR_OPENED", "FAILED", "ABANDONED"].includes(c.state) && (
              <Button
                variant="ghost"
                className="danger-action"
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
          <p className="muted">
            Pause interrupts execution and keeps the contribution resumable.
            Stop agent ends the active run. Abandon ends the contribution and
            preserves its files and history.
          </p>
          {!running && ["PLANNING", "PAUSED", "BLOCKED"].includes(c.state) && (
            <details className="command-result">
              <summary>Edit contribution instructions</summary>
              <label className="settings-row">
                Instructions for this contribution
                <textarea
                  aria-label="Contribution constraints"
                  maxLength={4000}
                  value={constraints ?? r?.constraints ?? ""}
                  onChange={(e) => setConstraints(e.target.value)}
                  placeholder="Keep the change small; preserve the public API…"
                />
              </label>
              <Button
                variant="secondary"
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
        </section>
      )}
      {r?.status === "BLOCKED" &&
        currentIncident &&
        r.summary &&
        optionsOpen && (
          <details className="command-result">
            <summary>Saved execution stop details</summary>
            <pre className="issue-body">{r.summary}</pre>
          </details>
        )}
      {r && (currentIncident || r.status === "BLOCKED") && (
        <IssueReactor
          incident={currentIncident}
          running={running}
          blocked={r.status === "BLOCKED"}
          network={r.network}
          attempts={r.recovery_attempts ?? 0}
          busy={busy}
          diagnosing={
            r.status === "BLOCKED" && !r.incident && diagnosis.isPending
          }
          diagnosisError={
            r.status === "BLOCKED" && !r.incident
              ? diagnosis.error?.message
              : undefined
          }
          onRecover={(allowDownloads) =>
            void act("recover", { approved: true, network: allowDownloads })
          }
          onEvidence={() => {
            setTestFilter("All");
            setTestSearch("");
            setEvidenceTest(currentIncident?.test_run_id ?? null);
            setTab("Tests");
          }}
        />
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
            onClick={() => {
              setInspectedStage(null);
              setTab(name);
            }}
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
                  <section className="plan-section">
                    <h3>What needs to change</h3>
                    <p>{r.plan.root_cause}</p>
                  </section>
                  <section className="plan-section">
                    <h3>Implementation approach</h3>
                    <p>
                      {strategy.length > 320
                        ? strategy.slice(0, 320).replace(/\s+\S*$/, "") + "…"
                        : strategy || "No implementation approach recorded."}
                    </p>
                    {strategy.length > 320 && (
                      <details className="implementation-details">
                        <summary>Read the full approach</summary>
                        <p>{strategy}</p>
                      </details>
                    )}
                  </section>
                  <section className="plan-section">
                    <h3>Files in scope</h3>
                    <div className="plan-files">
                      {r.plan.files.map((file) => (
                        <code key={file}>{file}</code>
                      ))}
                    </div>
                  </section>
                  <details className="plan-details">
                    <summary>Risks and open questions</summary>
                    <h3>Risks</h3>
                    {r.plan.risks.length ? (
                      <ul>
                        {r.plan.risks.map((risk, i) => (
                          <li key={i}>{risk}</li>
                        ))}
                      </ul>
                    ) : (
                      <p className="muted">
                        No risks recorded in the saved plan.
                      </p>
                    )}
                    <h3>Open questions</h3>
                    {r.plan.unknowns.length ? (
                      <ul>
                        {r.plan.unknowns.map((unknown, i) => (
                          <li key={i}>{unknown}</li>
                        ))}
                      </ul>
                    ) : (
                      <p className="muted">No open questions recorded.</p>
                    )}
                  </details>
                </>
              ) : (
                <p className="muted">
                  A plan will be saved before code is edited.
                </p>
              ))}
            {tab === "Tests" && (
              <>
                <div className="test-history-heading">
                  <div>
                    <h4>Command history</h4>
                    <p className="muted">
                      Real exits and output from every recorded attempt.
                    </p>
                  </div>
                  <span className="test-history-count">
                    {recorded.length} recorded
                  </span>
                </div>
                {!!tests.data?.length && (
                  <div className="test-history-tools">
                    <label className="test-search">
                      <Search size={16} />
                      <input
                        aria-label="Search test commands and output"
                        placeholder="Find a command or output…"
                        value={testSearch}
                        onChange={(e) => setTestSearch(e.target.value)}
                      />
                    </label>
                    <select
                      aria-label="Filter test results"
                      value={testFilter}
                      onChange={(e) => setTestFilter(e.target.value)}
                    >
                      {[
                        "All",
                        "Passed",
                        "Failed",
                        "Setup blocked",
                        "Interrupted",
                      ].map((value) => (
                        <option key={value}>{value}</option>
                      ))}
                    </select>
                  </div>
                )}
                {tests.error && <p className="error">{tests.error.message}</p>}
                {!tests.data?.length && (
                  <p className="muted">
                    No repository verification has run yet.
                  </p>
                )}
                {tests.data?.length && !visibleTests.length ? (
                  <p className="muted">
                    No commands match this search and filter.
                  </p>
                ) : null}
                {visibleTests.map((t) => (
                  <details
                    key={t.id}
                    id={"test-run-" + t.id}
                    className="command-result"
                  >
                    <summary>
                      <span
                        className={
                          "test-run-symbol " +
                          (testStatus(t) === "Passed"
                            ? "test-passed"
                            : testStatus(t) === "Failed"
                              ? "test-failed"
                              : "muted")
                        }
                      >
                        {testStatus(t) === "Passed" ? (
                          <CircleCheck size={18} />
                        ) : (
                          <Terminal size={18} />
                        )}
                      </span>
                      <span className="test-run-command">
                        {t.command.program} {t.command.arguments.join(" ")} ·
                        exit {t.finished_at ? t.exit_code : "pending"} ·{" "}
                        {t.finished_at &&
                        Number.isFinite(
                          Date.parse(t.finished_at) - Date.parse(t.started_at),
                        )
                          ? Math.max(
                              0,
                              Math.round(
                                (Date.parse(t.finished_at) -
                                  Date.parse(t.started_at)) /
                                  1000,
                              ),
                            ) + "s"
                          : "duration pending"}
                      </span>
                      <span
                        className={
                          "test-result test-result-" +
                          testStatus(t).toLowerCase().replaceAll(" ", "-")
                        }
                      >
                        {testStatus(t)}
                      </span>
                      <ChevronRight className="test-output-chevron" size={14} />
                    </summary>
                    {t.recovery_of && (
                      <p className="muted">
                        Rerun after a saved recovery attempt.
                      </p>
                    )}
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
          data-verified={
            !running &&
            ["READY", "PR_PREPARED", "PR_OPENED"].includes(c.state) &&
            r?.review?.verdict === "APPROVE"
          }
        >
          <div className="section-header">
            <h2>Verification</h2>
            <Terminal size={18} />
          </div>
          <div className="verification-body">
            <div className="verification-gate">
              <ShieldCheck size={20} />
              <div>
                <strong>
                  {running
                    ? "Verification in progress"
                    : ["READY", "PR_PREPARED", "PR_OPENED"].includes(c.state) &&
                        r?.review?.verdict === "APPROVE"
                      ? "Ready for human review"
                      : r?.review?.verdict === "REQUEST_CHANGES"
                        ? "Reviewer requested changes"
                        : "Review pending"}
                </strong>
                <p>
                  Required checks and independent review determine readiness.
                </p>
              </div>
            </div>
            <h3>Recorded command outcomes</h3>
            <div className="verification-outcomes">
              <button
                onClick={() => {
                  setTab("Tests");
                  setTestFilter("Passed");
                }}
              >
                <CircleCheck size={16} />
                <span>Passed</span>
                <strong>{tests.data ? passed : "—"}</strong>
              </button>
              <button
                onClick={() => {
                  setTab("Tests");
                  setTestFilter("Failed");
                }}
              >
                <span className="outcome-dot outcome-failed" />
                <span>Failed</span>
                <strong>{tests.data ? failed : "—"}</strong>
              </button>
              <button
                onClick={() => {
                  setTab("Tests");
                  setTestFilter("Setup blocked");
                }}
              >
                <span className="outcome-dot outcome-blocked" />
                <span>Setup blocked</span>
                <strong>{tests.data ? setupBlocked : "—"}</strong>
              </button>
              {interrupted > 0 && (
                <button
                  onClick={() => {
                    setTab("Tests");
                    setTestFilter("Interrupted");
                  }}
                >
                  <CirclePause size={16} />
                  <span>Interrupted</span>
                  <strong>{interrupted}</strong>
                </button>
              )}
            </div>
            <p className="verification-history-note">
              Includes earlier attempts. Select an outcome to inspect its
              output.
            </p>
            <details className="verification-details">
              <summary>Execution details</summary>
              <dl className="verification-facts">
                <div>
                  <dt>
                    <Clock size={16} />
                    Recorded command time
                  </dt>
                  <dd>
                    {tests.data ? Math.round(duration / 1000) + "s" : "—"}
                  </dd>
                </div>
                <div>
                  <dt>Dependency downloads</dt>
                  <dd>
                    {r
                      ? r.network
                        ? "Allowed"
                        : "Not allowed"
                      : "Not recorded"}
                  </dd>
                </div>
                <div>
                  <dt>Fix iterations</dt>
                  <dd>{r?.fix_iterations ?? "—"}</dd>
                </div>
                <div>
                  <dt>Review cycles</dt>
                  <dd>{r?.review_cycles ?? "—"}</dd>
                </div>
              </dl>
            </details>
            <button
              className="inspector-review-link"
              onClick={() => setTab("Review")}
            >
              <ShieldCheck size={16} />
              <span>Independent review</span>
              <strong>
                {r?.review?.verdict === "APPROVE"
                  ? "Approved"
                  : r?.review?.verdict === "REQUEST_CHANGES"
                    ? "Changes requested"
                    : "Not recorded"}
              </strong>
            </button>
            {tests.error && (
              <p className="error" role="alert">
                Verification evidence unavailable: {tests.error.message}
              </p>
            )}
            {!tests.data && !tests.error && (
              <p className="muted">Loading command history…</p>
            )}
          </div>
        </aside>
      </div>
    </section>
  );
}
