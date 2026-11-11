// Map style: the local base style and the merge with the remote style (F-06 design, "Style
// and layers", "Map layers"; ADR-013 decision 2, ADR-014 rev. 1). LAZY: reached only from
// map-view.ts. Pure; `import type` from maplibre-gl only.
//
// Every id of ours starts with WX_PREFIX. mergeStyle carries exactly those over a style
// swap, so the base style's own background must NOT carry the prefix (it would cover the
// remote style).

import type { FilterSpecification, LayerSpecification, SourceSpecification, StyleSpecification } from 'maplibre-gl';
import type { RouteLines, RouteMarker } from '../../lib/route';

/** OpenFreeMap liberty (user decision on OQ-18), the light scheme's style. */
export const OPENFREEMAP_STYLE_URL = 'https://tiles.openfreemap.org/styles/liberty';
/** OpenFreeMap dark, the dark scheme's style (F-14 D-12; same host, sources and fontstack as liberty). */
export const OPENFREEMAP_DARK_STYLE_URL = 'https://tiles.openfreemap.org/styles/dark';

export type Scheme = 'light' | 'dark';

export const styleUrlFor = (scheme: Scheme): string => (scheme === 'dark' ? OPENFREEMAP_DARK_STYLE_URL : OPENFREEMAP_STYLE_URL);
/** The only third-party host the viewer's browser contacts (I-6, ADR-014). */
export const TILE_HOST = 'https://tiles.openfreemap.org';
/** Liberty's own glyphs URL (verified 12 Oct 2026); in the base style for the lap labels. */
export const GLYPHS_URL = 'https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf';
/** Liberty's regular fontstack (verified 12 Oct 2026). */
export const LABEL_FONT = 'Noto Sans Regular';

export const WX_PREFIX = 'wx-';
export const ROUTE_SOURCE = 'wx-route';
export const MARKER_SOURCE = 'wx-markers';
/** The selected range on the route (F-17 R-10): above the route, below the markers. */
export const RANGE_SOURCE = 'wx-range';
/** The to-be-hidden start and end on the edit page (F-19 E-9): above the route, below the range. */
export const HIDDEN_SOURCE = 'wx-hidden';
/** Route colouring by metric (F-25 C-1, ADR-037): at most one MultiLineString per colour class; empty = plain route. */
export const COLOUR_SOURCE = 'wx-colour';

/** Cap for MapLibre's pixelRatio (v1.4): a 3× canvas has 2.25 times the pixels of 2× in every
 *  frame, first render included, with little visible gain on a map. F-26 may raise it. */
export const MAX_MAP_PIXEL_RATIO = 2;

/** MapLibre's pixelRatio for a device pixel ratio (F-07 design D-8, F-5; QA-06-r4-1):
 *  min(dpr, MAX_MAP_PIXEL_RATIO) for a finite dpr > 0, otherwise 1. */
export function mapPixelRatio(dpr: number): number {
  return Number.isFinite(dpr) && dpr > 0 ? Math.min(dpr, MAX_MAP_PIXEL_RATIO) : 1;
}

/** Our layers' colours: the light values, which are also the fallback of the theme's custom properties (F-14 D-9, D-10). */
export const MAP_COLOURS = {
  background: '#e8eaed',
  casing: '#ffffff',
  /** Magenta: a hue liberty does not use for roads (orange, yellow) or water (blue). */
  route: '#c2187a',
  /** The selected range (F-17 R-10): the accent blue, as the chart shading. */
  range: '#1f6fc5',
  /** To-be-hidden sections (F-19 E-9): a neutral grey over the route, "this part goes". */
  hidden: '#8a8f98',
  lapFill: '#ffffff',
  lapStroke: '#333333',
  lapText: '#222222',
  start: '#2e7d32',
  finish: '#c62828',
  markerStroke: '#ffffff',
  /** The selected range's outline with colouring on (F-25 C-9). */
  rangeOutline: '#4d4d4d',
  /** Not a colour: the opacity of the route outside the range with colouring on (F-25 C-9). */
  fade: '0.35',
} as const;

type MapColours = { readonly [K in keyof typeof MAP_COLOURS]: string };

