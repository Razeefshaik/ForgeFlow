import { test, expect } from "@playwright/test";

test("new repositories excluded by screening show pinned prerequisite evidence", async ({ page }) => {
  await page.route("**/api/repository-exclusions", async (route) => {
    await route.fulfill({ json: [{
      repository: "new/private-service",
      head_sha: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      status: "excluded",
      checks: [{kind:"trusted_signing",source:"CONTRIBUTING.md",line:7,reason:"Required trusted signer is unavailable"}],
      kind: "trusted_signing",
      reason: "Trusted signing receipt is unavailable",
      next_action: "Inspect prerequisite evidence and run discovery after the requirements change.",
      detected_at: "2026-10-12T10:00:00Z",
    }] });
  });
  await page.route("**/api/opportunities", (route) => route.fulfill({ json: [] }));
  await page.goto("/opportunities");
  const disclosure = page.getByText("Excluded repositories (1) · prerequisite screening");
  await expect(disclosure).toBeVisible();
  await disclosure.click();
  await expect(page.getByText("Trusted signing receipt is unavailable", { exact: false })).toBeVisible();
  await expect(page.getByRole("link", { name: "Read prerequisite evidence" })).toHaveAttribute("href", "https://github.com/new/private-service/blob/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/CONTRIBUTING.md#L7");
  await expect(page.getByRole("link", {name: "Inspect blocked contribution"})).toHaveCount(0);
  await expect(page.locator("tbody tr")).toHaveCount(0);
  await expect(page.getByText("No contribution was started.", { exact: false })).toBeVisible();
});
