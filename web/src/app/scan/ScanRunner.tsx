"use client";

import { useEffect, useRef, useState } from "react";

import type { ApiRequestError } from "@/lib/api-client";
import type { PackageInput } from "@/lib/api-types";
import type { ScanReport } from "@/generated/report-types";
import { ReportView } from "@/components/report/ReportView";
import { RequestErrorPanel } from "@/components/request/RequestErrorPanel";

type ViewState =
  | { kind: "loading" }
  | { kind: "cancelled" }
  | { kind: "error"; error: ApiRequestError; retryAfter: string | null }
  | { kind: "success"; report: ScanReport };

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

function looksLikeReport(value: unknown): value is ScanReport {
  return (
    isRecord(value) &&
    value.schemaVersion === "1.0.0" &&
    typeof value.status === "string" &&
    Array.isArray(value.findings) &&
    Array.isArray(value.checks) &&
    isRecord(value.target) &&
    isRecord(value.summary) &&
    isRecord(value.dependencies)
  );
}

function requestSignature(request: PackageInput): string {
  return `${request.ecosystem}:${request.name}@${request.version ?? "latest"}`;
}

function ScanAttempt({
  request,
  onRetry,
}: {
  request: PackageInput;
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
        const response = await fetch("/api/scan", {
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
              message: "The request did not produce a condition record.",
            };
          setState({
            kind: "error",
            error,
            retryAfter: response.headers.get("retry-after"),
          });
          return;
        }
        if (!looksLikeReport(payload)) {
          setState({
            kind: "error",
            error: {
              status: 502,
              code: "contract_violation",
              message:
                "The scan response could not be accepted as a v1 condition record.",
            },
            retryAfter: null,
          });
          return;
        }
        setState({ kind: "success", report: payload });
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
              "The scan request could not be completed. Check your connection and the local API.",
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
    // 请求字段决定是否重新发起扫描；重试通过重新挂载尝试组件完成。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [request.ecosystem, request.name, request.version]);

  if (state.kind === "loading") {
    return (
      <section className="sheet" aria-labelledby="scan-loading-heading">
        <div className="sheet-top px-4 py-5 sm:px-6">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
            <span className="tag border-amber-deep text-amber-deep">
              Request in flight
            </span>
            <span className="data-value text-sm text-ink-soft">
              {request.ecosystem} · {request.name}
              {request.version ? ` @ ${request.version}` : ""}
            </span>
          </div>
          <h2
            id="scan-loading-heading"
            className="mt-4 text-xl font-bold tracking-[-0.01em]"
          >
            Waiting for the scan engine
          </h2>
          <p className="mt-2 max-w-xl text-sm leading-6 text-ink-soft">
            The API does not expose partial progress. This screen waits for the
            finished condition record; the request is cancelled if you leave or
            press cancel.
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
        <h2 className="mt-2 text-xl font-bold">No condition was established</h2>
        <p className="mt-2 max-w-xl text-sm leading-6 text-ink-soft">
          The request was cancelled before the scan finished. Nothing from a
          partial scan is displayed.
        </p>
        <button
          type="button"
          className="button-secondary mt-4 text-sm"
          onClick={onRetry}
        >
          Run this request again
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
        retryLabel="Retry this scan"
      />
    );
  }

  return <ReportView report={state.report} />;
}

export function ScanRunner({ request }: { request: PackageInput }) {
  const [attempt, setAttempt] = useState(0);
  return (
    <ScanAttempt
      key={`${requestSignature(request)}:${attempt}`}
      request={request}
      onRetry={() => setAttempt((value) => value + 1)}
    />
  );
}
