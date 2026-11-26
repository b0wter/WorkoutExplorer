// B-12 (F-42 multisport sections, F-44 pool summary, F-43 device line): chart leg lines and names, picker labels, pool cells, device line.
// Laps-table legs and the picker's clearing/alignment are in laps.pw.mjs and legs.pw.mjs.
import { test, expect } from '@playwright/test';
import { workout, upload, open, texts, desktopOnly } from './helpers.mjs';

const keys = (page) => page.locator('.summary .metric').evaluateAll((ms) => ms.map((m) => m.dataset.k));
const cell = (page, k) => page.locator(`.summary .metric[data-k="${k}"]`);
const txt = async (loc) => (await loc.textContent()).replace(/\s+/g, ' ').trim();
const TINTED = ['ascent', 'speed', 'heart_rate', 'power', 'cadence', 'temperature'];

test('F-42: triathlon chart shows 4 leg lines and the leg names in every chart; picker lists sports without transitions', async ({ page }) => {
  test.slow(); // software-rendered WebKit redraws every chart on each axis switch
  const errors = await open(page, workout('triathlon').view);
  await expect(page.locator('.wx-leg-pick option')).toHaveText(['Whole workout', 'Swim', 'Ride', 'Run']);
  const charts = page.locator('.chart-area .u-over');
  const n = await charts.count();
  expect(n).toBeGreaterThan(1);
  for (const axis of ['Time', 'Distance']) {
    await page.locator('.chart-stack').getByRole('button', { name: axis }).first().click();
    for (let i = 0; i < n; i++) {
      const over = charts.nth(i);
      await expect(over.locator('.wx-leg-line'), `${axis} chart ${i}`).toHaveCount(4);
      const names = await texts(over.locator('.wx-leg-name'));
      expect(names).toEqual(['Swim', 'T1', 'Ride', 'T2', 'Run'].filter((s) => names.includes(s))); // time order, none repeated
    }
  }
  expect(errors).toEqual([]);
});

// A name that does not fit its section is left out (MS-7), so on a phone the first name can be missing: desktop only.
test.describe('first leg', () => {
  desktopOnly(test, 'leg names are dropped where they do not fit');
test('F-42: the first leg name sits at the left of the plot, no line there', async ({ page }) => {
  const errors = await open(page, workout('triathlon').view);
  const over = page.locator('.chart-area .u-over').first();
  await expect(over.locator('.wx-leg-name').first()).toHaveText('Swim');
  const o = await over.boundingBox();
  const nm = await over.locator('.wx-leg-name').first().boundingBox();
  expect(Math.abs(nm.x - o.x)).toBeLessThan(4);
  const lines = await over.locator('.wx-leg-line').evaluateAll((ls) => ls.map((l) => l.getBoundingClientRect().x));
  lines.forEach((x) => expect(x).toBeGreaterThan(o.x + 2));
  expect(errors).toEqual([]);
});
});

