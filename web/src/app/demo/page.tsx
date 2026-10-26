import type { Metadata } from "next";
import Link from "next/link";

import { ArrowRightIcon, InfoIcon } from "@/components/icons";
import { DemoBanner } from "@/components/report/DemoBanner";
import { StatusStamp } from "@/components/report/StatusStamp";
import { PageMasthead } from "@/components/site/PageMasthead";
import {
  listDemoScenarios,
  requireDemoScenario,
} from "@/lib/demo";
import { checkTally, plural } from "@/lib/format";

export const metadata: Metadata = {
  title: "Offline demo",
  description:
    "Deterministic, network-free DepScope fixtures showing a clean record, a finding, and a coverage gap.",
};

export default function DemoIndexPage() {
  const scenarios = listDemoScenarios();
  const notice =
    requireDemoScenario("findings").report.demoNotice ??
    "DEMO DATA - synthetic fixture; not a real security result.";

  return (
    <div className="record-container py-8 lg:py-10">
      <PageMasthead
        index="Reference 03 · offline fixtures"
        title="Offline demo"
        intro="Three fixed condition records show the report anatomy without the API, credentials, or network access. Every value is synthetic."
      />

      <div className="mt-6">
        <DemoBanner notice={notice} />
      </div>

      <div className="mt-8 grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        {scenarios.map((scenario, index) => {
          const tally = checkTally(scenario.report.checks);
          return (
            <article key={scenario.key} className="sheet flex flex-col">
              <div className="sheet-top flex items-start justify-between gap-3 px-4 pt-4 pb-3">
                <div>
                  <p className="rule-label text-ink-soft">
                    Specimen {String(index + 1).padStart(2, "0")}
                  </p>
                  <h2 className="mt-1 text-lg leading-tight font-bold">
                    <Link
                      href={`/demo/${scenario.key}`}
                      className="link-quiet"
                    >
                      {scenario.title}
                    </Link>
                  </h2>
                </div>
                <StatusStamp status={scenario.report.status} size="small" />
              </div>

              <div className="flex flex-1 flex-col px-4 py-4">
                <p className="text-sm leading-6 text-ink-soft">
                  {scenario.summary}
                </p>

                <dl className="mt-4 grid grid-cols-3 border-y border-rule">
                  <div className="py-2 pr-2">
                    <dt className="rule-label text-ink-soft">Confirmed</dt>
                    <dd className="data-value mt-1 text-lg leading-none font-semibold">
                      {scenario.report.summary.confirmedFindings}
                    </dd>
                  </div>
                  <div className="border-l border-rule px-2 py-2">
                    <dt className="rule-label text-ink-soft">Heuristic</dt>
                    <dd className="data-value mt-1 text-lg leading-none font-semibold">
                      {scenario.report.summary.heuristicWarnings}
                    </dd>
                  </div>
                  <div className="border-l border-rule py-2 pl-2">
                    <dt className="rule-label text-ink-soft">Coverage</dt>
                    <dd className="data-value mt-1 text-lg leading-none font-semibold">
                      {tally.succeeded}
                      <span className="text-sm text-ink-soft">
                        /{tally.total}
                      </span>
                    </dd>
                  </div>
                </dl>

                <div className="mt-4">
                  <p className="rule-label text-ink-soft">Demonstrates</p>
                  <p className="mt-1 text-sm leading-6">
                    {scenario.demonstrates}
                  </p>
                </div>

                <p className="mt-auto pt-4 font-mono text-[0.625rem] tracking-[0.06em] text-ink-soft uppercase">
                  {plural(scenario.report.checks.length, "check", "checks")} ·{" "}
                  {plural(scenario.report.findings.length, "finding", "findings")}
                </p>
              </div>

              <div className="flex items-center justify-between gap-3 border-t border-rule px-4 py-3">
                <span className="tag tag-medium">Demo data</span>
                <Link
                  href={`/demo/${scenario.key}`}
                  className="link-arrow text-sm"
                >
                  Open specimen
                  <ArrowRightIcon className="h-4 w-4" />
                </Link>
              </div>
            </article>
          );
        })}
      </div>

      <section
        aria-labelledby="demo-compare-heading"
        className="mt-10 border-2 border-ink bg-sheet px-5 py-5"
      >
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center">
          <div>
            <p className="rule-label text-ink-soft">Specimen comparison</p>
            <h2
              id="demo-compare-heading"
              className="mt-1 text-xl font-bold tracking-[-0.01em]"
            >
              Set two fixtures side by side.
            </h2>
            <p className="mt-2 max-w-xl text-sm leading-6 text-ink-soft">
              The demo comparison is assembled locally from the same fixtures
              and uses the same comparison envelope the API returns. No request
              leaves this site.
            </p>
          </div>
          <Link href="/demo/compare" className="button-primary text-sm">
            <span className="flex items-center gap-2">
              Open demo comparison
              <ArrowRightIcon className="h-4 w-4" />
            </span>
          </Link>
        </div>
      </section>

      <section
        aria-labelledby="demo-integrity-heading"
        className="mt-10 border-t border-rule pt-6"
      >
        <h2
          id="demo-integrity-heading"
          className="flex items-center gap-2 text-base font-bold"
        >
          <InfoIcon className="h-4 w-4 text-ink-soft" />
          How the demo stays honest
        </h2>
        <ul className="mt-3 grid gap-3 text-sm leading-6 text-ink-soft md:grid-cols-3">
          <li className="border-t border-rule pt-2">
            These pages read fixed JSON fixtures committed under{" "}
            <code className="bg-paper-deep px-1 py-0.5 font-mono text-xs">
              testdata/fixtures/demo
            </code>
            , validated against schema 1.0.0 before display.
          </li>
          <li className="border-t border-rule pt-2">
            Package names are synthetic, advisory identifiers begin with{" "}
            <span className="font-mono text-xs">DEMO-</span>, and timestamps are
            fixed.
          </li>
          <li className="border-t border-rule pt-2">
            The demo never contacts npm, PyPI, or OSV, and it never becomes a
            substitute for a live condition record.
          </li>
        </ul>
      </section>
    </div>
  );
}
