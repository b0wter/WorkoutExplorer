// B-18 (F-72 open items, YD-3, YD-8, DS-1): open-water swim with a map in yd, pins and added laps on the edit page, the fullscreen
// strip, and the imperial default from the browser language with no stored wx.units. Map parts: Chromium and WebKit (WebGL).
import { test, expect } from '@playwright/test';
import { workout, upload, open, hover, texts } from './helpers.mjs';

const NAME = 'swim-bike-distance-fall';
const NOMAP = ['firefox', 'firefox-phone'];
const YD = /\byd\b/;
const mapUp = (page) => expect(page.locator('.wx-map canvas.maplibregl-canvas')).toBeVisible({ timeout: 20000 });
const cell = async (page, label) => (await texts(page.locator('.chart-stack .readout .cell', { hasText: label })))[0];
const distanceAxis = (page) => page.locator('.chart-stack').getByRole('button', { name: 'Distance' }).first().click();
const swimLeg = (page) => page.locator('.wx-leg-pick').selectOption({ label: 'Swim' });

test('F-72: open-water swim with a map, imperial: distance in yd', async ({ page }, info) => {
  test.skip(NOMAP.includes(info.project.name), 'no WebGL');
  const errors = await open(page, workout(NAME).view, { storage: { 'wx.units': 'imperial' } });
  await mapUp(page);
  await distanceAxis(page);
  await swimLeg(page);
  await hover(page, 0.5);
  expect(await cell(page, 'Distance')).toMatch(YD);
  expect(errors).toEqual([]);
});

test('F-72, YD-3: fullscreen strip shows yd on the swim leg', async ({ page }, info) => {
  test.skip(NOMAP.includes(info.project.name), 'no WebGL');
  const errors = await open(page, workout(NAME).view, { storage: { 'wx.units': 'imperial' } });
  await mapUp(page);
  await swimLeg(page);
  await hover(page, 0.5);
  await page.locator('button.maplibregl-ctrl-fullscreen').click();
  await expect(page.locator('html')).toHaveClass(/wx-fs/);
  const strip = page.locator('.wx-strip');
  await expect(strip).toBeVisible();
  await expect.poll(async () => (await texts(strip.locator('.cell'))).join('|')).toMatch(YD);
  await page.keyboard.press('Escape');
  expect(errors).toEqual([]);
});

// Opens the draft edit page on the swim leg; `plus(item, frac)` picks a spot, then an item of the "+" control.
async function editSwim(page, request) {
  const w = await upload(request, NAME);
  const errors = await open(page, w.edit, { storage: { 'wx.units': 'imperial' } });
  await mapUp(page);
  await swimLeg(page);
  const plus = async (item, frac) => {
    await hover(page, frac);
    const row = page.locator('.pin-tools .plus-item:visible');
    const map = page.locator('.wx-addpin .plus-button:visible');
    if (await row.count()) await page.getByRole('button', { name: item, exact: true }).click();
    else if (await map.count()) { await expect(map).toBeEnabled(); await map.click(); await page.getByRole('button', { name: item, exact: true }).click(); }
    else {
      const canvas = page.locator('#map-slot canvas').first();
      await canvas.scrollIntoViewIfNeeded();
      await canvas.click({ button: 'right', position: { x: 40, y: 80 } });
      await page.locator('.map-menu-item', { hasText: item }).click();
    }
  };
  return { errors, plus };
}

test('F-72, YD-8: edit page: a pin on the swim leg reads in yd', async ({ page, request }, info) => {
  test.skip(NOMAP.includes(info.project.name), 'no WebGL: the "+" lives on the map in two columns');
  const { errors, plus } = await editSwim(page, request);
  await plus('Add pin', 0.3);
  const dlg = page.locator('dialog.pin-dlg');
  await expect(dlg).toBeVisible();
  await dlg.locator('input').first().fill('Buoy');
  await dlg.locator('button.edit-save').click();
  await expect(dlg).toBeHidden();
  const pin = page.locator('.pins-slot .pin-item[data-pin] > .pin-row').first();
  await expect(pin).toContainText('Buoy');
  expect(await pin.textContent()).toMatch(YD);
  expect(errors).toEqual([]);
});

test('F-72, YD-8: edit page: an added lap on the swim leg in yd', async ({ page, request }, info) => {
  test.skip(NOMAP.includes(info.project.name), 'no WebGL: the "+" lives on the map in two columns');
  const { errors, plus } = await editSwim(page, request);
  await plus('Add lap', 0.55);
  await expect(page.locator('.added-laps .pin-item:not(.added-head)')).toHaveCount(1);
  expect((await texts(page.locator('.added-laps .added-row .pin-d'))).join(' ')).toMatch(YD);
  expect(errors).toEqual([]);
});

for (const [locale, units] of [['en-US', 'imperial'], ['de-DE', 'metric']]) {
  test.describe(locale, () => {
    test.use({ locale });
    test(`F-72, DS-1: no stored wx.units, ${locale} defaults to ${units}`, async ({ page }) => {
      const errors = await open(page, workout(NAME).view);
      await expect(page.locator('select', { hasText: 'Imperial' }).first()).toHaveValue(units);
      await distanceAxis(page);
      await swimLeg(page);
      await hover(page, 0.5);
      const d = await cell(page, 'Distance');
      if (units == 'imperial') expect(d).toMatch(YD); else expect(d).not.toMatch(/yd|mi\b/);
      expect(errors).toEqual([]);
    });
  });
}
