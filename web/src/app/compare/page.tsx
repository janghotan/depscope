import type { Metadata } from "next";
import Link from "next/link";

import { AlertIcon, ArrowRightIcon } from "@/components/icons";
import { CompareForm } from "@/components/forms/CompareForm";
import { PageMasthead } from "@/components/site/PageMasthead";
import { parseCompareInput, singleParam } from "@/lib/request";
import { CompareRunner } from "./CompareRunner";

export const metadata: Metadata = {
  title: "Compare packages",
  description:
    "Compare two recorded dependency conditions side by side without treating missing coverage as safety.",
};

function rawSide(
  params: Record<string, string | string[] | undefined>,
  prefix: "left" | "right",
) {
  const version = singleParam(params[`${prefix}Version`]);
  return {
    ecosystem: singleParam(params[`${prefix}Ecosystem`]),
    name: singleParam(params[`${prefix}Name`]),
    ...(version === undefined ? {} : { version }),
  };
}

function compareKey(input: {
  left: { ecosystem: string; name: string; version?: string };
  right: { ecosystem: string; name: string; version?: string };
}): string {
  const side = (value: { ecosystem: string; name: string; version?: string }) =>
    `${value.ecosystem}:${value.name}@${value.version ?? "latest"}`;
  return `${side(input.left)}|${side(input.right)}`;
}

const COMPARE_KEYS = [
  "leftEcosystem",
  "leftName",
  "leftVersion",
  "rightEcosystem",
  "rightName",
  "rightVersion",
] as const;

export default async function ComparePage({
  searchParams,
}: PageProps<"/compare">) {
  const params = await searchParams;
  const hasRequest = COMPARE_KEYS.some((key) => params[key] !== undefined);
  const parsed = hasRequest
    ? parseCompareInput({
        left: rawSide(params, "left"),
        right: rawSide(params, "right"),
      })
    : null;
  const request = parsed?.ok ? parsed.value : null;

  return (
    <div className="record-container py-8 lg:py-10">
      <PageMasthead
        index="Inspection 02 · two packages"
        title="Compare two packages"
        intro="Each side is resolved and scanned independently. The comparison reports both recorded conditions and marks what differs; it does not pick a winner."
      />

      <div className="mt-6 grid gap-6 lg:grid-cols-[minmax(0,23rem)_minmax(0,1fr)] lg:items-start">
        <details className="sheet sheet-top lg:sticky lg:top-24" open={!request}>
          <summary className="cursor-pointer list-none px-4 py-4 sm:px-5">
            <div className="flex items-baseline justify-between gap-3">
              <h2 className="text-lg font-bold">Request</h2>
              <span className="rule-label text-ink-soft">
                {request ? "Revise" : "Two sides"}
              </span>
            </div>
            {request ? (
              <p className="data-value mt-2 text-xs leading-5 break-all text-ink-soft">
                A · {request.left.ecosystem}:{request.left.name}
                {request.left.version ? ` @ ${request.left.version}` : ""}
                <br />
                B · {request.right.ecosystem}:{request.right.name}
                {request.right.version ? ` @ ${request.right.version}` : ""}
              </p>
            ) : (
              <p className="mt-2 text-sm leading-6 text-ink-soft">
                Name both candidates. A side may be npm or PyPI, and either side
                may fail without hiding the other.
              </p>
            )}
          </summary>
          <div className="border-t border-rule px-4 py-5 sm:px-5">
            <CompareForm
              initialLeft={request?.left}
              initialRight={request?.right}
              key={request ? compareKey(request) : "empty"}
            />
          </div>
        </details>

        <div className="min-w-0">
          {parsed && !parsed.ok ? (
            <section
              role="alert"
              aria-labelledby="compare-request-error"
              className="border border-oxide-deep bg-sheet"
            >
              <div className="flex items-start gap-3 border-b border-rule px-4 py-3">
                <AlertIcon className="mt-0.5 h-5 w-5 shrink-0 text-oxide-deep" />
                <div>
                  <h2 id="compare-request-error" className="text-base font-bold">
                    This address cannot be used as a comparison request
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
                  Both sides need an ecosystem and a package name. Each side may
                  include an exact version; range operators, URL-like values,
                  and extra fields are rejected.
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <Link href="/demo/compare" className="button-secondary text-sm">
                    Open demo comparison
                  </Link>
                  <Link href="/about#comparison" className="link-quiet text-sm">
                    How comparison reads
                  </Link>
                </div>
              </div>
            </section>
          ) : request ? (
            <CompareRunner request={request} key={compareKey(request)} />
          ) : (
            <section aria-labelledby="compare-awaiting-heading" className="sheet">
              <div className="sheet-top px-4 py-5 sm:px-6">
                <p className="rule-label text-ink-soft">Awaiting request</p>
                <h2
                  id="compare-awaiting-heading"
                  className="mt-2 text-xl font-bold tracking-[-0.01em]"
                >
                  No comparison has been requested.
                </h2>
                <p className="mt-2 max-w-xl text-sm leading-6 text-ink-soft">
                  Submit both sides to see their conditions side by side. A
                  failed side is presented as a failed side, not as a clean or
                  missing result.
                </p>
                <div className="mt-5 grid gap-4 sm:grid-cols-2">
                  <div className="border border-rule bg-paper-deep px-4 py-3">
                    <p className="rule-label text-ink-soft">The view shows</p>
                    <ul className="mt-2 space-y-1.5 text-sm leading-5">
                      <li>Recorded condition and findings counts</li>
                      <li>Highest severity and check coverage</li>
                      <li>Which measures differ between the sides</li>
                      <li>Each side&apos;s full condition record</li>
                    </ul>
                  </div>
                  <div className="border border-rule bg-paper-deep px-4 py-3">
                    <p className="rule-label text-ink-soft">The view does not</p>
                    <ul className="mt-2 space-y-1.5 text-sm leading-5">
                      <li>Choose a package for you</li>
                      <li>Merge the two sides into one score</li>
                      <li>Hide a failed or incomplete side</li>
                      <li>Compare demo and live data as equivalent</li>
                    </ul>
                  </div>
                </div>
                <div className="mt-5 flex flex-wrap items-center gap-3">
                  <Link href="/demo/compare" className="button-secondary text-sm">
                    Open demo comparison
                    <ArrowRightIcon className="h-4 w-4" />
                  </Link>
                  <Link href="/about#comparison" className="link-quiet text-sm">
                    How to read a comparison
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
