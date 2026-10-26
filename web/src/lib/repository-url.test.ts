import { describe, expect, it } from "vitest";

import { repositoryUrl } from "@/lib/repository-url";

describe("repositoryUrl", () => {
  it("accepts a public HTTPS repository URL", () => {
    expect(repositoryUrl("https://github.com/example/depscope.git")).toBe(
      "https://github.com/example/depscope.git",
    );
  });

  it.each([
    "http://example.test/repository",
    "javascript:alert(1)",
    "https://user:password@example.test/repository",
    "https://example.test/repository?token=secret",
    "https://example.test/repository#private",
  ])("rejects unsafe or non-public URL %s", (value) => {
    expect(repositoryUrl(value)).toBeNull();
  });

  it("returns null when no repository is configured", () => {
    expect(repositoryUrl(undefined)).toBeNull();
  });
});
