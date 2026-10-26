import type { CompareInput, PackageInput } from "@/lib/api-types";

const MAX_PACKAGE_NAME_LENGTH = 214;
const MAX_VERSION_LENGTH = 256;

const NPM_SEGMENT = /^[a-z0-9][a-z0-9._~-]*$/;
const PYPI_NAME = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;
const VERSION = /^[A-Za-z0-9][A-Za-z0-9._+-]*$/;
const SCAN_FIELDS = new Set(["ecosystem", "name", "version"]);

export type ValidationIssue = {
  code: "invalid_json" | "unknown_field" | "invalid_request";
  message: string;
  side?: "left" | "right";
};

export type ValidationResult<T> =
  | { ok: true; value: T }
  | { ok: false; issue: ValidationIssue };

function fail(message: string, side?: "left" | "right"): ValidationResult<never> {
  return {
    ok: false,
    issue: {
      code: "invalid_request",
      message: side ? `${side}: ${message}` : message,
      ...(side ? { side } : {}),
    },
  };
}

export function isPlainObject(
  value: unknown,
): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** 与共享 Go 模型保持一致的规范化；返回 null 表示名称无效。 */
export function canonicalizeName(
  ecosystem: PackageInput["ecosystem"],
  name: string,
): string | null {
  if (ecosystem === "npm") {
    const canonical = name.toLowerCase();
    if (canonical.startsWith("@")) {
      const parts = canonical.split("/");
      if (parts.length !== 2) {
        return null;
      }
      const scope = parts[0].slice(1);
      const packageName = parts[1];
      if (!scope || !packageName) {
        return null;
      }
      if (!NPM_SEGMENT.test(scope) || !NPM_SEGMENT.test(packageName)) {
        return null;
      }
      return canonical;
    }
    if (canonical.includes("/") || !NPM_SEGMENT.test(canonical)) {
      return null;
    }
    return canonical;
  }

  const canonical = name.toLowerCase().replace(/[-_.]+/g, "-");
  return PYPI_NAME.test(canonical) ? canonical : null;
}

function parsePackageFields(
  value: Record<string, unknown>,
  side?: "left" | "right",
): ValidationResult<PackageInput> {
  for (const key of Object.keys(value)) {
    if (!SCAN_FIELDS.has(key)) {
      return {
        ok: false,
        issue: {
          code: "unknown_field",
          message: side
            ? `${side}: request contains an unknown field`
            : "request contains an unknown field",
          ...(side ? { side } : {}),
        },
      };
    }
  }

  const ecosystem = value.ecosystem;
  if (ecosystem !== "npm" && ecosystem !== "pypi") {
    return fail("ecosystem must be one of: npm, pypi", side);
  }

  const name = value.name;
  if (typeof name !== "string") {
    return fail("name must be a string", side);
  }
  if (name.trim() === "") {
    return fail("name must not be empty", side);
  }
  if (name.trim() !== name) {
    return fail("name must not contain leading or trailing whitespace", side);
  }
  if ([...name].length > MAX_PACKAGE_NAME_LENGTH) {
    return fail(
      `name must be at most ${MAX_PACKAGE_NAME_LENGTH} characters`,
      side,
    );
  }

  let version: string | undefined;
  if (value.version !== undefined) {
    if (typeof value.version !== "string") {
      return fail("version must be a string", side);
    }
    if (value.version === "") {
      return fail("version must not be empty when provided", side);
    }
    if ([...value.version].length > MAX_VERSION_LENGTH) {
      return fail(
        `version must be at most ${MAX_VERSION_LENGTH} characters`,
        side,
      );
    }
    if (!VERSION.test(value.version)) {
      return fail(
        "version must be a single version identifier without URLs, paths, or whitespace",
        side,
      );
    }
    version = value.version;
  }

  const canonical = canonicalizeName(ecosystem, name);
  if (canonical === null) {
    return fail(`name is not a valid ${ecosystem} package name`, side);
  }

  return {
    ok: true,
    value: {
      ecosystem,
      name: canonical,
      ...(version === undefined ? {} : { version }),
    },
  };
}

/** 严格解析单包请求：只接受 ecosystem、name 与可选 version。 */
export function parsePackageInput(value: unknown): ValidationResult<PackageInput> {
  if (!isPlainObject(value)) {
    return {
      ok: false,
      issue: {
        code: "invalid_json",
        message: "request body must be a JSON object",
      },
    };
  }
  return parsePackageFields(value);
}

/** 严格解析比较请求：顶层只接受 left 与 right。 */
export function parseCompareInput(value: unknown): ValidationResult<CompareInput> {
  if (!isPlainObject(value)) {
    return {
      ok: false,
      issue: {
        code: "invalid_json",
        message: "request body must be a JSON object",
      },
    };
  }
  for (const key of Object.keys(value)) {
    if (key !== "left" && key !== "right") {
      return {
        ok: false,
        issue: {
          code: "unknown_field",
          message: "request contains an unknown field",
        },
      };
    }
  }

  for (const side of ["left", "right"] as const) {
    const sideValue = value[side];
    if (!isPlainObject(sideValue)) {
      return fail(`${side} must be a JSON object`, side);
    }
  }

  const left = parsePackageFields(value.left as Record<string, unknown>, "left");
  if (!left.ok) {
    return left;
  }
  const right = parsePackageFields(
    value.right as Record<string, unknown>,
    "right",
  );
  if (!right.ok) {
    return right;
  }
  return { ok: true, value: { left: left.value, right: right.value } };
}

export function packageInputFromParams(params: {
  ecosystem?: string | string[];
  name?: string | string[];
  version?: string | string[];
}): ValidationResult<PackageInput> | null {
  const ecosystem = singleParam(params.ecosystem);
  const name = singleParam(params.name);
  if (ecosystem === undefined && name === undefined) {
    return null;
  }
  const version = singleParam(params.version);
  return parsePackageInput({
    ecosystem,
    name,
    ...(version === undefined ? {} : { version }),
  });
}

export function singleParam(
  value: string | string[] | undefined,
): string | undefined {
  return typeof value === "string" ? value : undefined;
}
