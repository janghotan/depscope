import { ScanReport } from "./generated/report-types";

/**
 * 判断扫描报告是否覆盖不完整或存在失败/不可用的检查。
 * 按照 DepScope 设计原则，任何失败或未通过的检查均不能视为 Clean。
 */
export function isReportIncomplete(report: ScanReport): boolean {
  if (report.status === "incomplete" || report.status === "failed") {
    return true;
  }
  const checks = report.checks || [];
  if (checks.length === 0) {
    return true;
  }
  return checks.some((c) => c.status === "failed" || c.status === "unavailable" || c.status === "skipped");
}

/**
 * 判断软件包是否属于完全无风险且所有检查均成功的 Clean 状态。
 * 必须同时满足：零已确认发现、零启发式警告、所有检查均 succeeded、状态为 clean。
 */
export function isReportClean(report: ScanReport): boolean {
  const findings = report.findings || [];
  if (findings.length > 0) {
    return false;
  }
  if (report.status !== "clean") {
    return false;
  }
  const checks = report.checks || [];
  if (checks.length === 0) {
    return false;
  }
  return checks.every((c) => c.status === "succeeded");
}
