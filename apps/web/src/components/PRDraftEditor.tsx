import { useState } from "react";
import { Button } from "./ui/button";

type Draft = { title: string; body: string; revision: string };
export default function PRDraftEditor({title,body,revision,editable,busy,onEditing,onSave}: {
 title:string;body:string;revision:string;editable:boolean;busy:boolean;
 onEditing:(v:boolean)=>void;onSave:(v:Draft)=>Promise<boolean>;
}) {
 const [draft,setDraft]=useState<Draft|null>(null);
 const stale=!!draft && draft.revision!==revision;
 if(!draft) return <><h4>{title}</h4><pre className="issue-body">{body}</pre>{editable && <Button variant="secondary" disabled={busy || !revision} onClick={()=>{setDraft({title,body,revision});onEditing(true)}}>Edit PR draft</Button>}</>;
 return <div className="pr-draft-editor">
 <label htmlFor="pr-draft-title">PR title</label>
 <input id="pr-draft-title" value={draft.title} maxLength={256} onChange={e=>setDraft({...draft,title:e.target.value})}/>
 <label htmlFor="pr-draft-body">PR description</label>
 <textarea id="pr-draft-body" value={draft.body} rows={14} maxLength={60000} onChange={e=>setDraft({...draft,body:e.target.value})}/>
 <p className="muted">Describe the change and actual verification. Saving requires fresh submission approval.</p>
 {stale && <p role="alert">This draft changed elsewhere. Cancel to load the latest version before editing again.</p>}
 <div className="button-row"><Button disabled={busy||stale||!draft.title.trim()||!draft.body.trim()} onClick={async()=>{if(await onSave(draft)){setDraft(null);onEditing(false)}}}>Save PR draft</Button>
 <Button variant="secondary" disabled={busy} onClick={()=>{setDraft(null);onEditing(false)}}>Cancel editing</Button></div>
 </div>;
}
