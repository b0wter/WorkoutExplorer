// An invalid navigator.language (Docker Chromium: en-US@posix) made Intl.NumberFormat throw in uPlot and left every page blank.
import { test, expect } from '@playwright/test';
import { workout, open } from './helpers.mjs';

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(Navigator.prototype, 'language', { get: () => 'en-US@posix', configurable: true });
    Object.defineProperty(Navigator.prototype, 'languages', { get: () => ['en-US@posix'], configurable: true });
  });
});

test('invalid navigator.language: viewing page renders, no page errors', async ({ page }) => {
  const errors = await open(page, workout('ride-30min').view);
  await expect(page.locator('.chart-stack canvas').first()).toBeVisible();
  expect(errors).toEqual([]);
});

test('invalid navigator.language: edit page renders, no page errors', async ({ page }) => {
  const errors = await open(page, workout('ride-30min').edit);
  await expect(page.locator('.chart-stack canvas').first()).toBeVisible();
  expect(errors).toEqual([]);
});
