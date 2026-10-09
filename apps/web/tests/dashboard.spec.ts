import { test, expect } from "@playwright/test";
test("demo dashboard, filtering, score inspection and human gate", async ({
  page,
}) => {
  await page.goto("/");
  await expect(
    page.getByRole("heading", { name: /Good work\.\s*Extraordinary flow\./ }),
  ).toBeVisible();
  await expect(
    page.getByText("DEMO WORKSPACE", { exact: false }),
  ).toBeVisible();
  await page
    .getByRole("link", { name: "Opportunities", exact: false })
    .first()
    .click();
  await page
    .getByRole("combobox", { name: "Filter language" })
    .selectOption("Python");
  await expect(page.locator("tbody tr")).toHaveCount(1);
  await expect(page.locator("tbody tr td").first()).toHaveText("5");
  await page.getByRole("button", { name: "Inspect mlflow/mlflow" }).click();
  await expect(
    page.getByRole("heading", {
      name: "Validate model artifact paths consistently",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Proceed to Contribute" }),
  ).toBeDisabled();
  await expect(
    page.getByText("Effort is informational.", { exact: false }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
});
test("Operator proposes a configuration change and waits for Apply", async ({
  page,
}) => {
  await page.goto("/");
  const configResponse = await page.request.get("/api/config");
  const base = await configResponse.json();
  // Make this test idempotent without deleting config/event history.
  if (base.config.profile.languages.Rust) {
    delete base.config.profile.languages.Rust;
    const proposal = await page.request.post("/api/config/proposals", {
      data: {
        base_version: base.version,
        config: base.config,
        reason: "Browser test setup: remove Rust",
      },
    });
    expect(proposal.ok()).toBeTruthy();
    const p = await proposal.json();
    expect(
      (
        await page.request.post("/api/config/proposals/" + p.id + "/apply", {
          data: {},
        })
      ).ok(),
    ).toBeTruthy();
  }
  await page.getByRole("button", { name: "Operator", exact: true }).click();
  await page.getByRole("button", { name: "Add Rust", exact: true }).click();
  await expect(
    page.getByText("Proposed adding Rust", { exact: false }),
  ).toBeVisible();
  const before = await (await page.request.get("/api/config")).json();
  expect(before.config.profile.languages.Rust).toBeUndefined();
  await page
    .getByRole("link", { name: "Review configuration proposal", exact: false })
    .click();
  await expect(
    page.getByText("profile.languages.Rust:", { exact: false }).last(),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "Apply changes", exact: true })
    .last()
    .click();
  await expect(page.getByRole("status")).toHaveText(
    "Saved. Configuration history and audit events updated.",
  );
  const after = await (await page.request.get("/api/config")).json();
  expect(after.config.profile.languages.Rust).toBe(0.8);
  expect(after.version).toBe(before.version + 1);
});
test("laptop/mobile navigation and light mode", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page
    .getByRole("button", { name: "Open navigation", exact: true })
    .click();
  await page.getByRole("link", { name: "Settings", exact: true }).click();
  await page.getByLabel("Color theme").selectOption("light");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
});
