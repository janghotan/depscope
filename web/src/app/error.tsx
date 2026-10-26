"use client";

import Link from "next/link";

import { AlertIcon, RefreshIcon } from "@/components/icons";

export default function ErrorPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="record-container py-12 lg:py-16">
      <section
        role="alert"
        aria-labelledby="page-error-heading"
        className="max-w-2xl border-t-2 border-oxide-deep pt-5"
      >
        <div className="flex items-start gap-3">
          <AlertIcon className="mt-1 h-5 w-5 shrink-0 text-oxide-deep" />
          <div>
            <p className="rule-label text-ink-soft">
              Reference error · no condition established
            </p>
            <h1
              id="page-error-heading"
              className="mt-2 text-3xl leading-tight font-bold tracking-[-0.02em]"
            >
              This view could not be rendered.
            </h1>
          </div>
        </div>
        <p className="mt-4 max-w-xl text-sm leading-6 text-ink-soft">
          No condition record is displayed from a partial or failed render. You
          can retry this view, return to the inspection desk, or open a demo
          record that needs no network.
        </p>
        {error.digest ? (
          <p className="data-value mt-4 text-xs text-ink-soft">
            Reference: {error.digest}
          </p>
        ) : null}
        <div className="mt-6 flex flex-wrap items-center gap-3">
          <button type="button" className="button-primary text-sm" onClick={reset}>
            <span className="flex items-center gap-2">
              <RefreshIcon className="h-4 w-4" />
              Retry this view
            </span>
          </button>
          <Link href="/" className="button-secondary text-sm">
            Inspection desk
          </Link>
          <Link href="/demo" className="link-quiet text-sm">
            Open a demo record
          </Link>
        </div>
      </section>
    </div>
  );
}
