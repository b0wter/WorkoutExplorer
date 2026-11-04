// Static checks of the About page template (F-29 AB-1, AB-2, AB-7; AC 1 to 3, AC 7, AC 8): CSS
// only, the generic page's metas, same-origin resources, the markers of the server contract
// (src/server/internal/about/about.go), and the facts the acceptance criteria name word for word.
import { describe, expect, it } from 'vitest';
import { readWebFile } from '../../test-support/source.mjs';

const html = readWebFile('about.html');
const count = (s: string) => html.split(s).length - 1;

describe('About page', () => {
  it('has no script, no inline style, no handler', () => {
    expect(html).not.toMatch(/<script/i);
    expect(html).not.toMatch(/<style[\s>]/i);
    expect(html).not.toMatch(/\sstyle\s*=/i);
    expect(html).not.toMatch(/\son[a-z]+\s*=/i);
    expect(html).not.toMatch(/\{\{/);
  });

  it('carries the metas', () => {
    expect(html).toContain('<meta name="color-scheme" content="light dark"');
    expect(html).toContain('<meta name="referrer" content="no-referrer"');
    expect(html).toContain('<meta name="robots" content="noindex, nofollow, noarchive"');
  });

  it('loads only same-origin resources and links to / and itself (AB-7)', () => {
    const loaded = [...html.matchAll(/<(?:link|script|img)[^>]*(?:href|src)="([^"]*)"/g)].map((m) => m[1]);
    expect(loaded).toEqual(['/src/about/about.css']);
    const footer = html.slice(html.indexOf('<footer>'));
    expect([...footer.matchAll(/href="([^"]*)"/g)].map((m) => m[1])).toEqual(['/', '/about']);
  });

  it('has each server marker exactly once, the version at least once', () => {
    for (const m of ['<!--wx:attributions-->', '<!--wx:operator-->', '<!--wx:pushover-->', '<!--/wx:pushover-->']) {
      expect(count(m), m).toBe(1);
    }
    expect(count('<!--wx:version-->')).toBeGreaterThan(0);
    expect(html.indexOf('<!--wx:pushover-->')).toBeLessThan(html.indexOf('<!--/wx:pushover-->'));
  });

  it('states the facts of AC 2 and AC 3', () => {
    const text = html.replace(/\s+/g, ' ');
    for (const fact of [
      'Workouts are not archived: after an app update, a damaged file may show less than before or be removed.',
      'The FIT file as it was uploaded',
      'The workout data built from it',
      'The settings:',
      'Reports from viewers',
      'choice of units, the x axis of the charts, the split between map and content',
      'and the theme',
      'not in a cookie, are never sent to the server',
      'They see your IP address',
      '7 days by default',
      'up to 30 days from upload',
      'The app makes no backup copies.',
      'Device and sensor serial numbers and ANT+ IDs',
      'The user profile (name, gender, age, height, weight, heart-rate settings)',
      'Saved locations',
      "Segment leaderboard entries, the segment leaders' activity IDs and segment account keys",
      'Raw ANT radio messages',
    ]) {
      expect(text, fact).toContain(fact);
    }
  });

  it('follows the system light or dark setting with CSS only (DS-4)', () => {
    expect(readWebFile('src/about/about.css')).toContain("@import '../theme/plain.css';");
  });
});
