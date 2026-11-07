// Route colouring by metric (F-25 design C-3, C-4, C-6; ADR-037; AP-9): which metrics are offered,
// a value per sample, the scale and its classes, and the legend's texts. Pure: no DOM, no MapLibre,
// no lib/route (the drawn edges come in as their end indices). LAZY: imported by map-view only.

import type { WorkoutData } from '../datafile';
import { STOP_SPEED, formatFixed, formatSpeedOrPace, isPaceSport, speedOrPaceUnit, type Units } from '../format';

/** In AP-9's order. */
export type ColourMetric = 'speed' | 'heart_rate' | 'power' | 'gradient';

/** Classes of the workout-relative metrics (C-4); gradient has GRADIENT_EDGES.length + 1. */
export const CLASS_COUNT = 12;
/** The class of a sample without a value: its edge is left out, so the plain route shows (C-2). */
export const NO_CLASS = 255;

// Starting values, reviewed on screen by the operator (C-3, C-4).
/** Centred mean over this many seconds of time. */
export const SPEED_WINDOW_S = 10;
export const POWER_WINDOW_S = 30;
/** Gradient: centred distance window, and the shortest span (at the ends of the route) that still counts. */
export const GRADIENT_WINDOW_M = 100;
export const GRADIENT_MIN_SPAN_M = 50;
/** Gradient class edges in %: descent, flat in the middle, climb (11 classes, C-4). */
export const GRADIENT_EDGES: readonly number[] = [-12, -8, -5, -3, -1, 1, 3, 5, 8, 12];
/** The gradient edges the legend labels (C-6: at most five, 0 % implied by the flat middle). */
export const GRADIENT_LABELS: readonly number[] = [-12, -5, 5, 12];

/** Metrics with a value at a positioned sample, in AP-9's order; gradient needs altitude and distance (C-3).
 *  Each stream's pass stops at its first such value. */
export function offeredMetrics(data: WorkoutData): ColourMetric[] {
  const lat = data.position?.latitude;
  if (!lat) return [];
  const has = (s: Float64Array | undefined): boolean => {
    if (s) for (let i = 0; i < s.length; i++) if (s[i] === s[i] && lat[i] === lat[i]) return true;
    return false;
  };
  const m = data.metrics;
  const out: ColourMetric[] = [];
  if (has(m.speed)) out.push('speed');
  if (has(m.heart_rate)) out.push('heart_rate');
  if (has(m.power)) out.push('power');
  if (has(m.altitude) && has(m.distance)) out.push('gradient');
  return out;
}

/** The option's text: speed is "Pace" for the pace sports by the workout sport (DS-2, F-13 D-4). */
export function metricLabel(metric: ColourMetric, sport: string): string {
  if (metric === 'speed') return isPaceSport(sport) ? 'Pace' : 'Speed';
  return metric === 'heart_rate' ? 'Heart rate' : metric === 'power' ? 'Power' : 'Gradient';
}

/** A copy of `src` with NaN in every sample of a `transition` session (F-08 hand-off, user answer 6). */
function withoutTransitions(data: WorkoutData, src: Float64Array): Float64Array {
  const out = Float64Array.from(src);
  for (const s of data.meta.sessions) if (s.sport === 'transition') out.fill(Number.NaN, s.start_index, s.end_index);
  return out;
}

/** Centred mean over `window` seconds of time, of present samples only; never across a time gap; a missing
 *  sample stays missing (C-3). O(N): prefix sums and two pointers. */
