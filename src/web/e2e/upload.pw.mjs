// Upload page (F-11, UP-1, UP-8): reserved drop zone (no layout shift), paused notice, fail-open status, one upload.
import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import { open } from './helpers.mjs';
import { fitPath } from './fixtures.mjs';

const box = (page, sel) => page.locator(sel).evaluate((e) => { const r = e.getBoundingClientRect(); return { x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height) }; });

test('F-11: the drop zone keeps its place while /api/status is pending (no layout shift)', async ({ page }) => {
  let release;
  const gate = new Promise((r) => { release = r; });
  await page.route('**/api/status', async (route) => { await gate; await route.continue(); });
  await open(page, '/', { wait: false });
  await expect(page.locator('#page')).toHaveAttribute('data-state', 'checking');
  const before = await box(page, '#drop');
  const linkBefore = await box(page, '.mine-link');
  expect(before.h).toBeGreaterThan(80); // reserved, not collapsed
  await expect(page.locator('#drop')).toBeHidden(); // hidden but occupying its box (visibility)
  release();
  await expect(page.locator('#page')).toHaveAttribute('data-state', 'ready');
  await expect(page.locator('#drop')).toBeVisible();
  expect(await box(page, '#drop')).toEqual(before);
  expect(await box(page, '.mine-link')).toEqual(linkBefore);
});

test('UP-8: storage full shows the paused notice and blocks picking', async ({ page }) => {
  await page.route('**/api/status', (r) => r.fulfill({ json: { uploads_open: false } }));
  await open(page, '/', { wait: false });
  await expect(page.locator('#page')).toHaveAttribute('data-state', 'paused');
  await expect(page.locator('#notice')).toBeVisible();
  await expect(page.locator('#file')).toBeDisabled();
});

test('U-5: a failing status endpoint fails open', async ({ page }) => {
  await page.route('**/api/status', (r) => r.fulfill({ status: 500, body: 'no' }));
  await open(page, '/', { wait: false });
  await expect(page.locator('#page')).toHaveAttribute('data-state', 'ready');
  await expect(page.locator('#file')).toBeEnabled();
});

test('UP-1: a file that is not FIT is refused with a message, the page stays usable', async ({ page }) => {
  await open(page, '/', { wait: false });
  await expect(page.locator('#page')).toHaveAttribute('data-state', 'ready');
  await page.locator('#file').setInputFiles({ name: 'notes.fit', mimeType: 'application/octet-stream', buffer: Buffer.from('this is not a FIT file at all') });
  await expect(page.locator('#message')).not.toBeEmpty();
  await expect(page.locator('#page')).toHaveAttribute('data-state', 'ready');
});

test('UP-1: choosing a FIT file uploads it and opens the edit page of the draft', async ({ page }) => {
  await open(page, '/', { wait: false });
  await expect(page.locator('#page')).toHaveAttribute('data-state', 'ready');
  await page.locator('#file').setInputFiles({ name: 'ride-30min.fit', mimeType: 'application/octet-stream', buffer: fs.readFileSync(fitPath('ride-30min')) });
  await page.waitForURL(/\/edit\/[0-9A-Za-z]{22}#/);
  await expect(page.locator('.chart-stack canvas').first()).toBeVisible();
});
