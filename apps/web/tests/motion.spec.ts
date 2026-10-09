import { test, expect } from "@playwright/test";

test("collapsed navigation stays named, restores preference, and exposes hoverable hints", async ({
  page,
}) => {
  await page.goto("/");
  await page
    .getByRole("button", { name: "Collapse navigation", exact: true })
    .click();
  await expect(page.locator(".app")).toHaveAttribute(
    "data-nav-collapsed",
    "true",
  );
  await expect
    .poll(() =>
      page
        .locator(".sidebar")
        .evaluate((el) => Math.round(el.getBoundingClientRect().width)),
    )
    .toBe(80);
  const agents = page.getByRole("link", { name: "Agents", exact: true });
  await agents.hover();
  const hint = page.getByRole("tooltip", { name: "Agents", exact: true });
  await expect(hint).toBeVisible();
  await expect(agents).toHaveAttribute(
    "aria-describedby",
    (await hint.getAttribute("id")) as string,
  );
  const bounds = await hint.boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(8);
  await hint.hover();
  await expect(hint).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(hint).toHaveCount(0);
  await agents.click();
  await expect(page).toHaveURL(/\/agents$/);
  await page.reload();
  await expect(page.locator(".app")).toHaveAttribute(
    "data-nav-collapsed",
    "true",
  );
  const expand = page.getByRole("button", {
    name: "Expand navigation",
    exact: true,
  });
  await expand.focus();
  await page.keyboard.press("Enter");
  await expect(page.locator(".app")).toHaveAttribute(
    "data-nav-collapsed",
    "false",
  );
  expect(
    await page
      .locator(".sidebar")
      .evaluate((el) => getComputedStyle(el).transitionDuration),
  ).toBe("0s");
});

test("pointer overlays have entry motion while command search and keyboard dismissal stay immediate", async ({
  page,
}) => {
  await page.goto("/");
  const help = page.getByRole("button", {
    name: "Getting started",
    exact: true,
  });
  await help.click();
  const dialog = page.getByRole("dialog", { name: "Welcome to ForgeFlow" });
  await expect(dialog).toBeVisible();
  expect(
    await dialog.evaluate((el) => getComputedStyle(el).animationName),
  ).toBe("modal-in");
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(help).toBeFocused();
  // Repeated open/close must release the focus trap and restore the same opener.
  for (let i = 0; i < 2; i++) {
    await help.click();
    await page
      .getByRole("button", { name: "Close dialog", exact: true })
      .click();
    // Changing modality during an exit must not strand Radix Presence.
    if (i === 0) await page.keyboard.press("Shift");
    await expect(dialog).toHaveCount(0);
    await expect(help).toBeFocused();
  }
  const operator = page.getByRole("button", { name: "Operator", exact: true });
  await operator.click();
  const drawer = page.getByRole("dialog", { name: "Operator", exact: true });
  await expect(drawer).toBeVisible();
  expect(
    await drawer.evaluate((el) => getComputedStyle(el).animationName),
  ).toBe("drawer-in");
  await page.keyboard.press("Escape");
  await expect(operator).toBeFocused();
  const search = page.getByRole("button", { name: "Open command search" });
  await search.click();
  const command = page.getByRole("dialog", { name: "Jump to a page" });
  expect(
    await command.evaluate((el) => getComputedStyle(el).animationName),
  ).toBe("none");
  await page.getByLabel("Search pages").fill("Opportunities");
  await expect(page.locator(".command-results button")).toHaveCount(1);
  await page.keyboard.press("Escape");
  await expect(search).toBeFocused();
});

test("mobile touch navigation stays usable and releases its scroll lock", async ({
  browser,
}) => {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    hasTouch: true,
  });
  const page = await context.newPage();
  try {
    await page.goto(test.info().project.use.baseURL!);
    await page
      .getByRole("button", { name: "Open navigation", exact: true })
      .tap();
    await expect(page.locator(".sidebar")).not.toHaveAttribute("inert", "");
    expect(await page.evaluate(() => document.body.style.overflow)).toBe(
      "hidden",
    );
    await page.getByRole("link", { name: "Agents", exact: true }).tap();
    await expect(page).toHaveURL(/\/agents$/);
    await expect(page.locator(".sidebar")).toHaveAttribute("inert", "");
    await expect(page.locator(".menu-backdrop")).toHaveCount(0);
    expect(await page.evaluate(() => document.body.style.overflow)).toBe("");
    await expect(page.getByRole("tooltip")).toHaveCount(0);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  } finally {
    await context.close();
  }
});

