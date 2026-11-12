// Metadata of a data file: F-03 §5 and §6 rule 4, F-04 design "Metadata decoding".
// Closed model: only the keys of §5 are kept, unknown keys at every level are
// dropped. Every listed key is required, also inside nested objects (stricter
// than Go `Decode`, which zero-fills; F-03 §6 rule 4 v1.3).
//
// Error messages name the key or the element index, never a value from the file.

import type { DataFileError, DataFileErrorCode, Device, Lap, Session, Summary, SummaryKey, TimeZone, Totals, TotalKey, WorkoutMeta } from './model';

/** A stream descriptor as written in the metadata (F-03 §5.5); consumed by the decoder. */
export interface Descriptor {
  readonly name: string;
  readonly type: string;
  readonly encoding: string;
  readonly scale: number;
  readonly unit: string;
  readonly byte_offset: number;
}

export type MetaResult =
  | { readonly ok: true; readonly meta: WorkoutMeta; readonly streams: readonly Descriptor[] }
  | { readonly ok: false; readonly error: DataFileError };

export function failure(
  code: DataFileErrorCode,
  rule: DataFileError['rule'],
  message: string,
): { readonly ok: false; readonly error: DataFileError } {
  const error: DataFileError = rule === undefined ? { code, message } : { code, rule, message };
  return { ok: false, error };
}

const TOTAL_KEYS: readonly TotalKey[] = [
  'elapsed_time',
  'timer_time',
  'distance',
  'ascent',
  'descent',
  'calories',
  'avg_speed',
  'max_speed',
  'avg_heart_rate',
  'max_heart_rate',
  'avg_cadence',
  'max_cadence',
  'avg_power',
  'max_power',
  'avg_temperature',
  'max_temperature',
];
const SUMMARY_KEYS: readonly SummaryKey[] = [...TOTAL_KEYS, 'moving_time', 'pool_length_m', 'pool_length_yd', 'lengths', 'pool_length_recorded_m'];

const INT32_MIN = -2147483648;
const INT32_MAX = 2147483647;

type Json = Record<string, unknown>;

/** Thrown inside this module only, caught by parseMeta. */
class MetaError extends Error {}

function isObject(v: unknown): v is Json {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function field(o: Json, key: string, where: string): unknown {
  if (!Object.hasOwn(o, key) || o[key] === null) throw new MetaError(`${where}${key} missing`);
  return o[key];
}

function str(o: Json, key: string, where: string): string {
  const v = field(o, key, where);
  if (typeof v !== 'string') throw new MetaError(`${where}${key} is not a string`);
  return v;
}

function int(o: Json, key: string, where: string): number {
  const v = field(o, key, where);
  if (typeof v !== 'number' || !Number.isInteger(v)) throw new MetaError(`${where}${key} is not an integer`);
  return v;
}

function int32(o: Json, key: string, where: string): number {
  const v = int(o, key, where);
  if (v < INT32_MIN || v > INT32_MAX) throw new MetaError(`${where}${key} is outside the int32 range`);
  return v;
}

function obj(o: Json, key: string, where: string): Json {
  const v = field(o, key, where);
  if (!isObject(v)) throw new MetaError(`${where}${key} is not an object`);
  return v;
}

function arr(o: Json, key: string, where: string): unknown[] {
  const v = field(o, key, where);
  if (!Array.isArray(v)) throw new MetaError(`${where}${key} is not an array`);
  return v;
}

function totals<K extends string>(o: Json, keys: readonly K[], where: string): Partial<Record<K, number>> {
  const out: Partial<Record<K, number>> = {};
  for (const k of keys) {
    if (!Object.hasOwn(o, k)) continue;
    const v = o[k];
    if (typeof v !== 'number' || !Number.isFinite(v)) throw new MetaError(`${where}${k} is not a number`);
    out[k] = v;
  }
  return out;
}

function element(v: unknown, label: string): Json {
  if (!isObject(v)) throw new MetaError(`${label} is not an object`);
  return v;
}

function session(v: unknown, i: number): Session {
  const w = `session ${i}: `;
  const o = element(v, `session ${i}`);
  return {
    sport: str(o, 'sport', w),
    sub_sport: str(o, 'sub_sport', w),
    start: int32(o, 'start', w),
    end: int32(o, 'end', w),
    start_index: int(o, 'start_index', w),
    end_index: int(o, 'end_index', w),
    totals: totals(obj(o, 'totals', w), TOTAL_KEYS, `${w}totals.`) as Totals,
  };
}

function lap(v: unknown, i: number): Lap {
  const w = `lap ${i}: `;
  const o = element(v, `lap ${i}`);
  return {
    start: int32(o, 'start', w),
    end: int32(o, 'end', w),
    start_index: int(o, 'start_index', w),
    end_index: int(o, 'end_index', w),
    trigger: str(o, 'trigger', w),
    totals: totals(obj(o, 'totals', w), TOTAL_KEYS, `${w}totals.`) as Totals,
  };
}

/** §5.7: the name as plain text, every sensor code a string. */
function device(o: Json): Device {
  const sensors = arr(o, 'sensors', 'device.');
  if (!sensors.every((s) => typeof s === 'string')) throw new MetaError('device.sensors holds a non-string');
  return { name: str(o, 'name', 'device.'), sensors: sensors as string[] };
}

function descriptor(v: unknown, i: number): Descriptor {
  const w = `stream ${i}: `;
  const o = element(v, `stream ${i}`);
  const scale = field(o, 'scale', w);
  if (typeof scale !== 'number') throw new MetaError(`${w}scale is not a number`);
  return {
    name: str(o, 'name', w),
    type: str(o, 'type', w),
    encoding: str(o, 'encoding', w),
    scale,
    unit: str(o, 'unit', w),
    byte_offset: int(o, 'byte_offset', w),
  };
}

// fatal: invalid UTF-8 throws. ignoreBOM: a BOM stays in the string, so JSON.parse rejects it as Go does.
const utf8 = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true });

