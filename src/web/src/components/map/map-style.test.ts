// Base style and style merge (F-06 design, Tests; ADR-013 decision 2). Node: map-style.ts
// imports only types from maplibre-gl, so nothing of MapLibre runs here.

import { describe, expect, it } from 'vitest';
import type { LayerSpecification, StyleSpecification } from 'maplibre-gl';
import { buildRoute, routeLines, routeMarkers, type RouteLines, type RouteMarker } from '../../lib/route';
import { golden } from '../../lib/route/testing/fixtures';
import {
  CLASS_COLOURS,
  COLOUR_SOURCE,
  DIVERGING,
  GLYPHS_URL,
  MAP_COLOURS,
  MARKER_SOURCE,
  MAX_MAP_PIXEL_RATIO,
  OPENFREEMAP_STYLE_URL,
  HIDDEN_SOURCE,
  RANGE_BLUES,
  RANGE_SOURCE,
  ROUTE_SOURCE,
  TILE_HOST,
  WX_LAYER_IDS,
  baseStyle,
  classColourExpression,
  colourData,
  isStyle,
  linesData,
  mapPixelRatio,
  mergeStyle,
  rangeData,
} from './map-style';

const markers: RouteMarker[] = [
  { kind: 'start', index: 0, lon: 8, lat: 47 },
  { kind: 'lap', index: 10, lon: 8.1, lat: 47.1, lap: 2 },
  { kind: 'finish', index: 20, lon: 8.2, lat: 47.2 },
];
const recorded: [number, number][][] = [
  [
    [8, 47],
    [8.1, 47.1],
  ],
  [
    [8.15, 47.15],
    [8.2, 47.2],
  ],
];
const bridges: [[number, number], [number, number]][] = [
  [
    [8.1, 47.1],
    [8.15, 47.15],
  ],
];
/** Recorded segments and one bridge (a GPS dropout). */
const lines: RouteLines = { recorded, bridges };
const none: RouteLines = { recorded: [], bridges: [] };

/** A stand-in for liberty: its two sources (vector and raster), a background and two layers. */
function remote(extra: Partial<StyleSpecification> = {}): StyleSpecification {
  return {
    version: 8,
    sprite: `${TILE_HOST}/sprites/ofm_f384/ofm`,
    glyphs: GLYPHS_URL,
    sources: {
      openmaptiles: { type: 'vector', url: `${TILE_HOST}/planet` },
      ne2_shaded: { type: 'raster', tiles: [`${TILE_HOST}/natural_earth/ne2sr/{z}/{x}/{y}.png`], tileSize: 256, maxzoom: 6 },
    },
    layers: [
      { id: 'background', type: 'background', paint: { 'background-color': '#f8f4f0' } },
      { id: 'natural_earth', type: 'raster', source: 'ne2_shaded' },
      { id: 'water', type: 'fill', source: 'openmaptiles', 'source-layer': 'water' },
    ],
    ...extra,
  };
}

const ids = (layers: readonly LayerSpecification[]) => layers.map((l) => l.id);

describe('constants', () => {
  it('liberty on the tile host; glyphs on the tile host', () => {
    expect(OPENFREEMAP_STYLE_URL).toBe('https://tiles.openfreemap.org/styles/liberty');
    expect(TILE_HOST).toBe('https://tiles.openfreemap.org');
    expect(GLYPHS_URL.startsWith(`${TILE_HOST}/`)).toBe(true);
    expect(MAP_COLOURS.route).toBe('#c2187a');
  });

  it('MapLibre pixel ratio capped at 2 (v1.4)', () => {
    expect(MAX_MAP_PIXEL_RATIO).toBe(2);
  });
});

