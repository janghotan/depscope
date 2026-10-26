import { describe, expect, it } from "vitest";

import {
  canonicalizeName,
  packageInputFromParams,
  parseCompareInput,
  parsePackageInput,
} from "@/lib/request";

describe("parsePackageInput", () => {
  it("normalizes npm names to lowercase", () => {
    const result = parsePackageInput({
      ecosystem: "npm",
      name: "React",
      version: "19.3.1",
    });
    expect(result).toEqual({
      ok: true,
      value: { ecosystem: "npm", name: "react", version: "19.3.1" },
    });
  });

  it("accepts a scoped npm name with exactly one slash", () => {
    expect(
      parsePackageInput({ ecosystem: "npm", name: "@Types/Node" }),
    ).toEqual({
      ok: true,
      value: { ecosystem: "npm", name: "@types/node" },
    });
  });

  it("rejects malformed scoped npm names", () => {
    for (const name of ["@scope", "@scope/", "@/name", "@a/b/c", "a/b"]) {
      expect(parsePackageInput({ ecosystem: "npm", name })).toMatchObject({
        ok: false,
        issue: { code: "invalid_request" },
      });
    }
  });

  it("normalizes PyPI names by lowercasing and collapsing separators", () => {
    expect(
      parsePackageInput({ ecosystem: "pypi", name: "Zope.Interface" }),
    ).toEqual({
      ok: true,
      value: { ecosystem: "pypi", name: "zope-interface" },
    });
    expect(canonicalizeName("pypi", "a__b..c--d")).toBe("a-b-c-d");
  });

  it("rejects an empty or whitespace-padded name", () => {
    expect(parsePackageInput({ ecosystem: "npm", name: "" })).toMatchObject({
      ok: false,
      issue: { code: "invalid_request" },
    });
    expect(
      parsePackageInput({ ecosystem: "npm", name: " lodash" }),
    ).toMatchObject({ ok: false });
  });

  it("rejects range operators, whitespace, and URL-like versions", () => {
    for (const version of ["^1.2.3", "~1.2.3", ">=1.2.3", "1.0.0 beta", "https://x"]) {
      expect(
        parsePackageInput({ ecosystem: "npm", name: "lodash", version }),
      ).toMatchObject({
        ok: false,
        issue: { code: "invalid_request" },
      });
    }
  });

  it("keeps a bare identifier verbatim without treating it as a settled version", () => {
    // 与 Go 端一致：语法只要求单一标识符；是否为已发布版本由解析阶段判断。
    expect(
      parsePackageInput({ ecosystem: "npm", name: "lodash", version: "latest" }),
    ).toEqual({
      ok: true,
      value: { ecosystem: "npm", name: "lodash", version: "latest" },
    });
  });

  it("rejects an empty version when the field is present", () => {
    expect(
      parsePackageInput({ ecosystem: "npm", name: "lodash", version: "" }),
    ).toMatchObject({ ok: false });
  });

  it("rejects unsupported ecosystems and unknown fields", () => {
    expect(parsePackageInput({ ecosystem: "maven", name: "x" })).toMatchObject({
      ok: false,
    });
    expect(
      parsePackageInput({ ecosystem: "npm", name: "x", extra: true }),
    ).toMatchObject({
      ok: false,
      issue: { code: "unknown_field" },
    });
  });

  it("rejects non-object input", () => {
    expect(parsePackageInput(null)).toMatchObject({
      ok: false,
      issue: { code: "invalid_json" },
    });
    expect(parsePackageInput([])).toMatchObject({
      ok: false,
      issue: { code: "invalid_json" },
    });
  });

  it("enforces the package name code-point limit", () => {
    const name = "a".repeat(215);
    expect(parsePackageInput({ ecosystem: "npm", name })).toMatchObject({
      ok: false,
    });
    expect(
      parsePackageInput({ ecosystem: "npm", name: "a".repeat(214) }),
    ).toMatchObject({ ok: true });
  });
});

describe("parseCompareInput", () => {
  it("parses both sides independently", () => {
    const result = parseCompareInput({
      left: { ecosystem: "npm", name: "React" },
      right: { ecosystem: "pypi", name: "Requests" },
    });
    expect(result).toEqual({
      ok: true,
      value: {
        left: { ecosystem: "npm", name: "react" },
        right: { ecosystem: "pypi", name: "requests" },
      },
    });
  });

  it("prefixes side failures", () => {
    const result = parseCompareInput({
      left: { ecosystem: "npm", name: "left" },
      right: { ecosystem: "npm", name: "^bad" },
    });
    expect(result).toMatchObject({
      ok: false,
      issue: { code: "invalid_request", side: "right" },
    });
    if (!result.ok) {
      expect(result.issue.message).toMatch(/^right: /);
    }
  });

  it("rejects unknown top-level and side fields", () => {
    expect(
      parseCompareInput({
        left: { ecosystem: "npm", name: "left" },
        right: { ecosystem: "npm", name: "right" },
        extra: true,
      }),
    ).toMatchObject({ ok: false, issue: { code: "unknown_field" } });
    expect(
      parseCompareInput({
        left: { ecosystem: "npm", name: "left", extra: true },
        right: { ecosystem: "npm", name: "right" },
      }),
    ).toMatchObject({ ok: false, issue: { code: "unknown_field", side: "left" } });
  });

  it("rejects a missing side", () => {
    expect(
      parseCompareInput({ left: { ecosystem: "npm", name: "left" } }),
    ).toMatchObject({ ok: false, issue: { side: "right" } });
  });
});

describe("packageInputFromParams", () => {
  it("returns null when no package parameter is present", () => {
    expect(packageInputFromParams({})).toBeNull();
    expect(packageInputFromParams({ version: "1.0.0" })).toBeNull();
  });

  it("parses and normalizes search parameters", () => {
    expect(
      packageInputFromParams({
        ecosystem: "npm",
        name: "React",
        version: "19.0.0",
      }),
    ).toEqual({
      ok: true,
      value: { ecosystem: "npm", name: "react", version: "19.0.0" },
    });
  });

  it("fails when only one required field is present", () => {
    expect(packageInputFromParams({ ecosystem: "npm" })).toMatchObject({
      ok: false,
    });
    expect(packageInputFromParams({ name: "react" })).toMatchObject({
      ok: false,
    });
  });
});
