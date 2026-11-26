// B-03 (F-18 laps table, F-46 triggers and switch, F-69 time presets, F-48 pool swim, F-64 per-leg tables): rows, switch, presets, row click.
import { test, expect } from '@playwright/test';
import { workout, open, texts } from './helpers.mjs';

const METRIC = { storage: { 'wx.units': 'metric' } };
const sec = (page) => page.locator('section.laps');
const heads = (page) => texts(sec(page).locator('table:visible').first().locator('th'));
const col = async (page, name) => (await heads(page)).findIndex((h) => h.startsWith(name));
const cells = (page, i) => texts(sec(page).locator('table:visible').first().locator('tbody tr').locator(`td:nth-child(${i + 1})`));
const switchTo = async (page, name) => {
  const b = sec(page).locator('.laps-switch button', { hasText: new RegExp(`^${name}$`) });
  await b.click();
  await expect(b).toHaveAttribute('aria-pressed', 'true');
};
const select = (page) => sec(page).locator('select[aria-label="Split length"]');

test('F-46: Laps/Splits switch swaps the table, the split drop-down shows only in Splits', async ({ page }) => {
  const errors = await open(page, workout('run-30min').view, METRIC);
  await sec(page).scrollIntoViewIfNeeded();
  await expect(sec(page).locator('.laps-switch button')).toHaveText(['Laps', 'Splits']);
  await expect(sec(page).locator('.laps-switch button').first()).toHaveAttribute('aria-pressed', 'true');
  await expect(select(page)).toBeHidden();
  const laps = await sec(page).locator('table:visible tbody tr').count();
  expect(laps).toBe(6);
  await switchTo(page, 'Splits');
  await expect(select(page)).toBeVisible();
  await expect(select(page)).toHaveAttribute('aria-label', 'Split length');
  await expect(select(page).locator('option:checked')).toHaveText('1 km'); // F-18: running default
  await switchTo(page, 'Laps');
  await expect(select(page)).toBeHidden();
  expect(await sec(page).locator('table:visible tbody tr').count()).toBe(laps);
  expect(errors).toEqual([]);
});

test('F-18, F-69: split presets 400 m, 1 km and the time presets change the rows', async ({ page }) => {
  const errors = await open(page, workout('run-30min').view, METRIC);
  await sec(page).scrollIntoViewIfNeeded();
  await switchTo(page, 'Splits');
  const rows = () => sec(page).locator('table:visible tbody tr').count();
  const dist = async () => (await cells(page, await col(page, 'Distance'))).map(Number);
  const time = async () => cells(page, await col(page, 'Time'));

  await select(page).selectOption({ label: '1 km' });
  const km = await dist();
  expect(km.length).toBeGreaterThanOrEqual(5);
  expect(km[0]).toBeCloseTo(1, 1);
  await select(page).selectOption({ label: '400 m' });
  await expect.poll(async () => (await dist())[0]).toBeCloseTo(0.4, 1);
  expect((await dist()).length).toBeGreaterThan(km.length * 2);
  // F-69: a time preset splits by moving time; 5 min over 30 min is 6 rows of 5:00.
  await select(page).selectOption({ label: '5 min' });
  await expect.poll(rows).toBe(6);
  expect((await time())[0]).toBe('5:00');
  await select(page).selectOption({ label: '10 min' });
  await expect.poll(rows).toBe(3);
  await expect(select(page).locator('optgroup')).toHaveAttribute('label', 'Time');
  expect(errors).toEqual([]);
});

test('F-46: the trigger column lists the device triggers, last column; absent without triggers', async ({ page }) => {
  const errors = await open(page, workout('ride-lap-triggers').view, METRIC);
  await sec(page).scrollIntoViewIfNeeded();
  const h = await heads(page);
  expect(h[h.length - 1]).toBe('Trigger');
  const t = await cells(page, h.length - 1);
  expect(t).toHaveLength(5);
  expect(t.slice(0, 4)).toEqual(['Button', 'Distance', 'Position', 'Stop']);
  await open(page, workout('run-30min').view, METRIC);
  expect(await heads(page)).not.toContain('Trigger');
  expect(errors).toEqual([]);
});

