import type { ScanReport } from "@/generated/report-types";

/** 与 API 请求体一致的包引用；version 省略表示解析最新版本。 */
export type PackageInput = {
  ecosystem: "npm" | "pypi";
  name: string;
  version?: string;
};

/** /api/v1/compare 的两侧输入。 */
export type CompareInput = {
  left: PackageInput;
  right: PackageInput;
};

export type ApiError = {
  code: string;
  message: string;
};

export type ErrorEnvelope = {
  error: ApiError;
};

export type CompareSide = {
  request: PackageInput;
  report?: ScanReport;
  error?: ApiError;
};

export type CompareResponse = {
  left: CompareSide;
  right: CompareSide;
};

export type HealthStatus = {
  status: "ok" | "unavailable";
};
