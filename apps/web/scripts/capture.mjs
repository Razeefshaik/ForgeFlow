import { chromium } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
const root = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../..",
);
const out = path.join(root, "artifacts");
fs.mkdirSync(out, { recursive: true });
const browser = await chromium.launch({
  channel: process.platform === "win32" ? "msedge" : undefined,
  headless: true,
});
try {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1000 },
  });
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  for (const [route, name] of [
    ["/", "overview-desktop"],
    ["/opportunities", "opportunities-desktop"],
    ["/configuration", "configuration-desktop"],
  ]) {
    await page.goto("http://127.0.0.1:5173" + route);
    await page.getByRole("heading", { level: 1 }).waitFor();
    await page.getByText("DEMO WORKSPACE", { exact: false }).waitFor();
    if (route === "/" || route === "/opportunities")
      await page.locator("tbody tr").first().waitFor();
    if (route === "/configuration") {
      await page.getByLabel("Configuration JSON", { exact: false }).waitFor();
      await page.waitForFunction(() =>
        document.getElementById("config-draft")?.value.includes('"languages"'),
      );
    }
    await page.screenshot({
      path: path.join(out, name + ".png"),
      fullPage: true,
    });
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("http://127.0.0.1:5173/");
  await page.getByRole("heading", { name: "Good work starts here." }).waitFor();
  await page.locator("tbody tr").first().waitFor();
  await page.screenshot({
    path: path.join(out, "overview-mobile.png"),
    fullPage: true,
  });
  if (
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    )
  )
    throw new Error("Mobile page overflow");
  if (errors.length) throw new Error(errors.join("\n"));
  console.log(
    "Captured four dashboard screenshots; no page errors or mobile page overflow.",
  );
} finally {
  await browser.close();
}
