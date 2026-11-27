import { expect } from '@playwright/test';
import fs from 'node:fs';
import { fitPath } from './fixtures.mjs';

const ids = () => JSON.parse(process.env.WX_WORKOUTS ?? '{}');

/** Links of a shared fixture (uploaded once by setup.mjs). Read-only tests only. */
export function workout(name) {
  const w = ids()[name];
  if (!w) throw new Error('no fixture ' + name);
  return { view: `/w/${w.id}`, edit: `/edit/${w.id}#${w.tok}`, id: w.id, tok: w.tok };
}

/** Uploads a fresh copy (tests that change it); a draft until publish is true. */
export async function upload(request, name, { publish = false } = {}) {
  const r = await request.post('/api/workouts', { multipart: { file: { name: name + '.fit', mimeType: 'application/octet-stream', buffer: fs.readFileSync(fitPath(name)) } } });
  if (!r.ok()) throw new Error(`upload ${name}: ${r.status()}`);
  const j = await r.json();
  const id = j.view_url.split('/').pop();
  const tok = j.edit_url.split('#')[1];
  if (publish) await request.post(`/api/workouts/${id}/publish`, { headers: { Authorization: 'Bearer ' + tok } });
  return { view: j.view_url, edit: j.edit_url, id, tok };
}

export const DESKTOP = ['chromium', 'webkit', 'firefox'];

/** Call inside test.describe: skips its tests on the phone and tablet projects. */
export function desktopOnly(test, why) {
  test.beforeEach(({}, info) => { test.skip(!DESKTOP.includes(info.project.name), why); });
}

// Runs before any page script: layout-shift sum (Chromium only), width of .map-slot at its first painted frame.
function probes() {
  window.__cls = 0;
  window.__clsOk = false;
  try {
    new PerformanceObserver((l) => { for (const e of l.getEntries()) if (!e.hadRecentInput) window.__cls += e.value; }).observe({ type: 'layout-shift', buffered: true });
    window.__clsOk = true;
  } catch {}
  const tick = () => {
    const m = document.querySelector('.map-slot');
    const L = document.querySelector('.layout');
    if (m && L && [...document.querySelectorAll('link[rel=stylesheet]')].every((l) => l.sheet)) {
      window.__first = { w: m.getBoundingClientRect().width, lw: L.getBoundingClientRect().width };
    } else requestAnimationFrame(tick);
  };
  requestAnimationFrame(tick);
}

const blockedByPage = new WeakMap();
const routed = new WeakSet();
/** Blocked outside requests of one page; the context route is registered once and sorts requests by their page. */
function blockedOf(page) {
  const ctx = page.context();
  if (!routed.has(ctx)) {
    routed.add(ctx);
    ctx.route(/^https?:\/\/(?!(localhost|127\.0\.0\.1)[:/])/, (r) => {
      let p;
      try { p = r.request().frame().page(); } catch {} // service-worker requests have no frame
      blockedByPage.get(p)?.push(r.request().url());
      return r.abort();
    });
  }
  if (!blockedByPage.has(page)) blockedByPage.set(page, []);
  return blockedByPage.get(page);
}

/** Opens a page with browser storage preset (wx.lang, wx.units, wx.theme, wx.split.*) before any script runs.
 *  Returns the list of page errors and CSP violations seen so far (the array keeps filling);
 *  `.blocked` on it lists the outside requests that were aborted. */
export async function open(page, url, { storage = {}, wait = true, block = false } = {}) {
  await page.addInitScript(probes);
  await page.addInitScript(({ s, block }) => {
    if (block) { Object.defineProperty(window, 'localStorage', { get() { throw new DOMException('blocked', 'SecurityError'); } }); return; }
    try { for (const [k, v] of Object.entries(s)) localStorage.setItem(k, v); } catch {}
  }, { s: { 'wx.lang': 'en', ...storage }, block });
  const errors = [];
  // A navigation (goto, reload) while lazy chunks load aborts them; WebKit logs that as "Unable to preload CSS". Not an app error.
  let navigating = 0;
  for (const m of ['goto', 'reload']) {
    const orig = page[m].bind(page);
    page[m] = async (...a) => { navigating++; try { return await orig(...a); } finally { navigating--; } };
  }
  // The suite never reaches an outside host: every non-local request is aborted and its URL recorded in `errors.blocked` (per page).
  Object.defineProperty(errors, 'blocked', { value: blockedOf(page) }); // non-enumerable: tests compare `errors` with toEqual([])
  // No WebGL in headless Firefox: the map cannot start there. WebKit logs the suite's own aborted tile/font fetches as page errors.
  page.on('pageerror', (e) => {
    if (navigating && /Unable to preload CSS/.test(String(e))) return;
    if (!/WebGL|(tiles\.openfreemap\.org|server\.arcgisonline\.com)/i.test(String(e))) errors.push(String(e));
  });
  page.on('console', (m) => { if (/Content.Security.Policy|Refused to/i.test(m.text())) errors.push('csp: ' + m.text()); });
  await page.goto(url);
  if (wait) await charts(page);
  // LG-2: a page view whose language file misses the 500 ms hold stays English by design. A cold first load (WebKit, fresh
  // browser) can miss it, and the preset is then not tested. Reload (warm caches; the init script presets again) until it applied.
  const want = storage['wx.lang'];
  if (wait && !block && want && want != 'en') {
    for (let i = 0; i < 4; i++) {
      await expect.poll(() => page.locator('html').evaluate((h) => 'hold' in h.dataset)).toBe(false);
      if ((await page.locator('html').getAttribute('lang')) == want) break;
      await page.reload();
      await charts(page);
    }
  }
  return errors;
}

