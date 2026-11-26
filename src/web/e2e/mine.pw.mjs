// B-09 (F-53 My workouts): empty state, an upload through the UI is listed, survives a reload, Forget empties the list.
import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import { open } from './helpers.mjs';
import { fitPath } from './fixtures.mjs';

const EMPTY = 'No workouts uploaded from this browser yet.';

test('F-53: list is empty at first; an upload is listed, survives a reload, Forget removes it', async ({ page: up, context }) => {
  // The edit page's own lazy chunks abort when it is left (B-21), so the list is checked on a separate page; storage is shared.
  let page = await context.newPage();
  const errors = await open(page, '/my-workouts', { wait: false });
  await expect(page.locator('main.mine h1')).toHaveText('My workouts');
  await expect(page.locator('main.mine')).toContainText(EMPTY);
  await expect(page.locator('main.mine table')).toHaveCount(0);

  page = up;
  await page.goto('/');
  await expect(page.locator('#page')).toHaveAttribute('data-state', 'ready');
  await page.locator('#file').setInputFiles({ name: 'ride-30min.fit', mimeType: 'application/octet-stream', buffer: fs.readFileSync(fitPath('ride-30min')) });
  await page.waitForURL(/\/edit\/[0-9A-Za-z]{22}#/);
  const id = new URL(page.url()).pathname.split('/').pop();
  await expect(page.locator('.chart-stack canvas').first()).toBeVisible();

  page = await context.newPage();
  const errors2 = await open(page, '/my-workouts', { wait: false });
  const rows = page.locator('main.mine tbody tr');
  await expect(rows).toHaveCount(1);
  await expect(rows.first()).toContainText('Draft');
  await expect(rows.first().locator('a[href^="/edit/' + id + '"]')).toBeVisible();
  await expect(page.locator('main.mine')).not.toContainText(EMPTY);

  await page.reload();
  await expect(rows).toHaveCount(1);
  await expect(rows.first().locator('a[href^="/edit/' + id + '"]')).toBeVisible();

  await rows.first().locator('[data-act^="forget:"]').click();
  await expect(page.locator('main.mine')).toContainText(EMPTY);
  await expect(page.locator('main.mine table')).toHaveCount(0);
  await page.reload();
  await expect(page.locator('main.mine')).toContainText(EMPTY);
  expect(errors).toEqual([]);
  expect(errors2).toEqual([]);
});
