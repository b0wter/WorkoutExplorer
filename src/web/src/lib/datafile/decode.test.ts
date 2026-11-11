// decodeDataFile unit tests: F-04 design, Tests (worked example, AC 1, gaps,
// metadata, AC 3 rejects, robustness). Inputs are built from stored integers by
// testing/build-file.ts or written byte by byte.

import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile } from './decode';
import { METRIC_ORDER } from './format';
import type { DataFileErrorCode, DataFileResult, MetricName, WorkoutData } from './model';
import { buildDataFile, type BuildOptions, type MetaJson, type Stored } from './testing/build-file';

function decodeOk(buffer: ArrayBuffer): WorkoutData {
  const r = decodeDataFile(buffer);
  if (!r.ok) throw new Error(`expected ok, got ${r.error.code} (rule ${r.error.rule}): ${r.error.message}`);
  return r.data;
}

function expectReject(buffer: () => ArrayBuffer, code: DataFileErrorCode, rule: number): DataFileResult {
  let r: DataFileResult | undefined;
  expect(() => {
    r = decodeDataFile(buffer());
  }).not.toThrow();
  const result = r as DataFileResult;
  expect(result.ok).toBe(false);
  if (!result.ok) {
    expect({ code: result.error.code, rule: result.error.rule }).toEqual({ code, rule });
    expect(result.error.message.startsWith('data file: ')).toBe(true);
  }
  return result;
}

const T5 = [0, 1, 2, 10, 11];
const nanArr = (a: Float64Array) => Array.from(a, (v) => (Number.isNaN(v) ? null : v));

/** Metadata length, bytes and descriptors of a built file, read by the test. */
function layoutOf(buffer: ArrayBuffer): { l: number; bytes: Uint8Array; streams: MetaJson['streams'] } {
  const l = new DataView(buffer).getUint32(10, true);
  const bytes = new Uint8Array(buffer.slice(14, 14 + l));
  return { l, bytes, streams: (JSON.parse(new TextDecoder().decode(bytes)) as MetaJson).streams };
}

function build(o: Partial<BuildOptions> = {}): ArrayBuffer {
  return buildDataFile({ time: T5, ...o });
}

// ---------------------------------------------------------------------------

describe('F-03 §8 worked example, written byte by byte', () => {
  function workedExample(): { buffer: ArrayBuffer; offsets: number[] } {
    const enc = new TextEncoder();
    const sizes = [20, 20, 20, 5, 5];
    const names = ['time_offset', 'latitude', 'longitude', 'heart_rate', 'temperature'];
    const desc = [
      ['int32', 'delta', '1', 's'],
      ['int32', 'delta', '1e-06', 'deg'],
      ['int32', 'delta', '1e-06', 'deg'],
      ['uint8', 'raw', '1', 'bpm'],
      ['int8', 'raw', '1', 'degC'],
    ];
    let json = '';
    let offsets: number[] = [];
    for (let l = 0, pass = 0; pass < 5; pass++) {
      offsets = [];
      let end = Math.ceil((14 + l) / 4) * 4;
      for (const s of sizes) {
        offsets.push(end);
        end = Math.ceil((end + s) / 4) * 4;
      }
      const streams = names
        .map(
          (n, i) =>
            `{"name":"${n}","type":"${desc[i][0]}","encoding":"${desc[i][1]}","scale":${desc[i][2]},"unit":"${desc[i][3]}","byte_offset":${offsets[i]}}`,
        )
        .join(',');
      json =
        '{"sport":"cycling","start_time":"2026-06-06T05:00:00Z","time_zone":{"known":true,"offset_seconds":7200},' +
        `"summary":{},"device":{"name":"","sensors":[]},"sessions":[],"laps":[],"streams":[${streams}]}`;
      if (enc.encode(json).length === l) break;
      l = enc.encode(json).length;
    }
    const meta = enc.encode(json);
    const length = offsets[4] + 5;
    const b = new Uint8Array(length);
    b.set([0x57, 0x45, 0x58, 0x44, 0x03, 0x00, 0x05, 0x00, 0x00, 0x00], 0);
    new DataView(b.buffer).setUint32(10, meta.length, true);
    b.set(meta, 14);
    const int32le = (vals: number[]) => new Uint8Array(new Int32Array(vals).buffer);
    b.set(int32le([0, 1, 1, 8, 1]), offsets[0]);
    b.set(int32le([47123456, 2, 2147483647, 542, 1]), offsets[1]);
    b.set(int32le([8500000, 1, 2147483647, 49, 1]), offsets[2]);
    b.set([0x78, 0x79, 0xff, 0x83, 0x84], offsets[3]);
    b.set([0xfe, 0xfd, 0xfd, 0x7f, 0xfd], offsets[4]);
    return { buffer: b.buffer, offsets };
  }

  it('decodes to the listed values', () => {
    const d = decodeOk(workedExample().buffer);
    const close = (actual: Float64Array, expected: (number | null)[]) => {
      expect(actual.length).toBe(expected.length);
      expected.forEach((e, i) => {
        if (e === null) expect(actual[i]).toBeNaN();
        else expect(Math.abs(actual[i] - e)).toBeLessThan(1e-9);
      });
    };
    expect(d.sampleCount).toBe(5);
    expect(Array.from(d.time)).toEqual([0, 1, 2, 10, 11]);
    expect(Array.from(d.gaps)).toEqual([3]);
    close(d.position?.latitude as Float64Array, [47.123456, 47.123458, null, 47.124, 47.124001]);
    close(d.position?.longitude as Float64Array, [8.5, 8.500001, null, 8.50005, 8.500051]);
    close(d.metrics.heart_rate as Float64Array, [120, 121, null, 131, 132]);
    close(d.metrics.temperature as Float64Array, [-2, -3, -3, null, -3]);
    expect(Object.keys(d.metrics)).toEqual(['heart_rate', 'temperature']);
  });

  it('does not keep the input buffer alive', () => {
    const d = decodeOk(workedExample().buffer);
    const arrays = [d.time, d.position?.latitude, d.position?.longitude, d.metrics.heart_rate, d.metrics.temperature];
    for (const a of arrays) expect((a as Float64Array).buffer.byteLength).toBe(5 * 8);
  });
});

