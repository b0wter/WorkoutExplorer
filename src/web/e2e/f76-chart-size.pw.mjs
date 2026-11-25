// F-76 (DS-8): taller charts. Toggle 120 -> 200 -> 120 px, remembered, blocked storage, landscape cap,
// no layout jump with tall stored, no CSP violation. Synthetic workouts only.
import { test, expect } from '@playwright/test';
import { workout, open } from './helpers.mjs';

const KEY = 'wx.charts';
const toggle = (page) => page.locator('.size-toggle');
const plots = (page) => page.locator('.chart-area .u-over').evaluateAll((os) => os.map((o) => Math.round(o.getBoundingClientRect().height)));
const expectPlots = async (page, px) => {
  await expect.poll(async () => [...new Set(await plots(page))], { message: `plot heights, want ${px}` }).toEqual([px]);
};
const stored = (page) => page.evaluate((k) => localStorage.getItem(k), KEY);
// The viewport matrix needs setViewportSize only; two engines are enough for it.
const matrixOnly = (info) => test.skip(!['chromium', 'webkit'].includes(info.project.name), 'viewport matrix runs on chromium and webkit');

test.describe('F-76 toggle', () => {
  test('fresh browser: 120 px, not pressed; toggle 120 -> 200 -> 120, every stacked chart, the key follows', async ({ page }) => {
    const errors = await open(page, workout('ride-30min').view);
    await expect(toggle(page)).toHaveAttribute('aria-pressed', 'false');
    await expectPlots(page, 120);
    expect(await page.locator('.chart-area .u-over').count()).toBeGreaterThan(1);
    await toggle(page).click();
    await expect(toggle(page)).toHaveAttribute('aria-pressed', 'true');
    await expectPlots(page, 200);
    expect(await stored(page)).toBe('tall');
    await toggle(page).click();
    await expect(toggle(page)).toHaveAttribute('aria-pressed', 'false');
    await expectPlots(page, 120);
    expect(await stored(page)).toBeNull();
    expect(errors).toEqual([]);
  });

  test('comparison chart keeps its size; the page below the charts moves, nothing else resizes', async ({ page }) => {
    await open(page, workout('interval-run').view, { storage: { 'wx.units': 'metric' } });
    const svg = page.locator('section.laps svg').filter({ has: page.locator('rect.bar') }).first();
    await expect(svg).toBeVisible();
    const h = async () => Math.round((await svg.boundingBox()).height);
    const before = await h();
    await toggle(page).click();
    await expectPlots(page, 200);
    expect(await h()).toBe(before);
  });

  test('remembered: reload, another workout and the edit page open tall; nothing in any request', async ({ page }) => {
    const sent = [];
    page.on('request', (r) => sent.push(r.url() + ' ' + JSON.stringify(r.headers()) + (r.postData() ?? '')));
    const a = workout('ride-30min');
    await open(page, a.view);
    await toggle(page).click();
    await expectPlots(page, 200);
    await page.reload();
    await expect(toggle(page)).toHaveAttribute('aria-pressed', 'true');
    await expectPlots(page, 200);
    await page.goto(workout('run-30min').view);
    await expectPlots(page, 200);
    await page.goto(a.edit);
    await expect(toggle(page)).toHaveAttribute('aria-pressed', 'true');
    await expectPlots(page, 200);
    expect(await page.url()).not.toMatch(/tall|chart/i);
    expect(sent.filter((s) => /wx\.charts|tall|chart-size|chartSize/i.test(s))).toEqual([]);
    await toggle(page).click(); // off again on the edit page
    await expectPlots(page, 120);
    await page.goto(a.view);
    await expectPlots(page, 120);
  });

  test('a stored value other than "tall" reads as normal', async ({ page }) => {
    const errors = await open(page, workout('ride-30min').view, { storage: { [KEY]: 'Tall' } });
    await expectPlots(page, 120);
    await expect(toggle(page)).toHaveAttribute('aria-pressed', 'false');
    expect(errors).toEqual([]);
  });

  test('blocked storage: 120 px, no error, the toggle still works for this page', async ({ page }) => {
    const errors = await open(page, workout('ride-30min').view, { block: true });
    await expectPlots(page, 120);
    await toggle(page).click();
    await expectPlots(page, 200);
    await toggle(page).click();
    await expectPlots(page, 120);
    expect(errors.filter((e) => !/localStorage|blocked/.test(e))).toEqual([]);
  });

  test('a workout without distance has the toggle alone in the toolbar row; the leg view keeps the size', async ({ page }) => {
    await open(page, workout('indoor-no-gps').view);
    await expect(toggle(page)).toBeVisible();
    await expectPlots(page, 120);
    const tri = await page.context().newPage();
    await open(tri, workout('triathlon').view, { storage: { [KEY]: 'tall' } });
    await expectPlots(tri, 200);
    await expect(tri.locator('.wx-leg-pick option')).toHaveText(['Whole workout', 'Swim', 'Ride', 'Run']);
    // Picker before the size toggle, the toggle last.
    expect(await tri.locator('.toolbar > *').evaluateAll((els) => els.map((e) => e.classList.contains('axis-toggle') ? 'axis' : e.classList.contains('wx-leg-pick') ? 'pick' : 'size'))).toEqual(['axis', 'pick', 'size']);
    await tri.locator('.wx-leg-pick').selectOption({ label: 'Run' });
    await expectPlots(tri, 200);
    await expect(toggle(tri)).toHaveAttribute('aria-pressed', 'true');
  });

  test('cursor and range still work at the tall size (mouse and touch)', async ({ page, isMobile }) => {
    await open(page, workout('ride-30min').view, { storage: { [KEY]: 'tall' } });
    const over = page.locator('.chart-area .u-over').first();
    await over.scrollIntoViewIfNeeded();
    const bb = await over.boundingBox();
    expect(Math.round(bb.height)).toBe(200);
    if (isMobile) await page.touchscreen.tap(bb.x + bb.width / 2, bb.y + 100);
    else await page.mouse.move(bb.x + bb.width / 2, bb.y + 100);
    await expect(page.locator('.chart-area .wx-cursor-line').first()).toHaveCSS('visibility', 'visible');
    if (!isMobile) {
      await page.mouse.down();
      await page.mouse.move(bb.x + bb.width * 0.7, bb.y + 100, { steps: 6 });
      await page.mouse.up();
      await expect(page.locator('.range-panel')).toBeVisible();
    }
  });

  test('the label fits at 360 px in every language (no horizontal scroll, button inside the row)', async ({ page }, info) => {
    matrixOnly(info);
    await page.setViewportSize({ width: 360, height: 740 });
    for (const lang of ['en', 'de', 'es', 'fr']) {
      const p = await page.context().newPage();
      await p.setViewportSize({ width: 360, height: 740 });
      await open(p, workout('triathlon').view, { storage: { 'wx.lang': lang } });
      await expect(p.locator('.wx-leg-pick')).toBeVisible();
      const m = await p.evaluate(() => {
        const b = document.querySelector('.size-toggle').getBoundingClientRect();
        const bar = document.querySelector('.toolbar').getBoundingClientRect();
        return { label: document.querySelector('.size-toggle').textContent, right: b.right, barRight: bar.right, left: b.left, barLeft: bar.left, scroll: document.documentElement.scrollWidth, w: document.documentElement.clientWidth, clipped: document.querySelector('.size-toggle').scrollWidth > document.querySelector('.size-toggle').clientWidth };
      });
      expect(m.label.length, lang).toBeGreaterThan(3);
      expect(m.right, lang).toBeLessThanOrEqual(m.barRight + 1);
      expect(m.left, lang).toBeGreaterThanOrEqual(m.barLeft - 1);
      expect(m.scroll, lang).toBeLessThanOrEqual(m.w);
      expect(m.clipped, lang).toBe(false);
      await p.close();
    }
  });
});