/** Custom property of each colour (page.css theme block). */
const COLOUR_PROPERTIES: Readonly<Record<keyof typeof MAP_COLOURS, string>> = {
  background: '--map-bg',
  casing: '--route-casing',
  route: '--route',
  range: '--range',
  hidden: '--hidden',
  lapFill: '--lap-fill',
  lapStroke: '--lap-stroke',
  lapText: '--lap-text',
  start: '--start',
  finish: '--finish',
  markerStroke: '--marker-stroke',
  rangeOutline: '--range-outline',
  fade: '--route-fade',
};

/** The colour classes' light values, the fallback of `--seq-0…11` (speed, pace, heart rate, power: low → high)
 *  and `--div-0…10` (gradient: descent, flat, climb) in page.css (F-25 C-5). Feature property `c` of wx-colour
 *  and wx-range indexes this list: 0–11 sequential, 12–22 diverging; in wx-range, NO_VALUE is an edge without one. */
export const CLASS_COLOURS: readonly string[] = [
  '#1b3f97', '#0057a8', '#0070ab', '#0088a9', '#04a19b', '#7ab05a', '#dcae00', '#e58900', '#df6800', '#d34900', '#c02e00', '#a81900',
  '#163990', '#1656a4', '#2574b0', '#488bb2', '#6e9db3', '#b3a381', '#c8843d', '#c76414', '#be4400', '#ab2c00', '#902014',
];
/** Offset of the diverging palette in CLASS_COLOURS. */
export const DIVERGING = 12;
/** Class `c` of a wx-range edge without a value while colouring is on: drawn in the route colour. */
export const NO_VALUE = 23;
const classProperty = (k: number): string => (k < DIVERGING ? '--seq-' + k : '--div-' + (k - DIVERGING));

/** The class colours of the current theme, read from `el`; the light values where unset. */
export function classColours(el: Element): string[] {
  const css = getComputedStyle(el);
  return CLASS_COLOURS.map((fallback, k) => css.getPropertyValue(classProperty(k)).trim() || fallback);
}

/** wx-colour-line's and wx-range-line's line-color: the class colour by feature property `c`, else `fallback`. */
export function classColourExpression(classes: readonly string[], fallback: string): unknown[] {
  const e: unknown[] = ['match', ['get', 'c']];
  classes.forEach((colour, k) => e.push(k, colour));
  e.push(fallback);
  return e;
}

/** The colours of the current theme, read from the custom properties of `el`; the light values where unset. */
export function mapColours(el: Element): MapColours {
  const css = getComputedStyle(el);
  const out: Record<string, string> = {};
  for (const k of Object.keys(MAP_COLOURS) as (keyof typeof MAP_COLOURS)[]) {
    out[k] = css.getPropertyValue(COLOUR_PROPERTIES[k]).trim() || MAP_COLOURS[k];
  }
  return out as unknown as MapColours;
}

/** Paint properties per layer id → the colour that feeds it. */
const PAINT: readonly [layer: string, property: string, colour: keyof typeof MAP_COLOURS][] = [
  ['background', 'background-color', 'background'],
  ['wx-route-bridge', 'line-color', 'route'],
  ['wx-route-casing', 'line-color', 'casing'],
  ['wx-route-line', 'line-color', 'route'],
  ['wx-hidden-bridge', 'line-color', 'hidden'],
  ['wx-hidden-line', 'line-color', 'hidden'],
  ['wx-range-casing', 'line-color', 'rangeOutline'],
  ['wx-range-bridge', 'line-color', 'range'],
  ['wx-lap-circle', 'circle-color', 'lapFill'],
  ['wx-lap-circle', 'circle-stroke-color', 'lapStroke'],
  ['wx-lap-label', 'text-color', 'lapText'],
  ['wx-leg-label', 'text-color', 'lapText'],
  ['wx-leg-label', 'text-halo-color', 'lapFill'],
  ['wx-start', 'circle-color', 'start'],
  ['wx-start', 'circle-stroke-color', 'markerStroke'],
  ['wx-finish', 'circle-color', 'finish'],
  ['wx-finish', 'circle-stroke-color', 'markerStroke'],
];

/** Repaints our layers (and the base background, while it exists) in place after a scheme change; the class
 *  colour expressions are rebuilt from `classes` (F-25 C-5). */
