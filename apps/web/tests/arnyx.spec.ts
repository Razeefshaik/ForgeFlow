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
  await page.getByRole("button", { name: "Clear chat" }).click();
  await expect(page.getByRole("log")).not.toContainText(
    "Active configuration:",
  );
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
  expect(Object.keys(payload!)).toEqual(["message", "history", "view"]);
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
