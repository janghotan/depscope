import { describe, expect, it } from "vitest";

import {
  demoCleanReport,
  demoFindingsReport,
  demoIncompleteReport,
  liveFindingsReport,
} from "@/test/fixtures";
import { isScanReport, parseScanReport, ReportContractError } from "@/lib/schema";

import cleanFixture from "@/generated/demo/demo-clean-report.json";
import findingsFixture from "@/generated/demo/demo-findings-report.json";
import incompleteFixture from "@/generated/demo/demo-incomplete-report.json";

describe("parseScanReport", () => {
  it("accepts every approved demo fixture", () => {
    for (const fixture of [
      cleanFixture,
      findingsFixture,
      incompleteFixture,
    ]) {
      expect(parseScanReport(fixture)).toBe(fixture);
    }
  });

  it("accepts a live report with live sources", () => {
    const live = liveFindingsReport();
    expect(parseScanReport(live)).toBe(live);
  });

  it("rejects a clean report that contains a failed check", () => {
    const failedCheck = {
      ...cleanFixture.checks[0],
      status: "failed" as const,
    };
    const bad = {
      ...cleanFixture,
      checks: [failedCheck, ...cleanFixture.checks.slice(1)],
    };

    expect(() => parseScanReport(bad)).toThrow(ReportContractError);
  });

  it("rejects a clean report that contains findings", () => {
    const bad = {
      ...cleanFixture,
      findings: findingsFixture.findings.slice(0, 1),
    };
    expect(() => parseScanReport(bad)).toThrow(/v1 contract/);
  });

  it("rejects an incomplete report without a gap check", () => {
    const bad = {
      ...demoIncompleteReport(),
      checks: demoCleanReport().checks,
    };
    expect(() => parseScanReport(bad)).toThrow(ReportContractError);
  });

  it("rejects a demo report without a demo notice", () => {
    const bad = { ...cleanFixture, demoNotice: null };
    expect(() => parseScanReport(bad)).toThrow(ReportContractError);
  });

  it("rejects a demo report that records a non-demo source", () => {
    const bad = {
      ...cleanFixture,
      sources: cleanFixture.sources.map((source) => ({
        ...source,
        provider: "osv",
      })),
    };
    expect(() => parseScanReport(bad)).toThrow(ReportContractError);
  });

  it("rejects a live report that keeps a demo notice", () => {
    const bad = { ...demoFindingsReport(), dataMode: "live" };
    expect(() => parseScanReport(bad)).toThrow(ReportContractError);
  });

  it("rejects unknown fields", () => {
    const bad = { ...cleanFixture, unexpected: true };
    expect(() => parseScanReport(bad)).toThrow(/v1 contract/);
  });

  it("reports the failing path", () => {
    const bad = { ...cleanFixture, status: "safe" };
    try {
      parseScanReport(bad);
      expect.unreachable("expected the report to be rejected");
    } catch (error) {
      expect(error).toBeInstanceOf(ReportContractError);
      expect((error as ReportContractError).path).toBe("/status");
    }
  });

  it("isScanReport rejects non-report values", () => {
    expect(isScanReport(null)).toBe(false);
    expect(isScanReport({})).toBe(false);
    expect(isScanReport(cleanFixture)).toBe(true);
  });
});