// ---------------------------------------------------------------------------

describe('alignment (AC 1)', () => {
  // Sport strings of length 0–3 shift (14 + L) % 4 through every value.
  const sports = ['', 'a', 'ab', 'abc'];
  const cases = sports.flatMap((sport) => [1, 2, 3, 5].map((n) => [sport, n] as const));

  it.each(cases)('sport %j, N = %i: uint8/int8 columns with 1–3 padding bytes decode', (sport, n) => {
    const time = Array.from({ length: n }, (_, i) => i * 2);
    const hr = Array.from({ length: n }, (_, i) => (i === 1 ? null : 100 + i));
    const cad = Array.from({ length: n }, (_, i) => 80 + i);
    const temp = Array.from({ length: n }, (_, i) => -5 + i);
    const buffer = buildDataFile({ time, metrics: { heart_rate: hr, cadence: cad, temperature: temp }, meta: { sport } });
    const l = new DataView(buffer).getUint32(10, true);
    const offsets = (JSON.parse(new TextDecoder().decode(new Uint8Array(buffer, 14, l))) as MetaJson).streams.map(
      (s) => s.byte_offset,
    );
    expect(offsets.every((o) => o % 4 === 0)).toBe(true);
    expect(offsets[0]).toBe(Math.ceil((14 + l) / 4) * 4);
    const d = decodeOk(buffer);
    expect(d.meta.sport).toBe(sport);
    expect(Array.from(d.time)).toEqual(time);
    expect(nanArr(d.metrics.heart_rate as Float64Array)).toEqual(hr);
    expect(Array.from(d.metrics.cadence as Float64Array)).toEqual(cad);
    expect(Array.from(d.metrics.temperature as Float64Array)).toEqual(temp);
  });

  it('covers every (14 + L) % 4', () => {
    const residues = sports.map((sport) => {
      const b = buildDataFile({ time: [0], meta: { sport } });
      return (14 + new DataView(b).getUint32(10, true)) % 4;
    });
    expect(new Set(residues)).toEqual(new Set([0, 1, 2, 3]));
  });

  it('rejects an in-bounds int32 column 2 bytes past its offset as not 4-aligned (rule 6), not as internal', () => {
    // QA-04-r1-1: the column is in bounds and nothing trails it, so only the alignment check can reject it;
    // without that check, new Int32Array(buffer, odd offset) raises a RangeError (code internal).
    const t = layoutOf(build()).streams[0].byte_offset;
    const b = build({ offsets: { time_offset: t + 2 }, trailing: 2 });
    expect(layoutOf(b).l).toBe(layoutOf(build()).l);
    const r = expectReject(() => b, 'corrupt', 6);
    if (!r.ok) expect(r.error.message).toContain('not 4-aligned');
  });
});

// ---------------------------------------------------------------------------

describe('delta decoding (AC 1)', () => {
  const alt = (values: Stored[]) => nanArr(decodeOk(build({ metrics: { altitude: values } })).metrics.altitude as Float64Array);
  const altStored = (values: Stored[]) => alt(values).map((v) => (v === null ? null : Math.round(v * 10)));

  it('first value is absolute', () => {
    expect(altStored([1234, 1235, 1236, 1237, 1238])).toEqual([1234, 1235, 1236, 1237, 1238]);
  });

  it('missing in the middle: the next delta is relative to the last valid value', () => {
    expect(altStored([100, null, null, 130, 131])).toEqual([100, null, null, 130, 131]);
  });

  it('leading and trailing missing', () => {
    expect(altStored([null, null, 50, 60, null])).toEqual([null, null, 50, 60, null]);
  });

  it('one valid value only', () => {
    expect(altStored([null, null, -7, null, null])).toEqual([null, null, -7, null, null]);
  });

  it('negative deltas', () => {
    expect(altStored([500, 400, -100, -1000, -999])).toEqual([500, 400, -100, -1000, -999]);
  });

  it('a running sum beyond int32 does not wrap (parity with Go int64)', () => {
    const d = decodeOk(build({ time: [0, 1], metrics: { altitude: [2000000000, 4000000000] } }));
    expect(Array.from(d.metrics.altitude as Float64Array)).toEqual([200000000, 400000000]);
  });

  it('decodes latitude and longitude deltas', () => {
    const latitude = [47123456, 47123458, null, 47124000, 47124001];
    const longitude = [8500000, 8500001, null, 8500050, 8500051];
    const d = decodeOk(build({ position: { latitude, longitude } }));
    const scaled = (a: (number | null)[]) => a.map((v) => (v === null ? null : v * 0.000001));
    expect(nanArr(d.position?.latitude as Float64Array)).toEqual(scaled(latitude));
    expect(nanArr(d.position?.longitude as Float64Array)).toEqual(scaled(longitude));
  });
});

