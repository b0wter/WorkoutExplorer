// F-74 (UP-12, AP-26, AP-24): a 1 s speed spike is removed and ignored by max speed; a kept 6 s glitch
// does not squash the speed chart (axis clipped), while the readout still shows the true value.
import { test, expect } from '@playwright/test';
import { workout, open, scan } from './helpers.mjs';

const METRIC = { storage: { 'wx.units': 'metric' } };
const SPIKE_AT = 900 / 1799; // both rides glitch at t = 900 s of 1,799 s; the time axis is the default
const maxSpeed = async (page) => Number(/max\s*([\d.]+)/.exec(await page.locator('.summary .metric[data-k="speed"]').innerText())[1]);

// Colour of the line is the only saturated colour on the canvas (grid and text are grey). Returns the topmost coloured row of
// the whole speed chart and the vertical spread of the line around x = 20 %, both as fractions of the canvas height.
const lineMetrics = (page) => page.locator('.metric-chart', { hasText: 'Speed' }).locator('canvas').evaluate((c) => {
  const d = c.getContext('2d').getImageData(0, 0, c.width, c.height).data;
  const hit = (x, y) => { const o = (y * c.width + x) * 4; return d[o + 3] > 200 && Math.max(d[o], d[o + 1], d[o + 2]) - Math.min(d[o], d[o + 1], d[o + 2]) > 60; };
  let top = null;
  for (let y = 0; y < c.height && top === null; y++) for (let x = 0; x < c.width; x += 2) if (hit(x, y)) { top = y / c.height; break; }
  const x0 = Math.floor(c.width * 0.2);
  let lo = Infinity, hi = -1;
  for (let x = x0 - 10; x <= x0 + 10; x++) for (let y = 0; y < c.height; y++) if (hit(x, y)) { lo = Math.min(lo, y); hi = Math.max(hi, y); }
  return { top, spread: hi < 0 ? 0 : (hi - lo) / c.height };
});

test('F-74: a 1 s spike is removed: max speed is the ride speed, the readout there is a dash', async ({ page }) => {
  await open(page, workout('spike-ride').view, METRIC);
  const max = await maxSpeed(page);
  expect(max).toBeGreaterThan(24);
  expect(max).toBeLessThan(35);
  // The spike is gone from the chart: nothing is drawn at the top edge (the axis keeps its padding above the data).
  expect((await lineMetrics(page)).top).toBeGreaterThan(0.1);
  // (The missing sample itself cannot be aimed at: a chart pixel is about 3 s wide, a spike is 1 s.)
});

test('F-74: a 6 s glitch is kept: true value in the readout and max speed, chart axis clipped (AP-26)', async ({ page }) => {
  await open(page, workout('glitch-ride').view, METRIC);
  expect(await maxSpeed(page)).toBeGreaterThan(80);
  // The cursor snaps to whole chart pixels: a 6 s glitch is hit only where a pixel is under ~6 s. iPhone's 294 px chart (6.1 s per
  // pixel) steps over it (probed: 26.0, 25.9, 26.3 around it); headless Firefox at phone width has a mouse and the same narrow chart.
  const wide = (await page.locator('.chart-area .u-over').first().boundingBox()).width >= 330;
  if (wide && (page.viewportSize().width >= 600 || await page.evaluate(() => matchMedia('(pointer: coarse)').matches))) expect(await scan(page, SPIKE_AT, 'Speed', /8\d\.\d\s*km\/h/, 14), 'readout shows the true value').toMatch(/8\d\.\d/);
  // The glitch is drawn at the top edge, and the axis is not stretched to 85 km/h: the 26 km/h line's noise spans a visible band.
  const m = await lineMetrics(page);
  expect(m.top).toBeLessThan(0.07); // the plot starts 8 px (6 %) below the canvas top
  expect(m.spread).toBeGreaterThan(0.1);
});

const WORDS = { en: /speed spike/i, de: /Geschwindigkeitsspitze/, es: /pico breve de velocidad/, fr: /pic de vitesse/ };
for (const [lang, re] of Object.entries(WORDS)) {
  test(`AP-24: the summary explanation names the speed spike filter (${lang})`, async ({ page }) => {
    await open(page, workout('spike-ride').view, { storage: { 'wx.lang': lang } });
    await page.locator('[data-summary-info]').click();
    await expect(page.locator('dialog[open] .dlg-body')).toContainText(re);
  });
}
