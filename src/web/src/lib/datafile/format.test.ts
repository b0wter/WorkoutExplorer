import { describe, expect, it } from 'vitest';
import { DYNAMICS_ORDER, GAP_SECONDS, HEADER_SIZE, MAGIC, METRIC_ORDER, METRIC_UNITS, STREAM_SPECS, VERSION, align4 } from './format';
import * as publicSurface from './index';

// F-03 §2 written out literally: name, type, encoding, scale, unit, size, sentinel; array index = order.
const SECTION_2 = [
  ['time_offset', 'int32', 'delta', 1, 's', 4, 2147483647],
  ['latitude', 'int32', 'delta', 1e-6, 'deg', 4, 2147483647],
  ['longitude', 'int32', 'delta', 1e-6, 'deg', 4, 2147483647],
  ['altitude', 'int32', 'delta', 0.1, 'm', 4, 2147483647],
  ['distance', 'int32', 'delta', 0.1, 'm', 4, 2147483647],
  ['speed', 'uint16', 'raw', 0.001, 'm/s', 2, 65535],
  ['heart_rate', 'uint8', 'raw', 1, 'bpm', 1, 255],
  ['cadence', 'uint8', 'raw', 1, '1/min', 1, 255],
  ['power', 'uint16', 'raw', 1, 'W', 2, 65535],
  ['temperature', 'int8', 'raw', 1, 'degC', 1, 127],
  ['moving_time', 'int32', 'delta', 1, 's', 4, 2147483647],
  ['elevation_gain', 'int32', 'delta', 0.1, 'm', 4, 2147483647],
  ['vertical_oscillation', 'uint16', 'raw', 0.1, 'cm', 2, 65535],
  ['ground_contact_time', 'uint16', 'raw', 1, 'ms', 2, 65535],
  ['stride_length', 'uint16', 'raw', 0.001, 'm', 2, 65535],
  ['vertical_ratio', 'uint16', 'raw', 0.1, '%', 2, 65535],
] as const;

describe('format constants', () => {
  it('STREAM_SPECS equals the F-03 §2 table, in order', () => {
    expect(STREAM_SPECS.map((s) => [s.name, s.type, s.encoding, s.scale, s.unit, s.size, s.sentinel])).toEqual(
      SECTION_2.map((r) => [...r]),
    );
  });

  it('header constants match F-03 §1', () => {
    expect(MAGIC).toBe('WEXD');
    expect(VERSION).toBe(8);
    expect(HEADER_SIZE).toBe(14);
  });

  it('METRIC_ORDER is §2 order without time and position', () => {
    expect(METRIC_ORDER).toEqual(['altitude', 'distance', 'speed', 'heart_rate', 'cadence', 'power', 'temperature', ...DYNAMICS_ORDER]);
    expect(DYNAMICS_ORDER).toEqual(['vertical_oscillation', 'ground_contact_time', 'stride_length', 'vertical_ratio']);
  });

  it('METRIC_UNITS are the §2 unit strings', () => {
    expect(METRIC_UNITS).toEqual({
      altitude: 'm',
      distance: 'm',
      speed: 'm/s',
      heart_rate: 'bpm',
      cadence: '1/min',
      power: 'W',
      temperature: 'degC',
    });
  });

  it('GAP_SECONDS is 5 (F-02 R-3, F-03 §7)', () => {
    expect(GAP_SECONDS).toBe(5);
  });

  it('align4 rounds up to a multiple of 4', () => {
    expect([0, 1, 2, 3, 4, 5, 415, 416, 4294967295 + 14].map(align4)).toEqual([0, 4, 4, 4, 4, 8, 416, 416, 4294967312]);
  });

  it('index.ts exports exactly the public surface', () => {
    expect(Object.keys(publicSurface).sort()).toEqual(
      ['DYNAMICS_ORDER', 'GAP_SECONDS', 'METRIC_ORDER', 'METRIC_UNITS', 'decodeDataFile', 'loadDataFile'].sort(),
    );
  });
});
