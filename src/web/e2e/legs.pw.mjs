// Multisport leg picker (F-67): a leg switch clears the range; charts without data in a leg stay aligned with the others.
import { test, expect } from '@playwright/test';
import { workout, open, hover, drag, desktopOnly } from './helpers.mjs';

test.describe('legs, mouse', () => {
  test.beforeEach(({ isMobile }) => { test.skip(isMobile, 'mouse scrubbing and range drag'); });

  test('F-67: switching the leg clears the selected range', async ({ page }) => {
    await open(page, workout('triathlon').view);
    await drag(page, 0.01, 0.05);
    await expect(page.locator('.range-panel')).toBeVisible();
    await page.locator('.wx-leg-pick').selectOption({ index: 1 });
    await expect(page.locator('.range-panel')).toBeHidden();
    await drag(page, 0.2, 0.5);
    await expect(page.locator('.range-panel')).toBeVisible();
    await page.locator('.wx-leg-pick').selectOption({ index: 0 });
    await expect(page.locator('.range-panel')).toBeHidden();
  });

  for (const units of ['metric', 'imperial']) {
    for (const axis of ['Time', 'Distance']) {
      test(`empty leg charts keep the plot area: cursor line at one x in every chart (${units}, ${axis} axis, all legs)`, async ({ page }) => {
        await open(page, workout('triathlon').view, { storage: { 'wx.units': units } });
        await page.locator('.chart-stack').getByRole('button', { name: axis }).first().click();
        const legs = await page.locator('.wx-leg-pick option').count();
        for (let leg = 0; leg < legs; leg++) {
          await page.locator('.wx-leg-pick').selectOption({ index: leg });
          await page.waitForTimeout(500);
          await hover(page, 0.5, 0.1);
          await page.waitForTimeout(200);
          const xs = await page.locator('.chart-area .wx-cursor-line').evaluateAll((ls) => ls.filter((l) => l.style.visibility !== 'hidden').map((l) => Math.round(l.getBoundingClientRect().x)));
          const lefts = await page.locator('.chart-area .u-over').evaluateAll((os) => os.map((o) => Math.round(o.getBoundingClientRect().x)));
          expect(xs.length, `leg ${leg}`).toBeGreaterThan(0);
          expect(new Set(xs).size, `cursor x per chart, leg ${leg}: ${xs}`).toBe(1);
          expect(new Set(lefts).size, `plot left per chart, leg ${leg}: ${lefts}`).toBe(1);
        }
      });
    }
  }
});

test('F-67: the picker lists whole workout and each leg, and a leg view shows only its charts', async ({ page }) => {
  await open(page, workout('triathlon').view);
  await expect(page.locator('.wx-leg-pick option')).toHaveText(['Whole workout', 'Swim', 'Ride', 'Run']); // lazy sections chunk: wait (QA-75-r1-1)
  await page.locator('.wx-leg-pick').selectOption({ label: 'Swim' });
  await expect(page.locator('.metric-chart').first()).toBeVisible();
});

test('F-67: a leg hides the charts without data in it, all legs brings them back', async ({ page }) => {
  await open(page, workout('triathlon').view);
  await expect(page.locator('.wx-leg-pick option')).toHaveText(['Whole workout', 'Swim', 'Ride', 'Run']);
  const charts = page.locator('.metric-chart');
  const all = await charts.count();
  await page.locator('.wx-leg-pick').selectOption({ label: 'Swim' });
  await expect.poll(() => charts.count()).toBeLessThan(all);
  expect(await charts.count()).toBeGreaterThan(0);
  await page.locator('.wx-leg-pick').selectOption({ index: 0 });
  await expect.poll(() => charts.count()).toBe(all);
});
