// The route: every positioned sample in index order (F-06 design, "Route and markers"; AP-3).
// Samples without a position (GPS dropout, pause without fix, hidden section) are skipped,
// so the line bridges them with one straight segment; it is never split or filled.

import type { WorkoutData } from '../datafile';
import { mercatorX, mercatorY } from './mercator';

export interface Route {
  /** Sample indices with a position, ascending; length M ≥ 1. */
  readonly indices: Uint32Array;
  /** Web Mercator x and y in [0, 1] per positioned sample, same order as indices. */
  readonly mx: Float64Array;
  readonly my: Float64Array;
  /** [west, south, east, north] in degrees over the positioned samples. */
  readonly bounds: readonly [number, number, number, number];
}

/** null unless routePresence(data) === 'route'. Float64 throughout (ADR-007: 10⁻⁶° needs float64). */
export function buildRoute(data: WorkoutData): Route | null {
  const position = data.position;
  if (!position) return null;
  const lat = position.latitude;
  const lon = position.longitude;
  let m = 0;
  for (let i = 0; i < lat.length; i++) if (lat[i] === lat[i]) m++;
  if (m === 0) return null;

  const indices = new Uint32Array(m);
  const mx = new Float64Array(m);
  const my = new Float64Array(m);
  let west = Infinity;
  let south = Infinity;
  let east = -Infinity;
  let north = -Infinity;
  let k = 0;
  for (let i = 0; i < lat.length; i++) {
    const la = lat[i];
    if (la !== la) continue;
    const lo = lon[i];
    indices[k] = i;
    mx[k] = mercatorX(lo);
    my[k] = mercatorY(la);
    k++;
    if (lo < west) west = lo;
    if (lo > east) east = lo;
    if (la < south) south = la;
    if (la > north) north = la;
  }
  return { indices, mx, my, bounds: [west, south, east, north] };
}

/** The route split into recorded segments and bridges (F-06 design v1.4, D-7; user request),
 *  coordinates [longitude, latitude] (GeoJSON order). Every pair of consecutive positioned
 *  samples (indices[k], indices[k+1]) is either an edge of exactly one recorded segment or
 *  exactly one bridge. */
export interface RouteLines {
  /** Maximal runs of positioned samples with no bridge between neighbours; only runs of ≥ 2 points. */
  readonly recorded: [number, number][][];
  /** One 2-point line per bridge. */
  readonly bridges: [[number, number], [number, number]][];
}

/** Consecutive positioned samples a, b are joined by a bridge (drawn dashed) if b − a > 1 (at
 *  least one sample without position between them: a GPS dropout) or b starts a gap of
 *  ≥ GAP_SECONDS (a pause or a recording drop); otherwise they are an edge of a recorded
 *  segment. One walk over indices with a second pointer over data.gaps. A positioned sample
 *  with bridges on both sides belongs to no recorded segment (a one-point line is invalid
 *  GeoJSON); it is just the joint of the two bridges. */
export function routeLines(data: WorkoutData, route: Route): RouteLines {
  return routeLinesIn(data, route, 0, data.sampleCount - 1);
}

/** First k with indices[k] ≥ i (indices.length if none). */
export function lowerBound(indices: Uint32Array, i: number): number {
  let lo = 0;
  let hi = indices.length;
  while (lo < hi) {
    const mid = (lo + hi) >>> 1;
    if (indices[mid] < i) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

/** routeLines over the positioned samples in [a, b] only (F-17 R-10: the range highlight), by the
 *  same rule: a sample without a position (a hidden section, a dropout) never gives a vertex.
 *  O(log M + positioned samples in the range). Empty when fewer than two are positioned. */
export function routeLinesIn(data: WorkoutData, route: Route, a: number, b: number): RouteLines {
  const recorded: [number, number][][] = [];
  const bridges: [[number, number], [number, number]][] = [];
  const position = data.position;
  const indices = route.indices;
  const k0 = lowerBound(indices, a);
  const k1 = lowerBound(indices, b + 1); // exclusive
  if (!position || k1 - k0 < 2) return { recorded, bridges };
  const lat = position.latitude;
  const lon = position.longitude;
  const gaps = data.gaps;
  const point = (i: number): [number, number] => [lon[i], lat[i]];

  let g = lowerBound(gaps, indices[k0]);
  let run: [number, number][] = [point(indices[k0])];
  for (let k = k0 + 1; k < k1; k++) {
    const a = indices[k - 1];
    const b = indices[k];
    while (g < gaps.length && gaps[g] < b) g++;
    if (b - a > 1 || (g < gaps.length && gaps[g] === b)) {
      if (run.length >= 2) recorded.push(run);
      bridges.push([point(a), point(b)]);
      run = [point(b)];
    } else {
      run.push(point(b));
    }
  }
  if (run.length >= 2) recorded.push(run);
  return { recorded, bridges };
}

/** Sample i of every recorded edge (i − 1, i), ascending (F-25 C-2): consecutive positioned samples with no
 *  bridge between them, by routeLinesIn's rule. */
export function recordedEdgeEnds(data: WorkoutData, route: Route): Uint32Array {
  const indices = route.indices;
  const gaps = data.gaps;
  const out = new Uint32Array(indices.length);
  let n = 0;
  let g = 0;
  for (let k = 1; k < indices.length; k++) {
    const b = indices[k];
    while (g < gaps.length && gaps[g] < b) g++;
    if (b - indices[k - 1] === 1 && !(g < gaps.length && gaps[g] === b)) out[n++] = b;
  }
  return out.subarray(0, n);
}

/** The recorded edges by class (F-25 C-2, ADR-037): lines[c] holds the edges (i − 1, i) of `ends` with
 *  classes[i] === c, consecutive edges of one class merged into one line, coordinates [longitude, latitude].
 *  An edge whose class is ≥ count is left out. No metric knowledge. */
export function classLines(data: WorkoutData, ends: Uint32Array, classes: Uint8Array, count: number): [number, number][][][] {
  const lines: [number, number][][][] = [];
  for (let c = 0; c < count; c++) lines.push([]);
  const position = data.position;
  if (!position) return lines;
  const lat = position.latitude;
  const lon = position.longitude;
  let run: [number, number][] | null = null;
  let prevEnd = -1;
  let prevClass = -1;
  for (let k = 0; k < ends.length; k++) {
    const i = ends[k];
    const c = classes[i];
    if (c >= count) {
      run = null;
      continue;
    }
    if (run !== null && c === prevClass && prevEnd === i - 1) {
      run.push([lon[i], lat[i]]);
    } else {
      run = [
        [lon[i - 1], lat[i - 1]],
        [lon[i], lat[i]],
      ];
      lines[c].push(run);
    }
    prevEnd = i;
    prevClass = c;
  }
  return lines;
}
