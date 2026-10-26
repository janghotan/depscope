import type { ScanReport } from "@/generated/report-types";

export type SerializedReport = {
  filename: string;
  contentType: "application/json;charset=utf-8";
  body: string;
};

function safeFileSegment(value: string): string {
  const normalized = value
    .toLowerCase()
    .replace(/[^a-z0-9._-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return normalized || "package";
}

export function serializeReport(report: ScanReport): SerializedReport {
  const ecosystem = safeFileSegment(report.target.package.ecosystem);
  const name = safeFileSegment(report.target.package.name);
  const version = safeFileSegment(report.target.resolvedVersion ?? "unresolved");

  return {
    filename: `depscope-${ecosystem}-${name}-${version}.json`,
    contentType: "application/json;charset=utf-8",
    body: `${JSON.stringify(report, null, 2)}\n`,
  };
}
