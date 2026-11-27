// B-11 (F-39 how-to bar, F-58 numbers dialog, F-40 shareable range), on ride-30min.
import { test, expect } from '@playwright/test';
import { workout, open, drag } from './helpers.mjs';

const METRIC = { storage: { 'wx.units': 'metric' } };
const rangeCells = (page) => page.locator('.range-panel .cell .val').allTextContents();

test.describe('how-to bar and numbers dialog', () => {
  test('F-39: the first-visit hint shows, x dismisses it, it stays dismissed', async ({ page }) => {
    const errors = await open(page, workout('ride-30min').view, METRIC);
    const bar = page.locator('.charts .howto');
    await expect(bar).toBeVisible();
    await expect(bar).toContainText(/select a range/i);
    await bar.locator('.hint-x').click();
    await expect(bar).toHaveCount(0);
    await page.reload();
    await expect(page.locator('.chart-stack canvas').first()).toBeVisible();
    await expect(page.locator('.charts .howto')).toHaveCount(0);
    await expect(page.locator('html')).toHaveAttribute('data-hint', 'done');
    expect(errors).toEqual([]);
  });

  test('F-58: "How these numbers are computed" lists its topics, then closes', async ({ page, isMobile }) => {
    const errors = await open(page, workout('ride-30min').view, METRIC);
    const trigger = page.locator('[data-summary-info]');
    await (isMobile ? trigger.tap() : trigger.click());
    const dlg = page.locator('dialog[open]');
    await expect(dlg).toBeVisible();
    await expect(dlg.locator('.dlg-head')).toContainText('About these numbers');
    const body = dlg.locator('.dlg-body');
    for (const topic of ['Totals', 'Moving time', 'Elapsed time', 'Averages and maximums', 'Range panel and laps', 'Hidden start and end']) {
      await expect(body.locator('b', { hasText: topic })).toBeVisible();
    }
    await expect(body).toContainText(/This workout: .*in /); // an example from the workout's own summary
    await dlg.locator('.dlg-x').click();
    await expect(page.locator('dialog[open]')).toHaveCount(0);
    expect(errors).toEqual([]);
  });
});

test.describe('shareable range', () => {
  test('F-40: a range link opens with the range selected (every project)', async ({ page }) => {
    const errors = await open(page, workout('ride-30min').view + '#range=300-900', METRIC);
    await expect(page.locator('.range-panel')).toBeVisible();
    await expect(page.locator('.range-panel .cell', { hasText: 'Time' }).locator('.val')).toContainText('10:00');
    await expect(page.locator('.chart-area .wx-range-shade').first()).toBeVisible();
    expect(errors).toEqual([]);
  });

  test('F-40: Copy link to this selection, opened in a new page, selects the same range', async ({ page, isMobile }) => {
    await page.addInitScript(() => {
      window.__copied = '';
      Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: (t) => { window.__copied = t; return Promise.resolve(); } } });
    });
    const errors = await open(page, workout('ride-30min').view, METRIC);
    await drag(page, 0.2, 0.6);
    await expect(page.locator('.range-panel')).toBeVisible();
    const before = await rangeCells(page);
    await (isMobile ? page.locator('.range-panel .share').tap() : page.locator('.range-panel .share').click());
    await expect(page.locator('.range-panel .share')).toHaveText('Link copied');
    const url = await page.evaluate(() => window.__copied);
    expect(url).toMatch(/\/w\/[^#]+#range=\d+-\d+$/);
    expect(url).not.toContain('edit');

    const other = await page.context().newPage();
    const errors2 = await open(other, url, METRIC);
    await expect(other.locator('.range-panel')).toBeVisible();
    expect(await rangeCells(other)).toEqual(before);
    expect(errors).toEqual([]);
    expect(errors2).toEqual([]);
  });
});
