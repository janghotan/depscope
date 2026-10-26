"use client";

import Link from "next/link";

import type { ApiRequestError } from "@/lib/api-client";
import { AlertIcon, RefreshIcon } from "@/components/icons";

function recoveryHint(code: string): string | null {
  switch (code) {
    case "upstream_unavailable":
    case "upstream_misconfigured":
      return "The local scan API must be running for live scans. You can start it from the repository root, or open an offline demo record now.";
    case "scan_timeout":
      return "The scan did not finish in time. Retry, or request an exact published version to narrow the work.";
    case "rate_limited":
    case "server_busy":
      return "The scan service is applying a limit. Wait for the indicated time before retrying.";
    case "contract_violation":
    case "upstream_response_too_large":
      return "The response could not be accepted as a v1 record, so no report is shown. This protects against displaying an unvalidated result.";
    case "request_cancelled":
      return "The request was cancelled. Retry when you are ready.";
    default:
      return null;
  }
}

export function RequestErrorPanel({
  error,
  retryAfter,
  onRetry,
  retryLabel = "Retry",
}: {
  error: ApiRequestError;
  retryAfter?: string | null;
  onRetry: () => void;
  retryLabel?: string;
}) {
  const hint = recoveryHint(error.code);

  return (
    <section
      role="alert"
      aria-labelledby="request-error-heading"
      className="border border-oxide-deep bg-sheet"
    >
      <div className="flex items-start gap-3 border-b border-rule px-4 py-3">
        <AlertIcon className="mt-0.5 h-5 w-5 shrink-0 text-oxide-deep" />
        <div>
          <h2 id="request-error-heading" className="text-base font-bold">
            No condition record was produced
          </h2>
          <p className="data-value mt-1 text-[0.6875rem] text-ink-soft">
            Code: {error.code}
          </p>
        </div>
      </div>
      <div className="space-y-4 px-4 py-4">
        <p className="text-sm leading-6">{error.message}</p>
        {retryAfter ? (
          <p className="text-xs text-ink-soft">
            The service asked clients to wait {retryAfter} seconds.
          </p>
        ) : null}
        {hint ? (
          <p className="border-t border-rule pt-3 text-sm leading-6 text-ink-soft">
            {hint}
          </p>
        ) : null}
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className="button-primary text-sm" onClick={onRetry}>
            <span className="flex items-center gap-2">
              <RefreshIcon className="h-4 w-4" />
              {retryLabel}
            </span>
          </button>
          <Link href="/demo" className="button-secondary text-sm">
            Open a demo record
          </Link>
          <Link href="/about#api" className="link-quiet text-sm">
            API notes
          </Link>
        </div>
      </div>
    </section>
  );
}
