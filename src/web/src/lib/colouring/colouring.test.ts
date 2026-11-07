// Route colouring values, scale, classes and legend (F-25 design C-3, C-4, C-6, Verification; AP-9, DS-1, DS-2).
// Node; synthetic data only.

import { describe, expect, it } from 'vitest';
import type { MetricName, Session, WorkoutData } from '../datafile';
import { buildRoute, recordedEdgeEnds } from '../route';
import { golden, positioned } from '../route/testing/fixtures';
import {
  CLASS_COUNT,
  GRADIENT_EDGES,
  GRADIENT_LABELS,
  NO_CLASS,
  centredMean,
  classify,
  colourScale,
  gradientValues,
  legend,
  metricLabel,
  metricValues,
  offeredMetrics,
  type ColourScale,
} from './index';

const N = Number.NaN;

/** n samples 1 s apart on a line, with the given streams and sessions. */
function workout(n: number, metrics: Partial<Record<MetricName, number[]>>, sport = 'cycling', sessions: Session[] = []): WorkoutData {
  const lat = Array.from({ length: n }, (_, i) => 47 + i * 1e-4);
  const base = positioned(lat, lat.map(() => 8));
  const m: Partial<Record<MetricName, Float64Array>> = {};
  for (const [k, v] of Object.entries(metrics)) m[k as MetricName] = Float64Array.from(v);
  return { ...base, metrics: m, meta: { ...base.meta, sport, sessions } };
}

const session = (sport: string, start_index: number, end_index: number): Session => ({ sport, sub_sport: '', start: start_index, end: end_index, start_index, end_index, totals: {} });
const ramp = (n: number, f: (i: number) => number): number[] => Array.from({ length: n }, (_, i) => f(i));
const all = (n: number): Uint32Array => Uint32Array.from(ramp(n - 1, (i) => i + 1));

describe('offeredMetrics (AC 1, C-3)', () => {
  it('a ride with heart rate but no power: speed, heart rate and gradient, not power', () => {
    expect(offeredMetrics(golden('run-30min'))).toEqual(['speed', 'heart_rate', 'gradient']);
    expect(offeredMetrics(workout(3, { speed: [1, 2, 3], heart_rate: [100, 110, 120], altitude: [1, 2, 3], distance: [0, 1, 2] }))).toEqual(['speed', 'heart_rate', 'gradient']);
  });
  it('gradient only with altitude and distance; power when present', () => {
    expect(offeredMetrics(workout(3, { power: [100, 100, 100], altitude: [1, 2, 3] }))).toEqual(['power']);
    expect(offeredMetrics(workout(3, { distance: [0, 1, 2] }))).toEqual([]);
    expect(offeredMetrics(golden('ride-30min'))).toEqual(['speed', 'heart_rate', 'power', 'gradient']);
    expect(offeredMetrics(golden('no-distance-with-gps'))).toEqual(['heart_rate', 'gradient']);
  });
  it('a value only on samples without a position does not count; no position stream: nothing', () => {
    const d = positioned([N, 47, 47.001], [N, 8, 8]);
    expect(offeredMetrics({ ...d, metrics: { heart_rate: Float64Array.from([120, N, N]) } })).toEqual([]);
    expect(offeredMetrics(golden('indoor-no-gps'))).toEqual([]);
  });
});

describe('metricLabel (AC 4, DS-2)', () => {
  it.each([
    ['running', 'Pace'],
    ['swimming', 'Pace'],
    ['rowing', 'Pace'],
    ['cycling', 'Speed'],
    ['multisport', 'Speed'],
  ])('%s: %s', (sport, label) => expect(metricLabel('speed', sport)).toBe(label));
  it('the others', () => {
    expect([metricLabel('heart_rate', 'running'), metricLabel('power', ''), metricLabel('gradient', '')]).toEqual(['Heart rate', 'Power', 'Gradient']);
  });
});

