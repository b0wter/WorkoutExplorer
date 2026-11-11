// Public surface of lib/datafile (F-04 design, "Module boundary"). App code
// imports this folder only through this file, never a file inside it.

export { decodeDataFile } from './decode';
export { loadDataFile } from './load';
export { GAP_SECONDS, METRIC_ORDER, METRIC_UNITS } from './format';
export type {
  DataFileError,
  DataFileErrorCode,
  DataFileResult,
  Device,
  Lap,
  MetricName,
  Position,
  Session,
  Summary,
  SummaryKey,
  TimeZone,
  TotalKey,
  Totals,
  WorkoutData,
  WorkoutMeta,
} from './model';