test('F-42: the laps legs are in the server HTML without JavaScript; single-sport files have no legs, lines or picker', async ({ page, request }) => {
  const html = await (await request.get(workout('triathlon').view)).text();
  const heads = [...html.matchAll(/<h3[^>]*>([^<]*)<\/h3>/g)].map((m) => m[1].trim());
  expect(heads.slice(-5)).toEqual(['Swim', 'T1', 'Ride', 'T2', 'Run']);
  const errors = await open(page, workout('ride-30min').view);
  await expect(page.locator('.wx-leg-pick')).toHaveCount(0);
  await expect(page.locator('.wx-leg-line, .wx-leg-name')).toHaveCount(0);
  await expect(page.locator('section.laps .laps-leg')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('F-44: pool swim shows Pool 25 m and Lengths 40, untinted, after Calories and before the first tinted cell', async ({ page }) => {
  const errors = await open(page, workout('pool-swim').view, { storage: { 'wx.units': 'metric' } });
  expect(await txt(cell(page, 'pool'))).toMatch(/^Pool\s*25 m$/);
  expect(await txt(cell(page, 'lengths'))).toMatch(/^Lengths\s*40$/); // the one rest length is not counted
  const k = await keys(page);
  expect(k.indexOf('pool')).toBeGreaterThan(k.indexOf('calories'));
  expect(k.indexOf('lengths')).toBe(k.indexOf('pool') + 1);
  const firstTinted = Math.min(...TINTED.map((t) => k.indexOf(t)).filter((i) => i >= 0));
  expect(k.indexOf('lengths')).toBeLessThan(firstTinted);
  const bg = (loc) => loc.evaluate((e) => getComputedStyle(e).backgroundColor);
  expect(await bg(cell(page, 'pool'))).toBe(await bg(cell(page, 'distance')));
  expect(await bg(cell(page, 'lengths'))).toBe(await bg(cell(page, 'distance')));
  expect(errors).toEqual([]);
});

test('F-44: a 25 yd pool shows "25 yd" in metric and imperial', async ({ page }) => {
  for (const units of ['metric', 'imperial']) {
    const errors = await open(page, workout('pool-swim-25yd').view, { storage: { 'wx.units': units } });
    await page.waitForLoadState('networkidle'); // lazy chunks of this page must finish before the next goto
    expect(await txt(cell(page, 'pool')), units).toMatch(/^Pool\s*25 yd$/);
    expect(await txt(cell(page, 'lengths')), units).toMatch(/^Lengths\s*40$/);
    expect(errors).toEqual([]);
  }
});

for (const name of ['triathlon', 'ride-30min']) {
  test(`F-44: ${name} shows no pool cells`, async ({ page }) => {
    const errors = await open(page, workout(name).view);
    await expect(cell(page, 'distance')).toBeVisible();
    await expect(page.locator('.summary .metric[data-k="pool"], .summary .metric[data-k="lengths"]')).toHaveCount(0);
    expect(errors).toEqual([]);
  });
}

test('F-43: no device line and no space without device name and sensors', async ({ page, request }) => {
  // ride-no-laps: manufacturer development, no product name, no sensors. The other synthetic files record the product name "corpusgen".
  const bare = await upload(request, 'ride-no-laps', { publish: true });
  const errors = await open(page, bare.view);
  await expect(page.locator('.summary')).toBeVisible();
  await expect(page.locator('p.device')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('F-43: a recorded product name alone gives "Recorded with <name>"', async ({ page }) => {
  const errors = await open(page, workout('ride-30min').view);
  await expect(page.locator('p.device')).toHaveText('Recorded with corpusgen');
  expect(errors).toEqual([]);
});

test('F-43: model and sensor types once each, under the summary strip, no horizontal scroll', async ({ page }) => {
  // indoor-no-gps: Wahoo ELEMNT BOLT, heart-rate monitor, two power meters, a shifting sensor and a built-in one
  const errors = await open(page, workout('indoor-no-gps').view);
  await expect(page.locator('p.device')).toHaveText('Recorded with Wahoo Fitness ELEMNT BOLT · heart-rate monitor, power meter');
  const s = await page.locator('.summary').boundingBox();
  const d = await page.locator('p.device').boundingBox();
  expect(d.y).toBeGreaterThanOrEqual(s.y + s.height - 1);
  expect(d.y - (s.y + s.height)).toBeLessThan(40);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect(errors).toEqual([]);
});

test('F-43: the device line is in the server HTML without JavaScript and follows the page language', async ({ page, request }) => {
  const html = await (await request.get(workout('indoor-no-gps').view)).text();
  expect(html).toMatch(/<p class="device">Recorded with Wahoo Fitness ELEMNT BOLT · heart-rate monitor, power meter<\/p>/);
  expect(html).not.toMatch(/serial/i);
  const errors = await open(page, workout('indoor-no-gps').view, { storage: { 'wx.lang': 'de' } });
  await expect(page.locator('p.device')).not.toContainText('Recorded with'); // lazy language file: retry
  await expect(page.locator('p.device')).toContainText('ELEMNT BOLT');
  expect(errors).toEqual([]);
});
