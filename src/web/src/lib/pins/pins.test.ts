// F-56 PN-3, PN-6, PN-11, PN-12: the pins file check, order and the edit page's mirror rules.
import { describe, expect, it } from 'vitest';
import { REMOVED, dimReason, fromApi, hasVisibleRoute, indexAt, instantAt, pinName, readPins, snap, toApi } from '.';

const p = (o: Record<string, unknown>) => ({ type: 'danger', title: 't', note: '', from: 1, ...o });

describe('readPins (PN-6, PN-11)', () => {
  it('accepts the shape and sorts by from, point before section, then to, then file order', () => {
    const file = [p({ from: 5, title: 'b' }), p({ from: 2, to: 4 }), p({ from: 2 }), p({ from: 5, title: 'a' }), p({ from: 2, to: 3 })];
    expect(readPins(file, 10)?.map((x) => [x.from, x.to, x.title])).toEqual([
      [2, undefined, 't'],
      [2, 3, 't'],
      [2, 4, 't'],
      [5, undefined, 'b'],
      [5, undefined, 'a'],
    ]);
    expect(readPins([], 10)).toEqual([]);
  });
  it.each([
    ['not an array', { a: 1 }],
    ['51 pins', Array.from({ length: 51 }, () => p({}))],
    ['an unknown key', [p({ lat: 47 })]],
    ['a missing key', [{ type: 'danger', title: '', from: 1 }]],
    ['an unknown type', [p({ type: 'shop' })]],
    ['a title of 61 code points', [p({ title: '😀'.repeat(61) })]],
    ['a note of 501 code points', [p({ note: 'x'.repeat(501) })]],
    ['an index past the data file', [p({ from: 10 })]],
    ['a negative index', [p({ from: -1 })]],
    ['a fractional index', [p({ from: 1.5 })]],
    ['to not after from', [p({ from: 3, to: 3 })]],
    ['a null to', [p({ to: null })]],
    ['a non-string title', [p({ title: 5 })]],
    ['null', [null]],
  ])('refuses %s (no pins shown)', (_, file) => {
    expect(readPins(file, 10)).toBeNull();
  });
  it('keeps 60 code points and hostile text as data', () => {
    const t = '<img src=x onerror=alert(1)>'.padEnd(60, 'x');
    expect(readPins([p({ title: t, note: '</script>' })], 10)?.[0]).toMatchObject({ title: t, note: '</script>' });
  });
});

describe('names (OQ-277)', () => {
  it('an empty title shows the type name', () => {
    expect(pinName({ type: 'danger', title: '' })).toBe('Danger');
    expect(pinName({ type: 'water_food', title: 'Fountain' })).toBe('Fountain');
  });
});

describe('instants (PN-3)', () => {
  const time = new Float64Array([0, 1, 2, 5, 10]);
  const startS = Date.parse('2026-11-20T10:00:00Z') / 1000;
  it('nearest sample, ties to the earlier; out of range clamps', () => {
    expect(indexAt(time, startS, '2026-11-20T10:00:04Z')).toBe(3);
    expect(indexAt(time, startS, '2026-11-20T10:00:03Z')).toBe(2); // 2 and 5 tie at 1 s
    expect(indexAt(time, startS, '2026-11-20T09:00:00Z')).toBe(0);
    expect(indexAt(time, startS, '2026-11-20T11:00:00Z')).toBe(4);
  });
  it('whole seconds UTC both ways', () => {
    expect(instantAt(time, startS, 4)).toBe('2026-11-20T10:00:10Z');
    const api = toApi({ type: 'note', title: '  a\tb  ', note: 'x\ny\u0007', from: 1, to: 3 }, time, startS);
    expect(api).toEqual({ type: 'note', title: 'a b', note: 'x\ny ', start_at: '2026-11-20T10:00:01Z', end_at: '2026-11-20T10:00:05Z' });
    expect(fromApi(api, time, startS)).toMatchObject({ from: 1, to: 3 });
    expect(toApi({ type: 'note', title: '😀'.repeat(70), note: '', from: 0 }, time, startS)).toMatchObject({ title: '😀'.repeat(60), end_at: null });
  });
});

describe('edit mirror (PN-12, OQ-234, OQ-275)', () => {
  const N = NaN;
  const lat = new Float64Array([N, 47, 47, N, N, 47, 47, 47, 47, N]);
  it('snap: the nearest positioned sample, ties earlier; none: -1', () => {
    expect(snap(lat, 1)).toBe(1);
    expect(snap(lat, 0)).toBe(1);
    expect(snap(lat, 3)).toBe(2);
    expect(snap(lat, 4)).toBe(5);
    expect(snap(lat, 9)).toBe(8);
    expect(snap(new Float64Array([N, N]), 1)).toBe(-1);
  });
  const shade = { a: 1, b: 8, from: 2, to: 7 };
  it('dimReason: trim wins, then each radius, both ends of a section; public is empty', () => {
    expect(dimReason({ from: 5 }, shade, lat)).toBe('');
    expect(dimReason({ from: 2, to: 7 }, shade, lat)).toBe('');
    expect(dimReason({ from: 0 }, shade, lat)).toBe(REMOVED);
    expect(dimReason({ from: 2, to: 9 }, shade, lat)).toBe(REMOVED);
    expect(dimReason({ from: 1 }, shade, lat)).toBe('Hidden by the start radius');
    expect(dimReason({ from: 8 }, shade, lat)).toBe('Hidden by the end radius');
    expect(dimReason({ from: 1, to: 8 }, shade, lat)).toBe('Hidden by the start and end radius');
    expect(dimReason({ from: 1, to: 5 }, shade, lat)).toBe('Hidden by the start radius'); // one end is enough, never clipped
    expect(dimReason({ from: 5 }, { ...shade, from: -1 }, lat)).toBe('Hidden by the start radius');
    expect(dimReason({ from: 3 }, shade, lat)).toBe('Hidden'); // no position
    expect(dimReason({ from: 5 }, shade, undefined)).toBe('Hidden');
  });
  it('hasVisibleRoute: a kept, unhidden, positioned sample exists', () => {
    expect(hasVisibleRoute(shade, lat)).toBe(true);
    expect(hasVisibleRoute({ ...shade, from: 3, to: 4 }, lat)).toBe(false);
    expect(hasVisibleRoute({ ...shade, to: -1 }, lat)).toBe(false);
    expect(hasVisibleRoute({ a: 3, b: 4, from: 0, to: 9 }, lat)).toBe(false);
    expect(hasVisibleRoute(shade, undefined)).toBe(false);
  });
});
