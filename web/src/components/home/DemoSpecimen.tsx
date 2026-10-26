import Link from "next/link";

import { ArrowRightIcon } from "@/components/icons";
import { CheckStatusGlyph } from "@/components/report/CheckStatusTag";
import { SeverityTag } from "@/components/report/SeverityTag";
import { StatusStamp } from "@/components/report/StatusStamp";
import type { DemoScenario } from "@/lib/demo";
import { findingsForNode, sortFindings } from "@/lib/format";

export function DemoSpecimen({ scenario }: { scenario: DemoScenario }) {
  const { report } = scenario;
  const findings = sortFindings(report.findings);
  const excerpt = findings[0];
  const checks = report.checks.slice(0, 3);
  const root = report.dependencies.nodes.find(
    (node) => node.id === report.dependencies.rootNodeId,
  );
  const child = root
    ? report.dependencies.nodes.find((node) => node.id === root.dependencies[0])
    : undefined;

  return (
    <section
      aria-labelledby="specimen-heading"
      className="border-2 border-ink bg-sheet"
    >
      <div className="flex flex-wrap items-center justify-between gap-2 border-b-2 border-ink bg-amber px-3 py-2 text-night">
        <span className="font-mono text-[0.625rem] font-bold tracking-[0.16em] uppercase">
          Demo data · specimen sheet
        </span>
        <span className="font-mono text-[0.625rem] font-medium tracking-[0.08em] uppercase">
          Offline · no network
        </span>
      </div>

      <div className="px-4 py-4 sm:px-5">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <p className="rule-label text-ink-soft">Synthetic package</p>
            <h2
              id="specimen-heading"
              className="data-value mt-1 text-base leading-tight font-semibold break-all"
            >
              {report.target.package.name}
            </h2>
            <p className="data-value mt-1 text-xs text-ink-soft">
              {report.target.package.ecosystem} · resolved{" "}
              {report.target.resolvedVersion ?? "unresolved"}
            </p>
          </div>
          <StatusStamp status={report.status} size="small" />
        </div>

        <div className="mt-4 grid grid-cols-3 border-y border-rule">
          {checks.map((check) => (
            <div
              key={check.id}
              className="min-w-0 px-2 py-2 first:pl-0 last:pr-0 [&+&]:border-l [&+&]:border-rule"
            >
              <CheckStatusGlyph status={check.status} />
              <p className="mt-1.5 truncate text-[0.6875rem] leading-4 font-semibold">
                {check.name}
              </p>
              <p className="data-value text-[0.625rem] text-ink-soft">
                Succeeded
              </p>
            </div>
          ))}
        </div>

        {excerpt ? (
          <div className="mt-4 border-l-2 border-safety pl-3">
            <div className="flex flex-wrap items-center gap-2">
              <SeverityTag severity={excerpt.severity} />
              <span className="tag tag-confirmed">
                {excerpt.kind === "confirmed" ? "Confirmed" : "Heuristic"}
              </span>
            </div>
            <p className="mt-1.5 text-sm leading-5 font-semibold">
              {excerpt.title}
            </p>
            <p className="mt-1 text-xs leading-5 text-ink-soft">
              {excerpt.evidence[0]?.summary ?? excerpt.description}
            </p>
          </div>
        ) : null}

        <div className="mt-4">
          <div className="flex items-baseline justify-between gap-3">
            <p className="rule-label text-ink-soft">Load path</p>
            <p className="font-mono text-[0.625rem] tracking-[0.08em] text-ink-soft uppercase">
              2 nodes · 1 checkpoint
            </p>
          </div>
          <ol className="mt-2">
            {root ? (
              <li className="flex items-start gap-2.5">
                <span className="mt-1.5 h-2.5 w-2.5 shrink-0 bg-safety" aria-hidden="true" />
                <div className="min-w-0 flex-1 border-b border-rule pb-2">
                  <p className="data-value text-xs leading-4 font-semibold break-all">
                    {root.package.name}
                    <span className="ml-1 font-normal text-ink-soft">
                      @ {root.version}
                    </span>
                  </p>
                  <p className="mt-0.5 text-[0.625rem] tracking-[0.08em] text-safety-deep uppercase">
                    {findingsForNode(root, findings).length} checkpoint on root
                  </p>
                </div>
              </li>
            ) : null}
            {child ? (
              <li className="ml-[4px] flex items-start gap-2.5 border-l border-rule pt-2 pl-4">
                <span
                  className="mt-1.5 h-2 w-2 shrink-0 border border-amber-deep bg-amber"
                  aria-hidden="true"
                />
                <div className="min-w-0 flex-1">
                  <p className="data-value text-xs leading-4 font-semibold break-all">
                    {child.package.name}
                    <span className="ml-1 font-normal text-ink-soft">
                      @ {child.version}
                    </span>
                  </p>
                  <p className="mt-0.5 text-[0.625rem] tracking-[0.08em] text-amber-deep uppercase">
                    Heuristic signal · not confirmed
                  </p>
                </div>
              </li>
            ) : null}
          </ol>
        </div>

        <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-rule pt-3">
          <p className="font-mono text-[0.625rem] tracking-[0.06em] text-ink-soft uppercase">
            Synthetic names · DEMO- identifiers
          </p>
          <Link
            href={`/demo/${scenario.key}`}
            className="link-arrow text-sm text-safety-deep"
          >
            Open the full specimen
            <ArrowRightIcon className="h-4 w-4" />
          </Link>
        </div>
      </div>
    </section>
  );
}
