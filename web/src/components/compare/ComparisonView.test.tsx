import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ComparisonView } from "@/components/compare/ComparisonView";
import type { CompareResponse } from "@/lib/api-types";
import { buildDemoComparison } from "@/lib/demo";
import { demoCleanReport, liveFindingsReport } from "@/test/fixtures";

describe("ComparisonView", () => {
  it("presents both recorded conditions and marks differences", () => {
    render(<ComparisonView response={buildDemoComparison("findings", "clean")} />);

    expect(
      screen.getByRole("heading", { name: "Side-by-side condition" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "Condition: Findings" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "Condition: Clean" }),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Differs").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Read the full condition record"),
    ).toHaveLength(2);
    expect(
      screen.getByText(/does not choose a package/),
    ).toBeInTheDocument();
  });

  it("keeps a failed side visible beside a successful side", () => {
    const response: CompareResponse = {
      left: {
        request: { ecosystem: "npm", name: "left-package" },
        report: demoCleanReport(),
      },
      right: {
        request: { ecosystem: "pypi", name: "right-package" },
        error: { code: "not_found", message: "Package not found." },
      },
    };

    render(<ComparisonView response={response} />);

    expect(screen.getByText("Package not found.")).toBeInTheDocument();
    expect(screen.getByText("Side error")).toBeInTheDocument();
    expect(
      screen.getByText(/This side produced no report/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: "Condition: Clean" }),
    ).toBeInTheDocument();
  });

  it("warns when the sides use different data modes", () => {
    const response: CompareResponse = {
      left: {
        request: { ecosystem: "npm", name: "live-package" },
        report: liveFindingsReport(),
      },
      right: {
        request: { ecosystem: "npm", name: "demo-package" },
        report: demoCleanReport(),
      },
    };

    render(<ComparisonView response={response} />);
    expect(
      screen.getByText(/different data modes/),
    ).toBeInTheDocument();
  });

  it("renders a side's full record only after its panel is opened", () => {
    render(<ComparisonView response={buildDemoComparison("findings", "clean")} />);

    expect(
      screen.queryByRole("heading", { name: "Coverage checks" }),
    ).toBeNull();

    const panel = screen
      .getAllByText("Read the full condition record")[0]
      .closest("details");
    expect(panel).not.toBeNull();
    panel!.open = true;
    fireEvent(panel!, new Event("toggle"));

    expect(document.getElementById("side-a-checks-heading")).not.toBeNull();
    expect(document.getElementById("side-b-checks-heading")).toBeNull();
    expect(
      screen.getAllByRole("heading", { name: "Coverage checks" }),
    ).toHaveLength(1);
  });

  it("does not claim a highest severity when a side has no report", () => {
    const response: CompareResponse = {
      left: {
        request: { ecosystem: "npm", name: "left-package" },
        error: { code: "scan_failed", message: "Scan failed." },
      },
      right: {
        request: { ecosystem: "npm", name: "right-package" },
        error: { code: "scan_failed", message: "Scan failed." },
      },
    };

    render(<ComparisonView response={response} />);
    const row = screen.getByRole("row", { name: /Highest severity/ });
    expect(row).toHaveTextContent("—");
    expect(row).not.toHaveTextContent("None");
  });
});
