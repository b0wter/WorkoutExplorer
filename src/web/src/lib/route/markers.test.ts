// Start, finish and lap markers (F-06 design, Tests; AC 3, AC 6).

import { describe, expect, it } from 'vitest';
import { buildRoute, routeMarkers, type RouteMarker } from './index';
import type { WorkoutData } from '../datafile';
import { golden, lap, positioned } from './testing/fixtures';

const N = Number.NaN;

function markers(data: WorkoutData): RouteMarker[] {
  const r = buildRoute(data);
  if (r === null) throw new Error('expected a route');
  return routeMarkers(data, r);
}

describe('routeMarkers', () => {
  it('start = first positioned sample, finish = last', () => {
    const m = markers(positioned([N, 47.1, 47.2, 47.3, N], [N, 8.1, 8.2, 8.3, N]));
    expect(m[0]).toEqual({ kind: 'start', index: 1, lon: 8.1, lat: 47.1 });
    expect(m[m.length - 1]).toEqual({ kind: 'finish', index: 3, lon: 8.3, lat: 47.3 });
  });

  it('golden ride-12h (AC 3): lap markers 2…24 at exactly the start_index position', () => {
    const data = golden('ride-12h');
    const p = data.position;
    if (!p) throw new Error('no position');
    const laps = data.meta.laps;
    expect(laps).toHaveLength(24);
    const m = markers(data);
    const lapMarkers = m.filter((x) => x.kind === 'lap');
    expect(lapMarkers).toHaveLength(laps.length - 1);
    lapMarkers.forEach((x, k) => {
      const s = laps[k + 1].start_index;
      expect(x.lap).toBe(k + 2);
      expect(x.index).toBe(s);
      expect(x.lat).toBe(p.latitude[s]);
      expect(x.lon).toBe(p.longitude[s]);
    });
    expect(m[0].kind).toBe('start');
    expect(m[m.length - 1].kind).toBe('finish');
    expect(m[m.length - 1].index).toBe(data.sampleCount - 1);
  });

  it('no marker for a lap whose start sample has no position, or whose start_index is N', () => {
    const lat = [47.0, 47.1, N, 47.3, 47.4];
    const data = positioned(lat, lat, [lap(0), lap(2), lap(3), lap(5)]);
    const m = markers(data);
    expect(m.map((x) => [x.kind, x.index, x.lap])).toEqual([
      ['start', 0, undefined],
      ['lap', 3, 3],
      ['finish', 4, undefined],
    ]);
  });

  it('brick-run-bike: the lap marker at the session boundary', () => {
    const data = golden('brick-run-bike');
    const sessions = data.meta.sessions;
    expect(sessions).toHaveLength(2);
    const laps = markers(data).filter((x) => x.kind === 'lap');
    expect(laps.map((x) => [x.index, x.lap])).toEqual([[sessions[1].start_index, 2]]);
  });

  it('one positioned sample: start and finish at the same point', () => {
    const m = markers(positioned([N, 47, N], [N, 8, N], [lap(0), lap(2)]));
    expect(m).toEqual([
      { kind: 'start', index: 1, lon: 8, lat: 47 },
      { kind: 'finish', index: 1, lon: 8, lat: 47 },
    ]);
  });

  it('F-42 MS-8: a leg marker at a positioned leg start replaces the lap marker there; none without a position', () => {
    const lat = [47.0, 47.1, N, 47.3, 47.4];
    const data = positioned(lat, lat, [lap(0), lap(1), lap(3)]);
    const r = buildRoute(data);
    if (r === null) throw new Error('expected a route');
    const m = routeMarkers(data, r, [{ name: 'T1', start: 2 }, { name: 'Run', start: 3 }]);
    expect(m.map((x) => [x.kind, x.index, x.lap ?? x.name])).toEqual([
      ['start', 0, undefined],
      ['lap', 1, 2],
      ['leg', 3, 'Run'],
      ['finish', 4, undefined],
    ]);
  });
});
