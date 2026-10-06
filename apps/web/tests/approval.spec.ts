import { test, expect } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

test("workspace approval requires fresh preview and explicit confirmation", async ({ page }) => {
  // UI contract fixture only. Backend integration tests exercise actual Git.
  const original = await page.request.get("/api/opportunities");
  const examples = await original.json();
  const o = { ...examples[0], id: "approval-ui-fixture", demo: false };
  await page.route("**/api/opportunities", r => r.fulfill({ json: [o] }));
  await page.route("**/api/opportunities/approval-ui-fixture/preview", r => r.fulfill({ json: {
    token: "f".repeat(64), configuration: { version: 1 }, base_commit: "a".repeat(40),
    checked_at: new Date().toISOString(), issue: { title: "Fresh approval fixture", body: "Current issue body", html_url: "https://github.com/example/project/issues/42" },
    warnings: ["Preparation only; coding adapter pending."]
  } }));
  let approvals = 0;
  await page.route("**/api/opportunities/approval-ui-fixture/proceed", async r => {
    const b = r.request().postDataJSON(); expect(b.approved).toBe(true); expect(b.token).toBe("f".repeat(64)); approvals++;
    await r.fulfill({ status: 202, json: { id: "fixture", state: "SELECTED" } });
  });
  await page.goto("/opportunities");
  await page.getByRole("button", { name: "Inspect " + o.repository }).click();
  await page.getByRole("button", { name: "Proceed to Contribute" }).click();
  await expect(page.getByRole("heading", { name: "Fresh approval fixture" })).toBeVisible();
  const confirm = page.getByRole("button", { name: "Approve and start contribution" });
  await expect(confirm).toBeDisabled(); expect(approvals).toBe(0);
  await page.getByRole("checkbox", { name: "I approve cloning, Codex coding, verification and review in this external workspace." }).check();
  await expect(confirm).toBeEnabled();
  await page.locator(".dialog-content").evaluate(el => { el.scrollTop = el.scrollHeight; });
  const artifacts = fileURLToPath(new URL("../../../artifacts/", import.meta.url));
  await page.screenshot({ path: path.join(artifacts, "workspace-approval-desktop.png"), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator(".dialog-content").evaluate(el => { el.scrollTop = el.scrollHeight; });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: path.join(artifacts, "workspace-approval-mobile.png"), fullPage: true });
  await confirm.click(); await expect(page).toHaveURL(/\/contributions$/); expect(approvals).toBe(1);
});
