"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

type EngineState = "checking" | "online" | "unreachable";

const COPY: Record<EngineState, { label: string; description: string }> = {
  checking: {
    label: "Checking",
    description: "Checking whether the local scan API is reachable.",
  },
  online: {
    label: "Online",
    description:
      "The local scan API is reachable. Live scans are available. This is a process health signal, not a guarantee that npm, PyPI, or OSV are reachable.",
  },
  unreachable: {
    label: "Unreachable",
    description:
      "The local scan API is not reachable. Offline demo records are still available. Read the API notes for how to start it.",
  },
};

export function EngineStatus() {
  const [state, setState] = useState<EngineState>("checking");

  useEffect(() => {
    const controller = new AbortController();
    let cancelled = false;

    async function check() {
      try {
        const response = await fetch("/api/health", {
          cache: "no-store",
          credentials: "same-origin",
          signal: controller.signal,
        });
        if (!response.ok) {
          throw new Error("unavailable");
        }
        const body: unknown = await response.json();
        if (
          typeof body === "object" &&
          body !== null &&
          "status" in body &&
          body.status === "ok"
        ) {
          if (!cancelled) {
            setState("online");
          }
          return;
        }
        throw new Error("unexpected response");
      } catch {
        if (!cancelled) {
          setState("unreachable");
        }
      }
    }

    void check();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, []);

  const copy = COPY[state];

  return (
    <Link
      href="/about#api"
      title={copy.description}
      className="rule-label group flex min-h-10 items-center gap-2 text-[#c6cfd2] transition-colors hover:text-paper"
    >
      <span
        aria-hidden="true"
        className={`inline-block h-2 w-2 border ${
          state === "online"
            ? "border-amber bg-amber"
            : state === "checking"
              ? "border-[#c6cfd2] bg-transparent"
              : "border-[#c6cfd2] bg-transparent"
        }`}
      />
      <span className="hidden sm:inline">Engine</span>
      <span className="text-paper" aria-live="polite">
        {copy.label}
      </span>
    </Link>
  );
}
