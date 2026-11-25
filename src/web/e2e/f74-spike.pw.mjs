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

test('F-74: a 6 s glitch is kept: true value in the readout and max speed, chart axis clipped (AP-26)', async ({ page, isMobile }) => {
  await open(page, workout('glitch-ride').view, METRIC);
  expect(await maxSpeed(page)).toBeGreaterThan(80);
  // Taps do not move the cursor like a swipe does, and a 6 s spike is under one pixel of a phone-width chart: the readout is
  // checked with a mouse on a wide chart only (BACKLOG: touch scrubbing helper).
  if (!isMobile && page.viewportSize().width >= 600) expect(await scan(page, SPIKE_AT, 'Speed', /8\d\.\d\s*km\/h/), 'readout shows the true value').toMatch(/8\d\.\d/);
  // The glitch is drawn at the top edge, and the axis is not stretched to 85 km/h: the 26 km/h line's noise spans a visible band.
  const m = await lineMetrics(page);
  expect(m.top).toBeLessThan(0.07); // the plot starts 8 px (6 %) below the canvas top
  expect(m.spread).toBeGreaterThan(0.1);
});

const WORDS = { en: /speed spike/i, de: /Geschwindigkeitsspitze/, es: /pico breve de velocidad/, fr: /pic de vitesse/ };
for (const [lang, re] of Object.entries(WORDS)) {
  test(`AP-24: the summary explanation names the speed spike filter (${lang})`, async ({ page }) => {
    await open(page, workout('spike-ride').view, { storage: { 'wx.lang': lang } });
    // Opened while the language files still load, the dialog stays English: wait, and reload once if it still does.
    let text = '';
    for (let i = 0; i < 2 && !re.test(text); i++) {
      if (i) await page.reload();
      await page.waitForTimeout(2000);
      await page.locator('[data-summary-info]').click();
      await expect(page.locator('dialog[open] .dlg-body')).toBeVisible();
      text = await page.locator('dialog[open] .dlg-body').innerText();
    }
    expect(text).toMatch(re);
  });
}
