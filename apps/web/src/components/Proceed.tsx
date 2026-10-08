import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { APIError, request, useAPI } from "../api";
import type { Opportunity, Contribution } from "../types";
import { Button } from "./ui/button";

type Preview = {
  token: string;
  configuration: { version: number };
  base_commit: string;
  checked_at: string;
  issue: { title: string; body: string; html_url: string };
  warnings: string[];
  workspace_root?: string;
  codex_model?: string;
};
export default function Proceed({ opportunity: o }: { opportunity: Opportunity }) {
  const [preview, setPreview] = useState<Preview | null>(null);
  const [approved, setApproved] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [rateLimited, setRateLimited] = useState(false);
  const [modelChoice, setModelChoice] = useState("");
  const [customModel, setCustomModel] = useState("");
  const discovery = useAPI<{authentication: string}>("/discovery");
  const runtime = useAPI<{codex_model?: string}>("/runtime");
  const catalogue = useAPI<{models: {slug: string; display_name: string}[]}>("/runtime/models");
  const navigate = useNavigate();
  const cache = useQueryClient();
  const requestedModel = modelChoice === "__custom__" ? customModel.trim() : modelChoice;
  function changeModel(choice: string) {
    setModelChoice(choice); setPreview(null); setApproved(false); setError("");
  }
  async function inspect() {
    setPending(true); setError(""); setRateLimited(false); setApproved(false); setPreview(null);
    try { setPreview(await request<Preview>(`/opportunities/${encodeURIComponent(o.id)}/preview`, { codex_model: requestedModel })); }
    catch (e) { setError((e as Error).message); setRateLimited(e instanceof APIError && e.status === 429); }
    finally { setPending(false); }
  }
  async function proceed() {
    if (!preview || !approved) return;
    setPending(true); setError(""); setRateLimited(false);
    try {
      await request<Contribution>(`/opportunities/${encodeURIComponent(o.id)}/proceed`, { token: preview.token, approved: true, execute: true, codex_model: requestedModel });
      await cache.invalidateQueries(); navigate("/contributions");
    } catch (e) { setError((e as Error).message); setRateLimited(e instanceof APIError && e.status === 429); setPreview(null); setApproved(false); }
    finally { setPending(false); }
  }
  if (o.demo) return <div className="dialog-footer"><p className="muted">Demo issues cannot create contribution workspaces. Start live mode to work with real repositories.</p><Button disabled>Proceed to Contribute</Button></div>;
  return <section aria-label="Contribution approval">
    <h3>Prepare a contribution workspace</h3>
    <p className="muted">Review fresh GitHub information, then approve an isolated clone, Codex implementation, real tests and independent review. Completed work stops for your review before PR submission.</p>
    <div className="model-selection">
      <label htmlFor="contribution-model">Codex model for this contribution</label>
      <select id="contribution-model" value={modelChoice} onChange={e => changeModel(e.target.value)} disabled={pending}>
        <option value="">Use default ({runtime.data?.codex_model || "Codex CLI choice"})</option>
        {catalogue.data?.models.filter(m => m.slug !== runtime.data?.codex_model).map(m => <option key={m.slug} value={m.slug}>{m.display_name} ({m.slug})</option>)}
        <option value="__custom__">Enter a model ID…</option>
      </select>
      {modelChoice === "__custom__" && <label className="model-custom" htmlFor="custom-model">Model ID<input id="custom-model" value={customModel} onChange={e => { setCustomModel(e.target.value); setPreview(null); setApproved(false); setError(""); }} disabled={pending} placeholder="Model ID from Codex" autoComplete="off" /></label>}
      {catalogue.data?.models.length === 0 && <p className="muted">The local model list is unavailable. Use the default or enter a model ID.</p>}
      <p className="muted">Used for planning, coding, fixes and independent review. The choice is saved with this contribution. A listed model may still be unavailable to your Codex account.</p>
    </div>
    {error && <p className="error" role="alert">{error}</p>}
    {rateLimited && <p className="note">Wait until the displayed local time before trying again. GitHub access: {discovery.data?.authentication ?? "check Opportunities for authentication status"}.{discovery.data?.authentication === "public unauthenticated" && " Sign in from the GitHub account page to enable authenticated access."}</p>}
    {preview && <>
      <h4>{preview.issue.title}</h4>
      <p><a href={preview.issue.html_url} target="_blank" rel="noreferrer">Open the current GitHub issue</a></p>
      <pre className="issue-body">{preview.issue.body}</pre>
      <dl className="key-values">
        <div><dt>Configuration snapshot</dt><dd>v{preview.configuration.version}</dd></div>
        <div><dt>Codex model</dt><dd>{preview.codex_model || runtime.data?.codex_model || "Codex CLI default"}</dd></div>
        <div><dt>Approved base commit</dt><dd><code>{preview.base_commit}</code></dd></div>
        <div><dt>Fresh check</dt><dd>{new Date(preview.checked_at).toLocaleString()}</dd></div>
        <div><dt>Branch</dt><dd>autopilot/issue-{o.number}</dd></div>
        <div><dt>Workspace</dt><dd>{preview.workspace_root ?? "Configured contributions root"}/&lt;contribution-id&gt;/repo</dd></div>
      </dl>
      <ul className="risk-list">{preview.warnings.map(w => <li key={w}>{w}</li>)}</ul>
      <label className="approval-checkbox"><input type="checkbox" checked={approved} onChange={e => setApproved(e.target.checked)} disabled={pending} /> I approve cloning, Codex coding, verification and review in this external workspace.</label>
    </>}
    <div className="dialog-footer">
      <Button variant="ghost" onClick={inspect} disabled={pending || (modelChoice === "__custom__" && !requestedModel)}>{pending ? "Working…" : preview ? "Refresh preview" : "Proceed to Contribute"}</Button>
      {preview && <Button onClick={proceed} disabled={pending || !approved}>Approve and start contribution</Button>}
    </div>
  </section>;
}
