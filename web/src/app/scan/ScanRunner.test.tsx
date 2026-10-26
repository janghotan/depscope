import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ScanRunner } from "@/app/scan/ScanRunner";
import { liveFindingsReport } from "@/test/fixtures";

describe("ScanRunner", () => {
  it("does not render a response that arrives after cancellation", async () => {
    const user = userEvent.setup();
    let resolveResponse: ((response: Response) => void) | undefined;
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
      <ScanRunner request={{ ecosystem: "npm", name: "lodash" }} />,
    );
    await user.click(screen.getByRole("button", { name: "Cancel request" }));

    await act(async () => {
      resolveResponse?.(
        new Response(JSON.stringify(liveFindingsReport()), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
    });

    expect(screen.getByText("Request cancelled")).toBeInTheDocument();
    expect(
      screen.queryByRole("article", { name: /Condition record for/ }),
    ).not.toBeInTheDocument();
  });
});
