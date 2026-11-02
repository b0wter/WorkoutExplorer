// Nearest positioned sample (F-06 design, Tests; ADR-015 decision 5; review decision 5).

import { describe, expect, it } from 'vitest';
import { buildRoute, createRouteIndex, mercatorX, mercatorY, type Route } from './index';
import type { WorkoutData } from '../datafile';
import { golden, positioned, prng } from './testing/fixtures';

const N = Number.NaN;

function route(data: WorkoutData): Route {
  const r = buildRoute(data);
  if (r === null) throw new Error('expected a route');
  return r;
}

/** The rule of RouteIndex.nearest, by brute force over every positioned sample. */
function reference(r: Route, x: number, y: number, radius: number, tie: number): number {
  const r2 = radius * radius;
  let best = Infinity;
  for (let k = 0; k < r.indices.length; k++) {
    const dx = r.mx[k] - x;
    const dy = r.my[k] - y;
    const d2 = dx * dx + dy * dy;
    if (d2 <= r2 && d2 < best) best = d2;
  }
  if (best === Infinity) return -1;
  const limit = Math.sqrt(best) + tie;
  const threshold = Math.max(best, Math.min(limit * limit, r2));
  for (let k = 0; k < r.indices.length; k++) {
    const dx = r.mx[k] - x;
    const dy = r.my[k] - y;
    if (dx * dx + dy * dy <= threshold) return r.indices[k];
  }
  return -1;
}

/** Mercator units per CSS px at a zoom (MapLibre's 512 px world). */
const px = (zoom: number): number => 1 / (512 * 2 ** zoom);

