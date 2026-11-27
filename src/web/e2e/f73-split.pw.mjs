// F-73 (AP-17): the remembered map split is applied before the first frame, no layout jump.
import { test, expect } from '@playwright/test';
import { workout, open, desktopOnly } from './helpers.mjs';

const ratio = (page) => page.evaluate(() => {
  const m = document.querySelector('.map-slot').getBoundingClientRect();
  const l = document.querySelector('.layout').getBoundingClientRect();
  return { r: m.width / l.width, first: window.__first ? window.__first.w / window.__first.lw : null, cls: window.__clsOk ? window.__cls : null };
});

test.describe('F-73 wide screen', () => {
  test.use({ viewport: { width: 1920, height: 1080 } });
  desktopOnly(test, 'two columns need a wide window');

  for (const [pageName, link] of [['view', (w) => w.view], ['edit', (w) => w.edit]]) {
    test(`stored 60 %: first frame equals final, no layout shift (${pageName} page)`, async ({ page }) => {
      await open(page, link(workout('run-30min')), { storage: { 'wx.split.wide': '60' } });
      await page.waitForTimeout(1500);
      const s = await ratio(page);
      expect(s.r).toBeGreaterThan(0.585);
      expect(s.r).toBeLessThan(0.615);
      expect(Math.abs(s.first - s.r)).toBeLessThan(0.005);
      if (s.cls !== null) expect(s.cls).toBeLessThan(0.01);
    });
  }

  test('values outside 30 to 70 or not numbers fall back to the default split', async ({ page }) => {
    const w = workout('run-30min');
    await open(page, w.view);
    const def = (await ratio(page)).r;
    expect(def).not.toBeCloseTo(0.6, 1);
    for (const v of ['20', '80', 'abc']) {
      const p = await page.context().newPage();
      await open(p, w.view, { storage: { 'wx.split.wide': v } });
      expect(Math.abs((await ratio(p)).r - def), `stored ${v}`).toBeLessThan(0.005);
      await p.close();
    }
  });

  test('blocked storage: page works with the default split', async ({ page }) => {
    const errors = await open(page, workout('run-30min').view, { block: true });
    expect((await ratio(page)).r).toBeGreaterThan(0.2);
    expect(errors).toEqual([]); // blocked storage raises no error at all: nothing is ignored (QA-76-r1-2)
  });

  test('no route: stored split does not break the page', async ({ page }) => {
    await open(page, workout('indoor-no-gps').view, { storage: { 'wx.split.wide': '60' } });
    await expect(page.locator('.summary')).toBeVisible();
  });

  test('JavaScript off: server markup has the same default split', async ({ browser, baseURL }) => {
    const w = workout('run-30min');
    const on = await browser.newContext({ viewport: { width: 1920, height: 1080 } });
    const off = await browser.newContext({ viewport: { width: 1920, height: 1080 }, javaScriptEnabled: false });
    const measure = async (ctx) => {
      const p = await ctx.newPage();
      await p.goto(baseURL + w.view);
      await p.waitForTimeout(1500);
      return ratio(p);
    };
    const a = await measure(on);
    const b = await measure(off);
    expect(Math.abs(a.r - b.r)).toBeLessThan(0.01);
    await on.close();
    await off.close();
  });
});

test.describe('F-73 phone landscape', () => {
  test.use({ viewport: { width: 844, height: 390 } });
  desktopOnly(test, 'viewport emulation only on desktop projects');

  test('landscape uses its own stored key, not the wide one', async ({ page }) => {
    await open(page, workout('run-30min').view, { storage: { 'wx.split.landscape': '60', 'wx.split.wide': '35' } });
    const s = await ratio(page);
    expect(s.r).toBeGreaterThan(0.58);
    expect(s.r).toBeLessThan(0.62);
  });
});
