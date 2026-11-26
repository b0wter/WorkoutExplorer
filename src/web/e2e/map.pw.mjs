// B-02 (F-06 map, F-26 fullscreen, F-33 cursor marker, AP-3 one connected line): the map. Tiles are stubbed (nothing leaves the host).
import { test, expect } from '@playwright/test';
import { workout, open, hover, texts } from './helpers.mjs';

const METRIC = { storage: { 'wx.units': 'metric' } };
const marker = (page) => page.locator('.wx-cursor-marker');

// Opens a workout with every non-local request stubbed (aborted and recorded); `external` lists them.
async function openMap(page, name) {
  const external = [];
  await page.route(/^https?:\/\/(?!(localhost|127\.0\.0\.1)[:/])/, (r) => { external.push(r.request().url()); return r.abort(); });
  const errors = await open(page, workout(name).view, METRIC);
  await expect(page.locator('.wx-map canvas.maplibregl-canvas')).toBeVisible({ timeout: 20000 });
  return { errors, external };
}

// Screenshot of the map box, decoded and sampled in a blank page (the viewing page's CSP blocks data: images). A pixel is "ink"
// when it is saturated and not the background. `pts`: [[x, y]] in map-box CSS px -> ink flags; without `pts`: ink count on a 4 px grid.
async function ink(page, pts) {
  const png = await page.locator('.wx-map').screenshot({ scale: 'css' });
  const p = await page.context().newPage();
  const r = await p.evaluate(async ({ b64, pts }) => {
    const img = new Image();
    img.src = 'data:image/png;base64,' + b64;
    await img.decode();
    const c = document.createElement('canvas');
    c.width = img.width; c.height = img.height;
    const g = c.getContext('2d');
    g.drawImage(img, 0, 0);
    const d = g.getImageData(0, 0, c.width, c.height).data;
    const bg = [d[0], d[1], d[2]];
    const is = (x, y) => {
      const o = (Math.round(y) * c.width + Math.round(x)) * 4;
      return Math.max(d[o], d[o + 1], d[o + 2]) - Math.min(d[o], d[o + 1], d[o + 2]) > 60 && Math.abs(d[o] - bg[0]) + Math.abs(d[o + 1] - bg[1]) + Math.abs(d[o + 2] - bg[2]) > 90;
    };
    if (pts) return pts.map(([x, y]) => is(x, y));
    let n = 0;
    for (let y = 0; y < c.height; y += 4) for (let x = 0; x < c.width; x += 4) if (is(x, y)) n++;
    return n;
  }, { b64: png.toString('base64'), pts });
  await p.close();
  return r;
}

