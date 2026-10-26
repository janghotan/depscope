import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { DemoSpecimen } from "@/components/home/DemoSpecimen";
import { requireDemoScenario } from "@/lib/demo";

describe("DemoSpecimen", () => {
  it("labels the sheet as demo data and links to the full specimen", () => {
    render(<DemoSpecimen scenario={requireDemoScenario("findings")} />);

    expect(screen.getByText(/Demo data · specimen sheet/)).toBeInTheDocument();
    expect(screen.getByText(/Offline · no network/)).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "Condition: Findings" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Open the full specimen/ }),
    ).toHaveAttribute("href", "/demo/findings");
    expect(
      screen.getAllByText("depscope-demo-vulnerable-package").length,
    ).toBeGreaterThan(0);
    expect(
      screen.getByText(/DEMO DATA: synthetic vulnerable dependency/),
    ).toBeInTheDocument();
  });

  it("shows three succeeded check strips and a two-node load path", () => {
    render(<DemoSpecimen scenario={requireDemoScenario("findings")} />);

    expect(screen.getAllByText("Succeeded")).toHaveLength(3);
    expect(screen.getByText("2 nodes · 1 checkpoint")).toBeInTheDocument();
    expect(screen.getByText(/checkpoint on root/)).toBeInTheDocument();
    expect(screen.getByText(/Heuristic signal · not confirmed/)).toBeInTheDocument();
  });
});
