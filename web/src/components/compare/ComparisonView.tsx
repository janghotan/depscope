import type { CompareResponse, CompareSide } from "@/lib/api-types";
import { ReportView } from "@/components/report/ReportView";
import { SeverityTag } from "@/components/report/SeverityTag";
import { StatusStamp } from "@/components/report/StatusStamp";
import { DeferredDetails } from "@/components/site/DeferredDetails";
import {
  CONDITION_COPY,
  SEVERITY_LABEL,
  checkTally,
  sortFindings,
} from "@/lib/format";

type Metric = {
  label: string;
  value: (side: CompareSide) => string;
  differsIsMeaningful: boolean;
};

function coverageLabel(side: CompareSide): string {
  if (!side.report) {
    return "No report";
  }
  const tally = checkTally(side.report.checks);
  return `${tally.succeeded}/${tally.total} succeeded`;
}

const METRICS: Metric[] = [
  {
    label: "Condition",
    value: (side) => (side.report ? CONDITION_COPY[side.report.status].label : "No report"),
    differsIsMeaningful: true,
  },
  {
    label: "Confirmed findings",
    value: (side) =>
      side.report ? String(side.report.summary.confirmedFindings) : "—",
    differsIsMeaningful: true,
  },
  {
    label: "Heuristic warnings",
    value: (side) =>
      side.report ? String(side.report.summary.heuristicWarnings) : "—",
    differsIsMeaningful: true,
  },
  {
    label: "Highest severity",
    value: (side) =>
      side.report
        ? side.report.summary.highestSeverity
          ? SEVERITY_LABEL[side.report.summary.highestSeverity]
          : "None"
        : "—",
    differsIsMeaningful: true,
  },
  {
    label: "Check coverage",
    value: coverageLabel,
    differsIsMeaningful: true,
  },
  {
    label: "Limitations",
    value: (side) =>
      side.report ? String(side.report.limitations.length) : "—",
    differsIsMeaningful: true,
  },
  {
    label: "Resolved version",
    value: (side) => side.report?.target.resolvedVersion ?? "Unresolved",
    differsIsMeaningful: false,
  },
];

function SideHeading({ side, label }: { side: CompareSide; label: string }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-rule pb-3">
      <div className="min-w-0">
        <p className="rule-label text-ink-soft">Side {label}</p>
        <h3 className="data-value mt-1 text-lg leading-tight font-semibold break-all">
          {side.request.name}
        </h3>
        <p className="data-value mt-1 text-xs text-ink-soft">
          {side.request.ecosystem}
          {side.request.version ? ` · requested ${side.request.version}` : " · latest requested"}
        </p>
      </div>
      {side.report ? (
        <StatusStamp status={side.report.status} size="small" />
      ) : (
        <span className="tag tag-unknown">No report</span>
      )}
    </div>
  );
}

function SideError({ side }: { side: CompareSide }) {
  if (!side.error) {
    return null;
  }
  return (
    <div className="mt-4 border border-oxide-deep bg-paper px-3 py-3">
      <p className="rule-label text-oxide-deep">Side error</p>
      <p className="mt-1 text-sm leading-6">{side.error.message}</p>
      <p className="data-value mt-2 text-[0.6875rem] text-ink-soft">
        Code: {side.error.code}
      </p>
      <p className="mt-2 text-xs leading-5 text-ink-soft">
        This side produced no report. The other side is shown unchanged.
      </p>
    </div>
  );
}

