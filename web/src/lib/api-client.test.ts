import { describe, expect, it, vi } from "vitest";

import {
  checkApiHealth,
  requestCompare,
  requestScan,
  resolveApiConfig,
} from "@/lib/api-client";
import { liveFindingsReport, demoFindingsReport } from "@/test/fixtures";

const ENV = { DEPSCOPE_API_BASE_URL: "http://api.test:8080" };

function jsonResponse(
  value: unknown,
  status = 200,
  headers: Record<string, string> = {},
): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
}

function fetchReturning(...responses: Response[]) {
  const mock = vi.fn();
  for (const response of responses) {
    mock.mockResolvedValueOnce(response);
  }
  return mock as unknown as typeof fetch;
}

const LIVE_REQUEST = { ecosystem: "npm" as const, name: "react" };

describe("resolveApiConfig", () => {
  it("uses the documented defaults", () => {
    expect(resolveApiConfig({})).toEqual({
      baseUrl: "http://127.0.0.1:8080",
      timeoutMs: 100_000,
      maxResponseBytes: 8 * 1024 * 1024,
    });
  });

  it("accepts a configured origin", () => {
    expect(
      resolveApiConfig({
        DEPSCOPE_API_BASE_URL: "https://api.example.test",
        DEPSCOPE_API_TIMEOUT_MS: "5000",
      }),
    ).toEqual({
      baseUrl: "https://api.example.test",
      timeoutMs: 5000,
      maxResponseBytes: 8 * 1024 * 1024,
    });
  });

  it("rejects unsupported or ambiguous configuration", () => {
    for (const env of [
      { DEPSCOPE_API_BASE_URL: "ftp://api.test" },
      { DEPSCOPE_API_BASE_URL: "http://user:pass@api.test" },
      { DEPSCOPE_API_BASE_URL: "http://api.test/path" },
      { DEPSCOPE_API_BASE_URL: "http://api.test?x=1" },
      { DEPSCOPE_API_BASE_URL: "not a url" },
      { DEPSCOPE_API_TIMEOUT_MS: "999" },
      { DEPSCOPE_API_TIMEOUT_MS: "120001" },
      { DEPSCOPE_API_TIMEOUT_MS: "soon" },
    ]) {
      expect(resolveApiConfig(env)).toBeNull();
    }
  });
});

describe("requestScan", () => {
  it("sends the normalized request and returns a validated live report", async () => {
    const live = liveFindingsReport();
    const fetchImpl = fetchReturning(jsonResponse(live));

    const result = await requestScan(LIVE_REQUEST, {
      fetchImpl,
      env: ENV,
    });

    expect(result).toEqual({ ok: true, value: live });
    expect(fetchImpl).toHaveBeenCalledTimes(1);
    const [url, init] = (fetchImpl as unknown as ReturnType<typeof vi.fn>).mock
      .calls[0];
    expect(url).toBe("http://api.test:8080/api/v1/scan");
    expect(init).toMatchObject({
      method: "POST",
      body: JSON.stringify(LIVE_REQUEST),
      cache: "no-store",
      redirect: "error",
    });
  });

  it("rejects a demo report returned by the live API", async () => {
    const fetchImpl = fetchReturning(jsonResponse(demoFindingsReport()));
    const result = await requestScan(LIVE_REQUEST, { fetchImpl, env: ENV });
    expect(result).toMatchObject({
      ok: false,
      error: { code: "contract_violation", status: 502 },
    });
  });

  it("returns the safe upstream envelope and Retry-After", async () => {
    const fetchImpl = fetchReturning(
      jsonResponse(
        { error: { code: "rate_limited", message: "Try again later." } },
        429,
        { "retry-after": "3" },
      ),
    );
    const result = await requestScan(LIVE_REQUEST, { fetchImpl, env: ENV });
    expect(result).toEqual({
      ok: false,
      error: {
        status: 429,
        code: "rate_limited",
        message: "Try again later.",
        retryAfter: "3",
      },
    });
  });

  it("does not relay an unsafe upstream error message", async () => {
    const fetchImpl = fetchReturning(
      jsonResponse(
        {
          error: {
            code: "Bad Code",
            message: "line one\nline two",
          },
        },
        500,
        { "retry-after": "tomorrow" },
      ),
    );
    const result = await requestScan(LIVE_REQUEST, { fetchImpl, env: ENV });
    expect(result).toEqual({
      ok: false,
      error: {
        status: 500,
        code: "upstream_error",
        message: "The scan service rejected the request.",
      },
    });
  });

  it("maps a timeout to scan_timeout", async () => {
    const fetchImpl = vi.fn(async () => {
      const error = new Error("timed out");
      error.name = "TimeoutError";
      throw error;
    }) as unknown as typeof fetch;

    const result = await requestScan(LIVE_REQUEST, { fetchImpl, env: ENV });
    expect(result).toMatchObject({
      ok: false,
      error: { status: 504, code: "scan_timeout" },
    });
  });

  it("maps caller cancellation to request_cancelled", async () => {
    const controller = new AbortController();
    controller.abort();
    const fetchImpl = vi.fn(async () => {
      throw new DOMException("Aborted", "AbortError");
    }) as unknown as typeof fetch;

    const result = await requestScan(LIVE_REQUEST, {
      fetchImpl,
      env: ENV,
      signal: controller.signal,
    });
    expect(result).toMatchObject({
      ok: false,
      error: { status: 499, code: "request_cancelled" },
    });
  });

  it("reports misconfiguration without attempting a fetch", async () => {
    const fetchImpl = vi.fn() as unknown as typeof fetch;
    const result = await requestScan(LIVE_REQUEST, {
      fetchImpl,
      env: { DEPSCOPE_API_BASE_URL: "ftp://api.test" },
    });
    expect(fetchImpl).not.toHaveBeenCalled();
    expect(result).toMatchObject({
      ok: false,
      error: { status: 500, code: "upstream_misconfigured" },
    });
  });
});

