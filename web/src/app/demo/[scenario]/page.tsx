import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { DemoBanner } from "@/components/report/DemoBanner";
import { ReportView } from "@/components/report/ReportView";
import { listDemoScenarios, getDemoScenario } from "@/lib/demo";

export const dynamicParams = false;

export function generateStaticParams() {
  return listDemoScenarios().map((scenario) => ({ scenario: scenario.key }));
}

export async function generateMetadata({
  params,
}: PageProps<"/demo/[scenario]">): Promise<Metadata> {
  const { scenario: key } = await params;
  const scenario = getDemoScenario(key);
  return {
    title: scenario ? `${scenario.title} — demo specimen` : "Demo specimen",
    description: scenario?.summary,
    robots: { index: false, follow: false },
  };
}

export default async function DemoScenarioPage({
  params,
}: PageProps<"/demo/[scenario]">) {
  const { scenario: key } = await params;
  const scenario = getDemoScenario(key);
  if (!scenario) {
    notFound();
  }

  const others = listDemoScenarios().filter((item) => item.key !== scenario.key);

  return (
    <div className="record-container py-8 lg:py-10">
      <nav aria-label="Demo breadcrumb" className="mb-4">
        <Link href="/demo" className="link-quiet text-sm">
          ← All demo specimens
        </Link>
      </nav>

      <header className="border-b border-rule pb-5">
        <p className="rule-label text-ink-soft">
          Demo specimen · {scenario.report.target.package.ecosystem} ·{" "}
          {scenario.report.target.resolvedVersion ?? "unresolved"}
        </p>
        <div className="mt-2 flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between lg:gap-12">
          <h1 className="max-w-2xl text-3xl leading-[1.05] font-bold tracking-[-0.02em] sm:text-4xl">
            {scenario.title}
          </h1>
          <p className="max-w-md text-sm leading-6 text-ink-soft">
            {scenario.demonstrates}
          </p>
        </div>
      </header>

      <div className="mt-5">
        <DemoBanner
          notice={
            scenario.report.demoNotice ??
            "DEMO DATA - synthetic fixture; not a real security result."
          }
        />
      </div>

      <div className="mt-6">
        <ReportView report={scenario.report} showDemoBanner={false} />
      </div>

      <nav
        aria-label="Other demo specimens"
        className="mt-8 border-t border-rule pt-5"
      >
        <p className="rule-label text-ink-soft">Other specimens</p>
        <ul className="mt-3 grid gap-3 sm:grid-cols-2">
          {others.map((other) => (
            <li key={other.key} className="border border-rule bg-sheet">
              <Link
                href={`/demo/${other.key}`}
                className="block px-4 py-3 hover:bg-paper-deep"
              >
                <span className="flex flex-wrap items-center gap-2">
                  <span className="text-sm font-semibold">{other.title}</span>
                  <span className="tag tag-medium">Demo data</span>
                </span>
                <span className="mt-1 block text-xs leading-5 text-ink-soft">
                  {other.summary}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      </nav>
    </div>
  );
}
