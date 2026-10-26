import Ajv2020 from "ajv/dist/2020";
import type { AnySchemaObject, ValidateFunction } from "ajv";
import addFormats from "ajv-formats";

import schema from "@/generated/scan-report.v1.schema.json";
import type { ScanReport } from "@/generated/report-types";

/** 报告不符合 v1 契约时抛出的错误；path 指向首个失败字段。 */
export class ReportContractError extends Error {
  readonly path: string;

  constructor(message: string, path = "") {
    super(message);
    this.name = "ReportContractError";
    this.path = path;
  }
}

let validateReport: ValidateFunction | null = null;

function getValidator(): ValidateFunction {
  if (validateReport) {
    return validateReport;
  }
  const ajv = new Ajv2020({
    allErrors: false,
    strict: true,
    // 规范允许 maxItems 等关键字不显式声明 type，严格模式会误报。
    strictTypes: false,
  });
  addFormats(ajv);
  validateReport = ajv.compile(schema as AnySchemaObject);
  return validateReport;
}

/** 校验并返回规范扫描报告；不修改输入。 */
export function parseScanReport(value: unknown): ScanReport {
  const validate = getValidator();
  if (!validate(value)) {
    const first = validate.errors?.[0];
    const path = first?.instancePath || "(root)";
    const detail = first?.message ? `: ${first.message}` : "";
    throw new ReportContractError(
      `Report does not match the v1 contract at ${path}${detail}.`,
      path,
    );
  }
  return value as ScanReport;
}

/** 便捷判断，仅用于响应边界；校验仍以 parseScanReport 为准。 */
export function isScanReport(value: unknown): value is ScanReport {
  return getValidator()(value);
}
