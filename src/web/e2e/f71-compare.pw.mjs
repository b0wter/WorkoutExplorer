// F-71 (AP-23): comparison chart of laps and splits with the non-symmetric axis (median line, break marks).
import { test, expect } from '@playwright/test';
import { workout, open } from './helpers.mjs';

const svg = (page) => page.locator('section.laps svg').filter({ has: page.locator('rect.bar') }).first();

for (const name of ['interval-run', 'ride-12h']) {
  test(`F-71: ${name}: laps chart has one bar per lap and a median line, the Splits view draws too`, async ({ page }) => {
    const errors = await open(page, workout(name).view, { storage: { 'wx.units': 'metric' } });
    await page.locator('section.laps').scrollIntoViewIfNeeded();
    const rows = await page.locator('section.laps tbody tr').count();
    expect(rows).toBeGreaterThan(5);
    await expect(svg(page)).toBeVisible();
    expect(await svg(page).locator('rect.bar').count()).toBe(rows);
    expect(await svg(page).locator('line.ref').count()).toBe(1);
    // The chart is as wide as its section and not clipped by it.
    const fits = await svg(page).evaluate((s) => { const a = s.getBoundingClientRect(); const b = s.closest('section').getBoundingClientRect(); return a.left >= b.left - 1 && a.right <= b.right + 1 && a.height > 40; });
    expect(fits).toBe(true);

    await page.locator('.laps-switch button').nth(1).click();
    await expect(page.locator('.laps-switch button').nth(1)).toHaveAttribute('aria-pressed', 'true');
    await expect(svg(page)).toBeVisible();
    expect(await svg(page).locator('rect.bar').count()).toBeGreaterThan(0);
    expect(errors).toEqual([]);
  });
}

test('F-71: the interval run shows the slow recovery laps against the fast reps (bar heights differ, axis is not symmetric)', async ({ page }) => {
  await open(page, workout('interval-run').view, { storage: { 'wx.units': 'metric' } });
  await page.locator('section.laps').scrollIntoViewIfNeeded();
  await expect(svg(page)).toBeVisible();
  await expect(svg(page).locator('line.ref')).toHaveCount(1);
  const bars = await svg(page).locator('rect.bar').evaluateAll((r) => r.map((b) => b.getBoundingClientRect().height));
  expect(Math.max(...bars)).toBeGreaterThan(Math.min(...bars) * 1.5);
  const ref = await svg(page).locator('line.ref').evaluate((l) => { const r = l.getBoundingClientRect(), s = l.closest('svg').getBoundingClientRect(); return (r.top - s.top) / s.height; });
  expect(ref).toBeGreaterThan(0.05);
  expect(ref).toBeLessThan(0.95);
});
