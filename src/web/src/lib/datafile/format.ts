// Data file format version 4 as constants: F-03 design, normative format §1–§2.
// Any change here is a format change (F-03 §9) and goes together with the Go
// writer and the golden fixtures.

import type { CumulativeName, MetricName } from './model';

export const MAGIC = 'WEXD';
export const VERSION = 4;
/** Magic (4) + version (2) + sample count (4) + metadata length (4). */
export const HEADER_SIZE = 14;
/** Two consecutive samples this far apart or more form a gap (F-02 R-3, F-03 §7). */
export const GAP_SECONDS = 5;

export type StreamName = 'time_offset' | 'latitude' | 'longitude' | MetricName | CumulativeName;
export type ColumnType = 'int32' | 'uint16' | 'uint8' | 'int8';
export type Encoding = 'delta' | 'raw';

export interface StreamSpec {
  readonly name: StreamName;
  readonly type: ColumnType;
  readonly encoding: Encoding;
  /** Multiplier from stored integer to unit (§2 literal). */
  readonly scale: number;
  readonly unit: string;
  /** Bytes per sample. */
  readonly size: 1 | 2 | 4;
  /** Stored value that marks a missing sample (the type's maximum). */
  readonly sentinel: number;
}

/** F-03 §2, in column order. */
export const STREAM_SPECS: readonly StreamSpec[] = [
  { name: 'time_offset', type: 'int32', encoding: 'delta', scale: 1, unit: 's', size: 4, sentinel: 2147483647 },
  { name: 'latitude', type: 'int32', encoding: 'delta', scale: 1e-6, unit: 'deg', size: 4, sentinel: 2147483647 },
  { name: 'longitude', type: 'int32', encoding: 'delta', scale: 1e-6, unit: 'deg', size: 4, sentinel: 2147483647 },
  { name: 'altitude', type: 'int32', encoding: 'delta', scale: 0.1, unit: 'm', size: 4, sentinel: 2147483647 },
  { name: 'distance', type: 'int32', encoding: 'delta', scale: 0.1, unit: 'm', size: 4, sentinel: 2147483647 },
  { name: 'speed', type: 'uint16', encoding: 'raw', scale: 0.001, unit: 'm/s', size: 2, sentinel: 65535 },
  { name: 'heart_rate', type: 'uint8', encoding: 'raw', scale: 1, unit: 'bpm', size: 1, sentinel: 255 },
  { name: 'cadence', type: 'uint8', encoding: 'raw', scale: 1, unit: '1/min', size: 1, sentinel: 255 },
  { name: 'power', type: 'uint16', encoding: 'raw', scale: 1, unit: 'W', size: 2, sentinel: 65535 },
  { name: 'temperature', type: 'int8', encoding: 'raw', scale: 1, unit: 'degC', size: 1, sentinel: 127 },
  // Version 2 (ADR-029): cumulative moving time and gain, never charted, never missing.
  { name: 'moving_time', type: 'int32', encoding: 'delta', scale: 1, unit: 's', size: 4, sentinel: 2147483647 },
  { name: 'elevation_gain', type: 'int32', encoding: 'delta', scale: 0.1, unit: 'm', size: 4, sentinel: 2147483647 },
];

/** Metric streams in §2 order: the key order of `WorkoutData.metrics`. */
export const METRIC_ORDER: readonly MetricName[] = [
  'altitude',
  'distance',
  'speed',
  'heart_rate',
  'cadence',
  'power',
  'temperature',
];

/** §2 unit of each metric stream, as written in the descriptors. */
export const METRIC_UNITS: Readonly<Record<MetricName, string>> = {
  altitude: 'm',
  distance: 'm',
  speed: 'm/s',
  heart_rate: 'bpm',
  cadence: '1/min',
  power: 'W',
  temperature: 'degC',
};

/** Smallest multiple of 4 that is ≥ x. Exact for any x below 2^53 (no bit operations). */
export function align4(x: number): number {
  return Math.ceil(x / 4) * 4;
}
