// The charts' x axis (F-17 design R-7, R-8): elapsed time, or distance. Built once per data
// value. The pointer maps to a sample through `keys` and `index` (MetricChart, with lib/cursor's
// nearestIndex: earliest sample on a tie), with no allocation.

export type AxisKind = 'time' | 'distance';

export interface ChartAxis {
  readonly kind: AxisKind;
  /** x of each sample; NaN = no x (a sample without distance: a break, no cursor line). */
  readonly x: Float64Array;
  /** x of each sample for range ends: never NaN (distance carried forward, else the first one). */
  readonly at: Float64Array;
  /** The axis range, the same on every chart (F-05 D-6). */
  readonly min: number;
  readonly max: number;
  /** Strictly increasing x values and the earliest sample with each (null: index = key position). */
  readonly keys: Float64Array;
  readonly index: Uint32Array | null;
}

export function timeAxis(time: Float64Array): ChartAxis {
  const n = time.length;
  return { kind: 'time', x: time, at: time, min: time[0], max: n > 1 ? time[n - 1] : time[0] + 1, keys: time, index: null };
}

/** The distance axis, or null when the distance never advances (no toggle, R-8). Distance is
 *  non-decreasing where present (F-08 R-14); it is not repaired here. */
export function distanceAxis(distance: Float64Array): ChartAxis | null {
  const n = distance.length;
  let distinct = 0;
  let last = Number.NaN;
  let first = Number.NaN;
  for (let i = 0; i < n; i++) {
    const v = distance[i];
    if (v !== v) continue;
    if (first !== first) first = v;
    if (v !== last) distinct++;
    last = v;
  }
  if (distinct < 2) return null;
  const keys = new Float64Array(distinct);
  const index = new Uint32Array(distinct);
  const at = new Float64Array(n);
  let k = -1;
  let carried = first;
  for (let i = 0; i < n; i++) {
    const v = distance[i];
    if (v === v) {
      if (k < 0 || v !== keys[k]) {
        keys[++k] = v;
        index[k] = i;
      }
      carried = v;
    }
    at[i] = carried;
  }
  return { kind: 'distance', x: distance, at, min: first, max: last, keys, index };
}
