import type { CheckResult, SourceReference } from "@/generated/report-types";
import { CheckStatusGlyph, CheckStatusTag } from "@/components/report/CheckStatusTag";
import { checkTally, providerLabel } from "@/lib/format";

function sourceName(sources: readonly SourceReference[], id: string): string {
  const source = sources.find((candidate) => candidate.id === id);
  return source ? `${providerLabel(source)} / ${source.identifier}` : id;
}

export function CheckRaster({
  checks,
  sources,
  idPrefix = "",
}: {
  checks: readonly CheckResult[];
  sources: readonly SourceReference[];
  idPrefix?: string;
}) {
  const tally = checkTally(checks);
  const headingId = `${idPrefix}checks-heading`;

  return (
    <section aria-labelledby={headingId}>
      <div className="border-b border-rule pb-3">
        <h2 id={headingId} className="text-xl font-bold tracking-[-0.01em]">
          Coverage checks
        </h2>
        <p className="mt-1 text-sm text-ink-soft">
          {tally.total === 0
            ? "No checks were recorded."
            : `${tally.succeeded} of ${tally.total} succeeded · ${tally.unavailable} unavailable · ${tally.skipped} skipped · ${tally.failed} failed`}
        </p>
      </div>

      {checks.length === 0 ? (
        <p className="mt-3 border border-amber-deep bg-amber/20 px-3 py-3 text-sm leading-6">
          No checks were recorded. This coverage gap must not be read as a clean
          result.
        </p>
      ) : (
        <ol className="divide-y divide-rule">
          {checks.map((check) => (
            <li key={check.id} className="flex gap-3 py-4">
              <CheckStatusGlyph status={check.status} />
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-sm font-semibold">{check.name}</h3>
                  <CheckStatusTag status={check.status} />
                </div>
                <p className="mt-1 text-sm leading-6 text-ink-soft">
                  {check.message}
                </p>
                {check.sourceIds.length > 0 ? (
                  <ul className="mt-2 flex flex-wrap gap-x-4 gap-y-1">
                    {check.sourceIds.map((id) => (
                      <li
                        key={id}
                        className="font-mono text-[0.625rem] leading-4 text-ink-soft"
                      >
                        {sourceName(sources, id)}
                      </li>
                    ))}
                  </ul>
                ) : null}
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
