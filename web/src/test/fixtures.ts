import type { ScanReport } from "@/generated/report-types";
import { parseScanReport } from "@/lib/schema";

import cleanFixture from "@/generated/demo/demo-clean-report.json";
import findingsFixture from "@/generated/demo/demo-findings-report.json";
import incompleteFixture from "@/generated/demo/demo-incomplete-report.json";

export function demoCleanReport(): ScanReport {
  return parseScanReport(structuredClone(cleanFixture));
}

export function demoFindingsReport(): ScanReport {
  return parseScanReport(structuredClone(findingsFixture));
}

export function demoIncompleteReport(): ScanReport {
  return parseScanReport(structuredClone(incompleteFixture));
}

/** 将演示报告转换为合法的 live 报告，仅用于 API 客户端测试。 */
export function liveFindingsReport(): ScanReport {
  const base = demoFindingsReport();
  return {
    ...base,
    dataMode: "live",
    demoNotice: null,
    sources: base.sources.map((source) => ({
      ...source,
      provider: "osv" as const,
      identifier: source.identifier.replace("DEMO-", "LIVE-"),
    })),
  };
}
