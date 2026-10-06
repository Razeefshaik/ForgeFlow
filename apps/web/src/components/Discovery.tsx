import { useState } from "react";
import { Link } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Radar, RefreshCw } from "lucide-react";
import { request, useAPI } from "../api";
import type { DiscoveryRun, DiscoveryStatus } from "../types";
import { Badge, SectionHeader } from "./primitives";
import { Button } from "./ui/button";

export default function Discovery() {
  const client = useQueryClient();
  const status = useQuery({
    queryKey: ["/discovery"],
    queryFn: () => request<DiscoveryStatus>("/discovery"),
    refetchInterval: 3000,
  });
  const runs = useAPI<DiscoveryRun[]>("/discovery/runs");
  const [showHistory, setShowHistory] = useState(false);
  const action = useMutation({
    mutationFn: (name: string) => request("/discovery/" + name, {}),
    onSuccess: () => {
      void client.invalidateQueries();
    },
  });
  const s = status.data;
  return (
    <section className="surface discovery-panel" aria-label="GitHub discovery">
      <SectionHeader
        title="GitHub discovery"
        extra={
          <Badge tone={s?.running ? "violet" : "neutral"}>
            <Radar size={13} />
            {s?.running ? "Scanning" : s?.available ? "Live" : "Demo"}
          </Badge>
        }
      />
      {status.error ? (
        <p className="error" role="alert">
          {status.error.message}
        </p>
      ) : (
        <>
          <p className="muted">{s?.message ?? "Loading discovery status…"}</p>
          {s?.available && (
            <>
              <div className="discovery-meta">
                <span>
                  Access: <b>{s.authentication}</b>
                  {s.authentication === "public unauthenticated" && <> · <Link to="/login">Sign in with GitHub</Link></>}
                </span>
                <span>
                  Automatic scans: <b>{s.automatic ? "on" : "paused"}</b>
                </span>
                {s.next_run && (
                  <span>
                    Next: <b>{new Date(s.next_run).toLocaleTimeString()}</b>
                  </span>
                )}
              </div>
              {s.retry_at && (
                <p className="note">
                  GitHub rate limit reached. Retry after{" "}
                  {new Date(s.retry_at).toLocaleString()}.
                </p>
              )}
              <div className="discovery-actions">
                <Button
                  disabled={s.running || action.isPending || !!s.retry_at}
                  onClick={() => action.mutate("run")}
                >
                  <RefreshCw size={14} />
                  Run discovery now
                </Button>
                <Button
                  variant="secondary"
                  disabled={
                    action.isPending ||
                    (!s.automatic && s.interval_seconds === 0)
                  }
                  onClick={() =>
                    action.mutate(s.automatic ? "pause" : "resume")
                  }
                >
                  {s.automatic
                    ? "Pause automatic scans"
                    : "Resume automatic scans"}
                </Button>
                {s.running && (
                  <Button
                    variant="secondary"
                    disabled={action.isPending}
                    onClick={() => action.mutate("cancel")}
                  >
                    Cancel scan
                  </Button>
                )}
              </div>
              <p className="muted discovery-hint">
                Scans use your applied profile. Older observations remain
                visible with their timestamp and configuration version.
                Selecting an issue will require a separate approval before
                execution.
              </p>
            </>
          )}
          {s?.last_run && (
            <div className="discovery-last" role="status">
              <strong>
                Latest scan:{" "}
                {s.last_run.status.toLowerCase().replaceAll("_", " ")}
              </strong>
              <span>
                {s.last_run.accepted} opportunities · {s.last_run.requests}{" "}
                GitHub requests · config v{s.last_run.config_version}
              </span>
            </div>
          )}
          {!!runs.data?.length && (
            <Button
              variant="ghost"
              size="small"
              onClick={() => setShowHistory(!showHistory)}
            >
              {showHistory ? "Hide scan history" : "View scan history"}
            </Button>
          )}
          {showHistory && (
            <div className="discovery-history">
              {runs.data?.map((run) => (
                <details key={run.id}>
                  <summary>
                    {new Date(run.started_at).toLocaleString()} · {run.status} ·{" "}
                    {run.accepted} accepted
                  </summary>
                  <p className="muted">
                    Config v{run.config_version} · {run.candidates} candidates
                    considered · {run.requests} requests
                  </p>
                  {run.warnings.length ? (
                    <ul className="risk-list">
                      {run.warnings.map((w, i) => (
                        <li key={i}>{w}</li>
                      ))}
                    </ul>
                  ) : (
                    <p className="muted">No scan warnings.</p>
                  )}
                </details>
              ))}
            </div>
          )}
        </>
      )}
      {action.error && (
        <p className="error" role="alert">
          {action.error.message}
        </p>
      )}
      {runs.error && <p className="error">{runs.error.message}</p>}
    </section>
  );
}
