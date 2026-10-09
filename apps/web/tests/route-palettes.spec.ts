import { test, expect } from "@playwright/test";

const screens = [
  ["/", "Overview", "cosmic-orchid"],
  ["/opportunities", "Opportunities", "ember-rose"],
  ["/contributions", "Contributions", "aurora-jade"],
  ["/agents", "Agents", "electric-indigo"],
  ["/activity", "Activity", "velvet-garnet"],
  ["/usage", "Usage & effort", "obsidian-gold"],
  ["/configuration", "Configuration", "velvet-garnet"],
  ["/settings", "Settings", "aurora-jade"],
  ["/login", "GitHub account", "ember-rose"],
] as const;

// Read-only route checks. Approval and execution behavior is covered by the
// existing fixture tests; these checks never launch a contribution.
for (const appearance of ["dark", "light"]) {
  test(`navigation recolors the complete shell with both palette colors in ${appearance}`, async ({ page }) => {
    await page.addInitScript(value => localStorage.setItem("forgeflow-theme", value), appearance);
    await page.goto("/");
    const pairs = new Map<string, string>();
    let previousSidebar = "";
    for (const [route, label, palette] of screens) {
      await page.locator(".sidebar nav").getByRole("link", { name: label, exact: true }).click();
      await expect(page).toHaveURL(new RegExp(route === "/" ? "/$" : route + "$"));
      await expect(page.locator("html")).toHaveAttribute("data-palette", palette);
      await expect(page.locator(".sidebar nav a.active")).toHaveAttribute("aria-label", label);
      const color = await page.evaluate(() => {
        const root = getComputedStyle(document.documentElement);
        const probe = document.createElement("span"); document.body.appendChild(probe);
        const normalize = (token: string) => { probe.style.color = `var(${token})`; return getComputedStyle(probe).color; };
        const first = normalize("--accent"), second = normalize("--accent-soft"), success = normalize("--green");
        probe.remove();
        const bg = (selector: string, pseudo?: string) => getComputedStyle(document.querySelector(selector)!, pseudo).backgroundImage;
        return { first, second, success, base: root.getPropertyValue("--bg"), sidebar: bg(".sidebar"), toolbar: bg(".topbar"), brand: bg(".brand-mark"), active: bg(".sidebar nav a.active"), glow: bg(".main-shell", "::before"), blur: bg(".main-shell", "::after") };
      });
      expect(color.first).not.toBe(color.second);
      // Strongest direct mix is the brand. All other paired surfaces must
      // contain two gradient stops, change with the route, and retain the base.
      expect(color.brand).toContain(color.first); expect(color.brand).toContain(color.second);
      for (const field of ["sidebar", "toolbar", "active", "glow", "blur"] as const) {
        expect(color[field], field).toContain("gradient"); expect(color[field]).not.toContain("rgba(0, 0, 0, 0), rgba(0, 0, 0, 0)");
      }
      if (previousSidebar) expect(color.sidebar).not.toBe(previousSidebar);
      previousSidebar = color.sidebar;
      pairs.set(palette, color.first + color.second);
      expect(color.success).toBe(appearance === "dark" ? "rgb(99, 230, 179)" : "rgb(23, 115, 79)");
      expect(color.base.trim()).toBe(appearance === "dark" ? "#0d0e12" : "#f5f5fa");
      await page.getByRole("button", { name: "Open command search" }).click();
      const dialog = page.getByRole("dialog"); await expect(dialog).toBeVisible();
      const portalPair = await dialog.evaluate(el => {
        const style = getComputedStyle(el);
        return [style.getPropertyValue("--accent").trim(), style.getPropertyValue("--accent-soft").trim(), style.backgroundImage];
      });
      expect(portalPair[2]).toContain("gradient");
      const rootPair = await page.locator("html").evaluate(el => [getComputedStyle(el).getPropertyValue("--accent").trim(), getComputedStyle(el).getPropertyValue("--accent-soft").trim()]);
      expect(portalPair.slice(0, 2)).toEqual(rootPair);
      await page.keyboard.press("Escape");
    }
    expect(new Set(pairs.values()).size).toBe(6);
    await page.goBack();
    await expect(page.locator("html")).toHaveAttribute("data-palette", "aurora-jade");
    await page.reload();
    await expect(page.locator("html")).toHaveAttribute("data-palette", "aurora-jade");
  });
}

test("mobile drawer follows the selected route and workspace details use the execution pair", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  for (const [, label, palette] of screens) {
    await page.getByRole("button", { name: "Open navigation", exact: true }).click();
    await page.locator(".sidebar nav").getByRole("link", { name: label, exact: true }).click();
    await expect(page.locator("html")).toHaveAttribute("data-palette", palette);
    await expect(page.locator(".sidebar")).toHaveAttribute("inert", "");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await page.route("**/api/contributions/palette-workspace", r => r.fulfill({ json: { id: "palette-workspace", repository: "fixture/palette", title: "Inspect saved execution", state: "READY", branch: "fixture/palette", config_version: 1, demo: false } }));
  await page.route("**/api/contributions/palette-workspace/execution", r => r.fulfill({ json: { status: "READY", phase: "READY", summary: "Stored UI contract fixture", fix_iterations: 0, review_cycles: 0 } }));
  await page.goto("/contributions/palette-workspace");
  await expect(page.locator("html")).toHaveAttribute("data-palette", "electric-indigo");
  await expect(page.locator(".sidebar nav a.active")).toHaveAttribute("aria-label", "Contributions");
  expect(await page.locator(".execution-commandbar").evaluate(el => getComputedStyle(el).backgroundImage)).toContain("gradient");
});
