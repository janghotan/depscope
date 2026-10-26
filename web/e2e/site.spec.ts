import { expect, test } from "@playwright/test";

import { liveFindingsReport } from "../src/test/fixtures";

test("visitor can submit a live scan and inspect evidence", async ({ page }) => {
  const requests: unknown[] = [];
  await page.route("**/api/scan", async (route) => {
    requests.push(route.request().postDataJSON());
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(liveFindingsReport()),
    });
  });

  await page.goto("/");
  await page.getByLabel("Package name").fill("example-package");
  await page.getByLabel("Exact version").fill("2.4.1");
  await page.getByRole("button", { name: "Inspect package" }).click();

  await expect(page).toHaveURL(/\/scan\?/);
  await expect(
    page.getByRole("article", { name: /Condition record for npm/ }),
  ).toBeVisible();
  await expect(page.getByRole("region", { name: "Findings" })).toContainText(
    "Confirmed",
  );
  expect(requests).toEqual([
    {
      ecosystem: "npm",
      name: "example-package",
      version: "2.4.1",
    },
  ]);
});

test("offline demo is visibly labelled and makes no scan request", async ({ page }) => {
  const apiRequests: string[] = [];
  page.on("request", (request) => {
    if (request.url().match(/\/api\/(scan|compare)(\?|$)/)) {
      apiRequests.push(request.url());
    }
  });

  await page.goto("/demo/incomplete");
  await expect(
    page.getByRole("complementary", { name: "Demo data notice" }),
  ).toBeVisible();
  await expect(
    page.getByRole("img", { name: "Condition: Incomplete" }),
  ).toBeVisible();
  expect(apiRequests).toEqual([]);
});

test("homepage remains usable on a narrow viewport", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "Inspect the package before you add it." })).toBeVisible();
  await expect(page.getByLabel("Package name")).toBeVisible();
  const dimensions = await page.evaluate(() => ({
    documentWidth: document.documentElement.scrollWidth,
    viewportWidth: document.documentElement.clientWidth,
    overflow: [...document.querySelectorAll("body *")]
      .filter(
        (element) =>
          element.getBoundingClientRect().right >
          document.documentElement.clientWidth + 1,
      )
      .map((element) => ({
        tag: element.tagName,
        className: element instanceof HTMLElement ? element.className : "",
        text: element.textContent?.slice(0, 80),
        right: element.getBoundingClientRect().right,
      }))
      .slice(0, 8),
  }));
  expect(
    dimensions.documentWidth,
    JSON.stringify(dimensions.overflow),
  ).toBeLessThanOrEqual(dimensions.viewportWidth);
});
