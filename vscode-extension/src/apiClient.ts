import { Ecosystem, ScanReport } from "./generated/report-types";

/**
 * 请求单包扫描的入参结构。
 */
export interface ScanPackageRequest {
  ecosystem: Ecosystem;
  name: string;
  version?: string;
}

/**
 * API 失败时返回的统一错误信封结构。
 */
export interface ApiErrorResponse {
  error: {
    code: string;
    message: string;
  };
}

/**
 * 自定义 API 错误类，携带错误码与 HTTP 状态。
 */
export class DepScopeApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number
  ) {
    super(`[${code}] ${message}`);
    this.name = "DepScopeApiError";
  }
}

/**
 * 调用本地 DepScope HTTP API 的 /api/v1/scan 接口执行包扫描。
 *
 * @param baseUrl 本地 API 服务根地址，例如 http://127.0.0.1:54321
 * @param request 扫描目标包信息（生态、名称、可选版本）
 * @param timeoutMs 超时时间（毫秒），默认为 60000ms
 * @returns 规范化的 ScanReport v1 对象
 */
export async function scanPackage(
  baseUrl: string,
  request: ScanPackageRequest,
  timeoutMs = 60000
): Promise<ScanReport> {
  const url = `${baseUrl.replace(/\/+$/, "")}/api/v1/scan`;

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        ecosystem: request.ecosystem,
        name: request.name,
        version: request.version || undefined,
      }),
      signal: controller.signal,
    });

    if (!response.ok) {
      let code = "request_failed";
      let message = `API request failed with status ${response.status}`;
      try {
        const errorJson = (await response.json()) as ApiErrorResponse;
        if (errorJson && errorJson.error) {
          code = errorJson.error.code || code;
          message = errorJson.error.message || message;
        }
      } catch {
        // 响应非 JSON 时使用默认错误信息
      }
      throw new DepScopeApiError(code, message, response.status);
    }

    const report = (await response.json()) as ScanReport;
    return report;
  } catch (err: unknown) {
    if (err instanceof DepScopeApiError) {
      throw err;
    }
    if (err instanceof Error && err.name === "AbortError") {
      throw new DepScopeApiError("timeout", `Scan request for ${request.name} timed out`, 504);
    }
    const message = err instanceof Error ? err.message : String(err);
    throw new DepScopeApiError("network_error", message, 0);
  } finally {
    clearTimeout(timer);
  }
}

/**
 * 探测 API 服务健康状态。
 *
 * @param baseUrl 本地 API 服务根地址
 * @param timeoutMs 超时时间（毫秒），默认为 3000ms
 * @returns true 表示服务健康正常，false 表示异常
 */
export async function checkHealth(baseUrl: string, timeoutMs = 3000): Promise<boolean> {
  const url = `${baseUrl.replace(/\/+$/, "")}/healthz`;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const response = await fetch(url, {
      method: "GET",
      signal: controller.signal,
    });
    if (!response.ok) {
      return false;
    }
    const data = (await response.json()) as { status?: string };
    return data.status === "ok";
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}
