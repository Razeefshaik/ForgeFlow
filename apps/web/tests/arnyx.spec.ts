import { test, expect } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";

test("Arnyx drags without opening, stays on screen, and opens with the keyboard", async ({
  page,
}) => {
  await page.goto("/");
  const bot = page.getByRole("button", {
    name: "Chat with Arnyx",
    exact: true,
  });
  const before = await bot.boundingBox();
  expect(before).toBeTruthy();
  await page.mouse.move(before!.x + 38, before!.y + 38);
  await page.mouse.down();
  await page.mouse.move(before!.x - 180, before!.y - 120, { steps: 10 });
  await page.mouse.up();
  await expect(
    page.getByRole("dialog", { name: "Arnyx", exact: true }),
  ).toHaveCount(0);
  const after = await bot.boundingBox();
  expect(after!.x).toBeLessThan(before!.x - 100);
  await bot.focus();
  await page.keyboard.press("ArrowLeft");
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("dialog", { name: "Arnyx", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Message Arnyx" }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(bot).toBeFocused();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect
    .poll(async () => {
      const resized = await bot.boundingBox();
      return resized!.x + resized!.width;
    })
    .toBeLessThanOrEqual(390);
});

test("Arnyx preserves conversation across closing and navigation without applying config", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Arnyx", exact: true }).click();
  const before = await (await page.request.get("/api/config")).json();
  const input = page.getByRole("textbox", { name: "Message Arnyx" });
  await input.fill("What are my current languages?");
  await page.getByRole("button", { name: "Send message" }).click();
  await expect(page.getByRole("log")).toContainText("Active configuration:");
  await page.keyboard.press("Escape");
  await page.getByRole("link", { name: "Contributions", exact: true }).click();
  await page
    .getByRole("button", { name: "Chat with Arnyx", exact: true })
    .click();
  await expect(page.getByRole("log")).toContainText(
    "What are my current languages?",
  );
  const after = await (await page.request.get("/api/config")).json();
  expect(after.version).toBe(before.version);
  await expect(
    page.getByText("Offline assistant", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "New chat" }).click();
  await expect(page.getByRole("log")).not.toContainText(
    "Active configuration:",
  );
  await page.getByRole("button", { name: "Previous chats" }).click();
  await page.locator(".arnyx-history").getByRole("button", { name: /What are my current languages\?/ }).first().click();
  await expect(page.getByRole("log")).toContainText("Active configuration:");
  await page.reload();
  await page.getByRole("button", { name: "Chat with Arnyx", exact: true }).click();
  await expect(page.getByRole("log")).toContainText("What are my current languages?");
  await expect(page.getByRole("log")).toContainText("Active configuration:");
});

test("Arnyx sends only bounded conversation and page focus, then renders a controlled reply", async ({
  page,
}) => {
  let payload:
    | {
        message: string;
        history: { role: string; content: string }[];
        view: { route: string };
      }
    | undefined;
  await page.route("**/api/arnyx/chat", async (route) => {
    payload = route.request().postDataJSON();
    await route.fulfill({
      json: {
        action: "answer",
        engine: "codex",
        context_bytes: 2300,
        message: "The current configuration is ready for review.",
      },
    });
  });
  await page.goto("/configuration");
  await page.getByRole("button", { name: "Chat with Arnyx" }).click();
  const input = page.getByRole("textbox", { name: "Message Arnyx" });
  await input.fill("Help me tune languages");
  await page.getByRole("button", { name: "Send message" }).click();
  await expect(page.getByRole("log")).toContainText("ready for review");
  await input.fill("Keep the other settings");
  await page.getByRole("button", { name: "Send message" }).click();
  await expect.poll(() => payload?.message).toBe("Keep the other settings");
  expect(payload?.view.route).toBe("/configuration");
  expect(payload?.history).toHaveLength(2);
  expect(Object.keys(payload!)).toEqual(["message", "history", "view", "conversation_id"]);
});

test("loading an earlier saved snapshot preserves a just-returned proposal", async ({ page }) => {
  const answer = "Proposed adding Rust. Review the configuration proposal.";
  await page.route("**/api/arnyx/chat", route => route.fulfill({json:{action:"proposeConfigChange",message:answer,proposal:{id:"fixture-proposal"}}}));
  await page.route("**/api/arnyx/conversations/**", async route => {
    if (route.request().method() !== "GET") return route.continue();
    await expect(page.getByText(answer,{exact:true})).toBeVisible();
    await route.fulfill({json:{messages:[{sequence:777,role:"user",text:"Add Rust"}],tasks:[]}});
  });
  await page.goto("/arnyx");
  await page.getByLabel("Message Arnyx").fill("Add Rust");
  await page.getByRole("button",{name:"Send message"}).click();
  const review = page.getByRole("link",{name:/Review configuration proposal/});
  await expect(review).toBeVisible();
  await expect(page.getByText("Loading saved conversation…",{exact:true})).toHaveCount(0);
  await expect(review).toBeVisible();
  await expect(page.getByText(answer,{exact:true})).toHaveCount(1);
});

test("Arnyx workspace approvals persist, then show activity and a stop control", async ({ page }) => {
  let task = { id: "ui-scoped-task", conversation_id: "", directory: "S:/fixtures/project", mode: "edit", prompt: "Fix the parser and run its tests", network: false, status: "AWAITING_APPROVAL", activity: "Review this request", updated_at: new Date().toISOString() };
  let approved = false;
  let stopped = false;
  await page.route("**/api/runtime", async route => {
    const response = await route.fetch();
    await route.fulfill({ json: { ...(await response.json()), mode: "live", codex_available: true, arnyx_workspace_available: true, project_root: "S:/fixtures/project" } });
  });
  await page.route("**/api/arnyx/conversations/**", async route => {
    const url = new URL(route.request().url());
    if (route.request().method() === "POST" && url.pathname.endsWith("/tasks")) {
      const body = route.request().postDataJSON();
      expect(body.mode).toBe("edit");expect(body.directory).toBe("S:/fixtures/project");expect(body.network).toBe(false);
      task.conversation_id = url.pathname.split("/").at(-2)!;
      await route.fulfill({ json: task });
    } else if (route.request().method() === "GET" && task.conversation_id && url.pathname.endsWith(task.conversation_id)) {
      await route.fulfill({ json: { messages: [{ sequence: 1, role: "user", text: task.prompt }], tasks: [task] } });
    } else await route.continue();
  });
  await page.route("**/api/arnyx/tasks/ui-scoped-task/*", async route => {
    if (route.request().url().endsWith("/approve")) {
      expect(route.request().postDataJSON()).toEqual({approved: true});approved = true;
      task = {...task, status: "RUNNING", activity: "Reading parser.ts"};
    } else {stopped = true;task = {...task, status: "CANCELLED", activity: "Task stopped"};}
    await route.fulfill({ json: task });
  });
  await page.goto("/arnyx");
  await page.getByLabel("Arnyx access mode").selectOption("edit");
  await page.getByLabel("Arnyx workspace folder").fill("S:/fixtures/project");
  await page.getByLabel("Message Arnyx").fill(task.prompt);
  await page.getByRole("button", {name:"Send message"}).click();
  const approval = page.getByRole("button", {name:"Approve changes in this folder"});
  await expect(approval).toBeVisible();expect(approved).toBe(false);
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await expect(approval).toBeVisible();
  await page.reload();
  await expect(approval).toBeVisible();expect(approved).toBe(false);
  await approval.click();
  await expect(page.getByText("Reading parser.ts", {exact:true})).toBeVisible();
  await page.getByRole("button", {name:"Stop Codex task"}).click();
  await expect(page.getByRole("button", {name:"Stop Codex task"})).toHaveCount(0);
  expect(approved).toBe(true);expect(stopped).toBe(true);
});

for (const mobile of [false, true])
  test(`Arnyx ${mobile ? "mobile" : "desktop"} visual and reduced motion`, async ({
    page,
  }) => {
    if (mobile) await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/contributions");
    await page.getByRole("button", { name: "Chat with Arnyx" }).click();
    const panel = page.getByRole("dialog", { name: "Arnyx", exact: true });
    await expect(panel).toBeVisible();
    const box = await panel.boundingBox();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.y).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(mobile ? 390 : 1440);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    const image = panel.locator(".arnyx-header-avatar img");
    await expect
      .poll(() => image.evaluate((el) => (el as HTMLImageElement).naturalWidth))
      .toBeGreaterThan(0);
    if (!mobile)
      await expect
        .poll(() =>
          page
            .locator(".main-shell")
            .evaluate((el) =>
              Math.round(parseFloat(getComputedStyle(el).marginLeft)),
            ),
        )
        .toBe(242);
    if (process.env.FORGEFLOW_SCREENSHOTS) {
      fs.mkdirSync(process.env.FORGEFLOW_SCREENSHOTS, { recursive: true });
      await page.screenshot({
        path: path.join(
          process.env.FORGEFLOW_SCREENSHOTS,
          `arnyx-${mobile ? "mobile" : "desktop"}.png`,
        ),
        fullPage: true,
      });
    }
    await page.emulateMedia({ reducedMotion: "reduce" });
    await expect
      .poll(() => panel.evaluate((el) => getComputedStyle(el).animationName))
      .toBe("none");
    await expect
      .poll(() =>
        page
          .locator(".arnyx-companion > img")
          .evaluate((el) => getComputedStyle(el).animationName),
      )
      .toBe("none");
  });
