import type { CheckResult, ScanReport } from "@/generated/report-types";
import { SeverityTag } from "@/components/report/SeverityTag";
import { checkTally, plural } from "@/lib/format";

export function SummaryTally({
  report,
  checks,
}: {
  report: ScanReport;
  checks: readonly CheckResult[];
}) {
  const tally = checkTally(checks);
  const gaps = tally.unavailable + tally.skipped + tally.failed;

  return (
    <dl className="grid grid-cols-2 border-t border-l border-rule bg-sheet md:grid-cols-4">
      <div className="border-r border-b border-rule p-4">
        <dt className="rule-label text-ink-soft">Confirmed findings</dt>
        <dd className="data-value mt-2 text-3xl leading-none font-semibold">
          {report.summary.confirmedFindings}
        </dd>
        <p className="mt-2 text-xs leading-4 text-ink-soft">Advisory-backed</p>
      </div>
      <div className="border-r border-b border-rule p-4">
        <dt className="rule-label text-ink-soft">Heuristic warnings</dt>
        <dd className="data-value mt-2 text-3xl leading-none font-semibold">
          {report.summary.heuristicWarnings}
        </dd>
        <p className="mt-2 text-xs leading-4 text-ink-soft">
          Signals, not confirmations
        </p>
      </div>
      <div className="border-r border-b border-rule p-4">
        <dt className="rule-label text-ink-soft">Highest severity</dt>
        <dd className="mt-2 min-h-8">
          {report.summary.highestSeverity ? (
            <SeverityTag severity={report.summary.highestSeverity} />
          ) : (
            <span className="text-sm font-semibold text-ink-soft">
              None reported
            </span>
          )}
        </dd>
        <p className="mt-2 text-xs leading-4 text-ink-soft">From the summary</p>
      </div>
      <div className="border-r border-b border-rule p-4">
        <dt className="rule-label text-ink-soft">Check coverage</dt>
        <dd className="data-value mt-2 text-3xl leading-none font-semibold">
          {tally.succeeded}
          <span className="text-lg text-ink-soft">/{tally.total}</span>
        </dd>
        <p className="mt-2 text-xs leading-4 text-ink-soft">
          {gaps === 0
            ? "No gaps recorded"
            : `${plural(gaps, "gap", "gaps")} recorded`}
        </p>
      </div>
    </dl>
  );
}
