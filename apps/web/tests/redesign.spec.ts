import { test, expect } from '@playwright/test';

test('mobile navigation closes when selecting the current page', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  await page.getByRole('button', { name: 'Open navigation', exact: true }).click();
  await expect(page.locator('.sidebar')).toHaveClass(/sidebar-open/);
  await page.getByRole('link', { name: 'Overview', exact: true }).click();
  await expect(page.locator('.sidebar')).not.toHaveClass(/sidebar-open/);
});

test('overview contribution links open its workspace and survive reload', async ({ page }) => {
  await page.goto('/');
  const link = page.locator('.workspace-work .contribution-row').first();
  const destination = await link.getAttribute('href');
  await link.click();
  await expect(page).toHaveURL(new RegExp('/contributions/[^/]+$'));
  await expect(page.getByLabel('Contribution workspace', { exact: true })).toBeVisible();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByLabel('Contribution stages')).toBeVisible();
  await expect(page.locator('.workspace-inspector')).not.toHaveAttribute('open');
  await page.reload();
  await expect(page.getByLabel('Contribution workspace', { exact: true })).toBeVisible();
  await page.getByText('Workspace details · Branch, location and configuration').click();
  await expect(page.getByText('Contribution ID', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Back to contributions' }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page).toHaveURL(/\/contributions$/);
  expect(destination).toMatch(/^\/contributions\/[^/]+$/);
});

test('long contribution data fits mobile and reduced-motion workspace', async ({ page }) => {
  const contribution = { id: 'layout-fixture', repository: 'organization/' + 'long-repository-name'.repeat(12), title: 'A detailed issue with a long title '.repeat(10), state: 'BLOCKED', branch: 'autopilot/issue-42', config_version: 1, demo: true, workspace: 'S:/external/' + 'long-path-segment/'.repeat(20) + 'repo' };
  await page.route('**/api/contributions/layout-fixture', route => route.fulfill({ json: contribution }));
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto('/contributions?contribution=layout-fixture');
  await expect(page).toHaveURL(/\/contributions\/layout-fixture$/);
  await expect(page.getByLabel('Contribution workspace', { exact: true })).toBeVisible();
  await page.locator('.workspace-inspector summary').click();
  const geometry = await page.locator('.contribution-page').evaluate(el => ({ width: el.clientWidth, scrollWidth: el.scrollWidth, animation: getComputedStyle(el).animationName }));
  expect(geometry.scrollWidth).toBeLessThanOrEqual(geometry.width);
  expect(geometry.animation).toBe('none');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('missing contribution has a recovery link and no execution controls', async ({ page }) => {
  await page.route('**/api/contributions/missing-ui', route => route.fulfill({ status: 404, json: { error: 'Contribution does not exist' } }));
  await page.goto('/contributions/missing-ui');
  await expect(page.getByRole('heading', { name: 'Contribution not found' })).toBeVisible();
  await expect(page.locator('.execution-panel')).toHaveCount(0);
  await page.getByRole('link', { name: 'Back to contributions' }).click();
  await expect(page).toHaveURL(/\/contributions$/);
});
