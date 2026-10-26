import { describe, expect, it } from "vitest";

import { safeHttpUrl } from "@/lib/safe-url";

describe("safeHttpUrl", () => {
  it("accepts http and https URLs", () => {
    expect(safeHttpUrl("https://osv.dev/vulnerability/OSV-1")).toBe(
      "https://osv.dev/vulnerability/OSV-1",
    );
    expect(safeHttpUrl("http://example.com/path?q=1")).toBe(
      "http://example.com/path?q=1",
    );
  });

  it("rejects non-http protocols", () => {
    for (const value of [
      "javascript:alert(1)",
      "data:text/html,<script>alert(1)</script>",
      "file:///etc/passwd",
      "mailto:security@example.com",
    ]) {
      expect(safeHttpUrl(value)).toBeNull();
    }
  });

  it("rejects URLs with credentials", () => {
    expect(safeHttpUrl("https://user:pass@example.com/")).toBeNull();
    expect(safeHttpUrl("https://token@example.com/")).toBeNull();
  });

  it("rejects empty and malformed values", () => {
    expect(safeHttpUrl(null)).toBeNull();
    expect(safeHttpUrl(undefined)).toBeNull();
    expect(safeHttpUrl("")).toBeNull();
    expect(safeHttpUrl("not a url")).toBeNull();
    expect(safeHttpUrl("https://")).toBeNull();
  });
});
