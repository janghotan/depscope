import type { SourceReference } from "@/generated/report-types";
import { ExternalLinkIcon } from "@/components/icons";
import { formatTimestamp, providerLabel } from "@/lib/format";
import { safeHttpUrl } from "@/lib/safe-url";

export function SourcesTable({
  sources,
  idPrefix = "",
}: {
  sources: readonly SourceReference[];
  idPrefix?: string;
}) {
  const headingId = `${idPrefix}sources-heading`;

  return (
    <section aria-labelledby={headingId}>
      <div className="border-b border-rule pb-3">
        <h2 id={headingId} className="text-xl font-bold tracking-[-0.01em]">
          Sources
        </h2>
        <p className="mt-1 text-sm text-ink-soft">
          What the record was built from, and when each reference was retrieved.
        </p>
      </div>

      {sources.length === 0 ? (
        <p className="mt-3 border border-rule bg-paper-deep px-3 py-2 text-sm text-ink-soft">
          No sources were recorded for this report.
        </p>
      ) : (
        <div className="scroll-x mt-3">
          <table className="w-full min-w-[34rem] border-collapse text-left text-sm">
            <caption className="sr-only">
              Sources recorded for this scan, with provider, identifier, and
              retrieval time
            </caption>
            <thead>
              <tr className="border-y border-rule-strong">
                <th scope="col" className="rule-label py-2 pr-3 text-ink-soft">
                  Provider
                </th>
                <th scope="col" className="rule-label py-2 pr-3 text-ink-soft">
                  Identifier
                </th>
                <th scope="col" className="rule-label py-2 text-ink-soft">
                  Retrieved
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-rule">
              {sources.map((source) => {
                const url = safeHttpUrl(source.url);
                return (
                  <tr key={source.id}>
                    <td className="py-2.5 pr-3 align-top font-semibold">
                      {providerLabel(source)}
                    </td>
                    <td className="data-value py-2.5 pr-3 align-top text-xs break-all">
                      {source.identifier}
                      {url ? (
                        <a
                          href={url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="link-quiet ml-2 inline-flex items-center gap-1 font-sans text-xs"
                        >
                          Open
                          <ExternalLinkIcon className="h-3 w-3" />
                          <span className="sr-only">(opens in a new tab)</span>
                        </a>
                      ) : null}
                    </td>
                    <td className="data-value py-2.5 align-top text-xs whitespace-nowrap text-ink-soft">
                      {formatTimestamp(source.retrievedAt)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
