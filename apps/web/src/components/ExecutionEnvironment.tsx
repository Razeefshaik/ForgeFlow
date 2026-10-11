import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { request } from "../api";
import { Button } from "./ui/button";
import { SectionHeader } from "./primitives";
import { Link, useNavigate, useSearchParams } from "react-router-dom";

type Environment = {
  available: boolean; platform: string; mode: string; requested_mode?: string;
  status: string; message: string; running: boolean; error?: string; checked_at?: string;
  checks: { name: string; passed: boolean; evidence?: string }[];
};

export default function ExecutionEnvironment() {
  const query = useQuery({ queryKey: ["/runtime/environment"], queryFn: () => request<Environment>("/runtime/environment"), refetchInterval: q => q.state.data?.running ? 1500 : false });
  const client = useQueryClient();
  const [params] = useSearchParams();
  const contributionId = params.get("resume");
  const navigate = useNavigate();
  const [mode, setMode] = useState("auto");
  const [approved, setApproved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const state = query.data;
  useEffect(() => { if (state?.mode) setMode(state.mode); }, [state?.mode]);
  const disabled = busy || !!state?.running || !state?.available;
  async function resume() {
    if (!contributionId) return;
    setBusy(true); setError("");
    try { await request(`/contributions/${encodeURIComponent(contributionId)}/recover`, { approved: true, network: false }); navigate(`/contributions/${encodeURIComponent(contributionId)}`); }
    catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  async function act(action: "check" | "setup" | "select") {
    setBusy(true); setError("");
    try {
      const result = await request<Environment>(`/runtime/environment/${action}`, { approved, mode: action === "check" ? state?.mode : mode });
      client.setQueryData(["/runtime/environment"], result);
      setApproved(false);
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  return <section className="surface environment-settings" id="sandbox" aria-label="Execution environment">
    <SectionHeader title="Execution environment" />
    <p>Check workspace isolation and Git shell startup. Save a sandbox mode only after both pass.</p>
    {query.isLoading ? <p role="status">Checking environment status…</p> : query.error ? <p role="alert" className="error">Environment status unavailable: {query.error.message}</p> : state && <>
      <dl className="key-values"><div><dt>Selected sandbox</dt><dd>{state.mode || "Unavailable"}</dd></div><div><dt>Verification</dt><dd>{state.status.replaceAll("_", " ")}</dd></div></dl>
      <p role="status" aria-live="polite">{state.message}</p>
      {state.checked_at && <p className="muted">Last checked <time dateTime={state.checked_at}>{new Date(state.checked_at).toLocaleString()}</time></p>}
      {state.checks?.length > 0 && <ul className="environment-checks">{state.checks.map((check, i) => <li key={`${check.name}-${i}`}><strong>{check.passed ? "Passed" : "Failed"}</strong> · {check.name}{check.evidence && <details><summary>View check output</summary><pre className="code">{check.evidence}</pre></details>}</li>)}</ul>}
      {state.error && <p className="error" role="alert">{state.error}</p>}
      {state.available && <>
        {state.platform === "windows" && <div className="settings-row"><label htmlFor="sandbox-mode">Windows sandbox</label><select id="sandbox-mode" value={mode} disabled={disabled} onChange={e => setMode(e.target.value)}><option value="auto">Automatic (prefer MXC)</option><option value="mxc">MXC</option><option value="elevated">Elevated sandbox</option><option value="unelevated">Unelevated sandbox</option></select></div>}
        <label className="environment-approval"><input type="checkbox" checked={approved} disabled={disabled} onChange={e => setApproved(e.target.checked)} />I approve sandbox setup or selection. Windows may ask for administrator approval.</label>
        <p className="muted">Setup provisions the sandbox. Contribution commands keep their workspace and download restrictions. Resume each contribution from its execution page.</p>
        <div className="environment-actions"><Button variant="secondary" disabled={disabled || (state.mode === "elevated" && !approved)} onClick={() => act("check")}>Check environment</Button>{state.platform === "windows" && <><Button disabled={disabled || !approved} onClick={() => act("setup")}>Set up Windows sandbox</Button><Button variant="secondary" disabled={disabled || !approved} onClick={() => act("select")}>Verify and save mode</Button></>}</div>
        {contributionId && <div className="environment-actions">{state.status === "ready" && <Button disabled={disabled} onClick={() => void resume()}>Resume contribution</Button>}<Link className="text-link" to={`/contributions/${encodeURIComponent(contributionId)}`}>Back to contribution</Link></div>}
      </>}
    </>}
    {error && <p className="error" role="alert">{error}</p>}
  </section>;
}
