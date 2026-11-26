// B-06 (F-16 start/end hiding, F-55 hiding radii, F-47 trim; SH-4, SH-8, UP-9): set on the edit page, then read the PUBLISHED data file.
// Privacy is checked on the bytes a viewer gets (data.bin of the published page), never on pixels.
import { test, expect } from '@playwright/test';
import { upload, open } from './helpers.mjs';

const form = (page) => page.locator('form.edit-panel');
const auth = (w) => ({ Authorization: 'Bearer ' + w.tok });
const save = async (page) => {
  await form(page).locator('button.edit-save[type=submit]').click();
  await expect(form(page).locator('.edit-status')).toHaveText('Saved');
};
const setRadius = async (page, name, m) => {
  await form(page).locator(`input[name=${name}]`).evaluate((el, v) => { el.value = String(v); el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); }, m);
};
const publish = async (request, w) => expect((await request.post(`/api/workouts/${w.id}/publish`, { headers: auth(w) })).ok()).toBe(true);

const INT32_SENTINEL = 2147483647;
/** Reads the published page's data file (URL from the HTML preload) and decodes time, positions and the summary (format in lib/datafile). */
async function published(request, view) {
  const html = await (await request.get(view)).text();
  const href = /id="wx-data"[^>]*href="([^"]+)"/.exec(html)?.[1];
  expect(href).toBeTruthy();
  const buf = await (await request.get(href)).body();
  const dv = new DataView(buf.buffer, buf.byteOffset, buf.byteLength);
  expect(buf.subarray(0, 4).toString('latin1')).toBe('WEXD');
  const n = dv.getUint32(6, true);
  const meta = JSON.parse(buf.subarray(14, 14 + dv.getUint32(10, true)).toString('utf8'));
  const delta = (name, scale) => {
    const d = meta.streams.find((s) => s.name === name);
    if (!d) return null;
    const out = new Array(n);
    let acc = 0;
    for (let i = 0; i < n; i++) {
      const v = dv.getInt32(d.byte_offset + 4 * i, true);
      if (v === INT32_SENTINEL) out[i] = NaN; else { acc += v; out[i] = acc * scale; }
    }
    return out;
  };
  return { html, n, meta, time: delta('time_offset', 1), lat: delta('latitude', 1e-6), lon: delta('longitude', 1e-6) };
}

const hav = (a, b, c, d) => {
  const r = Math.PI / 180;
  const h = Math.sin(((c - a) * r) / 2) ** 2 + Math.cos(a * r) * Math.cos(c * r) * Math.sin(((d - b) * r) / 2) ** 2;
  return 12742000 * Math.asin(Math.sqrt(h));
};
const positions = (d) => d.lat.map((la, i) => (Number.isNaN(la) ? null : [la, d.lon[i]]));
const key = (p) => p[0].toFixed(6) + ',' + p[1].toFixed(6);
/** Errors of a page that also loads the map: WebKit logs the suite's own aborted tile fetches as page errors. */
const appErrors = (errors) => errors.filter((e) => !/tiles\.openfreemap\.org/.test(e));
const summaryText = (page) => page.locator('.summary').innerText();

test('F-16, F-55, SH-8: radii set on the edit page and saved; no position inside either radius reaches the published page', async ({ page, request, context }) => {
  const base = await upload(request, 'run-30min', { publish: true });
  const full = await published(request, base.view);
  const fp = positions(full);
  expect(fp.every(Boolean)).toBe(true);

  const w = await upload(request, 'run-30min');
  const errors = await open(page, w.edit, { wait: false });
  await expect(form(page)).toBeVisible();
  await setRadius(page, 'hide_start_m', 500);
  await setRadius(page, 'hide_end_m', 300);
  await save(page);
  await page.waitForLoadState('networkidle');
  await page.reload();
  await expect(form(page).locator('input[name=hide_start_m]')).toHaveValue('500'); // saved
  await expect(form(page).locator('input[name=hide_end_m]')).toHaveValue('300');
  await publish(request, w);

  const hid = await published(request, w.view);
  const hp = positions(hid);
  expect(hid.n).toBe(full.n);
  const [s0, e0] = [fp[0], fp[fp.length - 1]];
  const inside = (p) => hav(p[0], p[1], ...s0) <= 500 || hav(p[0], p[1], ...e0) <= 300;
  const hidden = fp.filter(inside);
  expect(hidden.length).toBeGreaterThan(20);
  // Every original sample inside a radius lost its position, and no coordinate of it appears anywhere in the published data or page.
  fp.forEach((p, i) => { if (inside(p)) expect(hp[i], 'sample ' + i).toBeNull(); });
  const shown = hp.filter(Boolean);
  expect(shown.length).toBeGreaterThan(100);
  const hiddenKeys = new Set(hidden.map(key));
  expect(shown.filter((p) => hiddenKeys.has(key(p)))).toEqual([]);
  expect(shown.filter(inside)).toEqual([]);
  for (const p of hidden.slice(0, 50)) expect(hid.html).not.toContain(p[0].toFixed(5));
  // Positions only: time and totals are untouched (Hide changes latitude/longitude only).
  expect(hid.time).toEqual(full.time);
  expect(hid.meta.summary).toEqual(full.meta.summary);

  await page.waitForLoadState('networkidle');
  expect(errors).toEqual([]); // the edit page
  await page.close();
  const vp = await context.newPage();
  const viewErrors = await open(vp, w.view);
  await expect(vp.locator('.summary')).toBeVisible();
  await vp.waitForLoadState('networkidle');
  expect(appErrors(viewErrors)).toEqual([]);
});

