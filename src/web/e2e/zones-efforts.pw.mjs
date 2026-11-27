// B-13 (F-49 zones panel, F-50 power curve and best efforts, F-51 section ordering): present, formatted, no console error.
import { test, expect } from '@playwright/test';
import { workout, open, texts } from './helpers.mjs';

const MET = { storage: { 'wx.units': 'metric' } };

test('F-49: zones panel on a ride lists heart rate and power zones with times and shares', async ({ page }) => {
  const errors = await open(page, workout('zones-wahoo-ride').view, MET);
  const slot = page.locator('#zones-slot');
  await expect(slot).toBeVisible();
  await expect(slot.locator('h2')).toHaveText('Zones');
  await expect(slot.locator('h3')).toHaveText(['Heart rate', 'Power']);
  await expect(slot.locator('.zones-src')).toHaveCount(2);
  for (const k of ['heart_rate', 'power']) {
    const rows = slot.locator(`.zone[data-k="${k}"]`);
    expect(await rows.count()).toBeGreaterThanOrEqual(3);
    const t = await texts(rows.locator('.zone-t'));
    const p = await texts(rows.locator('.zone-p'));
    for (const s of t) expect(s).toMatch(/^\d+:\d\d(:\d\d)?$/);
    for (const s of p) expect(s).toMatch(/^\d+%$/);
    expect(p.map((s) => parseInt(s)).reduce((a, b) => a + b, 0)).toBeGreaterThan(95);
  }
  expect(errors).toEqual([]);
  expect(errors.blocked.filter((u) => !/openfreemap|tiles/i.test(u))).toEqual([]);
});

test('F-49: zones panel on a heart-rate-only run shows no power block', async ({ page }) => {
  const errors = await open(page, workout('zones-apple-run-6').view, MET);
  const slot = page.locator('#zones-slot');
  await expect(slot).toBeVisible();
  await expect(slot.locator('h3')).toHaveText(['Heart rate']);
  await expect(slot.locator('.zone[data-k="heart_rate"]').first()).toBeVisible();
  await expect(slot.locator('.zone[data-k="power"]')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('F-50, F-51: power curve SVG and best efforts tables on ride-30min', async ({ page }) => {
  const errors = await open(page, workout('ride-30min').view, MET);
  const slot = page.locator('#efforts-slot');
  await expect(slot).toBeVisible();
  await expect(slot.locator('h2')).toContainText(/Best efforts/);
  const curve = slot.locator('svg.curve');
  await expect(curve).toBeVisible();
  await expect(curve).toHaveAttribute('aria-label', /^Power curve: best average power for every duration from 1 s \(\d+ W\) to .+ \(\d+ W\)$/);
  expect(await curve.locator('polyline').count()).toBe(1);
  expect((await curve.locator('polyline').getAttribute('points')).trim().split(/\s+/).length).toBeGreaterThan(4);
  await expect(slot.locator('h3', { hasText: 'Power curve' })).toBeVisible();
  expect(await slot.locator('table').count()).toBeGreaterThanOrEqual(1);
  const heads = (await texts(slot.locator('th'))).join(' ');
  expect(heads).toMatch(/Duration/);
  expect(heads).toMatch(/Power/);
  const start = await texts(slot.locator('tbody tr td:last-child'));
  expect(start.length).toBeGreaterThan(0);
  for (const s of start) expect(s).toMatch(/^\d+:\d\d(:\d\d)?$/);
  const watts = await texts(slot.locator('tbody tr td:nth-child(2)'));
  for (const s of watts) expect(s).toMatch(/^\d+\s*W$/);
  expect(errors).toEqual([]);
});
