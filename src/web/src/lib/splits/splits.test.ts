// Split presets and rules (F-46 LS-5, LS-6; AP-20; AC 34 to 36).
import { describe, expect, it } from 'vitest';
import type { Summary } from '../datafile';
import { movingOf, partialLast, poolDistance, presetBounds, splitChoice } from '.';

const labels = (sport: string, advance = 1e6, summary: Summary = {}): string[] => splitChoice({ sport, summary }, advance).presets.map((p) => p.label);
const chosen = (sport: string, advance = 1e6, summary: Summary = {}): string => {
  const c = splitChoice({ sport, summary }, advance);
  return c.presets[c.index].label;
};
const pool = (len: number, recorded: number, unit: 'm' | 'yd' = 'm'): Summary => (unit === 'm' ? { pool_length_m: len, pool_length_recorded_m: recorded } : { pool_length_yd: len, pool_length_recorded_m: recorded });

describe('lists and defaults (AC 34)', () => {
  it.each([
    ['running', ['400 m', '1 km', '5 km', '10 km', '20 km', 'Half marathon'], '1 km'],
    ['rowing', ['250 m', '500 m', '1 km'], '500 m'],
    ['cycling', ['1 km', '2 km', '2.5 km', '5 km', '10 km', '15 km', '20 km', '25 km', '30 km', '40 km', '50 km', '100 km'], '5 km'],
    ['e_biking', ['1 km', '2 km', '2.5 km', '5 km', '10 km', '15 km', '20 km', '25 km', '30 km', '40 km', '50 km', '100 km'], '5 km'],
    ['swimming', ['50 m', '100 m', '200 m', '500 m', '1000 m'], '100 m'],
    ['hiking', ['1 km', '5 km', '10 km', '20 km'], '1 km'],
    ['walking', ['1 km', '5 km', '10 km', '20 km'], '1 km'],
    ['multisport', ['1 km', '5 km', '10 km', '20 km'], '1 km'],
  ])('%s', (sport, list, def) => {
    expect(labels(sport)).toEqual(list);
    expect(chosen(sport)).toBe(def);
  });

  it('a pool swim takes the pool list; without one shared pool length the open-water list', () => {
    expect(labels('swimming', 1e6, pool(25, 25))).toEqual(['100 m', '200 m', '400 m', '500 m', '1000 m']);
    expect(chosen('swimming', 1e6, pool(25, 25))).toBe('100 m');
    expect(labels('swimming', 1e6, { lengths: 40 })).toEqual(['50 m', '100 m', '200 m', '500 m', '1000 m']);
    // Pool values on another sport (a multisport file) do not make a pool list.
    expect(labels('multisport', 1e6, pool(25, 25))).toEqual(['1 km', '5 km', '10 km', '20 km']);
  });
});

describe('presets longer than the workout (AC 35)', () => {
  it('a 10.4 km run offers up to 10 km; 5 km splits are 5, 5 and 0.4 km', () => {
    const c = splitChoice({ sport: 'running', summary: {} }, 10400);
    expect(c.presets.map((p) => p.label)).toEqual(['400 m', '1 km', '5 km', '10 km']);
    const d = Float64Array.from({ length: 105 }, (_, i) => i * 100);
    const b = presetBounds(d, c.presets[2], null);
    expect(b.slice(1).map((x, i) => d[x] - d[b[i]])).toEqual([5000, 5000, 400]);
  });

  it('a 0.8 km run still offers its default, as one partial split', () => {
    const c = splitChoice({ sport: 'running', summary: {} }, 800);
    expect(c.presets.map((p) => p.label)).toEqual(['400 m', '1 km']);
    expect(c.presets[c.index].label).toBe('1 km');
    expect(presetBounds(Float64Array.from([0, 400, 800]), c.presets[c.index], null)).toEqual([0, 2]);
  });
});

