import type {
  EvidenceKind,
  Finding,
  SourceReference,
} from "@/generated/report-types";
import { ChevronDownIcon } from "@/components/icons";
import { SeverityTag } from "@/components/report/SeverityTag";
import { findingLocation, providerLabel } from "@/lib/format";

const EVIDENCE_LABEL: Record<EvidenceKind, string> = {
  advisory: "Advisory",
  heuristic: "Heuristic",
  registry: "Registry",
  "dependency-path": "Dependency path",
};

function sourceLabel(sources: readonly SourceReference[], sourceId: string): string {
  const source = sources.find((candidate) => candidate.id === sourceId);
  return source ? `${providerLabel(source)} / ${source.identifier}` : sourceId;
}

export type FindingEntry = {
  finding: Finding;
  anchor: string;
};

export function FindingsLedger({
  entries,
  sources,
  openIds,
  onToggle,
  onSetAll,
  idPrefix = "",
}: {
  entries: readonly FindingEntry[];
  sources: readonly SourceReference[];
  openIds: ReadonlySet<string>;
  onToggle: (id: string) => void;
  onSetAll: (open: boolean) => void;
  idPrefix?: string;
}) {
  const allOpen = entries.length > 0 && entries.every(({ finding }) => openIds.has(finding.id));
  const headingId = `${idPrefix}findings-heading`;

  return (
    <section aria-labelledby={headingId}>
      <div className="flex flex-wrap items-end justify-between gap-3 border-b border-rule pb-3">
        <div>
          <h2 id={headingId} className="text-xl font-bold tracking-[-0.01em]">
            Findings
          </h2>
          <p className="mt-1 text-sm text-ink-soft">
            {entries.length === 0
              ? "No findings were recorded."
              : "Confirmed evidence first, then heuristic signals. Open a record to read its evidence."}
          </p>
        </div>
        {entries.length > 0 ? (
          <button
            type="button"
            className="button-secondary text-xs"
            onClick={() => onSetAll(!allOpen)}
          >
            {allOpen ? "Close all evidence" : "Open all evidence"}
          </button>
        ) : null}
      </div>

      {entries.length === 0 ? (
        <div className="border-b border-rule bg-sheet px-4 py-6">
          <p className="text-sm leading-6 text-ink-soft">
            No confirmed advisories or heuristic warnings were returned. This is
            not a guarantee of safety: read the check coverage and limitations
            before relying on the record.
          </p>
        </div>
      ) : (
        <ol className="divide-y divide-rule border-b border-rule">
          {entries.map(({ finding, anchor }) => {
            const open = openIds.has(finding.id);
            const panelId = `${anchor}-evidence`;
            return (
              <li key={anchor} id={anchor} className="scroll-mt-28 bg-sheet px-4 py-5">
                <div className="flex flex-wrap items-center gap-2">
                  <SeverityTag severity={finding.severity} />
                  <span
                    className={`tag ${
                      finding.kind === "confirmed"
                        ? "tag-confirmed"
                        : "tag-heuristic"
                    }`}
                  >
                    {finding.kind === "confirmed" ? "Confirmed" : "Heuristic"}
                  </span>
                  <span className="data-value ml-auto text-xs text-ink-soft">
                    {finding.risk
                      ? `Risk ${finding.risk.score}/100`
                      : "No risk score"}
                  </span>
                </div>

                <h3 className="mt-3 text-lg leading-6 font-semibold">
                  <button
                    type="button"
                    aria-expanded={open}
                    aria-controls={panelId}
                    onClick={() => onToggle(finding.id)}
                    className="flex w-full items-start justify-between gap-3 text-left"
                  >
                    <span>{finding.title}</span>
                    <ChevronDownIcon
                      className={`mt-1 h-4 w-4 shrink-0 text-ink-soft transition-transform duration-200 ${
                        open ? "rotate-180" : ""
                      }`}
                    />
                  </button>
                </h3>

                <p className="data-value mt-1 text-xs text-ink-soft">
                  {findingLocation(finding)}
                </p>

                {open ? (
                  <div
                    id={panelId}
                    role="region"
                    aria-label={`Evidence for ${finding.title}`}
                    className="evidence-panel mt-4 border-t border-rule pt-4"
                  >
                    <p className="text-sm leading-6">{finding.description}</p>

                    {finding.risk ? (
                      <p className="mt-3 text-sm leading-6 text-ink-soft">
                        <span className="rule-label mr-2 text-ink">Risk rationale</span>
                        {finding.risk.rationale}
                      </p>
                    ) : null}

                    <div className="mt-4">
                      <p className="rule-label text-ink-soft">Evidence</p>
                      <ul className="mt-2 space-y-3">
                        {finding.evidence.map((evidence, index) => (
                          <li
                            key={`${anchor}-evidence-${index}`}
                            className="border-l border-rule-strong pl-3"
                          >
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="tag tag-low">
                                {EVIDENCE_LABEL[evidence.kind]}
                              </span>
                              {evidence.reference ? (
                                <code className="text-xs text-ink-soft">
                                  {evidence.reference}
                                </code>
                              ) : null}
                            </div>
                            <p className="mt-1.5 text-sm leading-6">
                              {evidence.summary}
                            </p>
                            <p className="mt-1 font-mono text-[0.6875rem] text-ink-soft">
                              Source: {sourceLabel(sources, evidence.sourceId)}
                            </p>
                          </li>
                        ))}
                      </ul>
                    </div>

                    {finding.remediation ? (
                      <div className="mt-4 border-t border-rule pt-3">
                        <p className="rule-label text-ink-soft">Remediation</p>
                        <p className="mt-1 text-sm leading-6">{finding.remediation}</p>
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}
