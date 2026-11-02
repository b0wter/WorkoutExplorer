// F-18 L-4 split boundaries (AC 1, AC 3) and the cross-language check of L-1's computed lap values
// against lib/range on the synthetic golden ride-laps-no-totals (B-1 hand-off).
import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile } from '../datafile';
import { M_PER_MILE } from '../format';
import { carriedDistance, createRangeStats, emptyValues, splitBounds } from './index';

const N = Number.NaN;

/** A run of `metres` in steps of `step` metres per sample, from `start`. */
function run(metres: number, step = 3, start = 0): Float64Array {
  const n = Math.round(metres / step) + 1;
  return Float64Array.from({ length: n }, (_, i) => start + Math.min(i * step, metres));
}

/** The splits' distances in metres. */
function lengths(d: Float64Array, length: number): number[] {
  const b = splitBounds(d, length);
  return b.slice(1).map((x, i) => Math.round((d[x] - d[b[i]]) * 10) / 10);
}

describe('splitBounds (L-4)', () => {
  it('AC 1: 10.4 km gives ten 1 km splits and one of 0.4 km; in miles six and a partial', () => {
    const d = run(10400, 1, 0.3);
    expect(lengths(d, 1000)).toEqual([...Array(10).fill(1000), 400]);
    const mi = lengths(d, M_PER_MILE);
    expect(mi).toHaveLength(7);
    for (const x of mi.slice(0, 6)) expect(Math.abs(x - M_PER_MILE)).toBeLessThan(1);
    expect(Math.abs(mi[6] - (10400 - 6 * M_PER_MILE))).toBeLessThan(1); // 1 m steps
  });

  it('a sample at exactly k·L ends split k; adjacent splits share it', () => {
    const d = Float64Array.from([0, 400, 1000, 1500, 2000]);
    expect(splitBounds(d, 1000)).toEqual([0, 2, 4]);
    // From a non-zero start with float error in the difference: 0.3 + 1000 − 0.3.
    expect(splitBounds(Float64Array.from([0.3, 500.3, 1000.3, 1200.3]), 1000)).toEqual([0, 2, 3]);
  });

  it('a boundary is the first sample at or beyond k·L; flat stretches and missing distance are carried', () => {
    const raw = Float64Array.from([N, 0, 300, N, N, 900, 900, 900, 1300, N, 2100, 2100]);
    const d = carriedDistance(raw);
    if (!d) throw new Error('no distance');
    // 1300 ≥ 1000 at sample 8; 2100 ≥ 2000 at sample 10; the flat tail 10…11 is in no split.
    expect(splitBounds(d, 1000)).toEqual([0, 8, 10]);
  });

  it('no zero-distance tail row; a tail that advances becomes the partial split', () => {
    expect(splitBounds(Float64Array.from([0, 500, 1000, 1000, 1000]), 1000)).toEqual([0, 2]);
    expect(splitBounds(Float64Array.from([0, 500, 1000, 1000, 1001]), 1000)).toEqual([0, 2, 4]);
  });

  it('a jump over several multiples ends one split, so no split is empty', () => {
    expect(splitBounds(Float64Array.from([0, 900, 3100, 3500]), 1000)).toEqual([0, 2, 3]);
  });

  it('AC 3: a distance shorter than one split gives one partial split', () => {
    expect(splitBounds(Float64Array.from([5, 200, 700]), 1000)).toEqual([0, 2]);
  });
});

describe('computed lap values = lib/range over the lap (L-1, L-2)', () => {
  it('ride-laps-no-totals: each lap [start_index, end_index − 1] within quantisation plus rounding', () => {
    const golden = loadGolden().find((g) => g.name === 'ride-laps-no-totals');
    if (!golden) throw new Error('no golden ride-laps-no-totals');
    const r = decodeDataFile(golden.bin);
    if (!r.ok) throw new Error(r.error.message);
    const data = r.data;
    const stats = createRangeStats(data);
    const v = emptyValues();
    const laps = data.meta.laps;
    expect(laps.length).toBeGreaterThan(1);
    const q = 0.0005; // 3-decimal rounding of the metadata
    for (const [k, lap] of laps.entries()) {
      const t = lap.totals;
      stats.query(lap.start_index, lap.end_index - 1, v);
      const near = (key: keyof typeof t, got: number, step: number): void => {
        expect(t[key], `lap ${k} ${key}`).toBeDefined();
        expect(Math.abs(got - (t[key] as number)), `lap ${k} ${key}`).toBeLessThanOrEqual(step + q);
      };
      near('elapsed_time', v.elapsed, 1);
      near('distance', v.distance, 0.1);
      near('ascent', v.gain, 0.1);
      // speed = d / m: the distance step over the moving time, plus the moving-time step.
      near('avg_speed', v.speed, (0.1 + v.speed * 1) / v.moving);
      // Means of integer-quantised samples: half a step per sample.
      near('avg_heart_rate', v.heartRate, 0.5);
      near('avg_power', v.power, 0.5);
      near('avg_cadence', v.cadence, 0.5);
    }
  });
});
