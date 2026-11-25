// Dialog shell (F-39 how to, F-58 summary explanation): header stays while the body scrolls, x closes, page behind does not scroll.
import { test, expect } from '@playwright/test';
import { workout, open } from './helpers.mjs';

for (const trigger of ['[data-howto]', '[data-summary-info]']) {
  test(`dialog ${trigger}: fixed header, scrolling body, locked page, x closes`, async ({ page, isMobile }) => {
    await open(page, workout('triathlon').view, { storage: { 'wx.lang': 'de' } });
    await page.evaluate(() => scrollTo(0, 120));
    const scrollBefore = await page.evaluate(() => Math.round(scrollY));
    await (isMobile ? page.locator(trigger).tap() : page.locator(trigger).click());
    const dlg = page.locator('dialog[open]');
    await expect(dlg).toBeVisible();
    const head = dlg.locator('.dlg-head');
    const body = dlg.locator('.dlg-body');
    const top = (l) => l.evaluate((e) => Math.round(e.getBoundingClientRect().top));
    const inView = await dlg.evaluate((e) => { const r = e.getBoundingClientRect(); return r.top >= 0 && r.bottom <= innerHeight + 1; });
    expect(inView).toBe(true);
    const headTop = await top(head);
    expect(await body.evaluate((e) => e.scrollTop)).toBe(0);
    // The body scrolls (there is no touch swipe API: set scrollTop), the header does not move.
    const scrollable = await body.evaluate((e) => e.scrollHeight - e.clientHeight);
    if (scrollable > 0) {
      await body.evaluate((e) => { e.scrollTop = 300; });
      expect(await body.evaluate((e) => e.scrollTop)).toBeGreaterThan(0);
      expect(await top(head)).toBe(headTop);
    }
    // The page behind does not move (scroll lock, it may be fixed at -scrollY), also not by a wheel over the backdrop;
    // after the close it is where it was.
    const yOpen = await page.evaluate(() => document.body.getBoundingClientRect().top);
    if (!isMobile) await page.mouse.wheel(0, 600).catch(() => {});
    expect(Math.abs((await page.evaluate(() => document.body.getBoundingClientRect().top)) - yOpen)).toBeLessThanOrEqual(1);
    await (isMobile ? dlg.locator('.dlg-x').tap() : dlg.locator('.dlg-x').click());
    await expect(page.locator('dialog[open]')).toHaveCount(0);
    expect(Math.abs((await page.evaluate(() => scrollY)) - scrollBefore)).toBeLessThanOrEqual(1);
  });
}
