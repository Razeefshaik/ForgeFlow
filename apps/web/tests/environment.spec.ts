import { test, expect } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

const initial = { available: true, platform: "windows", mode: "auto", status: "unverified", message: "Check workspace isolation and Git shell compatibility before execution.", running: false, checks: [] as { name: string; passed: boolean; evidence?: string }[] };

test("Windows setup requires approval, preserves real failures and permits a verified resume", async ({ page }) => {
  let state = { ...initial };
  let calls = 0;
  await page.route("**/api/runtime/environment", r => r.fulfill({ json: state }));
  await page.route("**/api/runtime/environment/setup", async r => {
    expect(r.request().postDataJSON()).toEqual({ approved: true, mode: "auto" });
    calls++;
    state = { ...state, status: "awaiting_windows_approval", running: true, message: "Approve the Windows elevation prompt to provision the Codex sandbox." };
    await r.fulfill({ status: 202, json: state });
  });
  await page.goto("/settings?resume=environment-ui#sandbox");
  const panel = page.getByRole("region", { name: "Execution environment" });
  await expect(panel.getByRole("button", { name: "Set up Windows sandbox" })).toBeDisabled();
  expect(calls).toBe(0);
  await panel.getByRole("checkbox").check();
  await panel.getByRole("button", { name: "Set up Windows sandbox" }).click();
  await expect(panel.getByRole("status")).toContainText("Approve the Windows elevation prompt");
  await expect(panel.getByRole("button", { name: "Check environment" })).toBeDisabled();
  state = { ...state, running: false, status: "failed", message: "The environment did not pass verification.", checks: [{ name: "Git shell startup", passed: false, evidence: "sh.exe: signal pipe access denied" }] };
  await expect(panel.getByText("Failed", { exact: true })).toBeVisible();
  await expect(panel.getByRole("button", { name: "Resume contribution" })).toHaveCount(0);
  await panel.getByText("View check output").click();
  await expect(panel.getByText("sh.exe: signal pipe access denied")).toBeVisible();
  state = { ...state, mode: "elevated", status: "ready", message: "Workspace isolation and Git shell startup passed." };
  await page.route("**/api/runtime/environment/check", async r => { state = { ...state }; await r.fulfill({ status: 202, json: state }); });
  await panel.getByRole("button", { name: "Check environment" }).click();
  await expect(panel.getByRole("button", { name: "Resume contribution" })).toBeVisible();
  const artifacts = fileURLToPath(new URL("../../../artifacts/", import.meta.url));
  await panel.screenshot({ path: path.join(artifacts, "environment-desktop.png") });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await panel.screenshot({ path: path.join(artifacts, "environment-mobile.png") });
  let resumes = 0;
  await page.route("**/api/contributions/environment-ui/recover", async r => { expect(r.request().postDataJSON()).toEqual({ approved: true, network: false }); resumes++; await r.fulfill({ status: 202, json: {} }); });
  await panel.getByRole("button", { name: "Resume contribution" }).click();
  await expect(page).toHaveURL(/\/contributions\/environment-ui$/);
  expect(resumes).toBe(1);
});

test("environment request errors remain visible and never imply success", async ({ page }) => {
  await page.route("**/api/runtime/environment", r => r.fulfill({ json: initial }));
  await page.route("**/api/runtime/environment/select", r => r.fulfill({ status: 409, json: { error: "environment operation is already running" } }));
  await page.goto("/settings#sandbox");
  const panel = page.getByRole("region", { name: "Execution environment" });
  await panel.getByLabel("Windows sandbox", { exact: true }).selectOption("elevated");
  await panel.getByRole("checkbox").check();
  await panel.getByRole("button", { name: "Verify and save mode" }).click();
  await expect(panel.getByRole("alert")).toContainText("already running");
  await expect(panel.getByText("unverified", { exact: true })).toBeVisible();
});
