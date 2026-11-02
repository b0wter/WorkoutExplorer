// Route geometry (F-06 design, Tests; AP-3, AC 4).

import { describe, expect, it } from 'vitest';
import type { WorkoutData } from '../datafile';
import { buildRoute, mercatorX, mercatorY, routeLines, type Route, type RouteLines } from './index';
import { golden, hidePositions, positioned, prng } from './testing/fixtures';

const N = Number.NaN;

function route(d: Parameters<typeof buildRoute>[0]): Route {
  const r = buildRoute(d);
  if (r === null) throw new Error('expected a route');
  return r;
}

describe('buildRoute', () => {
  it('indices are exactly the positioned samples, ascending; mx/my per sample; bounds brute force', () => {
    const lat = [N, 47.1, 47.2, N, N, 46.9, 47.05, N];
    const lon = [N, 8.3, 8.1, N, N, 8.6, 8.2, N];
    const data = positioned(lat, lon);
    const r = route(data);
    expect([...r.indices]).toEqual([1, 2, 5, 6]);
    r.indices.forEach((i, k) => {
      expect(r.mx[k]).toBe(mercatorX(lon[i]));
      expect(r.my[k]).toBe(mercatorY(lat[i]));
    });
    expect(r.bounds).toEqual([8.1, 46.9, 8.6, 47.2]);
  });

  it('golden ride-12h: every sample positioned, bounds equal the brute-force min/max', () => {
    const data = golden('ride-12h');
    const r = route(data);
    const p = data.position;
    if (!p) throw new Error('no position');
    expect(r.indices.length).toBe(data.sampleCount);
    for (let k = 1; k < r.indices.length; k++) expect(r.indices[k]).toBeGreaterThan(r.indices[k - 1]);
    const lats = [...p.latitude];
    const lons = [...p.longitude];
    expect(r.bounds).toEqual([Math.min(...lons), Math.min(...lats), Math.max(...lons), Math.max(...lats)]);
    expect(r.mx).toBeInstanceOf(Float64Array);
    expect(r.my).toBeInstanceOf(Float64Array);
  });

  it('null without a position stream and with every position missing', () => {
    expect(buildRoute(positioned(undefined))).toBeNull();
    expect(buildRoute(positioned([N, N]))).toBeNull();
  });
});

type LonLat = readonly [number, number];

/** data with gaps at the given sample indices (each starts a gap of ≥ 5 s); time made consistent. */
function withGaps(data: WorkoutData, gaps: readonly number[]): WorkoutData {
  const time = new Float64Array(data.sampleCount);
  const at = new Set(gaps);
  for (let i = 1; i < data.sampleCount; i++) time[i] = time[i - 1] + (at.has(i) ? 600 : 1);
  return { ...data, time, gaps: Uint32Array.from([...gaps].sort((a, b) => a - b)) };
}

/** The point of sample i, [lon, lat]. */
function pointOf(data: WorkoutData, i: number): LonLat {
  const p = data.position;
  if (!p) throw new Error('no position');
  return [p.longitude[i], p.latitude[i]];
}

/** Every edge of the recorded segments and every bridge, as "lonA,latA>lonB,latB" keys with a count. */
function edgeCounts(lines: RouteLines): { recorded: Map<string, number>; bridges: Map<string, number> } {
  const key = (a: LonLat, b: LonLat) => `${a[0]},${a[1]}>${b[0]},${b[1]}`;
  const add = (m: Map<string, number>, k: string) => m.set(k, (m.get(k) ?? 0) + 1);
  const recorded = new Map<string, number>();
  const bridges = new Map<string, number>();
  for (const seg of lines.recorded) for (let k = 1; k < seg.length; k++) add(recorded, key(seg[k - 1], seg[k]));
  for (const [a, b] of lines.bridges) add(bridges, key(a, b));
  return { recorded, bridges };
}

