import { describe, expect, it } from 'vitest';
import { decodeDataFile, type Lap, type WorkoutData } from '../datafile';
import { createRangeStats } from '../range';
import { loadAddedLaps, loadGolden } from '../../../test-support/golden.mjs';
import { addLaps, lapEnds, snapLapBoundary } from '.';

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
