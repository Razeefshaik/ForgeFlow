import { test, expect } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";
const artifacts = fileURLToPath(new URL("../../../artifacts/", import.meta.url));

test("demo discovery explains live mode and offers no scan action", async ({
  page,
}) => {
  await page.goto("/opportunities");
  await expect(
    page.getByRole("heading", { name: "GitHub discovery" }),
  ).toBeVisible();
  await expect(
    page.getByText("Demo uses illustrative data.", { exact: false }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Run discovery now" }),
  ).toHaveCount(0);
});

const liveURL = process.env.FORGEFLOW_LIVE_TEST_URL;
test("live dashboard displays persisted GitHub evidence", async ({ page }) => {
  test.skip(
    !liveURL,
    "Set FORGEFLOW_LIVE_TEST_URL to an already-scanned disposable live server.",
  );
  await page.goto(liveURL + "/opportunities");
  await expect(
    page.getByText("LIVE WORKSPACE", { exact: false }),
  ).toBeVisible();
  await expect(page.locator("tbody tr").first()).toBeVisible();
  await page
    .locator("tbody tr")
    .first()
    .getByRole("button", { name: /^Inspect / })
    .click();
  await expect(
    page.getByRole("heading", { name: "Observed on GitHub" }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open issue on GitHub" }),
  ).toHaveAttribute("href", /^https:\/\/github\.com\/.+\/issues\/\d+$/);
  await expect(
    page.getByRole("button", { name: "Proceed to Contribute" }),
  ).toBeDisabled();
  await page.getByText("Full issue description", { exact: true }).click();
  await expect(page.locator(".issue-description")).toBeVisible();
  await page.locator(".dialog-content").evaluate((el) => {
    el.scrollTop = 0;
  });
  await page.screenshot({
    path: path.join(artifacts, "discovery-evidence-desktop.png"),
    fullPage: true,
    animations: "disabled",
  });
  await page.keyboard.press("Escape");
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
  await page.screenshot({
    path: path.join(artifacts, "discovery-mobile.png"),
    fullPage: true,
    animations: "disabled",
  });
});

test("live Run action appends scan history while execution stays unavailable", async ({
  page,
}) => {
  test.skip(
    !liveURL,
    "Live scan check is opt-in and performs read-only GitHub requests.",
  );
  await page.goto(liveURL + "/opportunities");
  const before = await (
    await page.request.get(liveURL + "/api/discovery/runs")
  ).json();
  await page.getByRole("button", { name: "Run discovery now" }).click();
  await expect
    .poll(
      async () => {
        const runs = await (
          await page.request.get(liveURL + "/api/discovery/runs")
        ).json();
        return runs.length > before.length && runs[0].status !== "RUNNING";
      },
      { timeout: 60_000 },
    )
    .toBeTruthy();
  const overview = await (
    await page.request.get(liveURL + "/api/overview")
  ).json();
  expect(overview.execution_available).toBe(false);
  expect(overview.active_contributions).toBe(0);
  await expect(
    page.getByRole("button", { name: "Run discovery now" }),
  ).toBeEnabled();
  await expect(page.locator(".discovery-last")).not.toContainText("running");
  await page.getByRole("button", { name: "View scan history" }).click();
  await expect(
    page.locator(".discovery-history details").first(),
  ).toBeVisible();
  await page.screenshot({
    path: path.join(artifacts, "discovery-desktop.png"),
    fullPage: true,
  });
});
