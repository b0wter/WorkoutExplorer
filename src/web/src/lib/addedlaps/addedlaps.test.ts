import { describe, expect, it } from 'vitest';
import { decodeDataFile, type Lap, type WorkoutData } from '../datafile';
import { createRangeStats } from '../range';
import { loadAddedLaps, loadAutoLaps, loadGolden } from '../../../test-support/golden.mjs';
import { addLaps, isAutoLap, lapEnds, removableBoundaries, removeAutoLaps, snapLapBoundary } from '.';

const golden = (name: string): WorkoutData => {
  const g = loadGolden().find((x) => x.name === name)!;
  const r = decodeDataFile(g.bin);
  if (!r.ok) throw new Error(r.error.message);
  return r.data;
};
const merge = (data: WorkoutData, at: number[], lo = 0, hi = data.sampleCount - 1): Lap[] => addLaps(data, createRangeStats(data), at, lo, hi);
const shape = (laps: readonly Lap[]) => laps.map((l) => [l.start, l.end, l.start_index, l.end_index, l.trigger, l.stroke]);

describe('F-65 AL-3, AL-4: the browser merge agrees with the Go build (added-laps.json)', () => {
  for (const c of loadAddedLaps()) {
    it(c.name, () => {
      const data = golden(c.golden);
      const at = c.at.map((i) => snapLapBoundary(data.meta, data.time, i));
      const laps = merge(data, at);
      const want = c.laps as Lap[];
      expect(shape(laps)).toEqual(shape(want));
      laps.forEach((l, i) => {
        expect(Object.keys(l.totals).sort()).toEqual(Object.keys(want[i].totals).sort());
        for (const [k, v] of Object.entries(want[i].totals)) expect(Math.abs(l.totals[k as keyof Lap['totals']]! - v), `${c.name} lap ${i + 1} ${k}`).toBeLessThanOrEqual(0.25 + 2e-3 * Math.abs(v));
      });
    });
  }
});

describe('F-65 AL-3 rules', () => {
  const data = golden('ride-lap-triggers');
  const n = data.sampleCount;
  const device = data.meta.laps;

  it('unsplit laps stay the same objects; nothing to split returns the device laps', () => {
    expect(merge(data, [])).toEqual(device);
    const laps = merge(data, [214]);
    expect(laps.slice(2)).toEqual(device.slice(1).map((l) => expect.objectContaining(l)));
    expect(laps[2]).toBe(device[1]);
  });

  it('a lap end, a window end, outside the window and a repeat are skipped', () => {
    const ends = [...lapEnds(device, n)];
    expect(ends).toContain(0);
    expect(ends).toContain(n - 1);
    expect(merge(data, ends)).toEqual(device);
    expect(merge(data, [100], 100, n - 1)).toEqual(device);
    expect(merge(data, [100], 0, 50)).toEqual(device);
    expect(merge(data, [100, 100])).toHaveLength(device.length + 1);
  });

  it('is idempotent on its own boundaries: each part ends where it was cut', () => {
    const laps = merge(data, [100, 250]);
    const cut = laps.filter((l) => l.trigger === 'added').map((l) => l.end_index - 1);
    expect(cut).toEqual([100, 250]);
    expect(laps[0].end_index - 1).toBe(laps[1].start_index); // the shared sample
  });

  it('without device laps the first boundary gives two laps: added, then empty trigger', () => {
    const plain = golden('ride-no-laps');
    const laps = merge(plain, [300]);
    expect(laps.map((l) => [l.start_index, l.end_index, l.trigger])).toEqual([
      [0, 301, 'added'],
      [300, plain.sampleCount, ''],
    ]);
  });

  it('a two-sample implicit lap cannot be split; the uncovered tail is a lap only once split', () => {
    const laps: Lap[] = [{ ...device[0], start_index: 0, end_index: 10 }];
    const d = { ...data, meta: { ...data.meta, laps } };
    expect(lapEnds(laps, n)).toEqual(new Set([0, 9, n - 1]));
    expect(merge(d, [500]).map((l) => [l.start_index, l.end_index, l.trigger])).toEqual([
      [0, 10, device[0].trigger],
      [9, 501, 'added'],
      [500, n, ''],
    ]);
  });

  it('pool: a sample snaps to the nearest length end, ties to the earlier; outside a pool session it stays', () => {
    const pool = golden('pool-swim');
    const lengths = pool.meta.lengths;
    const end = lengths[3].end_index - 1;
    expect(snapLapBoundary(pool.meta, pool.time, end + 1)).toBe(end);
    expect(snapLapBoundary(pool.meta, pool.time, end)).toBe(end);
    expect(snapLapBoundary(data.meta, data.time, 123)).toBe(123);
  });
});