test.describe('F-76 sizes per device', () => {
  test.beforeEach(async ({}, info) => matrixOnly(info));
  const sizes = [
    ['iPhone portrait', 375, 812],
    ['iPad portrait', 820, 1180],
    ['desktop', 1440, 900],
  ];
  for (const [name, width, height] of sizes) {
    test(`${name} ${width} x ${height}: 120 px, tall 200 px, no horizontal scroll`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      const errors = await open(page, workout('ride-30min').view);
      await expectPlots(page, 120);
      await toggle(page).click();
      await expectPlots(page, 200);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
      expect(errors).toEqual([]);
    });
  }

  test('iPhone landscape 812 x 375: tall is capped so one chart (readout, title row, plot) fits the window', async ({ page }) => {
    await page.setViewportSize({ width: 812, height: 375 });
    await open(page, workout('ride-30min').view, { storage: { [KEY]: 'tall' } });
    const m = await page.evaluate(() => {
      const cs = getComputedStyle(document.querySelector('.charts'));
      const plot = document.querySelector('.chart-area .u-over').getBoundingClientRect().height;
      return { plot, plotCss: cs.getPropertyValue('--plot'), r: Number(cs.getPropertyValue('--r')), h: innerHeight };
    });
    expect(m.plot).toBeGreaterThanOrEqual(120);
    expect(m.plot).toBeLessThanOrEqual(200);
    expect(m.plot + 24 + m.r * 29 + 16).toBeLessThanOrEqual(m.h + 1);
    expect(Math.round(m.plot)).toBe(Math.min(200, Math.max(120, Math.round(m.h - m.r * 29 - 40))));
  });

  test('a very short landscape window caps below 200 px, never below 120 px, and the toggle still works', async ({ page }) => {
    await page.setViewportSize({ width: 812, height: 260 });
    await open(page, workout('triathlon').view, { storage: { [KEY]: 'tall' } });
    const plot = async () => Math.round(await page.locator('.chart-area .u-over').first().evaluate((o) => o.getBoundingClientRect().height));
    const capped = await plot();
    expect(capped).toBeGreaterThanOrEqual(120);
    expect(capped).toBeLessThan(200);
    await toggle(page).click();
    await expectPlots(page, 120);
  });

  test('rotating from landscape to portrait re-reads the height', async ({ page }) => {
    await page.setViewportSize({ width: 812, height: 260 });
    await open(page, workout('triathlon').view, { storage: { [KEY]: 'tall' } });
    await page.setViewportSize({ width: 375, height: 812 });
    await expectPlots(page, 200);
  });
});

