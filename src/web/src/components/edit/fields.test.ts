// The edit panel's rules (F-19 design E-6 to E-8; F-12 O-4 to O-6; AC 3, AC 5).
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Settings } from '../../lib/owner';
import { cleanText } from '../../lib/text';
import { expiryText, formOf, incompleteNotice, lifetimeOptions, patchOf, trimAt, trimClamp, trimIndex } from './fields';

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
  trim_start_at: null,
  trim_end_at: null,
  trimmed: null,
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

describe('trim window (F-47 TR-3, TR-4: the display mirror of the server rule)', () => {
  // 61 s → 3600 s is a pause without samples.
  const T = new Float64Array([0, 1, 2, 30, 60, 61, 3600, 3601, 3700]);
  const S = Date.parse('2026-10-01T10:00:00Z') / 1000;
  const at = (s: number): string => new Date((S + s) * 1000).toISOString().slice(0, 19) + 'Z';

  it('null is that end; instants round inward (start: first at or after, end: last at or before)', () => {
    expect([trimIndex(T, S, null, false), trimIndex(T, S, null, true)]).toEqual([0, 8]);
    expect([trimIndex(T, S, at(3), false), trimIndex(T, S, at(30), false)]).toEqual([3, 3]);
    expect([trimIndex(T, S, at(100), true), trimIndex(T, S, at(61), true)]).toEqual([5, 5]);
    expect([trimIndex(T, S, at(100), false), trimIndex(T, S, at(99_999), false), trimIndex(T, S, at(-5), true)]).toEqual([6, 8, 0]);
  });
  it('a handle is the sample instant in whole seconds, null at its own end; it reads back to the same sample', () => {
    expect([trimAt(T, S, 0, false), trimAt(T, S, 8, true)]).toEqual([null, null]);
    expect([trimAt(T, S, 3, false), trimAt(T, S, 0, true)]).toEqual(['2026-10-01T10:00:30Z', '2026-10-01T10:00:00Z']);
    for (let i = 0; i < T.length; i++) {
      expect(trimIndex(T, S, trimAt(T, S, i, false), false)).toBe(i);
      expect(trimIndex(T, S, trimAt(T, S, i, true), true)).toBe(i);
    }
  });
  it('at least 60 s remain: the moved handle is held back, 59 s never, 60 s and pauses fine', () => {
    expect(trimClamp(T, 0, 1, true)).toEqual([0, 4]);
    expect(trimClamp(T, 0, 4, true)).toEqual([0, 4]);
    expect(trimClamp(T, 1, 4, false)).toEqual([0, 4]);
    expect(trimClamp(T, 8, 8, false)).toEqual([7, 8]);
    expect(trimClamp(T, 5, 6, true)).toEqual([5, 6]);
    expect(trimClamp(T, 6, 6, true)).toEqual([6, 8]);
    // From every valid window, either handle dragged anywhere.
    for (let a = 0; a < T.length; a++) {
      for (let b = a; b < T.length; b++) {
        if (T[b] - T[a] < 60) continue;
        for (let p = 0; p < T.length; p++) {
          for (const [x, y] of [trimClamp(T, p, b, false), trimClamp(T, a, p, true)]) expect(T[y] - T[x]).toBeGreaterThanOrEqual(60);
        }
      }
    }
  });
  it('a recording under 60 s stays whole', () => {
    const short = new Float64Array([0, 10, 59]);
    expect(trimClamp(short, 2, 2, false)).toEqual([0, 2]);
    expect(trimClamp(short, 0, 0, true)).toEqual([0, 2]);
  });
  it('patchOf sends a changed trim, null included', () => {
    expect(patchOf(STORED, { ...formOf(STORED), trim_start_at: at(30) })).toEqual({ trim_start_at: at(30) });
    const trimmed: Settings = { ...STORED, trim_end_at: at(60) };
    expect(patchOf(trimmed, formOf(trimmed))).toEqual({});
    expect(patchOf(trimmed, { ...formOf(trimmed), trim_end_at: null })).toEqual({ trim_end_at: null });
  });
});