/** Charts drawn (no waitForFunction: it evals a string, which the page's CSP blocks). */
export async function charts(page) {
  await page.locator('.chart-stack canvas').first().waitFor({ state: 'visible' });
}

/** Puts the cursor on the first chart at a fraction of its width (a tap on touch projects, else the mouse). */
export async function hover(page, frac, yFrac = 0.5) {
  await page.locator('.chart-area .u-over').first().scrollIntoViewIfNeeded();
  const bb = await page.locator('.chart-area .u-over').first().boundingBox();
  const x = bb.x + bb.width * frac;
  const y = bb.y + bb.height * yFrac;
  if (await page.evaluate(() => matchMedia('(pointer: coarse)').matches)) await page.touchscreen.tap(x, y);
  else await page.mouse.move(x, y);
}

/** Hovers around a fraction of the first chart, one pixel at a time (up to +-px), until the readout cell with this label
 *  matches; a chart pixel is several seconds wide, so a one-second sample cannot be aimed at exactly. Returns the cell text. */
export async function scan(page, frac, label, re, px = 6) {
  await page.locator('.chart-area .u-over').first().scrollIntoViewIfNeeded();
  const bb = await page.locator('.chart-area .u-over').first().boundingBox();
  const coarse = await page.evaluate(() => matchMedia('(pointer: coarse)').matches);
  let text = '';
  const step = coarse ? 0.25 : 1; // a phone's chart pixel is seconds wide: aim in quarter pixels
  for (let k = 0; k <= px * 2 / step; k++) {
    const x = bb.x + bb.width * frac + (k % 2 ? -1 : 1) * Math.ceil(k / 2) * step;
    const y = bb.y + bb.height / 2;
    if (coarse) await touchPath(page, [(x - bb.x) / bb.width]); // fractional x: WebKit's tap rounds to whole pixels
    else await page.mouse.move(x, y);
    await page.waitForTimeout(80); // the cursor is drawn on the next frame
    text = (await page.locator('.chart-stack .readout .cell', { hasText: label }).first().textContent()).replace(/\s+/g, ' ').trim();
    if (re.test(text)) return text;
  }
  return text;
}

const isCoarse = (page) => page.evaluate(() => matchMedia('(pointer: coarse)').matches);

/** One touch on the first chart along fractions of its width: down at the first point, optional hold (ms), moves, up.
 *  Chromium: real touch events through CDP Input.dispatchTouchEvent. WebKit has no CDP and `page.touchscreen` only taps, so
 *  there the same sequence is dispatched as pointer events (pointerType touch, what the app's gestures read) with a real hold. */
export async function touchPath(page, fracs, { hold = 0, yFrac = 0.5, steps = 6 } = {}) {
  const over = page.locator('.chart-area .u-over').first();
  await over.scrollIntoViewIfNeeded();
  const bb = await over.boundingBox();
  const pts = [fracs[0]];
  for (let i = 1; i < fracs.length; i++) for (let k = 1; k <= steps; k++) pts.push(fracs[i - 1] + ((fracs[i] - fracs[i - 1]) * k) / steps);
  const at = (f) => ({ x: bb.x + bb.width * f, y: bb.y + bb.height * yFrac });
  if (page.context().browser().browserType().name() === 'chromium') {
    const cdp = await page.context().newCDPSession(page);
    const send = (type, f) => cdp.send('Input.dispatchTouchEvent', { type, touchPoints: f === null ? [] : [{ ...at(f), id: 1 }] });
    await send('touchStart', pts[0]);
    if (hold) await page.waitForTimeout(hold);
    for (const f of pts.slice(1)) { await send('touchMove', f); await page.waitForTimeout(16); }
    await send('touchEnd', null);
    await cdp.detach();
    return;
  }
  const fire = (type, f) => over.evaluate((el, { type, x, y }) => {
    el.dispatchEvent(new PointerEvent(type, { bubbles: true, cancelable: true, pointerId: 7, pointerType: 'touch', isPrimary: true, clientX: x, clientY: y, button: type == 'pointermove' ? -1 : 0 }));
  }, { type, ...at(f) });
  await fire('pointerdown', pts[0]);
  if (hold) await page.waitForTimeout(hold);
  for (const f of pts.slice(1)) { await fire('pointermove', f); await page.waitForTimeout(16); }
  await fire('pointerup', pts[pts.length - 1]);
}

/** Scrubs the first chart with a swipe from one fraction of its width to another (touch projects). */
export const scrub = (page, a, b) => touchPath(page, [a, b]);

/** Long press (past the app's 400 ms), then drag: selects a range on touch projects (F-17 R-5). */
export const touchRange = (page, a, b) => touchPath(page, [a, b], { hold: 550 });

/** Drags a range on the first chart: mouse press and drag, or a long press and drag on touch projects. */
export async function drag(page, a, b) {
  if (await isCoarse(page)) return touchRange(page, a, b);
  await page.locator('.chart-area .u-over').first().scrollIntoViewIfNeeded();
  const bb = await page.locator('.chart-area .u-over').first().boundingBox();
  await page.mouse.move(bb.x + bb.width * a, bb.y + bb.height / 2);
  await page.mouse.down();
  await page.mouse.move(bb.x + bb.width * b, bb.y + bb.height / 2, { steps: 8 });
  await page.mouse.up();
}

export const texts = (loc) => loc.evaluateAll((els) => els.map((e) => e.textContent.replace(/\s+/g, ' ').trim()));