export function centredMean(time: Float64Array, gaps: Uint32Array, raw: Float64Array, window: number): Float64Array {
  const n = raw.length;
  const half = window / 2;
  const sum = new Float64Array(n + 1);
  const count = new Uint32Array(n + 1);
  for (let i = 0; i < n; i++) {
    const v = raw[i];
    const present = v === v;
    sum[i + 1] = sum[i] + (present ? v : 0);
    count[i + 1] = count[i] + (present ? 1 : 0);
  }
  const out = new Float64Array(n);
  let g = 0;
  let segStart = 0;
  let lo = 0;
  let hi = 0; // window [lo, hi)
  for (let i = 0; i < n; i++) {
    while (g < gaps.length && gaps[g] <= i) segStart = gaps[g++];
    const segEnd = g < gaps.length ? gaps[g] : n;
    if (raw[i] !== raw[i]) {
      out[i] = Number.NaN;
      continue;
    }
    if (lo < segStart) lo = segStart;
    while (time[i] - time[lo] > half) lo++;
    if (hi <= i) hi = i + 1;
    while (hi < segEnd && time[hi] - time[i] <= half) hi++;
    out[i] = (sum[hi] - sum[lo]) / (count[hi] - count[lo]);
  }
  return out;
}

/** Gradient in % per sample (C-3): altitude change over a centred GRADIENT_WINDOW_M distance window, altitude
 *  interpolated in distance between valid samples. Valid: altitude and distance present, distance above the
 *  previous valid one, not in a transition. Others, and windows shorter than GRADIENT_MIN_SPAN_M, are NaN. */
export function gradientValues(data: WorkoutData): Float64Array {
  const n = data.sampleCount;
  const out = new Float64Array(n).fill(Number.NaN);
  const alt = data.metrics.altitude;
  const distance = data.metrics.distance;
  if (!alt || !distance) return out;
  const dist = withoutTransitions(data, distance);
  const D = new Float64Array(n);
  const A = new Float64Array(n);
  const I = new Uint32Array(n);
  let m = 0;
  let last = -Infinity;
  for (let i = 0; i < n; i++) {
    const a = alt[i];
    const d = dist[i];
    if (a === a && d > last) {
      D[m] = d;
      A[m] = a;
      I[m++] = i;
      last = d;
    }
  }
  const half = GRADIENT_WINDOW_M / 2;
  // Altitude at distance x, with D[j] ≤ x ≤ D[j + 1].
  const at = (j: number, x: number): number => A[j] + ((A[j + 1] - A[j]) * (x - D[j])) / (D[j + 1] - D[j]);
  let jl = 0;
  let ju = 0;
  for (let k = 0; k < m; k++) {
    const lo = Math.max(D[k] - half, D[0]);
    const hi = Math.min(D[k] + half, D[m - 1]);
    if (hi - lo < GRADIENT_MIN_SPAN_M) continue;
    while (D[jl + 1] < lo) jl++;
    while (D[ju + 1] < hi) ju++;
    out[I[k]] = ((at(ju, hi) - at(jl, lo)) / (hi - lo)) * 100;
  }
  return out;
}

/** The value per sample that is classed; NaN = no class (C-3). Speed for the pace sports is pace in s/m (the
 *  quantile edges are the same either way, C-4; the legend shows pace). */
export function metricValues(data: WorkoutData, metric: ColourMetric): Float64Array {
  const n = data.sampleCount;
  const s = data.metrics;
  if (metric === 'gradient') return gradientValues(data);
  if (metric === 'heart_rate') return s.heart_rate ?? new Float64Array(n).fill(Number.NaN);
  if (metric === 'power') {
    return s.power ? centredMean(data.time, data.gaps, withoutTransitions(data, s.power), POWER_WINDOW_S) : new Float64Array(n).fill(Number.NaN);
  }
  if (!s.speed) return new Float64Array(n).fill(Number.NaN);
  const raw = withoutTransitions(data, s.speed);
  const pace = isPaceSport(data.meta.sport);
  // DS-2: a stop has no pace; it is missing before the mean, so it never slows its neighbours.
  if (pace) for (let i = 0; i < n; i++) if (raw[i] < STOP_SPEED) raw[i] = Number.NaN;
  const v = centredMean(data.time, data.gaps, raw, SPEED_WINDOW_S);
  if (pace) for (let i = 0; i < n; i++) v[i] = 1 / v[i];
  return v;
}

