// Decimation with breaks (F-05 design, "Breaks and decimation", Tests; AC 3, AC 4).
// Every property of the design's table on seeded random data, plus hand cases
// and the golden pause-hours file. Synthetic data only (ADR-004 item 7).

import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { GAP_SECONDS, decodeDataFile } from '../datafile';
import { bucketCount, decimate, presentCharts, type Decimated } from './index';

const NaN_ = Number.NaN;
const f = (xs: number[]) => Float64Array.from(xs);

/** Seeded PRNG (mulberry32). */
function rng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Gap indices as F-04 computes them: i with x[i] − x[i−1] ≥ 5. */
function gapsOf(x: Float64Array): Uint32Array {
  const g: number[] = [];
  for (let i = 1; i < x.length; i++) if (x[i] - x[i - 1] >= GAP_SECONDS) g.push(i);
  return Uint32Array.from(g);
}

function axis(x: Float64Array): [number, number] {
  return x.length > 1 ? [x[0], x[x.length - 1]] : [x[0], x[0] + 1];
}

function run(x: Float64Array, y: Float64Array, buckets: number, gaps = gapsOf(x)): Decimated {
  const [lo, hi] = axis(x);
  return decimate(x, y, gaps, lo, hi, buckets);
}

function bucketOf(v: number, lo: number, hi: number, buckets: number): number {
  return hi === lo ? 0 : Math.min(buckets - 1, Math.floor(((v - lo) / (hi - lo)) * buckets));
}

/** Segment number per sample (−1 for a missing sample) and the segment count. */
function segmentsOf(x: Float64Array, y: Float64Array): { seg: Int32Array; count: number } {
  const seg = new Int32Array(x.length).fill(-1);
  let count = 0;
  let inSegment = false;
  for (let i = 0; i < x.length; i++) {
    if (i > 0 && x[i] - x[i - 1] >= GAP_SECONDS) inSegment = false;
    if (Number.isNaN(y[i])) {
      inSegment = false;
      continue;
    }
    if (!inSegment) {
      count++;
      inSegment = true;
    }
    seg[i] = count - 1;
  }
  return { seg, count };
}

/** Asserts every property of the design's table. */
function checkProperties(x: Float64Array, y: Float64Array, buckets: number): void {
  const out = run(x, y, buckets);
  const [lo, hi] = axis(x);
  const { seg, count: segments } = segmentsOf(x, y);
  let valid = 0;
  for (let i = 0; i < y.length; i++) if (!Number.isNaN(y[i])) valid++;

  expect(out.x.length).toBe(out.y.length);

  // x strictly increasing.
  for (let k = 1; k < out.x.length; k++) expect(out.x[k]).toBeGreaterThan(out.x[k - 1]);

  // y never NaN; no leading or trailing break.
  for (const v of out.y) expect(Number.isNaN(v)).toBe(false);
  if (out.y.length > 0) {
    expect(out.y[0]).not.toBeNull();
    expect(out.y[out.y.length - 1]).not.toBeNull();
  }

  // Index of each sample by x, to tie emitted points to samples.
  const indexOfX = new Map<number, number>();
  for (let i = 0; i < x.length; i++) indexOfX.set(x[i], i);

  // Every data point is a sample with the same value; group emitted points by (segment, bucket).
  const emitted = new Map<string, number[]>();
  let breaks = 0;
  let segmentOfOutput = 0;
  for (let k = 0; k < out.x.length; k++) {
    const v = out.y[k];
    if (v === null) {
      breaks++;
      segmentOfOutput++;
      continue;
    }
    const i = indexOfX.get(out.x[k]);
    expect(i).toBeDefined();
    if (i === undefined) continue;
    expect(v).toBe(y[i]);
    // null only at breaks: a data point is never a missing sample.
    expect(Number.isNaN(y[i])).toBe(false);
    expect(seg[i]).toBe(segmentOfOutput);
    const key = `${seg[i]}/${bucketOf(x[i], lo, hi, buckets)}`;
    const list = emitted.get(key) ?? [];
    list.push(v);
    emitted.set(key, list);
  }

  // Breaks = segments − 1.
  expect(breaks).toBe(Math.max(0, segments - 1));

  // Envelope exact per bucket and segment (brute force).
  const brute = new Map<string, { min: number; max: number }>();
  for (let i = 0; i < x.length; i++) {
    if (seg[i] < 0) continue;
    const key = `${seg[i]}/${bucketOf(x[i], lo, hi, buckets)}`;
    const b = brute.get(key);
    if (b) {
      b.min = Math.min(b.min, y[i]);
      b.max = Math.max(b.max, y[i]);
    } else {
      brute.set(key, { min: y[i], max: y[i] });
    }
  }
  expect(new Set(emitted.keys())).toEqual(new Set(brute.keys()));
  for (const [key, b] of brute) {
    const values = emitted.get(key) ?? [];
    expect(values.length).toBeLessThanOrEqual(2);
    expect(Math.min(...values)).toBe(b.min);
    expect(Math.max(...values)).toBe(b.max);
  }

  // Length bounds.
  expect(out.x.length).toBeLessThanOrEqual(2 * buckets + 3 * segments);
  expect(out.x.length).toBeLessThanOrEqual(2 * valid);

  // All NaN → empty.
  if (valid === 0) expect(out).toEqual({ x: [], y: [] });
}

