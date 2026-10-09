import { useQuery } from "@tanstack/react-query";

import { Button } from "./ui/button";

import { Link } from "react-router-dom";



type Health = {status:string;checked_at:string;checks:{id:string;name:string;status:string;message:string;free_bytes?:number}[]};

export default function SystemHealth(){

 const health=useQuery({queryKey:["/health/details"],queryFn:async()=>{

  const response=await fetch("/api/health/details",{cache:"no-store"});

  const data=await response.json();

  if(!Array.isArray(data.checks)) throw new Error("Health checks are unavailable. The backend may need restarting.");

  return data as Health;

 },refetchInterval:30000,retry:1});

 return <section className="surface surface-elevated system-health" id="health" aria-label="System health">

 <div className="section-header"><h2>System health</h2><Button variant="secondary" disabled={health.isFetching} onClick={()=>health.refetch()}>{health.isFetching?"Checking…":"Refresh checks"}</Button></div>

 {health.isError?<p role="alert">Health checks unavailable. {(health.error as Error).message}</p>:health.data?<>

 <p role="status">{health.data.status==="ok"?"Local checks passed":health.data.status==="warning"?"Attention needed before long tasks":"A required service is unavailable"} · Checked {new Date(health.data.checked_at).toLocaleTimeString()}</p>

 <ul className="health-checks">{health.data.checks.map(check=><li key={check.id} data-status={check.status}><div><strong>{check.name}</strong><span className="health-status">{check.status==="ok"?"Available":check.status==="warning"?"Attention":"Unavailable"}</span></div><p>{check.message}{check.free_bytes!==undefined && ` (${(check.free_bytes/1024/1024).toFixed(0)} MiB free)`}</p></li>)}</ul>

 <Link to="/contributions">Inspect saved contributions</Link><p className="muted">Local checks do not run agents, contact GitHub or resume work.</p>

 </>:<p className="muted">Checking local services…</p>}

 </section>;

}

