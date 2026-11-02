// Start, finish and lap markers (F-06 design, "Route and markers"; AC 3, AC 6).

import type { WorkoutData } from '../datafile';
import type { Route } from './route';

export interface RouteMarker {
  readonly kind: 'start' | 'finish' | 'lap';
  /** Sample index. */
  readonly index: number;
  readonly lon: number;
  readonly lat: number;
  /** 1-based lap number, kind 'lap' only. */
  readonly lap?: number;
}

/** Start at the first positioned sample, then lap markers, then finish at the last positioned
 *  sample. Lap marker k (k ≥ 2, metadata order across all sessions) sits at lap k's start_index,
 *  only if that sample exists and has a position: never a substitute point. Lap 1 begins at the
 *  start marker. */
export function routeMarkers(data: WorkoutData, route: Route): RouteMarker[] {
  const position = data.position;
  if (!position) return [];
  const lat = position.latitude;
  const lon = position.longitude;
  const n = data.sampleCount;
  const first = route.indices[0];
  const last = route.indices[route.indices.length - 1];
  const out: RouteMarker[] = [{ kind: 'start', index: first, lon: lon[first], lat: lat[first] }];
  const laps = data.meta.laps;
  for (let k = 1; k < laps.length; k++) {
    const s = laps[k].start_index;
    if (s >= 0 && s < n && lat[s] === lat[s]) out.push({ kind: 'lap', index: s, lon: lon[s], lat: lat[s], lap: k + 1 });
  }
  out.push({ kind: 'finish', index: last, lon: lon[last], lat: lat[last] });
  return out;
}