test.describe('map (B-02)', () => {
  test.beforeEach(({ browserName }) => { test.skip(browserName === 'firefox', 'headless Firefox has no WebGL: the map never starts'); });

  test('F-06: the route is drawn, nothing but the stubbed tile host is asked', async ({ page }) => {
    const { errors, external } = await openMap(page, 'ride-30min');
    await page.waitForTimeout(1000);
    expect(errors).toEqual([]); // before the screenshots: WebKit refuses the stylesheet Playwright injects for them (CSP), which is no app error
    await expect.poll(() => ink(page), { timeout: 30000 }).toBeGreaterThan(20);
    expect(external.filter((u) => !/openfreemap\.org/.test(u))).toEqual([]);
  });

  test.describe('mouse', () => {
    test.beforeEach(({ isMobile }) => { test.skip(isMobile, 'mouse scrubbing (touch: B-19)'); });

    test('F-33: the cursor marker follows the chart cursor', async ({ page }) => {
      const { errors } = await openMap(page, 'ride-30min');
      await expect(marker(page)).toBeHidden();
      await hover(page, 0.2);
      await expect(marker(page)).toBeVisible();
      const a = await marker(page).boundingBox();
      await hover(page, 0.8);
      await expect.poll(async () => { const b = await marker(page).boundingBox(); return Math.hypot(b.x - a.x, b.y - a.y); }).toBeGreaterThan(5);
      expect(errors).toEqual([]);
    });

    test('F-26: fullscreen button, readout in fullscreen, Esc leaves', async ({ page }) => {
      const { errors } = await openMap(page, 'ride-30min');
      await hover(page, 0.5); // a cursor to show in the strip
      const enter = page.locator('button.maplibregl-ctrl-fullscreen');
      await enter.click();
      await expect(page.locator('button.maplibregl-ctrl-shrink')).toBeVisible();
      await expect(page.locator('html')).toHaveClass(/wx-fs/);
      const box = await page.locator('.maplibregl-pseudo-fullscreen').boundingBox();
      const vp = page.viewportSize();
      expect(box.width).toBeGreaterThan(vp.width - 4);
      expect(box.height).toBeGreaterThan(vp.height - 4);
      const strip = page.locator('.wx-strip');
      await expect(strip).toBeVisible();
      await expect.poll(async () => (await texts(strip.locator('.cell .val'))).join('|')).not.toMatch(/^[–\-|\s]*$/);
      await page.keyboard.press('Escape');
      await expect(page.locator('html')).not.toHaveClass(/wx-fs/);
      await expect(page.locator('.maplibregl-pseudo-fullscreen')).toHaveCount(0);
      await expect(enter).toBeVisible();
      expect(errors).toEqual([]);
    });

    test('AP-3: gps-dropout: the route bridges the dropout with a straight segment', async ({ page }) => {
      const { errors } = await openMap(page, 'gps-dropout');
      await hover(page, 0.46); // inside the dropout (1500-1800 s of 3600 s): no position, no marker
      await page.waitForTimeout(150);
      await expect(marker(page)).toBeHidden();
      // last position before and first after the dropout: scan the chart in 1 px steps
      const bb = await page.locator('.chart-area .u-over').first().boundingBox();
      const at = async (x) => { await page.mouse.move(bb.x + x, bb.y + bb.height / 2); await page.waitForTimeout(60); return (await marker(page).isVisible()) ? await marker(page).boundingBox() : null; };
      let A = null, B = null;
      for (let x = Math.floor(bb.width * 0.40); x < bb.width * 0.5; x++) { const m = await at(x); if (m) A = m; else if (A) break; }
      for (let x = Math.ceil(bb.width * 0.52); x > bb.width * 0.4; x--) { const m = await at(x); if (m) B = m; else if (B) break; }
      expect(A && B, 'positions before and after the dropout').toBeTruthy();
      await page.mouse.move(0, 0);
      await expect(marker(page)).toBeHidden();
      const mb = await page.locator('.wx-map').boundingBox();
      const c = (m) => [m.x + m.width / 2 - mb.x, m.y + m.height / 2 - mb.y];
      const [ax, ay] = c(A), [bx, by] = c(B);
      const len = Math.hypot(bx - ax, by - ay);
      expect(len, 'the dropout spans a visible distance').toBeGreaterThan(20);
      // ink along the straight line (dashed: part painted) against two parallel lines 12 px away
      const nx = -(by - ay) / len * 12, ny = (bx - ax) / len * 12;
      const ts = Array.from({ length: 81 }, (_, k) => 0.1 + k * 0.01);
      const nb = [[0, 0], [1, 0], [-1, 0], [0, 1], [0, -1]];
      const line = (dx, dy) => ts.flatMap((t) => nb.map(([ox, oy]) => [ax + (bx - ax) * t + dx + ox, ay + (by - ay) * t + dy + oy]));
      expect(errors).toEqual([]); // before the screenshot (see the F-06 note)
      const flags = await ink(page, [...line(0, 0), ...line(nx, ny), ...line(-nx, -ny)]);
      const density = (k) => ts.filter((_, i) => flags.slice((k * ts.length + i) * 5, (k * ts.length + i) * 5 + 5).some(Boolean)).length / ts.length;
      expect(density(0)).toBeGreaterThan(0.35);
      expect(density(1) + density(2)).toBeLessThan(density(0));
    });
  });
});

test.describe('no location (AP-3)', () => {
  test('indoor-no-gps: hint shown, no map chunk requested, no errors', async ({ page }) => {
    const seen = [];
    page.on('request', (r) => seen.push(r.url()));
    const errors = await open(page, workout('indoor-no-gps').view, METRIC);
    await expect(page.locator('.hint', { hasText: 'No location data in this workout' })).toBeVisible();
    await page.waitForTimeout(1000);
    expect(seen.filter((u) => /map-view|map-style|map-glue|maplibre/i.test(u))).toEqual([]);
    expect(await page.locator('.wx-map, canvas.maplibregl-canvas').count()).toBe(0);
    expect(errors).toEqual([]);
  });
});
