// F-75 (ADR-011 decision 7): the range panel and the map-only glue arrive in lazy chunks, without a layout jump.
import { test, expect } from '@playwright/test';
import { workout, open, drag, desktopOnly } from './helpers.mjs';

test.describe('F-75 range panel (T-1)', () => {
  test('a lap tap shows the panel when its chunk lands', async ({ page }) => {
    await open(page, workout('run-30min').view);
    await expect(page.locator('section.laps.live')).toBeVisible();
    await page.locator('section.laps tbody tr').first().click();
    await expect(page.locator('#range-slot .range-panel')).toBeVisible();
  });

  test('before any range the slot is empty and takes no room', async ({ page }) => {
    await open(page, workout('run-30min').view);
    await page.waitForTimeout(500);
    const box = await page.locator('#range-slot').boundingBox();
    expect(box?.height ?? 0).toBe(0);
  });

  test.describe('mouse', () => {
    test.beforeEach(({ isMobile }) => { test.skip(isMobile, 'range drag'); });
    test('a dragged range shows the panel', async ({ page }) => {
      await open(page, workout('run-30min').view);
      await drag(page, 0.2, 0.5);
      await expect(page.locator('.range-panel')).toBeVisible();
    });
  });
});

test.describe('F-75 map glue (T-2, T-3)', () => {
  test.use({ viewport: { width: 1920, height: 1080 } });
  desktopOnly(test, 'two columns need a wide window');

  test('the splitter handle arrives with the map and drags the split', async ({ page, browserName }) => {
    test.skip(browserName === 'firefox', 'headless Firefox has no WebGL: the map never starts');
    await open(page, workout('run-30min').view);
    const handle = page.locator('#map-slot .splitter');
    await expect(handle).toBeAttached({ timeout: 20000 });
    const w0 = (await page.locator('.map-slot').boundingBox()).width;
    const bb = await handle.boundingBox();
    await page.mouse.move(bb.x + bb.width / 2, bb.y + bb.height / 2);
    await page.mouse.down();
    await page.mouse.move(bb.x + bb.width / 2 + 200, bb.y + bb.height / 2, { steps: 6 });
    await page.mouse.up();
    await expect.poll(async () => (await page.locator('.map-slot').boundingBox()).width).toBeGreaterThan(w0 + 100);
  });

  test('a workout with a route carries the colouring tokens; one without downloads no map code', async ({ page, browserName }) => {
    test.skip(browserName === 'firefox', 'headless Firefox has no WebGL: the map never starts');
    await open(page, workout('run-30min').view);
    await expect.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--seq-0').trim()), { timeout: 20000 }).not.toBe('');
    const p2 = await page.context().newPage();
    const seen = [];
    p2.on('request', (r) => seen.push(r.url()));
    await open(p2, workout('indoor-no-gps').view);
    await p2.waitForTimeout(1000);
    expect(await p2.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--seq-0').trim())).toBe('');
    expect(seen.filter((u) => /map-view|map-style|map-glue/.test(u))).toEqual([]);
    expect(await p2.locator('.splitter, .map-pin, .wx-strip').count()).toBe(0);
  });
});
