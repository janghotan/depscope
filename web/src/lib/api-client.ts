import type {
  ApiError,
  CompareResponse,
  CompareSide,
  PackageInput,
} from "@/lib/api-types";
import { ReportContractError, parseScanReport } from "@/lib/schema";
import type { ScanReport } from "@/generated/report-types";
import { isPlainObject, parsePackageInput } from "@/lib/request";
import { BodyLimitError, InvalidJsonError, readStreamCapped } from "@/lib/http";

const DEFAULT_BASE_URL = "http://127.0.0.1:8080";
const DEFAULT_TIMEOUT_MS = 100_000;
const MIN_TIMEOUT_MS = 1_000;
const MAX_TIMEOUT_MS = 120_000;
const MAX_RESPONSE_BYTES = 8 * 1024 * 1024;

export type ApiConfig = {
  baseUrl: string;
  timeoutMs: number;
  maxResponseBytes: number;
};

export type ApiRequestError = {
  status: number;
  code: string;
  message: string;
  retryAfter?: string;
};

export type ApiResult<T> =
  | { ok: true; value: T }
  | { ok: false; error: ApiRequestError };

/** 只依赖站点用到的环境变量形态，避免把 Next.js 的 ProcessEnv 扩展泄漏到测试。 */
export type ApiEnvironment = Record<string, string | undefined>;

export type ApiClientOptions = {
  signal?: AbortSignal;
  fetchImpl?: typeof fetch;
  env?: ApiEnvironment;
};

/** 读取站点自己的环境配置；无效配置返回 null，由调用方转换为通用错误。 */
export function resolveApiConfig(env: ApiEnvironment = process.env): ApiConfig | null {
  const rawBase = (env.DEPSCOPE_API_BASE_URL ?? DEFAULT_BASE_URL).trim();
  let base: URL;
  try {
    base = new URL(rawBase);
  } catch {
    return null;
  }
  if (base.protocol !== "http:" && base.protocol !== "https:") {
    return null;
  }
  if (base.username || base.password || base.search || base.hash) {
    return null;
  }
  if (base.pathname !== "/") {
    return null;
  }

  const rawTimeout = (env.DEPSCOPE_API_TIMEOUT_MS ?? String(DEFAULT_TIMEOUT_MS)).trim();
  if (!/^\d+$/.test(rawTimeout)) {
    return null;
  }
  const timeoutMs = Number(rawTimeout);
  if (timeoutMs < MIN_TIMEOUT_MS || timeoutMs > MAX_TIMEOUT_MS) {
    return null;
  }

  return {
    baseUrl: base.origin,
    timeoutMs,
    maxResponseBytes: MAX_RESPONSE_BYTES,
  };
}

function errorResult(
  status: number,
  code: string,
  message: string,
  retryAfter?: string,
): ApiResult<never> {
  return {
    ok: false,
    error: { status, code, message, ...(retryAfter ? { retryAfter } : {}) },
  };
}

function isTimeout(error: unknown, callerSignal?: AbortSignal): boolean {
  if (!(error instanceof Error)) {
    return false;
  }
  if (callerSignal?.aborted) {
    return false;
  }
  return error.name === "TimeoutError" || error.name === "AbortError";
}

function sanitizeError(value: unknown): ApiError | null {
  if (!isPlainObject(value)) {
    return null;
  }
  const { code, message } = value;
  if (typeof code !== "string" || !/^[a-z][a-z0-9_]*$/.test(code)) {
    return null;
  }
  if (
    typeof message !== "string" ||
    message.length === 0 ||
    message.length > 500 ||
    /[\u0000-\u001f\u007f]/.test(message)
  ) {
    return null;
  }
  return { code, message };
}

function sanitizeRetryAfter(value: string | null): string | undefined {
  if (!value) {
    return undefined;
  }
  return /^\d{1,6}$/.test(value) ? value : undefined;
}

function requireLiveReport(report: ScanReport): ScanReport {
  if (report.dataMode !== "live" || report.demoNotice !== null) {
    throw new ReportContractError("Upstream report is not a live report.");
  }
  return report;
}

function parseCompareSide(value: unknown): CompareSide {
  if (!isPlainObject(value)) {
    throw new ReportContractError("Comparison side is not an object.");
  }
  const requestResult = parsePackageInput(value.request);
  if (!requestResult.ok) {
    throw new ReportContractError("Comparison request is not valid.");
  }
  const hasReport = "report" in value;
  const hasError = "error" in value;
  if (hasReport === hasError) {
    throw new ReportContractError(
      "Comparison side must contain exactly one of report or error.",
    );
  }
  if (hasReport) {
    return {
      request: requestResult.value,
      report: requireLiveReport(parseScanReport(value.report)),
    };
  }
  const sideError = sanitizeError(value.error);
  if (!sideError) {
    throw new ReportContractError("Comparison side error is not valid.");
  }
  return { request: requestResult.value, error: sideError };
}

function parseCompareResponse(value: unknown): CompareResponse {
  if (!isPlainObject(value)) {
    throw new ReportContractError("Comparison response is not an object.");
  }
  return {
    left: parseCompareSide(value.left),
    right: parseCompareSide(value.right),
  };
}

