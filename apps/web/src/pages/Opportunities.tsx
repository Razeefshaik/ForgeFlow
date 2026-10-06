import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { flexRender, type SortingState } from "@tanstack/react-table";
import {
  getCoreRowModel,
  getSortedRowModel,
  useLegacyTable,
  type LegacyColumnDef,
} from "@tanstack/react-table/legacy";
import { ArrowDownUp, ArrowRight, GitPullRequest, Search } from "lucide-react";
import { useAPI } from "../api";
import type { Opportunity } from "../types";
import { Badge, Empty, Score } from "../components/primitives";
import { Button } from "../components/ui/button";
import { Dialog } from "../components/ui/dialog";
import Evidence from "../components/Evidence";
export function OpportunityDetail({
  opportunity: o,
  close,
}: {
  opportunity: Opportunity | null;
  close: () => void;
}) {
  return (
    <Dialog
      open={!!o}
      onOpenChange={(v) => {
        if (!v) close();
      }}
      title={o?.title ?? "Opportunity"}
      description={
        o
          ? o.repository +
            " · #" +
            o.number +
            (o.demo ? " · Illustrative demo issue" : "")
          : ""
      }
      wide
    >
      {o && (
        <>
          <p className="summary">{o.summary}</p>
          <div className="detail-summary">
            <div>
              <span className="eyebrow">CONTRIBUTION SCORE</span>
              <strong>
                {o.ranking.score}
                <small>/100</small>
              </strong>
            </div>
            <div>
              <span className="eyebrow">ESTIMATED CODEX EFFORT</span>
              <strong className="effort-heading">
                {o.estimate.category.replaceAll("_", " ").toLowerCase()}
              </strong>
              <span className="muted">
                {Math.round(o.estimate.confidence * 100)}% heuristic confidence
              </span>
            </div>
          </div>
          {o.evidence && <Evidence e={o.evidence} />}
          <h3>Why this opportunity ranks here</h3>
          <div className="factors">
            {o.ranking.factors.map((f) => (
              <div key={f.key}>
                <div>
                  <span>{f.label}</span>
                  <b>{f.score}/100</b>
                </div>
                <div className="factor-track">
                  <i style={{ width: f.score + "%" }} />
                </div>
                <p>
                  {f.reason} Weight: {Math.round(f.weight * 100)}%.
                </p>
              </div>
            ))}
          </div>
          <div className="note">
            <GitPullRequest size={17} />
            <span>
              Effort is informational. It never changes the canonical
              contribution score.
            </span>
          </div>
          <h3>Execution estimate</h3>
          <dl className="key-values">
            <div>
              <dt>Coding iterations</dt>
              <dd>{o.estimate.iterations.join("–")}</dd>
            </div>
            <div>
              <dt>Relevant files</dt>
              <dd>{o.estimate.files.join("–")}</dd>
            </div>
            <div>
              <dt>Testing</dt>
              <dd>{o.estimate.test_complexity}</dd>
            </div>
            <div>
              <dt>Plan allowance impact</dt>
              <dd>Unavailable</dd>
            </div>
          </dl>
          {o.estimate.explanation.map((x) => (
            <p className="muted" key={x}>
              {x}
            </p>
          ))}
          <h3>Risks & unknowns</h3>
          <ul className="risk-list">
            {o.risks.map((x) => (
              <li key={x}>{x}</li>
            ))}
          </ul>
          <div className="dialog-footer">
            <p>
              Workspace manager and execution adapter are pending. No
              contribution can start yet.
            </p>
            <Button disabled>
              Proceed to Contribute <ArrowRight size={15} />
            </Button>
          </div>
        </>
      )}
    </Dialog>
  );
}
export default function Opportunities({
  compact = false,
}: {
  compact?: boolean;
}) {
  const query = useAPI<Opportunity[]>("/opportunities");
  const [search, setSearch] = useState("");
  const [language, setLanguage] = useState("");
  const [selected, setSelected] = useState<Opportunity | null>(null);
  const [sorting, setSorting] = useState<SortingState>([
    { id: "score", desc: true },
  ]);
  const canonicalRanks = useMemo(
    () => new Map((query.data ?? []).map((o, i) => [o.id, i + 1])),
    [query.data],
  );
  const data = useMemo(
    () =>
      (query.data ?? [])
        .filter(
          (o) =>
            (!language || o.language === language) &&
            (o.repository + " " + o.title)
              .toLowerCase()
              .includes(search.toLowerCase()),
        )
        .slice(0, compact ? 4 : undefined),
    [query.data, search, language, compact],
  );
  const columns = useMemo<LegacyColumnDef<Opportunity>[]>(
    () => [
      {
        id: "rank",
        header: "#",
        cell: ({ row }) => (
          <span className="rank">{canonicalRanks.get(row.original.id)}</span>
        ),
        enableSorting: false,
      },
      {
        id: "issue",
        header: "Opportunity",
        cell: ({ row }) => (
          <button
            className="issue-button"
            onClick={() => setSelected(row.original)}
          >
            <span className="repo-name">
              {row.original.repository} <span>#{row.original.number}</span>
            </span>
            <span className="issue-title">{row.original.title}</span>
          </button>
        ),
        enableSorting: false,
      },
      {
        accessorKey: "language",
        header: "Language",
        cell: ({ getValue }) => (
          <span
            className={"language language-" + String(getValue()).toLowerCase()}
          >
            <i />
            {String(getValue())}
          </span>
        ),
      },
      {
        id: "score",
        accessorFn: (o) => o.ranking.score,
        header: "Score",
        cell: ({ getValue }) => <Score value={getValue<number>()} />,
      },
      {
        accessorKey: "difficulty",
        header: "Difficulty",
        cell: ({ getValue }) => (
          <span className="capitalize muted">{String(getValue())}</span>
        ),
      },
      {
        id: "effort",
        header: "Codex effort",
        cell: ({ row }) => (
          <Badge>
            {row.original.estimate.category.replaceAll("_", " ").toLowerCase()}
          </Badge>
        ),
        enableSorting: false,
      },
      {
        id: "action",
        header: "",
        cell: ({ row }) => (
          <Button
            variant="ghost"
            size="small"
            aria-label={"Inspect " + row.original.repository}
            onClick={() => setSelected(row.original)}
          >
            Inspect <ArrowRight size={13} />
          </Button>
        ),
        enableSorting: false,
      },
    ],
    [canonicalRanks],
  );
  const table = useLegacyTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });
  return (
    <>
      <div className="section-header">
        <div className="section-heading">
          <h2>{compact ? "Top opportunities" : "Ranked opportunities"}</h2>
          <span className="count">{query.data?.length ?? 0}</span>
        </div>
        {compact ? (
          <Link className="text-link" to="/opportunities">
            View all <ArrowRight size={13} />
          </Link>
        ) : (
          <Badge tone="violet">Quality ranking · effort excluded</Badge>
        )}
      </div>
      {!compact && (
        <div className="table-toolbar">
          <label className="search">
            <Search size={16} />
            <input
              aria-label="Search opportunities"
              placeholder="Search repositories or issues…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </label>
          <select
            aria-label="Filter language"
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
          >
            <option value="">All languages</option>
            {[...new Set((query.data ?? []).map((o) => o.language))].map(
              (x) => (
                <option key={x}>{x}</option>
              ),
            )}
          </select>
        </div>
      )}
      {query.isPending ? (
        <div className="skeleton" aria-label="Loading opportunities" />
      ) : query.error ? (
        <p className="error" role="alert">
          {query.error.message}
        </p>
      ) : data.length === 0 ? (
        <Empty title="No opportunities found">
          {query.data?.length
            ? "Adjust your filters to see more issues."
            : "Run GitHub discovery to find issues matching your applied profile. Demo mode provides illustrative examples."}
        </Empty>
      ) : (
        <div className="table-scroll">
          <table className="opportunity-table">
            <thead>
              {table.getHeaderGroups().map((group) => (
                <tr key={group.id}>
                  {group.headers.map((h) => (
                    <th key={h.id}>
                      {h.column.getCanSort() ? (
                        <button
                          onClick={h.column.getToggleSortingHandler()}
                          aria-label={
                            "Sort by " + String(h.column.columnDef.header)
                          }
                        >
                          {flexRender(
                            h.column.columnDef.header,
                            h.getContext(),
                          )}
                          <ArrowDownUp size={11} />
                        </button>
                      ) : (
                        flexRender(h.column.columnDef.header, h.getContext())
                      )}
                    </th>
                  ))}
                </tr>
              ))}
            </thead>
            <tbody>
              {table.getRowModel().rows.map((row) => (
                <tr key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id}>
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext(),
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <OpportunityDetail
        opportunity={selected}
        close={() => setSelected(null)}
      />
    </>
  );
}