describe('routeLines', () => {
  it('[lon, lat] order (lat ≠ lon), no NaN', () => {
    const data = positioned([47.5, 47.25, N, 46.5], [8.25, 8.5, N, 9.75]);
    const lines = routeLines(data, route(data));
    expect(lines.recorded).toEqual([
      [
        [8.25, 47.5],
        [8.5, 47.25],
      ],
    ]);
    expect(lines.bridges).toEqual([
      [
        [8.5, 47.25],
        [9.75, 46.5],
      ],
    ]);
    const all = [...lines.recorded.flat(), ...lines.bridges.flat()];
    expect(all.every(([lo, la]) => Number.isFinite(lo) && Number.isFinite(la))).toBe(true);
  });

  it('partition property on seeded random data: each consecutive positioned pair is exactly one recorded edge or one bridge', () => {
    const rand = prng(0x06b1d9e5);
    for (let trial = 0; trial < 200; trial++) {
      const n = 2 + Math.floor(rand() * 300);
      const lat: number[] = [];
      const lon: number[] = [];
      let missing = 0;
      for (let i = 0; i < n; i++) {
        // Random runs of missing positions (GPS dropouts, hidden ends).
        if (missing === 0 && rand() < 0.04) missing = 1 + Math.floor(rand() * 12);
        if (missing > 0) {
          missing--;
          lat.push(N);
          lon.push(N);
        } else {
          // Unique coordinates per sample, lat ≠ lon, so edge keys identify the pair.
          lat.push(45 + i * 1e-4);
          lon.push(7 + i * 2e-4);
        }
      }
      if (lat.every((v) => Number.isNaN(v))) {
        lat[0] = 45;
        lon[0] = 7;
      }
      const gaps: number[] = [];
      for (let i = 1; i < n; i++) if (rand() < 0.03) gaps.push(i);
      const data = withGaps(positioned(lat, lon), gaps);
      const r = route(data);
      const lines = routeLines(data, r);

      for (const seg of lines.recorded) expect(seg.length).toBeGreaterThanOrEqual(2);
      for (const b of lines.bridges) expect(b).toHaveLength(2);

      const { recorded, bridges } = edgeCounts(lines);
      const gapSet = new Set(gaps);
      let expectedBridges = 0;
      for (let k = 1; k < r.indices.length; k++) {
        const a = r.indices[k - 1];
        const b = r.indices[k];
        const key = `${pointOf(data, a)[0]},${pointOf(data, a)[1]}>${pointOf(data, b)[0]},${pointOf(data, b)[1]}`;
        const isBridge = b - a > 1 || gapSet.has(b);
        if (isBridge) expectedBridges++;
        expect(bridges.get(key) ?? 0).toBe(isBridge ? 1 : 0);
        expect(recorded.get(key) ?? 0).toBe(isBridge ? 0 : 1);
      }
      // Nothing beyond the consecutive pairs.
      expect(lines.bridges).toHaveLength(expectedBridges);
      const edges = lines.recorded.reduce((s, seg) => s + seg.length - 1, 0);
      expect(edges + expectedBridges).toBe(r.indices.length - 1);
    }
  });

  it('golden gps-dropout (AC 4, GPS dropout): one bridge 1,499 → 1,800; recorded segments of 1,500 and 1,801 points', () => {
    const data = golden('gps-dropout');
    const p = data.position;
    if (!p) throw new Error('no position');
    // The dropout: samples 1,500–1,799 (25:00–30:00) without position.
    expect(Number.isNaN(p.latitude[1500])).toBe(true);
    expect(Number.isNaN(p.latitude[1799])).toBe(true);
    expect(data.time[1500]).toBe(1500);
    expect(data.time[1799]).toBe(1799);
    const lines = routeLines(data, route(data));
    expect(lines.bridges).toEqual([[pointOf(data, 1499), pointOf(data, 1800)]]);
    expect(lines.recorded.map((s) => s.length)).toEqual([1500, 1801]);
    expect(lines.recorded[0][0]).toEqual(pointOf(data, 0));
    expect(lines.recorded[0][1499]).toEqual(pointOf(data, 1499));
    expect(lines.recorded[1][0]).toEqual(pointOf(data, 1800));
    expect(lines.recorded[1][1800]).toEqual(pointOf(data, data.sampleCount - 1));
  });

  it('golden pause-hours (AC 4, time gap): exactly one bridge, across the gap', () => {
    const data = golden('pause-hours');
    expect(data.gaps).toHaveLength(1);
    const g = data.gaps[0];
    expect(data.time[g] - data.time[g - 1]).toBeGreaterThanOrEqual(3 * 3600);
    const r = route(data);
    // Both ends positioned and adjacent: the bridge comes from the time gap alone, not a dropout.
    const k = r.indices.indexOf(g);
    expect(k).toBeGreaterThan(0);
    expect(r.indices[k - 1]).toBe(g - 1);
    const lines = routeLines(data, r);
    expect(lines.bridges).toEqual([[pointOf(data, g - 1), pointOf(data, g)]]);
    expect(lines.recorded.reduce((s, seg) => s + seg.length - 1, 0)).toBe(r.indices.length - 2);
  });

  it('golden ride-12h: no bridge, one recorded segment of 43,200 points', () => {
    const data = golden('ride-12h');
    const lines = routeLines(data, route(data));
    expect(lines.bridges).toEqual([]);
    expect(lines.recorded).toHaveLength(1);
    expect(lines.recorded[0]).toHaveLength(43_200);
  });

  it('a lone positioned sample between two missing runs: two bridges, in no recorded segment', () => {
    const data = positioned([47.0, 47.1, N, N, 47.2, N, 47.3, 47.4], [8.0, 8.1, N, N, 8.2, N, 8.3, 8.4]);
    const lines = routeLines(data, route(data));
    expect(lines.bridges).toEqual([
      [pointOf(data, 1), pointOf(data, 4)],
      [pointOf(data, 4), pointOf(data, 6)],
    ]);
    expect(lines.recorded).toEqual([
      [pointOf(data, 0), pointOf(data, 1)],
      [pointOf(data, 6), pointOf(data, 7)],
    ]);
    expect(lines.recorded.flat()).not.toContainEqual(pointOf(data, 4));
  });

  it('two adjacent positioned samples across a gap: one bridge, no recorded segment', () => {
    const data = withGaps(positioned([47.0, 47.1], [8.0, 8.1]), [1]);
    const lines = routeLines(data, route(data));
    expect(lines.bridges).toEqual([[pointOf(data, 0), pointOf(data, 1)]]);
    expect(lines.recorded).toEqual([]);
  });

  it('a gap inside a missing run gives one bridge, not two', () => {
    const data = withGaps(positioned([47.0, 47.1, N, 47.3, 47.4], [8.0, 8.1, N, 8.3, 8.4]), [2]);
    const lines = routeLines(data, route(data));
    expect(lines.bridges).toEqual([[pointOf(data, 1), pointOf(data, 3)]]);
    expect(lines.recorded).toHaveLength(2);
  });

  it('hidden ends: no bridge at either end; the line starts and ends at the visible part', () => {
    const base = golden('ride-4h');
    const k = 300;
    const data = hidePositions(base, (i) => i < k || i >= base.sampleCount - k);
    const lines = routeLines(data, route(data));
    expect(lines.bridges).toEqual([]);
    expect(lines.recorded).toHaveLength(1);
    const seg = lines.recorded[0];
    expect(seg).toHaveLength(base.sampleCount - 2 * k);
    expect(seg[0]).toEqual(pointOf(base, k));
    expect(seg[seg.length - 1]).toEqual(pointOf(base, base.sampleCount - k - 1));
  });

  it('M = 1: no recorded segment and no bridge', () => {
    const data = positioned([N, 47, N], [N, 8, N]);
    expect(routeLines(data, route(data))).toEqual({ recorded: [], bridges: [] });
  });

  it('M = 2 adjacent: one recorded segment of two points', () => {
    const data = positioned([N, 47, 47.1, N], [N, 8, 8.1, N]);
    expect(routeLines(data, route(data))).toEqual({ recorded: [[pointOf(data, 1), pointOf(data, 2)]], bridges: [] });
  });
});
