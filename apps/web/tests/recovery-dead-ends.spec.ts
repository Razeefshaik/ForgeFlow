import { test, expect } from "@playwright/test";

const contribution = (id: string) => ({ id, repository: "fixture/repo", title: "Recovery fixture", state: "PAUSED", branch: "fixture", config_version: 1, demo: false });

test("an issue edited after approval needs explicit approval before resuming", async ({ page }) => {
  const id = "issue-update-fixture";
  await page.route(`**/api/contributions/${id}`, r => r.fulfill({ json: contribution(id) }));
  await page.route(`**/api/contributions/${id}/tests`, r => r.fulfill({ json: [] }));
  await page.route(`**/api/contributions/${id}/diagnosis`, r => r.fulfill({ json: null }));
  await page.route(`**/api/contributions/${id}/execution`, r => r.fulfill({ json: {
    status: "NEEDS_INPUT", phase: "PLANNING", summary: "issue scope changed since approval", fix_iterations: 0, review_cycles: 0, network: true,
    incident: { id: "i1", kind: "issue_changed", summary: "The upstream issue was edited after you approved this contribution.", next_action: "Review the updated issue.", evidence: "Both versions are saved.", status: "needs_input", can_recover: false, attempt: 0, detected_at: new Date().toISOString() },
    pending_issue: { title: "Correct Add for all integers", body: "Also keep the public signature.", fetched_at: new Date().toISOString() },
    diff: "", report: "", pr_title: "", pr_body: "", submission_token: "", pr_url: "",
  } }));
  let approvals = 0;
  await page.route(`**/api/contributions/${id}/approve-issue`, r => {
    expect(r.request().postDataJSON()).toEqual({ approved: true });
    approvals++;
    return r.fulfill({ status: 202, json: { status: "accepted" } });
  });
  await page.goto(`/contributions/${id}`);
  const panel = page.getByRole("region", { name: "Updated issue awaiting approval" });
  await expect(panel).toContainText("Correct Add for all integers");
  await expect(panel).toContainText("Also keep the public signature.");
  const approve = panel.getByRole("button", { name: "Approve updated issue" });
  await expect(approve).toBeDisabled();
  await panel.getByRole("checkbox").check();
  await approve.click();
  await expect.poll(() => approvals).toBe(1);
  // Recovery is not offered for an unapproved issue edit.
  await expect(page.getByRole("button", { name: /recover/i })).toHaveCount(0);
});

test("resuming keeps the saved download approval visible and selected", async ({ page }) => {
  const id = "download-resume-fixture";
  await page.route(`**/api/contributions/${id}`, r => r.fulfill({ json: contribution(id) }));
  await page.route(`**/api/contributions/${id}/tests`, r => r.fulfill({ json: [] }));
  await page.route(`**/api/contributions/${id}/execution`, r => r.fulfill({ json: {
    status: "PAUSED", phase: "TESTING", summary: "Execution paused", fix_iterations: 1, review_cycles: 0, network: true,
    diff: "", report: "", pr_title: "", pr_body: "", submission_token: "", pr_url: "",
  } }));
  let body: unknown;
  await page.route(`**/api/contributions/${id}/start`, r => { body = r.request().postDataJSON(); return r.fulfill({ status: 202, json: {} }); });
  await page.goto(`/contributions/${id}`);
  await page.getByRole("button", { name: "Resume execution" }).click();
  const downloads = page.getByRole("checkbox", { name: "Allow dependency downloads during execution." });
  await expect(downloads).toBeChecked();
  await page.getByRole("checkbox", { name: /I approve Codex coding/ }).check();
  await page.getByRole("button", { name: "Approve and resume" }).click();
  await expect.poll(() => body).toEqual({ approved: true, network: true });
});

test("baseline results and pre-existing failures are labelled, not shown as passes", async ({ page }) => {
  const id = "baseline-fixture";
  await page.route(`**/api/contributions/${id}`, r => r.fulfill({ json: { ...contribution(id), state: "READY" } }));
  const now = new Date().toISOString();
  await page.route(`**/api/contributions/${id}/tests`, r => r.fulfill({ json: [
    { id: "b1", command: { program: "python", arguments: ["-m", "pytest"] }, exit_code: 1, output: "FAILED legacy/test_reports.py::test_v2", started_at: now, finished_at: now, truncated: false, failure_kind: "code_failure", baseline: true },
    { id: "p1", command: { program: "python", arguments: ["-m", "pytest"] }, exit_code: 1, output: "FAILED legacy/test_reports.py::test_v2", started_at: now, finished_at: now, truncated: false, failure_kind: "code_failure" },
  ] }));
  await page.route(`**/api/contributions/${id}/execution`, r => r.fulfill({ json: {
    status: "READY", phase: "READY", summary: "Fixed add", fix_iterations: 0, review_cycles: 1, network: true,
    baseline: { status: "preexisting_failures", base_commit: "4fe48cf9eb17685716743f97", commands: [{ command: { program: "python", arguments: ["-m", "pytest"] }, test_run_id: "b1", exit_code: 1, passed: false, failed_tests: ["pytest:legacy/test_reports.py::test_v2"] }] },
    preexisting_failures: [{ command: { program: "python", arguments: ["-m", "pytest"] }, test_run_id: "p1", exit_code: 1, failed_tests: ["pytest:legacy/test_reports.py::test_v2"], reason: "Every failing test also failed on the untouched base." }],
    diff: "", report: "", pr_title: "", pr_body: "", submission_token: "", pr_url: "",
  } }));
  await page.goto(`/contributions/${id}`);
  await page.getByRole("tab", { name: /^Tests/ }).click();
  const summary = page.getByRole("region", { name: "Baseline on the approved base" });
  await expect(summary).toContainText("4fe48cf9eb17");
  await expect(summary).toContainText("Every failing test also failed on the untouched base.");
  await expect(page.getByText("Baseline ·", { exact: false })).toHaveCount(1);
});