// ---------------------------------------------------------------------------

describe('sentinels per type (AC 1)', () => {
  const deltaStreams: MetricName[] = ['altitude', 'distance'];

  it.each(deltaStreams)('int32 2147483647 in %s → NaN', (name) => {
    const d = decodeOk(build({ metrics: { [name]: [1, null, 3, null, 5] } }));
    expect(nanArr(d.metrics[name] as Float64Array).map((v) => (v === null ? null : Math.round(v * 10)))).toEqual([1, null, 3, null, 5]);
  });

  it('int32 2147483647 in latitude and longitude → NaN', () => {
    const d = decodeOk(build({ position: { latitude: [1, null, 3, 4, 5], longitude: [1, null, 3, 4, 5] } }));
    expect(Number.isNaN(d.position?.latitude[1])).toBe(true);
    expect(Number.isNaN(d.position?.longitude[1])).toBe(true);
    expect(d.position?.latitude[2]).toBe(3 * 1e-6);
  });

  it.each([
    ['speed', 65535],
    ['power', 65535],
    ['heart_rate', 255],
    ['cadence', 255],
    ['temperature', 127],
  ] as const)('%s sentinel %i → NaN', (name, sentinel) => {
    // null in the builder writes the sentinel; check that it really is the stored value.
    const b = build({ metrics: { [name]: [0, null, 0, 0, 0] } });
    const l = new DataView(b).getUint32(10, true);
    const desc = (JSON.parse(new TextDecoder().decode(new Uint8Array(b, 14, l))) as MetaJson).streams[1];
    const size = sentinel === 65535 ? 2 : 1;
    const dv = new DataView(b);
    const stored = size === 2 ? dv.getUint16(desc.byte_offset + size, true) : name === 'temperature' ? dv.getInt8(desc.byte_offset + 1) : dv.getUint8(desc.byte_offset + 1);
    expect(stored).toBe(sentinel);
    const values = decodeOk(b).metrics[name] as Float64Array;
    expect(Number.isNaN(values[1])).toBe(true);
    expect(values[0]).toBe(0);
  });

  it('values next to the sentinel are numbers', () => {
    const d = decodeOk(
      build({
        time: [0, 1],
        metrics: {
          altitude: [2147483646, 2147483646],
          speed: [65534, 0],
          power: [65534, 0],
          heart_rate: [254, 0],
          cadence: [254, 0],
          temperature: [126, -128],
        },
      }),
    );
    expect(Array.from(d.metrics.altitude as Float64Array)).toEqual([2147483646 * 0.1, 2147483646 * 0.1]);
    expect(Array.from(d.metrics.speed as Float64Array)).toEqual([65534 * 0.001, 0]);
    expect(Array.from(d.metrics.power as Float64Array)).toEqual([65534, 0]);
    expect(Array.from(d.metrics.heart_rate as Float64Array)).toEqual([254, 0]);
    expect(Array.from(d.metrics.cadence as Float64Array)).toEqual([254, 0]);
    expect(Array.from(d.metrics.temperature as Float64Array)).toEqual([126, -128]);
  });
});

// ---------------------------------------------------------------------------

describe('scale (AC 1)', () => {
  it('applies the §2 scale by multiplication, bit-identical', () => {
    const d = decodeOk(
      build({
        time: [0],
        position: { latitude: [47123456], longitude: [-8500000] },
        metrics: { altitude: [1234], distance: [5], speed: [7999], temperature: [-128] },
      }),
    );
    expect(d.position?.latitude[0]).toBe(47123456 * 0.000001);
    expect(d.position?.longitude[0]).toBe(-8500000 * 0.000001);
    expect(d.metrics.altitude?.[0]).toBe(1234 * 0.1);
    expect(d.metrics.distance?.[0]).toBe(5 * 0.1);
    expect(d.metrics.speed?.[0]).toBe(7999 * 0.001);
    expect(d.metrics.temperature?.[0]).toBe(-128);
    // And within float error of the readable value.
    expect(Math.abs((d.position?.latitude[0] as number) - 47.123456)).toBeLessThan(1e-12);
    expect(Math.abs((d.metrics.altitude?.[0] as number) - 123.4)).toBeLessThan(1e-12);
    expect(Math.abs((d.metrics.distance?.[0] as number) - 0.5)).toBeLessThan(1e-12);
    expect(Math.abs((d.metrics.speed?.[0] as number) - 7.999)).toBeLessThan(1e-12);
  });

  function withLatScale(spelling: string): ArrayBuffer {
    // Replace the serialised latitude/longitude scale with a given spelling (numerical comparison, F-03 §5.5).
    const base = buildDataFile({ time: [0], position: { latitude: [1], longitude: [1] } });
    const l = new DataView(base).getUint32(10, true);
    const json = new TextDecoder().decode(new Uint8Array(base, 14, l)).replaceAll('"scale":0.000001', `"scale":${spelling}`);
    expect(json).toContain(`"scale":${spelling}`);
    return buildDataFile({ time: [0], position: { latitude: [1], longitude: [1] }, rawMetadata: json.padEnd(l, ' ').slice(0, l) });
  }

  it('accepts the scale spelled 0.000001 or 1e-06', () => {
    expect(decodeDataFile(withLatScale('0.000001')).ok).toBe(true);
    expect(decodeDataFile(withLatScale('1e-06')).ok).toBe(true);
  });

  it('rejects scale 1e-05 (rule 5)', () => {
    expectReject(() => withLatScale('1e-05'), 'corrupt', 5);
  });
});

