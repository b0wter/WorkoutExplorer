// lib/compare (F-51 SC-4 to SC-6; AP-23 acceptance criteria 1, 2, 5, 6).
import { describe, expect, it } from 'vitest';
import { compare, halfRange, overlay, type CompareRow } from '.';

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
    const c = compare(rows([10, 10.3, 9.7, 10.31, 9.69, 10.8, 9.2]), false)!;
    expect(c.reference).toBe(10);
    expect(c.bars.map((b) => b!.cls)).toEqual(['pace', 'pace', 'pace', 'faster', 'slower', 'faster', 'slower']);
    // Up = smaller fraction; the reference is at 0.5.
    expect(c.bars[5]!.y0).toBeLessThan(0.5);
    expect(c.bars[5]!.y1).toBe(0.5);
    expect(c.bars[6]!.y0).toBe(0.5);
    expect(c.bars[6]!.y1).toBeGreaterThan(0.5);
    // Symmetric: the furthest bar end touches an edge.
    expect(c.bars[5]!.y0).toBeCloseTo(0, 12);
    expect(c.top).toBeCloseTo(10.8, 12);
    expect(c.bottom).toBeCloseTo(9.2, 12);
    expect(c.band[0]).toBeCloseTo(0.5 - 0.3 / 1.6, 12);
    expect(c.band[1]).toBeCloseTo(0.5 + 0.3 / 1.6, 12);
    expect(c.bars.some((b) => b!.clipped)).toBe(false);
  });

  it('pace: faster still points up; the band follows the pace edges and stays visible', () => {
    const c = compare(rows([3, 3.1, 2.9, 3.3, 2.7, 3.6, 2.5]), true)!;
    expect(c.reference).toBe(3);
    expect(c.bars[5]!.cls).toBe('faster');
    expect(c.bars[5]!.y0).toBeLessThan(0.5);
    expect(c.bars[6]!.cls).toBe('slower');
    expect(c.bars[6]!.y1).toBeGreaterThan(0.5);
    // Pace is 1/v: the slow bar (1/2.5 − 1/3) is further than the fast one (1/3 − 1/3.6), so it sets the range.
    expect(c.bars[6]!.y1).toBeCloseTo(1, 12);
    expect(c.bars.some((b) => b!.clipped)).toBe(false);
    expect(1 / c.bottom).toBeCloseTo(1 / 2.5, 12);
    // The band is asymmetric in pace: slower edge (1/0.97) further than the faster one (1/1.03).
    expect(c.band[1] - 0.5).toBeGreaterThan(0.5 - c.band[0]);
    const flat = compare(rows([3, 3]), true)!;
    expect(flat.band[0]).toBeGreaterThan(0);
    expect(flat.band[1]).toBeCloseTo(1, 12);
  });
});

describe('scale and clipping (SC-11; QA-51-r1-1)', () => {
  it('QA example, run: 6 × 4.5, 5 × 2.5, 1 × 0.3 m/s: the rest lap is clipped, the others are drawn to scale, every edge is real', () => {
    const c = compare(rows([...Array(6).fill(4.5), ...Array(5).fill(2.5), 0.3]), true)!;
    expect(c.reference).toBe(3.5);
    expect(c.bars.map((b) => b!.clipped)).toEqual([...Array(11).fill(false), true]);
    expect(c.bars[11]!.y1).toBe(1);
    // Others' largest deviation 1/2.5 − 1/3.5 ≈ 0.114 s/m, cap 0.343; the pace edge limit 1/(2·3.5) ≈ 0.143 wins.
    expect(c.top).toBeCloseTo(7, 9); // twice the reference speed
    expect(1 / c.bottom).toBeCloseTo(1 / 3.5 + 1 / 7, 9);
    // Work and recovery bars are long, not 1–2 px: 44 % and 80 % of the half-height.
    expect(0.5 - c.bars[0]!.y0).toBeCloseTo((1 / 3.5 - 1 / 4.5) * 3.5, 9);
    expect(c.bars[6]!.y1 - 0.5).toBeCloseTo((1 / 2.5 - 1 / 3.5) * 3.5, 9);
    expect(c.bars.map((b) => b!.cls)).toEqual([...Array(6).fill('faster'), ...Array(6).fill('slower')]);
  });

  it('two rest laps are clipped too', () => {
    const c = compare(rows([...Array(7).fill(4.5), ...Array(5).fill(2.5), 0.3, 0.4]), true)!;
    expect(c.bars.filter((b) => b!.clipped)).toHaveLength(2);
  });

  it('13 interval laps with the median inside the recovery group: work laps drawn, only the 2 rest laps clipped', () => {
    const c = compare(rows([...Array(6).fill(4.5), ...Array(5).fill(2.5), 0.3, 0.4]), true)!;
    expect(c.reference).toBe(2.5);
    expect(c.bars.map((b) => b!.clipped)).toEqual([...Array(11).fill(false), true, true]);
    // 75th percentile: the work laps' 1/2.5 − 1/4.5 ≈ 0.178 s/m, cap 0.533; the pace edge limit 1/(2·2.5) = 0.2 wins.
    expect(c.top).toBeCloseTo(5, 9);
    expect(0.5 - c.bars[0]!.y0).toBeCloseTo(((1 / 2.5 - 1 / 4.5) / 0.2) * 0.5, 9);
  });

  it('speed: the slow edge never goes below 0', () => {
    const c = compare(rows([10, 10.2, 9.8, 0.5, 40]), false)!;
    expect(c.bottom).toBeGreaterThanOrEqual(0);
    expect(c.bars[4]!.clipped).toBe(true);
    expect(c.bars[4]!.y0).toBe(0);
  });

  it('halfRange: furthest bar within 3× the 75th percentile (nearest rank); at least the band edge; at most the limit', () => {
    expect(halfRange([0.1, 0.2, 0.3], 0.05, 10)).toBe(0.3);
    expect(halfRange([0.1, 0.2, 0.3, 5], 0.05, 10)).toBeCloseTo(0.9, 12); // 3rd of 4
    expect(halfRange([0.1, 0.2, 5], 0.05, 10)).toBe(5); // known limit: more than a quarter are outliers
    expect(halfRange([0, 0, 0, 1], 0.05, 10)).toBeCloseTo(0.15, 12); // cap at least 3× the band edge
    expect(halfRange([0, 0], 0.05, 10)).toBe(0.05);
    expect(halfRange([0.1, 0.2, 0.3], 0.05, 0.25)).toBe(0.25);
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
