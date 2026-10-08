import { useState } from "react";
import { Copy, Check, GitBranch } from "./icons";

export function RepositoryAvatar({ repository }: { repository: string }) {
  const owner = repository.split("/")[0];
  const [failed, setFailed] = useState(false);
  const [loaded, setLoaded] = useState(false);
  return (
    <span className="repository-avatar" aria-hidden="true">
      <GitBranch size={20} />
      {!failed && /^[a-zA-Z0-9-]+$/.test(owner) && (
        <img
          key={owner}
          src={`https://github.com/${encodeURIComponent(owner)}.png?size=80`}
          alt=""
          width="36"
          height="36"
          loading="lazy"
          referrerPolicy="no-referrer"
          style={{ opacity: loaded ? 1 : 0 }}
          onLoad={() => setLoaded(true)}
          onError={() => setFailed(true)}
        />
      )}
    </span>
  );
}

export function ProgressMeter({
  value,
  label,
  indeterminate = false,
}: {
  value?: number;
  label: string;
  indeterminate?: boolean;
}) {
  const bounded = Math.max(0, Math.min(100, value ?? 0));
  return (
    <div
      className={
        "progress-meter" + (indeterminate ? " progress-indeterminate" : "")
      }
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={indeterminate ? undefined : bounded}
      aria-valuetext={
        indeterminate ? "In progress; completion percentage unknown" : undefined
      }
    >
      <span
        style={
          indeterminate ? undefined : { transform: `scaleX(${bounded / 100})` }
        }
      />
    </div>
  );
}

export function CopyValue({ value }: { value: string }) {
  const [status, setStatus] = useState("");
  return (
    <span className="copy-value">
      <code>{value}</code>
      <button
        className="icon-button"
        aria-label={`Copy ${value}`}
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(value);
            setStatus("Copied");
          } catch {
            setStatus("Unable to copy; select the value to copy manually");
          }
        }}
      >
        {status === "Copied" ? <Check size={15} /> : <Copy size={15} />}
      </button>
      <span className="sr-only" role="status">
        {status}
      </span>
    </span>
  );
}

export function AnimatedNumber({ value }: { value: number | undefined }) {
  return (
    <strong className="animated-number" key={value}>
      {value === undefined ? "—" : value.toLocaleString()}
    </strong>
  );
}