// ---------------------------------------------------------------------------

describe('absent streams (AC 1)', () => {
  it('time only: no metrics, no position', () => {
    const d = decodeOk(build());
    expect(Object.keys(d.metrics)).toEqual([]);
    expect(d.position).toBeUndefined();
    expect('position' in d).toBe(false);
  });

  it('every subset of metrics keeps METRIC_ORDER and has no array for absent streams', () => {
    for (let mask = 1; mask < 1 << METRIC_ORDER.length; mask++) {
      const names = METRIC_ORDER.filter((_, i) => mask & (1 << i));
      const metrics: Partial<Record<MetricName, Stored[]>> = {};
      for (const n of names) metrics[n] = [1, 2, 3, 4, 5];
      const d = decodeOk(build({ metrics }));
      expect(Object.keys(d.metrics)).toEqual(names);
      for (const n of METRIC_ORDER) {
        if (!names.includes(n)) expect(n in d.metrics).toBe(false);
      }
    }
  });
});

// ---------------------------------------------------------------------------

describe('cumulative streams (format version 2, ADR-029)', () => {
  it('decodes moving_time (1 s) and elevation_gain (0.1 m) as separate arrays, not metrics', () => {
    const d = decodeOk(
      build({ metrics: { heart_rate: [1, 2, 3, 4, 5], moving_time: [0, 1, 2, 2, 3], elevation_gain: [0, 0, 5, 12, 12] } }),
    );
    expect(Array.from(d.movingTime as Float64Array)).toEqual([0, 1, 2, 2, 3]);
    [0, 0, 0.5, 1.2, 1.2].forEach((v, i) => expect((d.elevationGain as Float64Array)[i]).toBeCloseTo(v, 9));
    expect(Object.keys(d.metrics)).toEqual(['heart_rate']);
  });

  it('are absent when the file has no such stream', () => {
    const d = decodeOk(build({ metrics: { moving_time: [0, 1, 2, 3, 4] } }));
    expect(d.movingTime).toBeDefined();
    expect(d.elevationGain).toBeUndefined();
    expect('elevationGain' in decodeOk(build())).toBe(false);
  });

  it('a missing sample is rejected (rule 7)', () => {
    expectReject(() => build({ metrics: { moving_time: [0, 1, null, 3, 4] } }), 'corrupt', 7);
    expectReject(() => build({ metrics: { elevation_gain: [0, 1, 2, null, 4] } }), 'corrupt', 7);
  });
});

// ---------------------------------------------------------------------------

describe('gaps', () => {
  const gapsOf = (time: number[]) => Array.from(decodeOk(buildDataFile({ time })).gaps);

  it('offsets 0, 1, 2, 10, 11 → [3]', () => expect(gapsOf(T5)).toEqual([3]));
  it('a difference of exactly 5 is a gap, 4 is not', () => {
    expect(gapsOf([0, 5])).toEqual([1]);
    expect(gapsOf([0, 4])).toEqual([]);
  });
  it('N = 1 → no gaps', () => expect(gapsOf([7])).toEqual([]));
  it('many gaps (growing buffer)', () => {
    const time = Array.from({ length: 100 }, (_, i) => i * 10);
    expect(gapsOf(time)).toEqual(Array.from({ length: 99 }, (_, i) => i + 1));
  });
  it('values are not altered at gaps', () => {
    const d = decodeOk(build({ metrics: { power: [1, 2, 3, 4, 5] } }));
    expect(Array.from(d.metrics.power as Float64Array)).toEqual([1, 2, 3, 4, 5]);
  });
});

// ---------------------------------------------------------------------------

