// lib/compare (F-51 SC-4 to SC-6; AP-23 acceptance criteria 1, 2, 5, 6).
import { describe, expect, it } from 'vitest';
import { compare, overlay, type CompareRow } from '.';

const rows = (speeds: (number | undefined)[], extra: Partial<Record<'avg_heart_rate' | 'avg_power', number>>[] = []): CompareRow[] =>
  speeds.map((v, i) => ({ totals: { avg_speed: v, ...extra[i] } }));

describe('reference and best (SC-4, SC-5; AC 1)', () => {
  it('10.4 km on 1 km splits: 11 bars, the partial 0.4 km left out of median and best', () => {
    // Ten whole splits and a fast partial one; m/s.
    const v = [3.2, 3.3, 3.1, 3.4, 3.0, 3.25, 3.35, 3.15, 3.05, 3.5, 4.0];
    const r = rows(v);
    const last = { ...r[10], partial: true };
    const c = compare([...r.slice(0, 10), last], true)!;
    expect(c.bars).toHaveLength(11);
    expect(c.bars.every((b) => b !== null)).toBe(true);
    // Sorted whole splits: 3.0 3.05 3.1 3.15 3.2 | 3.25 3.3 3.35 3.4 3.5 → median (3.2 + 3.25) / 2.
    expect(c.reference).toBeCloseTo(3.225, 12);
    expect(c.best).toBe(3.5);
  });

  it('odd count: the middle value', () => {
    expect(compare(rows([2, 5, 3]), false)!.reference).toBe(3);
  });
});

describe('classes and geometry (SC-5; AC 2)', () => {
  it('more than 3 % faster is up and faster, more than 3 % slower down and slower, ±3 % inclusive on pace', () => {
    const c = compare(rows([10, 10.3, 9.7, 10.31, 9.69, 12, 8]), false)!;
    expect(c.reference).toBe(10);
    expect(c.bars.map((b) => b!.cls)).toEqual(['pace', 'pace', 'pace', 'faster', 'slower', 'faster', 'slower']);
    // Up = smaller fraction; the reference is at 0.5.
    expect(c.bars[5]!.y0).toBeLessThan(0.5);
    expect(c.bars[5]!.y1).toBe(0.5);
    expect(c.bars[6]!.y0).toBe(0.5);
    expect(c.bars[6]!.y1).toBeGreaterThan(0.5);
    // Symmetric: the furthest bar end touches an edge.
    expect(c.bars[5]!.y0).toBeCloseTo(0, 12);
    expect(c.top).toBeCloseTo(12, 12);
    expect(c.bottom).toBeCloseTo(8, 12);
    expect(c.band[0]).toBeCloseTo(0.5 - 0.3 / 4, 12);
    expect(c.band[1]).toBeCloseTo(0.5 + 0.3 / 4, 12);
  });

  it('pace: faster still points up; the band follows the pace edges and stays visible', () => {
    const c = compare(rows([3, 3, 3.6, 2.5]), true)!;
    expect(c.reference).toBe(3);
    expect(c.bars[2]!.cls).toBe('faster');
    expect(c.bars[2]!.y0).toBeLessThan(0.5);
    expect(c.bars[3]!.cls).toBe('slower');
    expect(c.bars[3]!.y1).toBeGreaterThan(0.5);
    // Pace is 1/v: the slow bar (1/2.5 − 1/3) is further than the fast one (1/3 − 1/3.6), so it sets the range.
    expect(c.bars[3]!.y1).toBeCloseTo(1, 12);
    expect(1 / c.bottom).toBeCloseTo(1 / 2.5, 12);
    // The band is asymmetric in pace: slower edge (1/0.97) further than the faster one (1/1.03).
    expect(c.band[1] - 0.5).toBeGreaterThan(0.5 - c.band[0]);
    const flat = compare(rows([3, 3]), true)!;
    expect(flat.band[0]).toBeGreaterThan(0);
    expect(flat.band[1]).toBeCloseTo(1, 12);
  });
});

describe('slots (SC-4; AC 6)', () => {
  it('a row without speed keeps an empty slot', () => {
    const c = compare(rows([3, undefined, 0, NaN, 4]), false)!;
    expect(c.bars.map((b) => b !== null)).toEqual([true, false, false, false, true]);
    expect(c.reference).toBe(3.5);
  });

  it('fewer than 2 bars or more than 200 slots: no chart', () => {
    expect(compare(rows([3, undefined]), false)).toBeNull();
    expect(compare(rows([]), false)).toBeNull();
    expect(compare(rows(Array(200).fill(3)), false)).not.toBeNull();
    expect(compare(rows(Array(201).fill(3)), false)).toBeNull();
  });
});

describe('overlay (SC-6; AC 5)', () => {
  it('offers only metrics a barred row has; one dot per bar with a value on its own scale', () => {
    const r = rows([3, 4, undefined, 5], [{ avg_heart_rate: 140 }, {}, { avg_power: 200 }, { avg_heart_rate: 160 }]);
    const c = compare(r, false)!;
    expect(c.overlays).toEqual(['avg_heart_rate']);
    const o = overlay(r, c, 'avg_heart_rate');
    expect([o.max, o.min]).toEqual([160, 140]);
    expect(o.ys.map((y) => (y === y ? y : null))).toEqual([1, null, null, 0]);
  });

  it('one value: centred', () => {
    const r = rows([3, 4], [{ avg_heart_rate: 150 }]);
    expect(overlay(r, compare(r, false)!, 'avg_heart_rate').ys[0]).toBe(0.5);
  });
});