function SideBody({ side, idPrefix }: { side: CompareSide; idPrefix: string }) {
  if (!side.report) {
    return <SideError side={side} />;
  }
  const report = side.report;
  const findings = sortFindings(report.findings);
  const shown = findings.slice(0, 4);
  const gaps = checkTally(report.checks).total - checkTally(report.checks).succeeded;

  return (
    <div className="mt-4 space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <span
          className={`tag ${
            report.dataMode === "demo"
              ? "tag-medium"
              : "border-inspection-deep text-inspection-deep"
          }`}
        >
          {report.dataMode === "demo" ? "Demo data" : "Live data"}
        </span>
        <span className="data-value text-xs text-ink-soft">
          {report.target.resolvedVersion
            ? `@ ${report.target.resolvedVersion}`
            : "version unresolved"}
        </span>
      </div>

      <p className="text-sm leading-6 text-ink-soft">
        {CONDITION_COPY[report.status].description}
      </p>

      {findings.length === 0 ? (
        <p className="text-sm leading-6 text-ink-soft">
          No findings were recorded. Review coverage before reading this as safe.
        </p>
      ) : (
        <ul className="divide-y divide-rule border-y border-rule">
          {shown.map((finding) => (
            <li key={finding.id} className="py-2.5">
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
              </div>
              <p className="mt-1 text-sm leading-5">{finding.title}</p>
              <p className="data-value mt-1 text-[0.6875rem] text-ink-soft">
                {finding.package.ecosystem} · {finding.package.name}
                {finding.version ? ` @ ${finding.version}` : ""}
              </p>
            </li>
          ))}
        </ul>
      )}

      {findings.length > shown.length ? (
        <p className="text-xs text-ink-soft">
          {findings.length - shown.length} more{" "}
          {findings.length - shown.length === 1 ? "finding" : "findings"} in the
          full record below.
        </p>
      ) : null}

      <div className="flex flex-wrap items-center gap-2 text-xs text-ink-soft">
        <span>Checks:</span>
        {report.checks.length === 0 ? (
          <span>none recorded</span>
        ) : (
          <>
            <span className="data-value">
              {report.checks.length - gaps}/{report.checks.length} succeeded
            </span>
            {gaps > 0 ? (
              <span className="tag tag-unknown">{gaps} gaps</span>
            ) : null}
          </>
        )}
      </div>

      <DeferredDetails
        className="border-t border-rule pt-3"
        summary="Read the full condition record"
      >
        <ReportView
          report={report}
          bare
          showDemoBanner={false}
          idPrefix={idPrefix}
        />
      </DeferredDetails>
    </div>
  );
}

export function ComparisonView({ response }: { response: CompareResponse }) {
  const mixedModes =
    response.left.report &&
    response.right.report &&
    response.left.report.dataMode !== response.right.report.dataMode;

  return (
    <section aria-labelledby="comparison-heading" className="space-y-6">
      <div className="border-b border-rule pb-3">
        <h2 id="comparison-heading" className="text-xl font-bold tracking-[-0.01em]">
          Side-by-side condition
        </h2>
        <p className="mt-1 text-sm leading-6 text-ink-soft">
          This view compares recorded conditions. It does not choose a package,
          and it does not treat missing coverage as safety.
        </p>
      </div>

      {mixedModes ? (
        <p className="border border-amber-deep bg-amber/20 px-3 py-2 text-sm leading-6">
          These sides use different data modes. Do not read the comparison as a
          like-for-like live result.
        </p>
      ) : null}

      <div className="scroll-x">
        <table className="w-full min-w-[36rem] border-collapse text-left text-sm">
          <caption className="sr-only">
            Side-by-side comparison of recorded conditions
          </caption>
          <thead>
            <tr className="border-y border-rule-strong">
              <th scope="col" className="rule-label py-2 pr-4 text-ink-soft">
                Measure
              </th>
              <th scope="col" className="rule-label py-2 pr-4 text-ink-soft">
                A · {response.left.request.name}
              </th>
              <th scope="col" className="rule-label py-2 text-ink-soft">
                B · {response.right.request.name}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-rule">
            {METRICS.map((metric) => {
              const left = metric.value(response.left);
              const right = metric.value(response.right);
              const differs =
                metric.differsIsMeaningful && left !== right && left !== "—" && right !== "—";
              return (
                <tr key={metric.label}>
                  <th scope="row" className="py-2.5 pr-4 align-top font-medium">
                    {metric.label}
                    {differs ? (
                      <span className="rule-label ml-2 text-safety-deep">
                        Differs
                      </span>
                    ) : null}
                  </th>
                  <td className="data-value py-2.5 pr-4 align-top text-xs">
                    {left}
                  </td>
                  <td className="data-value py-2.5 align-top text-xs">
                    {right}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div className="grid gap-8 lg:grid-cols-2">
        {(
          [
            ["A", response.left],
            ["B", response.right],
          ] as const
        ).map(([label, side]) => (
          <section
            key={label}
            aria-label={`Side ${label} condition`}
            className="min-w-0"
          >
            <SideHeading side={side} label={label} />
            <SideBody side={side} idPrefix={`side-${label.toLowerCase()}-`} />
          </section>
        ))}
      </div>
    </section>
  );
}