export interface ColourScale {
  readonly metric: ColourMetric;
  /** Ascending edges in the value's unit, one fewer than the classes: class = number of edges ≤ value.
   *  Relative metrics: NaN when no drawn edge has a value. */
  readonly edges: readonly number[];
  /** Pace: class 0 is the largest value (slow → fast for every sport, C-5). */
  readonly reverse: boolean;
}

/** C-4. `ends`: sample i of every drawn (recorded) edge (i − 1, i); only their values set the edges, the k/12
 *  quantiles, so each class covers about a twelfth of the drawn route. Ties leave classes between them empty. */
export function colourScale(metric: ColourMetric, values: Float64Array, ends: Uint32Array, sport: string): ColourScale {
  const reverse = metric === 'speed' && isPaceSport(sport);
  if (metric === 'gradient') return { metric, edges: GRADIENT_EDGES, reverse };
  const base = new Float64Array(ends.length);
  let n = 0;
  // Speed sports: a stop takes the slowest class but does not move the edges.
  const floor = metric === 'speed' && !reverse ? STOP_SPEED : -Infinity;
  for (let k = 0; k < ends.length; k++) {
    const v = values[ends[k]];
    if (v >= floor) base[n++] = v;
  }
  const sorted = base.subarray(0, n).sort();
  const edges: number[] = [];
  for (let k = 1; k < CLASS_COUNT; k++) edges.push(n > 0 ? sorted[Math.floor((k * n) / CLASS_COUNT)] : Number.NaN);
  return { metric, edges, reverse };
}

/** Class per sample (0 … edges.length), NO_CLASS where the value is NaN. */
export function classify(values: Float64Array, scale: ColourScale): Uint8Array {
  const out = new Uint8Array(values.length);
  const e = scale.edges;
  for (let i = 0; i < values.length; i++) {
    const v = values[i];
    if (v !== v) {
      out[i] = NO_CLASS;
      continue;
    }
    // Reversed (pace): count edges strictly below, so ties land on the same side as on a ride.
    let c = 0;
    while (c < e.length && (scale.reverse ? e[c] < v : e[c] <= v)) c++;
    out[i] = scale.reverse ? e.length - c : c;
  }
  return out;
}

export interface Legend {
  /** Relative scales: [left end, median edge, right end], the ends marked ≤ or ≥; gradient: GRADIENT_LABELS. */
  readonly labels: readonly string[];
  readonly unit: string;
}

/** The legend's texts in the viewer's units (C-6, DS-1, DS-2). Metric ↔ imperial only relabels (C-4). */
export function legend(scale: ColourScale, sport: string, units: Units): Legend {
  const m = scale.metric;
  if (m === 'gradient') return { labels: GRADIENT_LABELS.map((e) => formatFixed(e, 0)), unit: '%' };
  const e = scale.edges;
  const lo = e[0];
  const hi = e[e.length - 1];
  // The end classes hold everything beyond the lowest and highest edge. Pace: the slow (left) end is the larger number.
  const shown = scale.reverse ? [hi, e[e.length >> 1], lo] : [lo, e[e.length >> 1], hi];
  const marks = lo !== lo ? ['', '', ''] : scale.reverse ? ['≥ ', '', '≤ '] : ['≤ ', '', '≥ '];
  let text = (v: number): string => formatFixed(v, 0);
  if (m === 'speed') {
    // A missing edge gives '' from formatSpeedOrPace; show the missing-value dash instead.
    text = (v) => formatSpeedOrPace(sport, scale.reverse ? 1 / v : v, units) || formatFixed(Number.NaN, 0);
  }
  return { labels: shown.map((v, k) => marks[k] + text(v)), unit: m === 'speed' ? speedOrPaceUnit(sport, units) : m === 'heart_rate' ? 'bpm' : 'W' };
}
