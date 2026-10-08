import { test, expect } from "@playwright/test";
import path from "node:path";
import { fileURLToPath } from "node:url";

test.use({ timezoneId: "Asia/Kolkata" });

test("rate-limited preview shows local reset time and never permits execution", async ({
  page,
}) => {
  const examples = await (await page.request.get("/api/opportunities")).json();
  const opportunity = { ...examples[0], id: "quota-ui-fixture", demo: false };
  await page.route("**/api/opportunities", (route) =>
    route.fulfill({ json: [opportunity] }),
  );
  await page.route("**/api/discovery", (route) =>
    route.fulfill({
      json: {
        available: true,
        running: false,
        automatic: false,
        authentication: "public unauthenticated",
        last_run: null,
        retry_at: null,
      },
    }),
  );
  await page.route("**/api/opportunities/quota-ui-fixture/preview", (route) =>
    route.fulfill({
      status: 429,
      json: {
        error: "GitHub rate limit reached; retry after 2026-10-06T21:06:19Z",
      },
    }),
  );
  let executions = 0;
  await page.route("**/api/opportunities/quota-ui-fixture/proceed", (route) => {
    executions++;
    return route.fulfill({
      status: 500,
      json: { error: "Unexpected approval" },
    });
  });
  await page.goto("/opportunities");
  await page
    .getByRole("button", { name: "Inspect " + opportunity.repository })
    .click();
  await page.getByRole("button", { name: "Proceed to Contribute" }).click();
  await expect(page.getByRole("alert")).toContainText("2:36:19");
  await expect(page.getByRole("alert")).not.toContainText(
    "2026-10-06T21:06:19Z",
  );
  await expect(
    page.getByText(/Sign in from the GitHub account page/),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Approve and start contribution" }),
  ).toHaveCount(0);
  expect(executions).toBe(0);
});

test("workspace approval requires fresh preview and explicit confirmation", async ({
  page,
}) => {
  // UI contract fixture only. Backend integration tests exercise actual Git.
  const original = await page.request.get("/api/opportunities");
  const examples = await original.json();
  const o = { ...examples[0], id: "approval-ui-fixture", demo: false };
  await page.route("**/api/opportunities", (r) => r.fulfill({ json: [o] }));
  await page.route("**/api/runtime/models", (r) => r.fulfill({ json: { models: [{ slug: "model-a", display_name: "Model A" }] } }));
  await page.route("**/api/opportunities/approval-ui-fixture/preview", (r) => {
    expect(r.request().postDataJSON().codex_model).toBe("model-a");
    return r.fulfill({
      json: {
        token: "f".repeat(64),
        codex_model: "model-a",
        configuration: { version: 1 },
        base_commit: "a".repeat(40),
        checked_at: new Date().toISOString(),
        issue: {
          title: "Fresh approval fixture",
          body: "Current issue body",
          html_url: "https://github.com/example/project/issues/42",
        },
        warnings: ["Preparation only; coding adapter pending."],
      },
    });
  });
  let approvals = 0;
  await page.route(
    "**/api/opportunities/approval-ui-fixture/proceed",
    async (r) => {
      const b = r.request().postDataJSON();
      expect(b.approved).toBe(true);
      expect(b.token).toBe("f".repeat(64));
      expect(b.codex_model).toBe("model-a");
      approvals++;
      await r.fulfill({
        status: 202,
        json: { id: "fixture", state: "SELECTED" },
      });
    },
  );
  await page.goto("/opportunities");
  await page.getByRole("button", { name: "Inspect " + o.repository }).click();
  await page.getByLabel("Codex model for this contribution").selectOption("model-a");
  await page.getByRole("button", { name: "Proceed to Contribute" }).click();
  await expect(page.getByText("model-a", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Fresh approval fixture" }),
  ).toBeVisible();
  const confirm = page.getByRole("button", {
    name: "Approve and start contribution",
  });
  await expect(confirm).toBeDisabled();
  expect(approvals).toBe(0);
  await page
    .getByRole("checkbox", {
      name: "I approve cloning, Codex coding, verification and review in this external workspace.",
    })
    .check();
  await expect(confirm).toBeEnabled();
  await confirm.scrollIntoViewIfNeeded();
  const artifacts = fileURLToPath(
    new URL("../../../artifacts/", import.meta.url),
  );
  await page.screenshot({
    path: path.join(artifacts, "workspace-approval-desktop.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await confirm.scrollIntoViewIfNeeded();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: path.join(artifacts, "workspace-approval-mobile.png"),
    fullPage: true,
  });
  await confirm.click();
  await expect(page).toHaveURL(/\/contributions$/);
  expect(approvals).toBe(1);
});
