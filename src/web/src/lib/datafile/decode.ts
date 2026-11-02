// decodeDataFile: F-03 §6 rules 1–7 in Go `Decode`'s order, typed-array views
// on the file's ArrayBuffer, one conversion pass per column into a Float64Array
// with NaN for missing samples, time gaps. F-04 design "Decoding", "Validation"; ADR-007.
//
// Pure and synchronous; never throws (D-5). Hot loops have no per-sample
// function calls and no Array intermediates.

import { GAP_SECONDS, HEADER_SIZE, MAGIC, METRIC_ORDER, STREAM_SPECS, VERSION, align4, type StreamSpec } from './format';
import { failure, parseMeta, type Descriptor } from './meta';
import type { DataFileError, DataFileResult, MetricName, Position, WorkoutData, WorkoutMeta } from './model';

// Typed arrays use the platform's byte order; the format is little-endian (F-03 §4).
const LITTLE_ENDIAN = new Uint8Array(new Uint16Array([1]).buffer)[0] === 1;

const INT32_SENTINEL = 2147483647;

interface Column {
  readonly spec: StreamSpec;
  readonly offset: number;
}

type Failure = { readonly ok: false; readonly error: DataFileError };

export function decodeDataFile(buffer: ArrayBuffer): DataFileResult {
  try {
    return decode(buffer);
  } catch (e) {
    return failure('internal', undefined, `data file: internal error: ${e instanceof Error ? e.message : String(e)}`);
  }
}

function decode(buffer: ArrayBuffer): DataFileResult {
  if (!LITTLE_ENDIAN) return failure('unsupported-platform', undefined, 'data file: big-endian platform not supported');
  const length = buffer.byteLength;

  // Rule 1.
  if (length < HEADER_SIZE) return failure('not-a-data-file', 1, `data file: ${length} bytes, shorter than the header`);
  const view = new DataView(buffer);
  for (let i = 0; i < 4; i++) {
    if (view.getUint8(i) !== MAGIC.charCodeAt(i)) return failure('not-a-data-file', 1, 'data file: bad magic');
  }
  // Rule 2.
  const version = view.getUint16(4, true);
  if (version !== VERSION) return failure('unsupported-version', 2, `data file: unsupported version ${version}`);
  // Rule 3.
  const n = view.getUint32(6, true);
  const l = view.getUint32(10, true);
  if (n === 0) return failure('corrupt', 3, 'data file: no samples');
  if (HEADER_SIZE + l > length) return failure('corrupt', 3, `data file: metadata length ${l} beyond the end of the file`);

  // Rule 4.
  const parsed = parseMeta(new Uint8Array(buffer, HEADER_SIZE, l));
  if (!parsed.ok) return parsed;

  // Rules 5 and 6, for all columns before any view is created.
  const layout = checkLayout(parsed.streams, n, l, length);
  if (!layout.ok) return layout;

  // Columns, with rule 7 checks inside the loops.
  let time: Float64Array = new Float64Array(0); // replaced: time_offset is always the first column (rule 5)
  let gaps: Uint32Array = new Uint32Array(0);
  let latitude: Float64Array | undefined;
  let longitude: Float64Array | undefined;
  const byName: Partial<Record<MetricName, Float64Array>> = {};
  let movingTime: Float64Array | undefined;
  let elevationGain: Float64Array | undefined;
  for (const { spec, offset } of layout.columns) {
    if (spec.name === 'time_offset') {
      const t = decodeTime(buffer, offset, n);
      if (!t.ok) return t;
      ({ time, gaps } = t);
    } else if (spec.name === 'latitude') {
      latitude = decodeDelta(buffer, offset, n, spec.scale);
    } else if (spec.name === 'longitude') {
      longitude = decodeDelta(buffer, offset, n, spec.scale);
      const lat = latitude as Float64Array;
      for (let i = 0; i < n; i++) {
        // x !== x is the NaN test without a function call.
        if ((lat[i] !== lat[i]) !== (longitude[i] !== longitude[i])) {
          return failure('corrupt', 7, `data file: half a position at sample ${i}`);
        }
      }
    } else if (spec.name === 'moving_time' || spec.name === 'elevation_gain') {
      const c = decodeDelta(buffer, offset, n, spec.scale);
      for (let i = 0; i < n; i++) {
        if (c[i] !== c[i]) return failure('corrupt', 7, `data file: ${spec.name} missing at sample ${i}`);
      }
      if (spec.name === 'moving_time') movingTime = c;
      else elevationGain = c;
    } else if (spec.encoding === 'delta') {
      byName[spec.name as MetricName] = decodeDelta(buffer, offset, n, spec.scale);
    } else {
      byName[spec.name as MetricName] = decodeRaw(buffer, offset, n, spec);
    }
  }

  const meta = parsed.meta;
  const ranges = checkRanges(meta, n);
  if (ranges !== undefined) return ranges;

  // Key order of `metrics` is METRIC_ORDER (insertion order), whatever the loop above did.
  const metrics: Partial<Record<MetricName, Float64Array>> = {};
  for (const name of METRIC_ORDER) {
    const values = byName[name];
    if (values !== undefined) metrics[name] = values;
  }
  const position: Position | undefined =
    latitude !== undefined && longitude !== undefined ? { latitude, longitude } : undefined;
  const data: WorkoutData = {
    sampleCount: n,
    time,
    gaps,
    ...(position === undefined ? {} : { position }),
    metrics,
    ...(movingTime === undefined ? {} : { movingTime }),
    ...(elevationGain === undefined ? {} : { elevationGain }),
    meta,
  };
  return { ok: true, data };
}