describe('metadata', () => {
  const full = {
    sport: 'multisport',
    start_time: '2026-06-06T05:00:00Z',
    time_zone: { known: true, offset_seconds: -18000 },
    summary: {
      elapsed_time: 11, timer_time: 10, distance: 27012.5, ascent: 112, descent: 100, calories: 500, avg_speed: 7.999,
      max_speed: 12, avg_heart_rate: 141, max_heart_rate: 180, avg_cadence: 85, max_cadence: 110, avg_power: 200,
      max_power: 600, avg_temperature: -2, max_temperature: 3, moving_time: 9, pool_length_yd: 25, lengths: 64,
    },
    device: { name: 'Wahoo Fitness ELEMNT BOLT', sensors: ['heart_rate_monitor', 'power_meter'] },
    sessions: [
      { sport: 'running', sub_sport: '', start: 0, end: 2, start_index: 0, end_index: 3, totals: { elapsed_time: 2, distance: 10 } },
      { sport: 'cycling', sub_sport: 'road', start: 10, end: 11, start_index: 3, end_index: 5, totals: {} },
    ],
    laps: [
      { start: 0, end: 2, start_index: 0, end_index: 3, totals: { elapsed_time: 2 } },
      { start: 3, end: 9, start_index: 3, end_index: 3, totals: {} },
    ],
  };

  it('maps every §5 key with its wire name', () => {
    expect(decodeOk(build({ meta: full })).meta).toEqual(full);
  });

  it('absent totals stay absent, never 0', () => {
    const d = decodeOk(build({ meta: { summary: { distance: 5 }, laps: [{ start: 0, end: 1, start_index: 0, end_index: 2, totals: {} }] } }));
    expect(d.meta.summary).toEqual({ distance: 5 });
    expect('elapsed_time' in d.meta.summary).toBe(false);
    expect(d.meta.laps[0].totals).toEqual({});
  });

  it('empty summary, sessions and laps', () => {
    const d = decodeOk(build());
    expect(d.meta.summary).toEqual({});
    expect(d.meta.sessions).toEqual([]);
    expect(d.meta.laps).toEqual([]);
  });

  it('unknown time zone', () => {
    const d = decodeOk(build({ meta: { time_zone: { known: false, offset_seconds: 0 } } }));
    expect(d.meta.time_zone).toEqual({ known: false, offset_seconds: 0 });
  });

  it('drops unknown keys at every level, and moving_time outside the summary', () => {
    const d = decodeOk(
      build({
        meta: {
          time_zone: { known: true, offset_seconds: 0, name: 'x' },
          summary: { distance: 1, bogus: 'x' },
          device: { name: 'x', sensors: [], serial: 1 },
          sessions: [{ sport: 's', sub_sport: '', start: 0, end: 1, start_index: 0, end_index: 2, totals: { moving_time: 1, x: 1 }, extra: 1 }],
          laps: [{ start: 0, end: 1, start_index: 0, end_index: 2, totals: { avg_power: 1, y: 'z' }, sport: 'x' }],
        },
        patch: (m) => ({ ...m, extra: { a: 1 }, streams: m.streams.map((s) => ({ ...s, note: 'x' })) }),
      }),
    );
    expect(Object.keys(d.meta)).toEqual(['sport', 'start_time', 'time_zone', 'summary', 'device', 'sessions', 'laps']);
    expect(d.meta.device).toEqual({ name: 'x', sensors: [] });
    expect(d.meta.time_zone).toEqual({ known: true, offset_seconds: 0 });
    expect(d.meta.summary).toEqual({ distance: 1 });
    expect(d.meta.sessions[0]).toEqual({ sport: 's', sub_sport: '', start: 0, end: 1, start_index: 0, end_index: 2, totals: {} });
    expect(d.meta.laps[0]).toEqual({ start: 0, end: 1, start_index: 0, end_index: 2, totals: { avg_power: 1 } });
  });

  it('does not expose the stream descriptors', () => {
    expect('streams' in decodeOk(build()).meta).toBe(false);
  });

  it('accepts an integer written as 4.0 (the one documented case where F-04 accepts what Go rejects)', () => {
    // Two characters shorter in the sport, two longer in the lap end: same L, so the offsets stay valid.
    const meta = { sport: 'cycling--', laps: [{ start: 0, end: 1, start_index: 0, end_index: 2, totals: {} }] };
    const b = build({ meta });
    const l = new DataView(b).getUint32(10, true);
    const json = new TextDecoder()
      .decode(new Uint8Array(b, 14, l))
      .replace('"sport":"cycling--"', '"sport":"cycling"')
      .replace('"end":1,', '"end":1.0,');
    expect(json.length).toBe(l);
    expect(json).toContain('"end":1.0,');
    expect(decodeOk(build({ meta, rawMetadata: json })).meta.laps[0].end).toBe(1);
  });
});

// ---------------------------------------------------------------------------

