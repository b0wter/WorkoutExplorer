// F-68 extra sensor streams: running dynamics and Stryd power charts only when present; labels fit at 360 px in de/es/fr.
import { test, expect } from '@playwright/test';
import { workout, open, hover, texts, DESKTOP } from './helpers.mjs';

const labels = async (page) => (await texts(page.locator('.chart-stack .readout .cell .label'))).filter(Boolean);
const count = (page) => page.locator('.chart-area .u-over').count();

test('F-68: absent streams: no dynamics or Stryd charts on a plain run', async ({ page }) => {
  const errors = await open(page, workout('run-30min').view);
  expect((await labels(page)).join('|')).not.toMatch(/Vert\.|Contact|Stride|Power/);
  expect(errors).toEqual([]);
});

for (const [fixture, re] of [['run-dynamics', /Vert\. osc\.|Contact|Stride/], ['stryd-power', /Power/]]) {
  test(`F-68: ${fixture} adds its charts and readout cells`, async ({ page }) => {
    const e1 = await open(page, workout('run-30min').view);
    const base = await count(page);
    await page.goto('about:blank');
    const e2 = await open(page, workout(fixture).view);
    expect(await count(page)).toBeGreaterThan(base);
    expect((await labels(page)).join('|')).toMatch(re);
    expect([...e1, ...e2]).toEqual([]);
  });
}

// Runs in the page: findings (empty = fine).
function probe() {
  const R = (e) => e.getBoundingClientRect();
  const f = [];
  const de = document.documentElement;
  if (de.scrollWidth > de.clientWidth + 1) f.push(`document overflow: ${de.scrollWidth} > ${de.clientWidth}`);
  for (const ro of document.querySelectorAll('.chart-stack .readout')) {
    const cells = [...ro.querySelectorAll('.cell')].filter((c) => R(c).width > 0);
    for (const c of cells) {
      if (R(c).right > R(ro).right + 1) f.push(`cell past readout: ${c.textContent.trim()}`);
      for (const k of c.querySelectorAll('.label,.val')) if (k.scrollWidth > k.clientWidth + 1 || R(k).right > R(c).right + 1) f.push(`clipped: ${k.textContent.trim()}`);
    }
    for (let i = 0; i < cells.length; i++) for (let j = i + 1; j < cells.length; j++) {
      const a = R(cells[i]), b = R(cells[j]);
      if (Math.min(a.right, b.right) - Math.max(a.left, b.left) > 2 && Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top) > 2) f.push(`cells overlap: ${cells[i].textContent.trim()} / ${cells[j].textContent.trim()}`);
    }
  }
  return f;
}

for (const fixture of ['run-dynamics', 'stryd-power']) {
  for (const lang of ['de', 'es', 'fr']) {
    test(`F-68: ${fixture} labels fit at 360 px, ${lang}`, async ({ page }, info) => {
      test.skip(!DESKTOP.includes(info.project.name), 'sets its own 360 px viewport');
      await page.setViewportSize({ width: 360, height: 740 });
      const errors = await open(page, workout(fixture).view, { storage: { 'wx.lang': lang, 'wx.theme': 'light' } });
      await hover(page, 0.5);
      await page.waitForTimeout(300);
      expect(await page.evaluate(probe)).toEqual([]);
      expect(errors).toEqual([]);
    });
  }
}