/** Random workout: mostly 1 s steps with random gaps; a random walk with random NaN runs. */
function randomSeries(seed: number, n: number): { x: Float64Array; y: Float64Array } {
  const r = rng(seed);
  const x = new Float64Array(n);
  const y = new Float64Array(n);
  let t = r() * 100;
  let v = 100;
  let nanLeft = 0;
  const gapRate = r() * 0.01;
  const nanRate = r() * 0.02;
  for (let i = 0; i < n; i++) {
    x[i] = t;
    const p = r();
    t += p < gapRate ? 5 + Math.floor(r() * 3600) : p < 0.2 ? 0.5 + r() * 4 : 1;
    if (nanLeft === 0 && r() < nanRate) nanLeft = 1 + Math.floor(r() * 60);
    v += (r() - 0.5) * 10;
    if (nanLeft > 0) {
      y[i] = NaN_;
      nanLeft--;
    } else {
      // Repeated values sometimes, so min/max ties are exercised.
      y[i] = r() < 0.1 ? 100 : Math.round(v * 10) / 10;
    }
  }
  return { x, y };
}

describe('decimate: properties on seeded random data', () => {
  const sizes = [1, 2, 3, 1000, 43200];
  const bucketsList = [1, 7, 640, 2400];
  for (const n of sizes) {
    for (const buckets of bucketsList) {
      it(`N ${n}, B ${buckets}`, () => {
        const seeds = n >= 43200 ? [1, 2] : [1, 2, 3, 4, 5, 6, 7, 8];
        for (const seed of seeds) {
          const { x, y } = randomSeries(seed * 7919 + n + buckets, n);
          checkProperties(x, y, buckets);
        }
      });
    }
  }

  it('all-NaN and single-valid-sample series', () => {
    const x = f([0, 1, 2, 3, 4]);
    checkProperties(x, f([NaN_, NaN_, NaN_, NaN_, NaN_]), 3);
    checkProperties(x, f([NaN_, NaN_, 7, NaN_, NaN_]), 3);
  });
});