describe('centredMean (C-3)', () => {
  const time = Float64Array.from(ramp(20, (i) => i));
  it('centred over the window; a missing sample stays missing and is never counted', () => {
    const raw = Float64Array.from(ramp(20, (i) => (i === 10 ? N : i)));
    const v = centredMean(time, new Uint32Array(0), raw, 10);
    expect(v[10]).toBeNaN();
    expect(v[8]).toBeCloseTo((3 + 4 + 5 + 6 + 7 + 8 + 9 + 11 + 12 + 13) / 10);
    expect(v[0]).toBeCloseTo((0 + 1 + 2 + 3 + 4 + 5) / 6);
  });
  it('never crosses a time gap', () => {
    const t = Float64Array.from([0, 1, 2, 3, 20, 21, 22]);
    const raw = Float64Array.from([1, 1, 1, 1, 9, 9, 9]);
    const v = centredMean(t, Uint32Array.from([4]), raw, 60);
    expect(Array.from(v)).toEqual([1, 1, 1, 1, 9, 9, 9]);
  });
});

describe('gradientValues (C-3)', () => {
  it('a steady 5 % climb is 5 % wherever the window spans 50 m or more', () => {
    const n = 100;
    const g = gradientValues(workout(n, { distance: ramp(n, (i) => i * 5), altitude: ramp(n, (i) => i * 0.25) }));
    for (let i = 0; i < n; i++) if (g[i] === g[i]) expect(g[i]).toBeCloseTo(5);
    expect(g[0]).toBeCloseTo(5); // [0, 50 m]: exactly the shortest span
    expect(g[50]).toBeCloseTo(5);
  });
  it('a standstill or held distance gives no class, never ±∞; missing altitude gives none; a short route gives none', () => {
    const dist = [0, 10, 20, 20, 20, 30, 40, 50, 60, 70, 80];
    const alt = [0, 1, 2, 2, N, 3, 4, 5, 6, 7, 8];
    const g = gradientValues(workout(dist.length, { distance: dist, altitude: alt }));
    expect(g[3]).toBeNaN();
    expect(g[4]).toBeNaN();
    for (const v of g) expect(Number.isFinite(v) || Number.isNaN(v)).toBe(true);
    expect(g[5]).toBeCloseTo(10);
    expect(Array.from(gradientValues(workout(3, { distance: [0, 10, 20], altitude: [0, 1, 2] }))).every(Number.isNaN)).toBe(true);
  });
  it('flat-noise (±1 m white noise per sample): mostly flat, never a steep class', () => {
    const d = golden('flat-noise');
    const values = metricValues(d, 'gradient');
    const classes = classify(values, colourScale('gradient', values, all(d.sampleCount), d.meta.sport));
    let flat = 0;
    for (const c of classes) {
      if (c === 5) flat++;
      expect(c === 0 || c === 10).toBe(false);
    }
    expect(flat / d.sampleCount).toBeGreaterThan(0.8);
  });
});

describe('transitions (F-08 hand-off, C-3)', () => {
  it('no class for speed, power and gradient in a transition session; heart rate keeps one', () => {
    const d = golden('ride-transition-ride');
    const t = d.meta.sessions.find((s) => s.sport === 'transition');
    if (!t) throw new Error('no transition');
    const inside = (t.start_index + t.end_index) >> 1;
    for (const m of ['speed', 'power', 'gradient'] as const) expect(metricValues(d, m)[inside], m).toBeNaN();
    expect(metricValues(d, 'heart_rate')[inside]).toBeGreaterThan(0);
    const s = workout(4, { speed: [5, 5, 5, 5] }, 'multisport', [session('cycling', 0, 2), session('transition', 2, 4)]);
    expect(Array.from(metricValues(s, 'speed'))).toEqual([5, 5, N, N]);
  });
});