describe('pool rule (LS-6; AC 36)', () => {
  const first = (s: Summary) => splitChoice({ sport: 'swimming', summary: s }, 1e6).presets[0];

  it.each([
    ['25 m', pool(25, 25), 4, '100 m'],
    ['33 m from 32.5', pool(33, 32.5), 3, '100 m'],
    ['33 m from 33.0', pool(33, 33), 3, '100 m'],
    ['33 m from 33.33', pool(33, 33.33), 3, '100 m'],
    ['34 m from 33.5: not 33⅓', pool(34, 33.5), 3, '102 m'],
    ['30 m: 90 m', pool(30, 30), 3, '90 m'],
    ['50 m', pool(50, 50), 2, '100 m'],
    ['25 yd', pool(25, 22.86, 'yd'), 4, '100 yd'],
  ])('%s', (_name, s, n, label) => {
    const p = first(s);
    expect([p.lengths, p.label]).toEqual([n, label]);
    expect(p.length).toBeCloseTo(n * s.pool_length_recorded_m!, 9);
  });

  it('a 30 m pool at 1000 m is 33 lengths, 990 m: within 1 %, so labelled 1000 m', () => {
    expect(splitChoice({ sport: 'swimming', summary: pool(30, 30) }, 1e6).presets[4].label).toBe('1000 m');
  });

  it('boundaries every n recorded lengths within 1 % of a length; the distance cell is the target or whole lengths × P', () => {
    const s = pool(33, 33.33);
    const c = splitChoice({ sport: 'swimming', summary: s }, 1e6);
    const p = c.presets[0];
    // One sample per length, quantised to 0.1 m (99.99 → 100.0, 199.98 → 200.0), one a little short; 7 lengths.
    const d = Float64Array.from({ length: 8 }, (_, i) => Math.round(i * 333.3) / 10);
    d[6] -= 0.2;
    expect(presetBounds(d, p, c.pool)).toEqual([0, 3, 6, 7]);
    expect(poolDistance(d[3] - d[0], p, c.pool!)).toBe(100);
    expect(Math.round(poolDistance(d[7] - d[6], p, c.pool!))).toBe(33);
    expect(poolDistance(30 * 2, splitChoice({ sport: 'swimming', summary: pool(30, 30) }, 1e6).presets[0], { shown: 30, unit: 'm', recorded: 30 })).toBe(60);
  });

  it('offers pool presets by their stream length', () => {
    expect(splitChoice({ sport: 'swimming', summary: pool(25, 25) }, 450).presets.map((p) => p.label)).toEqual(['100 m', '200 m', '400 m']);
  });
});

describe('partial last split (F-51 SC-4)', () => {
  const km = { label: '1 km', length: 1000 };
  const d = (...xs: number[]) => Float64Array.from(xs);

  it('10.4 km: the 0.4 km split is partial; 10.0 km ends on a whole split', () => {
    const run = Float64Array.from({ length: 105 }, (_, i) => i * 100);
    expect(partialLast(run, presetBounds(run, km, null), km, null)).toBe(true);
    const even = run.subarray(0, 101);
    expect(partialLast(even, presetBounds(even, km, null), km, null)).toBe(false);
  });

  it('a whole last split after an overshooting boundary is not partial; one partial split is', () => {
    const x = d(0, 1003, 1500, 2000);
    expect(presetBounds(x, km, null)).toEqual([0, 1, 3]);
    expect(partialLast(x, [0, 1, 3], km, null)).toBe(false);
    expect(partialLast(d(0, 400, 800), [0, 2], km, null)).toBe(true);
  });

  it('pool: a split short by less than 1 % of a length is whole', () => {
    const s = pool(33, 33.33);
    const c = splitChoice({ sport: 'swimming', summary: s }, 1e6);
    const x = Float64Array.from({ length: 8 }, (_, i) => Math.round(i * 333.3) / 10);
    x[6] -= 0.2;
    const b = presetBounds(x, c.presets[0], c.pool);
    expect(partialLast(x, b, c.presets[0], c.pool)).toBe(true);
    expect(partialLast(x, b.slice(0, 3), c.presets[0], c.pool)).toBe(false);
  });
});

describe('windowed boundaries (F-64 LL-4)', () => {
  // 0 … 1200 m in 100 m steps, a standstill, then 300 m more: samples 0 … 16.
  const x = Float64Array.from([0, 100, 200, 300, 400, 500, 600, 700, 800, 900, 1000, 1100, 1200, 1200, 1300, 1400, 1500]);
  const km = { label: '1 km', length: 1000 };
  const m500 = { label: '500 m', length: 500 };

  it('the whole stream is the default window', () => {
    expect(presetBounds(x, km, null, 0, x.length - 1)).toEqual(presetBounds(x, km, null));
  });

  it('starts at a, measures from d̄[a], ends at b with a partial split; no split leaves the window', () => {
    const b = presetBounds(x, m500, null, 3, 12);
    expect(b).toEqual([3, 8, 12]);
    expect(partialLast(x, b, m500, null)).toBe(true);
    expect(presetBounds(x, m500, null, 12, 16)).toEqual([12, 16]); // shorter than the preset: one partial split
    expect(partialLast(x, [12, 16], m500, null)).toBe(true);
  });

  it('adjacent windows share their boundary sample; their distances sum to the whole', () => {
    const sum = (a: number, b: number): number => {
      const s = presetBounds(x, km, null, a, b);
      return x[s[s.length - 1]] - x[s[0]];
    };
    expect(sum(0, 8) + sum(8, 16)).toBe(x[16] - x[0]);
  });

  it('a tail without advance is in no split', () => {
    expect(presetBounds(x, m500, null, 7, 13)).toEqual([7, 12]);
  });
});

