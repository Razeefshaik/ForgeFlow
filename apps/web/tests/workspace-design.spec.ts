import { test, expect } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

test("completed stages, model identity and evidence stay interactive through a rerun", async ({ page }) => {
  // UI contract fixtures only. These do not execute or submit contributions.
  const contribution = { id: "design-fixture", repository: "fixture/contribution-workspace", title: "Preserve credentials when creating an isolated clone", state: "READY", previous_state: "REVIEWING", branch: "autopilot/issue-42", config_version: 1, codex_model: "selected-model-fixture", demo: false, workspace: "external/design-fixture/repo" };
  const approvedReview = { verdict: "APPROVE", summary: "Independent review approved", findings: [] };
  let execution = { status: "READY", phase: "READY", message: "Real verification passed and independent review approved. Ready for your review.", summary: "Corrected credential helper handling", fix_iterations: 1, review_cycles: 1, network: true, plan: { summary: "Preserve the configured credential helper during clone", root_cause: "The clone wrapper clears the helper configured during repository setup.", strategy: "Use the configured helper, preserve workspace isolation, and cover the regression with focused tests.", files: ["internal/git/clone.go", "internal/git/clone_test.go"], risks: ["The change touches Git authentication behavior."], unknowns: [] }, review: approvedReview as typeof approvedReview | null, diff: "diff --git a/clone.go b/clone.go\n-return runGit()\n+return cloneWithHelper()", report: "The saved implementation and verification are ready for human review." };
  const command = { program: "go", arguments: ["test", "./..."] };
  await page.route("**/api/contributions/design-fixture", r => r.fulfill({ json: contribution }));
  await page.route("**/api/contributions/design-fixture/execution", r => r.fulfill({ json: execution }));
  await page.route("**/api/contributions/design-fixture/tests", r => {
    if (r.request().method() === "POST") {
      expect(r.request().postDataJSON()).toEqual({ approved: true });
      reruns++;
      execution = { ...execution, status: "RUNNING", phase: "TESTING", message: "Execution running: testing", review: null };
      return r.fulfill({ status: 202, json: execution });
    }
    return r.fulfill({ json: [
      { id: "failed-fixture", command, exit_code: 1, output: "FAIL: credential helper was cleared", started_at: "2026-10-09T10:00:00Z", finished_at: "2026-10-09T10:00:02Z", failure_kind: "code_failure" },
      { id: "passed-fixture", command, exit_code: 0, output: "ok fixture/git", started_at: "2026-10-09T10:01:00Z", finished_at: "2026-10-09T10:01:03Z" },
    ] });
  });
  await page.route("**/api/agents", r => r.fulfill({ json: [] }));
  await page.route("**/api/events**", r => r.request().url().includes("stream") ? r.abort() : r.fulfill({ json: [] }));
  let reruns = 0;
  await page.addInitScript(() => localStorage.setItem("forgeflow-theme", "light"));
  await page.goto("/contributions/design-fixture");
  await expect(page.getByLabel("Selected contribution AI model")).toContainText(contribution.codex_model);
  const stages = page.getByRole("list", { name: "Contribution stages" });
  await expect(stages.locator('[data-status="complete"]')).toHaveCount(5);
  await expect(stages.getByRole("button", { name: /Human review: Your turn/ })).toBeVisible();
  await expect(page.getByRole("region", { name: "More contribution actions" })).toHaveCount(0);
  const artifacts = fileURLToPath(new URL("../../../artifacts/", import.meta.url));
  await page.screenshot({ path: path.join(artifacts, "workspace-refined-light.png"), fullPage: true, animations: "disabled" });
  await page.getByRole("button", { name: "Switch to dark mode", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.screenshot({ path: path.join(artifacts, "workspace-refined-dark.png"), fullPage: true, animations: "disabled" });
  await stages.getByRole("button", { name: /Test: Completed/ }).click();
  await expect(page.getByRole("tab", { name: "Tests (2)" })).toBeFocused();
  await page.getByLabel("Filter test results").selectOption("Failed");
  await expect(page.locator("#test-run-failed-fixture")).toBeVisible();
  await expect(page.locator("#test-run-passed-fixture")).toHaveCount(0);
  await page.getByLabel("Search test commands and output").fill("missing-command");
  await expect(page.getByText("No commands match this search and filter.")).toBeVisible();
  await page.getByLabel("Search test commands and output").fill("");
  await page.getByLabel("Filter test results").selectOption("All");
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.evaluate(() => window.scrollTo(0, 0));
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: path.join(artifacts, "workspace-refined-mobile.png"), fullPage: true, animations: "disabled" });
  await page.getByRole("button", { name: "More actions" }).click();
  await page.getByRole("button", { name: "Retry tests", exact: true }).click();
  await expect(stages.getByRole("button", { name: /Test: In progress/ })).toBeVisible();
  await expect(stages.locator('[data-status="complete"]')).toHaveCount(2);
  await expect(page.getByRole("heading", { name: "Running verification", exact: true })).toBeVisible();
  expect(reruns).toBe(1);
});

