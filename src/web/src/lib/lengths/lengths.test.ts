import { describe, expect, it } from 'vitest';
import type { Length } from '../datafile';
import { hasStrokeCounts, lengthRows } from '.';

const active = (time?: number, strokes?: number): Length => ({ start_index: 0, end_index: 1, rest: false, stroke: 'freestyle', time, strokes });
const rest = (time?: number): Length => ({ start_index: 0, end_index: 1, rest: true, stroke: '', time });

describe('lengthRows (F-48 PL-8)', () => {
  it('numbers active lengths across the workout; rest rows are unnumbered and the numbering continues', () => {
    expect(lengthRows([active(20), active(21), rest(30), active(22), rest(15), rest(10), active(23)]).map((r) => r.number)).toEqual([1, 2, 0, 3, 0, 0, 4]);
  });

  it('SWOLF is the rounded time plus the stroke count: 21.6 s with 14 strokes is 36', () => {
    expect(lengthRows([active(21.6, 14)])[0]).toEqual({ number: 1, time: 22, swolf: 36 });
  });

  it('rounds half away from zero, once, for time and SWOLF alike', () => {
    expect(lengthRows([active(21.5, 10), active(21.49, 10)]).map((r) => [r.time, r.swolf])).toEqual([[22, 32], [21, 31]]);
  });

  it('no stroke count, no time, or a rest length: SWOLF NaN', () => {
    const rows = lengthRows([active(20), active(undefined, 12), rest(30)]);
    expect(rows[0].swolf).toBeNaN();
    expect(rows[1].time).toBeNaN();
    expect(rows[1].swolf).toBeNaN();
    expect(rows[2]).toMatchObject({ number: 0, time: 30 });
    expect(rows[2].swolf).toBeNaN();
  });
});

describe('hasStrokeCounts', () => {
  it('true when any active length has a stroke count', () => {
    expect(hasStrokeCounts([active(20), active(20, 12)])).toBe(true);
    expect(hasStrokeCounts([active(20), rest(20)])).toBe(false);
    expect(hasStrokeCounts([])).toBe(false);
  });
});
