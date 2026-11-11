// Test-only builder of data files from STORED integers (F-04 design, Tests).
// Never imported by app code. It does no quantisation (tests pass integers), so
// it cannot share a rounding bug with the writer; the golden fixtures and the
// hand-written F-03 §8 bytes are the independent checks.
//
// Layout as the writer's (F-03 §4): header, metadata JSON, zero padding,
// columns at align4. Overrides let a test break exactly one thing.

import { HEADER_SIZE, STREAM_SPECS, VERSION, align4, type StreamName } from '../format';
import type { CumulativeName, MetricName } from '../model';

/** A stored absolute integer, or null for a missing sample. */
export type Stored = number | null;

export interface DescriptorJson {
  name: string;
  type: string;
  encoding: string;
  scale: number;
  unit: string;
  byte_offset: number;
}

/** Top-level metadata keys other than `streams` (F-03 §5.1); `unknown` so tests can break them. */
export interface MetaFields {
  sport: unknown;
  start_time: unknown;
  time_zone: unknown;
  summary: unknown;
  device: unknown;
  sessions: unknown;
  laps: unknown;
}

export interface MetaJson extends MetaFields {
  streams: DescriptorJson[];
}

export interface BuildOptions {
  /** time_offset, stored absolute integers. Its length is N unless `sampleCount` is given. */
  time: readonly Stored[];
  position?: { latitude: readonly Stored[]; longitude: readonly Stored[] };
  metrics?: Partial<Record<MetricName | CumulativeName, readonly Stored[]>>;
  /** Metadata fields other than `streams`; defaults are a valid cycling workout. */
  meta?: Partial<MetaFields>;
  /** Changes the metadata object (with correct descriptors) before it is serialised. */
  patch?: (meta: MetaJson) => unknown;
  /** Replaces the metadata bytes entirely (offsets in it are the test's business). */
  rawMetadata?: string | Uint8Array;
  /** Descriptor byte_offset overrides; columns are still written at the correct positions. */
  offsets?: Partial<Record<StreamName, number>>;
  /** Header overrides. */
  sampleCount?: number;
  version?: number;
  magic?: string;
  /** Bytes appended after the last column. */
  trailing?: number;
}

const DEFAULT_META: MetaFields = {
  sport: 'cycling',
  start_time: '2026-06-06T05:00:00Z',
  time_zone: { known: true, offset_seconds: 7200 },
  summary: {},
  device: { name: '', sensors: [] },
  sessions: [],
  laps: [],
};

const encoder = new TextEncoder();

/** Delta-encodes stored absolute integers per F-03 §3 (missing = sentinel, skipped by the sum). */
export function deltas(values: readonly Stored[]): number[] {
  const out: number[] = [];
  let prev = 0;
  for (const v of values) {
    if (v === null) {
      out.push(2147483647);
    } else {
      out.push(v - prev);
      prev = v;
    }
  }
  return out;
}

export function buildDataFile(o: BuildOptions): ArrayBuffer {
  const n = o.time.length;
  const present: { name: StreamName; values: readonly Stored[] }[] = [{ name: 'time_offset', values: o.time }];
  if (o.position) {
    present.push({ name: 'latitude', values: o.position.latitude });
    present.push({ name: 'longitude', values: o.position.longitude });
  }
  for (const spec of STREAM_SPECS) {
    const values = o.metrics?.[spec.name as MetricName | CumulativeName];
    if (values !== undefined) present.push({ name: spec.name, values });
  }
  const specs = present.map((p) => {
    const spec = STREAM_SPECS.find((s) => s.name === p.name);
    if (!spec) throw new Error(`build: unknown stream ${p.name}`);
    if (p.values.length !== n) throw new Error(`build: ${p.name} has ${p.values.length} samples, not ${n}`);
    return spec;
  });

  // Offsets depend on L and L on the offsets: iterate until stable (as F-03 W-8).
  let metaBytes: Uint8Array = new Uint8Array(0);
  let positions: number[] = [];
  for (let pass = 0, l = 0; pass < 10; pass++) {
    positions = [];
    let end = align4(HEADER_SIZE + l);
    for (const spec of specs) {
      positions.push(end);
      end = align4(end + n * spec.size);
    }
    if (o.rawMetadata !== undefined) {
      metaBytes = typeof o.rawMetadata === 'string' ? encoder.encode(o.rawMetadata) : o.rawMetadata;
    } else {
      const meta: MetaJson = {
        ...DEFAULT_META,
        ...o.meta,
        streams: specs.map((s, i) => ({
          name: s.name,
          type: s.type,
          encoding: s.encoding,
          scale: s.scale,
          unit: s.unit,
          byte_offset: o.offsets?.[s.name] ?? positions[i],
        })),
      };
      metaBytes = encoder.encode(JSON.stringify(o.patch ? o.patch(meta) : meta));
    }
    if (metaBytes.length === l) break;
    l = metaBytes.length;
  }

  const last = specs.length - 1;
  const length = positions[last] + n * specs[last].size + (o.trailing ?? 0);
  const buffer = new ArrayBuffer(length);
  const view = new DataView(buffer);
  const magic = o.magic ?? 'WEXD';
  for (let i = 0; i < 4; i++) view.setUint8(i, magic.charCodeAt(i));
  view.setUint16(4, o.version ?? VERSION, true);
  view.setUint32(6, o.sampleCount ?? n, true);
  view.setUint32(10, metaBytes.length, true);
  new Uint8Array(buffer, HEADER_SIZE, metaBytes.length).set(metaBytes);

  specs.forEach((spec, k) => {
    const values = present[k].values;
    const off = positions[k];
    const stored = spec.encoding === 'delta' ? deltas(values) : values.map((v) => (v === null ? spec.sentinel : v));
    for (let i = 0; i < n; i++) {
      const v = stored[i];
      const at = off + i * spec.size;
      const fits =
        spec.type === 'int32'
          ? v >= -2147483648 && v <= 2147483647
          : spec.type === 'uint16'
            ? v >= 0 && v <= 65535
            : spec.type === 'uint8'
              ? v >= 0 && v <= 255
              : v >= -128 && v <= 127;
      if (!Number.isInteger(v) || !fits) throw new Error(`build: ${spec.name}[${i}] = ${v} does not fit ${spec.type}`);
      if (spec.type === 'int32') view.setInt32(at, v, true);
      else if (spec.type === 'uint16') view.setUint16(at, v, true);
      else if (spec.type === 'uint8') view.setUint8(at, v);
      else view.setInt8(at, v);
    }
  });
  return buffer;
}