describe('rejects (AC 3)', () => {
  const lap = { start: 0, end: 1, start_index: 0, end_index: 2, totals: {} };
  const ses = { sport: 'cycling', sub_sport: '', ...lap };
  const without = (o: Record<string, unknown>, key: string) => {
    const c = { ...o };
    delete c[key];
    return c;
  };
  const topKeys = ['sport', 'start_time', 'time_zone', 'summary', 'device', 'sessions', 'laps', 'streams'] as const;
  const wrongType: Record<(typeof topKeys)[number], unknown> = {
    sport: 1,
    start_time: null,
    time_zone: [],
    summary: 'x',
    device: [],
    sessions: {},
    laps: 'x',
    streams: {},
  };

  const cases: [string, () => ArrayBuffer, DataFileErrorCode, number][] = [
    // Rule 1
    ['0 bytes', () => new ArrayBuffer(0), 'not-a-data-file', 1],
    ['13 bytes', () => build().slice(0, 13), 'not-a-data-file', 1],
    ['bad magic', () => build({ magic: 'WEXE' }), 'not-a-data-file', 1],
    // Rule 2
    ['version 0', () => build({ version: 0 }), 'unsupported-version', 2],
    ['version 1', () => build({ version: 1 }), 'unsupported-version', 2],
    // Rule 3
    ['N = 0', () => build({ sampleCount: 0 }), 'corrupt', 3],
    [
      'L beyond the end',
      () => {
        const b = build();
        new DataView(b).setUint32(10, b.byteLength, true);
        return b;
      },
      'corrupt',
      3,
    ],
    // Rule 4
    ['invalid UTF-8', () => build({ rawMetadata: new Uint8Array([0x7b, 0xff, 0x7d]) }), 'corrupt', 4],
    ['BOM before the JSON', () => build({ rawMetadata: new Uint8Array([0xef, 0xbb, 0xbf, 0x7b, 0x7d]) }), 'corrupt', 4],
    ['invalid JSON', () => build({ rawMetadata: '{"sport":' }), 'corrupt', 4],
    ['top-level array', () => build({ rawMetadata: '[]' }), 'corrupt', 4],
    ['top-level null', () => build({ rawMetadata: 'null' }), 'corrupt', 4],
    ...topKeys.map((k): [string, () => ArrayBuffer, DataFileErrorCode, number] => [
      `${k} missing`,
      () => build({ patch: (m) => without(m as unknown as Record<string, unknown>, k) }),
      'corrupt',
      4,
    ]),
    ...topKeys.map((k): [string, () => ArrayBuffer, DataFileErrorCode, number] => [
      `${k} with the wrong type`,
      () => build({ patch: (m) => ({ ...m, [k]: wrongType[k] }) }),
      'corrupt',
      4,
    ]),
    ['time_zone.known missing', () => build({ meta: { time_zone: { offset_seconds: 0 } } }), 'corrupt', 4],
    ['time_zone.known not boolean', () => build({ meta: { time_zone: { known: 1, offset_seconds: 0 } } }), 'corrupt', 4],
    ['time_zone.offset_seconds fractional', () => build({ meta: { time_zone: { known: true, offset_seconds: 0.5 } } }), 'corrupt', 4],
    ['summary total as quoted number', () => build({ meta: { summary: { distance: '12' } } }), 'corrupt', 4],
    ['summary total null', () => build({ meta: { summary: { distance: null } } }), 'corrupt', 4],
    ['device.name missing', () => build({ meta: { device: { sensors: [] } } }), 'corrupt', 4],
    ['device.name not a string', () => build({ meta: { device: { name: 1, sensors: [] } } }), 'corrupt', 4],
    ['device.sensors not an array', () => build({ meta: { device: { name: '', sensors: 'x' } } }), 'corrupt', 4],
    ['device.sensors holds a number', () => build({ meta: { device: { name: '', sensors: [1] } } }), 'corrupt', 4],
    ['lap start as string', () => build({ meta: { laps: [{ ...lap, start: '0' }] } }), 'corrupt', 4],
    ['lap start_index missing', () => build({ meta: { laps: [without(lap, 'start_index')] } }), 'corrupt', 4],
    ['lap totals missing', () => build({ meta: { laps: [without(lap, 'totals')] } }), 'corrupt', 4],
    ['"sessions": [null]', () => build({ meta: { sessions: [null] } }), 'corrupt', 4],
    ['"laps": [1]', () => build({ meta: { laps: [1] } }), 'corrupt', 4],
    ['session sub_sport missing', () => build({ meta: { sessions: [without(ses, 'sub_sport')] } }), 'corrupt', 4],
    ['session start 2^31', () => build({ meta: { sessions: [{ ...ses, start: 2 ** 31, end: 2 ** 31 }] } }), 'corrupt', 4],
    [
      'descriptor byte_offset as string',
      () => build({ patch: (m) => ({ ...m, streams: m.streams.map((s) => ({ ...s, byte_offset: String(s.byte_offset) })) }) }),
      'corrupt',
      4,
    ],
    ['descriptor scale missing', () => build({ patch: (m) => ({ ...m, streams: m.streams.map((s) => without({ ...s }, 'scale')) }) }), 'corrupt', 4],
    ['streams: [null]', () => build({ patch: (m) => ({ ...m, streams: [null] }) }), 'corrupt', 4],
    // Rule 5
    ['empty streams', () => build({ patch: (m) => ({ ...m, streams: [] }) }), 'corrupt', 5],
    [
      'first stream not time_offset',
      () => build({ metrics: { power: [1, 2, 3, 4, 5] }, patch: (m) => ({ ...m, streams: [m.streams[1], m.streams[0]] }) }),
      'corrupt',
      5,
    ],
    ['unknown name', () => build({ metrics: { power: [1, 2, 3, 4, 5] }, patch: (m) => ({ ...m, streams: [m.streams[0], { ...m.streams[1], name: 'watts' }] }) }), 'corrupt', 5],
    ['repeated name', () => build({ metrics: { power: [1, 2, 3, 4, 5] }, patch: (m) => ({ ...m, streams: [m.streams[0], m.streams[1], m.streams[1]] }) }), 'corrupt', 5],
    [
      'out of order',
      () =>
        build({
          metrics: { heart_rate: [1, 2, 3, 4, 5], power: [1, 2, 3, 4, 5] },
          patch: (m) => ({ ...m, streams: [m.streams[0], m.streams[2], m.streams[1]] }),
        }),
      'corrupt',
      5,
    ],
    [
      'latitude without longitude',
      () => build({ position: { latitude: [1, 2, 3, 4, 5], longitude: [1, 2, 3, 4, 5] }, patch: (m) => ({ ...m, streams: m.streams.filter((s) => s.name !== 'longitude') }) }),
      'corrupt',
      5,
    ],
    [
      'longitude without latitude',
      () => build({ position: { latitude: [1, 2, 3, 4, 5], longitude: [1, 2, 3, 4, 5] }, patch: (m) => ({ ...m, streams: m.streams.filter((s) => s.name !== 'latitude') }) }),
      'corrupt',
      5,
    ],
    ...(
      [
        ['type', 'int32'],
        ['encoding', 'delta'],
        ['unit', 'watts'],
        ['scale', 0.1],
      ] as const
    ).map(([key, value]): [string, () => ArrayBuffer, DataFileErrorCode, number] => [
      `descriptor ${key} differs from §2`,
      () => build({ metrics: { power: [1, 2, 3, 4, 5] }, patch: (m) => ({ ...m, streams: [m.streams[0], { ...m.streams[1], [key]: value }] }) }),
      'corrupt',
      5,
    ]),
    // Rule 6
    ['byte_offset not a multiple of 4', () => build({ offsets: { time_offset: 1001 } }), 'corrupt', 6],
    ['byte_offset negative', () => build({ offsets: { time_offset: -4 } }), 'corrupt', 6],
    ['byte_offset > length', () => build({ offsets: { time_offset: 1000000 } }), 'corrupt', 6],
    ['byte_offset 4,294,967,292', () => build({ offsets: { time_offset: 4294967292 } }), 'corrupt', 6],
    [
      'byte_offset 9e18 with N = 4,294,967,295',
      () => build({ offsets: { time_offset: 9000000000000000000 }, sampleCount: 4294967295 }),
      'corrupt',
      6,
    ],
    ['byte_offset 1e20', () => build({ offsets: { time_offset: 1e20 } }), 'corrupt', 6],
    ['byte_offset below align4(14 + L)', () => build({ offsets: { time_offset: 12 } }), 'corrupt', 6],
    [
      'column overlapping the previous one',
      () => {
        const ok = build({ metrics: { power: [1, 2, 3, 4, 5] } });
        const l = new DataView(ok).getUint32(10, true);
        const t = (JSON.parse(new TextDecoder().decode(new Uint8Array(ok, 14, l))) as MetaJson).streams[0].byte_offset;
        return build({ metrics: { power: [1, 2, 3, 4, 5] }, offsets: { power: t + 16 } });
      },
      'corrupt',
      6,
    ],
    ['N × size beyond the end', () => build({ sampleCount: 6 }), 'corrupt', 6],
    ['one trailing byte', () => build({ trailing: 1 }), 'corrupt', 6],
    // Rule 7
    ['time missing at one index', () => build({ time: [0, 1, null, 3, 4] }), 'corrupt', 7],
    ['time equal', () => build({ time: [0, 1, 1, 3, 4] }), 'corrupt', 7],
    ['time decreasing', () => build({ time: [0, 2, 1, 3, 4] }), 'corrupt', 7],
    ['half a position at one index', () => build({ position: { latitude: [1, 2, null, 4, 5], longitude: [1, 2, 3, 4, 5] } }), 'corrupt', 7],
    ['session end < start', () => build({ meta: { sessions: [{ ...ses, start: 2, end: 1 }] } }), 'corrupt', 7],
    ['session start_index > end_index', () => build({ meta: { sessions: [{ ...ses, start_index: 3, end_index: 2 }] } }), 'corrupt', 7],
    ['session end_index > N', () => build({ meta: { sessions: [{ ...ses, end_index: 6 }] } }), 'corrupt', 7],
    ['session start_index < 0', () => build({ meta: { sessions: [{ ...ses, start_index: -1 }] } }), 'corrupt', 7],
    ['lap end < start', () => build({ meta: { laps: [{ ...lap, start: 2, end: 1 }] } }), 'corrupt', 7],
    ['lap start_index > end_index', () => build({ meta: { laps: [{ ...lap, start_index: 3, end_index: 2 }] } }), 'corrupt', 7],
    ['lap end_index > N', () => build({ meta: { laps: [{ ...lap, end_index: 6 }] } }), 'corrupt', 7],
    ['lap start_index < 0', () => build({ meta: { laps: [{ ...lap, start_index: -1 }] } }), 'corrupt', 7],
    ['lap start_index 1e20', () => build({ meta: { laps: [{ ...lap, start_index: 1e20, end_index: 1e20 }] } }), 'corrupt', 7],
  ];

  it.each(cases)('%s', (_name, buffer, code, rule) => {
    expectReject(buffer, code, rule);
  });

  it('checks in Go order: a file with faults in rules 6 and 7 reports rule 6', () => {
    expectReject(() => build({ time: [0, 0, 0, 0, 0], trailing: 1 }), 'corrupt', 6);
  });

  it('sessions are checked before laps', () => {
    const r = expectReject(
      () => build({ meta: { sessions: [{ ...ses, end_index: 9 }], laps: [{ ...lap, start: 2, end: 1 }] } }),
      'corrupt',
      7,
    );
    if (!r.ok) expect(r.error.message).toContain('session 0');
  });

  it('messages hold no metadata or sample values', () => {
    const r = expectReject(
      () => build({ metrics: { power: [1, 2, 3, 4, 5] }, patch: (m) => ({ ...m, streams: [m.streams[0], { ...m.streams[1], name: 'secret-47.123' }] }) }),
      'corrupt',
      5,
    );
    if (!r.ok) expect(r.error.message).not.toContain('47.123');
  });
});

