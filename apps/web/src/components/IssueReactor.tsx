import { ShieldCheck } from "./icons";
import { Button } from "./ui/button";

export type ExecutionIncident = {
  id: string;
  kind: string;
  summary: string;
  evidence: string;
  next_action: string;
  status: string;
  test_run_id?: string;
  command?: { program: string; arguments: string[] };
  attempt: number;
  can_recover: boolean;
  detected_at: string;
};

export default function IssueReactor({ incident, running, blocked, network, attempts, busy, diagnosing, diagnosisError, onRecover, onEvidence }: {
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
  const needsDownloads = !!incident && !network && ["dependency_network", "network_transport", "dependencies_missing"].includes(incident.kind);
  return <section className={`reactor-panel ${incident ? "reactor-" + incident.status : ""}`} aria-label="Issue reactor">
    <div className="reactor-heading"><ShieldCheck size={17} /><strong>Issue reactor</strong><span className="muted">{incident ? incident.status === "recovering" ? "Recovering automatically" : incident.status === "verifying" ? "Checking the saved patch" : incident.status === "fixing" ? "Investigating a failed check" : "Needs a recovery decision" : "Automatic checks enabled"}</span></div>
    {incident ? <>
      <h4>{incident.summary}</h4>
      <p>{incident.next_action}</p>
      <details className="reactor-evidence"><summary>Diagnosis evidence</summary><p>{incident.evidence}</p>
        {incident.command && <code>{incident.command.program} {incident.command.arguments.join(" ")}</code>}
        {incident.detected_at && <p className="muted">Recorded {new Date(incident.detected_at).toLocaleString()}</p>}
        {incident.test_run_id && <Button variant="ghost" onClick={onEvidence}>View saved command output</Button>}
      </details>
      {!running && blocked && (incident.can_recover || needsDownloads) && <Button disabled={busy} onClick={() => onRecover(needsDownloads)}>{needsDownloads ? "Allow downloads and recover" : "Recover verification"}</Button>}
    </> : diagnosing ? <p className="muted" role="status">Checking saved execution evidence…</p> : diagnosisError ? <p role="alert">Diagnosis is unavailable: {diagnosisError}. Inspect the saved execution output before resuming.</p> : <p className="muted">Dependency and environment failures are checked before a code fix. Required tests and independent review decide whether work is ready.</p>}
    {attempts > 0 && <p className="reactor-attempts muted">{attempts} automatic recovery {attempts === 1 ? "action" : "actions"} in this execution. Earlier evidence stays in the timeline.</p>}
  </section>;
}
