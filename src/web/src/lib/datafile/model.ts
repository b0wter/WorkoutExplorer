// Types of the decoded workout data (F-04 design, "Output model"; ADR-007).
// Wire names are kept verbatim (D-7): one name per concept across Go,
// TypeScript, JSON and the architecture documents.

/** A metric stream: every stream of F-03 §2 except time and position. */
export type MetricName =
  | 'altitude'
  | 'distance'
  | 'speed'
  | 'heart_rate'
  | 'cadence'
  | 'power'
  | 'temperature';

/** The cumulative streams of format version 2 (ADR-029), for ranges (F-17). */
export type CumulativeName = 'moving_time' | 'elevation_gain';

/** Decoded `data.bin`. Every array has length `sampleCount` and is index-aligned with `time`. */
export interface WorkoutData {
  /** N from the header, at least 1. */
  readonly sampleCount: number;
  /** `time_offset` in seconds; strictly increasing; never NaN. */
  readonly time: Float64Array;
  /** Ascending indices i, 1 ≤ i < N, with time[i] − time[i−1] ≥ GAP_SECONDS. */
  readonly gaps: Uint32Array;
  /** Present iff the file has latitude and longitude columns (AP-3). */
  readonly position?: Position;
  /** Present streams only, keys in METRIC_ORDER (AP-4). Values in the F-03 §2 unit, NaN = missing sample. */
  readonly metrics: Readonly<Partial<Record<MetricName, Float64Array>>>;
  /**
   * Cumulative moving time (s) and elevation gain (m) per sample (ADR-029): non-decreasing, never NaN.
   * Present iff the file has the stream (no moving-time source, or no altitude, means absent).
   * Not a chart metric; a range's value is the difference of two entries (F-17 R-3).
   */
  readonly movingTime?: Float64Array;
  readonly elevationGain?: Float64Array;
  readonly meta: WorkoutMeta;
}

export interface Position {
  /** Degrees; NaN exactly where longitude is NaN. */
  readonly latitude: Float64Array;
  readonly longitude: Float64Array;
}

export type TotalKey =
  | 'elapsed_time'
  | 'timer_time'
  | 'distance'
  | 'ascent'
  | 'descent'
  | 'calories'
  | 'avg_speed'
  | 'max_speed'
  | 'avg_heart_rate'
  | 'max_heart_rate'
  | 'avg_cadence'
  | 'max_cadence'
  | 'avg_power'
  | 'max_power'
  | 'avg_temperature'
  | 'max_temperature';

/** Device totals; an absent key means "not recorded" (AP-2), never 0. */
export type Totals = Readonly<Partial<Record<TotalKey, number>>>;
/** Summary keys beyond the totals: moving time, (version 3, F-44) the pool values in whole units and (version 4, ADR-046) the recorded pool length in metres, never printed. */
export type SummaryKey = TotalKey | 'moving_time' | 'pool_length_m' | 'pool_length_yd' | 'lengths' | 'pool_length_recorded_m';
/** Version 5 (ADR-048 item 4): `stroke`, the one stroke code of the whole swim, is the summary's only string. */
export type Summary = Readonly<Partial<Record<SummaryKey, number>>> & { readonly stroke?: string };

/** The device line's data (F-03 §5.7, F-43): display name ('' = none) and sensor codes in their fixed order. */
export interface Device {
  readonly name: string;
  readonly sensors: readonly string[];
}

export interface TimeZone {
  readonly known: boolean;
  readonly offset_seconds: number;
}

export interface Session {
  readonly sport: string;
  readonly sub_sport: string;
  readonly start: number;
  readonly end: number;
  readonly start_index: number;
  readonly end_index: number;
  readonly totals: Totals;
}

export interface Lap {
  readonly start: number;
  readonly end: number;
  readonly start_index: number;
  readonly end_index: number;
  /** Version 4 (ADR-046): button, time, distance, position, stop, equipment; '' or any other string = unknown. */
  readonly trigger: string;
  /** Version 5 (ADR-048 item 2): freestyle, backstroke, breaststroke, butterfly, drill, mixed, im; '' or any other string = unknown. */
  readonly stroke: string;
  readonly totals: Totals;
}

/** A pool length (F-03 §5.8, version 5). The optional values are absent when not recorded or derivable. */
export interface Length {
  readonly start_index: number;
  readonly end_index: number;
  readonly rest: boolean;
  /** As the lap's stroke; '' on every rest length. */
  readonly stroke: string;
  /** Recorded timer time, s. */
  readonly time?: number;
  /** m/s, server-derived (F-48 PL-3); never on a rest length. */
  readonly avg_speed?: number;
  /** Stroke count, at least 1; never on a rest length. */
  readonly strokes?: number;
}

/** F-49 kinds with zones (F-03 §5.9). */
export type ZoneKind = 'heart_rate' | 'power';
/** Version 6 (ADR-049): the file's zones (`from`: the lowest value of zones 2 … k) or a basis for
 *  lib/zones' default model; exactly one of the two. */
export type ZoneEntry = { readonly from: readonly number[]; readonly basis?: undefined } | { readonly basis: number; readonly from?: undefined };
export type Zones = Readonly<Partial<Record<ZoneKind, ZoneEntry>>>;

export interface WorkoutMeta {
  readonly sport: string;
  /** RFC 3339 UTC, whole seconds (F-03 §5.1). */
  readonly start_time: string;
  readonly time_zone: TimeZone;
  readonly summary: Summary;
  readonly device: Device;
  readonly sessions: readonly Session[];
  readonly laps: readonly Lap[];
  /** Version 5: pool lengths in recorded order, [] for most workouts. */
  readonly lengths: readonly Length[];
  /** Version 6: a kind only with its stream present, {} for most workouts. */
  readonly zones: Zones;
}

export type DataFileErrorCode =
  | 'not-a-data-file' // rule 1
  | 'unsupported-version' // rule 2
  | 'corrupt' // rules 3–7
  | 'unsupported-platform' // big-endian
  | 'network' // loadDataFile: request rejected or body unreadable
  | 'http-status' // loadDataFile: response not ok
  | 'internal'; // unexpected exception: a bug, never expected

export interface DataFileError {
  readonly code: DataFileErrorCode;
  /** F-03 §6 rule, for decode errors. */
  readonly rule?: 1 | 2 | 3 | 4 | 5 | 6 | 7;
  /** HTTP status, for http-status. */
  readonly status?: number;
  /** English, for developers and the console. Starts with "data file: ". Never holds sample or metadata values. */
  readonly message: string;
}

export type DataFileResult =
  | { readonly ok: true; readonly data: WorkoutData }
  | { readonly ok: false; readonly error: DataFileError };
