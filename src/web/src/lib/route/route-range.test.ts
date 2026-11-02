// routeLinesIn (F-17 R-10, AC 3): the range highlight covers only positioned samples in [a, b],
// by F-06 D-7's segment and bridge rule.
import { describe, expect, it } from 'vitest';
import { buildRoute, routeLines, routeLinesIn, type Route } from './index';
import { golden, hidePositions, positioned } from './testing/fixtures';
import type { WorkoutData } from '../datafile';

const N = Number.NaN;
const routeOf = (d: WorkoutData): Route => {
  const r = buildRoute(d);
  if (!r) throw new Error('no route');
  return r;
};
const vertices = (l: ReturnType<typeof routeLinesIn>): [number, number][] => [...l.recorded.flat(), ...l.bridges.flat()];

describe('routeLinesIn', () => {
  it('the whole range equals routeLines', () => {
    for (const name of ['gps-dropout', 'pause-hours', 'ride-30min']) {
      const d = golden(name);
      const r = routeOf(d);
      expect(routeLinesIn(d, r, 0, d.sampleCount - 1)).toEqual(routeLines(d, r));
    }
  });

  it('a range overlapping a hidden start gives vertices only from the visible part', () => {
    const base = golden('ride-30min');
    const d = hidePositions(base, (i) => i < 300);
    const lines = routeLinesIn(d, routeOf(d), 100, 600);
    const lat = d.position?.latitude as Float64Array;
    const lon = d.position?.longitude as Float64Array;
    const visible = new Set<string>();
    for (let i = 300; i <= 600; i++) visible.add(lon[i] + ',' + lat[i]);
    const vs = vertices(lines);
    expect(vs.length).toBeGreaterThan(0);
    for (const [x, y] of vs) expect(visible.has(x + ',' + y)).toBe(true);
    expect(lines.recorded[0][0]).toEqual([lon[300], lat[300]]);
    expect(lines.recorded.at(-1)?.at(-1)).toEqual([lon[600], lat[600]]);
  });

  it('a range inside a hidden section, or with one positioned sample, is empty', () => {
    const d = positioned([N, N, N, 47, 47.1, N], [N, N, N, 8, 8.1, N]);
    const r = routeOf(d);
    expect(routeLinesIn(d, r, 0, 2)).toEqual({ recorded: [], bridges: [] });
    expect(routeLinesIn(d, r, 0, 3)).toEqual({ recorded: [], bridges: [] });
    expect(routeLinesIn(d, r, 3, 5)).toEqual({ recorded: [[[8, 47], [8.1, 47.1]]], bridges: [] });
  });

  it('a dropout inside the range is a bridge', () => {
    const d = positioned([47, 47.1, N, 47.3, 47.4], [8, 8.1, N, 8.3, 8.4]);
    const lines = routeLinesIn(d, routeOf(d), 1, 4);
    expect(lines.bridges).toEqual([[[8.1, 47.1], [8.3, 47.3]]]);
    expect(lines.recorded).toEqual([[[8.3, 47.3], [8.4, 47.4]]]);
  });
});