/** Rules 5 and 6 (F-04 design, Validation steps 5–8). Returns the columns or an error result. */
function checkLayout(
  streams: readonly Descriptor[],
  n: number,
  l: number,
  length: number,
): { readonly ok: true; readonly columns: readonly Column[] } | Failure {
  if (streams.length === 0) return failure('corrupt', 5, 'data file: no streams');
  if (streams[0].name !== 'time_offset') return failure('corrupt', 5, 'data file: first stream is not time_offset');

  const columns: Column[] = [];
  const seen = new Set<string>();
  let order = -1;
  let end = align4(HEADER_SIZE + l);
  for (let i = 0; i < streams.length; i++) {
    const d = streams[i];
    const k = STREAM_SPECS.findIndex((s) => s.name === d.name);
    if (k < 0) return failure('corrupt', 5, `data file: stream ${i} has an unknown name`);
    const spec = STREAM_SPECS[k];
    if (seen.has(spec.name)) return failure('corrupt', 5, `data file: stream ${spec.name} repeated`);
    seen.add(spec.name);
    if (k < order) return failure('corrupt', 5, `data file: stream ${spec.name} out of order`);
    order = k;
    if (d.type !== spec.type || d.encoding !== spec.encoding || d.unit !== spec.unit || d.scale !== spec.scale) {
      return failure('corrupt', 5, `data file: descriptor of ${spec.name} differs from the format`);
    }
    // Rule 6. JS numbers are exact up to 2^53, and n × size ≤ 2^34, so no overflow (F-03 §6 rule 6).
    const off = d.byte_offset;
    if (off % 4 !== 0) return failure('corrupt', 6, `data file: column ${spec.name} is not 4-aligned`);
    if (off < 0 || off > length) return failure('corrupt', 6, `data file: column ${spec.name} out of bounds`);
    if (off < end) {
      return failure('corrupt', 6, `data file: column ${spec.name} at byte ${off} overlaps the metadata or the previous column`);
    }
    if (n * spec.size > length - off) return failure('corrupt', 6, `data file: column ${spec.name} out of bounds`);
    end = off + n * spec.size;
    columns.push({ spec, offset: off });
  }
  if (seen.has('latitude') !== seen.has('longitude')) {
    return failure('corrupt', 5, 'data file: latitude and longitude must both be present or both absent');
  }
  if (end !== length) return failure('corrupt', 6, `data file: ${length - end} trailing bytes after the last column`);
  return { ok: true, columns };
}

/** time_offset: prefix sum, rule 7 (never missing, strictly increasing) and gaps in one pass. */
function decodeTime(
  buffer: ArrayBuffer,
  offset: number,
  n: number,
): { readonly ok: true; readonly time: Float64Array; readonly gaps: Uint32Array } | Failure {
  const src = new Int32Array(buffer, offset, n);
  const time = new Float64Array(n);
  let gaps = new Uint32Array(16);
  let count = 0;
  let acc = 0;
  let prev = 0;
  for (let i = 0; i < n; i++) {
    const d = src[i];
    if (d === INT32_SENTINEL) return failure('corrupt', 7, `data file: time_offset missing at sample ${i}`);
    acc += d;
    if (i > 0) {
      if (acc <= prev) return failure('corrupt', 7, `data file: time_offset not strictly increasing at sample ${i}`);
      if (acc - prev >= GAP_SECONDS) {
        if (count === gaps.length) {
          const grown = new Uint32Array(count * 2);
          grown.set(gaps);
          gaps = grown;
        }
        gaps[count++] = i;
      }
    }
    time[i] = acc; // scale 1
    prev = acc;
  }
  return { ok: true, time, gaps: gaps.slice(0, count) };
}

/** Delta column (F-03 §3 reader): the sum is a plain number, never int32 (ADR-007 item 2); value = acc × scale. */
function decodeDelta(buffer: ArrayBuffer, offset: number, n: number, scale: number): Float64Array {
  const src = new Int32Array(buffer, offset, n);
  const out = new Float64Array(n);
  let acc = 0;
  for (let i = 0; i < n; i++) {
    const d = src[i];
    if (d === INT32_SENTINEL) {
      out[i] = NaN;
    } else {
      acc += d;
      out[i] = acc * scale;
    }
  }
  return out;
}

/** Raw column: view on the file, sentinel → NaN, otherwise stored × scale. */
function decodeRaw(buffer: ArrayBuffer, offset: number, n: number, spec: StreamSpec): Float64Array {
  const src =
    spec.type === 'uint16'
      ? new Uint16Array(buffer, offset, n)
      : spec.type === 'uint8'
        ? new Uint8Array(buffer, offset, n)
        : new Int8Array(buffer, offset, n);
  const out = new Float64Array(n);
  const sentinel = spec.sentinel;
  const scale = spec.scale;
  for (let i = 0; i < n; i++) {
    const v = src[i];
    out[i] = v === sentinel ? NaN : v * scale;
  }
  return out;
}

/** Rule 7 for sessions, then laps: end ≥ start and 0 ≤ start_index ≤ end_index ≤ N. */
function checkRanges(meta: WorkoutMeta, n: number): Failure | undefined {
  const groups: readonly [string, readonly { start: number; end: number; start_index: number; end_index: number }[]][] = [
    ['session', meta.sessions],
    ['lap', meta.laps],
  ];
  for (const [kind, items] of groups) {
    for (let i = 0; i < items.length; i++) {
      const r = items[i];
      if (r.end < r.start) return failure('corrupt', 7, `data file: ${kind} ${i}: end before start`);
      if (r.start_index < 0 || r.start_index > r.end_index || r.end_index > n) {
        return failure('corrupt', 7, `data file: ${kind} ${i}: index range outside the samples`);
      }
    }
  }
  return undefined;
}