// ---------------------------------------------------------------------------

// QA-04-r1-1: one input per check that no earlier or later check rejects, so each test fails when
// exactly its check is removed. The generic reject cases above may be caught by several checks.
describe('distinguishing rejects (QA-04-r1-1)', () => {
  const five = [1, 2, 3, 4, 5];

  it('overlap: a column starting inside the previous one, file ending at that column (rule 6)', () => {
    const t = layoutOf(build({ metrics: { power: five } })).streams[0].byte_offset;
    // time takes t … t+19; power at t+12 overlaps it by 8 bytes and ends at t+22, the new file end.
    const b = build({ metrics: { power: five }, offsets: { power: t + 12 } }).slice(0, t + 22);
    expect(layoutOf(b).streams[1].byte_offset).toBe(t + 12);
    const r = expectReject(() => b, 'corrupt', 6);
    if (!r.ok) expect(r.error.message).toContain('overlaps');
  });

  it('overlap boundary: a column overlapping the previous one by exactly 4 bytes (rule 6, QA-04-r2-1)', () => {
    const t = layoutOf(build({ metrics: { power: five } })).streams[0].byte_offset;
    // time takes t … t+19; power at t+16 overlaps its last sample and ends at t+26, the new file end.
    const b = build({ metrics: { power: five }, offsets: { power: t + 16 } }).slice(0, t + 26);
    expect(layoutOf(b).streams[1].byte_offset).toBe(t + 16);
    const r = expectReject(() => b, 'corrupt', 6);
    if (!r.ok) expect(r.error.message).toContain('overlaps');
  });

  it('first stream: latitude and longitude without any time_offset (rule 5)', () => {
    const b = build({ position: { latitude: five, longitude: five }, patch: (m) => ({ ...m, streams: m.streams.slice(1) }) });
    expect(layoutOf(b).streams.map((s) => s.name)).toEqual(['latitude', 'longitude']);
    const r = expectReject(() => b, 'corrupt', 5);
    if (!r.ok) expect(r.error.message).toContain('first stream is not time_offset');
  });

  it('order: adjacent §2 neighbours swapped, cadence before heart_rate (rule 5)', () => {
    const b = build({
      metrics: { heart_rate: five, cadence: five },
      patch: (m) => ({ ...m, streams: [m.streams[0], m.streams[2], m.streams[1]] }),
    });
    expect(layoutOf(b).streams.map((s) => s.name)).toEqual(['time_offset', 'cadence', 'heart_rate']);
    const r = expectReject(() => b, 'corrupt', 5);
    if (!r.ok) expect(r.error.message).toContain('heart_rate out of order');
  });

  it('UTF-8: byte 0xFF inside the sport string of otherwise valid metadata (rule 4)', () => {
    const meta = { sport: 'cyclinX' };
    const { bytes } = layoutOf(build({ meta }));
    const at = new TextDecoder().decode(bytes).indexOf('cyclinX') + 6;
    const raw = bytes.slice();
    raw[at] = 0xff;
    const r = expectReject(() => build({ meta, rawMetadata: raw }), 'corrupt', 4);
    if (!r.ok) expect(r.error.message).toContain('not valid UTF-8');
  });

  it('BOM: EF BB BF before complete, valid metadata of the same length (rule 4, as Go)', () => {
    // Three characters longer in the sport, three shorter after the rewrite, plus the 3-byte BOM: same L.
    const meta = { sport: 'cyclingabc' };
    const { l, bytes } = layoutOf(build({ meta }));
    const json = new TextEncoder().encode(new TextDecoder().decode(bytes).replace('"cyclingabc"', '"cycling"'));
    const raw = new Uint8Array(l);
    raw.set([0xef, 0xbb, 0xbf]);
    raw.set(json, 3);
    expect(json.length + 3).toBe(l);
    // The same bytes without the BOM (padded with a trailing space) are accepted.
    const noBom = new Uint8Array(l).fill(0x20);
    noBom.set(json);
    expect(decodeDataFile(build({ meta, rawMetadata: noBom })).ok).toBe(true);
    const r = expectReject(() => build({ meta, rawMetadata: raw }), 'corrupt', 4);
    if (!r.ok) expect(r.error.message).toContain('not valid JSON');
  });
});

