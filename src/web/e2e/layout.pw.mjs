// Layout sweep: no horizontal overflow, header controls stay in their row and do not overlap, readout labels and values
// neither overlap nor clip, summary cards hold their content. Four languages; desktop projects also run four window widths.
import { test, expect } from '@playwright/test';
import { workout, open, hover, DESKTOP } from './helpers.mjs';

const LANGS = ['en', 'de', 'es', 'fr'];
const WIDTHS = [[360, 740], [768, 1024], [1100, 800], [1440, 900]];
const PAGES = { 'run view': (w) => w.view, 'run edit': (w) => w.edit, 'triathlon view': (w) => w.view };
const FIXTURE = { 'run view': 'run-30min', 'run edit': 'run-30min', 'triathlon view': 'triathlon' };

// Runs in the page. Returns a list of findings (empty = fine).
function probe() {
  const R = (e) => e.getBoundingClientRect();
  const vw = document.documentElement.clientWidth;
  const inter = (a, b) => Math.max(0, Math.min(a.right, b.right) - Math.max(a.left, b.left)) * Math.max(0, Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top));
  const vis = (e) => { const r = R(e); return r.width > 0 && r.height > 0 && getComputedStyle(e).visibility !== 'hidden'; };
  const name = (e) => e.tagName.toLowerCase() + (typeof e.className === 'string' && e.className ? '.' + e.className.trim().split(/\s+/).join('.') : '') + ` "${(e.textContent || '').trim().slice(0, 30)}"`;
  const f = [];
  const de = document.documentElement;
  if (de.scrollWidth > de.clientWidth + 1) f.push(`document overflow: ${de.scrollWidth} > ${de.clientWidth}`);
  const clipped = (e) => { for (let p = e.parentElement; p; p = p.parentElement) if (getComputedStyle(p).overflowX !== 'visible') return true; return false; };
  const out = [...document.body.querySelectorAll('*')].filter((e) => vis(e) && R(e).right > vw + 1 && !clipped(e) && getComputedStyle(e).position !== 'fixed');
  for (const e of out.filter((e) => !out.some((o) => o !== e && e.contains(o))).slice(0, 5)) f.push(`past viewport: ${name(e)}`);
  for (const ctl of document.querySelectorAll('.ctl')) {
    const c = R(ctl);
    if (ctl.scrollWidth > ctl.clientWidth + 1) f.push(`.ctl overflows: ${ctl.scrollWidth} > ${ctl.clientWidth}`);
    const ks = [...ctl.querySelectorAll('a,button,select')].filter(vis);
    for (const k of ks) if (R(k).right > c.right + 1 || R(k).right > vw + 1) f.push(`control past row: ${name(k)}`);
    for (let i = 0; i < ks.length; i++) for (let j = i + 1; j < ks.length; j++) if (!ks[i].contains(ks[j]) && !ks[j].contains(ks[i]) && inter(R(ks[i]), R(ks[j])) > 2) f.push(`controls overlap: ${name(ks[i])} / ${name(ks[j])}`);
  }
  for (const ro of document.querySelectorAll('.readout')) {
    if (!vis(ro)) continue;
    const cells = [...ro.querySelectorAll('.cell')].filter(vis);
    for (let i = 0; i < cells.length; i++) for (let j = i + 1; j < cells.length; j++) if (inter(R(cells[i]), R(cells[j])) > 2) f.push(`readout cells overlap: ${name(cells[i])} / ${name(cells[j])}`);
    for (const v of ro.querySelectorAll('.val')) if (vis(v) && v.scrollWidth > v.clientWidth + 1) f.push(`readout value clipped: ${name(v)}`);
    for (const c of cells) if (R(c).right > R(ro).right + 1) f.push(`readout cell past readout: ${name(c)}`);
  }
  for (const m of document.querySelectorAll('.summary .metric')) {
    if (!vis(m)) continue;
    if (m.scrollWidth > m.clientWidth + 1) f.push(`summary card overflows: ${name(m)}`);
    for (const k of m.querySelectorAll('.k,.v,.n,.u,.max')) if (vis(k) && R(k).right > R(m).right + 1) f.push(`summary card part past card: ${name(k)}`);
  }
  return f;
}

for (const [pageName, link] of Object.entries(PAGES)) {
  for (const lang of LANGS) {
    test(`layout: ${pageName}, ${lang}: nothing overflows, overlaps or clips`, async ({ page }, info) => {
      const sizes = DESKTOP.includes(info.project.name) ? WIDTHS : [null];
      for (const size of sizes) {
        if (size) await page.setViewportSize({ width: size[0], height: size[1] });
        await open(page, link(workout(FIXTURE[pageName])), { storage: { 'wx.lang': lang, 'wx.theme': 'light' } });
        if (!(await page.evaluate(() => matchMedia('(pointer: coarse)').matches))) await hover(page, 0.5); // values in the readout
        await page.waitForTimeout(300);
        expect(await page.evaluate(probe), `${lang} ${size ? size.join('x') : 'native'}`).toEqual([]);
        await page.goto('about:blank');
      }
    });
  }
}

test('layout: dark theme, run view, German', async ({ page }) => {
  await open(page, workout('run-30min').view, { storage: { 'wx.lang': 'de', 'wx.theme': 'dark' } });
  await page.waitForTimeout(300);
  expect(await page.evaluate(probe)).toEqual([]);
});
