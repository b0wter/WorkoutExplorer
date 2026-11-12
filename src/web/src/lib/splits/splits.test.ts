// Split presets and rules (F-46 LS-5, LS-6; AP-20; AC 34 to 36).
import { describe, expect, it } from 'vitest';
import type { Summary } from '../datafile';
import { poolDistance, presetBounds, splitChoice } from '.';

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