describe('time splits (F-69)', () => {
  const time = (sport: string, moving: number, summary: Summary = {}) => splitChoice({ sport, summary }, 1e6, moving).presets.filter((p) => p.time).map((p) => p.label);
  /** Moving time of one sample per second; `stops` are [from, to) second ranges in which it stays flat. */
  const clock = (n: number, stops: [number, number][] = []): Float64Array => {
    const m = new Float64Array(n);
    for (let i = 1; i < n; i++) m[i] = m[i - 1] + (stops.some(([f, t]) => i > f && i <= t) ? 0 : 1);
    return m;
  };
  const rows = (m: Float64Array, min: number, a = 0, b = m.length - 1) => {
    const p = splitChoice({ sport: 'running', summary: {} }, 1e6, 1e6).presets.find((x) => x.label === min + ' min')!;
    const bo = presetBounds(m, p, null, a, b);
    return { bo, secs: bo.slice(1).map((x, i) => m[x] - m[bo[i]]), partial: partialLast(m, bo, p, null) };
  };

  it('offer: presets up to the moving time, from 60 s, after the distance ones, default unchanged (AC 1)', () => {
    expect(time('running', 50 * 60)).toEqual(['1 min', '5 min', '10 min', '15 min', '30 min']);
    expect(time('running', 60)).toEqual(['1 min']);
    expect(time('running', 59)).toEqual([]);
    expect(time('running', 0)).toEqual([]);
    const c = splitChoice({ sport: 'running', summary: {} }, 1e6, 3600);
    expect(c.presets.map((p) => p.label).slice(-6)).toEqual(['1 min', '5 min', '10 min', '15 min', '30 min', '60 min']);
    expect(c.presets[c.index].label).toBe('1 km');
  });

  it('no time group for a pool swim; open water has one (AC 6)', () => {
    expect(time('swimming', 3600, pool(25, 25))).toEqual([]);
    expect(time('swimming', 3600)).toContain('10 min');
  });

  it('movingOf: absent stream, lap swim and empty window give no offer', () => {
    const m = clock(100);
    expect(movingOf(m, 10, 60, false)).toBe(50);
    expect(movingOf(undefined, 0, 99, false)).toBe(0);
    expect(movingOf(m, 0, 99, true)).toBe(0);
    expect(movingOf(m, 5, 5, false)).toBe(0);
  });

  it('50 min in 10 min splits: five rows of 10:00; 47 min: four and a partial 7:00 (AC 1, 2)', () => {
    expect(rows(clock(3001), 10).secs).toEqual([600, 600, 600, 600, 600]);
    const r = rows(clock(2821), 10);
    expect(r.secs).toEqual([600, 600, 600, 600, 420]);
    expect(r.partial).toBe(true);
    expect(rows(clock(3001), 10).partial).toBe(false);
  });

  it('a stop lies inside its row and adds no time; at a boundary it falls into the next row (AC 3)', () => {
    const r = rows(clock(2401 + 1200, [[840, 2040]]), 10);
    expect(r.secs).toEqual([600, 600, 600, 600]);
    expect(r.bo[2] - r.bo[1]).toBe(1800); // row 2: 10 min moving, 30 min elapsed
    const at = rows(clock(1201 + 600, [[600, 1200]]), 10); // the stop begins at sample 600, exactly a boundary
    expect(at.bo).toEqual([0, 600, 1800]);
    expect(at.secs).toEqual([600, 600]);
  });

  it('a gap counted as moving ends one row, late; across two multiples it still ends one row (AC 4)', () => {
    const m = Float64Array.from([0, 300, 420, 1500, 1560, 1800]); // jump 420 → 1500 crosses 600 and 1200
    const r = rows(m, 10);
    expect(r.bo).toEqual([0, 3, 5]);
    expect(r.secs).toEqual([1500, 300]);
  });

  it('smart recording: irregular spacing, boundaries at the first sample at or past the multiple', () => {
    const m = Float64Array.from([0, 7, 250, 603, 700, 1190, 1203, 1500]);
    expect(rows(m, 10).bo).toEqual([0, 3, 6, 7]);
  });

  it('multisport leg start and a trimmed start count from the window start (AC 5)', () => {
    const m = clock(3000);
    const r = rows(m, 5, 1000, 2900); // a leg from sample 1000: first split ends 300 s later
    expect(r.bo.slice(0, 3)).toEqual([1000, 1300, 1600]);
    expect(r.secs.reduce((x, y) => x + y)).toBe(1900); // rows sum to the window's moving time
  });

  it('a window shorter than 1 min is one partial row; a closing stop is inside the last row', () => {
    expect(rows(clock(30), 1).secs).toEqual([29]);
    const t = rows(clock(300, [[200, 299]]), 1); // a stop at the end stays inside the last row
    expect(t.secs).toEqual([60, 60, 60, 20]);
  });
});
