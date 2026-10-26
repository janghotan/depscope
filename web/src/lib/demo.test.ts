import { describe, expect, it } from "vitest";

import {
  buildDemoComparison,
  getDemoScenario,
  isDemoScenarioKey,
  listDemoScenarios,
  requireDemoScenario,
} from "@/lib/demo";

describe("demo scenarios", () => {
  it("lists the three deterministic scenarios in a fixed order", () => {
    expect(listDemoScenarios().map((scenario) => scenario.key)).toEqual([
      "findings",
      "clean",
      "incomplete",
    ]);
  });

  it("validates scenario keys", () => {
    expect(isDemoScenarioKey("clean")).toBe(true);
    expect(isDemoScenarioKey("../clean")).toBe(false);
    expect(getDemoScenario("missing")).toBeNull();
    expect(() => requireDemoScenario("missing" as "clean")).toThrow(
      /unavailable/,
    );
  });

  it("keeps every fixture unmistakably synthetic", () => {
    for (const scenario of listDemoScenarios()) {
      expect(scenario.report.dataMode).toBe("demo");
      expect(scenario.report.demoNotice).toMatch(/^DEMO DATA/);
      expect(scenario.report.scannedAt).toBe("2026-01-15T12:00:00Z");
      expect(
        scenario.report.sources.every((source) => source.provider === "demo"),
      ).toBe(true);
    }
  });

  it("builds the documented comparison envelope without a network request", () => {
    const response = buildDemoComparison("findings", "clean");
    expect(response.left.request).toEqual({
      ecosystem: "npm",
      name: "depscope-demo-vulnerable-package",
      version: "2.0.0",
    });
    expect(response.left.report?.status).toBe("findings");
    expect(response.right.request).toEqual({
      ecosystem: "npm",
      name: "@depscope-demo/clean-package",
      version: "1.0.0",
    });
    expect(response.right.report?.status).toBe("clean");
    expect("error" in response.left).toBe(false);
    expect("error" in response.right).toBe(false);
  });

  it("rejects an unknown side", () => {
    expect(() => buildDemoComparison("clean", "missing" as "clean")).toThrow(
      /Unknown demo scenario/,
    );
  });
});
