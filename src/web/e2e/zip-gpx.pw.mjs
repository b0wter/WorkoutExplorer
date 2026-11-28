// F-57 GPX route download, F-35 zip upload, F-63/F-34 iOS Shortcut hint and About link (run.sh sets a throwaway WORKOUT_EXPLORER_IOS_SHORTCUT_URL).
import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { open, workout } from './helpers.mjs';
import { fitPath } from './fixtures.mjs';

const crcTable = Array.from({ length: 256 }, (_, n) => { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; return c >>> 0; });
const crc32 = (b) => { let c = 0xffffffff; for (const x of b) c = crcTable[(c ^ x) & 255] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; };

/** One-entry zip (deflate) at its simplest: local header, data, central directory, end record. */
function zipOne(name, data) {
  const comp = zlib.deflateRawSync(data), n = Buffer.from(name), crc = crc32(data);
  const lh = Buffer.alloc(30); lh.writeUInt32LE(0x04034b50, 0); lh.writeUInt16LE(20, 4); lh.writeUInt16LE(8, 8);
  lh.writeUInt32LE(crc, 14); lh.writeUInt32LE(comp.length, 18); lh.writeUInt32LE(data.length, 22); lh.writeUInt16LE(n.length, 26);
  const cd = Buffer.alloc(46); cd.writeUInt32LE(0x02014b50, 0); cd.writeUInt16LE(20, 4); cd.writeUInt16LE(20, 6); cd.writeUInt16LE(8, 10);
  cd.writeUInt32LE(crc, 16); cd.writeUInt32LE(comp.length, 20); cd.writeUInt32LE(data.length, 24); cd.writeUInt16LE(n.length, 28);
  const off = 30 + n.length + comp.length, size = 46 + n.length;
  const end = Buffer.alloc(22); end.writeUInt32LE(0x06054b50, 0); end.writeUInt16LE(1, 8); end.writeUInt16LE(1, 10); end.writeUInt32LE(size, 12); end.writeUInt32LE(off, 16);
  return Buffer.concat([lh, n, comp, cd, n, end]);
}

test('F-57: Download route (GPX) gives a GPX track with points, no time, no console errors', async ({ page }) => {
  const errors = await open(page, workout('ride-30min').view);
  const button = page.locator('button[data-gpx]');
  await expect(button).toBeVisible();
  const [dl] = await Promise.all([page.waitForEvent('download'), button.click()]);
  expect(dl.suggestedFilename()).toMatch(/^route(-\d{4}-\d{2}-\d{2})?\.gpx$/);
  const file = path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'gpx-')), 'r.gpx');
  await dl.saveAs(file);
  const xml = fs.readFileSync(file, 'utf8');
  expect(xml).toContain('<gpx');
  expect((xml.match(/<trk>/g) ?? []).length).toBe(1);
  expect((xml.match(/<trkseg>/g) ?? []).length).toBe(1);
  expect((xml.match(/<trkpt /g) ?? []).length).toBeGreaterThan(100);
  expect(xml).toMatch(/<trkpt lat="-?\d+\.\d+" lon="-?\d+\.\d+"/);
  expect(xml).not.toContain('<time>');
  expect(errors).toEqual([]);
});

test('F-35: a zip holding one FIT uploads through the UI and opens the draft', async ({ page }) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'zip-')); // never committed
  const zip = path.join(dir, 'ride-30min.zip');
  fs.writeFileSync(zip, zipOne('ride-30min.fit', fs.readFileSync(fitPath('ride-30min'))));
  const errors = await open(page, '/', { wait: false });
  await expect(page.locator('#page')).toHaveAttribute('data-state', 'ready');
  await page.locator('#file').setInputFiles(zip);
  await page.waitForURL(/\/edit\/[0-9A-Za-z]{22}#/);
  await expect(page.locator('.chart-stack canvas').first()).toBeVisible();
  expect(errors).toEqual([]);
});

const SHORTCUT = 'https://www.icloud.com/shortcuts/e2e-throwaway'; // run.sh sets WORKOUT_EXPLORER_IOS_SHORTCUT_URL; "unset: no hint" is the Go shell test

// The hint is server-configured, not user-agent dependent: it shows for every visitor.
for (const [label, ua] of [['default UA', undefined], ['iPhone UA', 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1']]) {
  test.describe(label, () => {
    test.use(ua ? { userAgent: ua } : {});
    test('F-63: the upload page hint links to the Shortcut and to the About section', async ({ page }) => {
      const errors = await open(page, '/', { wait: false });
      await expect(page.locator('#page')).toHaveAttribute('data-state', 'ready');
      const hint = page.locator('p.shortcut');
      await expect(hint).toBeVisible();
      await expect(hint.locator('a[data-p=link]')).toHaveAttribute('href', SHORTCUT);
      await expect(hint.locator('a[data-p=more]')).toHaveAttribute('href', '/about#shortcut');
      expect(errors).toEqual([]);
    });
  });
}

test('F-34: the About page has the Shortcut section with the link', async ({ page }) => {
  const errors = await open(page, '/about', { wait: false });
  await expect(page.locator('#shortcut')).toBeVisible();
  await expect(page.locator(`#shortcut ~ p a[href="${SHORTCUT}"]`).first()).toBeVisible();
  expect(errors).toEqual([]);
});
