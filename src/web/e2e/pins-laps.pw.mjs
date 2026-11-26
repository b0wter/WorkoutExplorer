// B-07 (F-56 pins, F-65 owner-added laps, F-66 remove auto-laps; PIN-1, PIN-3, PIN-4, AP-8): the edit page of a fresh draft.
// The "+" sits on the map in two columns (needs WebGL: not in desktop Firefox) and in a row under the map in one column (all projects).
import { test, expect } from '@playwright/test';
import { upload, open, hover } from './helpers.mjs';

const form = (page) => page.locator('form.edit-panel');
const rows = (page) => page.locator('section.laps tbody tr');
const bars = (page) => page.locator('section.laps svg rect.bar');
const pinRows = (page) => page.locator('.pins-slot .pin-item[data-pin] > .pin-row');
const dlg = (page) => page.locator('dialog.pin-dlg');
const save = async (page) => {
  await form(page).locator('button.edit-save[type=submit]').click();
  await expect(form(page).locator('.edit-status')).toHaveText('Saved');
};

/** Opens the draft and tells where the "+" cannot be reached (two columns need the map's WebGL). */
async function edit(page, request, info, name = 'ride-30min') {
  const w = await upload(request, name);
  const errors = await open(page, w.edit, { storage: { 'wx.units': 'metric' } });
  return { w, errors, skipMap: info.project.name === 'firefox' }; // desktop Firefox is two columns; firefox-phone is one
}

/** Picks a spot with the chart cursor, then an item of the "+" control: open buttons in the row under the map (one column),
 *  the "+" on the map (two columns, touch), or the map's right-click menu (two columns, mouse: the "+" is hidden there). */
async function plus(page, item, frac = 0.3) {
  await hover(page, frac);
  const row = page.locator('.pin-tools .plus-item:visible');
  const map = page.locator('.wx-addpin .plus-button:visible');
  if (await row.count()) {
    await page.getByRole('button', { name: item, exact: true }).click();
  } else if (await map.count()) {
    await expect(map).toBeEnabled();
    await map.click();
    await page.getByRole('button', { name: item, exact: true }).click();
  } else {
    const canvas = page.locator('#map-slot canvas').first();
    await canvas.scrollIntoViewIfNeeded();
    await canvas.click({ button: 'right', position: { x: 40, y: 80 } });
    await page.locator('.map-menu-item', { hasText: item }).click();
  }
}

async function addPin(page, title, frac) {
  const n = await pinRows(page).count();
  await plus(page, 'Add pin', frac);
  await expect(dlg(page)).toBeVisible();
  await dlg(page).locator('input').first().fill(title);
  await dlg(page).locator('button.edit-save').click();
  await expect(dlg(page)).toBeHidden();
  await expect(pinRows(page)).toHaveCount(n + 1);
}

test('F-56, PIN-1: add a pin through the dialog; the list shows it; Save keeps it over a reload', async ({ page, request }, info) => {
  const { errors, skipMap } = await edit(page, request, info);
  test.skip(skipMap, 'no WebGL: the "+" lives on the map in two columns');
  await plus(page, 'Add pin');
  await expect(dlg(page)).toBeVisible();
  await expect(dlg(page).locator('select option')).not.toHaveCount(0);
  await dlg(page).locator('input').first().fill('Café stop');
  await dlg(page).locator('textarea').fill('Good coffee.');
  await dlg(page).locator('button.edit-save').click();
  await expect(dlg(page)).toBeHidden();
  await expect(pinRows(page)).toHaveCount(1);
  await expect(pinRows(page).first()).toContainText('Café stop');
  await save(page);
  await page.waitForLoadState('networkidle');
  await page.reload();
  await expect(pinRows(page)).toHaveCount(1);
  await expect(pinRows(page).first()).toContainText('Café stop');
  expect(errors).toEqual([]);
});