test('F-16, F-47, F-55, SH-8: a route that is fully hidden has no position in the data and the page shows no route', async ({ page, request, context }) => {
  const w = await upload(request, 'run-30min');
  const ref = await upload(request, 'run-30min', { publish: true });
  const t0 = new Date((await published(request, ref.view)).meta.start_time).getTime();
  // Trim to the first two minutes (about 400 m of route), then hide 500 m at the start on the edit page: nothing is left.
  const end = new Date(t0 + 120000).toISOString().replace('.000Z', 'Z');
  expect((await request.patch(`/api/workouts/${w.id}`, { headers: auth(w), data: { trim_end_at: end } })).ok()).toBe(true);
  const errors = await open(page, w.edit, { wait: false });
  await expect(form(page)).toBeVisible();
  await setRadius(page, 'hide_start_m', 500);
  await save(page);
  await publish(request, w);

  const d = await published(request, w.view);
  expect(d.lat).toBeNull();
  expect(d.lon).toBeNull();
  expect(d.meta.streams.map((s) => s.name)).not.toContain('latitude');
  expect(d.meta.streams.map((s) => s.name)).not.toContain('longitude');
  expect(d.n).toBeGreaterThan(60);

  await page.waitForLoadState('networkidle');
  expect(errors).toEqual([]); // the edit page
  await page.close();
  const vp = await context.newPage();
  const viewErrors = await open(vp, w.view);
  await expect(vp.locator('.chart-stack canvas').first()).toBeVisible();
  await expect(vp.locator('.map-slot canvas, .maplibregl-canvas')).toHaveCount(0);
  await expect(vp.locator('body')).toContainText("The entire route has been hidden by the uploader.");
  await vp.waitForLoadState('networkidle');
  expect(appErrors(viewErrors)).toEqual([]);
});

test('F-47, SH-8: trim saved on the draft: the published data and totals cover only the kept part', async ({ page, request, context, browserName }) => {
  const base = await upload(request, 'ride-30min', { publish: true });
  const full = await published(request, base.view);
  const baseErrors = await open(page, base.view);
  await page.waitForLoadState('networkidle');
  expect(appErrors(baseErrors)).toEqual([]);
  const fullSummary = await summaryText(page);

  const w = await upload(request, 'ride-30min');
  const t0 = new Date(full.meta.start_time).getTime();
  const iso = (s) => new Date(t0 + s * 1000).toISOString().replace('.000Z', 'Z');
  const r = await request.patch(`/api/workouts/${w.id}`, { headers: auth(w), data: { trim_start_at: iso(300), trim_end_at: iso(1200) } });
  expect(r.ok()).toBe(true);
  const errors = await open(page, w.edit, { wait: false });
  await expect(form(page)).toBeVisible(); // the edit page opens with the stored trim
  if (browserName !== 'firefox') await expect(page.locator('.maplibregl-canvas').first()).toBeVisible(); // map worker started: leaving earlier aborts it (WebKit logs a page error)
  await page.waitForLoadState('networkidle');
  await publish(request, w);

  const t = await published(request, w.view);
  expect(t.n).toBeLessThan(full.n);
  expect(t.n).toBeGreaterThanOrEqual(900);
  expect(t.n).toBeLessThanOrEqual(902);
  expect(new Date(t.meta.start_time).getTime() - t0).toBe(300000); // the time range starts at the trim
  expect(t.time[t.n - 1] - t.time[0]).toBeGreaterThanOrEqual(899);
  expect(t.time[t.n - 1] - t.time[0]).toBeLessThanOrEqual(901);
  expect(t.meta.summary.distance).toBeLessThan(full.meta.summary.distance);
  expect(t.meta.summary.elapsed_time).toBeLessThan(full.meta.summary.elapsed_time);
  // No position of the trimmed-off parts is in the published data.
  const fp = positions(full);
  const kept = new Set(fp.slice(299, 1202).map(key));
  expect(positions(t).filter(Boolean).filter((p) => !kept.has(key(p)))).toEqual([]);
  const cut = fp.slice(0, 295).concat(fp.slice(1206)).map(key);
  const tk = new Set(positions(t).filter(Boolean).map(key));
  expect(cut.filter((k) => tk.has(k))).toEqual([]);

  await page.waitForLoadState('networkidle');
  expect(errors).toEqual([]); // the edit page
  await page.close();
  const vp = await context.newPage();
  const viewErrors = await open(vp, w.view);
  expect(await summaryText(vp)).not.toBe(fullSummary);
  await vp.waitForLoadState('networkidle');
  expect(appErrors(viewErrors)).toEqual([]);
});
