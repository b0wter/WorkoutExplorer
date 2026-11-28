// B-27 (F-27, INF-2): the viewer side of reporting that info-pages.pw.mjs leaves out: the rate-limit answer.
// The server's limit is 60 reports per hour per IP and the suite shares one IP, so the 429 is answered by a route with the
// server's real body; exhausting the real limit would break every later report test. The operator delete link exists only in
// the Pushover/ntfy notification (F-70, INF-6); the suite configures none, so that side is covered by Go tests, not here.
import { test, expect } from '@playwright/test';
import { workout, open } from './helpers.mjs';

test('F-27, INF-2: rate-limit answer is shown, the form keeps reason and text, a retry then succeeds', async ({ page }) => {
  const errors = await open(page, workout('ride-30min').view);
  let posts = 0;
  await page.route('**/api/reports', (r) => {
    if (++posts > 1) return r.continue();
    return r.fulfill({ status: 429, headers: { 'Retry-After': '3600' }, contentType: 'application/json', body: JSON.stringify({ error: 'rate_limited', message: 'Too many reports. Please try again later.' }) });
  });
  await page.locator('button[data-report]').click();
  const dlg = page.locator('dialog.report');
  await dlg.locator('input[name=reason]').first().check();
  await dlg.locator('textarea').fill('keep me');
  await dlg.locator('button[type=submit]').click();
  await expect(dlg.locator('.report-error')).toHaveText('Too many requests. Please try again later.');
  await expect(dlg.locator('.report-done')).toBeHidden();
  await expect(dlg.locator('textarea')).toHaveValue('keep me');
  await expect(dlg.locator('input[name=reason]').first()).toBeChecked();
  await dlg.locator('button[type=submit]').click();
  await expect(dlg.locator('.report-done')).toBeVisible();
  expect(posts).toBe(2);
  expect(errors.filter((e) => !/429/.test(String(e)))).toEqual([]);
});