describe("requestCompare", () => {
  const request = {
    left: { ecosystem: "npm" as const, name: "left-package" },
    right: { ecosystem: "pypi" as const, name: "right-package" },
  };

  it("parses a side with a report and a side with an error", async () => {
    const response = {
      left: { request: request.left, report: liveFindingsReport() },
      right: {
        request: request.right,
        error: { code: "not_found", message: "Package not found." },
      },
    };
    const fetchImpl = fetchReturning(jsonResponse(response));
    const result = await requestCompare(request, { fetchImpl, env: ENV });

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.left.report?.dataMode).toBe("live");
      expect(result.value.right.error?.code).toBe("not_found");
    }
  });

  it("rejects a side containing both a report and an error", async () => {
    const response = {
      left: {
        request: request.left,
        report: liveFindingsReport(),
        error: { code: "not_found", message: "Package not found." },
      },
      right: {
        request: request.right,
        error: { code: "not_found", message: "Package not found." },
      },
    };
    const fetchImpl = fetchReturning(jsonResponse(response));
    const result = await requestCompare(request, { fetchImpl, env: ENV });
    expect(result).toMatchObject({
      ok: false,
      error: { code: "contract_violation" },
    });
  });

  it("rejects a demo report on a live comparison side", async () => {
    const response = {
      left: { request: request.left, report: demoFindingsReport() },
      right: {
        request: request.right,
        error: { code: "not_found", message: "Package not found." },
      },
    };
    const fetchImpl = fetchReturning(jsonResponse(response));
    const result = await requestCompare(request, { fetchImpl, env: ENV });
    expect(result).toMatchObject({
      ok: false,
      error: { code: "contract_violation" },
    });
  });

  it("rejects an unsafe side error", async () => {
    const response = {
      left: { request: request.left, report: liveFindingsReport() },
      right: {
        request: request.right,
        error: { code: "NOT_SAFE", message: "x" },
      },
    };
    const fetchImpl = fetchReturning(jsonResponse(response));
    const result = await requestCompare(request, { fetchImpl, env: ENV });
    expect(result).toMatchObject({
      ok: false,
      error: { code: "contract_violation" },
    });
  });
});

describe("checkApiHealth", () => {
  it("returns true only for status ok", async () => {
    await expect(
      checkApiHealth({
        fetchImpl: fetchReturning(jsonResponse({ status: "ok" })),
        env: ENV,
      }),
    ).resolves.toBe(true);
    await expect(
      checkApiHealth({
        fetchImpl: fetchReturning(jsonResponse({ status: "degraded" })),
        env: ENV,
      }),
    ).resolves.toBe(false);
    await expect(
      checkApiHealth({
        fetchImpl: fetchReturning(jsonResponse({ status: "ok" }, 503)),
        env: ENV,
      }),
    ).resolves.toBe(false);
  });

  it("returns false without leaking errors", async () => {
    const fetchImpl = vi.fn(async () => {
      throw new Error("ECONNREFUSED 127.0.0.1:8080");
    }) as unknown as typeof fetch;
    await expect(checkApiHealth({ fetchImpl, env: ENV })).resolves.toBe(false);
    await expect(
      checkApiHealth({
        fetchImpl,
        env: { DEPSCOPE_API_BASE_URL: "ftp://api.test" },
      }),
    ).resolves.toBe(false);
  });
});
