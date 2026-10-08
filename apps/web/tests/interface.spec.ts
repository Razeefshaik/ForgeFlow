import { test, expect } from "@playwright/test";

test("system appearance follows the OS and explicit preference survives reload", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/settings");
  await page.getByLabel("Color theme").selectOption("system");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  await page.getByRole("button", { name: "Use light theme" }).click();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await expect(page.getByLabel("Color theme")).toHaveValue("light");
});

test("inline opportunity inspection changes evidence without opening a modal", async ({
  page,
}) => {
  const issues = await (await page.request.get("/api/opportunities")).json();
  await page.goto("/opportunities");
  const selected = page.getByRole("region", {
    name: "Selected opportunity",
    exact: true,
  });
  await expect(
    selected.getByRole("heading", { name: issues[0].title, exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", {
      name: "Inspect " + issues[1].repository,
      exact: true,
    })
    .click();
  await expect(
    selected.getByRole("heading", { name: issues[1].title, exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page
    .getByLabel("Search opportunities")
    .fill("no-matching-repository-123");
  await expect(
    page.getByRole("heading", { name: "No opportunities found" }),
  ).toBeVisible();
  await expect(selected).toHaveCount(0);
});

test("activity filtering searches persisted events without sending a mutation", async ({
  page,
}) => {
  let mutations = 0;
  page.on("request", (request) => {
    if (request.url().includes("/api/") && request.method() !== "GET")
      mutations++;
  });
  await page.goto("/activity");
  await page.getByLabel("Search activity").fill("no-matching-event-123");
  await expect(
    page.getByRole("heading", { name: "No matching events" }),
  ).toBeVisible();
  await page.getByLabel("Search activity").clear();
  await expect(page.locator(".event-row").first()).toBeVisible();
  expect(mutations).toBe(0);
});

test("running state has unknown completion while command history has measured results", async ({
  page,
}) => {
  const c = {
    id: "meter-ui",
    repository: "fixture/meters",
    title: "Verify progress semantics",
    state: "TESTING",
    branch: "autopilot/issue-1",
    config_version: 1,
    demo: false,
  };
  await page.route("**/api/contributions/meter-ui", (route) =>
    route.fulfill({ json: c }),
  );
  await page.route("**/api/contributions/meter-ui/execution", (route) =>
    route.fulfill({
      json: {
        status: "RUNNING",
        phase: "TESTING",
        summary: "",
        fix_iterations: 0,
        review_cycles: 0,
      },
    }),
  );
  await page.route("**/api/contributions/meter-ui/tests", (route) =>
    route.fulfill({
      json: [0, 1].map((exit_code, i) => ({
        id: "meter-test-" + i,
        command: { program: "go", arguments: ["test", "./..."] },
        exit_code,
        output: "UI contract fixture",
        started_at: "2026-10-07T10:00:00Z",
        finished_at: "2026-10-07T10:00:02Z",
        truncated: false,
      })),
    }),
  );
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/contributions/meter-ui");
  const ongoing = page.getByRole("progressbar", {
    name: "Execution in progress",
  });
  await expect(ongoing).toBeVisible();
  await expect(ongoing).not.toHaveAttribute("aria-valuenow");
  await expect(ongoing).toHaveAttribute(
    "aria-valuetext",
    /completion percentage unknown/,
  );
  await expect(
    page.getByRole("progressbar", { name: "Successful recorded command runs" }),
  ).toHaveAttribute("aria-valuenow", "50");
  expect(
    await ongoing
      .locator("span")
      .evaluate((el) => getComputedStyle(el).animationName),
  ).toBe("none");
  await page.getByRole("tab", { name: "Plan", exact: true }).focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: "Tests (2)" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page.getByRole("tabpanel", { name: "Tests (2)" })).toBeVisible();
});
