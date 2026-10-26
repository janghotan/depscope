import Link from "next/link";

import { ArrowRightIcon } from "@/components/icons";
import { ScanForm } from "@/components/forms/ScanForm";
import { DemoSpecimen } from "@/components/home/DemoSpecimen";
import { listDemoScenarios, requireDemoScenario } from "@/lib/demo";
import { checkTally } from "@/lib/format";

const ANATOMY = [
  {
    index: "01",
    title: "Condition",
    body: "The stamp summarises the record. It never replaces the checks, sources, and limitations that produced it.",
  },
  {
    index: "02",
    title: "Evidence",
    body: "Confirmed advisories and heuristic signals are labelled separately. A heuristic is a lead to inspect, not a vulnerability match.",
  },
  {
    index: "03",
    title: "Coverage",
    body: "Unavailable, skipped, and failed checks stay on the record. If coverage is missing, the condition cannot read as clean.",
  },
];

function DemoComparisonPreview() {
  const clean = requireDemoScenario("clean").report;
  const findings = requireDemoScenario("findings").report;
  const cleanChecks = checkTally(clean.checks);
  const findingChecks = checkTally(findings.checks);

  const rows = [
    {
      label: "Condition",
      clean: "Clean",
      findings: "Findings",
    },
    {
      label: "Confirmed findings",
      clean: String(clean.summary.confirmedFindings),
      findings: String(findings.summary.confirmedFindings),
    },
    {
      label: "Heuristic warnings",
      clean: String(clean.summary.heuristicWarnings),
      findings: String(findings.summary.heuristicWarnings),
    },
    {
      label: "Highest severity",
      clean: "None",
      findings: "High",
    },
    {
      label: "Check coverage",
      clean: `${cleanChecks.succeeded}/${cleanChecks.total}`,
      findings: `${findingChecks.succeeded}/${findingChecks.total}`,
    },
  ];

  return (
    <div className="min-w-0 border border-rule-strong bg-paper-deep/40 p-4">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-rule pb-2">
        <p className="rule-label text-ink-soft">Demo comparison preview</p>
        <span className="tag tag-medium">Demo data</span>
      </div>
      <div className="scroll-x mt-2">
        <table className="w-full min-w-[22rem] border-collapse text-left text-sm">
          <caption className="sr-only">
            Synthetic comparison of a clean record and a record with findings,
            used only to illustrate the comparison layout.
          </caption>
          <thead>
            <tr className="border-b border-rule">
              <th scope="col" className="rule-label py-2 pr-3 text-ink-soft">
                Measure
              </th>
              <th scope="col" className="rule-label py-2 pr-3 text-ink-soft">
                Clean
              </th>
              <th scope="col" className="rule-label py-2 text-ink-soft">
                Findings
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-rule">
            {rows.map((row) => {
              const differs = row.clean !== row.findings;
              return (
                <tr key={row.label}>
                  <th scope="row" className="py-2 pr-3 align-top font-medium">
                    {row.label}
                    {differs ? (
                      <span className="rule-label ml-2 text-safety-deep">
                        Differs
                      </span>
                    ) : null}
                  </th>
                  <td className="data-value py-2 pr-3 align-top text-xs">
                    {row.clean}
                  </td>
                  <td className="data-value py-2 align-top text-xs">
                    {row.findings}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <p className="mt-2 border-t border-rule pt-2 text-xs leading-5 text-ink-soft">
        Both sides are synthetic fixtures. The live comparison view scans each
        side independently through the shared API.
      </p>
    </div>
  );
}

export default function HomePage() {
  const specimen = requireDemoScenario("findings");
  const scenarios = listDemoScenarios();

  return (
    <div className="record-container pt-8 pb-12 lg:pt-10">
      <section aria-labelledby="home-heading">
        <div className="flex flex-col gap-6 border-b border-rule pb-6 lg:flex-row lg:items-end lg:justify-between lg:gap-12">
          <div>
            <p className="rule-label text-ink-soft">
              Dependency condition records
            </p>
            <h1
              id="home-heading"
              className="mt-3 max-w-[18ch] text-[2.15rem] leading-[1.02] font-bold tracking-[-0.025em] sm:text-5xl"
            >
              Inspect the package before you add it.
            </h1>
          </div>
          <p className="max-w-md text-sm leading-6 text-ink-soft lg:pb-1">
            One record for identity, advisories, heuristic signals, failed
            checks, and gaps. Evidence and uncertainty stay separate from the
            verdict.
          </p>
        </div>

        <div className="survey-ticks mt-6" aria-hidden="true" />

        <div className="mt-5 grid gap-6 lg:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)] lg:items-start">
          <div className="sheet sheet-top">
            <div className="px-4 pt-4 sm:px-6">
              <div className="flex flex-wrap items-baseline justify-between gap-2 border-b border-rule pb-3">
                <h2 className="text-lg font-bold">Inspection request</h2>
                <Link
                  href="/about#api"
                  className="rule-label text-ink-soft hover:text-ink"
                >
                  Live · API required
                </Link>
              </div>
              <p className="mt-3 text-sm leading-6 text-ink-soft">
                Name one npm or PyPI package. Add an exact version to pin the
                inspection, or leave it blank to resolve the latest release.
              </p>
            </div>
            <div className="px-4 py-5 sm:px-6">
              <ScanForm />
              <div className="mt-5 border-t border-rule pt-4">
                <p className="rule-label text-ink-soft">Prefer the terminal?</p>
                <p className="data-value mt-2 break-all text-xs text-ink">
                  go install ./cmd/depscope
                </p>
                <Link href="/about#installation" className="link-arrow mt-3 w-fit text-sm">
                  CLI quick start
                  <ArrowRightIcon className="h-4 w-4" />
                </Link>
              </div>
            </div>
          </div>

          <DemoSpecimen scenario={specimen} />
        </div>
      </section>

      <section aria-labelledby="anatomy-heading" className="mt-12">
        <div className="flex flex-col gap-3 border-b border-rule pb-3 sm:flex-row sm:items-end sm:justify-between">
          <h2
            id="anatomy-heading"
            className="text-2xl font-bold tracking-[-0.015em]"
          >
            A record is read in three passes.
          </h2>
          <Link href="/about" className="link-arrow w-fit text-sm">
            How to read a record
            <ArrowRightIcon className="h-4 w-4" />
          </Link>
        </div>
        <ol className="mt-5 grid gap-6 md:grid-cols-3">
          {ANATOMY.map((item) => (
            <li key={item.index} className="border-t-2 border-ink pt-3">
              <div className="flex items-baseline gap-3">
                <span className="data-value text-xs text-safety-deep">
                  {item.index}
                </span>
                <h3 className="text-base font-bold">{item.title}</h3>
              </div>
              <p className="mt-2 text-sm leading-6 text-ink-soft">
                {item.body}
              </p>
            </li>
          ))}
        </ol>
      </section>

      <section
        aria-labelledby="compare-teaser-heading"
        className="mt-12 grid gap-6 border-t border-rule pt-8 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)] lg:items-start"
      >
        <div>
          <p className="rule-label text-ink-soft">Comparison</p>
          <h2
            id="compare-teaser-heading"
            className="mt-2 text-2xl leading-tight font-bold tracking-[-0.015em]"
          >
            Two candidates, two records.
          </h2>
          <p className="mt-3 max-w-md text-sm leading-6 text-ink-soft">
            Compare what each scan established and what each could not. The view
            marks differences in the recorded condition without choosing a
            package for you.
          </p>
          <Link href="/compare" className="link-arrow mt-4 w-fit text-sm">
            Compare two packages
            <ArrowRightIcon className="h-4 w-4" />
          </Link>
        </div>
        <DemoComparisonPreview />
      </section>

      <section
        aria-labelledby="demo-strip-heading"
        className="mt-10 border border-shell-line bg-shell px-4 py-5 text-paper sm:px-6"
      >
        <div className="grid gap-5 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)] lg:items-center">
          <div>
            <p className="rule-label text-[#9dabb0]">Offline demo</p>
            <h2
              id="demo-strip-heading"
              className="mt-2 text-xl font-bold tracking-[-0.01em]"
            >
              Three records, no network.
            </h2>
            <p className="mt-2 max-w-md text-sm leading-6 text-[#c6cfd2]">
              Deterministic fixtures show a clean result, a confirmed finding,
              and a coverage gap. Every demo surface is stamped{" "}
              <span className="font-mono text-amber">DEMO DATA</span>.
            </p>
          </div>
          <ul className="grid border border-shell-line sm:grid-cols-3">
            {scenarios.map((scenario) => (
              <li
                key={scenario.key}
                className="border-b border-shell-line last:border-b-0 sm:border-r sm:border-b-0 sm:last:border-r-0"
              >
                <Link
                  href={`/demo/${scenario.key}`}
                  className="flex h-full flex-col gap-1 px-3 py-3 transition-colors hover:bg-shell-raised"
                >
                  <span className="rule-label text-[#9dabb0]">
                    {scenario.title}
                  </span>
                  <span className="text-sm leading-5">{scenario.summary}</span>
                  <span className="mt-auto pt-2 font-mono text-[0.625rem] tracking-[0.08em] text-amber uppercase">
                    Open specimen
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </section>
    </div>
  );
}
