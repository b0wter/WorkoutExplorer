// F-04 AC 2 and AC 4 against the golden data files of F-03 (closes OQ-8).
// Every committed synthetic fixture: stored integers recovered exactly, counts,
// statistics and SHA-256 digests as in F-03 "Golden fixtures".

import { describe, expect, it } from 'vitest';
import { loadGolden, type Golden } from '../../../test-support/golden.mjs';
import { decodeDataFile } from './decode';
import type { WorkoutData } from './model';

interface FixtureStream {
  name: string;
  byte_offset: number;
  valid: number;
  missing: number;
  first: number;
  last: number;
  min: number;
  max: number;
  digest: string;
}
interface Fixture {
  fixture_version: number;
  format_version: number;
  byte_length: number;
  sample_count: number;
  metadata_length: number;
  streams: FixtureStream[];
}

// The test's own §2 scale literals, independent of format.ts.
const SCALE: Record<string, number> = {
  time_offset: 1,
  latitude: 0.000001,
  longitude: 0.000001,
  altitude: 0.1,
  distance: 0.1,
  speed: 0.001,
  heart_rate: 1,
  cadence: 1,
  power: 1,
  temperature: 1,
  moving_time: 1,
  elevation_gain: 0.1,
};

const goldens = loadGolden();

function streamsOf(data: WorkoutData): [string, Float64Array][] {
  const out: [string, Float64Array][] = [['time_offset', data.time]];
  if (data.position) {
    out.push(['latitude', data.position.latitude], ['longitude', data.position.longitude]);
  }
  for (const [name, values] of Object.entries(data.metrics)) out.push([name, values]);
  if (data.movingTime) out.push(['moving_time', data.movingTime]);
  if (data.elevationGain) out.push(['elevation_gain', data.elevationGain]);
  return out;
}

async function digest(lines: string): Promise<string> {
  const hash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(lines));
  return Array.from(new Uint8Array(hash), (b) => b.toString(16).padStart(2, '0'))
    .join('')
    .slice(0, 16);
}

function decodeOk(bin: ArrayBuffer): WorkoutData {
  const r = decodeDataFile(bin);
  if (!r.ok) throw new Error(`decode failed: ${r.error.message}`);
  return r.data;
}

function fixtureOf(g: Golden): Fixture {
  const f = g.fixture as Fixture;
  if (f.fixture_version !== 1) throw new Error(`${g.name}: fixture shape changed (fixture_version ${f.fixture_version})`);
  return f;
}

describe('golden fixtures', () => {
  it('include the required files', () => {
    const names = goldens.map((g) => g.name);
    for (const required of ['ride-12h', 'gps-dropout', 'brick-run-bike', 'indoor-no-gps', 'indoor-no-distance-no-gps', 'pool-swim']) {
      expect(names).toContain(required);
    }
    const ride12h = goldens.find((g) => g.name === 'ride-12h') as Golden;
    expect(decodeOk(ride12h.bin).sampleCount).toBe(43200);
  });

  it('gps-dropout has missing positions', () => {
    const g = goldens.find((x) => x.name === 'gps-dropout') as Golden;
    const lat = decodeOk(g.bin).position?.latitude as Float64Array;
    expect(lat.some((v) => Number.isNaN(v))).toBe(true);
  });

  describe.each(goldens.map((g) => [g.name, g] as const))('%s', (_name, g) => {
    it('decodes to the fixture (AC 2)', async () => {
      const f = fixtureOf(g);
      const data = decodeOk(g.bin);
      const header = new DataView(g.bin);
      expect(g.bin.byteLength).toBe(f.byte_length);
      expect(data.sampleCount).toBe(f.sample_count);
      expect(header.getUint16(4, true)).toBe(f.format_version);
      const l = header.getUint32(10, true);
      expect(l).toBe(f.metadata_length);

      const streams = streamsOf(data);
      expect(streams.map(([name]) => name)).toEqual(f.streams.map((s) => s.name));

      // Byte offsets from the file's own descriptors, read by the test.
      const meta = JSON.parse(new TextDecoder().decode(new Uint8Array(g.bin, 14, l))) as {
        streams: { name: string; byte_offset: number }[];
      };
      expect(meta.streams.map((s) => s.byte_offset)).toEqual(f.streams.map((s) => s.byte_offset));

      for (let k = 0; k < streams.length; k++) {
        const [name, values] = streams[k];
        const exp = f.streams[k];
        const s = SCALE[name];
        expect(values.length, name).toBe(f.sample_count);
        let valid = 0;
        let missing = 0;
        let first: number | undefined;
        let last: number | undefined;
        let min = Infinity;
        let max = -Infinity;
        let firstValid = -1;
        let offGrid = 0;
        const lines: string[] = [];
        for (let i = 0; i < values.length; i++) {
          const v = values[i];
          if (Number.isNaN(v)) {
            missing++;
            lines.push('-\n');
            continue;
          }
          const k2 = Math.round(v / s);
          if (!(Math.abs(v / s - k2) < 1e-6)) offGrid++;
          valid++;
          if (first === undefined) {
            first = k2;
            firstValid = i;
          }
          last = k2;
          if (k2 < min) min = k2;
          if (k2 > max) max = k2;
          lines.push(`${k2}\n`);
        }
        expect(offGrid, `${name}: values off the ${s} grid`).toBe(0);
        expect({ valid, missing, first, last, min, max }, name).toEqual({
          valid: exp.valid,
          missing: exp.missing,
          first: exp.first,
          last: exp.last,
          min: exp.min,
          max: exp.max,
        });
        expect(await digest(lines.join('')), `${name} digest`).toBe(exp.digest);
        expect(values[firstValid], `${name}: first value = first × scale`).toBe(exp.first * s);
      }
    });

    it('rejects a changed magic byte (rule 1)', () => {
      const b = new Uint8Array(g.bin.slice(0));
      b[0] ^= 0xff;
      const r = decodeDataFile(b.buffer);
      expect(r.ok).toBe(false);
      if (!r.ok) expect([r.error.code, r.error.rule]).toEqual(['not-a-data-file', 1]);
    });

    it('rejects version 1 (rule 2)', () => {
      const b = g.bin.slice(0);
      new DataView(b).setUint16(4, 1, true);
      const r = decodeDataFile(b);
      expect(r.ok).toBe(false);
      if (!r.ok) expect([r.error.code, r.error.rule]).toEqual(['unsupported-version', 2]);
    });

    it('rejects one appended byte (rule 6)', () => {
      const b = new Uint8Array(g.bin.byteLength + 1);
      b.set(new Uint8Array(g.bin));
      const r = decodeDataFile(b.buffer);
      expect(r.ok).toBe(false);
      if (!r.ok) expect([r.error.code, r.error.rule]).toEqual(['corrupt', 6]);
    });

    it('rejects the last byte removed (rule 6)', () => {
      const r = decodeDataFile(g.bin.slice(0, g.bin.byteLength - 1));
      expect(r.ok).toBe(false);
      if (!r.ok) expect([r.error.code, r.error.rule]).toEqual(['corrupt', 6]);
    });
  });

  // AC 4: indoor files decode with no position stream and no error.
  it.each(['indoor-no-gps', 'indoor-no-distance-no-gps', 'pool-swim'])('%s has no position (AC 4)', (name) => {
    const g = goldens.find((x) => x.name === name) as Golden;
    const r = decodeDataFile(g.bin);
    expect(r.ok).toBe(true);
    if (r.ok) {
      expect(r.data.position).toBeUndefined();
      expect('latitude' in r.data.metrics || 'longitude' in r.data.metrics).toBe(false);
    }
  });
});