describe('F-66 RA-3, RA-5: the browser removal agrees with the Go build (auto-laps.json)', () => {
  for (const c of loadAutoLaps()) {
    it(c.name, () => {
      const data = golden(c.golden);
      const stats = createRangeStats(data);
      let laps = removeAutoLaps(data, stats);
      if (c.at.length) {
        const base = { ...data, meta: { ...data.meta, laps } };
        laps = addLaps(base, stats, c.at.map((i) => snapLapBoundary(base.meta, data.time, i)), 0, data.sampleCount - 1);
      }
      const want = c.laps as Lap[];
      expect(shape(laps)).toEqual(shape(want));
      laps.forEach((l, i) => {
        expect(Object.keys(l.totals).sort()).toEqual(Object.keys(want[i].totals).sort());
        for (const [k, v] of Object.entries(want[i].totals)) expect(Math.abs(l.totals[k as keyof Lap['totals']]! - v), `${c.name} lap ${i + 1} ${k}`).toBeLessThanOrEqual(0.25 + 2e-3 * Math.abs(v));
      });
    });
  }
});

describe('F-66 rules', () => {
  const data = golden('ride-lap-triggers');
  const stats = createRangeStats(data);
  const lap = (i: number, trigger: string, stroke = ''): Lap => ({ ...data.meta.laps[0], start: i * 10, end: i * 10 + 10, start_index: i * 10, end_index: i * 10 + 11, trigger, stroke });
  const mk = (laps: Lap[], sessions = data.meta.sessions) => ({ ...data, meta: { ...data.meta, laps, sessions } });

  it('auto triggers are distance, time, position and none', () => {
    expect(['distance', 'time', 'position', ''].every(isAutoLap)).toBe(true);
    expect(['button', 'stop', 'equipment', 'added'].some(isAutoLap)).toBe(false);
  });

  it('merges a run into the lap after it: first start, last end, last trigger, range values; others stay', () => {
    const d = mk([lap(0, 'button'), lap(1, 'distance'), lap(2, 'time'), lap(3, 'stop')]);
    const out = removeAutoLaps(d, stats);
    expect(out.map((l) => [l.start_index, l.end_index, l.trigger])).toEqual([
      [0, 11, 'button'],
      [10, 41, 'stop'],
    ]);
    expect(out[0]).toBe(d.meta.laps[0]);
    expect(out[1].totals.elapsed_time).toBe(30);
  });

  it('nothing removable (no auto-lap with a next lap): the same laps', () => {
    const d = mk([lap(0, 'button'), lap(1, 'stop'), lap(2, 'distance')]);
    expect(removeAutoLaps(d, stats)).toEqual(d.meta.laps);
    expect(removableBoundaries(d.meta, d.time, d.sampleCount, 0, d.sampleCount - 1)).toBe(0);
  });

  it('one lap left after a removal: none (splits only)', () => {
    expect(removeAutoLaps(mk([lap(0, 'distance'), lap(1, 'distance'), lap(2, 'time')]), stats)).toEqual([]);
  });

  it('stroke: common, ignoring rest laps; different strokes give mixed', () => {
    expect(removeAutoLaps(mk([lap(0, 'distance', 'freestyle'), lap(1, 'distance', ''), lap(2, 'button', 'freestyle'), lap(3, 'stop')]), stats)[0].stroke).toBe('freestyle');
    expect(removeAutoLaps(mk([lap(0, 'distance', 'freestyle'), lap(1, 'button', 'backstroke'), lap(2, 'stop')]), stats)[0].stroke).toBe('mixed');
  });

  it('a leg boundary stays: no merge across two sessions', () => {
    const s = data.meta.sessions[0];
    const sessions = [
      { ...s, start: 0, end: 15 },
      { ...s, start: 15, end: 100 },
    ];
    const d = mk([lap(0, 'distance'), lap(1, 'distance'), lap(2, 'time'), lap(3, 'stop')], sessions);
    expect(removeAutoLaps(d, stats).map((l) => [l.start_index, l.end_index])).toEqual([
      [0, 21],
      [20, 41],
    ]);
    expect(removableBoundaries(d.meta, d.time, d.sampleCount, 0, 1000)).toBe(2);
  });

  it('count: only boundaries strictly inside the window', () => {
    const d = mk([lap(0, 'distance'), lap(1, 'distance'), lap(2, 'time'), lap(3, 'stop')]);
    const c = (lo: number, hi: number) => removableBoundaries(d.meta, d.time, d.sampleCount, lo, hi);
    expect([c(0, 1000), c(10, 1000), c(10, 31), c(10, 30)]).toEqual([3, 2, 2, 1]);
  });
});