// ---------------------------------------------------------------------------

describe('robustness', () => {
  // mulberry32: small seeded PRNG, so failures reproduce.
  function prng(seed: number): () => number {
    let a = seed >>> 0;
    return () => {
      a = (a + 0x6d2b79f5) >>> 0;
      let t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  it('500 random buffers: no throw, never internal', () => {
    const rnd = prng(4);
    for (let k = 0; k < 500; k++) {
      const len = Math.floor(rnd() * 601);
      const b = new Uint8Array(len);
      for (let i = 0; i < len; i++) b[i] = Math.floor(rnd() * 256);
      // Half of them get a valid magic and version so the deeper rules are reached.
      if (k % 2 === 1 && len >= 6) b.set([0x57, 0x45, 0x58, 0x44, 0x02, 0x00]);
      const r = decodeDataFile(b.buffer);
      if (!r.ok) expect(r.error.code, `buffer ${k}`).not.toBe('internal');
    }
  });

  it('500 copies of gps-dropout.bin with one random byte changed: no throw, never internal', () => {
    const g = loadGolden().find((x) => x.name === 'gps-dropout');
    if (!g) throw new Error('gps-dropout golden missing');
    const l = new DataView(g.bin).getUint32(10, true);
    const rnd = prng(7);
    for (let k = 0; k < 500; k++) {
      const b = new Uint8Array(g.bin.slice(0));
      // Half anywhere, half within header and metadata, where most checks live.
      const limit = k % 2 === 0 ? b.length : 14 + l;
      const at = Math.floor(rnd() * limit);
      b[at] = (b[at] + 1 + Math.floor(rnd() * 255)) & 0xff;
      const r = decodeDataFile(b.buffer);
      if (!r.ok) expect(r.error.code, `copy ${k}, byte ${at}`).not.toBe('internal');
    }
  });
});
