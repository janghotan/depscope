import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { ReportView } from "@/components/report/ReportView";
import type { ScanReport } from "@/generated/report-types";
import { demoFindingsReport, liveFindingsReport } from "@/test/fixtures";

describe("ReportView", () => {
  it("renders the condition, tally, checks, sources, and limitations", () => {
    render(<ReportView report={demoFindingsReport()} />);

    expect(
      screen.getByRole("img", { name: "Condition: Findings" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Confirmed findings")).toBeInTheDocument();
    expect(screen.getByText("Heuristic warnings")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Coverage checks" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Dependency load path" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Sources" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Limitations" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Demo data notice")).toHaveTextContent(
      /DEMO DATA/,
    );
  });

  it("expands and collapses finding evidence with a labelled control", async () => {
    const user = userEvent.setup();
    render(<ReportView report={demoFindingsReport()} />);

    const toggle = screen.getByRole("button", {
      name: /DEMO DATA: synthetic vulnerable dependency/,
    });
    expect(toggle).toHaveAttribute("aria-expanded", "false");

    await user.click(toggle);

    expect(toggle).toHaveAttribute("aria-expanded", "true");
    const evidence = screen.getByRole("region", {
      name: /Evidence for DEMO DATA: synthetic vulnerable dependency/,
    });
    expect(evidence).toBeInTheDocument();
    // 纯文本副本也包含说明文字，因此只在证据面板内断言。
    expect(
      within(evidence).getByText(/this finding exists only to exercise/),
    ).toBeInTheDocument();

    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "false");
  });

  it("opens the matching finding from a load-path checkpoint", async () => {
    const user = userEvent.setup();
    render(<ReportView report={demoFindingsReport()} />);

    const marker = screen.getAllByRole("button", {
      name: /Open evidence/,
    })[0];
    await user.click(marker);

    expect(
      screen.getByRole("button", {
        name: /DEMO DATA: synthetic vulnerable dependency/,
      }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByRole("region", {
        name: /Evidence for DEMO DATA: synthetic vulnerable dependency/,
      }),
    ).toBeInTheDocument();
  });

  it("lets keyboard users inspect a dependency node and open its evidence", async () => {
    const user = userEvent.setup();
    render(<ReportView report={demoFindingsReport()} />);

    const packageButton = screen.getByRole("button", {
      name: /Inspect package @depscope-demo\/transitive-package/,
    });
    packageButton.focus();
    await user.keyboard("{Enter}");

    const details = screen.getByRole("region", {
      name: /Package details for @depscope-demo\/transitive-package/,
    });
    expect(within(details).getByText("direct")).toBeInTheDocument();

    const evidenceButton = within(details).getByRole("button", {
      name: /Open evidence/,
    });
    evidenceButton.focus();
    await user.keyboard("{Enter}");

    expect(
      screen.getByRole("region", { name: /Evidence for/ }),
    ).toBeInTheDocument();
  });

  it("renders untrusted text as text, never as markup", () => {
    const base = demoFindingsReport();
    const report: ScanReport = {
      ...base,
      findings: base.findings.map((finding, index) =>
        index === 0
          ? {
              ...finding,
              title: '<img src=x onerror="alert(1)">',
              description: "<script>alert(2)</script>",
            }
          : finding,
      ),
    };

    const { container } = render(<ReportView report={report} />);

    expect(
      screen.getByText('<img src=x onerror="alert(1)">'),
    ).toBeInTheDocument();
    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("script")).toBeNull();
  });

  it("links only safe http(s) source URLs", () => {
    const live = liveFindingsReport();
    const unsafe: ScanReport = {
      ...live,
      sources: live.sources.map((source, index) =>
        index === 0
          ? { ...source, url: "javascript:alert(1)" }
          : { ...source, url: "https://example.test/advisory" },
      ),
    };

    render(<ReportView report={unsafe} />);

    const links = screen.getAllByRole("link", { name: /Open/ });
    expect(links).toHaveLength(1);
    expect(links[0]).toHaveAttribute(
      "href",
      "https://example.test/advisory",
    );
    expect(links[0]).toHaveAttribute("rel", "noopener noreferrer");
    expect(links[0]).toHaveAttribute("target", "_blank");
  });
});
