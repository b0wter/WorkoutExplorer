// Static checks of the generic page (F-15 design P-12; AC 6, AC 7; DS-4): one static file with
// no script, no Open Graph data, no workout data and no link to a workout; its theme is the
// plain pages' CSS-only light and dark theme.
import { describe, expect, it } from 'vitest';
import { readWebFile } from '../../test-support/source.mjs';

const html = readWebFile('generic.html');

describe('generic page', () => {
  it('has no script, no inline style, no handler, no Open Graph or other card data', () => {
    expect(html).not.toMatch(/<script/i);
    expect(html).not.toMatch(/<style[\s>]/i);
    expect(html).not.toMatch(/\sstyle\s*=/i);
    expect(html).not.toMatch(/\son[a-z]+\s*=/i);
    expect(html).not.toMatch(/og:|twitter:|property=/i);
    expect(html).not.toMatch(/\{\{/);
  });

  it('carries the metas, the title and the sentence', () => {
    expect(html).toContain('<meta name="color-scheme" content="light dark"');
    expect(html).toContain('<meta name="referrer" content="no-referrer"');
    expect(html).toContain('<meta name="robots" content="noindex, nofollow, noarchive"');
    expect(html).toContain('<title>Workout Explorer</title>');
    expect(html).toContain('This workout has expired or is not available.');
  });

  it('links only to About and its own stylesheet, never to a workout', () => {
    const hrefs = [...html.matchAll(/(?:href|src)="([^"]*)"/g)].map((m) => m[1]);
    expect(hrefs).toEqual(['/src/generic/generic.css', '/about']);
  });

  it('follows the system light or dark setting with CSS only (DS-4)', () => {
    expect(readWebFile('src/generic/generic.css')).toContain("@import '../theme/plain.css';");
    const theme = readWebFile('src/theme/plain.css');
    expect(theme).toMatch(/:root \{[^}]*color-scheme: light dark;/);
    expect(theme).toMatch(/@media \(prefers-color-scheme: dark\) \{\s*:root \{/);
  });
});
