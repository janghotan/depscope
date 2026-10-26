"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import type { ScanReport } from "@/generated/report-types";
import { CheckRaster } from "@/components/report/CheckRaster";
import { DemoBanner } from "@/components/report/DemoBanner";
import { FindingsLedger, type FindingEntry } from "@/components/report/FindingsLedger";
import { LimitationLedger } from "@/components/report/LimitationLedger";
import { LoadPath } from "@/components/report/LoadPath";
import { RecordActions } from "@/components/report/RecordActions";
import { SourcesTable } from "@/components/report/SourcesTable";
import { StatusStamp } from "@/components/report/StatusStamp";
import { SummaryTally } from "@/components/report/SummaryTally";
import {
  CONDITION_COPY,
  formatTimestamp,
  sortFindings,
  targetLabel,
} from "@/lib/format";

function anchorForIndex(prefix: string, index: number): string {
  return `${prefix}finding-record-${index}`;
}

function scrollToAnchor(anchor: string | undefined) {
  if (!anchor) {
    return;
  }
  const element = document.getElementById(anchor);
  if (!element) {
    return;
  }
  const reduceMotion = window.matchMedia(
    "(prefers-reduced-motion: reduce)",
  ).matches;
  element.scrollIntoView({
    behavior: reduceMotion ? "auto" : "smooth",
    block: "center",
  });
  element.querySelector<HTMLElement>("button, [href]")?.focus({
    preventScroll: true,
  });
}

export function ReportView({
  report,
  bare = false,
  showDemoBanner = true,
  idPrefix = "",
}: {
  report: ScanReport;
  bare?: boolean;
  showDemoBanner?: boolean;
  idPrefix?: string;
}) {
  const [openIds, setOpenIds] = useState<ReadonlySet<string>>(new Set());
  const pendingFocusRef = useRef<string | null>(null);

  const entries: FindingEntry[] = useMemo(
    () =>
      sortFindings(report.findings).map((finding, index) => ({
        finding,
        anchor: anchorForIndex(idPrefix, index),
      })),
    [report.findings, idPrefix],
  );

  const anchorFor = useMemo(
    () => new Map(entries.map(({ finding, anchor }) => [finding.id, anchor])),
    [entries],
  );

  useEffect(() => {
    const target = pendingFocusRef.current;
    if (!target) {
      return;
    }
    pendingFocusRef.current = null;
    scrollToAnchor(target);
  }, [openIds]);

  function toggle(id: string) {
    setOpenIds((current) => {
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  function openFromPath(id: string) {
    const anchor = anchorFor.get(id);
    if (openIds.has(id)) {
      scrollToAnchor(anchor);
      return;
    }
    pendingFocusRef.current = anchor ?? null;
    setOpenIds((current) => {
      if (current.has(id)) {
        return current;
      }
      const next = new Set(current);
      next.add(id);
      return next;
    });
  }

  function setAll(open: boolean) {
    setOpenIds(
      open ? new Set(entries.map(({ finding }) => finding.id)) : new Set(),
    );
  }

  const condition = CONDITION_COPY[report.status];

  return (
    <article aria-label={`Condition record for ${targetLabel(report)}`}>
      <div className={bare ? "bg-sheet" : "sheet"}>
        <header
          className={`${bare ? "border-y border-rule" : "sheet-top"} px-4 pt-4 pb-5 sm:px-6 lg:px-8`}
        >
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-rule pb-3">
            <span
              className={`tag ${
                report.dataMode === "demo"
                  ? "tag-medium"
                  : "border-inspection-deep text-inspection-deep"
              }`}
            >
              {report.dataMode === "demo" ? "Demo data" : "Live data"}
            </span>
            <span className="rule-label text-ink-soft">
              Schema {report.schemaVersion}
            </span>
            <span className="rule-label ml-auto text-ink-soft">
              Scanned {formatTimestamp(report.scannedAt)}
            </span>
          </div>

          <div className="mt-5 flex flex-col gap-5 md:flex-row md:items-start md:justify-between">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="tag tag-low">
                  {report.target.package.ecosystem}
                </span>
                <span className="data-value text-2xl leading-tight font-semibold break-all sm:text-3xl">
                  {report.target.package.name}
                </span>
              </div>
              <p className="data-value mt-2 text-sm text-ink-soft">
                {report.target.resolvedVersion
                  ? `Resolved version ${report.target.resolvedVersion}`
                  : "Version unresolved"}
              </p>
            </div>
            <div className="shrink-0 self-start md:self-auto">
              <StatusStamp status={report.status} animate />
            </div>
          </div>

          <p className="mt-5 max-w-3xl text-sm leading-6 text-ink-soft">
            <span className="font-semibold text-ink">{condition.label}.</span>{" "}
            {condition.description}
          </p>

          <div className="mt-5">
            <RecordActions report={report} />
          </div>
        </header>

        <div className="px-4 pt-5 sm:px-6 lg:px-8">
          <SummaryTally report={report} checks={report.checks} />
        </div>

        {showDemoBanner && report.dataMode === "demo" && report.demoNotice ? (
          <div className="px-4 pt-5 sm:px-6 lg:px-8">
            <DemoBanner notice={report.demoNotice} />
          </div>
        ) : null}

        <div className="grid gap-10 px-4 py-8 sm:px-6 lg:grid-cols-[minmax(0,1.55fr)_minmax(0,1fr)] lg:px-8">
          <div className="min-w-0 space-y-10">
            <FindingsLedger
              entries={entries}
              sources={report.sources}
              openIds={openIds}
              onToggle={toggle}
              onSetAll={setAll}
              idPrefix={idPrefix}
            />
            <LoadPath
              graph={report.dependencies}
              findings={sortFindings(report.findings)}
              anchorFor={anchorFor}
              openIds={openIds}
              onOpenFinding={openFromPath}
              idPrefix={idPrefix}
            />
          </div>

          <aside className="min-w-0 space-y-10">
            <CheckRaster
              checks={report.checks}
              sources={report.sources}
              idPrefix={idPrefix}
            />
            <SourcesTable sources={report.sources} idPrefix={idPrefix} />
            <LimitationLedger
              limitations={report.limitations}
              idPrefix={idPrefix}
            />
          </aside>
        </div>
      </div>
    </article>
  );
}