type PaintTarget = { getLayer(id: string): unknown; setPaintProperty(id: string, name: string, value: unknown): void };

export function recolour(
  map: PaintTarget,
  colours: MapColours,
  classes: readonly string[] = CLASS_COLOURS,
): void {
  for (const [layer, property, colour] of PAINT) if (map.getLayer(layer)) map.setPaintProperty(layer, property, colours[colour]);
  if (map.getLayer('wx-colour-line')) map.setPaintProperty('wx-colour-line', 'line-color', classColourExpression(classes, colours.route));
  if (map.getLayer('wx-range-line')) map.setPaintProperty('wx-range-line', 'line-color', rangeColourExpression(classes, colours));
}

/** wx-range-line's line-color: a classed edge in its class colour, NO_VALUE in the route colour, else the range colour. */
export const rangeColourExpression = (classes: readonly string[], colours: MapColours): unknown[] =>
  classColourExpression([...classes, colours.route], colours.range);

/** F-25 C-9: `on` (colouring on, a range on the route) fades the route outside the range; off, every opacity
 *  is as before F-25. */
export function emphasise(map: PaintTarget, colours: MapColours, on: boolean): void {
  const fade = on ? +colours.fade : 1;
  for (const [id, opacity] of [['wx-route-bridge', 0.85 * fade], ['wx-route-casing', fade], ['wx-route-line', fade], ['wx-colour-line', fade]] as const) {
    if (map.getLayer(id)) map.setPaintProperty(id, 'line-opacity', opacity);
  }
}

/** Bottom → top, as in the design's layer table. */
export const WX_LAYER_IDS = ['wx-route-bridge', 'wx-route-casing', 'wx-route-line', 'wx-colour-line', 'wx-hidden-bridge', 'wx-hidden-line', 'wx-range-casing', 'wx-range-bridge', 'wx-range-line', 'wx-lap-circle', 'wx-lap-label', 'wx-leg-label', 'wx-start', 'wx-finish'] as const;

const kindIs = (kind: string): FilterSpecification => ['==', ['get', 'kind'], kind];

/** One feature of the wx-route source (v1.4); a GeoJSON Feature, typed here because the
 *  GeoJSON namespace types are not in this project's tsconfig `types`. */
interface RouteFeature {
  readonly type: 'Feature';
  readonly properties: { readonly kind: 'recorded' | 'bridge' };
  readonly geometry: { readonly type: 'MultiLineString'; readonly coordinates: readonly (readonly [number, number])[][] };
}

/** The GeoJSON of a RouteLines: at most two MultiLineStrings, kind 'recorded' and 'bridge', each only if non-empty. */
export function linesData(lines: RouteLines): { type: 'FeatureCollection'; features: RouteFeature[] } {
  const features: RouteFeature[] = [];
  if (lines.recorded.length > 0) {
    features.push({ type: 'Feature', properties: { kind: 'recorded' }, geometry: { type: 'MultiLineString', coordinates: lines.recorded } });
  }
  if (lines.bridges.length > 0) {
    features.push({ type: 'Feature', properties: { kind: 'bridge' }, geometry: { type: 'MultiLineString', coordinates: lines.bridges } });
  }
  return { type: 'FeatureCollection', features };
}

const NO_LINES: RouteLines = { recorded: [], bridges: [] };

/** The GeoJSON of wx-colour: one MultiLineString per non-empty class k of `lines`, properties c = k
 *  and kind 'recorded' (so the range layers' filter takes them too). */
export function colourData(lines: readonly [number, number][][][]): { type: 'FeatureCollection'; features: unknown[] } {
  const features: unknown[] = [];
  lines.forEach((coordinates, k) => {
    if (coordinates.length > 0) features.push({ type: 'Feature', properties: { kind: 'recorded', c: k }, geometry: { type: 'MultiLineString', coordinates } });
  });
  return { type: 'FeatureCollection', features };
}

/** The GeoJSON of wx-range with colouring on (F-25 C-9): the recorded edges by class (c = k, NO_VALUE included),
 *  then the bridges. */
export function rangeData(classed: readonly [number, number][][][], bridges: RouteLines['bridges']): { type: 'FeatureCollection'; features: unknown[] } {
  return { type: 'FeatureCollection', features: [...colourData(classed).features, ...linesData({ recorded: [], bridges }).features] };
}

