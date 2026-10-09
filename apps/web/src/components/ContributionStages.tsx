import type { Contribution } from "../types";
import { Check, CirclePause } from "./icons";

type Progress = {
  phase?: string;
  status: string;
  plan?: unknown;
  fix_iterations?: number;
  review?: { verdict: string };
};
type StageStatus = "complete" | "current" | "pending" | "skipped";
const stages = [
  {
    label: "Plan",
    tab: "Plan",
    phases: [
      "SELECTED",
      "CLONING",
      "PREPARING",
      "ANALYZING_REPOSITORY",
      "PLANNING",
    ],
  },
  { label: "Code", tab: "Diff", phases: ["CODING"] },
  { label: "Test", tab: "Tests", phases: ["TESTING"] },
  { label: "Fix", tab: "Tests", phases: ["FIXING"] },
  { label: "Review", tab: "Review", phases: ["REVIEWING"] },
  {
    label: "Human review",
    tab: "Report",
    phases: ["READY", "PR_PREPARED", "PR_OPENED"],
  },
];

export default function ContributionStages({
  contribution: c,
  execution: r,
  onSelect,
  inspectedStage,
}: {
  contribution: Contribution;
  execution?: Progress;
  onSelect?: (tab: string, stage: string) => void;
  inspectedStage?: string | null;
}) {
  const phase =
    r?.phase ||
    (["BLOCKED", "PAUSED"].includes(c.state) ? c.previous_state : c.state) ||
    "";
  const reviewReached = [
    "REVIEWING",
    "READY",
    "PR_PREPARED",
    "PR_OPENED",
  ].includes(phase);
  const finished = ["READY", "PR_PREPARED", "PR_OPENED"].includes(phase);
  const implemented = [
    "TESTING",
    "FIXING",
    "REVIEWING",
    "READY",
    "PR_PREPARED",
    "PR_OPENED",
  ].includes(phase);
  const stopped =
    r?.status !== "RUNNING" &&
    ["BLOCKED", "PAUSED", "ABANDONED", "FAILED"].includes(c.state);
  const statusFor = (index: number): StageStatus => {
    if (!c.demo) {
      if (index === 0 && r?.plan) return "complete";
      if (index === 1 && r?.plan && implemented) return "complete";
      if (index === 2 && reviewReached && r?.plan) return "complete";
      if (index === 3 && reviewReached && r?.plan)
        return r.fix_iterations ? "complete" : "skipped";
      if (index === 4 && finished && r?.review?.verdict === "APPROVE")
        return "complete";
      if (index === 5 && c.state === "PR_OPENED") return "complete";
    }
    return stages[index].phases.includes(phase) ? "current" : "pending";
  };
  return (
    <ol className="stage-rail" aria-label="Contribution stages">
      {stages.map((stage, index) => {
        const status = statusFor(index);
        const detail =
          status === "complete"
            ? "Completed"
            : status === "skipped"
              ? "Not needed"
              : status === "current"
                ? stopped
                  ? c.state === "PAUSED"
                    ? "Paused"
                    : "Needs attention"
                  : index === 5
                    ? "Your turn"
                    : "In progress"
                : index === 2 && phase === "FIXING"
                  ? "Needs rerun"
                  : index === 3 && phase === "TESTING" && r?.fix_iterations
                    ? stopped
                      ? "Awaiting tests"
                      : "Checking fix"
                    : index === 4 && r?.review?.verdict === "REQUEST_CHANGES"
                      ? "Changes requested"
                      : index === 3
                        ? "If needed"
                        : "Not started";
        return (
          <li
            key={stage.label}
            data-status={status}
            data-inspected={inspectedStage === stage.label}
            className={stopped && status === "current" ? "stage-stopped" : ""}
          >
            <button
              type="button"
              onClick={() => onSelect?.(stage.tab, stage.label)}
              disabled={!onSelect}
              aria-current={status === "current" ? "step" : undefined}
              aria-label={`${stage.label}: ${detail}. View ${stage.tab.toLowerCase()}`}
            >
              <span className="stage-symbol">
                {status === "complete" ? (
                  <Check size={15} weight="bold" />
                ) : status === "current" && c.state === "PAUSED" ? (
                  <CirclePause size={16} />
                ) : status === "skipped" ? (
                  "−"
                ) : (
                  index + 1
                )}
              </span>
              <span className="stage-copy">
                <strong>{stage.label}</strong>
                <small>{detail}</small>
              </span>
            </button>
          </li>
        );
      })}
    </ol>
  );
}
