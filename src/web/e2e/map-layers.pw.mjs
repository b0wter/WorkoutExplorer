// B-14 (F-25 route colouring, F-61 satellite, F-62 show my location). Chromium and WebKit only (WebGL). Nothing leaves the site:
// open() blocks outside requests; the satellite tiles are never answered, the blocked request is the proof.
import { test, expect } from '@playwright/test';
import { workout, open, texts } from './helpers.mjs';

const METRIC = { storage: { 'wx.units': 'metric' } };
const SAT = /server\.arcgisonline\.com/;

// The remote style is stubbed (empty background): satellite mode is built from it, and without it the switch never happens.
const STYLE = { version: 8, sources: {}, layers: [{ id: 'bg', type: 'background', paint: { 'background-color': '#ddd' } }] };

async function openMap(page, stubStyle = false) {
  if (stubStyle) await page.route(/tiles\.openfreemap\.org\/styles\//, (r) => r.fulfill({ json: STYLE, headers: { 'access-control-allow-origin': '*' } }));
  const errors = await open(page, workout('ride-30min').view, METRIC);
  await expect(page.locator('.wx-map canvas.maplibregl-canvas')).toBeVisible({ timeout: 20000 });
  return errors;
}

test.describe('map layers and location (B-14)', () => {
  test.beforeEach(({ browserName }) => { test.skip(browserName === 'firefox', 'headless Firefox has no WebGL: the map never starts'); });

  test('F-25: the colour select changes the legend', async ({ page }) => {
    const errors = await openMap(page);
    const select = page.locator('.wx-colour select');
    const legend = page.locator('.wx-legend');
    await expect(legend).toBeHidden();
    const metrics = await select.locator('option').evaluateAll((o) => o.map((e) => e.value).filter(Boolean));
    expect(metrics.length).toBeGreaterThanOrEqual(2);
    const seen = [];
    for (const m of metrics.slice(0, 2)) {
      await select.selectOption(m);
      await expect(legend).toBeVisible();
      await expect(legend.locator('.wx-swatches span').first()).toBeVisible();
      seen.push((await texts(legend.locator('.wx-labels span'))).join('|') + '/' + (await page.locator('.wx-unit').textContent()));
    }
    expect(seen[0]).not.toBe(seen[1]);
    await select.selectOption('');
    await expect(legend).toBeHidden();
    expect(errors).toEqual([]);
  });

  test('F-61: satellite is fetched only after the toggle', async ({ page }) => {
    const errors = await openMap(page, true);
    await page.waitForTimeout(1500);
    expect(errors.blocked.filter((u) => SAT.test(u))).toEqual([]);
    const sat = page.locator('button.wx-sat');
    await expect(sat).toHaveAttribute('aria-pressed', 'false');
    await sat.click();
    await expect(sat).toHaveAttribute('aria-pressed', 'true');
    await expect.poll(() => errors.blocked.filter((u) => SAT.test(u)).length, { timeout: 20000 }).toBeGreaterThan(0);
    expect(errors.blocked.filter((u) => !SAT.test(u) && !/openfreemap\.org/.test(u))).toEqual([]);
    expect(errors).toEqual([]);
  });

  test('F-62: no location is asked before the button is pressed', async ({ page, context }) => {
    await context.grantPermissions(['geolocation']);
    await context.setGeolocation({ latitude: 52.52, longitude: 13.405, accuracy: 20 });
    await page.addInitScript(() => {
      window.__geo = 0;
      const g = navigator.geolocation;
      for (const k of ['getCurrentPosition', 'watchPosition']) { const f = g[k].bind(g); g[k] = (...a) => { window.__geo++; return f(...a); }; }
    });
    const errors = await openMap(page);
    await page.waitForTimeout(1500);
    expect(await page.evaluate(() => window.__geo)).toBe(0);
    const btn = page.locator('.wx-loc-group button[aria-label="Show my location"]');
    await expect(btn).toHaveAttribute('aria-pressed', 'false');
    await btn.click();
    await expect(btn).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('.wx-loc-dot')).toBeVisible({ timeout: 15000 });
    expect(await page.evaluate(() => window.__geo)).toBeGreaterThan(0);
    expect(errors.blocked.filter((u) => !/openfreemap\.org/.test(u))).toEqual([]);
    expect(errors).toEqual([]);
  });
});