test("agent model fallback is recorded evidence and resuming requires approval", async ({ page }) => {
  const c = { id: "model-fallback", repository: "fixture/models", title: "Resume the saved patch", state: "PAUSED", previous_state: "CODING", branch: "autopilot/issue-1", config_version: 1, demo: false };
  await page.route("**/api/contributions/model-fallback", r => r.fulfill({ json: c }));
  await page.route("**/api/contributions/model-fallback/execution", r => r.fulfill({ json: { status: "PAUSED", phase: "CODING", summary: "Saved execution paused", fix_iterations: 0, review_cycles: 0, network: false } }));
  await page.route("**/api/contributions/model-fallback/tests", r => r.fulfill({ json: [] }));
  await page.route("**/api/agents", r => r.fulfill({ json: [{ contribution_id: c.id, model: "recorded-agent-model", started_at: "2026-10-09T10:00:00Z" }] }));
  let starts = 0;
  await page.route("**/api/contributions/model-fallback/start", r => { starts++; return r.fulfill({ status: 202, json: {} }); });
  await page.goto("/contributions/model-fallback");
  await expect(page.getByLabel("Selected contribution AI model")).toContainText("recorded-agent-model");
  await page.getByRole("button", { name: "Resume execution", exact: true }).click();
  await expect(page.getByRole("button", { name: "Approve and resume" })).toBeDisabled();
  expect(starts).toBe(0);
  await page.getByRole("checkbox", { name: /I approve Codex coding/ }).check();
  await expect(page.getByRole("button", { name: "Approve and resume" })).toBeEnabled();
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  expect(starts).toBe(0);
});

test("unused fixes and review requests never produce misleading completion ticks", async ({ page }) => {
  const c = { id: "stages-fixture", repository: "fixture/stages", title: "Review the contribution", state: "REVIEWING", branch: "autopilot/issue-3", config_version: 1, demo: false };
  let r = { status: "RUNNING", phase: "REVIEWING", fix_iterations: 0, review_cycles: 1, plan: { summary: "Saved plan", files: [], risks: [], unknowns: [] }, review: null as { verdict: string; findings: [] } | null };
  await page.route("**/api/contributions/stages-fixture", route => route.fulfill({ json: c }));
  await page.route("**/api/contributions/stages-fixture/execution", route => route.fulfill({ json: r }));
  await page.route("**/api/contributions/stages-fixture/tests", route => route.fulfill({ json: [] }));
  await page.route("**/api/agents", route => route.fulfill({ json: [] }));
  await page.goto("/contributions/stages-fixture");
  const stages = page.getByRole("list", { name: "Contribution stages" });
  await expect(stages.getByRole("button", { name: /Fix: Not needed/ })).toBeVisible();
  await expect(stages.locator('[data-status="complete"]')).toHaveCount(3);
  await expect(page.getByLabel("Selected contribution AI model")).toContainText("Not recorded");
  r = { ...r, phase: "FIXING", fix_iterations: 1, review: { verdict: "REQUEST_CHANGES", findings: [] } };
  await expect(stages.getByRole("button", { name: /Test: Needs rerun/ })).toBeVisible({ timeout: 10000 });
  await expect(stages.getByRole("button", { name: /Review: Changes requested/ })).toBeVisible();
  await expect(stages.locator('[data-status="complete"]')).toHaveCount(2);
});