describe('decimate: hand cases', () => {
  const x10 = f([0, 1, 2, 3, 4, 5, 6, 7, 8, 9]);

  it('a NaN run in the middle gives exactly one null, at the first missing sample’s time', () => {
    const out = run(x10, f([1, 2, 3, NaN_, NaN_, NaN_, 4, 5, 6, 7]), 100);
    expect(out.y.filter((v) => v === null)).toHaveLength(1);
    expect(out.x[out.y.indexOf(null)]).toBe(3);
    expect(out.x).toEqual([0, 1, 2, 3, 6, 7, 8, 9]);
  });

  it('a gap gives a null at the midpoint', () => {
    const x = f([0, 1, 2, 100, 101]);
    const out = run(x, f([1, 2, 3, 4, 5]), 1000);
    expect(out.x).toEqual([0, 1, 2, 51, 100, 101]);
    expect(out.y).toEqual([1, 2, 3, null, 4, 5]);
  });

  it('a 4 s step is not a gap', () => {
    const out = run(f([0, 4, 8]), f([1, 2, 3]), 1000);
    expect(out.y).not.toContain(null);
  });

  it('a NaN directly at a gap gives one break', () => {
    const x = f([0, 1, 2, 100, 101, 102]);
    const atGap = run(x, f([1, 2, 3, NaN_, 5, 6]), 1000);
    expect(atGap.y.filter((v) => v === null)).toHaveLength(1);
    expect(atGap.x[atGap.y.indexOf(null)]).toBe(51);
    const beforeGap = run(x, f([1, 2, NaN_, 4, 5, 6]), 1000);
    expect(beforeGap.y.filter((v) => v === null)).toHaveLength(1);
    expect(beforeGap.x[beforeGap.y.indexOf(null)]).toBe(2);
  });

  it('leading and trailing NaN give no leading or trailing break', () => {
    const out = run(x10, f([NaN_, NaN_, 1, 2, 3, 4, 5, NaN_, NaN_, NaN_]), 100);
    expect(out.y).not.toContain(null);
    expect(out.x).toEqual([2, 3, 4, 5, 6]);
  });

  it('a leading gap before the first valid sample gives no leading break', () => {
    const out = run(f([0, 100, 101]), f([NaN_, 1, 2]), 100);
    expect(out.y).toEqual([1, 2]);
  });

  it('all NaN gives empty arrays', () => {
    expect(run(x10, new Float64Array(10).fill(NaN_), 7)).toEqual({ x: [], y: [] });
  });

  it('a stream starting late keeps xMin/xMax from the arguments', () => {
    const x = new Float64Array(100);
    const y = new Float64Array(100).fill(NaN_);
    for (let i = 0; i < 100; i++) x[i] = i;
    for (let i = 50; i < 100; i++) y[i] = i % 7;
    // Samples 50..99 all lie in bucket 1 of [0, 99] with B = 2: one run, two points.
    const out = decimate(x, y, new Uint32Array(0), 0, 99, 2);
    expect(out.x.length).toBe(2);
    expect(out.x[0]).toBeGreaterThan(0);
    // min 0 first at 56, max 6 first at 55: emitted in index order.
    expect(out.y).toEqual([6, 0]);
  });

  it('the last sample lands in bucket B−1', () => {
    const x = f([0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
    const y = f([0, 0, 0, 0, 0, 0, 0, 0, -5, 1, 100]);
    // B = 5 over [0, 10]: samples 8, 9, 10 share bucket 4 (10 would be bucket 5 without the clamp).
    const out = run(x, y, 5);
    const tail = out.x.filter((v) => v >= 8);
    expect(tail).toEqual([8, 10]);
  });

  it('emits every sample when B > N', () => {
    const y = f([5, 3, 8, 1, 9, 2, 7, 4, 6, 0]);
    const out = run(x10, y, 100);
    expect(out.x).toEqual(Array.from(x10));
    expect(out.y).toEqual(Array.from(y));
  });

  it('emits min and max in index order, the first occurrence on ties', () => {
    const out = run(f([0, 1, 2, 3, 4]), f([5, 9, 1, 9, 1]), 1);
    expect(out.x).toEqual([1, 2]);
    expect(out.y).toEqual([9, 1]);
  });

  it('a constant bucket emits one point', () => {
    const out = run(f([0, 1, 2]), f([4, 4, 4]), 1);
    expect(out).toEqual({ x: [0], y: [4] });
  });

  it('never emits NaN, and never 0 where the input was NaN', () => {
    const y = f([1, NaN_, 2, NaN_, NaN_, 3, NaN_, 4, 5, NaN_]);
    const out = run(x10, y, 100);
    for (let k = 0; k < out.y.length; k++) {
      expect(Number.isNaN(out.y[k])).toBe(false);
      const i = out.x[k];
      if (Number.isNaN(y[i])) expect(out.y[k]).toBeNull();
    }
    expect(out.y).not.toContain(0);
  });

  it('a single sample with xMax = xMin uses bucket 0', () => {
    expect(decimate(f([5]), f([3]), new Uint32Array(0), 5, 5, 4)).toEqual({ x: [5], y: [3] });
  });

  it('does not change its input', () => {
    const x = Float64Array.from(x10);
    const y = f([1, NaN_, 2, 3, 4, 5, 6, 7, 8, 9]);
    const yCopy = Float64Array.from(y);
    run(x, y, 3);
    expect(x).toEqual(x10);
    expect(Array.from(y)).toEqual(Array.from(yCopy));
  });
});

describe('decimate: golden pause-hours', () => {
  const golden = loadGolden().find((g) => g.name === 'pause-hours');
  if (!golden) throw new Error('golden pause-hours missing');
  const r = decodeDataFile(golden.bin);
  if (!r.ok) throw new Error(r.error.message);
  const data = r.data;

  it('has a multi-hour gap', () => {
    let longest = 0;
    for (const g of data.gaps) longest = Math.max(longest, data.time[g] - data.time[g - 1]);
    expect(longest).toBeGreaterThanOrEqual(3600);
  });

  it('every charted stream has a null inside the multi-hour gap', () => {
    let at = 0;
    for (const g of data.gaps) if (data.time[g] - data.time[g - 1] >= 3600) at = g;
    const from = data.time[at - 1];
    const to = data.time[at];
    const charts = presentCharts(data);
    expect(charts.length).toBeGreaterThan(0);
    for (const spec of charts) {
      const values = data.metrics[spec.metric];
      if (!values) throw new Error('present chart without values');
      for (const buckets of [1, 640, 2400]) {
        const out = decimate(data.time, values, data.gaps, data.time[0], data.time[data.sampleCount - 1], buckets);
        const inside = out.x.some((x, k) => out.y[k] === null && x > from && x < to);
        expect(inside, `${spec.metric}, B ${buckets}`).toBe(true);
      }
    }
  });
});

describe('bucketCount', () => {
  it('rounds plot width × DPR', () => {
    expect(bucketCount(600, 1)).toBe(600);
    expect(bucketCount(600, 2)).toBe(1200);
    expect(bucketCount(333.3, 1.5)).toBe(500);
    expect(bucketCount(100.4, 1)).toBe(100);
    expect(bucketCount(100.5, 1)).toBe(101);
  });

  it('gives 0 for no width', () => {
    expect(bucketCount(0, 2)).toBe(0);
    expect(bucketCount(-10, 2)).toBe(0);
    expect(bucketCount(Number.NaN, 2)).toBe(0);
  });
});
