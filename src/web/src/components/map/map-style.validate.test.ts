// Style-spec validation of our styles in CI (F-06 design v1.5, Tests; ADR-013 rev. 3
// decision 7; QA-06-r3-2). Production skips MapLibre's runtime validation
// (`validateStyle` is dev-only), so this test checks every style map-style.ts produces with
// the validator of the exact style-spec version the locked maplibre-gl uses.
//
// Rule for later features: every feature that adds or changes wx- sources or layers (F-14,
// F-17, F-25, F-26) adds its styles here.
//
// Node; synthetic data only. The remote style is a small stand-in written here; liberty is
// not committed.

import { describe, expect, it } from 'vitest';
import { validateStyleMin } from '@maplibre/maplibre-gl-style-spec';
import type { StyleSpecification } from 'maplibre-gl';
import { buildRoute, classLines, recordedEdgeEnds, routeLines, routeLinesIn, routeMarkers, type Route } from '../../lib/route';
import { CLASS_COUNT, classify, colourScale, metricValues } from '../../lib/colouring';
import type { WorkoutData } from '../../lib/datafile';
import { golden, positioned } from '../../lib/route/testing/fixtures';
import { COLOUR_SOURCE, DIVERGING, GLYPHS_URL, ROUTE_SOURCE, TILE_HOST, baseStyle, colourData, mergeStyle } from './map-style';

const N = Number.NaN;

function base(data: WorkoutData): StyleSpecification {
  const route: Route | null = buildRoute(data);
  if (route === null) throw new Error('expected a route');
  return baseStyle(routeLines(data, route), routeMarkers(data, route));
}

/** Validation messages (empty when the style is valid). */
function errors(style: StyleSpecification): string[] {
  return validateStyleMin(style).map((e) => e.message);
}

/** Kinds of the wx-route features, to show which case a style covers. */
function routeKinds(style: StyleSpecification): unknown[] {
  const src = style.sources[ROUTE_SOURCE];
  if (src === undefined) return [];
  if (src.type !== 'geojson' || typeof src.data !== 'object' || src.data.type !== 'FeatureCollection') throw new Error('unexpected wx-route');
  return src.data.features.map((f: { properties?: { kind?: unknown } | null }) => f.properties?.kind);
}

/** A stand-in for a remote style such as liberty: a vector source with a fill and a symbol
 *  layer, a raster source, a raster-dem source for terrain, a background, projection and terrain. */
function remote(): StyleSpecification {
  return {
    version: 8,
    glyphs: GLYPHS_URL,
    sources: {
      openmaptiles: { type: 'vector', url: `${TILE_HOST}/planet` },
      ne2_shaded: { type: 'raster', tiles: [`${TILE_HOST}/natural_earth/ne2sr/{z}/{x}/{y}.png`], tileSize: 256, maxzoom: 6 },
      dem: { type: 'raster-dem', tiles: [`${TILE_HOST}/dem/{z}/{x}/{y}.png`], tileSize: 256 },
    },
    projection: { type: 'globe' },
    terrain: { source: 'dem', exaggeration: 1 },
    layers: [
      { id: 'background', type: 'background', paint: { 'background-color': '#f8f4f0' } },
      { id: 'natural_earth', type: 'raster', source: 'ne2_shaded', paint: { 'raster-opacity': 0.5 } },
      { id: 'water', type: 'fill', source: 'openmaptiles', 'source-layer': 'water', paint: { 'fill-color': '#a0c8f0' } },
      {
        id: 'place_label',
        type: 'symbol',
        source: 'openmaptiles',
        'source-layer': 'place',
        layout: { 'text-field': ['get', 'name'], 'text-font': ['Noto Sans Regular'], 'text-size': 12 },
        paint: { 'text-color': '#333333' },
      },
    ],
  };
}

