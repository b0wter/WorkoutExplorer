// B-01 (F-05 cursor readout, F-17 range panel, F-04 axis toggle): the viewing page's chart interaction, with the mouse.
import { test, expect } from '@playwright/test';
import { workout, open, hover, drag, scrub, touchPath, texts } from './helpers.mjs';

const METRIC = { storage: { 'wx.units': 'metric' } };
const readout = (page) => texts(page.locator('.chart-stack .readout .cell .val'));
const panelCell = (page, label) => page.locator('.range-panel .cell', { hasText: label }).locator('.val');

test.describe('viewing page', () => {
  test('F-05: the cursor readout follows the mouse', async ({ page }) => {
    const errors = await open(page, workout('ride-30min').view, METRIC);
    await hover(page, 0.2);
    await expect.poll(async () => (await readout(page)).join('|')).not.toMatch(/^[–\-|\s]*$/);
    const a = (await readout(page)).join('|');
    await hover(page, 0.8);
    await expect.poll(async () => (await readout(page)).join('|')).not.toBe(a);
    expect(errors).toEqual([]);
  });

  test('F-05: a touch swipe scrubs the cursor and the readout follows (touch projects)', async ({ page, isMobile }) => {
    test.skip(!isMobile, 'swipe needs touch');
    const errors = await open(page, workout('ride-30min').view, METRIC);
    await scrub(page, 0.2, 0.5);
    await expect.poll(async () => (await readout(page)).join('|')).not.toMatch(/^[–\-|\s]*$/);
    const a = (await readout(page)).join('|');
    await scrub(page, 0.5, 0.8);
    await expect.poll(async () => (await readout(page)).join('|')).not.toBe(a);
    await expect(page.locator('.range-panel')).toBeHidden(); // a swipe is not a range
    expect(errors).toEqual([]);
  });

  test('F-17: a range drag shows distance, time and gain; Clear and a click outside clear it', async ({ page, isMobile }) => {
    const errors = await open(page, workout('ride-30min').view, METRIC);
    await expect(page.locator('.range-panel')).toBeHidden();
    await drag(page, 0.2, 0.6);
    await expect(page.locator('.range-panel')).toBeVisible();
    for (const l of ['Distance', 'Time', 'Gain']) await expect(panelCell(page, l)).toHaveText(/\d/);
    await expect(page.locator('.chart-area .wx-range-shade').first()).toBeVisible();
    await page.locator('.range-panel .clear').click();
    await expect(page.locator('.range-panel')).toBeHidden();
    await drag(page, 0.2, 0.6);
    await expect(page.locator('.range-panel')).toBeVisible();
    if (isMobile) await touchPath(page, [0.9]); // a tap outside the range
    else { await hover(page, 0.9); await page.mouse.down(); await page.mouse.up(); }
    await expect(page.locator('.range-panel')).toBeHidden();
    expect(errors).toEqual([]);
  });

  test('F-04: the Time/Distance axis toggle keeps the range', async ({ page }) => {
    const errors = await open(page, workout('ride-30min').view, METRIC);
    const toggle = page.locator('.axis-toggle');
    await expect(toggle.getByRole('button', { name: 'Time' })).toHaveAttribute('aria-pressed', 'true');
    await drag(page, 0.2, 0.6);
    const before = await texts(page.locator('.range-panel .cell .val'));
    await toggle.getByRole('button', { name: 'Distance' }).click();
    await expect(toggle.getByRole('button', { name: 'Distance' })).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('.range-panel')).toBeVisible();
    await expect(page.locator('.chart-area .wx-range-shade').first()).toBeVisible();
    expect(await texts(page.locator('.range-panel .cell .val'))).toEqual(before);
    await toggle.getByRole('button', { name: 'Time' }).click();
    await expect(page.locator('.range-panel')).toBeVisible();
    expect(errors).toEqual([]);
  });

  test('F-05: gps-dropout: chart lines run on through the 5 min without positions (no false gap), no errors', async ({ page }) => {
    const errors = await open(page, workout('gps-dropout').view, METRIC);
    // The dropout is 1500-1800 s of 3600 s (42 % to 50 % of the time axis); colour is the only saturated pixel on the canvas.
    const cols = await page.locator('.metric-chart').first().locator('canvas').evaluate((c) => {
      const d = c.getContext('2d').getImageData(0, 0, c.width, c.height).data;
      const has = (x) => { for (let y = 0; y < c.height; y++) { const o = (y * c.width + x) * 4; if (d[o + 3] > 200 && Math.max(d[o], d[o + 1], d[o + 2]) - Math.min(d[o], d[o + 1], d[o + 2]) > 60) return true; } return false; };
      return [0.3, 0.44, 0.46, 0.48, 0.7].map((f) => has(Math.floor(c.width * f)));
    });
    expect(cols, 'line drawn left of, inside and right of the dropout').toEqual([true, true, true, true, true]);
    await hover(page, 0.46);
    await expect.poll(async () => (await readout(page)).join('|')).not.toMatch(/^[–\-|\s]*$/);
    expect(errors).toEqual([]);
  });
});
