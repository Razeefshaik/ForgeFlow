import { test, expect } from "@playwright/test";

// Exercise the real frontend against the disposable demo server used by the suite.
// No backend action, authentication change or contribution execution is performed.
const routes = [
  "/",
  "/opportunities",
  "/contributions",
  "/agents",
  "/activity",
  "/usage",
  "/configuration",
  "/settings",
  "/login",
  "/operator",
];
for (const theme of ["dark", "light"]) {
  for (const width of [1440, 1024, 390]) {
    test(`all pages fit ${width}px in ${theme} theme without mutations`, async ({
      page,
    }) => {
      test.setTimeout(60_000);
      await page.setViewportSize({ width, height: 1000 });
      await page.emulateMedia({ reducedMotion: "reduce" });
      await page.addInitScript(
        (value) => localStorage.setItem("forgeflow-theme", value),
        theme,
      );
      const errors: string[] = [];
      const mutations: string[] = [];
      page.on("pageerror", (error) => errors.push(error.message));
      page.on("request", (request) => {
        if (request.url().includes("/api/") && request.method() !== "GET")
          mutations.push(request.url());
      });
      for (const route of routes) {
        await page.goto(route);
        await expect(page.locator("main h1").first()).toBeVisible();
        await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
        await expect(page.locator(".skeleton")).toHaveCount(0);
        expect(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth,
          ),
          route,
        ).toBe(true);
        if (width === 390) {
          await expect(
            page.getByRole("button", { name: "Open command search" }),
          ).toBeVisible();
          await expect(page.locator(".sidebar")).toHaveAttribute("inert", "");
        }
      }
      expect(errors).toEqual([]);
      expect(mutations).toEqual([]);
    });
  }
}

test("command search filters routes and restores the opener on Escape", async ({
  page,
}) => {
  await page.goto("/");
  const opener = page.getByRole("button", { name: "Open command search" });
  await opener.click();
  await page.getByLabel("Search pages").fill("Usage");
  await expect(page.locator(".command-results button")).toHaveCount(1);
  await page.keyboard.press("Escape");
  await expect(opener).toBeFocused();
  await page.keyboard.press("Control+k");
  await page.getByLabel("Search pages").fill("Settings");
  await page
    .locator(".command-results")
    .getByRole("button", { name: "Settings", exact: true })
    .click();
  await expect(page).toHaveURL(/\/settings$/);
});

test("mobile navigation traps focus and restores its opener", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  const opener = page.getByRole("button", {
    name: "Open navigation",
    exact: true,
  });
  await opener.click();
  await expect(
    page.getByRole("button", { name: "Close navigation", exact: true }).first(),
  ).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(page.locator(".sidebar-bottom > button").last()).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(page.locator(".mobile-close")).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(opener).toBeFocused();
  await expect(page.locator(".sidebar")).toHaveAttribute("inert", "");
});

test("running command shows pending exit and duration instead of a false pass", async ({
  page,
}) => {
  const c = {
    id: "pending-command-ui",
    repository: "fixture/pending",
    title: "Verify pending command presentation",
    // Contribution and execution polling can briefly disagree during a rerun.
    state: "READY",
    branch: "test",
    config_version: 1,
    codex_model: "recorded-model",
    demo: false,
  };
  await page.route("**/api/contributions/pending-command-ui", (route) =>
    route.fulfill({ json: c }),
  );
  await page.route(
    "**/api/contributions/pending-command-ui/execution",
    (route) =>
      route.fulfill({
        json: {
          status: "RUNNING",
          phase: "TESTING",
          review: { verdict: "APPROVE", summary: "Prior review", findings: [] },
          fix_iterations: 0,
          review_cycles: 0,
        },
      }),
  );
  await page.route("**/api/contributions/pending-command-ui/tests", (route) =>
    route.fulfill({
      json: [
        {
          id: "pending",
          command: { program: "go", arguments: ["test", "./..."] },
          exit_code: 0,
          output: "",
          started_at: new Date().toISOString(),
          finished_at: "",
        },
      ],
    }),
  );
  await page.goto("/contributions/pending-command-ui");
  await expect(page.locator(".verification-gate")).toContainText("Verification in progress");
  await expect(page.locator(".verification-summary")).toHaveAttribute("data-verified", "false");
  await page.getByRole("tab", { name: "Tests (1)" }).click();
  await expect(page.locator("#test-run-pending summary")).toContainText(
    "exit pending",
  );
  await expect(page.locator("#test-run-pending summary")).toContainText(
    "duration pending",
  );
  await expect(page.locator("#test-run-pending .test-result")).toHaveText(
    "Running",
  );
  await expect(page.locator("#test-run-pending .test-passed")).toHaveCount(0);
});
