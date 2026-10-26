import type { CompareResponse } from "@/lib/api-types";
import type { ScanReport } from "@/generated/report-types";
import { parseScanReport } from "@/lib/schema";

import cleanFixture from "@/generated/demo/demo-clean-report.json";
import findingsFixture from "@/generated/demo/demo-findings-report.json";
import incompleteFixture from "@/generated/demo/demo-incomplete-report.json";

export type DemoScenarioKey = "clean" | "findings" | "incomplete";

export type DemoScenario = {
  key: DemoScenarioKey;
  title: string;
  summary: string;
  demonstrates: string;
  report: ScanReport;
};

const scenarios: readonly DemoScenario[] = [
  {
    key: "findings",
    title: "Known advisory",
    summary:
      "One confirmed advisory on the root package and one heuristic warning on a dependency.",
    demonstrates:
      "How confirmed evidence, heuristic signals, and a traced load path read together.",
    report: parseScanReport(findingsFixture),
  },
  {
    key: "clean",
    title: "Clean release",
    summary: "All four checks succeeded and no findings were returned.",
    demonstrates:
      "A clean condition — and the checks that still have to succeed before it can be read that way.",
    report: parseScanReport(cleanFixture),
  },
  {
    key: "incomplete",
    title: "Coverage gap",
    summary:
      "Dependency resolution failed and the vulnerability lookup was unavailable.",
    demonstrates:
      "Why missing coverage is not safety: the report stays incomplete and names the gaps.",
    report: parseScanReport(incompleteFixture),
  },
];

export function listDemoScenarios(): readonly DemoScenario[] {
  return scenarios;
}

export function isDemoScenarioKey(value: string): value is DemoScenarioKey {
  return scenarios.some((scenario) => scenario.key === value);
}

export function getDemoScenario(key: string): DemoScenario | null {
  return scenarios.find((scenario) => scenario.key === key) ?? null;
}

/** 供静态演示页面使用；夹具缺失属于构建期错误，不应静默降级。 */
export function requireDemoScenario(key: DemoScenarioKey): DemoScenario {
  const scenario = getDemoScenario(key);
  if (!scenario) {
    throw new Error(`Demo scenario ${key} is unavailable.`);
  }
  return scenario;
}

/** 用批准的演示夹具构造与 API 相同的比较信封，不发起网络请求。 */
export function buildDemoComparison(
  leftKey: DemoScenarioKey,
  rightKey: DemoScenarioKey,
): CompareResponse {
  const left = getDemoScenario(leftKey);
  const right = getDemoScenario(rightKey);
  if (!left || !right) {
    throw new Error("Unknown demo scenario.");
  }

  const side = (scenario: DemoScenario) => ({
    request: {
      ecosystem: scenario.report.target.package.ecosystem,
      name: scenario.report.target.package.name,
      ...(scenario.report.target.resolvedVersion === null
        ? {}
        : { version: scenario.report.target.resolvedVersion }),
    },
    report: scenario.report,
  });

  return { left: side(left), right: side(right) };
}