/** Local style: background, glyphs URL, sources wx-route and wx-markers (inline GeoJSON), and
 *  the wx- layers. wx-route (v1.4) is a FeatureCollection of at most two MultiLineStrings,
 *  kind 'recorded' (solid) and kind 'bridge' (dashed), each only if non-empty; the source and
 *  its layers are absent when there is neither. The route draws with this style before (or
 *  without) the tile host. */
export function baseStyle(
  lines: RouteLines,
  markers: readonly RouteMarker[],
  colours: MapColours = MAP_COLOURS,
  range: RouteLines = NO_LINES,
  classes: readonly string[] = CLASS_COLOURS,
): StyleSpecification {
  const sources: Record<string, SourceSpecification> = {
    [MARKER_SOURCE]: {
      type: 'geojson',
      data: {
        type: 'FeatureCollection',
        features: markers.map((m) => ({
          type: 'Feature',
          properties: m.lap !== undefined ? { kind: m.kind, lap: m.lap } : m.name !== undefined ? { kind: m.kind, name: m.name } : { kind: m.kind },
          geometry: { type: 'Point', coordinates: [m.lon, m.lat] },
        })),
      },
    },
  };
  const layers: LayerSpecification[] = [{ id: 'background', type: 'background', paint: { 'background-color': colours.background } }];
  const routeData = linesData(lines);
  if (routeData.features.length > 0) {
    sources[ROUTE_SOURCE] = { type: 'geojson', data: routeData };
    const join = { 'line-join': 'round', 'line-cap': 'round' } as const;
    const recorded = kindIs('recorded');
    layers.push(
      // Below the recorded line, so its solid ends cover the dash ends. Dash lengths are in line widths.
      {
        id: 'wx-route-bridge',
        type: 'line',
        source: ROUTE_SOURCE,
        filter: kindIs('bridge'),
        layout: { 'line-cap': 'butt' },
        paint: { 'line-color': colours.route, 'line-width': 2.5, 'line-opacity': 0.85, 'line-dasharray': [2, 1.5] },
      },
      { id: 'wx-route-casing', type: 'line', source: ROUTE_SOURCE, filter: recorded, layout: join, paint: { 'line-color': colours.casing, 'line-width': 6 } },
      { id: 'wx-route-line', type: 'line', source: ROUTE_SOURCE, filter: recorded, layout: join, paint: { 'line-color': colours.route, 'line-width': 3 } },
    );
  }
  // The colour, hidden and range sources always exist (empty at first), so the map view can setData them.
  sources[COLOUR_SOURCE] = { type: 'geojson', data: { type: 'FeatureCollection', features: [] } };
  sources[HIDDEN_SOURCE] = { type: 'geojson', data: linesData(NO_LINES) };
  sources[RANGE_SOURCE] = { type: 'geojson', data: linesData(range) };
  layers.push(
    // F-25 C-1: over the plain route line, at its width; an edge without a class is not in the source.
    {
      id: 'wx-colour-line',
      type: 'line',
      source: COLOUR_SOURCE,
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': classColourExpression(classes, colours.route) as never, 'line-width': 3 },
    },
    {
      id: 'wx-hidden-bridge',
      type: 'line',
      source: HIDDEN_SOURCE,
      filter: kindIs('bridge'),
      layout: { 'line-cap': 'butt' },
      paint: { 'line-color': colours.hidden, 'line-width': 2.5, 'line-dasharray': [2, 1.5] },
    },
    {
      id: 'wx-hidden-line',
      type: 'line',
      source: HIDDEN_SOURCE,
      filter: kindIs('recorded'),
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': colours.hidden, 'line-width': 4 },
    },
    {
      // F-25 C-9: the range's outline, only with colouring on: only rangeData's recorded edges carry a class.
      id: 'wx-range-casing',
      type: 'line',
      source: RANGE_SOURCE,
      filter: ['has', 'c'],
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': colours.rangeOutline, 'line-width': 9 },
    },
    {
      id: 'wx-range-bridge',
      type: 'line',
      source: RANGE_SOURCE,
      filter: kindIs('bridge'),
      layout: { 'line-cap': 'butt' },
      paint: { 'line-color': colours.range, 'line-width': 4, 'line-dasharray': [2, 1.5] },
    },
    {
      id: 'wx-range-line',
      type: 'line',
      source: RANGE_SOURCE,
      filter: kindIs('recorded'),
      layout: { 'line-join': 'round', 'line-cap': 'round' },
      paint: { 'line-color': rangeColourExpression(classes, colours) as never, 'line-width': 5 },
    },
  );
  layers.push(
    {
      // F-42 MS-8: a leg marker is a lap circle with its name beside it.
      id: 'wx-lap-circle',
      type: 'circle',
      source: MARKER_SOURCE,
      filter: ['in', ['get', 'kind'], ['literal', ['lap', 'leg']]],
      paint: { 'circle-radius': 6, 'circle-color': colours.lapFill, 'circle-stroke-color': colours.lapStroke, 'circle-stroke-width': 1.5 },
    },
    {
      id: 'wx-lap-label',
      type: 'symbol',
      source: MARKER_SOURCE,
      filter: kindIs('lap'),
      layout: { 'text-field': ['to-string', ['get', 'lap']], 'text-font': [LABEL_FONT], 'text-size': 10 },
      paint: { 'text-color': colours.lapText },
    },
    {
      id: 'wx-leg-label',
      type: 'symbol',
      source: MARKER_SOURCE,
      filter: kindIs('leg'),
      layout: {
        'text-field': ['get', 'name'], 'text-font': [LABEL_FONT], 'text-size': 12,
        // Colliding leg labels (legs starting at one place) move to another side of their marker; MapLibre
        // drops one only when every position collides, and shows it again when zoomed in.
        'text-variable-anchor-offset': ['left', [0.8, 0], 'right', [-0.8, 0], 'top', [0, 0.8], 'bottom', [0, -0.8], 'top-left', [0.6, 0.6], 'bottom-left', [0.6, -0.6], 'top-right', [-0.6, 0.6], 'bottom-right', [-0.6, -0.6]],
        'text-justify': 'auto', 'text-allow-overlap': false,
      },
      paint: { 'text-color': colours.lapText, 'text-halo-color': colours.lapFill, 'text-halo-width': 1.5 },
    },
    markerCircle('wx-start', 'start', colours.start, colours),
    markerCircle('wx-finish', 'finish', colours.finish, colours),
  );
  return { version: 8, glyphs: GLYPHS_URL, sources, layers };
}

