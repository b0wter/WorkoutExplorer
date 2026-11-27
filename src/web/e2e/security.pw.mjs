// B-17 (F-22, F-23, F-36): response headers as deploy/Caddyfile sets them (security snippet, every response), Open Graph tags,
// no request leaving the site except the tile/style hosts. Headers are the same in every browser: chromium only.
import { test, expect } from '@playwright/test';
import { workout, open } from './helpers.mjs';

const paths = { published: workout('ride-30min').view, upload: '/', about: '/about', 'my-workouts': '/my-workouts',
  generic: '/w/' + 'A'.repeat(22), edit: '/edit/' + 'A'.repeat(22), 'report-delete': '/report-delete' };

for (const [name, path] of Object.entries(paths)) {
  test(`F-22: ${name} page carries CSP, Referrer-Policy and X-Robots-Tag`, async ({ request }, info) => {
    test.skip(info.project.name !== 'chromium', 'headers do not depend on the browser');
    const h = (await request.get(path)).headers();
    const csp = h['content-security-policy'];
    expect(csp).toContain("default-src 'self'");
    expect(csp).toContain("object-src 'none'");
    expect(csp).toContain("frame-ancestors 'none'");
    expect(csp).toMatch(/script-src 'self'/);
    expect(csp).not.toContain('unsafe-inline');
    expect(csp).not.toMatch(/script-src[^;]*\*/);
    expect(h['referrer-policy']).toBe('no-referrer');
    expect(h['x-robots-tag']).toBe('noindex, nofollow, noarchive');
    expect(h['x-content-type-options']).toBe('nosniff');
  });
}

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
