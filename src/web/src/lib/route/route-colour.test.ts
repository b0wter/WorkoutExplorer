// Recorded edges by colour class (F-25 design C-2, Verification AC 6, AC 7; ADR-037): every coloured edge is a
// recorded edge, no vertex from a sample without a position, bridges never coloured.

import { describe, expect, it } from 'vitest';
import type { WorkoutData } from '../datafile';
import { buildRoute, classLines, recordedEdgeEnds, routeLines, type Route } from './index';
import { golden, hidePositions, positioned } from './testing/fixtures';

const N = Number.NaN;

function route(d: WorkoutData): Route {
  const r = buildRoute(d);
  if (r === null) throw new Error('expected a route');
  return r;
}

/** Every edge of routeLines' recorded runs, as 'lon,lat→lon,lat'. */
function recordedEdges(d: WorkoutData): Set<string> {
  const out = new Set<string>();
  for (const run of routeLines(d, route(d)).recorded) for (let k = 1; k < run.length; k++) out.add(run[k - 1] + '→' + run[k]);
  return out;
}

const cases: [string, () => WorkoutData][] = [
  ['gps-dropout', () => golden('gps-dropout')],
  ['pause-hours', () => golden('pause-hours')],
  ['hidden ends', () => hidePositions(golden('ride-30min'), (i) => i < 300 || i > 1500)],
];

describe('recordedEdgeEnds', () => {
  for (const [name, data] of cases) {
    it(`${name}: exactly the recorded edges of routeLines`, () => {
      const d = data();
      const ends = recordedEdgeEnds(d, route(d));
      const lat = d.position!.latitude;
      const lon = d.position!.longitude;
      const got = new Set(Array.from(ends, (i) => [lon[i - 1], lat[i - 1]] + '→' + [lon[i], lat[i]]));
      expect(got).toEqual(recordedEdges(d));
      expect(got.size).toBe(ends.length);
    });
  }
  it('a gap start and a dropout end no edge', () => {
    const d = { ...positioned([47, 47.001, N, 47.003, 47.004, 47.005]), gaps: Uint32Array.from([4]) };
    expect(Array.from(recordedEdgeEnds(d, route(d)))).toEqual([1, 5]);
  });
});

describe('classLines (partition property)', () => {
  for (const [name, data] of cases) {
    it(`${name}: coloured edges ⊆ recorded edges; edges without a class left out; no unpositioned vertex`, () => {
      const d = data();
      const ends = recordedEdgeEnds(d, route(d));
      // A class per sample that changes often, with gaps of NO_CLASS (255).
      const classes = new Uint8Array(d.sampleCount).map((_, i) => (i % 97 < 10 ? 255 : (i >> 6) % 7));
      const lines = classLines(d, ends, classes, 7);
      const recorded = recordedEdges(d);
      let edges = 0;
      for (const [c, runs] of lines.entries()) {
        for (const run of runs) {
          expect(run.length).toBeGreaterThanOrEqual(2);
          for (const p of run) expect(Number.isFinite(p[0]) && Number.isFinite(p[1])).toBe(true);
          for (let k = 1; k < run.length; k++) {
            expect(recorded.has(run[k - 1] + '→' + run[k])).toBe(true);
            edges++;
          }
          expect(c).toBeLessThan(7);
        }
      }
      let expected = 0;
      for (const i of ends) if (classes[i] < 7) expected++;
      expect(edges).toBe(expected);
    });
  }
  it('consecutive edges of one class merge into one line; the class is the end sample’s', () => {
    const d = positioned([47, 47.001, 47.002, 47.003, 47.004], [8, 8, 8, 8, 8]);
    const ends = recordedEdgeEnds(d, route(d));
    const lines = classLines(d, ends, Uint8Array.from([9, 2, 2, 255, 4]), 7);
    expect(lines[2]).toEqual([[[8, 47], [8, 47.001], [8, 47.002]]]);
    expect(lines[4]).toEqual([[[8, 47.003], [8, 47.004]]]);
    expect(lines.filter((l) => l.length > 0)).toHaveLength(2);
  });
});
