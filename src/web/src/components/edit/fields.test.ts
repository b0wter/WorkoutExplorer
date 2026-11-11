// The edit panel's rules (F-19 design E-6 to E-8; F-12 O-4 to O-6; AC 3, AC 5).
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Settings } from '../../lib/owner';
import { cleanText } from '../../lib/text';
import { expiryText, formOf, incompleteNotice, lifetimeOptions, patchOf } from './fields';

const STORED: Settings = {
  title: 'Morning loop',
  description: 'Easy.\nSecond line.',
  hide_ends: false,
  allow_download: false,
  visibility: 'draft',
  lifetime_days: 30,
  created_at: '2026-10-01T10:00:00Z',
  expires_at: '2026-10-31T10:00:00Z',
  data_end_s: null,
  view_url: '/w/x',
  hide_ends_ranges: [],
  suggested_title: '',
};

afterEach(() => {
  vi.unstubAllEnvs();
});

describe('cleanText (UP-4: no control character is ever sent, nothing else is lost)', () => {
  it('turns every Unicode Cc into a space, the line feed too in a title', () => {
    expect(cleanText('a\tb\rc\0d\x7fe\x85f\x9fg\nh', false)).toBe('a b c d e f g h');
  });
  it('keeps the line feed in a description', () => {
    expect(cleanText('a\tb\nc\r\nd', true)).toBe('a b\nc \nd');
  });
  it('leaves other text, including non-Cc format characters and astral code points, alone', () => {
    expect(cleanText('Ünïcödé 🚴‍ ', false)).toBe('Ünïcödé 🚴‍ ');
  });
});

describe('patchOf (E-6: only what changed; E-7: never what the server refuses)', () => {
  it('an unchanged form is an empty patch', () => {
    expect(patchOf(STORED, formOf(STORED))).toEqual({});
  });
  it('sends only the changed fields', () => {
    expect(patchOf(STORED, { ...formOf(STORED), title: 'New', hide_ends: true, lifetime_days: 7 })).toEqual({ title: 'New', hide_ends: true, lifetime_days: 7 });
  });
  it('a pasted tab becomes a space; the rest of the text stays', () => {
    expect(patchOf(STORED, { ...formOf(STORED), description: 'one\ttwo\nthree' })).toEqual({ description: 'one two\nthree' });
  });
  it('never more than 100 / 1,000 code points (astral characters count once)', () => {
    const p = patchOf(STORED, { ...formOf(STORED), title: '🚴'.repeat(150), description: '🚴'.repeat(1200) });
    expect([...p.title!]).toHaveLength(100);
    expect([...p.description!]).toHaveLength(1000);
  });
  it('never sends visibility (F-20 C-11), draft or published', () => {
    expect(formOf(STORED)).not.toHaveProperty('visibility');
    const published: Settings = { ...STORED, visibility: 'unlisted' };
    expect(patchOf(published, { ...formOf(published), title: 'x' })).toEqual({ title: 'x' });
  });
});

describe('lifetime (AC 3, O-5)', () => {
  const created = '2026-10-01T10:00:00Z';
  const at = (iso: string): number => Date.parse(iso);

  it('right after upload: 1 to 30 days, never more', () => {
    expect(lifetimeOptions(created, at('2026-10-01T10:05:00Z'))).toEqual(Array.from({ length: 30 }, (_, i) => i + 1));
  });
  it('only lifetimes whose expiry is after now', () => {
    expect(lifetimeOptions(created, at('2026-10-11T10:00:00Z'))).toEqual(Array.from({ length: 20 }, (_, i) => i + 11));
    expect(lifetimeOptions(created, at('2026-10-11T09:59:59Z'))[0]).toBe(10);
  });
  it('the expiry is shown in the browser time zone, from local getters, no Intl', () => {
    vi.stubEnv('TZ', 'America/New_York');
    expect(expiryText(created, 30)).toBe('31 Oct 2026, 06:00');
    expect(expiryText(created, 1)).toBe('2 Oct 2026, 06:00');
    vi.stubEnv('TZ', 'Europe/Berlin');
    // Berlin leaves summer time on 25 Oct 2026: the offset is the one at the expiry.
    expect(expiryText(created, 30)).toBe('31 Oct 2026, 11:00');
    expect(expiryText(created, 1)).toBe('2 Oct 2026, 12:00');
  });
});

describe('incomplete-file notice (UP-3, AC 5, E-8)', () => {
  it('none for a complete file', () => {
    expect(incompleteNotice(null)).toBeNull();
  });
  it('elapsed time as hh:mm:ss, hours not capped at 24', () => {
    expect(incompleteNotice(3723)).toBe('The file was incomplete; data after 01:02:03 is missing.');
    expect(incompleteNotice(0)).toBe('The file was incomplete; data after 00:00:00 is missing.');
    expect(incompleteNotice(100 * 3600 + 59)).toBe('The file was incomplete; data after 100:00:59 is missing.');
  });
});
