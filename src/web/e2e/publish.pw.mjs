// B-05 (F-20 publish, share, delete; F-12 viewing page; SH-6, UP-6): Publish, the published page, Share and Copy link, edit link, Delete.
import { test, expect } from '@playwright/test';
import { upload, open } from './helpers.mjs';

const act = (page, a) => page.locator(`[data-act="${a}"]`);
const publish = async (page) => {
  await act(page, 'publish').click();
  await expect(page.locator('main.published h1')).toHaveText('Workout published');
};

test('F-20: Publish opens the published page with both links, the warning and Go there', async ({ page, request }) => {
  const w = await upload(request, 'ride-30min');
  const errors = await open(page, w.edit, { wait: false });
  await expect(act(page, 'share')).toHaveCount(0);
  await publish(page);
  const origin = new URL(page.url()).origin;
  await expect(page.locator('main.published .view-box .edit-link')).toHaveText(origin + w.view);
  await expect(page.locator('main.published .edit-warn [translate=no]')).toHaveText(origin + w.edit);
  await expect(page.locator('main.published .edit-warn')).toContainText('Save this link');
  await expect(page.locator('main.published .view-box a.go')).toHaveAttribute('href', w.view);
  await expect(act(page, 'delete')).toBeVisible();
  expect((await request.get(w.view)).ok()).toBe(true);
  expect(errors).toEqual([]);
});

test('F-20, SH-6: Share on the edit page shows the viewing link; Copy link copies exactly it', async ({ page, request, context, browserName }) => {
  const w = await upload(request, 'ride-30min', { publish: true });
  const canRead = browserName === 'chromium';
  if (canRead) await context.grantPermissions(['clipboard-read', 'clipboard-write']);
  const errors = await open(page, w.edit, { wait: false });
  const origin = new URL(page.url()).origin;
  await expect(page.locator('.edit-share')).toHaveCount(0);
  await act(page, 'share').click();
  await expect(page.locator('.edit-share .edit-link')).toHaveText(origin + w.view);
  await page.locator('.edit-share a[data-copy-link]').click();
  if (canRead) await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(origin + w.view);
  // Elsewhere the dialog's link text above is the check; the token never shows in the share box.
  await expect(page.locator('.edit-share')).not.toContainText(w.tok);
  await act(page, 'share').click();
  await expect(page.locator('.edit-share')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('F-20, F-12: the viewing link opens the published workout page', async ({ page, request }) => {
  const w = await upload(request, 'ride-30min');
  const errors = await open(page, w.edit, { wait: false });
  await publish(page);
  await page.locator('main.published .view-box a.go').click();
  await expect(page).toHaveURL(new RegExp(w.view + '$'));
  await page.locator('.chart-stack canvas').first().waitFor({ state: 'visible' });
  await expect(page.locator('.summary')).toBeVisible();
  await expect(page.locator('.edit-panel')).toHaveCount(0);
  expect(errors).toEqual([]);
});

test('F-20: after publishing, the edit link still opens the edit view', async ({ page, request }) => {
  const w = await upload(request, 'ride-30min');
  const errors = await open(page, w.edit, { wait: false });
  await publish(page);
  await act(page, 'back').click(); // Go there in the edit-link box
  await expect(page).toHaveURL(new RegExp('/edit/' + w.id + '#'));
  await expect(page.locator('form.edit-panel')).toBeVisible();
  await expect(page.locator('form.edit-panel .edit-field', { hasText: 'Visibility' })).toContainText('Unlisted');
  await expect(act(page, 'publish')).toHaveCount(0);
  await expect(act(page, 'share')).toBeVisible();
  expect(errors).toEqual([]);
});

test('F-20: Delete asks first, then removes the workout; the viewing link shows the generic gone page', async ({ page, request }) => {
  const w = await upload(request, 'ride-30min', { publish: true });
  const errors = await open(page, w.edit, { wait: false });
  await act(page, 'delete').click();
  await expect(page.locator('.edit-confirm')).toContainText('permanently');
  await act(page, 'cancel').click();
  await expect(page.locator('.edit-confirm')).toHaveCount(0);
  expect((await request.get(w.view)).status()).toBe(200);
  await act(page, 'delete').click();
  await act(page, 'confirm').click();
  await expect(page.locator('main.generic p').first()).toHaveText('This workout has been deleted.');
  await expect(page.locator('main.generic a[href="/"]')).toBeVisible();
  const gone = await page.goto(w.view);
  expect(gone.status()).toBe(404);
  await expect(page.locator('main p').first()).toContainText(/expired or is not available/);
  await expect(page.locator('.summary')).toHaveCount(0);
  expect(errors).toEqual([]);
});
