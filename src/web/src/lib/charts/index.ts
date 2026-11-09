// Public surface of lib/charts (F-05 design, "Module boundary"). Pure
// TypeScript; imports only lib/datafile (its index.ts, types only).

export { CHART_ORDER, CHART_SPECS, chartSpecs, chartValues, presentCharts, readoutFields } from './specs';
export type { ChartMetric, ChartSpec, ReadoutField } from './specs';
export { bucketCount, decimate } from './decimate';
export { distanceAxis, timeAxis } from './axis';
export type { AxisKind, ChartAxis } from './axis';
export type { Decimated } from './decimate';
export { MISSING_TEXT, formatDistance, formatElapsed, formatFixed } from '../format';
export { IDLE, LONG_PRESS_MS, MOUSE_SLOP, TOUCH_SLOP, blocksScroll, holdsTouch, step } from './gesture';
export type {
  GestureAction,
  GestureCursor,
  GestureInput,
  GestureInputKind,
  GestureState,
  GestureStep,
  RangeAction,
  TouchAxis,
} from './gesture';
/** The range hint's state (F-39 HU-3): `plain` as before (edit page), `once` with its ×, `done`
 *  an empty line that keeps its space, `none` no line (the page reserved none). */
export type HintMode = 'plain' | 'once' | 'done' | 'none';