test('F-56: the pin dialog: Cancel and Esc change nothing; clicking the row edits; OK applies; Delete removes', async ({ page, request, browserName }, info) => {
  const { errors, skipMap } = await edit(page, request, info);
  test.skip(skipMap, 'no WebGL: the "+" lives on the map in two columns');
  await plus(page, 'Add pin');
  await expect(dlg(page)).toBeVisible();
  await dlg(page).locator('input').first().fill('Never');
  await dlg(page).getByRole('button', { name: 'Cancel' }).click();
  await expect(dlg(page)).toBeHidden();
  await expect(pinRows(page)).toHaveCount(0);
  await plus(page, 'Add pin');
  await expect(dlg(page)).toBeVisible();
  // Playwright's Linux WebKit does not close a modal dialog on a synthetic Escape (unverified in a real Safari): Esc is checked elsewhere.
  if (browserName === 'webkit') await dlg(page).getByRole('button', { name: 'Cancel' }).click();
  else await page.keyboard.press('Escape');
  await expect(dlg(page)).toBeHidden();
  await expect(pinRows(page)).toHaveCount(0);
  await addPin(page, 'First', 0.3);
  await pinRows(page).first().click();
  await expect(dlg(page)).toBeVisible();
  await expect(dlg(page).locator('input').first()).toHaveValue('First');
  await expect(dlg(page).getByRole('button', { name: 'Delete pin' })).toBeVisible();
  await dlg(page).locator('input').first().fill('Renamed');
  await dlg(page).locator('button.edit-save').click();
  await expect(pinRows(page).first()).toContainText('Renamed');
  await pinRows(page).first().click();
  await dlg(page).getByRole('button', { name: 'Delete pin' }).click();
  await expect(dlg(page)).toBeHidden();
  await expect(pinRows(page)).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('F-56: move a pin (the app has no drag: delete and add elsewhere; the list stays sorted along the route); the row x deletes', async ({ page, request }, info) => {
  const { errors, skipMap } = await edit(page, request, info);
  test.skip(skipMap, 'no WebGL: the "+" lives on the map in two columns');
  await addPin(page, 'B', 0.7);
  await addPin(page, 'A', 0.2);
  await expect(pinRows(page).nth(0)).toContainText('A');
  await expect(pinRows(page).nth(1)).toContainText('B');
  const where = async (k) => (await pinRows(page).nth(k).locator('.pin-d').textContent()).trim();
  const before = await where(1);
  await page.locator('.pins-slot .pin-item[data-pin="1"] .added-x-btn').click();
  await expect(pinRows(page)).toHaveCount(1);
  await addPin(page, 'B', 0.9);
  expect(await where(1)).not.toBe(before);
  await expect(pinRows(page).nth(1)).toContainText('B');
  await page.locator('.pins-slot .pin-item[data-pin="0"] .added-x-btn').click();
  await expect(pinRows(page)).toHaveCount(1);
  await expect(pinRows(page).first()).toContainText('B');
  expect(errors).toEqual([]);
});

test('F-65: add a lap boundary: the laps table and comparison chart gain a lap; the added table lists it; x removes it', async ({ page, request }, info) => {
  const { errors, skipMap } = await edit(page, request, info);
  test.skip(skipMap, 'no WebGL: the "+" lives on the map in two columns');
  await page.locator('section.laps').scrollIntoViewIfNeeded();
  const n0 = await rows(page).count();
  const b0 = await bars(page).count();
  await plus(page, 'Add lap', 0.45); // 0.5 is a device lap end (every 5 min)
  await expect(rows(page)).toHaveCount(n0 + 1);
  await expect(page.locator('.added-laps')).toBeVisible();
  await expect(page.locator('.added-laps .pin-item:not(.added-head)')).toHaveCount(1);
  if (b0) await expect(bars(page)).toHaveCount(b0 + 1);
  await page.locator('section.laps .lap-x-btn').click();
  await expect(rows(page)).toHaveCount(n0);
  await expect(page.locator('.added-laps')).toBeHidden();
  if (b0) await expect(bars(page)).toHaveCount(b0);
  await plus(page, 'Add lap', 0.4);
  await expect(rows(page)).toHaveCount(n0 + 1);
  await page.locator('.added-laps .added-x-btn').click();
  await expect(rows(page)).toHaveCount(n0);
  await plus(page, 'Add lap', 0.6);
  await expect(rows(page)).toHaveCount(n0 + 1);
  await save(page);
  await page.waitForLoadState('networkidle');
  await page.reload();
  await expect(rows(page)).toHaveCount(n0 + 1);
  expect(errors).toEqual([]);
});

test('F-66: Remove auto-laps merges the auto laps (table and chart follow); Restore brings them back', async ({ page, request }, info) => {
  const { errors } = await edit(page, request, info, 'ride-lap-triggers');
  const auto = page.locator('.laps-auto-btn');
  await page.locator('section.laps').scrollIntoViewIfNeeded();
  await expect(auto).toHaveText(/^Remove auto-laps \(\d+\)$/);
  const n0 = await rows(page).count();
  const b0 = await bars(page).count();
  const k = Number(/\((\d+)\)/.exec(await auto.textContent())[1]);
  expect(n0).toBe(5);
  await auto.click();
  await expect(auto).toHaveText('Restore auto-laps');
  await expect(rows(page)).toHaveCount(n0 - k);
  if (b0) await expect(bars(page)).toHaveCount(n0 - k);
  await save(page);
  await page.waitForLoadState('networkidle');
  await page.reload();
  await expect(auto).toHaveText('Restore auto-laps');
  await expect(rows(page)).toHaveCount(n0 - k);
  await auto.click();
  await expect(auto).toHaveText(/^Remove auto-laps/);
  await expect(rows(page)).toHaveCount(n0);
  expect(errors).toEqual([]);
});