test('F-48: pool swim: Stroke column on laps, Lengths view with a stroke per length', async ({ page }) => {
  const errors = await open(page, workout('pool-swim').view, METRIC);
  await sec(page).scrollIntoViewIfNeeded();
  await expect(sec(page).locator('.laps-switch button')).toHaveText(['Laps', 'Splits', 'Lengths']);
  const si = await col(page, 'Stroke');
  expect(si).toBeGreaterThan(0);
  expect((await cells(page, si)).filter(Boolean).every((s) => /^(Freestyle|Backstroke|Breaststroke|Butterfly|Drill|Mixed|IM)$/.test(s))).toBe(true);
  await switchTo(page, 'Lengths');
  const li = await col(page, 'Stroke');
  expect(li).toBeGreaterThan(0);
  const strokes = await cells(page, li);
  expect(strokes.length).toBeGreaterThan(10);
  expect(strokes).toContain('Freestyle');
  const num = await cells(page, 0);
  num.forEach((n, i) => { if (n === 'Rest') expect(strokes[i]).toBe(''); });
  await switchTo(page, 'Splits');
  await expect(select(page).locator('option').first()).toHaveText('100 m');
  expect(await heads(page)).toContain('Distance (m)');
  expect(errors).toEqual([]);
});

test('F-64: triathlon: one table per leg, per-leg split tables with their own drop-downs', async ({ page }) => {
  const errors = await open(page, workout('triathlon').view, METRIC);
  await sec(page).scrollIntoViewIfNeeded();
  await expect(sec(page).locator('.laps-leg h3')).toHaveText(['Swim', 'T1', 'Ride', 'T2', 'Run']);
  await expect(sec(page).locator('.laps-leg table')).toHaveCount(5);
  await switchTo(page, 'Splits');
  const legs = sec(page).locator('.laps-legs.splits .laps-leg');
  await expect(legs).toHaveCount(5);
  await expect(legs.locator('h3')).toContainText(['Swim', 'T1', 'Ride', 'T2', 'Run']);
  await expect(legs.locator('select[aria-label="Split length"]')).toHaveCount(3);
  const ride = legs.nth(2);
  const before = await ride.locator('tbody tr').count();
  expect(before).toBeGreaterThan(0);
  const cur = await ride.locator('select[aria-label="Split length"] option:checked').textContent();
  const other = (await texts(ride.locator('select[aria-label="Split length"] option'))).find((o) => o !== cur && !/min$/.test(o));
  await ride.locator('select[aria-label="Split length"]').selectOption({ label: other });
  await expect.poll(() => ride.locator('tbody tr').count()).not.toBe(before);
  expect(await legs.nth(4).locator('tbody tr').count()).toBeGreaterThan(0);
  expect(errors).toEqual([]);
});

test('F-18: clicking a row selects its range, another row moves it, a second click clears it; split rows too', async ({ page }) => {
  const errors = await open(page, workout('interval-run').view, METRIC);
  await sec(page).scrollIntoViewIfNeeded();
  await expect(page.locator('.range-panel')).toBeHidden();
  const body = sec(page).locator('table:visible tbody');
  const row = body.locator('tr').nth(2);
  await row.click();
  await expect(page.locator('.range-panel')).toBeVisible();
  await expect(row).toHaveClass(/\bon\b/);
  const a = await texts(page.locator('.range-panel .cell .val'));
  const row2 = body.locator('tr').nth(3);
  await row2.click();
  await expect(row2).toHaveClass(/\bon\b/);
  await expect(row).not.toHaveClass(/\bon\b/);
  expect(await texts(page.locator('.range-panel .cell .val'))).not.toEqual(a);
  await row2.click();
  await expect(page.locator('.range-panel')).toBeHidden();
  await expect(row2).not.toHaveClass(/\bon\b/);

  await switchTo(page, 'Splits');
  await select(page).selectOption({ label: '1 km' });
  const split = sec(page).locator('table:visible tbody tr').nth(1);
  await split.click();
  await expect(page.locator('.range-panel')).toBeVisible();
  await expect(split).toHaveClass(/\bon\b/);
  const d = await page.locator('.range-panel .cell', { hasText: 'Distance' }).locator('.val').textContent();
  expect(parseFloat(d)).toBeCloseTo(1, 0);
  expect(errors).toEqual([]);
});

test('F-64: a triathlon leg row selects its range, the leg heading the whole leg', async ({ page }) => {
  const errors = await open(page, workout('triathlon').view, METRIC);
  await sec(page).scrollIntoViewIfNeeded();
  const leg = sec(page).locator('.laps-leg').nth(2);
  await leg.locator('tbody tr').first().click();
  await expect(page.locator('.range-panel')).toBeVisible();
  const row = await texts(page.locator('.range-panel .cell .val'));
  await leg.locator('h3').click();
  await expect(page.locator('.range-panel')).toBeVisible();
  await expect.poll(() => texts(page.locator('.range-panel .cell .val'))).not.toEqual(row);
  expect(errors).toEqual([]);
});
