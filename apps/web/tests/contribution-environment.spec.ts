import { test, expect } from "@playwright/test";

test("environment pause exposes saved setup evidence and explicit recovery", async ({ page }) => {
  const id = "environment-flow-fixture";
  await page.route(`**/api/contributions/${id}`, r => r.fulfill({ json: { id, repository: "fixture/repo", title: "Environment recovery", state: "PAUSED", branch: "fixture", config_version: 1, demo: false } }));
  await page.route(`**/api/contributions/${id}/tests`, r => r.fulfill({ json: [] }));
  await page.route(`**/api/contributions/${id}/execution`, r => r.fulfill({ json: {
    status: "NEEDS_INPUT", phase: "TESTING", summary: "Preparation blocked", fix_iterations: 0, review_cycles: 0, network: false,
    incident: { kind: "environment_blocked", summary: "Required setup is missing", next_action: "Restore the prerequisite before recovering", evidence: "Hook manager is missing", status: "needs_input", can_recover: false },
    environment: { status: "blocked", error: "Hook manager is missing", checks: [{ command: { program: "make", arguments: ["hooks-check"] }, exit_code: 2, output: "lefthook unavailable" }] },
    environment_plan: { completion_gates: [{ requirement: "Signed receipt", source: "CONTRIBUTING.md", action: "Use the trusted signer", binding: "commit" }] },
  } }));
  await page.goto(`/contributions/${id}`);
  await expect(page.getByRole("button", { name: "Retry after setup is restored" })).toBeVisible();
  let recoveries = 0;
  await page.route(`**/api/contributions/${id}/recover`, r => {
    expect(r.request().postDataJSON()).toEqual({ approved: true, network: true });
    recoveries++;
    return r.fulfill({ status: 202, json: {} });
  });
  const downloads = page.getByRole("checkbox", { name: "Allow downloading public dependencies when recovering this environment." });
  await expect(downloads).not.toBeChecked();
  expect(recoveries).toBe(0);
  await downloads.check();
  await page.getByRole("button", { name: "Allow downloads and recover" }).click();
  expect(recoveries).toBe(1);
  await page.getByRole("tab", { name: "Environment", exact: true }).click();
  await expect(page.getByRole("region", { name: "Repository environment" })).toContainText("Signed receipt");
  await expect(page.getByRole("region", { name: "Repository environment" })).toContainText("a committed tree");
  await page.getByText("make hooks-check — exit 2", { exact: true }).click();
  await expect(page.getByText("lefthook unavailable", { exact: true })).toBeVisible();
});
