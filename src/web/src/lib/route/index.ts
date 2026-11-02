// Public surface of lib/route (F-06 design, "Module boundary"). Pure TypeScript: no Svelte,
// no DOM, no MapLibre; imports only lib/datafile (types). App code imports this folder only
// through this file. No file here has top-level side effects, so static code that uses only
// routePresence keeps the rest out of the entry chunk.

export { routePresence } from './presence';
export type { RoutePresence } from './presence';
export { buildRoute, classLines, lowerBound, recordedEdgeEnds, routeLines, routeLinesIn } from './route';
export type { Route, RouteLines } from './route';
export { routeMarkers } from './markers';
export type { RouteMarker } from './markers';
export { MAX_MERCATOR_LAT, mercatorX, mercatorY } from './mercator';
export { createRouteIndex } from './route-index';
export type { RouteIndex } from './route-index';
export {
  HOVER_RADIUS_PX,
  ROUTE_IDLE,
  TIE_PX,
  TOUCH_DRAG_RADIUS_PX,
  TOUCH_START_RADIUS_PX,
  hitRadiusPx,
  interceptsTouch,
  routeStep,
  wantsHit,
} from './gesture';
export type {
  RouteCursor,
  RouteGestureAction,
  RouteGestureInput,
  RouteGestureKind,
  RouteGestureState,
  RouteGestureStep,
} from './gesture';
