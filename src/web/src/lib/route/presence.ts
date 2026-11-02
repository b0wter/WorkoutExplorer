// Route presence (F-06 design, "Route presence"; AP-3). The only part of lib/route that
// static code uses (RouteMap decides from it whether the map loads at all), so it
// imports nothing else from lib/route.

import type { WorkoutData } from '../datafile';

/** no-location-data: no position stream (hint, MapLibre never loaded). no-visible-route: the
 *  stream is present but no sample has a position (e.g. entirely hidden; nothing shown, OQ-15).
 *  route: at least one positioned sample. */
export type RoutePresence = 'no-location-data' | 'no-visible-route' | 'route';

/** One pass over latitude; stops at the first value that is not NaN (ADR-007: latitude and
 *  longitude are NaN together, and a decoded value is never ±Infinity). */
export function routePresence(data: WorkoutData): RoutePresence {
  const position = data.position;
  if (!position) return 'no-location-data';
  const lat = position.latitude;
  for (let i = 0; i < lat.length; i++) if (lat[i] === lat[i]) return 'route';
  return 'no-visible-route';
}