describe('style-spec validation (QA-06-r3-2)', () => {
  const cases: [string, () => WorkoutData, unknown[]][] = [
    ['ride-12h (recorded only, 23 lap markers)', () => golden('ride-12h'), ['recorded']],
    ['gps-dropout (recorded and bridge features)', () => golden('gps-dropout'), ['recorded', 'bridge']],
    ['pause-hours (a bridge across a time gap)', () => golden('pause-hours'), ['recorded', 'bridge']],
    ['M = 1 (no route source)', () => positioned([N, 47, N], [N, 8, N]), []],
    ['a route with no lap markers', () => positioned([47, 47.01, 47.02], [8, 8.01, 8.02]), ['recorded']],
  ];

  for (const [name, data, kinds] of cases) {
    it(`baseStyle: ${name}`, () => {
      const style = base(data());
      expect(routeKinds(style)).toEqual(kinds);
      expect(errors(style)).toEqual([]);
    });
  }

  it('baseStyle with a range highlight (F-17 R-10): recorded and bridge range features, also merged', () => {
    const data = golden('gps-dropout');
    const route = buildRoute(data);
    if (route === null) throw new Error('expected a route');
    const style = baseStyle(routeLines(data, route), routeMarkers(data, route), undefined, routeLinesIn(data, route, 0, data.sampleCount - 1));
    const range = style.sources['wx-range'];
    if (range?.type !== 'geojson' || typeof range.data !== 'object' || range.data.type !== 'FeatureCollection') throw new Error('no wx-range');
    expect(range.data.features.map((f: { properties?: { kind?: unknown } | null }) => f.properties?.kind)).toEqual(['recorded', 'bridge']);
    expect(errors(style)).toEqual([]);
    expect(errors(mergeStyle(style, remote()))).toEqual([]);
  });

  it('F-25: wx-colour with class features (sequential and diverging), also merged', () => {
    const data = golden('gps-dropout');
    const route = buildRoute(data);
    if (route === null) throw new Error('expected a route');
    const ends = recordedEdgeEnds(data, route);
    for (const [metric, offset] of [['speed', 0], ['gradient', DIVERGING]] as const) {
      const values = metricValues(data, metric);
      const classes = classify(values, colourScale(metric, values, ends, data.meta.sport));
      const style = base(data);
      style.sources[COLOUR_SOURCE] = { type: 'geojson', data: colourData(classLines(data, ends, classes, CLASS_COUNT), offset) as never };
      const src = style.sources[COLOUR_SOURCE];
      if (src?.type !== 'geojson' || typeof src.data !== 'object' || src.data.type !== 'FeatureCollection') throw new Error('no wx-colour');
      expect(src.data.features.length).toBeGreaterThan(1);
      expect(errors(style)).toEqual([]);
      expect(errors(mergeStyle(style, remote()))).toEqual([]);
    }
  });

  it('the case without lap markers really has none', () => {
    const style = base(positioned([47, 47.01, 47.02], [8, 8.01, 8.02]));
    const markers = style.sources['wx-markers'];
    if (markers?.type !== 'geojson' || typeof markers.data !== 'object' || markers.data.type !== 'FeatureCollection') throw new Error('no markers');
    expect(markers.data.features.map((f: { properties?: { kind?: unknown } | null }) => f.properties?.kind)).toEqual(['start', 'finish']);
  });

  it('the synthetic remote style is itself valid (so the merge test checks the merge)', () => {
    expect(errors(remote())).toEqual([]);
  });

  it('mergeStyle(baseStyle(…), remote): valid, for a route with bridges and for M = 1', () => {
    for (const data of [golden('gps-dropout'), positioned([N, 47, N], [N, 8, N])]) {
      const merged = mergeStyle(base(data), remote());
      expect(errors(merged)).toEqual([]);
    }
  });

  it('mergeStyle applied twice and with previous undefined: valid', () => {
    const once = mergeStyle(base(golden('gps-dropout')), remote());
    expect(errors(mergeStyle(once, once))).toEqual([]);
    expect(errors(mergeStyle(undefined, remote()))).toEqual([]);
  });

  it('the validator catches a broken layer (the test can fail)', () => {
    const style = base(golden('gps-dropout'));
    const broken: StyleSpecification = {
      ...style,
      layers: [...style.layers, { id: 'wx-broken', type: 'line', source: 'wx-missing', paint: { 'line-dasharray': 'dashed' } } as never],
    };
    expect(errors(broken).length).toBeGreaterThan(0);
  });
});
