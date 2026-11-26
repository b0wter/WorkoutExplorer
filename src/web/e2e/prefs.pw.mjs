// B-08 (F-14 units, F-54 language, F-41 theme and tint cards): live switches, no data or API request, nothing stored server side.
import { test, expect } from '@playwright/test';
import { workout, open, texts } from './helpers.mjs';

const speedCard = (page) => page.locator('.summary .metric[data-k="speed"]');
// By slot, not by label: the labels follow the language.
const sel = (page, label) => page.locator(`#${{ Units: 'units', Theme: 'theme', Language: 'lang' }[label]}-slot select`);
const card = async (page) => (await texts(speedCard(page)))[0];
// Data and API requests only: a language switch may fetch its static language file, never the data file or the API.
const watch = (page) => {
  const hits = [];
  page.on('request', (r) => { if (/\/api\/|\.bin(\?|$)|pins\.[^/]*\.json/.test(r.url())) hits.push(r.method() + ' ' + r.url()); });
  return hits;
};

test('F-14: run: pace; Metric/Imperial switch changes values live, no data or API request', async ({ page }) => {
  const errors = await open(page, workout('run-30min').view, { storage: { 'wx.units': 'metric' } });
  await sel(page, 'Units').waitFor();
  const hits = watch(page);
  const metric = await card(page);
  expect(metric).toMatch(/Avg pace.*\/km/);
  await sel(page, 'Units').selectOption('imperial');
  await expect.poll(() => card(page)).toMatch(/Avg pace.*\/mi/);
  expect(await card(page)).not.toBe(metric);
  await expect(page.locator('section.laps th').filter({ hasText: /\/mi/ }).first()).toBeVisible();
  await sel(page, 'Units').selectOption('metric');
  await expect.poll(() => card(page)).toBe(metric);
  expect(hits).toEqual([]);
  expect(errors).toEqual([]);
});

test('F-14: ride shows speed (km/h, mph), never pace', async ({ page }) => {
  const errors = await open(page, workout('ride-30min').view, { storage: { 'wx.units': 'metric' } });
  await sel(page, 'Units').waitFor();
  const hits = watch(page);
  expect(await card(page)).toMatch(/Avg speed.*km\/h/);
  await sel(page, 'Units').selectOption('imperial');
  await expect.poll(() => card(page)).toMatch(/Avg speed.*mph/);
  expect(await card(page)).not.toMatch(/pace/i);
  expect(hits).toEqual([]);
  expect(errors).toEqual([]);
});

test('F-14: swim shows pace per 100 m, per 100 yd imperial', async ({ page }) => {
  const errors = await open(page, workout('pool-swim').view, { storage: { 'wx.units': 'metric' } });
  await sel(page, 'Units').waitFor();
  const hits = watch(page);
  expect(await card(page)).toMatch(/Avg pace.*\/100 m/);
  await sel(page, 'Units').selectOption('imperial');
  await expect.poll(() => card(page)).toMatch(/Avg pace.*\/100 yd/);
  expect(hits).toEqual([]);
  expect(errors).toEqual([]);
});

test('F-54: switching language between en, de, es, fr applies at once, no data or API request', async ({ page }) => {
  const errors = await open(page, workout('run-30min').view);
  await sel(page, 'Language').waitFor();
  const hits = watch(page);
  const seen = { en: await card(page) };
  expect(seen.en).toMatch(/^Avg pace/);
  for (const l of ['de', 'es', 'fr']) {
    await sel(page, 'Language').selectOption(l);
    await expect.poll(() => card(page), l).not.toMatch(/^Avg pace/);
    await expect(sel(page, 'Language')).toHaveValue(l);
    seen[l] = await card(page);
  }
  expect(new Set(Object.values(seen)).size).toBe(4);
  await sel(page, 'Language').selectOption('en');
  await expect.poll(() => card(page)).toBe(seen.en);
  expect(hits).toEqual([]);
  expect(errors).toEqual([]);
});

test('F-41, DS-4: theme System/Light/Dark switches live; the tint cards differ from plain cards in both themes', async ({ page }) => {
  await page.emulateMedia({ colorScheme: 'light' });
  const errors = await open(page, workout('run-30min').view);
  await sel(page, 'Theme').waitFor();
  const hits = watch(page);
  await expect(sel(page, 'Theme').locator('option')).toHaveText(['System', 'Light', 'Dark']);
  await expect(sel(page, 'Theme')).toHaveValue('system');
  const bg = (loc) => loc.first().evaluate((e) => getComputedStyle(e).backgroundColor);
  const pageBg = () => page.evaluate(() => getComputedStyle(document.documentElement).backgroundColor);
  const tinted = ['speed', 'heart_rate', 'ascent', 'power', 'cadence', 'temperature'];
  const tint = page.locator('.summary .metric[data-k="speed"], .summary .metric[data-k="heart_rate"], .summary .metric[data-k="ascent"]');
  const plain = page.locator('.summary .metric' + tinted.map((k) => `:not([data-k="${k}"])`).join(''));
  await expect(tint.first()).toBeVisible();
  const seen = {};
  for (const th of ['light', 'dark']) {
    await sel(page, 'Theme').selectOption(th);
    await expect(page.locator('html')).toHaveAttribute('data-theme', th);
    seen[th] = { page: await pageBg(), tint: await bg(tint), plain: await bg(plain) };
    expect(seen[th].tint, th).not.toBe(seen[th].plain);
  }
  expect(seen.light.page).not.toBe(seen.dark.page);
  expect(seen.light.tint).not.toBe(seen.dark.tint);
  // System follows the OS scheme and drops the attribute.
  await page.emulateMedia({ colorScheme: 'dark' });
  await sel(page, 'Theme').selectOption('system');
  await expect(page.locator('html')).not.toHaveAttribute('data-theme', /.*/);
  await expect.poll(pageBg).toBe(seen.dark.page);
  await page.emulateMedia({ colorScheme: 'light' });
  await expect.poll(pageBg).toBe(seen.light.page);
  expect(hits).toEqual([]);
  expect(errors).toEqual([]);
});

test('F-14, F-54, DS-4: choices stay in the browser, a switch sends no API request and no write', async ({ page }) => {
  const errors = await open(page, workout('run-30min').view);
  await sel(page, 'Units').waitFor();
  await page.waitForLoadState('networkidle');
  const all = [];
  page.on('request', (r) => { if (r.method() !== 'GET' || /\/api\//.test(r.url())) all.push(r.method() + ' ' + r.url()); });
  await sel(page, 'Units').selectOption('imperial');
  await sel(page, 'Theme').selectOption('dark');
  await sel(page, 'Language').selectOption('de');
  await expect.poll(() => card(page)).not.toMatch(/^Avg pace/);
  expect(all).toEqual([]);
  const stored = await page.evaluate(() => ({ u: localStorage.getItem('wx.units'), t: localStorage.getItem('wx.theme'), l: localStorage.getItem('wx.lang') }));
  expect(stored).toEqual({ u: 'imperial', t: 'dark', l: 'de' });
  expect(errors).toEqual([]);
});