type RequestOptions<T> = ApiClientOptions & {
  method: "GET" | "POST";
  path: string;
  body?: unknown;
  parse: (value: unknown) => T;
};

async function callApi<T>(options: RequestOptions<T>): Promise<ApiResult<T>> {
  const config = resolveApiConfig(options.env);
  if (!config) {
    return errorResult(
      500,
      "upstream_misconfigured",
      "The scan service is not configured correctly for this site.",
    );
  }

  const fetchImpl = options.fetchImpl ?? fetch;
  const timeout = AbortSignal.timeout(config.timeoutMs);
  const signal = options.signal
    ? AbortSignal.any([timeout, options.signal])
    : timeout;

  let response: Response;
  try {
    response = await fetchImpl(`${config.baseUrl}${options.path}`, {
      method: options.method,
      headers: {
        Accept: "application/json",
        ...(options.body === undefined
          ? {}
          : { "Content-Type": "application/json" }),
      },
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      signal,
      cache: "no-store",
      redirect: "error",
    });
  } catch (error) {
    if (options.signal?.aborted) {
      return errorResult(
        499,
        "request_cancelled",
        "The request was cancelled before a result was available.",
      );
    }
    if (isTimeout(error, options.signal)) {
      return errorResult(
        504,
        "scan_timeout",
        "The scan did not finish before the request deadline.",
      );
    }
    return errorResult(
      502,
      "upstream_unavailable",
      "The scan service could not be reached. Check that the API is running.",
    );
  }

  let text: string;
  try {
    text = await readStreamCapped(response.body, config.maxResponseBytes);
  } catch (error) {
    if (error instanceof BodyLimitError) {
      return errorResult(
        502,
        "upstream_response_too_large",
        "The scan service returned a response larger than this site accepts.",
      );
    }
    if (error instanceof InvalidJsonError) {
      return errorResult(
        502,
        "contract_violation",
        "The scan service returned a report that does not match the v1 contract.",
      );
    }
    if (options.signal?.aborted) {
      return errorResult(
        499,
        "request_cancelled",
        "The request was cancelled before a result was available.",
      );
    }
    if (isTimeout(error, options.signal)) {
      return errorResult(
        504,
        "scan_timeout",
        "The scan did not finish before the request deadline.",
      );
    }
    return errorResult(
      502,
      "upstream_error",
      "The scan service response could not be read.",
    );
  }

  if (!response.ok) {
    let envelope: ApiError | null = null;
    try {
      const parsed: unknown = JSON.parse(text);
      if (isPlainObject(parsed)) {
        envelope = sanitizeError(parsed.error);
      }
    } catch {
      envelope = null;
    }
    if (envelope) {
      return errorResult(
        response.status,
        envelope.code,
        envelope.message,
        sanitizeRetryAfter(response.headers.get("retry-after")),
      );
    }
    return errorResult(
      response.status,
      "upstream_error",
      "The scan service rejected the request.",
      sanitizeRetryAfter(response.headers.get("retry-after")),
    );
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return errorResult(
      502,
      "contract_violation",
      "The scan service returned a report that does not match the v1 contract.",
    );
  }

  try {
    return { ok: true, value: options.parse(parsed) };
  } catch (error) {
    if (error instanceof ReportContractError) {
      return errorResult(
        502,
        "contract_violation",
        "The scan service returned a report that does not match the v1 contract.",
      );
    }
    return errorResult(
      502,
      "contract_violation",
      "The scan service returned a response that could not be interpreted.",
    );
  }
}

/** 调用共享 API 的 scan 端点；成功时返回已校验的 live 报告。 */
export function requestScan(
  input: PackageInput,
  options: ApiClientOptions = {},
): Promise<ApiResult<ScanReport>> {
  return callApi({
    ...options,
    method: "POST",
    path: "/api/v1/scan",
    body: input,
    parse: (value) => requireLiveReport(parseScanReport(value)),
  });
}

/** 调用共享 API 的 compare 端点；两侧报告分别通过契约校验。 */
export function requestCompare(
  input: { left: PackageInput; right: PackageInput },
  options: ApiClientOptions = {},
): Promise<ApiResult<CompareResponse>> {
  return callApi({
    ...options,
    method: "POST",
    path: "/api/v1/compare",
    body: input,
    parse: parseCompareResponse,
  });
}

/** 顶部状态条使用；失败只返回 false，不暴露上游地址或细节。 */
export async function checkApiHealth(
  options: ApiClientOptions = {},
): Promise<boolean> {
  const config = resolveApiConfig(options.env);
  if (!config) {
    return false;
  }
  const fetchImpl = options.fetchImpl ?? fetch;
  const timeout = AbortSignal.timeout(Math.min(config.timeoutMs, 4_000));
  try {
    const response = await fetchImpl(`${config.baseUrl}/healthz`, {
      method: "GET",
      headers: { Accept: "application/json" },
      signal: options.signal
        ? AbortSignal.any([timeout, options.signal])
        : timeout,
      cache: "no-store",
      redirect: "error",
    });
    if (!response.ok) {
      return false;
    }
    const text = await readStreamCapped(response.body, 16 * 1024);
    const parsed: unknown = JSON.parse(text);
    return isPlainObject(parsed) && parsed.status === "ok";
  } catch {
    return false;
  }
}
