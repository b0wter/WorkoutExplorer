// B-17 (F-23, F-36): Open Graph tags (response headers: TestCaddy, caddy_test.go),
// no request leaving the site except the tile/style hosts.
import { test, expect } from '@playwright/test';
import { workout, open } from './helpers.mjs';

test('F-23: published page has Open Graph tags in its head', async ({ request }, info) => {
  test.skip(info.project.name !== 'chromium', 'static HTML, no browser needed');
  const html = await (await request.get(workout('ride-30min').view)).text();
  const head = html.slice(0, html.indexOf('</head>'));
  for (const p of ['og:title', 'og:description', 'og:image']) expect(head).toMatch(new RegExp(`<meta property="${p}" content="[^"]+"`));
});

test('F-36: the viewing page asks no host but the tile and style hosts', async ({ page }) => {
  const errors = await open(page, workout('ride-30min').view);
  expect(errors).toEqual([]);
  expect(errors.blocked.filter((u) => !/^https:\/\/(tiles\.openfreemap\.org|server\.arcgisonline\.com)\//.test(u))).toEqual([]);
});