describe('colourScale and classify (AC 3, C-4)', () => {
  const n = 120;
  const hr = workout(n, { heart_rate: ramp(n, (i) => 100 + i) });
  const count = (c: Uint8Array, k: number): number => c.filter((x) => x === k).length;
  it('edges are the k/12 quantiles of the drawn edges: on spread data each class holds a twelfth; monotonic, no clipping', () => {
    const values = metricValues(hr, 'heart_rate');
    const scale = colourScale('heart_rate', values, Uint32Array.from(ramp(n, (i) => i)), 'cycling');
    expect(scale.edges).toEqual(ramp(CLASS_COUNT - 1, (k) => 110 + 10 * k));
    const c = classify(values, scale);
    for (let i = 1; i < n; i++) expect(c[i]).toBeGreaterThanOrEqual(c[i - 1]);
    for (let k = 0; k < CLASS_COUNT; k++) expect(count(c, k)).toBe(n / CLASS_COUNT);
    // Beyond the drawn values: still the end classes.
    expect(Array.from(classify(Float64Array.from([0, 1000]), scale))).toEqual([0, CLASS_COUNT - 1]);
  });
  it('a skewed metric still gives each class about a twelfth', () => {
    const values = Float64Array.from(ramp(1200, (i) => 100 + (i / 12) ** 2));
    const c = classify(values, colourScale('heart_rate', values, all(1200), 'cycling'));
    for (let k = 0; k < CLASS_COUNT; k++) expect(Math.abs(count(c, k) - 100)).toBeLessThanOrEqual(1);
  });
  it('only the drawn edges set the edges', () => {
    const values = metricValues(hr, 'heart_rate');
    const scale = colourScale('heart_rate', values, Uint32Array.from(ramp(12, (i) => 10 + i)), 'cycling');
    expect(scale.edges).toEqual(ramp(CLASS_COUNT - 1, (k) => 111 + k));
  });
  it('ties leave classes empty, never out of order; a constant metric gives one class', () => {
    const values = Float64Array.from([...ramp(10, () => 120), ...ramp(10, () => 150)]);
    const c = classify(values, colourScale('heart_rate', values, all(20), 'cycling'));
    expect(new Set(c)).toEqual(new Set([5, CLASS_COUNT - 1]));
    for (let i = 1; i < c.length; i++) expect(c[i]).toBeGreaterThanOrEqual(c[i - 1]);
    const flat = Float64Array.from([120, 120, N, 120]);
    expect(new Set(classify(flat, colourScale('heart_rate', flat, Uint32Array.from([1, 2, 3]), 'cycling')))).toEqual(new Set([CLASS_COUNT - 1, NO_CLASS]));
  });
  it('NaN gives NO_CLASS; no drawn value gives NaN edges and dash labels', () => {
    const values = Float64Array.from([120, 120, N, 120]);
    const none = colourScale('heart_rate', values, Uint32Array.from([2]), 'cycling');
    expect(none.edges.every(Number.isNaN)).toBe(true);
    expect(classify(values, none)[2]).toBe(NO_CLASS);
    expect(legend(none, 'cycling', 'metric').labels).toEqual(['–', '–', '–']);
  });
  it('speed sports: a stop takes the slowest class but does not move the edges', () => {
    const d = workout(40, { speed: ramp(40, (i) => (i < 20 ? 0 : i)) });
    const values = metricValues(d, 'speed');
    const scale = colourScale('speed', values, all(40), 'cycling');
    expect(scale.edges[0]).toBeGreaterThanOrEqual(0.3);
    expect(classify(values, scale)[0]).toBe(0);
  });
  it('pace sports class in pace, slow → fast; a stop has no class (DS-2)', () => {
    const speed = ramp(60, (i) => (i < 10 ? 0 : i < 35 ? 2.5 : 5));
    const d = workout(60, { speed }, 'running');
    const values = metricValues(d, 'speed');
    expect(values[0]).toBeNaN();
    expect(values[20]).toBeCloseTo(1 / 2.5);
    const scale = colourScale('speed', values, all(60), 'running');
    expect(scale.reverse).toBe(true);
    const c = classify(values, scale);
    expect(c[0]).toBe(NO_CLASS);
    expect(c[20]).toBe(0); // slow
    expect(c[50]).toBeGreaterThan(5); // fast; two tied values, so the classes between them stay empty
  });
  it('gradient uses the 10 fixed edges: 11 classes, flat (±1 %) in the middle', () => {
    const values = Float64Array.from([-20, -12, -6, -1, 0, 0.99, 1, 4, 12, 30]);
    const scale = colourScale('gradient', values, Uint32Array.from([1]), 'cycling');
    expect(scale.edges).toEqual(GRADIENT_EDGES);
    expect(GRADIENT_EDGES).toHaveLength(10);
    expect(Array.from(classify(values, scale))).toEqual([0, 1, 2, 5, 5, 5, 6, 7, 10, 10]);
    expect(GRADIENT_LABELS.every((e) => GRADIENT_EDGES.includes(e))).toBe(true);
  });
});

