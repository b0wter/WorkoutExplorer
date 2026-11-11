// Range statistics (F-17 R-2, R-4; verification AC 2, AC 7): against a brute-force reference on
// seeded synthetic data and on the goldens; additivity; whole-workout moving time.
import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../datafile';
import { carriedDistance, createRangeStats, emptyValues, type RangeValues } from './index';

const N = Number.NaN;

/** Deterministic PRNG (mulberry32). */
function rng(seed: number): () => number {
  let s = seed >>> 0;
  return () => {
    s = (s + 0x6d2b79f5) >>> 0;
    let t = s;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Synthetic workout: missing runs, a gap, flat and missing distance, zero cadence and power. */
function synthetic(seed: number, n = 400): WorkoutData {
  const r = rng(seed);
  const time = new Float64Array(n);
  const dist = new Float64Array(n);
  const hr = new Float64Array(n);
  const power = new Float64Array(n);
  const cad = new Float64Array(n);
  const temp = new Float64Array(n);
  const moving = new Float64Array(n);
  const gain = new Float64Array(n);
  const alt = new Float64Array(n);
  let t = 0;
  let d = 100;
  for (let i = 0; i < n; i++) {
    t += i === 200 ? 600 : 1;
    time[i] = t;
    if (r() > 0.3) d += Math.round(r() * 80) / 10; // flat stretches
    dist[i] = i < 5 || (i > 50 && i < 60) || r() < 0.05 ? N : d;
    hr[i] = r() < 0.1 ? N : 100 + Math.floor(r() * 80);
    power[i] = r() < 0.1 ? N : r() < 0.2 ? 0 : Math.floor(r() * 400);
    cad[i] = r() < 0.1 ? N : r() < 0.2 ? 0 : 60 + Math.floor(r() * 40);
    temp[i] = i > 300 ? N : -5 + Math.floor(r() * 30);
    moving[i] = i === 0 ? 0 : moving[i - 1] + (r() < 0.8 && i !== 200 ? 1 : 0);
    gain[i] = i === 0 ? 0 : gain[i - 1] + (r() < 0.1 ? Math.round(r() * 30) / 10 : 0);
    alt[i] = (i > 100 && i < 140) || r() < 0.1 ? N : 200 + gain[i];
  }
  return {
    sampleCount: n,
    time,
    gaps: new Uint32Array([200]),
    metrics: { altitude: alt, distance: dist, heart_rate: hr, power, cadence: cad, temperature: temp },
    movingTime: moving,
    elevationGain: gain,
    meta: { sport: 'cycling', start_time: '2026-10-01T08:00:00Z', time_zone: { known: false, offset_seconds: 0 }, summary: {}, device: { name: '', sensors: [] }, sessions: [], laps: [] },
  };
}

function avg(v: Float64Array | undefined, a: number, b: number, skipZero = false): number {
  if (!v) return N;
  let s = 0;
  let c = 0;
  for (let i = a; i <= b; i++) {
    if (Number.isNaN(v[i]) || (skipZero && v[i] === 0)) continue;
    s += v[i];
    c++;
  }
  return c > 0 ? s / c : N;
}

function reference(d: WorkoutData, a: number, b: number): RangeValues {
  const dist = d.metrics.distance;
  let distance = N;
  if (dist && dist.some((v) => !Number.isNaN(v))) {
    const first = dist.find((v) => !Number.isNaN(v)) as number;
    const at = (i: number): number => {
      for (let k = i; k >= 0; k--) if (!Number.isNaN(dist[k])) return dist[k];
      return first;
    };
    distance = at(b) - at(a);
  }
  const moving = d.movingTime ? d.movingTime[b] - d.movingTime[a] : N;
  return {
    elapsed: d.time[b] - d.time[a],
    moving,
    distance,
    gain: d.elevationGain && !Number.isNaN(avg(d.metrics.altitude, a, b)) ? d.elevationGain[b] - d.elevationGain[a] : N,
    speed: moving > 0 ? distance / moving : N,
    heartRate: avg(d.metrics.heart_rate, a, b),
    power: avg(d.metrics.power, a, b),
    cadence: avg(d.metrics.cadence, a, b, true),
    temperature: avg(d.metrics.temperature, a, b),
  };
}

function expectClose(actual: RangeValues, expected: RangeValues, where: string): void {
  for (const k of Object.keys(expected) as (keyof RangeValues)[]) {
    if (Number.isNaN(expected[k])) expect(actual[k], `${where} ${k}`).toBeNaN();
    else expect(actual[k], `${where} ${k}`).toBeCloseTo(expected[k], 6);
  }
}

const goldens = loadGolden().map((g) => {
  const r = decodeDataFile(g.bin);
  if (!r.ok) throw new Error(g.name);
  return [g.name, r.data] as const;
});

describe('createRangeStats', () => {
  it('matches the brute-force reference on seeded synthetic data', () => {
    for (const seed of [1, 2, 3]) {
      const d = synthetic(seed);
      const stats = createRangeStats(d);
      const out = emptyValues();
      const r = rng(seed + 100);
      for (let k = 0; k < 300; k++) {
        const a = Math.floor(r() * d.sampleCount);
        const b = Math.floor(r() * d.sampleCount);
        const [lo, hi] = a <= b ? [a, b] : [b, a];
        expectClose(stats.query(lo, hi, out), reference(d, lo, hi), `seed ${seed} [${lo}, ${hi}]`);
      }
    }
  });

  it('distance is never negative and 0 inside a stretch without distance or progress', () => {
    const d = synthetic(4);
    const stats = createRangeStats(d);
    const out = emptyValues();
    for (let a = 0; a < d.sampleCount - 1; a += 3) {
      for (let b = a + 1; b < d.sampleCount; b += 17) expect(stats.query(a, b, out).distance).toBeGreaterThanOrEqual(0);
    }
    expect(stats.query(0, 4, out).distance).toBe(0); // before the first valid distance
    expect(stats.query(52, 58, out).distance).toBe(0); // inside a dropout
  });

  it('a range without samples of a stream shows NaN, never 0', () => {
    const d = synthetic(5);
    const out = createRangeStats(d).query(320, 330, emptyValues());
    expect(out.temperature).toBeNaN();
    // Gain without a valid altitude sample in the range (QA-18-r1-1, as Go workout/laps.go).
    expect(createRangeStats(d).query(105, 135, emptyValues()).gain).toBeNaN();
  });

  it('distance, moving time, gain and elapsed time are additive: [a, c] = [a, b] + [b, c]', () => {
    for (const [name, d] of goldens) {
      const stats = createRangeStats(d);
      const n = d.sampleCount;
      const [a, b, c] = [0, Math.floor(n / 3), n - 1];
      const ac = { ...stats.query(a, c, emptyValues()) };
      const ab = { ...stats.query(a, b, emptyValues()) };
      const bc = { ...stats.query(b, c, emptyValues()) };
      for (const k of ['distance', 'moving', 'gain', 'elapsed'] as const) {
        if (Number.isNaN(ab[k]) || Number.isNaN(bc[k])) continue; // gain: a part without altitude
        expect(ab[k] + bc[k], `${name} ${k}`).toBeCloseTo(ac[k], 6);
      }
    }
  });

  it('matches the reference on the goldens', () => {
    for (const [name, d] of goldens) {
      const stats = createRangeStats(d);
      const n = d.sampleCount;
      const out = emptyValues();
      for (const [a, b] of [[0, n - 1], [Math.floor(n / 4), Math.floor(n / 2)], [n - 2, n - 1]]) {
        if (a >= b) continue;
        expectClose(stats.query(a, b, out), reference(d, a, b), `${name} [${a}, ${b}]`);
      }
    }
  });

  it('AC 7: a whole-workout range has the summary moving time', () => {
    let checked = 0;
    for (const [name, d] of goldens) {
      const mt = d.meta.summary.moving_time;
      if (mt === undefined || !d.movingTime) continue;
      const out = createRangeStats(d).query(0, d.sampleCount - 1, emptyValues());
      expect(out.moving, name).toBeCloseTo(mt, 0);
      checked++;
    }
    expect(checked).toBeGreaterThan(3);
  });

  it('lists only the fields the workout has, in panel order', () => {
    expect(createRangeStats(synthetic(1)).fields).toEqual([
      'distance', 'elapsed', 'moving', 'gain', 'speed', 'heartRate', 'power', 'cadence', 'temperature',
    ]);
    const d = synthetic(1);
    const bare: WorkoutData = { ...d, metrics: { cadence: new Float64Array(d.sampleCount) }, movingTime: undefined, elevationGain: undefined };
    expect(createRangeStats(bare).fields).toEqual(['elapsed']); // all-zero cadence has no average
  });

  it('carriedDistance: null without any valid distance', () => {
    expect(carriedDistance(undefined)).toBeNull();
    expect(carriedDistance(new Float64Array([N, N]))).toBeNull();
    expect([...(carriedDistance(new Float64Array([N, 5, N, 7])) as Float64Array)]).toEqual([5, 5, 5, 7]);
  });
});