function markerCircle(id: string, kind: string, colour: string, colours: MapColours): LayerSpecification {
  return {
    id,
    type: 'circle',
    source: MARKER_SOURCE,
    filter: kindIs(kind),
    paint: { 'circle-radius': 7, 'circle-color': colour, 'circle-stroke-color': colours.markerStroke, 'circle-stroke-width': 2 },
  };
}

/** transformStyle callback: next's sources and layers (without any wx- ones), then every wx-
 *  source and layer of previous on top, in previous's order; projection forced to mercator;
 *  terrain and sky removed. Idempotent: a wx- layer never appears twice. next's other
 *  sources are kept as they are (e.g. liberty's raster `ne2_shaded`). */
export function mergeStyle(previous: StyleSpecification | undefined, next: StyleSpecification): StyleSpecification {
  const { terrain: _terrain, sky: _sky, ...rest } = next;
  const sources: Record<string, SourceSpecification> = {};
  for (const [id, source] of Object.entries(next.sources)) if (!id.startsWith(WX_PREFIX)) sources[id] = source;
  const layers = next.layers.filter((l) => !l.id.startsWith(WX_PREFIX));
  if (previous) {
    for (const [id, source] of Object.entries(previous.sources)) if (id.startsWith(WX_PREFIX)) sources[id] = source;
    for (const l of previous.layers) if (l.id.startsWith(WX_PREFIX)) layers.push(l);
  }
  return { ...rest, projection: { type: 'mercator' }, sources, layers };
}

/** True if a fetched JSON value can be passed to setStyle (enough for mergeStyle not to throw;
 *  MapLibre validates the rest and reports problems through its error event). */
export function isStyle(value: unknown): value is StyleSpecification {
  const v = value as StyleSpecification | null;
  return typeof v === 'object' && v !== null && v.version === 8 && Array.isArray(v.layers) && typeof v.sources === 'object' && v.sources !== null;
}