describe('legend (AC 3, AC 4, DS-1, DS-2)', () => {
  /** Edges lo … hi with the median edge at mid. */
  const scale = (metric: ColourScale['metric'], lo: number, mid: number, hi: number, reverse = false): ColourScale => ({
    metric,
    edges: [lo, ...ramp(4, () => lo), mid, ...ramp(4, () => hi), hi],
    reverse,
  });
  it('speed in km/h or mph: lowest, median and highest edge; a units switch only relabels', () => {
    expect(legend(scale('speed', 5, 7.5, 10), 'cycling', 'metric')).toEqual({ labels: ['≤ 18.0', '27.0', '≥ 36.0'], unit: 'km/h' });
    expect(legend(scale('speed', 5, 7.5, 10), 'cycling', 'imperial')).toEqual({ labels: ['≤ 11.2', '16.8', '≥ 22.4'], unit: 'mph' });
  });
  it.each([
    ['running', 'metric', ['≥ 6:40', '5:00', '≤ 3:20'], '/km'],
    ['running', 'imperial', ['≥ 10:44', '8:03', '≤ 5:22'], '/mi'],
    ['swimming', 'metric', ['≥ 0:40', '0:30', '≤ 0:20'], '/100 m'],
    ['swimming', 'imperial', ['≥ 0:37', '0:27', '≤ 0:18'], '/100 yd'],
    ['rowing', 'imperial', ['≥ 3:20', '2:30', '≤ 1:40'], '/500 m'],
  ] as const)('pace for %s (%s): slow pace on the left, the larger number marked ≥', (sport, units, labels, unit) => {
    // edges in s/m: 0.2 s/m = 5 m/s (fast), 0.4 s/m = 2.5 m/s (slow)
    expect(legend(scale('speed', 0.2, 0.3, 0.4, true), sport, units)).toEqual({ labels, unit });
  });
  it('heart rate, power and gradient', () => {
    expect(legend(scale('heart_rate', 110.4, 140, 171.6), 'cycling', 'imperial')).toEqual({ labels: ['≤ 110', '140', '≥ 172'], unit: 'bpm' });
    expect(legend(scale('power', 0, 180, 312), 'cycling', 'metric')).toEqual({ labels: ['≤ 0', '180', '≥ 312'], unit: 'W' });
    expect(legend(colourScale('gradient', new Float64Array(0), new Uint32Array(0), 'cycling'), 'cycling', 'imperial')).toEqual({ labels: ['-12', '-5', '5', '12'], unit: '%' });
  });
});

describe('golden files', () => {
  it('ride-12h: every metric classes every drawn edge it has a value for (a few ms each)', () => {
    const d = golden('ride-12h');
    const r = buildRoute(d);
    if (!r) throw new Error('no route');
    const ends = recordedEdgeEnds(d, r);
    for (const m of offeredMetrics(d)) {
      const values = metricValues(d, m);
      const c = classify(values, colourScale(m, values, ends, d.meta.sport));
      for (const i of ends) if (values[i] === values[i]) expect(c[i]).toBeLessThan(CLASS_COUNT);
    }
  });
});
