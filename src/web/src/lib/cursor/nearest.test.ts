// nearestIndex (F-05 design, Tests; review decision 5: tie → earliest sample).

import { describe, expect, it } from 'vitest';
import { nearestIndex } from './index';

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

function linearNearest(values: Float64Array, target: number): number {
  let best = 0;
  for (let i = 1; i < values.length; i++) {
    if (Math.abs(values[i] - target) < Math.abs(values[best] - target)) best = i;
  }
  return best;
}

describe('nearestIndex', () => {
  const v = f([0, 10, 20, 30]);

  it('finds an exact hit', () => {
    expect(nearestIndex(v, 20)).toBe(2);
    expect(nearestIndex(v, 0)).toBe(0);
    expect(nearestIndex(v, 30)).toBe(3);
  });

  it('picks the closer of two neighbours', () => {
    expect(nearestIndex(v, 13)).toBe(1);
    expect(nearestIndex(v, 17)).toBe(2);
  });

  it('picks the lower index at the exact midpoint', () => {
    expect(nearestIndex(v, 15)).toBe(1);
    expect(nearestIndex(v, 5)).toBe(0);
    expect(nearestIndex(v, 25)).toBe(2);
  });

  it('clamps below the first and above the last value', () => {
    expect(nearestIndex(v, -100)).toBe(0);
    expect(nearestIndex(v, 1e9)).toBe(3);
  });

  it('handles a single value', () => {
    expect(nearestIndex(f([5]), 5)).toBe(0);
    expect(nearestIndex(f([5]), -1)).toBe(0);
    expect(nearestIndex(f([5]), 99)).toBe(0);
  });

  it('returns −1 for non-finite targets', () => {
    expect(nearestIndex(v, Number.NaN)).toBe(-1);
    expect(nearestIndex(v, Number.POSITIVE_INFINITY)).toBe(-1);
    expect(nearestIndex(v, Number.NEGATIVE_INFINITY)).toBe(-1);
  });

  it('handles a large gap', () => {
    const g = f([0, 1, 2, 10000, 10001]);
    expect(nearestIndex(g, 5000)).toBe(2);
    expect(nearestIndex(g, 5001)).toBe(2);
    expect(nearestIndex(g, 5002)).toBe(3);
  });

  it('equals a linear-search reference on 1,000 seeded targets over 43,200 values', () => {
    const r = rng(42);
    const values = new Float64Array(43200);
    let t = 0;
    for (let i = 0; i < values.length; i++) {
      values[i] = t;
      // Mostly 1 s steps, sometimes gaps, sometimes fractional steps.
      const p = r();
      t += p < 0.001 ? 5 + Math.floor(r() * 7200) : p < 0.1 ? 0.25 + r() : 1;
    }
    const lo = values[0] - 100;
    const hi = values[values.length - 1] + 100;
    for (let k = 0; k < 1000; k++) {
      // Some targets exactly at midpoints, to exercise the tie rule.
      let target: number;
      if (k % 10 === 0) {
        const i = 1 + Math.floor(r() * (values.length - 1));
        target = (values[i - 1] + values[i]) / 2;
      } else {
        target = lo + r() * (hi - lo);
      }
      expect(nearestIndex(values, target)).toBe(linearNearest(values, target));
    }
  });
});
