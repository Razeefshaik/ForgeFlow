import { test, expect } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

test("recovery requests explicit downloads and clears historical errors after resume", async ({ page }) => {
  // UI contract fixture. Go tests exercise real commands; this never executes a contribution.
  const c = { id: "reactor-ui", repository: "fixture/reactor", title: "Restore dependency verification", state: "BLOCKED", branch: "autopilot/issue-1", config_version: 1, demo: false, workspace: "external/reactor-ui/repo", base_commit: "a".repeat(40) };
  let execution = { status: "BLOCKED", phase: "TESTING", summary: "Dependency setup failed", fix_iterations: 0, review_cycles: 0, network: false, diff: "", report: "" };
  const incident = { id: "failure", kind: "network_transport", summary: "Dependency setup could not reach its configured proxy.", evidence: "Saved command output reports a proxy connection failure.", next_action: "Allow dependency downloads to rerun the same required check.", status: "blocked", can_recover: false, attempt: 0, test_run_id: "failed-test", detected_at: "2026-10-09T10:00:00Z", command: { program: "go", arguments: ["test", "./..."] } };
  await page.route("**/api/contributions/reactor-ui", r => r.fulfill({ json: c }));
  await page.route("**/api/contributions/reactor-ui/execution", r => r.fulfill({ json: execution }));
  await page.route("**/api/contributions/reactor-ui/diagnosis", r => r.fulfill({ json: incident }));
  await page.route("**/api/contributions/reactor-ui/tests", r => r.fulfill({ json: [{ id: "failed-test", command: incident.command, exit_code: 1, output: "proxyconnect tcp: dial tcp 127.0.0.1:9: connection refused", failure_kind: "network_transport", started_at: "2026-10-09T10:00:00Z", finished_at: "2026-10-09T10:00:02Z" }] }));
  let recoveries = 0;
  await page.route("**/api/contributions/reactor-ui/recover", async r => {
    expect(r.request().postDataJSON()).toEqual({ approved: true, network: true });
    recoveries++;
    execution = { ...execution, status: "RUNNING", summary: "Running required checks", network: true };
    await r.fulfill({ status: 202, json: execution });
  });
  await page.goto("/contributions/reactor-ui");
  const panel = page.getByRole("region", { name: "Issue reactor" });
  await expect(panel.getByText(incident.summary)).toBeVisible();
  expect(recoveries).toBe(0);
  await panel.getByText("Diagnosis evidence").click();
  await panel.getByRole("button", { name: "View saved command output" }).click();
  await expect(page.getByText("proxyconnect tcp: dial tcp 127.0.0.1:9: connection refused", { exact: true })).toBeVisible();
  const artifacts = fileURLToPath(new URL("../../../artifacts/", import.meta.url));
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: path.join(artifacts, "reactor-desktop.png"), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(panel.getByRole("button", { name: "Allow downloads and recover" })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: path.join(artifacts, "reactor-mobile.png"), fullPage: true });
  await panel.getByRole("button", { name: "Allow downloads and recover" }).click();
  await expect(panel).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Running verification", exact: true })).toBeVisible();
  await expect(panel.getByText(incident.summary)).toHaveCount(0);
  expect(recoveries).toBe(1);
});

test("diagnosis failure gives honest feedback without a recovery button", async ({ page }) => {
  await page.route("**/api/contributions/reactor-ui", r => r.fulfill({ json: { id: "reactor-ui", repository: "fixture/reactor", title: "Blocked contribution", state: "BLOCKED", config_version: 1 } }));
  await page.route("**/api/contributions/reactor-ui/execution", r => r.fulfill({ json: { status: "BLOCKED", summary: "Inspect saved evidence", network: false, fix_iterations: 0, review_cycles: 0 } }));
  await page.route("**/api/contributions/reactor-ui/tests", r => r.fulfill({ json: [] }));
  await page.route("**/api/contributions/reactor-ui/diagnosis", r => r.fulfill({ status: 503, json: { error: "Recovery service unavailable" } }));
  await page.goto("/contributions/reactor-ui");
  const panel = page.getByRole("region", { name: "Issue reactor" });
  await expect(panel.getByRole("alert")).toContainText("Diagnosis is unavailable");
  await expect(panel.getByRole("button", { name: /recover/i })).toHaveCount(0);
});

