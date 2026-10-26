import { describe, expect, it } from "vitest";

import { demoFindingsReport } from "@/test/fixtures";
import {
  checkTally,
  findingsForNode,
  formatTimestamp,
  plainTextReport,
  providerLabel,
  sortFindings,
  targetLabel,
} from "@/lib/format";

describe("report formatting", () => {
  it("formats timestamps as stable UTC without local drift", () => {
    expect(formatTimestamp("2026-01-15T12:00:00Z")).toBe(
      "2026-01-15 12:00 UTC",
    );
    expect(formatTimestamp("2026-03-01T00:05:00Z")).toBe(
      "2026-03-01 00:05 UTC",
    );
  });

  it("keeps invalid timestamps visible rather than inventing a date", () => {
    expect(formatTimestamp("not-a-date")).toBe("not-a-date");
  });

  it("labels a report target with its resolved version", () => {
    expect(targetLabel(demoFindingsReport())).toBe(
      "npm · depscope-demo-vulnerable-package @ 2.0.0",
    );
  });

  it("orders confirmed findings before heuristic signals", () => {
    const report = demoFindingsReport();
    const ordered = sortFindings(report.findings);
    expect(ordered.map((finding) => finding.kind)).toEqual([
      "confirmed",
      "heuristic",
    ]);
  });

  it("tallies every check status", () => {
    const report = demoFindingsReport();
    expect(checkTally(report.checks)).toEqual({
      total: 4,
      succeeded: 4,
      unavailable: 0,
      skipped: 0,
      failed: 0,
    });
  });

  it("matches findings to their exact dependency node", () => {
    const report = demoFindingsReport();
    const root = report.dependencies.nodes.find(
      (node) => node.id === report.dependencies.rootNodeId,
    );
    const child = report.dependencies.nodes.find(
      (node) => node.id !== report.dependencies.rootNodeId,
    );

    expect(root).toBeDefined();
    expect(child).toBeDefined();
    expect(
      findingsForNode(root!, report.findings).map((finding) => finding.kind),
    ).toEqual(["confirmed"]);
    expect(
      findingsForNode(child!, report.findings).map((finding) => finding.kind),
    ).toEqual(["heuristic"]);
  });

  it("uses explicit provider labels", () => {
    expect(
      providerLabel({
        id: "demo",
        provider: "demo",
        identifier: "DEMO-1",
        retrievedAt: "2026-01-15T12:00:00Z",
      }),
    ).toBe("Demo fixture");
  });

  it("renders a plain-text record with the demo notice and no markup", () => {
    const text = plainTextReport(demoFindingsReport());
    expect(text).toContain("DEPSCOPE CONDITION RECORD");
    expect(text).toContain("Data mode: DEMO");
    expect(text).toContain("Demo notice: DEMO DATA");
    expect(text).toContain("Condition: FINDINGS");
    expect(text).toContain("SOURCES");
    expect(text).not.toContain("<");
    expect(text).not.toContain(">");
  });
});
