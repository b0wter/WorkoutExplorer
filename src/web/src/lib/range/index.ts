// Range statistics (F-17 design R-2 to R-4, range values table; ADR-029; AP-7). Pure. Built once
// per WorkoutData at full resolution; a query is O(1) and writes into a reused result.
//
// Distance, moving time and gain are differences of cumulative series (distance carried forward;
// moving time and gain from the server). Averages come from prefix sums of value and count, so
// missing samples drop out; power counts zeros, cadence does not. Stored values are integers in
// their scale, so the sums are exact in a double.

import type { WorkoutData } from '../datafile';

export interface RangeValues {
  /** Seconds, pauses included. */
  elapsed: number;
  /** Seconds (ADR-029 M). NaN without the stream. */
  moving: number;
  /** Metres, never negative. NaN without distance. */
  distance: number;
  /** Metres (ADR-029 G). NaN without the stream or without a valid altitude sample in the range. */
  gain: number;
  /** m/s: distance ÷ moving time; NaN when the moving time is 0 or either is absent. */
  speed: number;
  /** Means of the present samples; NaN when the range has none. */
  heartRate: number;
  power: number;
  cadence: number;
  temperature: number;
}

export type RangeField = keyof RangeValues;

export interface RangeStats {
  /** The fields this workout can show (the table's "shown when" column), in panel order. */
  readonly fields: readonly RangeField[];
  /** d̄, the carried distance (F-18 L-4); null without any valid distance. */
  readonly distance: Float64Array | null;
  /** a ≤ b, both inclusive sample indices. Writes into and returns `out`; allocates nothing. */
  query(a: number, b: number, out: RangeValues): RangeValues;
}

export function emptyValues(): RangeValues {
  const n = Number.NaN;
  return { elapsed: n, moving: n, distance: n, gain: n, speed: n, heartRate: n, power: n, cadence: n, temperature: n };
}

interface Prefix {
  readonly sum: Float64Array;
  readonly count: Float64Array;
}

/** sum[i] and count[i] over samples 0…i−1 (length N + 1). */
function prefix(values: Float64Array, skipZero: boolean): Prefix {
  const n = values.length;
  const sum = new Float64Array(n + 1);
  const count = new Float64Array(n + 1);
  let s = 0;
  let c = 0;
  for (let i = 0; i < n; i++) {
    const v = values[i];
    if (v === v && !(skipZero && v === 0)) {
      s += v;
      c++;
    }
    sum[i + 1] = s;
    count[i + 1] = c;
  }
  return { sum, count };
}

function mean(p: Prefix | null, a: number, b: number): number {
  if (p === null) return Number.NaN;
  const c = p.count[b + 1] - p.count[a];
  return c > 0 ? (p.sum[b + 1] - p.sum[a]) / c : Number.NaN;
}

/** d̄[i]: the valid distance at or before i, or else the first valid distance. null without any. */
export function carriedDistance(distance: Float64Array | undefined): Float64Array | null {
  if (distance === undefined) return null;
  const n = distance.length;
  let first = -1;
  for (let i = 0; i < n; i++) {
    if (distance[i] === distance[i]) {
      first = i;
      break;
    }
  }
  if (first < 0) return null;
  const out = new Float64Array(n);
  let last = distance[first];
  for (let i = 0; i < n; i++) {
    const v = distance[i];
    if (v === v) last = v;
    out[i] = last;
  }
  return out;
}

export function createRangeStats(data: WorkoutData): RangeStats {
  const time = data.time;
  const m = data.metrics;
  const dist = carriedDistance(m.distance);
  const moving = data.movingTime ?? null;
  const gain = data.elevationGain ?? null;
  const p = (v: Float64Array | undefined, skipZero = false): Prefix | null => {
    if (v === undefined) return null;
    const r = prefix(v, skipZero);
    return r.count[v.length] > 0 ? r : null;
  };
  const hr = p(m.heart_rate);
  const power = p(m.power);
  const cadence = p(m.cadence, true);
  const temperature = p(m.temperature);
  // Gain needs a valid altitude sample in the range, else NaN, never 0 (AP-8; Go workout/laps.go).
  const altitude = p(m.altitude);

  const fields: RangeField[] = [];
  if (dist) fields.push('distance');
  fields.push('elapsed');
  if (moving) fields.push('moving');
  if (gain) fields.push('gain');
  if (dist && moving) fields.push('speed');
  if (hr) fields.push('heartRate');
  if (power) fields.push('power');
  if (cadence) fields.push('cadence');
  if (temperature) fields.push('temperature');

  return {
    fields,
    distance: dist,
    query(a, b, out) {
      out.elapsed = time[b] - time[a];
      out.moving = moving ? moving[b] - moving[a] : Number.NaN;
      out.distance = dist ? dist[b] - dist[a] : Number.NaN;
      out.gain = gain && altitude && altitude.count[b + 1] > altitude.count[a] ? gain[b] - gain[a] : Number.NaN;
      out.speed = out.moving > 0 ? out.distance / out.moving : Number.NaN;
      out.heartRate = mean(hr, a, b);
      out.power = mean(power, a, b);
      out.cadence = mean(cadence, a, b);
      out.temperature = mean(temperature, a, b);
      return out;
    },
  };
}

/** Split boundaries over d̄ (F-18 L-4) for splits of `length` metres: boundary 0 is sample 0,
 *  boundary k the first sample with d̄ − d̄[0] ≥ k·length (k·length per k, never accumulated), and
 *  split k is [boundary k−1, boundary k]. A last partial split ends at N−1 when its distance is > 0.
 *  A sample that crosses several multiples ends one split only, so no split is empty. O(N). */
export function splitBounds(d: Float64Array, length: number): number[] {
  const out = [0];
  const n = d.length;
  const d0 = d[0];
  let k = 1;
  // The tolerance absorbs float error of the subtraction; it is far below the 0.1 m quantisation.
  for (let i = 1; i < n; i++) {
    if (d[i] - d0 < k * length - 1e-6) continue;
    out.push(i);
    while (d[i] - d0 >= k * length - 1e-6) k++;
  }
  if (d[n - 1] > d[out[out.length - 1]]) out.push(n - 1);
  return out;
}
