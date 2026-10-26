"use client";

import { useEffect, useRef, useState } from "react";

import type { ApiRequestError } from "@/lib/api-client";
import type { CompareInput, CompareResponse } from "@/lib/api-types";
import { ComparisonView } from "@/components/compare/ComparisonView";
import { RequestErrorPanel } from "@/components/request/RequestErrorPanel";

type ViewState =
  | { kind: "loading" }
  | { kind: "cancelled" }
  | { kind: "error"; error: ApiRequestError; retryAfter: string | null }
  | { kind: "success"; response: CompareResponse };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function readError(payload: unknown): ApiRequestError | null {
  if (!isRecord(payload) || !isRecord(payload.error)) {
    return null;
  }
  const { code, message } = payload.error;
  if (typeof code !== "string" || typeof message !== "string") {
    return null;
  }
  return { status: 0, code, message };
}

function looksLikeSide(value: unknown): boolean {
  if (!isRecord(value) || !isRecord(value.request)) {
    return false;
  }
  const hasReport = "report" in value;
  const hasError = "error" in value;
  if (hasReport === hasError) {
    return false;
  }
  return hasReport
    ? isRecord(value.report) && value.report.schemaVersion === "1.0.0"
    : isRecord(value.error) && typeof value.error.message === "string";
}

function looksLikeCompare(value: unknown): value is CompareResponse {
  return (
    isRecord(value) &&
    looksLikeSide(value.left) &&
    looksLikeSide(value.right)
  );
}

function compareSignature(request: CompareInput): string {
  const side = (value: CompareInput["left"]) =>
    `${value.ecosystem}:${value.name}@${value.version ?? "latest"}`;
  return `${side(request.left)}|${side(request.right)}`;
}

function RequestSide({
  label,
  input,
}: {
  label: string;
  input: CompareInput["left"];
}) {
  return (
    <li className="flex flex-wrap items-baseline gap-x-2 gap-y-1 py-1.5">
      <span className="rule-label w-12 text-ink-soft">{label}</span>
      <span className="tag tag-low">{input.ecosystem}</span>
      <span className="data-value text-sm break-all">{input.name}</span>
      <span className="data-value text-xs text-ink-soft">
        {input.version ? `@ ${input.version}` : "latest requested"}
      </span>
    </li>
  );
}

function CompareAttempt({
  request,
  onRetry,
}: {
  request: CompareInput;
  onRetry: () => void;
}) {
  const [state, setState] = useState<ViewState>({ kind: "loading" });
  const [elapsed, setElapsed] = useState(0);
  const controllerRef = useRef<AbortController | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    controllerRef.current?.abort();
    controllerRef.current = controller;
    let active = true;

    const started = Date.now();
    const ticker = window.setInterval(() => {
      if (active) {
        setElapsed(Math.floor((Date.now() - started) / 1000));
      }
    }, 1000);

    async function run() {
      try {
        const response = await fetch("/api/compare", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(request),
          signal: controller.signal,
          cache: "no-store",
        });
        if (!active || controller.signal.aborted) {
          return;
        }
        const payload: unknown = await response.json().catch(() => null);
        if (!active || controller.signal.aborted) {
          return;
        }
        if (!response.ok) {
          const error =
            readError(payload) ?? {
              status: response.status,
              code: "request_failed",
              message: "The request did not produce a comparison.",
            };
          setState({
            kind: "error",
            error,
            retryAfter: response.headers.get("retry-after"),
          });
          return;
        }
        if (!looksLikeCompare(payload)) {
          setState({
            kind: "error",
            error: {
              status: 502,
              code: "contract_violation",
              message:
                "The comparison response could not be accepted as a pair of v1 condition records.",
            },
            retryAfter: null,
          });
          return;
        }
        setState({ kind: "success", response: payload });
      } catch {
        if (!active) {
          return;
        }
        if (controller.signal.aborted) {
          setState({ kind: "cancelled" });
          return;
        }
        setState({
          kind: "error",
          error: {
            status: 502,
            code: "upstream_unavailable",
            message:
              "The comparison request could not be completed. Check your connection and the local API.",
          },
          retryAfter: null,
        });
      } finally {
        window.clearInterval(ticker);
      }
    }

    void run();
    return () => {
      active = false;
      window.clearInterval(ticker);
      controller.abort();
    };
    // 两侧请求字段决定是否重新发起比较；重试通过重新挂载尝试组件完成。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    request.left.ecosystem,
    request.left.name,
    request.left.version,
    request.right.ecosystem,
    request.right.name,
    request.right.version,
  ]);

  if (state.kind === "loading") {
    return (
      <section className="sheet" aria-labelledby="compare-loading-heading">
        <div className="sheet-top px-4 py-5 sm:px-6">
          <span className="tag border-amber-deep text-amber-deep">
            Request in flight
          </span>
          <h2
            id="compare-loading-heading"
            className="mt-4 text-xl font-bold tracking-[-0.01em]"
          >
            Waiting for both sides
          </h2>
          <ul className="mt-3 max-w-xl divide-y divide-rule border-y border-rule">
            <RequestSide label="Side A" input={request.left} />
            <RequestSide label="Side B" input={request.right} />
          </ul>
          <p className="mt-4 max-w-xl text-sm leading-6 text-ink-soft">
            Each side is scanned independently. The API returns one comparison
            envelope when both sides are settled; it does not stream partial
            progress.
          </p>
          <div className="scan-track mt-5" aria-hidden="true" />
          <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
            <p className="data-value text-xs text-ink-soft" aria-live="polite">
              Waiting · <span aria-hidden="true">{elapsed}s</span>
              <span className="sr-only">{elapsed} seconds elapsed</span>
            </p>
            <button
              type="button"
              className="button-secondary text-xs"
              onClick={() => {
                controllerRef.current?.abort();
                setState({ kind: "cancelled" });
              }}
            >
              Cancel request
            </button>
          </div>
        </div>
      </section>
    );
  }

  if (state.kind === "cancelled") {
    return (
      <section className="sheet px-4 py-6 sm:px-6" aria-live="polite">
        <p className="rule-label text-ink-soft">Request cancelled</p>
        <h2 className="mt-2 text-xl font-bold">No comparison was produced</h2>
        <p className="mt-2 max-w-xl text-sm leading-6 text-ink-soft">
          The request was cancelled before both sides settled. Neither side is
          displayed from a partial scan.
        </p>
        <button
          type="button"
          className="button-secondary mt-4 text-sm"
          onClick={onRetry}
        >
          Run this comparison again
        </button>
      </section>
    );
  }

  if (state.kind === "error") {
    return (
      <RequestErrorPanel
        error={state.error}
        retryAfter={state.retryAfter}
        onRetry={onRetry}
        retryLabel="Retry this comparison"
      />
    );
  }

  return <ComparisonView response={state.response} />;
}

export function CompareRunner({ request }: { request: CompareInput }) {
  const [attempt, setAttempt] = useState(0);
  return (
    <CompareAttempt
      key={`${compareSignature(request)}:${attempt}`}
      request={request}
      onRetry={() => setAttempt((value) => value + 1)}
    />
  );
}
