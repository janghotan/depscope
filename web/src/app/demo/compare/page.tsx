import type { Metadata } from "next";
import Link from "next/link";

import { AlertIcon, ArrowRightIcon } from "@/components/icons";
import { ComparisonView } from "@/components/compare/ComparisonView";
import { DemoBanner } from "@/components/report/DemoBanner";
import { PageMasthead } from "@/components/site/PageMasthead";
import {
  buildDemoComparison,
  isDemoScenarioKey,
  listDemoScenarios,
  requireDemoScenario,
} from "@/lib/demo";
import { singleParam } from "@/lib/request";

export const metadata: Metadata = {
  title: "Demo comparison",
  description:
    "Compare two synthetic DepScope fixtures side by side with no network access.",
  robots: { index: false, follow: false },
};

const DEFAULT_LEFT = "findings";
const DEFAULT_RIGHT = "clean";

export default async function DemoComparePage({
  searchParams,
}: PageProps<"/demo/compare">) {
  const params = await searchParams;
  const rawLeft = singleParam(params.left);
  const rawRight = singleParam(params.right);
  const invalidLeft = rawLeft !== undefined && !isDemoScenarioKey(rawLeft);
  const invalidRight = rawRight !== undefined && !isDemoScenarioKey(rawRight);
  const invalid = invalidLeft || invalidRight;

  const leftKey = rawLeft && isDemoScenarioKey(rawLeft) ? rawLeft : DEFAULT_LEFT;
  const rightKey =
    rawRight && isDemoScenarioKey(rawRight) ? rawRight : DEFAULT_RIGHT;
  const response = buildDemoComparison(leftKey, rightKey);
  const scenarios = listDemoScenarios();
  const notice =
    requireDemoScenario("findings").report.demoNotice ??
    "DEMO DATA - synthetic fixture; not a real security result.";

  return (
    <div className="record-container py-8 lg:py-10">
      <PageMasthead
        index="Reference 03 · offline comparison"
        title="Demo comparison"
        intro="Two synthetic condition records are assembled locally into the same comparison envelope the API returns. No scan is performed."
      />

      <div className="mt-6">
        <DemoBanner notice={notice} />
      </div>

      {invalid ? (
        <section
          role="alert"
          aria-labelledby="demo-compare-invalid"
          className="mt-6 border border-oxide-deep bg-sheet"
        >
          <div className="flex items-start gap-3 border-b border-rule px-4 py-3">
            <AlertIcon className="mt-0.5 h-5 w-5 shrink-0 text-oxide-deep" />
            <div>
              <h2 id="demo-compare-invalid" className="text-base font-bold">
                One selected specimen is not available
              </h2>
              <p className="mt-1 text-sm leading-6 text-ink-soft">
                This page fell back to the default pair. Choose a listed specimen
                below.
              </p>
            </div>
          </div>
        </section>
      ) : null}

      <form
        method="get"
        action="/demo/compare"
        className="mt-6 border border-rule-strong bg-sheet px-4 py-4 sm:px-5"
      >
        <div className="flex flex-wrap items-baseline justify-between gap-2 border-b border-rule pb-2">
          <h2 className="text-lg font-bold">Specimen pair</h2>
          <span className="rule-label text-ink-soft">Offline · no network</span>
        </div>
        <div className="mt-4 grid gap-4 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
          <div>
            <label htmlFor="demo-left" className="field-label">
              Side A
            </label>
            <select
              id="demo-left"
              name="left"
              defaultValue={leftKey}
              className="field-input"
            >
              {scenarios.map((scenario) => (
                <option key={scenario.key} value={scenario.key}>
                  {scenario.title} — {scenario.report.status}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="demo-right" className="field-label">
              Side B
            </label>
            <select
              id="demo-right"
              name="right"
              defaultValue={rightKey}
              className="field-input"
            >
              {scenarios.map((scenario) => (
                <option key={scenario.key} value={scenario.key}>
                  {scenario.title} — {scenario.report.status}
                </option>
              ))}
            </select>
          </div>
          <button type="submit" className="button-primary text-sm">
            <span className="flex items-center gap-2">
              Compare specimens
              <ArrowRightIcon className="h-4 w-4" />
            </span>
          </button>
        </div>
        <p className="mt-3 text-xs leading-5 text-ink-soft">
          Both sides are demo data. A comparison of demo fixtures is not a live
          scan and must not be used as a security result.
        </p>
      </form>

      <div className="mt-8">
        <ComparisonView response={response} />
      </div>

      <p className="mt-8 border-t border-rule pt-4 text-sm text-ink-soft">
        <Link href="/demo" className="link-quiet">
          All demo specimens
        </Link>{" "}
        ·{" "}
        <Link href="/compare" className="link-quiet">
          Compare live packages
        </Link>
      </p>
    </div>
  );
}
