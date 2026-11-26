// B-10 (F-29 About, F-38 Impressum, F-28 Report form, F-37 report-delete page; INF-1, INF-2): static info pages, generic 404, Report dialog.
// The suite's server has no Pushover/ntfy and no Impressum file: nothing leaves the machine, and a delete link cannot be minted
// (it comes only from the operator notification), so the delete page is checked with an invalid key.
import { test, expect } from '@playwright/test';
import { workout, open } from './helpers.mjs';

test('F-29: About page lists the attributions, each with its licence text', async ({ page }) => {
  const errors = await open(page, '/about', { wait: false });
  await expect(page.locator('h1')).toHaveText('About Workout Explorer');
  const items = page.locator('.attributions li details');
  expect(await items.count()).toBeGreaterThan(3);
  await expect(items.first().locator('summary')).not.toBeEmpty();
  await items.first().locator('summary').click();
  await expect(items.first().locator('pre')).toBeVisible();
  expect((await items.first().locator('pre').textContent()).length).toBeGreaterThan(50);
  await expect(page.locator('footer a[href="/about"]')).toBeVisible();
  expect(errors).toEqual([]);
});

test('F-38: no Impressum page (bare 404 body) while the hoster set none; robots.txt disallows it', async ({ request }) => {
  const r = await request.get('/impressum');
  expect(r.status()).toBe(404);
  expect(await r.text()).toBe('404'); // Caddy's bare answer, no page
  expect(await (await request.get('/robots.txt')).text()).toContain('Disallow: /impressum');
});

test('F-28: the generic expired page for an unknown workout', async ({ page }) => {
  const r = await page.goto('/w/' + 'A'.repeat(22));
  expect(r.status()).toBe(404);
  await expect(page.locator('main p').first()).toContainText(/expired or is not available/);
  await expect(page.locator('footer a[href="/about"]')).toBeVisible();
  await expect(page.locator('.summary')).toHaveCount(0);
});

test('F-28, INF-2: Report form opens, validates, keeps text on cancel, answers on submit', async ({ page }) => {
  const errors = await open(page, workout('ride-30min').view);
  await expect(page.locator('footer a[href="/about"]')).toBeVisible();
  const dlg = page.locator('dialog.report');
  await expect(dlg).toHaveCount(0);
  await page.locator('button[data-report]').click();
  await expect(dlg).toBeVisible();
  await expect(dlg.locator('h2')).toHaveText('Report this workout');
  await expect(dlg.locator('input[name=reason]')).toHaveCount(4);
  // No reason chosen: the native validation stops the submit, no request goes out.
  let posts = 0;
  await page.route('**/api/reports', (r) => { posts++; return r.continue(); });
  await dlg.locator('button[type=submit]').click();
  await expect(dlg.locator('form')).toBeVisible();
  expect(posts).toBe(0);
  await expect(dlg.locator('textarea')).toHaveAttribute('maxlength', '1000');
  await dlg.locator('textarea').fill('hello');
  await expect(dlg.locator('.report-count')).toHaveText('5 / 1000');
  await dlg.locator('button[type=button]', { hasText: 'Cancel' }).click();
  await expect(dlg).toBeHidden();
  await page.locator('button[data-report]').click();
  await expect(dlg.locator('textarea')).toHaveValue('hello');
  // Submit: the server answers (no notifier is configured, so nothing leaves the machine); assert only what the UI shows.
  await dlg.locator('input[name=reason][value=spam]').check();
  await dlg.locator('button[type=submit]').click();
  await expect(dlg.locator('.report-done')).toBeVisible();
  await expect(dlg.locator('.report-done p').first()).toHaveText('Thank you. The report has been sent.');
  expect(posts).toBe(1);
  await dlg.locator('.report-done button').click();
  await expect(dlg).toBeHidden();
  expect(errors).toEqual([]);
  expect(errors.blocked.every((u) => u.startsWith('https://tiles.openfreemap.org/'))).toBe(true); // only the map's tile host, aborted
});

test('F-37: report-delete page with an invalid key shows the invalid-link answer and no button action', async ({ page }) => {
  const errors = await open(page, '/report-delete#' + 'A'.repeat(22), { wait: false });
  await expect(page.locator('h1')).toHaveText('Delete a reported workout');
  await expect(page.locator('#message')).toContainText('no longer valid');
  await expect(page.locator('#delete')).toBeDisabled();
  await expect(page.locator('#facts dt')).toHaveCount(0);
  // A malformed key never reaches the API.
  const p2 = await page.context().newPage();
  let api = 0;
  await p2.route('**/api/report-delete', (r) => { api++; return r.abort(); });
  await p2.goto('/report-delete#short');
  await expect(p2.locator('#message')).toContainText('no longer valid');
  expect(api).toBe(0);
  // 404 for the well-formed but unknown key is the console's own failed-resource line; everything else must be clean.
  expect(errors).toEqual([]);
});
