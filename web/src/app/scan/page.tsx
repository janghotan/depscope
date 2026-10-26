import type { Metadata } from "next";
import Link from "next/link";

import { AlertIcon } from "@/components/icons";
import { ScanForm } from "@/components/forms/ScanForm";
import { PageMasthead } from "@/components/site/PageMasthead";
import { packageInputFromParams } from "@/lib/request";
import { ScanRunner } from "./ScanRunner";

export const metadata: Metadata = {
  title: "Scan a package",
  description:
    "Inspect one npm or PyPI package for known advisories, heuristic signals, coverage gaps, and the dependency path that connects them.",
};

function requestKey(input: {
  ecosystem: string;
  name: string;
  version?: string;
}): string {
  return `${input.ecosystem}:${input.name}@${input.version ?? "latest"}`;
}

export default async function ScanPage({ searchParams }: PageProps<"/scan">) {
  const params = await searchParams;
  const parsed = packageInputFromParams(params);
  const request = parsed?.ok ? parsed.value : null;

  return (
    <div className="record-container py-8 lg:py-10">
      <PageMasthead
        index="Inspection 01 · single package"
        title="Scan one package"
        intro="The API resolves the package, matches advisories, records every check, and returns one condition record. Missing coverage stays visible."
      />

      <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,22rem)_minmax(0,1fr)] lg:items-start">
        <div className="sheet sheet-top lg:sticky lg:top-24">
          <div className="px-4 pt-4 sm:px-5">
            <div className="flex flex-wrap items-baseline justify-between gap-2 border-b border-rule pb-3">
              <h2 className="text-lg font-bold">Request</h2>
              <span className="rule-label text-ink-soft">Live scan</span>
            </div>
          </div>
          <div className="px-4 py-5 sm:px-5">
            <ScanForm
              initial={request ?? undefined}
              key={request ? requestKey(request) : "empty"}
              compact={Boolean(request)}
            />
          </div>
          <div className="border-t border-rule px-4 py-3 sm:px-5">
            <p className="text-xs leading-5 text-ink-soft">
              Editing the request creates a new scan. A completed record is not
              stored by this site.
            </p>
          </div>
        </div>

        <div className="min-w-0">
          {parsed && !parsed.ok ? (
            <section
              role="alert"
              aria-labelledby="scan-request-error"
              className="border border-oxide-deep bg-sheet"
            >
              <div className="flex items-start gap-3 border-b border-rule px-4 py-3">
                <AlertIcon className="mt-0.5 h-5 w-5 shrink-0 text-oxide-deep" />
                <div>
                  <h2 id="scan-request-error" className="text-base font-bold">
                    This address cannot be used as a scan request
                  </h2>
                  <p className="mt-1 text-sm leading-6 text-ink-soft">
                    No request was sent to the scan API.
                  </p>
                </div>
              </div>
              <div className="space-y-4 px-4 py-4">
                <p className="data-value border border-rule bg-paper-deep px-3 py-2 text-sm">
                  {parsed.issue.message}
                </p>
                <p className="text-sm leading-6 text-ink-soft">
                  Requests name one npm or PyPI package with an optional exact
                  version. Range operators, URL-like values, and extra fields
                  are rejected rather than approximated.
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <Link href="/demo/findings" className="button-secondary text-sm">
                    Open a demo record
                  </Link>
                  <Link href="/about#checks" className="link-quiet text-sm">
                    What a scan checks
                  </Link>
                </div>
              </div>
            </section>
          ) : request ? (
            <ScanRunner request={request} key={requestKey(request)} />
          ) : (
            <section
              aria-labelledby="scan-awaiting-heading"
              className="sheet"
            >
              <div className="sheet-top px-4 py-5 sm:px-6">
                <p className="rule-label text-ink-soft">Awaiting request</p>
                <h2
                  id="scan-awaiting-heading"
                  className="mt-2 text-xl font-bold tracking-[-0.01em]"
                >
                  No package has been inspected yet.
                </h2>
                <p className="mt-2 max-w-xl text-sm leading-6 text-ink-soft">
                  Submit the form to create a live condition record. The scan
                  requires the local DepScope API; if it is unavailable, this
                  page says so instead of showing an empty result.
                </p>
                <dl className="mt-5 grid border-t border-l border-rule sm:grid-cols-2">
                  <div className="border-r border-b border-rule p-4">
                    <dt className="rule-label text-ink-soft">What is checked</dt>
                    <dd className="mt-2 text-sm leading-6">
                      Package identity, dependency paths, OSV advisories,
                      registry state, and typosquat heuristics.
                    </dd>
                  </div>
                  <div className="border-r border-b border-rule p-4">
                    <dt className="rule-label text-ink-soft">
                      What is never done
                    </dt>
                    <dd className="mt-2 text-sm leading-6">
                      No installs, no lifecycle scripts, no package code
                      execution, and no risk score invented by this site.
                    </dd>
                  </div>
                </dl>
                <div className="mt-5 flex flex-wrap items-center gap-3">
                  <Link href="/demo/findings" className="button-secondary text-sm">
                    Open a demo record
                  </Link>
                  <Link href="/about#record" className="link-quiet text-sm">
                    How to read a record
                  </Link>
                </div>
              </div>
            </section>
          )}
        </div>
      </div>
    </div>
  );
}
