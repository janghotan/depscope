import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { CompareRunner } from "@/app/compare/CompareRunner";
import { liveFindingsReport } from "@/test/fixtures";

describe("CompareRunner", () => {
  it("does not render a comparison that arrives after cancellation", async () => {
    const user = userEvent.setup();
    let resolveResponse: ((response: Response) => void) | undefined;
    const report = liveFindingsReport();
    vi.stubGlobal(
      "fetch",
      vi.fn(
        () =>
          new Promise<Response>((resolve) => {
            resolveResponse = resolve;
          }),
      ),
    );

    render(
      <CompareRunner
        request={{
          left: { ecosystem: "npm", name: "left-package" },
          right: { ecosystem: "pypi", name: "right-package" },
        }}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Cancel request" }));

    await act(async () => {
      resolveResponse?.(
        new Response(
          JSON.stringify({
            left: {
              request: { ecosystem: "npm", name: "left-package" },
              report,
            },
            right: {
              request: { ecosystem: "pypi", name: "right-package" },
              report,
            },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );
    });

    expect(screen.getByText("Request cancelled")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Side-by-side condition" }),
    ).not.toBeInTheDocument();
  });
});
