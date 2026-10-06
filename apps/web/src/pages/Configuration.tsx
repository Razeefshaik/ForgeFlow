import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Check, RotateCcw, SlidersHorizontal } from "lucide-react";
import { request, useAPI } from "../api";
import type { Config, ConfigVersion, Proposal } from "../types";
import { Badge, SectionHeader } from "../components/primitives";
import { Button } from "../components/ui/button";
function changes(before: Config, after: Config): string[] {
  const result: string[] = [];
  function walk(a: unknown, b: unknown, path: string) {
    if (JSON.stringify(a) === JSON.stringify(b)) return;
    if (
      a &&
      b &&
      typeof a === "object" &&
      typeof b === "object" &&
      !Array.isArray(a) &&
      !Array.isArray(b)
    ) {
      const aa = a as Record<string, unknown>,
        bb = b as Record<string, unknown>;
      for (const key of new Set([...Object.keys(aa), ...Object.keys(bb)]))
        walk(aa[key], bb[key], path ? path + "." + key : key);
    } else
      result.push(
        path +
          ": " +
          (a === undefined ? "(absent)" : JSON.stringify(a)) +
          " → " +
          (b === undefined ? "(absent)" : JSON.stringify(b)),
      );
  }
  walk(before, after, "");
  return result.length ? result : ["No effective changes"];
}
export default function Configuration() {
  const current = useAPI<ConfigVersion>("/config");
  const history = useAPI<ConfigVersion[]>("/config/history");
  const proposals = useAPI<Proposal[]>("/config/proposals");
  const client = useQueryClient();
  const [draft, setDraft] = useState("");
  const [draftVersion, setDraftVersion] = useState(0);
  const [dirty, setDirty] = useState(false);
  const [reason, setReason] = useState("");
  const [editError, setEditError] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    if (current.data && !dirty) {
      setDraft(JSON.stringify(current.data.config, null, 2));
      setDraftVersion(current.data.version);
    }
  }, [current.data, dirty]);
  const mutation = useMutation({
    mutationFn: ({ path, body }: { path: string; body: unknown }) =>
      request(path, body),
    onSuccess: async () => {
      setEditError("");
      setDirty(false);
      setNotice("Saved. Configuration history and audit events updated.");
      await client.invalidateQueries();
    },
  });
  function propose() {
    try {
      const config = JSON.parse(draft) as Config;
      if (!current.data) return;
      setEditError("");
      setNotice("");
      mutation.mutate({
        path: "/config/proposals",
        body: { base_version: draftVersion, config, reason },
      });
    } catch {
      setEditError("The configuration must be valid JSON.");
    }
  }
  return (
    <>
      <div className="page-intro">
        <div>
          <span className="eyebrow">VERSIONED & AUDITABLE</span>
          <h1>Configuration</h1>
          <p>
            Propose a change, inspect the diff, then choose when to apply it.
          </p>
        </div>
        <Badge tone="violet">v{current.data?.version ?? "—"}</Badge>
      </div>
      {current.error && (
        <p className="error" role="alert">
          {current.error.message}
        </p>
      )}
      {(editError || mutation.error) && (
        <p className="error" role="alert">
          {editError || mutation.error?.message}
        </p>
      )}
      {notice && (
        <p className="notice" role="status">
          {notice}
        </p>
      )}
      <div className="config-layout">
        <section className="surface">
          <SectionHeader
            title="Active profile"
            extra={<SlidersHorizontal size={16} />}
          />
          <div className="config-editor">
            <label htmlFor="config-draft">
              Configuration JSON · based on v{draftVersion}
            </label>
            <textarea
              id="config-draft"
              spellCheck={false}
              value={draft}
              onChange={(e) => {
                setDirty(true);
                setDraft(e.target.value);
              }}
            />
            <label htmlFor="config-reason">Reason for change</label>
            <input
              id="config-reason"
              placeholder="e.g. Include Rust backend projects"
              value={reason}
              maxLength={1000}
              onChange={(e) => setReason(e.target.value)}
            />
            <div className="editor-footer">
              <span>Changes are validated before saving.</span>
              {dirty && (
                <Button variant="ghost" onClick={() => setDirty(false)}>
                  Reset to current
                </Button>
              )}
              <Button
                onClick={propose}
                disabled={!current.data || !reason.trim() || mutation.isPending}
              >
                Propose changes
              </Button>
            </div>
          </div>
        </section>
        <div>
          <section className="surface">
            <SectionHeader title="Safety gates" />
            <div className="safety-row">
              <Check size={16} />
              <div>
                <strong>Human selects every contribution</strong>
                <p>No coding from ranking alone.</p>
              </div>
            </div>
            <div className="safety-row">
              <Check size={16} />
              <div>
                <strong>Human submits every PR</strong>
                <p>Automatic creation and merging are forbidden.</p>
              </div>
            </div>
            <div className="safety-row">
              <Check size={16} />
              <div>
                <strong>Effort is separate from ranking</strong>
                <p>Enforced by the ranking input boundary.</p>
              </div>
            </div>
          </section>
          <section className="surface history">
            <SectionHeader title="Version history" />
            {history.error ? (
              <p className="error">{history.error.message}</p>
            ) : (
              history.data?.map((v) => (
                <div className="history-row" key={v.version}>
                  <div>
                    <Badge>v{v.version}</Badge>
                    <strong>{v.reason}</strong>
                    <p>
                      {v.actor} · {new Date(v.created_at).toLocaleString()}
                    </p>
                  </div>
                  {v.version !== current.data?.version && (
                    <Button
                      variant="ghost"
                      size="small"
                      disabled={mutation.isPending}
                      onClick={() =>
                        mutation.mutate({
                          path: "/config/rollback",
                          body: {
                            version: v.version,
                            base_version: current.data?.version,
                          },
                        })
                      }
                    >
                      <RotateCcw size={13} /> Restore
                    </Button>
                  )}
                </div>
              ))
            )}
          </section>
        </div>
      </div>
      <section className="surface proposals">
        <SectionHeader title="Proposals awaiting a decision" />
        {proposals.error ? (
          <p className="error">{proposals.error.message}</p>
        ) : proposals.data?.filter((p) => p.status === "PENDING").length ? (
          proposals.data
            .filter((p) => p.status === "PENDING")
            .map((p) => (
              <div className="proposal" key={p.id}>
                <div className="section-header">
                  <div>
                    <h3>{p.reason}</h3>
                    {p.expires_at && <p className="muted">Temporary until {new Date(p.expires_at).toLocaleString()}. Expiration restores the prior version if no newer configuration has replaced this one.</p>}
                    <p className="muted">
                      Based on v{p.base_version}
                      {p.base_version !== current.data?.version
                        ? " · Stale: cancel and propose again"
                        : ""}
                    </p>
                  </div>
                  <Badge tone="amber">Pending approval</Badge>
                </div>
                <pre>
                  {current.data
                    ? changes(current.data.config, p.config).join("\n")
                    : ""}
                </pre>
                <div className="proposal-actions">
                  <Button
                    variant="secondary"
                    disabled={mutation.isPending}
                    onClick={() =>
                      mutation.mutate({
                        path: "/config/proposals/" + p.id + "/cancel",
                        body: {},
                      })
                    }
                  >
                    Cancel
                  </Button>
                  <Button
                    disabled={
                      mutation.isPending ||
                      p.base_version !== current.data?.version
                    }
                    onClick={() =>
                      mutation.mutate({
                        path: "/config/proposals/" + p.id + "/apply",
                        body: {},
                      })
                    }
                  >
                    Apply changes
                  </Button>
                </div>
              </div>
            ))
        ) : (
          <p className="inline-empty">
            No pending proposals. The active profile is unchanged.
          </p>
        )}
      </section>
    </>
  );
}
