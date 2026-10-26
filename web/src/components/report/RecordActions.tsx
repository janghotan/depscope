"use client";

import { useEffect, useRef, useState } from "react";

import type { ScanReport } from "@/generated/report-types";
import { CopyIcon, InfoIcon } from "@/components/icons";
import { serializeReport } from "@/lib/export";
import { plainTextReport } from "@/lib/format";

type CopyState = "idle" | "copied" | "failed";

export function RecordActions({ report }: { report: ScanReport }) {
  const [state, setState] = useState<CopyState>("idle");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const text = plainTextReport(report);

  useEffect(() => {
    return () => {
      if (timer.current) {
        clearTimeout(timer.current);
      }
    };
  }, []);

  async function copyRecord() {
    try {
      if (!navigator.clipboard) {
        throw new Error("Clipboard unavailable");
      }
      await navigator.clipboard.writeText(text);
      setState("copied");
    } catch {
      setState("failed");
    }
    if (timer.current) {
      clearTimeout(timer.current);
    }
    timer.current = setTimeout(() => setState("idle"), 3000);
  }

  function downloadJSON() {
    const output = serializeReport(report);
    const blob = new Blob([output.body], { type: output.contentType });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = output.filename;
    link.hidden = true;
    document.body.appendChild(link);
    link.click();
    link.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  }

  return (
    <div data-no-print className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => void copyRecord()}
          className="button-secondary text-xs"
        >
          <CopyIcon className="h-4 w-4" />
          Copy summary
        </button>
        <button
          type="button"
          onClick={() => window.print()}
          className="button-secondary text-xs"
        >
          Print record
        </button>
        <button
          type="button"
          onClick={downloadJSON}
          className="button-secondary text-xs"
        >
          Download JSON
        </button>
        <span
          role="status"
          className="font-mono text-[0.6875rem] tracking-[0.08em] text-ink-soft uppercase"
        >
          {state === "copied"
            ? "Copied to clipboard"
            : state === "failed"
              ? "Copy failed — use the text record"
              : ""}
        </span>
      </div>

      <details className="group">
        <summary className="rule-label flex w-fit cursor-pointer list-none items-center gap-1.5 text-ink-soft hover:text-ink">
          <InfoIcon className="h-3.5 w-3.5" />
          Plain-text record
        </summary>
        <pre className="mt-2 max-h-72 overflow-auto border border-rule bg-paper-deep p-3 font-mono text-xs leading-5 whitespace-pre-wrap">
          {text}
        </pre>
      </details>
    </div>
  );
}