/** Rule 4: UTF-8, JSON, top-level keys and kinds (in Go's order), nested shapes. */
export function parseMeta(bytes: Uint8Array): MetaResult {
  let text: string;
  try {
    text = utf8.decode(bytes);
  } catch {
    return failure('corrupt', 4, 'data file: metadata is not valid UTF-8');
  }
  let top: unknown;
  try {
    top = JSON.parse(text);
  } catch {
    return failure('corrupt', 4, 'data file: metadata is not valid JSON');
  }
  if (!isObject(top)) return failure('corrupt', 4, 'data file: metadata is not a JSON object');

  const kinds: readonly [string, 'string' | 'object' | 'array'][] = [
    ['sport', 'string'],
    ['start_time', 'string'],
    ['time_zone', 'object'],
    ['summary', 'object'],
    ['device', 'object'],
    ['sessions', 'array'],
    ['laps', 'array'],
    ['streams', 'array'],
  ];
  for (const [key, kind] of kinds) {
    if (!Object.hasOwn(top, key)) return failure('corrupt', 4, `data file: metadata key ${key} missing`);
    const v = top[key];
    const actual = typeof v === 'string' ? 'string' : Array.isArray(v) ? 'array' : isObject(v) ? 'object' : 'other';
    if (actual !== kind) return failure('corrupt', 4, `data file: metadata key ${key} is not a JSON ${kind}`);
  }

  try {
    const tz = top.time_zone as Json;
    const known = field(tz, 'known', 'time_zone.');
    if (typeof known !== 'boolean') throw new MetaError('time_zone.known is not a boolean');
    const time_zone: TimeZone = { known, offset_seconds: int32(tz, 'offset_seconds', 'time_zone.') };
    const meta: WorkoutMeta = {
      sport: top.sport as string,
      start_time: top.start_time as string,
      time_zone,
      summary: totals(top.summary as Json, SUMMARY_KEYS, 'summary.') as Summary,
      device: device(top.device as Json),
      sessions: (top.sessions as unknown[]).map(session),
      laps: (top.laps as unknown[]).map(lap),
    };
    const streams = (top.streams as unknown[]).map(descriptor);
    return { ok: true, meta, streams };
  } catch (e) {
    if (e instanceof MetaError) return failure('corrupt', 4, `data file: metadata ${e.message}`);
    throw e;
  }
}