test("live activity, native disclosures and filters preserve recorded evidence", async ({
  page,
}) => {
  // API contract fixtures, not product data or actual agent execution.
  const id = "motion-evidence-fixture";
  const now = new Date().toISOString();
  let active = true;
  await page.route(`**/api/contributions/${id}`, (r) =>
    r.fulfill({
      json: {
        id,
        repository: "fixture/motion",
        title: "Verify interaction semantics",
        state: "TESTING",
        branch: "test",
        config_version: 1,
        codex_model: "recorded-fixture-model",
        demo: false,
      },
    }),
  );
  await page.route(`**/api/contributions/${id}/execution`, (r) =>
    r.fulfill({
      json: {
        status: "RUNNING",
        phase: "TESTING",
        fix_iterations: 0,
        review_cycles: 0,
      },
    }),
  );
  await page.route(`**/api/contributions/${id}/tests`, (r) =>
    r.fulfill({
      json: [
        {
          id: "recorded-failure",
          command: { program: "go", arguments: ["test", "./..."] },
          exit_code: 1,
          output: "FAIL: recorded regression output",
          started_at: now,
          finished_at: now,
        },
      ],
    }),
  );
  await page.route("**/api/agents", (r) =>
    r.fulfill({
      json: active
        ? [
            {
              id: "fixture-agent",
              contribution_id: id,
              role: "contributor",
              status: "RUNNING",
              started_at: now,
            },
          ]
        : [],
    }),
  );
  await page.route("**/api/events**", (r) =>
    r.request().url().includes("stream") ? r.abort() : r.fulfill({ json: [] }),
  );
  await page.goto(`/contributions/${id}`);
  const live = page.locator(".live-execution");
  await expect(live).toHaveAttribute("data-process-running", "true");
  await expect(live).toContainText("Waiting for output");
  await expect(live).not.toContainText("100%");
  await page.getByRole("tab", { name: "Tests (1)" }).click();
  const evidence = page.locator("#test-run-recorded-failure");
  await evidence.locator("summary").click();
  await expect(evidence.locator("pre")).toContainText(
    "FAIL: recorded regression output",
  );
  await evidence.locator("summary").click();
  await expect(evidence).not.toHaveAttribute("open", "");
  const filter = page.getByLabel("Filter test results");
  await filter.selectOption("Passed");
  await expect(evidence).toHaveCount(0);
  await filter.selectOption("Failed");
  await expect(evidence).toBeVisible();
  // Native picker keyboard selection still updates the existing React filter.
  await filter.focus();
  await page.keyboard.press("Space");
  await page.keyboard.press("Home");
  await page.keyboard.press("Enter");
  await expect(filter).toHaveValue("All");
  active = false;
  await expect(live).toHaveAttribute("data-process-running", "false", {
    timeout: 10_000,
  });
  await expect(live.locator(".live-pill")).toHaveText("Live activity");
});

test("reduced motion removes overlay, navigation, shimmer and activity animation", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  await page
    .getByRole("button", { name: "Collapse navigation", exact: true })
    .click();
  expect(
    await page
      .locator(".sidebar")
      .evaluate((el) => getComputedStyle(el).transitionDuration),
  ).toBe("0s");
  await page.getByRole("button", { name: "Operator", exact: true }).click();
  const drawer = page.getByRole("dialog", { name: "Operator", exact: true });
  expect(
    await drawer.evaluate((el) => getComputedStyle(el).animationName),
  ).toBe("none");
  await page.keyboard.press("Escape");
  const animated = await page.evaluate(() =>
    Array.from(document.querySelectorAll("*"))
      .filter((el) => getComputedStyle(el).animationName !== "none")
      .map((el) => el.className),
  );
  expect(animated).toEqual([]);
  expect(
    await page
      .locator(".page-transition")
      .evaluate((el) => getComputedStyle(el).transitionDuration),
  ).toBe("0s");
});