describe('createRouteIndex / nearest', () => {
  it('1,000 seeded random queries on ride-12h (repeated loops) equal the brute-force reference', () => {
    const r = route(golden('ride-12h'));
    const index = createRouteIndex(r);
    const rand = prng(20261012);
    let [minX, maxX, minY, maxY] = [Infinity, -Infinity, Infinity, -Infinity];
    for (let k = 0; k < r.mx.length; k++) {
      minX = Math.min(minX, r.mx[k]);
      maxX = Math.max(maxX, r.mx[k]);
      minY = Math.min(minY, r.my[k]);
      maxY = Math.max(maxY, r.my[k]);
    }
    const spanX = maxX - minX;
    const spanY = maxY - minY;
    let hits = 0;
    let earlier = 0;
    for (let q = 0; q < 1000; q++) {
      // Half the queries near a random sample (so they hit), half anywhere around the box.
      let x: number;
      let y: number;
      if (q % 2 === 0) {
        const k = Math.floor(rand() * r.mx.length);
        x = r.mx[k] + (rand() - 0.5) * 40 * px(14);
        y = r.my[k] + (rand() - 0.5) * 40 * px(14);
      } else {
        x = minX - 0.1 * spanX + rand() * 1.2 * spanX;
        y = minY - 0.1 * spanY + rand() * 1.2 * spanY;
      }
      const zoom = 8 + rand() * 8;
      const radius = (4 + rand() * 60) * px(zoom);
      const tie = (rand() < 0.2 ? 0.01 : rand() * 6) * px(zoom);
      const got = index.nearest(x, y, radius, tie);
      expect(got, `query ${q}`).toBe(reference(r, x, y, radius, tie));
      if (got >= 0) {
        hits++;
        if (got !== reference(r, x, y, radius, 1e-300)) earlier++;
      }
    }
    // The queries exercise both outcomes and the tie rule.
    expect(hits).toBeGreaterThan(400);
    expect(hits).toBeLessThan(1000);
    expect(earlier).toBeGreaterThan(0);
  });

  const unit = 1e-6; // Mercator units for the hand cases
  function line(xs: readonly number[], ys: readonly number[]): { r: Route; nearest: (x: number, y: number, radius: number, tie: number) => number } {
    // Positions chosen so that Mercator coordinates are exactly what the cases need: build
    // the Route directly (indices, mx, my), bypassing degrees.
    const r: Route = {
      indices: Uint32Array.from(xs.map((_, k) => k * 2 + 1)), // sample indices 1, 3, 5, … (gaps between)
      mx: Float64Array.from(xs),
      my: Float64Array.from(ys),
      bounds: [0, 0, 0, 0],
    };
    const index = createRouteIndex(r);
    return { r, nearest: (x, y, radius, tie) => index.nearest(x, y, radius, tie) };
  }

  it('nothing within the radius → −1', () => {
    const { nearest } = line([0.5, 0.5 + 10 * unit], [0.5, 0.5]);
    expect(nearest(0.5, 0.5 + 5 * unit, 4 * unit, unit)).toBe(-1);
    expect(nearest(0.9, 0.9, 4 * unit, unit)).toBe(-1);
  });

  it('a query exactly at a sample → that sample (its sample index, not its route position)', () => {
    const { nearest } = line([0.5, 0.5 + 10 * unit, 0.5 + 20 * unit], [0.5, 0.5, 0.5]);
    expect(nearest(0.5 + 10 * unit, 0.5, 4 * unit, 0.1 * unit)).toBe(3);
    expect(nearest(0.5 + 20 * unit, 0.5, 4 * unit, 0.1 * unit)).toBe(5);
  });

  it('two samples at identical coordinates → the lower index (decision 5)', () => {
    const { nearest } = line([0.5, 0.5 + 50 * unit, 0.5], [0.5, 0.5, 0.5]);
    expect(nearest(0.5 + unit, 0.5, 10 * unit, 1e-12)).toBe(1);
  });

  it('a later sample nearer by less than tie → the earlier one; by more than tie → the nearer one', () => {
    // Earlier sample at distance 2, later at distance 1 from the query.
    const { nearest } = line([0.5 - 2 * unit, 0.5 + unit], [0.5, 0.5]);
    expect(nearest(0.5, 0.5, 10 * unit, 1.5 * unit)).toBe(1);
    expect(nearest(0.5, 0.5, 10 * unit, 0.5 * unit)).toBe(3);
  });

  it('a candidate within tie of the best but outside the radius is never returned', () => {
    // Later sample at 1 (inside radius 2.5), earlier at 3 (within tie 3, outside the radius).
    const { nearest } = line([0.5 + 3 * unit, 0.5 - unit], [0.5, 0.5]);
    expect(nearest(0.5, 0.5, 2.5 * unit, 3 * unit)).toBe(3);
  });

  it('samples without position are never returned (sample indices of positioned samples)', () => {
    const lat = [N, 47, N, N, 47.001, N];
    const data = positioned(lat, [N, 8, N, N, 8.001, N]);
    const index = createRouteIndex(route(data));
    const x = mercatorX(8.001);
    const y = mercatorY(47.001);
    expect(index.nearest(x, y, 1e-3, 1e-12)).toBe(4);
    expect(index.nearest(mercatorX(8), mercatorY(47), 1e-3, 1e-12)).toBe(1);
  });

  it('one positioned sample', () => {
    const data = positioned([N, 47, N], [N, 8, N]);
    const index = createRouteIndex(route(data));
    expect(index.nearest(mercatorX(8), mercatorY(47), 1e-6, 1e-9)).toBe(1);
    expect(index.nearest(mercatorX(8) + 1e-5, mercatorY(47), 1e-6, 1e-9)).toBe(-1);
  });

  it('zero-area bounding box: every sample at one point → the lowest index', () => {
    const data = positioned([N, 47, 47, 47], [N, 8, 8, 8]);
    const index = createRouteIndex(route(data));
    expect(index.nearest(mercatorX(8) + 1e-7, mercatorY(47), 1e-6, 1e-12)).toBe(1);
  });

  it('a straight north–south line (zero width) and a radius covering the whole route', () => {
    const n = 1000;
    const lat = Array.from({ length: n }, (_, i) => 47 + i * 1e-4);
    const data = positioned(lat, lat.map(() => 8));
    const r = route(data);
    const index = createRouteIndex(r);
    expect(index.nearest(mercatorX(8), mercatorY(47.05), 1e-6, 1e-12)).toBe(500);
    // Radius covering everything: the nearest one with a tiny tie; the earliest with a huge tie.
    expect(index.nearest(mercatorX(8), mercatorY(47.05), 1, 1e-12)).toBe(500);
    expect(index.nearest(mercatorX(8), mercatorY(47.05), 1, 1)).toBe(0);
    for (let q = 0; q < 50; q++) {
      const y = mercatorY(47 + q * 2.1e-3);
      expect(index.nearest(mercatorX(8) + 1e-7, y, 1e-5, 1e-8)).toBe(reference(r, mercatorX(8) + 1e-7, y, 1e-5, 1e-8));
    }
  });
});
