// Test-only data for lib/route and components/map tests (F-06 design, Tests). Never imported
// by app code. Synthetic only (ADR-004 item 7): arrays built here, or the committed synthetic
// golden data files.

import { loadGolden } from '../../../../test-support/golden.mjs';
import { decodeDataFile, type Lap, type WorkoutData } from '../../datafile';

let decoded: Map<string, WorkoutData> | null = null;

/** A decoded synthetic golden file by name (e.g. 'ride-12h'). */
export function golden(name: string): WorkoutData {
  if (decoded === null) {
    decoded = new Map();
    for (const g of loadGolden()) {
      const r = decodeDataFile(g.bin);
      if (!r.ok) throw new Error(`${g.name}: ${r.error.message}`);
      decoded.set(g.name, r.data);
    }
  }
  const d = decoded.get(name);
  if (!d) throw new Error(`no golden ${name}`);
  return d;
}

/** A lap starting at start_index (other fields are not read by lib/route). */
export function lap(start_index: number, end_index = start_index): Lap {
  return { start: start_index, end: end_index, start_index, end_index, trigger: '', totals: {} };
}

/** WorkoutData with time 0..n−1 and the given positions (NaN = no position); no position
 *  stream when lat is undefined. */
export function positioned(
  lat: readonly number[] | undefined,
  lon?: readonly number[],
  laps: readonly Lap[] = [],
): WorkoutData {
  const n = lat ? lat.length : 1;
  const time = new Float64Array(n);
  for (let i = 0; i < n; i++) time[i] = i;
  return {
    sampleCount: n,
    time,
    gaps: new Uint32Array(0),
    position: lat ? { latitude: Float64Array.from(lat), longitude: Float64Array.from(lon ?? lat) } : undefined,
    metrics: {},
    meta: {
      sport: 'cycling',
      start_time: '2026-06-06T05:00:00Z',
      time_zone: { known: true, offset_seconds: 0 },
      summary: {},
      device: { name: '', sensors: [] },
      sessions: [],
      laps,
    },
  };
}

/** A copy of data with positions NaN where hide(i) is true (start/end hiding stand-in). */
export function hidePositions(data: WorkoutData, hide: (i: number) => boolean): WorkoutData {
  const p = data.position;
  if (!p) throw new Error('no position stream');
  const latitude = Float64Array.from(p.latitude);
  const longitude = Float64Array.from(p.longitude);
  for (let i = 0; i < data.sampleCount; i++) {
    if (hide(i)) {
      latitude[i] = Number.NaN;
      longitude[i] = Number.NaN;
    }
  }
  return { ...data, position: { latitude, longitude } };
}

/** Deterministic PRNG in [0, 1) (mulberry32). */
export function prng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
