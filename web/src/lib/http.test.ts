import { describe, expect, it } from "vitest";

import {
  BodyLimitError,
  InvalidJsonError,
  hasJsonContentType,
  jsonError,
  jsonResponse,
  parseJsonObject,
  readStreamCapped,
} from "@/lib/http";

describe("readStreamCapped", () => {
  it("returns an empty string for a missing body", async () => {
    expect(await readStreamCapped(null, 10)).toBe("");
  });

  it("reads a body at the limit", async () => {
    const response = new Response("abcdef");
    expect(await readStreamCapped(response.body, 6)).toBe("abcdef");
  });

  it("rejects a body over the limit", async () => {
    const response = new Response("abcdef");
    await expect(readStreamCapped(response.body, 5)).rejects.toBeInstanceOf(
      BodyLimitError,
    );
  });

  it("rejects invalid UTF-8", async () => {
    const response = new Response(new Uint8Array([0xff, 0xfe]));
    await expect(readStreamCapped(response.body, 10)).rejects.toBeInstanceOf(
      InvalidJsonError,
    );
  });
});

describe("parseJsonObject", () => {
  it("parses a single object", () => {
    expect(parseJsonObject('{"name":"react"}')).toEqual({ name: "react" });
  });

  it("rejects empty, invalid, array, and scalar bodies", () => {
    for (const body of ["", "   ", "{", "[]", "null", '"text"', "1"]) {
      expect(() => parseJsonObject(body)).toThrow(InvalidJsonError);
    }
  });

  it("rejects trailing content", () => {
    expect(() => parseJsonObject('{"a":1} {"b":2}')).toThrow(InvalidJsonError);
  });
});

describe("hasJsonContentType", () => {
  it("accepts application/json with parameters", () => {
    const request = new Request("http://localhost/api/scan", {
      method: "POST",
      headers: { "content-type": "Application/JSON; charset=utf-8" },
    });
    expect(hasJsonContentType(request)).toBe(true);
  });

  it("rejects missing and non-JSON content types", () => {
    expect(
      hasJsonContentType(new Request("http://localhost/api/scan")),
    ).toBe(false);
    expect(
      hasJsonContentType(
        new Request("http://localhost/api/scan", {
          headers: { "content-type": "text/plain" },
        }),
      ),
    ).toBe(false);
  });
});

describe("JSON responses", () => {
  it("adds no-store and nosniff headers", async () => {
    const response = jsonResponse({ ok: true });
    expect(response.status).toBe(200);
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(response.headers.get("x-content-type-options")).toBe("nosniff");
    expect(response.headers.get("content-type")).toBe(
      "application/json; charset=utf-8",
    );
    await expect(response.json()).resolves.toEqual({ ok: true });
  });

  it("wraps errors in the documented envelope", async () => {
    const response = jsonError(429, "rate_limited", "Slow down.", {
      "Retry-After": "3",
    });
    expect(response.status).toBe(429);
    expect(response.headers.get("retry-after")).toBe("3");
    await expect(response.json()).resolves.toEqual({
      error: { code: "rate_limited", message: "Slow down." },
    });
  });
});
