// F-72 (DS-1): swim distance in yards for imperial viewers (laps table, readout, range, legs), metres otherwise.
import { test, expect } from '@playwright/test';
import { workout, open, hover, drag, texts } from './helpers.mjs';

const IMP = { storage: { 'wx.units': 'imperial' } };
const MET = { storage: { 'wx.units': 'metric' } };
const cell = async (page, label) => (await texts(page.locator('.chart-stack .readout .cell', { hasText: label })))[0];
const lapHeads = (page) => texts(page.locator('section.laps th'));
const distanceAxis = (page) => page.locator('.chart-stack').getByRole('button', { name: 'Distance' }).first().click();

test('F-72: 25 yd pool, imperial: laps table and readout are in yd', async ({ page, isMobile }) => {
  await open(page, workout('pool-swim-25yd').view, IMP);
  expect((await lapHeads(page)).join(' ')).toMatch(/\(\/100 yd\)/);
  await distanceAxis(page);
  await hover(page, 0.5);
  expect(await cell(page, 'Distance')).toMatch(/\byd\b/);
});

test('F-72: 25 m pool, metric: m, never yd', async ({ page }) => {
  await open(page, workout('pool-swim').view, MET);
  const heads = (await lapHeads(page)).join(' ');
  expect(heads).toMatch(/\(\/100 m\)/);
  expect(heads).not.toMatch(/yd/);
});

test('F-72: switching units live changes yd to m without reloading', async ({ page }) => {
  await open(page, workout('pool-swim').view, IMP);
  expect((await lapHeads(page)).join(' ')).toMatch(/\(\/100 yd\)/);
  await page.locator('select', { hasText: 'Imperial' }).first().selectOption({ label: 'Metric' });
  await expect.poll(async () => (await lapHeads(page)).join(' ')).toMatch(/\(\/100 m\)/);
  expect((await lapHeads(page)).join(' ')).not.toMatch(/yd/);
});

test('F-72: a range on a swim reads in yd', async ({ page, isMobile }) => {
  test.skip(isMobile, 'range selection is a mouse drag');
  await open(page, workout('pool-swim').view, IMP);
  await drag(page, 0.3, 0.6);
  const range = (await texts(page.locator('.range-panel .cell'))).join(' ');
  expect(range).toMatch(/\d\s*yd\b/);
});

test('F-72: triathlon, imperial: swim leg in yd, ride leg in mi', async ({ page }) => {
  await open(page, workout('triathlon').view, IMP);
  await distanceAxis(page);
  await page.locator('.wx-leg-pick').selectOption({ label: 'Swim' });
  await hover(page, 0.5);
  expect(await cell(page, 'Distance')).toMatch(/\byd\b/);
  await page.locator('.wx-leg-pick').selectOption({ label: 'Ride' });
  await hover(page, 0.5);
  const ride = await cell(page, 'Distance');
  expect(ride).toMatch(/\bmi\b/);
  expect(ride).not.toMatch(/yd/);
});
