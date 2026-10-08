import type { ReactNode } from "react";
import { Activity, ArrowUpRight, CircleDashed } from "./icons";
import type { Event } from "../types";
export function Badge({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: string;
}) {
  return <span className={"badge badge-" + tone}>{children}</span>;
}
export function StateBadge({ state }: { state: string }) {
  const tone =
    state === "READY"
      ? "green"
      : state === "TESTING" || state === "BLOCKED"
        ? "amber"
        : state === "CODING" || state === "REVIEWING"
          ? "violet"
          : "neutral";
  return (
    <Badge tone={tone}>
      {state === "READY"
        ? "READY FOR HUMAN REVIEW"
        : state.replaceAll("_", " ")}
    </Badge>
  );
}
export function Score({ value }: { value: number }) {
  return (
    <span className="score">
      <span>{value.toFixed(0)}</span>
      <span className="score-track">
        <i
          style={{
            transform: `scaleX(${Math.max(0, Math.min(100, value)) / 100})`,
          }}
        />
      </span>
    </span>
  );
}
export function Empty({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="empty">
      <CircleDashed size={28} />
      <h3>{title}</h3>
      <p>{children}</p>
    </div>
  );
}
export function EventRows({
  events,
  compact = false,
}: {
  events: Event[];
  compact?: boolean;
}) {
  return (
    <div className="event-list">
      {events.length === 0 ? (
        <Empty title="No activity yet">
          Persisted events will appear here as the control plane works.
        </Empty>
      ) : (
        [...events]
          .reverse()
          .slice(0, compact ? 6 : 100)
          .map((e) => (
            <div className="event-row" key={e.id}>
              <span
                className={
                  "event-icon " + (e.type === "ConfigChanged" ? "violet" : "")
                }
              >
                <Activity size={14} />
              </span>
              <div>
                <span className="event-type">
                  {e.type.replace(/([a-z])([A-Z])/g, "$1 $2")}
                </span>
                <p>{e.message}</p>
              </div>
              <time title={new Date(e.created_at).toLocaleString()}>
                {new Date(e.created_at).toLocaleTimeString([], {
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </time>
            </div>
          ))
      )}
    </div>
  );
}
export function SectionHeader({
  title,
  extra,
}: {
  title: string;
  extra?: ReactNode;
}) {
  return (
    <div className="section-header">
      <h2>{title}</h2>
      {extra}
    </div>
  );
}
export function ExternalGlyph() {
  return <ArrowUpRight size={14} />;
}
