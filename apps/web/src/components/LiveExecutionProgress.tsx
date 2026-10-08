import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { request } from "../api";
import type { Event } from "../types";
import { ProgressMeter } from "./Visuals";
import { Terminal } from "./icons";

type Agent = {
  id: string;
  contribution_id: string;
  role: string;
  status: string;
  started_at: string;
};
function activityLabel(e: Event) {
  if (e.type !== "AgentActivity") return e.message;
  const v = e.data as {
    type?: string;
    item?: { type?: string; command?: string; exit_code?: number };
  };
  if (v?.item?.type === "command_execution" && v.item.command) {
    const cmd = v.item.command.replace(/\s+/g, " ").slice(0, 240);
    return v.type === "item.completed"
      ? `Command completed (exit ${v.item.exit_code ?? "unknown"}): ${cmd}`
      : `Command started: ${cmd}`;
  }
  if (v?.type === "thread.started") return "Codex session connected";
  if (v?.type === "turn.completed") return "Codex turn completed";
  if (v?.item?.type) return `Codex: ${v.item.type.replaceAll("_", " ")}`;
  return e.message;
}
export default function LiveExecutionProgress({
  id,
  phase,
}: {
  id: string;
  phase: string;
}) {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  const agents = useQuery({
    queryKey: ["/agents"],
    queryFn: () => request<Agent[]>("/agents"),
    refetchInterval: 3000,
  });
  // A live activity view also works with servers that still return the first entity page.
  const events = useQuery({
    queryKey: ["/events"],
    queryFn: () => request<Event[]>("/events"),
    refetchInterval: 3000,
  });
  const active = agents.data
    ?.filter((a) => a.contribution_id === id && a.status === "RUNNING")
    .sort((a, b) => Date.parse(b.started_at) - Date.parse(a.started_at))[0];
  const recent = (events.data ?? [])
    .filter((e) => e.entity_id === id)
    .slice(-8)
    .reverse();
  const output = active
    ? recent.find(
        (e) =>
          e.type === "AgentActivity" &&
          Date.parse(e.created_at) >= Date.parse(active.started_at),
      )
    : undefined;
  const seconds = (date: string) =>
    Math.max(0, Math.floor((now - Date.parse(date)) / 1000));
  const duration = (s: number) =>
    s < 60 ? `${s}s` : `${Math.floor(s / 60)}m ${s % 60}s`;
  const command = recent.find((e) => e.type === "TestRunStarted");
  return (
    <div
      className="live-execution"
      aria-label="Live execution progress"
      style={{ display: "block", overflowWrap: "anywhere" }}
    >
      <div className="live-heading">
        <Terminal size={20} />
        <strong>
          {active
            ? `Codex ${active.role} process · ${duration(seconds(active.started_at))} elapsed`
            : `Verification phase: ${phase.toLowerCase()}`}
        </strong>
        <span className="live-pill">Live</span>
      </div>
      <ProgressMeter indeterminate label="Execution in progress" />
      {active && (
        <p>
          {output
            ? `Last progress event ${duration(seconds(output.created_at))} ago.`
            : `Awaiting first Codex progress event · ${duration(seconds(active.started_at))} elapsed.`}{" "}
          The process is marked running; this does not confirm that it is making
          progress.
        </p>
      )}
      {!active && phase === "TESTING" && command && (
        <p>
          Current verification started {duration(seconds(command.created_at))}{" "}
          ago. Command output is saved when it completes.
        </p>
      )}
      {(agents.error || events.error) && (
        <p role="alert">
          Live status could not refresh. Check the backend connection.
        </p>
      )}
      <details open>
        <summary>Recent activity · refreshed every 3 seconds</summary>
        {recent.length ? (
          recent.map((e) => (
            <p className="live-event" key={e.id}>
              {new Date(e.created_at).toLocaleTimeString()} · {activityLabel(e)}
            </p>
          ))
        ) : (
          <p>No recent activity received.</p>
        )}
      </details>
    </div>
  );
}
