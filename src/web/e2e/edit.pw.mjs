// B-04 (F-19 edit page, F-45 suggested title, F-21 lifetime and expiry; UP-4, UP-7, SH-6): fields, visibility, lifetime, token handling.
import { test, expect } from '@playwright/test';
import { upload, open } from './helpers.mjs';

const form = (page) => page.locator('form.edit-panel');
const save = async (page) => {
  await form(page).locator('button.edit-save[type=submit]').click();
  await expect(form(page).locator('.edit-status')).toHaveText('Saved');
};

test('F-19: title and description save and survive a reload', async ({ page, request }) => {
  const w = await upload(request, 'ride-30min');
  const errors = await open(page, w.edit, { wait: false });
  await expect(form(page)).toBeVisible();
  await form(page).locator('input[name=title]').fill('Morning spin');
  await form(page).locator('textarea[name=description]').fill('Easy legs, windy.');
  await save(page);
  await page.reload();
  await expect(form(page).locator('input[name=title]')).toHaveValue('Morning spin');
  await expect(form(page).locator('textarea[name=description]')).toHaveValue('Easy legs, windy.');
  expect(errors).toEqual([]);
});

test('F-45: the suggested title shows as placeholder while the title is empty', async ({ page, request }) => {
  const w = await upload(request, 'run-30min');
  const errors = await open(page, w.edit, { wait: false });
  const title = form(page).locator('input[name=title]');
  await expect(title).toHaveValue('');
  const ph = await title.getAttribute('placeholder');
  expect(ph).toMatch(/\S/);
  expect(ph).toMatch(/run/i);
  expect(errors).toEqual([]);
});

test('F-19, F-21: visibility starts as Draft; the lifetime select saves and the expiry text follows it', async ({ page, request }) => {
  const w = await upload(request, 'ride-30min');
  const errors = await open(page, w.edit, { wait: false });
  await expect(form(page).locator('.edit-field', { hasText: 'Visibility' })).toContainText('Draft');
  const sel = form(page).locator('select[name=lifetime]');
  const opts = await sel.locator('option').allTextContents();
  expect(opts.length).toBeGreaterThan(1);
  const pick = opts[0] === (await sel.inputValue()) ? opts[1] : opts[0];
  const before = await form(page).locator('.edit-expiry').textContent();
  await sel.selectOption(pick);
  await expect(form(page).locator('.edit-expiry')).not.toHaveText(before);
  await save(page);
  await page.waitForLoadState('networkidle'); // a reload while lazy chunks load aborts them: WebKit logs that as a page error
  await page.reload();
  await expect(sel).toHaveValue(pick);
  // Publishing flips the visibility label.
  await request.post(`/api/workouts/${w.id}/publish`, { headers: { Authorization: 'Bearer ' + w.tok } });
  await page.waitForLoadState('networkidle');
  await page.reload();
  await expect(form(page).locator('.edit-field', { hasText: 'Visibility' })).toContainText('Unlisted');
  expect(errors).toEqual([]);
});

test('F-21, UP-7: the viewing page shows the expiry date, and it follows the saved lifetime', async ({ page, request }) => {
  const w = await upload(request, 'ride-30min', { publish: true });
  const errors = await open(page, w.view);
  const exp = page.locator('time[data-expires]');
  await expect(exp).toHaveText(/^Expires in \d+ days?$/);
  const days = (s) => Number(/(\d+)/.exec(s)[1]);
  const first = days(await exp.textContent());
  const r = await request.patch(`/api/workouts/${w.id}`, { headers: { Authorization: 'Bearer ' + w.tok }, data: { lifetime_days: 3 } });
  expect(r.ok()).toBe(true);
  await page.reload();
  const second = days(await page.locator('time[data-expires]').textContent());
  expect(second).toBeLessThanOrEqual(3);
  expect(second).toBeLessThanOrEqual(first);
  expect(errors).toEqual([]);
});

test('F-19, UP-6: the edit token stays in the fragment: no request has it in the path or query', async ({ page, request }) => {
  const w = await upload(request, 'ride-30min');
  const urls = [];
  page.on('request', (r) => urls.push(r.url()));
  const errors = await open(page, w.edit, { wait: false });
  await form(page).locator('input[name=title]').fill('Token check');
  await save(page);
  expect(urls.length).toBeGreaterThan(3);
  expect(urls.filter((u) => u.includes(w.tok))).toEqual([]);
  expect(await page.evaluate(() => location.hash)).toContain(w.tok);
  expect(await page.evaluate(() => location.search)).toBe('');
  expect(errors).toEqual([]);
});
