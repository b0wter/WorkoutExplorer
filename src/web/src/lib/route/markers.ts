// Start, finish and lap markers (F-06 design, "Route and markers"; AC 3, AC 6).

import type { WorkoutData } from '../datafile';
import type { Route } from './route';

export interface RouteMarker {
  readonly kind: 'start' | 'finish' | 'lap' | 'leg';
  /** Sample index. */
  readonly index: number;
  readonly lon: number;
  readonly lat: number;
  /** 1-based lap number, kind 'lap' only. */
  readonly lap?: number;
  /** The leg's name, kind 'leg' only (F-42 MS-8). */
  readonly name?: string;
}

/** Start at the first positioned sample, then lap markers, then finish at the last positioned
 *  sample. Lap marker k (k ≥ 2, metadata order across all sessions) sits at lap k's start_index,
 *  only if that sample exists and has a position: never a substitute point. Lap 1 begins at the
 *  start marker. A leg start (F-42 MS-8, `legs`: the caller's section starts after the first) gets
 *  a leg marker under the same rule, which replaces a lap marker at that sample. */
export function routeMarkers(data: WorkoutData, route: Route, legs: readonly { readonly name: string; readonly start: number }[] = []): RouteMarker[] {
  const position = data.position;
  if (!position) return [];
  const lat = position.latitude;
  const lon = position.longitude;
  const n = data.sampleCount;
  const first = route.indices[0];
  const last = route.indices[route.indices.length - 1];
  const out: RouteMarker[] = [{ kind: 'start', index: first, lon: lon[first], lat: lat[first] }];
  const has = (s: number): boolean => s >= 0 && s < n && lat[s] === lat[s];
  const laps = data.meta.laps;
  for (let k = 1; k < laps.length; k++) {
    const s = laps[k].start_index;
    if (has(s) && !legs.some((g) => g.start === s)) out.push({ kind: 'lap', index: s, lon: lon[s], lat: lat[s], lap: k + 1 });
  }
  for (const { name, start: s } of legs) if (has(s)) out.push({ kind: 'leg', index: s, lon: lon[s], lat: lat[s], name });
  out.push({ kind: 'finish', index: last, lon: lon[last], lat: lat[last] });
  return out;
}