test("scheduled recovery displays a real retry time and permits stopping", async ({ page }) => {
  const at = "2026-10-11T10:00:00Z";
  await page.route("**/api/contributions/reactor-ui", r => r.fulfill({ json: { id: "reactor-ui", repository: "fixture/reactor", title: "Retry service", state: "TESTING", config_version: 1 } }));
  await page.route("**/api/contributions/reactor-ui/execution", r => r.fulfill({ json: { status: "RUNNING", phase: "TESTING", summary: "Retrying original command", network: true, fix_iterations: 0, review_cycles: 0, recovery_attempts: 1, incident: { id: "scheduled", kind: "dependency_network", status: "waiting_retry", summary: "Dependency service did not respond.", evidence: "HTTP 503", next_action: "Retry the original verification command after backoff.", can_recover: true, retryable: true, retry_at: at } } }));
  await page.route("**/api/contributions/reactor-ui/tests", r => r.fulfill({ json: [] }));
  await page.goto("/contributions/reactor-ui");
  const panel = page.getByRole("region", { name: "Issue reactor" });
  await expect(panel.getByText("Waiting for scheduled retry")).toBeVisible();
  await expect(panel.locator("time")).toHaveAttribute("datetime", at);
  await expect(panel.getByRole("button", { name: /recover|retry/i })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Pause contribution" })).toBeVisible();
});

test("authentication needs input without pretending that coding can repair it", async ({ page }) => {
  await page.route("**/api/contributions/reactor-ui", r => r.fulfill({ json: { id: "reactor-ui", repository: "fixture/reactor", title: "Restore credentials", state: "PAUSED", previous_state: "TESTING", config_version: 1 } }));
  await page.route("**/api/contributions/reactor-ui/execution", r => r.fulfill({ json: { status: "NEEDS_INPUT", phase: "TESTING", summary: "Authentication rejected", network: true, fix_iterations: 0, review_cycles: 0, incident: { id: "auth", kind: "authentication", status: "needs_input", summary: "A required service rejected authentication.", evidence: "HTTP 401", next_action: "Configure the service credentials outside the agent, then retry verification.", can_recover: false } } }));
  await page.route("**/api/contributions/reactor-ui/tests", r => r.fulfill({ json: [] }));
  await page.goto("/contributions/reactor-ui");
  await expect(page.getByRole("heading", { name: "Setup needs your input" })).toBeVisible();
  const panel = page.getByRole("region", { name: "Issue reactor" });
  await expect(panel.getByText("Needs input", { exact: true })).toBeVisible();
  await expect(panel.getByRole("button", { name: "Retry after setup is restored" })).toBeVisible();
  await expect(panel.getByText("Recovering automatically")).toHaveCount(0);
});

test("legacy rejected plans offer planning recovery instead of a tests action", async ({ page }) => {
  await page.route("**/api/contributions/reactor-ui", r => r.fulfill({ json: { id: "reactor-ui", repository: "fixture/reactor", title: "Validate plan", state: "BLOCKED", config_version: 1 } }));
  await page.route("**/api/contributions/reactor-ui/execution", r => r.fulfill({ json: { status: "BLOCKED", phase: "PLANNING", summary: "unsafe verification argument", network: false, fix_iterations: 0, review_cycles: 0, incident: { id: "legacy", kind: "execution_blocked", status: "blocked", can_recover: false } } }));
  await page.route("**/api/contributions/reactor-ui/diagnosis", r => r.fulfill({ json: { id: "plan", kind: "plan_invalid", status: "needs_input", summary: "Verification commands were rejected.", evidence: "unsafe verification argument", next_action: "Validate saved plan or request corrected read-only planning.", can_recover: true } }));
  await page.route("**/api/contributions/reactor-ui/tests", r => r.fulfill({ json: [] }));
  await page.goto("/contributions/reactor-ui");
  const panel = page.getByRole("region", { name: "Issue reactor" });
  await expect(panel.getByRole("button", { name: "Recover planning" })).toBeVisible();
  await expect(panel.getByRole("button", { name: "Recover verification" })).toHaveCount(0);
});

test("Codex service retries retain dependency download approval", async ({ page }) => {
  const c = { id: "reactor-ui", repository: "fixture/reactor", title: "Reconnect Codex", state: "PAUSED", config_version: 1 };
  let execution: Record<string, unknown> = { status: "NEEDS_INPUT", phase: "CODING", network: false, fix_iterations: 0, review_cycles: 0, incident: { id: "codex", kind: "dependency_network", source: "codex", status: "needs_input", summary: "Codex service connection failed.", evidence: "HTTP 503", next_action: "Retry the original coding stage with partial work preserved.", can_recover: true } };
  await page.route("**/api/contributions/reactor-ui", r => r.fulfill({ json: c }));
  await page.route("**/api/contributions/reactor-ui/execution", r => r.fulfill({ json: execution }));
  await page.route("**/api/contributions/reactor-ui/tests", r => r.fulfill({ json: [] }));
  await page.route("**/api/contributions/reactor-ui/recover", r => {
    expect(r.request().postDataJSON()).toEqual({ approved: true, network: false });
    execution = { ...execution, status: "RUNNING", incident: undefined };
    return r.fulfill({ status: 202, json: execution });
  });
  await page.goto("/contributions/reactor-ui");
  const panel = page.getByRole("region", { name: "Issue reactor" });
  await expect(panel.getByRole("button", { name: "Allow downloads and recover" })).toHaveCount(0);
  await panel.getByRole("button", { name: "Retry service connection" }).click();
  await expect(page.getByRole("heading", { name: "Implementing changes" })).toBeVisible();
});
