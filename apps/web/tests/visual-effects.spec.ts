import { test, expect } from "@playwright/test";

test("cards are borderless until hovered, follow the pointer, and restore their clean edge", async ({ page }) => {
  await page.goto("/");
  const card = page.locator(".metric.glow-card").first();
  await expect(card).toBeVisible();
  await page.mouse.move(10, 10);
  const rim = () => card.evaluate(el => getComputedStyle(el, "::after").opacity);
  await expect.poll(rim).toBe("0");
  expect(await card.evaluate(el => getComputedStyle(el).borderTopColor)).toBe("rgba(0, 0, 0, 0)");
  await card.hover({ position: { x: 30, y: 30 } });
  await expect.poll(rim).toBe("0.8");
  const pointerX = () => card.evaluate(el => parseFloat((el as HTMLElement).style.getPropertyValue("--pointer-x")));
  expect(await pointerX()).toBeGreaterThanOrEqual(30);
  expect(await pointerX()).toBeLessThanOrEqual(32);
  await card.hover({ position: { x: 130, y: 30 } });
  await expect.poll(pointerX).toBeGreaterThan(125);
  const gradient = await card.evaluate(el => getComputedStyle(el, "::after").backgroundImage);
  expect(gradient).toContain("linear-gradient");
  await page.mouse.move(10, 10);
  await expect.poll(rim).toBe("0");
});

test("scroll reveals each card once and suspends artwork outside the viewport", async ({ page }) => {
  await page.goto("/");
  const art = page.locator(".hero-art");
  await expect(art).toHaveAttribute("data-in-view", "");
  expect(await art.locator("img").evaluate(el => getComputedStyle(el).animationPlayState)).toBe("running");
  const bottom = page.locator("main .glow-card").last();
  await bottom.scrollIntoViewIfNeeded();
  await expect(bottom).toHaveAttribute("data-revealed", "true");
  await expect(art).not.toHaveAttribute("data-in-view", "");
  expect(await art.locator("img").evaluate(el => getComputedStyle(el).animationPlayState)).toBe("paused");
  await expect.poll(() => bottom.evaluate(el => el.getAnimations().length)).toBe(0);
  await page.evaluate(() => scrollTo(0, 0));
  await bottom.scrollIntoViewIfNeeded();
  expect(await bottom.evaluate(el => el.getAnimations().length)).toBe(0);
});

test("pause preference survives navigation and reload without trapping modal focus", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Pause animations", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-motion", "paused");
  expect(await page.locator(".hero-art img").evaluate(el => getComputedStyle(el).animationName)).toBe("none");
  await page.getByRole("button", { name: "Getting started", exact: true }).click();
  await page.getByRole("button", { name: "Close dialog", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Getting started", exact: true })).toBeFocused();
  await page.locator(".sidebar nav").getByRole("link", { name: "Usage & effort", exact: true }).click();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("data-motion", "paused");
  await page.getByRole("button", { name: "Resume animations", exact: true }).click();
  await expect(page.locator("html")).toHaveAttribute("data-motion", "full");
});

test("touch has no persistent hover rim and reduced motion also disables scroll and artwork effects", async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 390, height: 844 }, hasTouch: true, reducedMotion: "reduce" });
  const page = await context.newPage();
  try {
    await page.goto(test.info().project.use.baseURL!);
    const card = page.locator(".metric.glow-card").first();
    await card.tap();
    expect(await card.evaluate(el => getComputedStyle(el, "::after").opacity)).toBe("0");
    expect(await page.locator(".hero-art img").evaluate(el => getComputedStyle(el).animationName)).toBe("none");
    expect(await page.locator(".hero-art").evaluate(el => getComputedStyle(el).transform)).toBe("none");
    expect(await page.evaluate(() => document.getAnimations().filter(a => a.playState === "running").length)).toBe(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  } finally { await context.close(); }
});
