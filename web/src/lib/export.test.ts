import { describe, expect, it } from "vitest";

import { serializeReport } from "@/lib/export";
import { liveFindingsReport } from "@/test/fixtures";

describe("serializeReport", () => {
  it("exports the canonical report and a safe filename", () => {
    const report = liveFindingsReport();
    const output = serializeReport(report);

    expect(output.filename).toBe(
      "depscope-npm-depscope-demo-vulnerable-package-2.0.0.json",
    );
    expect(output.contentType).toBe("application/json;charset=utf-8");
    expect(JSON.parse(output.body)).toEqual(report);
    expect(output.body.endsWith("\n")).toBe(true);
  });
});
