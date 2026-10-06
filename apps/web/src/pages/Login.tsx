import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { GitBranch, ExternalLink, ShieldCheck } from "lucide-react";
import { request } from "../api";
import { Button } from "../components/ui/button";

type Account = { available?: boolean; configured: boolean; client_id: string; connected: boolean; source: string; login: string; pending: boolean; user_code?: string; verification_uri?: string; expires_at?: string; message: string };
export default function Login() {
 const client = useQueryClient();
 const account = useQuery({ queryKey: ["/auth/github"], queryFn: () => request<Account>("/auth/github"), refetchInterval: 2000 });
 const [id, setID] = useState("");
 const [busy, setBusy] = useState(false);
 const [error, setError] = useState("");
 const [setup, setSetup] = useState(false);
 const s = account.data;
 async function act(action: string) {
  setBusy(true); setError("");
  try { const result=await request<Account>("/auth/github/"+action, action==="configure"?{client_id:id.trim()}:{});client.setQueryData(["/auth/github"],result);setSetup(false);await client.invalidateQueries({queryKey:["/discovery"]}); }
  catch(e) {setError(e instanceof Error?e.message:"Account action failed");}
  finally {setBusy(false);}
 }
 return <>
  <div className="page-intro"><div><span className="eyebrow">YOUR GITHUB CONNECTION</span><h1>GitHub account</h1><p>Sign in once in your browser. ForgeFlow remembers your account across restarts.</p></div></div>
  <section className="surface" style={{padding:28,maxWidth:780}}>
   <h2 style={{display:"flex",alignItems:"center",gap:10,marginBottom:20}}><GitBranch size={22}/> {s?.connected ? (s.login ? `Connected as ${s.login}` : "GitHub connected") : "Sign in with GitHub"}</h2>
   {account.isPending && <p>Loading account…</p>}
   {account.error && <p role="alert">{account.error.message}</p>}
   {error && <p className="note" role="alert">{error}</p>}
   {s?.message && <p className="note" role="status">{s.message}</p>}
   {s?.available===false ? null : s && <>
    {s.connected ? <><p>Access: {s.source}. GitHub access is used for discovery and approved contribution actions.</p><Button variant="secondary" disabled={busy} onClick={()=>void act("logout")}>Sign out of ForgeFlow</Button> <a href="https://github.com/settings/applications" target="_blank" rel="noreferrer">Manage GitHub authorizations <ExternalLink size={13} style={{display:"inline",verticalAlign:"middle"}}/></a></> : s.pending ? <>
     <p>Copy this one-time code, then open GitHub to sign in and approve access.</p>
     <p style={{fontSize:30,letterSpacing:5,fontFamily:"monospace"}} aria-label="GitHub verification code">{s.user_code}</p>
     <Button asChild><a href="https://github.com/login/device" target="_blank" rel="noreferrer">Continue on GitHub <ExternalLink size={15}/></a></Button>
     <p>Waiting for approval. Code expires {s.expires_at?new Date(s.expires_at).toLocaleTimeString():"soon"}.</p>
     <Button variant="secondary" disabled={busy} onClick={()=>void act("cancel")}>Cancel sign-in</Button>
    </> : <>
     {s.configured && !setup && <><p>ForgeFlow requests access to public repositories for discovery, forks, branch pushes and PRs. Submission still requires your separate approval.</p><Button disabled={busy} onClick={()=>void act("login")}><GitBranch size={16}/> {busy?"Connecting…":"Sign in with GitHub"}</Button> <Button variant="secondary" onClick={()=>{setID(s.client_id);setSetup(true)}}>Change OAuth app</Button></>}
     {(!s.configured || setup) && <>
      <h3 style={{marginBottom:12}}>One-time OAuth app setup</h3><p>GitHub requires an OAuth Client ID to identify ForgeFlow. This is a public app identifier, not your personal access token.</p>
      <ol style={{listStyle:"decimal",paddingLeft:24,margin:"18px 0",lineHeight:1.8}}><li><a href="https://github.com/settings/applications/new" target="_blank" rel="noreferrer">Register an OAuth app on GitHub <ExternalLink size={13} style={{display:"inline",verticalAlign:"middle"}}/></a> named <strong>ForgeFlow</strong>.</li><li>Homepage: <code>http://127.0.0.1:5173</code>. Callback URL: <code>http://127.0.0.1:5173/login</code> (device sign-in does not use a callback).</li><li>Enable <strong>Device Flow</strong> in the app settings, then copy its <strong>Client ID</strong> below. You do not need a client secret.</li></ol>
      <label htmlFor="github-client-id">OAuth Client ID</label><input id="github-client-id" value={id} onChange={e=>setID(e.target.value)} placeholder="Client ID from your OAuth app" autoComplete="off" style={{display:"block",width:"100%",margin:"12px 0",padding:12}}/>
      <Button disabled={busy||!id.trim()} onClick={()=>void act("configure")}>Save Client ID</Button>
     </>}
    </>}
    <p className="muted" style={{marginTop:28}}><ShieldCheck size={16} style={{display:"inline",verticalAlign:"middle",marginRight:5}}/> Credentials stay on the backend and are saved encrypted for your Windows account. They are never sent to this page or stored in the repository. Expiring sessions refresh automatically. Signing out removes ForgeFlow’s saved credential; revoke the OAuth app in GitHub settings to remove its authorization.</p>
   </>}
  </section>
 </>;
}
