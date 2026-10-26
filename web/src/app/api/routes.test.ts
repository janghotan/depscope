import { beforeEach, describe, expect, it, vi } from "vitest";

import { liveFindingsReport } from "@/test/fixtures";

const mocks = vi.hoisted(() => ({
  requestScan: vi.fn(),
  requestCompare: vi.fn(),
  checkApiHealth: vi.fn(),
}));

vi.mock("@/lib/api-client", () => ({
  requestScan: mocks.requestScan,
  requestCompare: mocks.requestCompare,
  checkApiHealth: mocks.checkApiHealth,
}));

import { POST as scanPost } from "@/app/api/scan/route";
import { POST as comparePost } from "@/app/api/compare/route";
import { GET as healthGet } from "@/app/api/health/route";

const SCAN_URL = "http://localhost/api/scan";
const COMPARE_URL = "http://localhost/api/compare";

function post(url: string, body: string, contentType = "application/json") {
  return new Request(url, {
    method: "POST",
    headers: { "content-type": contentType },
    body,
  });
}

async function readError(response: Response) {
  return (await response.json()) as {
    error: { code: string; message: string };
  };
}

beforeEach(() => {
  mocks.requestScan.mockReset();
  mocks.requestCompare.mockReset();
  mocks.checkApiHealth.mockReset();
});

describe("POST /api/scan", () => {
  it("rejects query parameters before reading the body", async () => {
    const response = await scanPost(
      post(`${SCAN_URL}?name=react`, '{"ecosystem":"npm","name":"react"}'),
    );
    expect(response.status).toBe(400);
    expect((await readError(response)).error.code).toBe("invalid_request");
    expect(mocks.requestScan).not.toHaveBeenCalled();
  });

  it("requires a JSON content type", async () => {
    const response = await scanPost(
      post(SCAN_URL, '{"ecosystem":"npm","name":"react"}', "text/plain"),
    );
    expect(response.status).toBe(415);
    expect((await readError(response)).error.code).toBe("unsupported_media_type");
  });

  it("rejects invalid JSON and unknown fields", async () => {
    const invalid = await scanPost(post(SCAN_URL, "{"));
    expect(invalid.status).toBe(400);
    expect((await readError(invalid)).error.code).toBe("invalid_json");

    const unknown = await scanPost(
      post(SCAN_URL, '{"ecosystem":"npm","name":"react","extra":true}'),
    );
    expect(unknown.status).toBe(400);
    expect((await readError(unknown)).error.code).toBe("unknown_field");
  });

  it("rejects a body above 16 KiB", async () => {
    const response = await scanPost(
      post(
        SCAN_URL,
        JSON.stringify({
          ecosystem: "npm",
          name: "react",
          padding: "a".repeat(17 * 1024),
        }),
      ),
    );
    expect(response.status).toBe(413);
    expect((await readError(response)).error.code).toBe("request_too_large");
  });

  it("passes the normalized request to the API client", async () => {
    mocks.requestScan.mockResolvedValue({
      ok: true,
      value: liveFindingsReport(),
    });
    const response = await scanPost(
      post(SCAN_URL, '{"ecosystem":"npm","name":"React","version":"19.3.1"}'),
    );

    expect(response.status).toBe(200);
    expect(mocks.requestScan).toHaveBeenCalledWith(
      { ecosystem: "npm", name: "react", version: "19.3.1" },
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    );
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(response.headers.get("x-content-type-options")).toBe("nosniff");
    expect(await response.json()).toMatchObject({
      schemaVersion: "1.0.0",
      dataMode: "live",
    });
  });

  it("maps a safe upstream error and Retry-After", async () => {
    mocks.requestScan.mockResolvedValue({
      ok: false,
      error: {
        status: 429,
        code: "rate_limited",
        message: "Try again later.",
        retryAfter: "2",
      },
    });
    const response = await scanPost(
      post(SCAN_URL, '{"ecosystem":"npm","name":"react"}'),
    );
    expect(response.status).toBe(429);
    expect(response.headers.get("retry-after")).toBe("2");
    expect(await readError(response)).toEqual({
      error: { code: "rate_limited", message: "Try again later." },
    });
  });
});

describe("POST /api/compare", () => {
  const body = JSON.stringify({
    left: { ecosystem: "npm", name: "left-package" },
    right: { ecosystem: "pypi", name: "right-package" },
  });

  it("rejects malformed sides", async () => {
    const response = await comparePost(
      post(COMPARE_URL, '{"left":{"ecosystem":"npm","name":"left"}}'),
    );
    expect(response.status).toBe(400);
    expect((await readError(response)).error.message).toMatch(/^right: /);
    expect(mocks.requestCompare).not.toHaveBeenCalled();
  });

  it("returns the comparison envelope", async () => {
    const envelope = {
      left: {
        request: { ecosystem: "npm", name: "left-package" },
        report: liveFindingsReport(),
      },
      right: {
        request: { ecosystem: "pypi", name: "right-package" },
        error: { code: "not_found", message: "Package not found." },
      },
    };
    mocks.requestCompare.mockResolvedValue({ ok: true, value: envelope });
    const response = await comparePost(post(COMPARE_URL, body));
    expect(response.status).toBe(200);
    expect(await response.json()).toEqual(envelope);
  });

  it("rejects query parameters", async () => {
    const response = await comparePost(post(`${COMPARE_URL}?left=x`, body));
    expect(response.status).toBe(400);
    expect(mocks.requestCompare).not.toHaveBeenCalled();
  });
});

describe("GET /api/health", () => {
  it("returns 200 only when the API is healthy", async () => {
    mocks.checkApiHealth.mockResolvedValue(true);
    const healthy = await healthGet(new Request("http://localhost/api/health"));
    expect(healthy.status).toBe(200);
    expect(await healthy.json()).toEqual({ status: "ok" });

    mocks.checkApiHealth.mockResolvedValue(false);
    const unavailable = await healthGet(
      new Request("http://localhost/api/health"),
    );
    expect(unavailable.status).toBe(503);
    expect(await unavailable.json()).toEqual({ status: "unavailable" });
  });
});