test.describe('F-76 no layout jump, no CSP violation', () => {
  test('tall stored: the element below the charts is where it will stay at the first frame, and no layout shift', async ({ page }) => {
    await page.addInitScript(() => {
      window.__below = [];
      const tick = () => {
        const next = document.querySelector('section.laps'); // pre-rendered below the charts
        if (next && [...document.querySelectorAll('link[rel=stylesheet]')].every((l) => l.sheet)) {
          window.__below.push(Math.round(next.getBoundingClientRect().top + scrollY));
          if (window.__below.length < 3 && !document.querySelector('.chart-stack canvas')) requestAnimationFrame(tick);
        } else requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    });
    const errors = await open(page, workout('ride-30min').view, { storage: { [KEY]: 'tall' } });
    await expectPlots(page, 200);
    await page.waitForTimeout(1000);
    const r = await page.evaluate(() => ({ first: window.__below[0], last: Math.round(document.querySelector('section.laps').getBoundingClientRect().top + scrollY), cls: window.__clsOk ? window.__cls : null }));
    expect(r.first).toBeGreaterThan(0);
    expect(r.last).toBe(r.first);
    if (r.cls !== null) expect(r.cls).toBeLessThan(0.01);
    expect(errors).toEqual([]);
  });

  test('JavaScript off: the server markup reserves the normal size', async ({ browser, baseURL }) => {
    const ctx = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 1000, height: 800 } });
    const p = await ctx.newPage();
    await p.goto(baseURL + workout('ride-30min').view);
    expect(await p.evaluate(() => getComputedStyle(document.querySelector('.charts')).getPropertyValue('--plot'))).toBe('120px');
    await ctx.close();
  });

  for (const [name, url] of [['viewing', () => workout('ride-30min').view], ['edit', () => workout('ride-30min').edit], ['upload', () => '/'], ['About', () => '/about']]) {
    test(`no CSP violation on the ${name} page with tall stored`, async ({ page }) => {
      const errors = await open(page, url(), { storage: { [KEY]: 'tall' }, wait: name === 'viewing' || name === 'edit' });
      await page.waitForTimeout(500);
      expect(errors).toEqual([]);
    });
  }
});
