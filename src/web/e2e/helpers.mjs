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
  // The suite never reaches an outside host: every non-local request is aborted and its URL recorded in `errors.blocked`.
  Object.defineProperty(errors, 'blocked', { value: [] }); // non-enumerable: tests compare `errors` with toEqual([])
  await page.context().route(/^https?:\/\/(?!(localhost|127\.0\.0\.1)[:/])/, (r) => { errors.blocked.push(r.request().url()); return r.abort(); });
  // No WebGL in headless Firefox: the map cannot start there, which is not what the suite checks.
  page.on('pageerror', (e) => { if (!/WebGL/i.test(String(e))) errors.push(String(e)); });
  page.on('console', (m) => { if (/Content.Security.Policy|Refused to/i.test(m.text())) errors.push('csp: ' + m.text()); });
  await page.goto(url);
  if (wait) await charts(page);
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
  for (let k = 0; k <= px * 2; k++) {
    const x = bb.x + bb.width * frac + (k % 2 ? -1 : 1) * Math.ceil(k / 2);
    const y = bb.y + bb.height / 2;
    if (coarse) await page.touchscreen.tap(x, y); else await page.mouse.move(x, y);
    await page.waitForTimeout(80); // the cursor is drawn on the next frame
    text = (await page.locator('.chart-stack .readout .cell', { hasText: label }).first().textContent()).replace(/\s+/g, ' ').trim();
    if (re.test(text)) return text;
  }
  return text;
}

/** Drags a range on the first chart (mouse; call only on non-touch projects). */
export async function drag(page, a, b) {
  await page.locator('.chart-area .u-over').first().scrollIntoViewIfNeeded();
  const bb = await page.locator('.chart-area .u-over').first().boundingBox();
  await page.mouse.move(bb.x + bb.width * a, bb.y + bb.height / 2);
  await page.mouse.down();
  await page.mouse.move(bb.x + bb.width * b, bb.y + bb.height / 2, { steps: 8 });
  await page.mouse.up();
}

export const texts = (loc) => loc.evaluateAll((els) => els.map((e) => e.textContent.replace(/\s+/g, ' ').trim()));
