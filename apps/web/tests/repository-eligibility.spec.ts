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
  const disclosure = page.getByText("Excluded repositories (1) · contributability screening");
  await expect(disclosure).toBeVisible();
  await disclosure.click();
  await expect(page.getByText("Trusted signing receipt is unavailable", { exact: false })).toBeVisible();
  await expect(page.getByRole("link", { name: "Read prerequisite evidence" })).toHaveAttribute("href", "https://github.com/new/private-service/blob/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/CONTRIBUTING.md#L7");
  await expect(page.getByRole("link", {name: "Inspect blocked contribution"})).toHaveCount(0);
  await expect(page.locator("tbody tr")).toHaveCount(0);
  await expect(page.getByText("No contribution was started.", { exact: false })).toBeVisible();
});

test("repositories ForgeFlow cannot build or verify are explained and never ranked", async ({ page }) => {
  await page.route("**/api/repository-exclusions", (route) => route.fulfill({ json: [
    { repository: "example/huge-build", head_sha: "a".repeat(40), status: "excluded", kind: "build_too_large", reason: "The build has 17 JVM build files (multi-module build).", next_action: "Run discovery again after the requirements change.", detected_at: "2026-10-20T00:00:00Z",
      checks: [{ kind: "build_too_large", source: "repository tree", line: 0, reason: "The build has 17 JVM build files (multi-module build)." }, { kind: "heavy_toolchain", source: "web/widgetset/App.gwt.xml", line: 0, reason: "Verification requires GWT/Vaadin widgetset compilation." }] },
    { repository: "example/scaffold", head_sha: "b".repeat(40), status: "needs_assessment", kind: "needs_assessment", reason: "No successful recent GitHub Actions verification run was found.", next_action: "Run discovery again later.", detected_at: "2026-10-20T00:00:00Z",
      checks: [{ kind: "needs_assessment", source: "GitHub Actions", line: 0, reason: "No successful recent GitHub Actions verification run was found." }] },
  ] }));
  await page.route("**/api/opportunities", (route) => route.fulfill({ json: [] }));
  await page.goto("/opportunities");
  await page.getByText("Excluded repositories (2) · contributability screening").click();
  await expect(page.getByText("Not contributable by ForgeFlow")).toBeVisible();
  await expect(page.getByText("Needs assessment")).toBeVisible();
  // Tree- and CI-level findings are not files, so they get no broken evidence link.
  await expect(page.getByRole("link", { name: "Read prerequisite evidence" })).toHaveCount(1);
  await expect(page.getByRole("link", { name: "Read prerequisite evidence" })).toHaveAttribute("href", `https://github.com/example/huge-build/blob/${"a".repeat(40)}/web/widgetset/App.gwt.xml`);
  await expect(page.locator("tbody tr")).toHaveCount(0);
});
