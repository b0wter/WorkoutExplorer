// README screenshots of synthetic workouts. Run: make screenshots. Unlike the suite, the map tile/style/font host is let through
// (OpenFreeMap, what the app normally uses) so the map is not empty; every other outside request stays blocked.
import { test, expect } from '@playwright/test';
import { workout, open, hover, blockedOf } from './helpers.mjs';

const OUT = '/repo/docs/screenshots';
const TILES = /^https:\/\/[^/]*openfreemap\.org\//;
const SAT = /^https:\/\/server\.arcgisonline\.com\//; // only for the satellite shot

async function shoot(page, url, storage, name, { leg, cursor = 0.5, clip, satellite } = {}) {
  blockedOf(page); // registers the blocking route first; routes added later win, so tiles pass and the rest stays blocked
  await page.context().route(satellite ? new RegExp(TILES.source + '|' + SAT.source) : TILES, (r) => r.continue());
  await open(page, url, { storage: { 'wx.units': 'metric', ...storage } });
  await expect(page.locator('.wx-map canvas.maplibregl-canvas')).toBeVisible({ timeout: 30000 });
  if (leg) {
    await expect(page.locator('.wx-leg-pick option')).toHaveText(['Whole workout', 'Swim', 'Ride', 'Run']);
    await page.locator('.wx-leg-pick').selectOption({ label: leg });
  }
  if (satellite) await page.locator('button.wx-sat').click();
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(3000); // tiles painted, map idle
  await hover(page, cursor);
  await page.waitForTimeout(500);
  await page.screenshot({ path: `${OUT}/${name}.jpg`, type: 'jpeg', quality: 82, ...(clip ? { fullPage: true, clip } : {}) });
}

test('desktop: ride, dark, satellite', async ({ page }, info) => {
  test.skip(info.project.name !== 'desktop');
  await shoot(page, workout('ride-30min').view, { 'wx.theme': 'dark' }, 'ride-desktop', { satellite: true });
});

test('phone: run, light', async ({ page }, info) => {
  test.skip(info.project.name !== 'phone');
  await shoot(page, workout('run-30min').view, { 'wx.theme': 'light' }, 'run-phone', { clip: { x: 0, y: 0, width: 390, height: 1450 } });
});

// Chromium at iPad size: WebKit crashes on the triathlon page at that size (B-24).
test('tablet: triathlon, Ride leg, light', async ({ page }, info) => {
  test.skip(info.project.name !== 'tablet');
  await shoot(page, workout('triathlon').view, { 'wx.theme': 'light' }, 'triathlon-tablet', { leg: 'Ride' });
});
