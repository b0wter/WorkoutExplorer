// The x axis (F-17 R-7): time and distance axes, the pointer lookup keys, decimation with
// missing and flat distance.
import { describe, expect, it } from 'vitest';
import { nearestIndex } from '../cursor/nearest';
import { decimate, distanceAxis, timeAxis, type ChartAxis } from './index';

const N = Number.NaN;
const sampleAt = (axis: ChartAxis, v: number): number => {
  const k = nearestIndex(axis.keys, v);
  return k < 0 || axis.index === null ? k : axis.index[k];
};

describe('timeAxis', () => {
  it('is the time array itself, its first to last value', () => {
    const t = new Float64Array([5, 6, 9]);
    const a = timeAxis(t);
    expect([a.min, a.max, a.index]).toEqual([5, 9, null]);
    expect(a.x).toBe(t);
    expect(sampleAt(a, 8)).toBe(2);
    expect(timeAxis(new Float64Array([4])).max).toBe(5);
  });
});

describe('distanceAxis', () => {
  const d = new Float64Array([N, 10, 10, 10, N, 12, 15, 15, N]);

  it('null when the distance never advances or is missing', () => {
    expect(distanceAxis(new Float64Array([N, N]))).toBeNull();
    expect(distanceAxis(new Float64Array([7, 7, N, 7]))).toBeNull();
  });

  it('range from the first to the last valid distance; at carries forward, from the first', () => {
    const a = distanceAxis(d) as ChartAxis;
    expect([a.min, a.max]).toEqual([10, 15]);
    expect([...a.at]).toEqual([10, 10, 10, 10, 10, 12, 15, 15, 15]);
    expect([...a.keys]).toEqual([10, 12, 15]);
    expect([...(a.index as Uint32Array)]).toEqual([1, 5, 6]);
  });

  it('the pointer maps to the earliest sample nearest its distance, never one without distance', () => {
    const a = distanceAxis(d) as ChartAxis;
    expect(sampleAt(a, 0)).toBe(1);
    expect(sampleAt(a, 10.9)).toBe(1);
    expect(sampleAt(a, 11)).toBe(1); // tie → earliest
    expect(sampleAt(a, 14)).toBe(6);
    expect(sampleAt(a, 99)).toBe(6);
    expect(sampleAt(a, N)).toBe(-1);
  });

  it('decimation: missing distance is a break, a flat stretch folds into one column, x non-decreasing', () => {
    const a = distanceAxis(d) as ChartAxis;
    const y = new Float64Array([1, 2, 8, 3, 4, 5, 6, 7, 9]);
    const out = decimate(a.x, y, new Uint32Array(0), a.min, a.max, 100);
    for (let k = 1; k < out.x.length; k++) expect(out.x[k]).toBeGreaterThanOrEqual(out.x[k - 1]);
    expect(out.x.every((v) => Number.isFinite(v))).toBe(true);
    // Samples 1–3 at distance 10 fold into one column: min 2 and max 8 kept, in index order.
    expect(out.y.slice(0, 2)).toEqual([2, 8]);
    expect(out.y).toContain(null); // the missing distance at sample 4
    expect(out.y).not.toContain(1); // sample 0 has no x
    expect(out.y).not.toContain(9);
  });
});
