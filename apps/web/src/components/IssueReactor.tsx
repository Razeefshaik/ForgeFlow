import { ShieldCheck } from "./icons";
import { Button } from "./ui/button";
import { Link } from "react-router-dom";
import { useState } from "react";

export type ExecutionIncident = {
  id: string;
  kind: string;
  source?: string;
  summary: string;
  evidence: string;
  next_action: string;
  status: string;
  test_run_id?: string;
  command?: { program: string; arguments: string[] };
  attempt: number;
  can_recover: boolean;
  retry_at?: string;
  retryable?: boolean;
  detected_at: string;
  diagnosis_version?: number;
  path?: string;
  secondary_kinds?: string[];
  failed_tests?: string[];
};

export default function IssueReactor({ contributionId, incident, running, blocked, network, attempts, busy, diagnosing, diagnosisError, onRecover, onEvidence }: {
  contributionId?: string;
  incident?: ExecutionIncident | null;
  running: boolean;
  blocked: boolean;
  network: boolean;
  attempts: number;
  busy: boolean;
  diagnosing?: boolean;
  diagnosisError?: string;
  onRecover: (allowDownloads: boolean) => void;
  onEvidence: () => void;
}) {
  const [approveDownloads, setApproveDownloads] = useState(false);
  const service = incident?.source === "github" || incident?.source === "codex";
  const needsDownloads = !!incident && !service && !network && ["dependency_network", "network_transport", "dependencies_missing", "rate_limit"].includes(incident.kind);
  const setup = !!incident && ["environment_blocked", "toolchain_mismatch", "usage_limit", "authentication", "runtime_unavailable", "workspace_permissions", "tool_unavailable", "disk_capacity", "dependency_unavailable", "verification_timeout", "resource_busy", "workspace_runtime", "agent_output", "sandbox_process"].includes(incident.kind);
  const canApproveDownloads = incident?.kind === "environment_blocked" && !network;
  const recoverWithDownloads = needsDownloads || (canApproveDownloads && approveDownloads);
  return <section className={`reactor-panel ${incident ? "reactor-" + incident.status : ""}`} aria-label="Issue reactor">
    <div className="reactor-heading"><ShieldCheck size={17} /><strong>Issue reactor</strong><span className="muted">{incident ? incident.status === "waiting_retry" ? "Waiting for scheduled retry" : incident.status === "needs_input" ? "Needs input" : incident.status === "recovering" ? "Recovering automatically" : incident.status === "verifying" ? "Checking the saved patch" : incident.status === "fixing" ? "Investigating a failed check" : "Needs a recovery decision" : "Automatic checks enabled"}</span></div>
    {incident ? <>
      <h4>{incident.summary}</h4>
      <p>{incident.next_action}</p>
      {incident.failed_tests?.length ? <p>Recorded failing tests: <code>{incident.failed_tests.join(", ")}</code></p> : null}
      {!running && incident.kind === "sandbox_process" && <Link className="text-link" to={`/settings${contributionId ? "?resume=" + encodeURIComponent(contributionId) : ""}#sandbox`}>Set up execution environment</Link>}
      {incident.retry_at && <p className="reactor-attempts" role="status">{running ? "Next retry" : "Service retry time"}: <time dateTime={incident.retry_at}>{new Date(incident.retry_at).toLocaleString()}</time></p>}
      <details className="reactor-evidence"><summary>Diagnosis evidence</summary><p>{incident.evidence}</p>
        {incident.command && <code>{incident.command.program} {incident.command.arguments.join(" ")}</code>}
        {incident.detected_at && <p className="muted">Recorded {new Date(incident.detected_at).toLocaleString()}</p>}
        {incident.test_run_id && <Button variant="ghost" onClick={onEvidence}>View saved command output</Button>}
      </details>
      {!running && blocked && canApproveDownloads && <label className="approval-checkbox"><input type="checkbox" checked={approveDownloads} disabled={busy} onChange={e => setApproveDownloads(e.target.checked)} />Allow downloading public dependencies when recovering this environment.</label>}
      {!running && blocked && (incident.can_recover || needsDownloads || setup || incident.kind === "generated_path_access") && <Button disabled={busy} onClick={() => onRecover(recoverWithDownloads)}>{recoverWithDownloads ? "Allow downloads and recover" : incident.kind === "completion_gate" ? "Verify supplied evidence" : incident.kind === "generated_path_access" && incident.can_recover ? "Inspect path and retry" : incident.kind === "plan_invalid" ? "Recover planning" : setup ? "Retry after setup is restored" : service ? "Retry service connection" : "Recover verification"}</Button>}
    </> : diagnosing ? <p className="muted" role="status">Checking saved execution evidence…</p> : diagnosisError ? <p role="alert">Diagnosis is unavailable: {diagnosisError}. Inspect the saved execution output before resuming.</p> : <p className="muted">Dependency and environment failures are checked before a code fix. Required tests and independent review decide whether work is ready.</p>}
    {attempts > 0 && <p className="reactor-attempts muted">{attempts} automatic recovery {attempts === 1 ? "action" : "actions"} in this execution. Earlier evidence stays in the timeline.</p>}
  </section>;
}