describe('baseStyle', () => {
  it('version 8, glyphs, sources wx-route and wx-markers, layers in the table order', () => {
    const s = baseStyle(lines, markers);
    expect(s.version).toBe(8);
    expect(s.glyphs).toBe(GLYPHS_URL);
    expect(s.sprite).toBeUndefined();
    expect(Object.keys(s.sources).sort()).toEqual([COLOUR_SOURCE, HIDDEN_SOURCE, MARKER_SOURCE, RANGE_SOURCE, ROUTE_SOURCE].sort());
    expect(ids(s.layers)).toEqual(['background', ...WX_LAYER_IDS]);
    expect(WX_LAYER_IDS).toEqual([
      'wx-route-bridge', 'wx-route-casing', 'wx-route-line', 'wx-colour-line', 'wx-hidden-bridge', 'wx-hidden-line', 'wx-range-bridge', 'wx-range-line', 'wx-lap-circle', 'wx-lap-label', 'wx-start', 'wx-finish',
    ]);
  });

  it('wx-route: a FeatureCollection of a recorded and a bridge MultiLineString (v1.4)', () => {
    const route = baseStyle(lines, markers).sources[ROUTE_SOURCE];
    expect(route).toEqual({
      type: 'geojson',
      data: {
        type: 'FeatureCollection',
        features: [
          { type: 'Feature', properties: { kind: 'recorded' }, geometry: { type: 'MultiLineString', coordinates: recorded } },
          { type: 'Feature', properties: { kind: 'bridge' }, geometry: { type: 'MultiLineString', coordinates: bridges } },
        ],
      },
    });
  });

  it('each feature only when non-empty: no bridge → recorded only; only bridges → bridge only', () => {
    const kinds = (l: RouteLines) => {
      const src = baseStyle(l, markers).sources[ROUTE_SOURCE];
      if (src?.type !== 'geojson' || typeof src.data !== 'object' || src.data.type !== 'FeatureCollection') throw new Error('no collection');
      return src.data.features.map((f: { properties?: { kind?: unknown } | null }) => f.properties?.kind);
    };
    expect(kinds({ recorded, bridges: [] })).toEqual(['recorded']);
    expect(kinds({ recorded: [], bridges })).toEqual(['bridge']);
    // The layers stay in place either way: a filter over an absent kind draws nothing.
    expect(ids(baseStyle({ recorded: [], bridges }, markers).layers)).toEqual(['background', ...WX_LAYER_IDS]);
  });

  it('AC 4: a GPS dropout (gps-dropout) and a time gap (pause-hours) each give one bridge, drawn only by the dashed layer', () => {
    for (const name of ['gps-dropout', 'pause-hours']) {
      const data = golden(name);
      const r = buildRoute(data);
      if (r === null) throw new Error('expected a route');
      const s = baseStyle(routeLines(data, r), routeMarkers(data, r));
      const src = s.sources[ROUTE_SOURCE];
      if (src?.type !== 'geojson' || typeof src.data !== 'object' || src.data.type !== 'FeatureCollection') throw new Error('no collection');
      const bridge = src.data.features.find((f: { properties?: { kind?: unknown } | null }) => f.properties?.kind === 'bridge');
      expect(bridge?.geometry).toMatchObject({ type: 'MultiLineString' });
      expect((bridge?.geometry as { coordinates: unknown[] }).coordinates).toHaveLength(1);
      const drawing = s.layers.filter(
        (l) => 'source' in l && l.source === ROUTE_SOURCE && 'filter' in l && JSON.stringify(l.filter) === JSON.stringify(['==', ['get', 'kind'], 'bridge']),
      );
      expect(drawing.map((l) => l.id)).toEqual(['wx-route-bridge']);
      expect(drawing[0]).toMatchObject({ paint: { 'line-dasharray': [2, 1.5] } });
    }
  });

  it('filters: bridge layer on kind bridge, casing and line on kind recorded', () => {
    const byId = new Map(baseStyle(lines, markers).layers.map((l) => [l.id, l]));
    expect(byId.get('wx-route-bridge')).toMatchObject({ source: ROUTE_SOURCE, filter: ['==', ['get', 'kind'], 'bridge'] });
    expect(byId.get('wx-route-casing')).toMatchObject({ source: ROUTE_SOURCE, filter: ['==', ['get', 'kind'], 'recorded'] });
    expect(byId.get('wx-route-line')).toMatchObject({ source: ROUTE_SOURCE, filter: ['==', ['get', 'kind'], 'recorded'] });
  });

  it('bridge look: route colour, 2.5 px, opacity 0.85, dash [2, 1.5], butt caps; below the casing', () => {
    const s = baseStyle(lines, markers);
    const bridge = s.layers.find((l) => l.id === 'wx-route-bridge');
    expect(bridge).toEqual({
      id: 'wx-route-bridge',
      type: 'line',
      source: ROUTE_SOURCE,
      filter: ['==', ['get', 'kind'], 'bridge'],
      layout: { 'line-cap': 'butt' },
      paint: { 'line-color': '#c2187a', 'line-width': 2.5, 'line-opacity': 0.85, 'line-dasharray': [2, 1.5] },
    });
    const order = ids(s.layers);
    expect(order.indexOf('wx-route-bridge')).toBeLessThan(order.indexOf('wx-route-casing'));
    expect(order.indexOf('wx-route-casing')).toBeLessThan(order.indexOf('wx-route-line'));
  });

  it('markers are points with kind and lap, lon first', () => {
    const src = baseStyle(lines, markers).sources[MARKER_SOURCE];
    expect(src).toEqual({
      type: 'geojson',
      data: {
        type: 'FeatureCollection',
        features: [
          { type: 'Feature', properties: { kind: 'start' }, geometry: { type: 'Point', coordinates: [8, 47] } },
          { type: 'Feature', properties: { kind: 'lap', lap: 2 }, geometry: { type: 'Point', coordinates: [8.1, 47.1] } },
          { type: 'Feature', properties: { kind: 'finish' }, geometry: { type: 'Point', coordinates: [8.2, 47.2] } },
        ],
      },
    });
  });

  it('neither a recorded segment nor a bridge (one positioned sample): no wx-route source and no route layers', () => {
    const s = baseStyle(none, markers.filter((m) => m.kind !== 'lap'));
    expect(Object.keys(s.sources)).toEqual([MARKER_SOURCE, COLOUR_SOURCE, HIDDEN_SOURCE, RANGE_SOURCE]);
    expect(ids(s.layers)).toEqual(['background', 'wx-colour-line', 'wx-hidden-bridge', 'wx-hidden-line', 'wx-range-bridge', 'wx-range-line', 'wx-lap-circle', 'wx-lap-label', 'wx-start', 'wx-finish']);
  });

  it('F-17 R-10: wx-range holds the given range (empty by default), drawn by its own layers in the range colour (classed edges in the range blues, F-25)', () => {
    const empty = baseStyle(lines, markers).sources[RANGE_SOURCE];
    expect(empty).toEqual({ type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
    const s = baseStyle(lines, markers, MAP_COLOURS, { recorded: [recorded[0]], bridges });
    const src = s.sources[RANGE_SOURCE];
    if (src?.type !== 'geojson' || typeof src.data !== 'object' || src.data.type !== 'FeatureCollection') throw new Error('no collection');
    expect(src.data.features.map((f: { properties?: { kind?: unknown } | null }) => f.properties?.kind)).toEqual(['recorded', 'bridge']);
    const byId = new Map(s.layers.map((l) => [l.id, l]));
    expect(byId.get('wx-range-line')).toMatchObject({ source: RANGE_SOURCE, filter: ['==', ['get', 'kind'], 'recorded'], paint: { 'line-color': classColourExpression(CLASS_COLOURS, MAP_COLOURS.range), 'line-width': 5 } });
    expect(byId.get('wx-range-bridge')).toMatchObject({ source: RANGE_SOURCE, filter: ['==', ['get', 'kind'], 'bridge'], paint: { 'line-dasharray': [2, 1.5] } });
  });

  it('route colour and widths; the background is not a wx- layer', () => {
    const s = baseStyle(lines, markers);
    const byId = new Map(s.layers.map((l) => [l.id, l]));
    expect(byId.get('wx-route-line')).toMatchObject({ paint: { 'line-color': '#c2187a', 'line-width': 3 } });
    expect(byId.get('wx-route-casing')).toMatchObject({ paint: { 'line-color': '#ffffff', 'line-width': 6 } });
    expect(byId.get('wx-lap-label')).toMatchObject({ layout: { 'text-font': ['Noto Sans Regular'], 'text-size': 10 } });
    expect(s.layers[0].id.startsWith('wx-')).toBe(false);
  });
});

describe('F-25: route colouring (C-1, ADR-037)', () => {
  it('wx-colour starts empty (plain route); its layer lies right above the route line, at its width, coloured by class', () => {
    const s = baseStyle(lines, markers);
    expect(s.sources[COLOUR_SOURCE]).toEqual({ type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
    const order = ids(s.layers);
    expect(order.indexOf('wx-colour-line')).toBe(order.indexOf('wx-route-line') + 1);
    const byId = new Map(s.layers.map((l) => [l.id, l]));
    expect(byId.get('wx-colour-line')).toMatchObject({
      source: COLOUR_SOURCE,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-width': 3, 'line-color': classColourExpression(CLASS_COLOURS, MAP_COLOURS.route) },
    });
    expect(byId.get('wx-colour-line')).not.toHaveProperty('filter');
  });

  it('the class expression maps c = 0…20 to the 21 class colours, else the fallback', () => {
    const e = classColourExpression(['#000001', '#000002'], '#ff0000');
    expect(e).toEqual(['match', ['get', 'c'], 0, '#000001', 1, '#000002', '#ff0000']);
    expect(CLASS_COLOURS).toHaveLength(RANGE_BLUES + 7);
    expect(RANGE_BLUES).toBe(2 * DIVERGING);
  });

  it('colourData: one MultiLineString per non-empty class, c offset for the diverging palette', () => {
    const a: [number, number][][] = [[[8, 47], [8.1, 47]]];
    const d = colourData([a, [], a], DIVERGING);
    expect(d.features).toEqual([
      { type: 'Feature', properties: { kind: 'recorded', c: 7 }, geometry: { type: 'MultiLineString', coordinates: a } },
      { type: 'Feature', properties: { kind: 'recorded', c: 9 }, geometry: { type: 'MultiLineString', coordinates: a } },
    ]);
    expect(colourData([]).features).toEqual([]);
  });

  it('rangeData: the plain range lines first, then the classed edges in the range blues', () => {
    const a: [number, number][][] = [[[8, 47], [8.1, 47]]];
    const bridge: [[number, number], [number, number]] = [[8.1, 47], [8.2, 47]];
    const d = rangeData({ recorded: a, bridges: [bridge] }, [[], a]);
    expect(d.features).toEqual([
      ...linesData({ recorded: a, bridges: [bridge] }).features,
      { type: 'Feature', properties: { kind: 'recorded', c: RANGE_BLUES + 1 }, geometry: { type: 'MultiLineString', coordinates: a } },
    ]);
  });
});

describe('F-19 E-9: the to-be-hidden marks', () => {
  it('wx-hidden starts empty and is drawn over the route, under the range, in the hidden colour', () => {
    const s = baseStyle(lines, markers);
    expect(s.sources[HIDDEN_SOURCE]).toEqual({ type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
    const byId = new Map(s.layers.map((l) => [l.id, l]));
    expect(byId.get('wx-hidden-line')).toMatchObject({ source: HIDDEN_SOURCE, filter: ['==', ['get', 'kind'], 'recorded'], paint: { 'line-color': MAP_COLOURS.hidden, 'line-width': 4 } });
    expect(byId.get('wx-hidden-bridge')).toMatchObject({ source: HIDDEN_SOURCE, filter: ['==', ['get', 'kind'], 'bridge'], paint: { 'line-dasharray': [2, 1.5] } });
    const order = ids(s.layers);
    expect(order.indexOf('wx-hidden-line')).toBeGreaterThan(order.indexOf('wx-route-line'));
    expect(order.indexOf('wx-hidden-line')).toBeLessThan(order.indexOf('wx-range-bridge'));
  });
});

describe('mergeStyle', () => {
  const previous = baseStyle(lines, markers);

  it("next's layers first, then every wx- layer of previous in its order; wx- sources added", () => {
    const merged = mergeStyle(previous, remote());
    expect(ids(merged.layers)).toEqual(['background', 'natural_earth', 'water', ...WX_LAYER_IDS]);
    expect(Object.keys(merged.sources).sort()).toEqual(['ne2_shaded', 'openmaptiles', COLOUR_SOURCE, HIDDEN_SOURCE, MARKER_SOURCE, RANGE_SOURCE, ROUTE_SOURCE].sort());
    expect(merged.sources[ROUTE_SOURCE]).toBe(previous.sources[ROUTE_SOURCE]);
    // The base background is not carried over; liberty's own is kept.
    expect(merged.layers.filter((l) => l.id === 'background')).toHaveLength(1);
    expect(merged.layers[0]).toMatchObject({ paint: { 'background-color': '#f8f4f0' } });
  });

  it("the base style's background is dropped; liberty's own background stays at the bottom", () => {
    expect(previous.layers.some((l) => l.id === 'background')).toBe(true);
    const merged = mergeStyle(previous, remote());
    expect(merged.layers[0]).toEqual(remote().layers[0]);
    expect(merged.layers.filter((l) => l.id === 'background')).toHaveLength(1);
    // A remote style without a background layer: ours is not carried over either.
    const bare = remote();
    bare.layers = bare.layers.filter((l) => l.id !== 'background');
    expect(mergeStyle(previous, bare).layers.some((l) => l.id === 'background')).toBe(false);
  });

  it("next's own sources unchanged, including the raster source ne2_shaded; sprite and glyphs from next", () => {
    const next = remote();
    const merged = mergeStyle(previous, next);
    expect(merged.sources.openmaptiles).toEqual(next.sources.openmaptiles);
    expect(merged.sources.ne2_shaded).toEqual(next.sources.ne2_shaded);
    expect(merged.sprite).toBe(next.sprite);
    expect(merged.glyphs).toBe(next.glyphs);
  });

  it('projection forced to mercator; terrain and sky removed', () => {
    const next = remote({
      projection: { type: 'globe' },
      terrain: { source: 'dem', exaggeration: 1 },
      sky: { 'sky-color': '#000000' },
    });
    const merged = mergeStyle(previous, next);
    expect(merged.projection).toEqual({ type: 'mercator' });
    expect('terrain' in merged).toBe(false);
    expect('sky' in merged).toBe(false);
  });

  it('idempotent: applying it twice yields no duplicate wx- layer or source', () => {
    const once = mergeStyle(previous, remote());
    const twice = mergeStyle(once, once);
    expect(ids(twice.layers)).toEqual(ids(once.layers));
    expect(Object.keys(twice.sources).sort()).toEqual(Object.keys(once.sources).sort());
    const again = mergeStyle(previous, mergeStyle(previous, remote()));
    expect(ids(again.layers)).toEqual(ids(once.layers));
  });

  it('previous undefined → next unchanged except projection, terrain and sky', () => {
    const next = remote({ terrain: { source: 'dem' } });
    const merged = mergeStyle(undefined, next);
    const { terrain: _t, ...expected } = next;
    expect(merged).toEqual({ ...expected, projection: { type: 'mercator' } });
  });

  it('does not change its inputs', () => {
    const next = remote();
    const before = JSON.stringify(next);
    const prevBefore = JSON.stringify(previous);
    mergeStyle(previous, next);
    expect(JSON.stringify(next)).toBe(before);
    expect(JSON.stringify(previous)).toBe(prevBefore);
  });
});

describe('isStyle', () => {
  it('accepts a version 8 style with layers and sources; rejects everything else', () => {
    expect(isStyle(remote())).toBe(true);
    expect(isStyle(null)).toBe(false);
    expect(isStyle('style')).toBe(false);
    expect(isStyle({ version: 7, layers: [], sources: {} })).toBe(false);
    expect(isStyle({ version: 8, layers: {}, sources: {} })).toBe(false);
    expect(isStyle({ version: 8, layers: [] })).toBe(false);
  });
});

describe('mapPixelRatio (F-07 D-8, QA-06-r4-1)', () => {
  it.each([
    [1, 1],
    [1.5, 1.5],
    [2, 2],
    [3, 2],
    [0, 1],
    [Number.NaN, 1],
    [-1, 1],
    [Number.POSITIVE_INFINITY, 1],
  ])('dpr %s → %s', (dpr, expected) => {
    expect(mapPixelRatio(dpr)).toBe(expected);
  });
});
