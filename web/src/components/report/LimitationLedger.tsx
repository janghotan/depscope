import type { Limitation } from "@/generated/report-types";

export function LimitationLedger({
  limitations,
  idPrefix = "",
}: {
  limitations: readonly Limitation[];
  idPrefix?: string;
}) {
  const headingId = `${idPrefix}limitations-heading`;

  return (
    <section aria-labelledby={headingId}>
      <div className="border-b border-rule pb-3">
        <h2
          id={headingId}
          className="text-xl font-bold tracking-[-0.01em]"
        >
          Limitations
        </h2>
        <p className="mt-1 text-sm text-ink-soft">
          Constraints recorded by the scan engine. They bound what this record can
          prove.
        </p>
      </div>

      {limitations.length === 0 ? (
        <p className="mt-3 text-sm leading-6 text-ink-soft">
          No limitations were recorded for this report. Read the coverage checks
          for what ran.
        </p>
      ) : (
        <ul className="divide-y divide-rule">
          {limitations.map((limitation) => (
            <li key={`${limitation.code}-${limitation.message}`} className="py-3">
              <p className="data-value text-[0.6875rem] font-semibold tracking-[0.06em] text-amber-deep">
                {limitation.code}
              </p>
              <p className="mt-1 text-sm leading-6">{limitation.message}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
