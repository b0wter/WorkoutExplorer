// The lazy map adapter (F-06 design, "Loading and start-up", "Cursor marker", "Route
// interaction"; ADR-013, ADR-014 rev. 1, ADR-015). LAZY: reached only through
// RouteMap's import('./map-view'). The only value importer of 'maplibre-gl' in the app.
//
// Thin on purpose: route geometry, markers, the nearest-sample grid and the gesture reducer
// live in lib/route, the style and its merge in map-style.ts (all with Node tests); this file
// wires them to MapLibre, the DOM and the cursor store (jsdom test with a recording fake,
// ADR-012 rev. 3).
//
// Checked against maplibre-gl 6.14.0 (dist/maplibre-gl-dev.mjs), for the route-scrub lock:
// - HandlerManager adds its touchstart (passive), touchmove (non-passive), touchend and
//   touchcancel listeners, without capture, on the canvas container
//   (.maplibregl-canvas-container), a descendant of our outer element. Touch pan, the
//   cooperative-gesture handler (notifyGestureBlocked from TouchPanHandler.touchmove), tap,
//   tap-drag zoom and two-finger zoom all receive touch events only through it.
// - No MapLibre code listens for pointer events.
// - The only touchmove listener elsewhere is on window, added by the compass button's
//   MouseRotateWrapper during a touch on the compass; the compass is off (showCompass: false).
// So stopPropagation() in a capture-phase touchmove listener on the outer element keeps a
// route scrub's moves from MapLibre: it neither pans nor shows its two-finger hint.

import { LngLat, Map as MapLibreMap, Marker, NavigationControl, setWorkerUrl, type StyleSpecification } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url';
import './map-view.css';
import type { WorkoutData } from '../../lib/datafile';
import type { CursorIndex, CursorStore } from '../../lib/cursor';
import type { SelectionStore } from '../../lib/selection';
import type { UnitsStore } from '../../lib/units';
import { CLASS_COUNT, classify, colourScale, legend, metricLabel, metricValues, offeredMetrics, type ColourMetric } from '../../lib/colouring';
import {
  ROUTE_IDLE,
  TIE_PX,
  buildRoute,
  classLines,
  createRouteIndex,
  hitRadiusPx,
  interceptsTouch,
  lowerBound,
  mercatorX,
  mercatorY,
  recordedEdgeEnds,
  routeLines,
  routeLinesIn,
  routeMarkers,
  routeStep,
  wantsHit,
  type RouteGestureInput,
  type RouteGestureKind,
  type RouteGestureState,
} from '../../lib/route';
import {
  COLOUR_SOURCE,
  DIVERGING,
  HIDDEN_SOURCE,
  NO_VALUE,
  RANGE_SOURCE,
  baseStyle,
  classColours,
  colourData,
  emphasise,
  isStyle,
  linesData,
  mapColours,
  mapPixelRatio,
  mergeStyle,
  rangeData,
  recolour,
  styleUrlFor,
  type Scheme,
} from './map-style';

/** The range highlight is rebuilt this long after the range stops changing (F-17 R-10). */
export const RANGE_SETTLE_MS = 150;

/** Fit padding. In the two-column layout the sticky map is 100lvh tall, so with the browser bar
 *  showing its bottom strip is out of view (F-13 D-18): 60 px at the bottom keeps the route visible. */
function fitPadding(): number | { top: number; right: number; bottom: number; left: number } {
  // Also the short-landscape two-column layout (F-17, page.css), whose map is 100lvh too.
  const el = document.documentElement;
  const w = el.clientWidth;
  const h = el.clientHeight;
  const two = w >= 1024 || (w > h && h > 0 && h <= 500);
  return two ? { top: 32, right: 32, bottom: 60, left: 32 } : 32;
}

export interface MountOptions {
  /** RouteMap's div.route-map; map-view creates MapLibre's container inside it. */
  readonly container: HTMLElement;
  readonly data: WorkoutData;
  readonly cursor: CursorStore;
  /** The range selection (F-17 R-10); read only, never written by the map (OQ-73). */
  readonly selection?: SelectionStore;
  readonly whenQuiet: () => Promise<void>;
  /** Overrides the scheme's style URL (tests, bench). */
  readonly styleUrl?: string;
  /** The colour scheme at mount: picks the style and our layers' colours (F-14 D-12). Default light. */
  readonly scheme?: Scheme;
  /** The viewer's units, for the colouring legend (F-25 C-6); metric when absent (tests, bench). */
  readonly units?: UnitsStore;
  readonly signal: AbortSignal;
}

export interface RouteMapHandle {
  /** The scheme changed: repaint our layers and swap the base style in place; never a remount or a refit (F-14 D-12). */
  setScheme(scheme: Scheme): void;
  /** Marks these inclusive sample ranges as to-be-hidden (F-19 E-9, ADR-030); [] marks nothing. */
  setHidden(ranges: readonly (readonly [number, number])[]): void;
  destroy(): void;
}

/** MapLibre's world is 512 · 2^zoom CSS px wide in Mercator units (pitch 0, Mercator). */
const WORLD_PX = 512;

type MutableInput = { -readonly [K in keyof RouteGestureInput]: RouteGestureInput[K] };

const swallow = (): void => {};
const inertHandle: RouteMapHandle = { setScheme: swallow, setHidden: swallow, destroy: swallow };

export async function mountRouteMap(options: MountOptions): Promise<RouteMapHandle> {
  const { container, data, cursor, selection, whenQuiet, signal } = options;

  // The remote style JSON: fetched by us (no main-thread cost while it loads), so we decide
  // when it is applied and the request carries no referrer and no credentials (ADR-014).
  const styleFetch = new AbortController();
  const abortStyle = (): void => styleFetch.abort();
  signal.addEventListener('abort', abortStyle);
  // Without credentials or referrer; a failed or invalid style is null and the current one stays.
  const fetchStyle = (url: string): Promise<StyleSpecification | null> =>
    fetch(url, { credentials: 'omit', referrerPolicy: 'no-referrer', signal: styleFetch.signal })
      .then((r) => (r.ok ? r.json() : null))
      .then((v: unknown) => (isStyle(v) ? v : null))
      .catch(() => null); // offline, blocked or aborted: the base style stays
  let scheme: Scheme = options.scheme ?? 'light';
  const remoteStyle = fetchStyle(options.styleUrl ?? styleUrlFor(scheme));

  const settle = (): void => {
    signal.removeEventListener('abort', abortStyle);
    styleFetch.abort();
  };

  await whenQuiet();
  if (signal.aborted) {
    settle();
    return inertHandle;
  }

  // Step 6: the route model (a few ms), measured for the start-up table (v1.4, User Timing).
  const modelStart = performance.now();
  const route = buildRoute(data);
  const position = data.position;
  if (route === null || !position) {
    settle();
    throw new Error('map-view: the workout has no positioned sample');
  }
  const index = createRouteIndex(route);
  const lines = routeLines(data, route);
  // The range highlight: the positioned samples in [a, b] (R-10, AC 3); the latest is kept, so the
  // style's own load and a style swap (merge) carry it.
  const rangeLines = (a: number, b: number) => (a < 0 ? { recorded: [], bridges: [] } : routeLinesIn(data, route, a, b));
  const initialRange = rangeLines(selection ? selection.start() : -1, selection ? selection.end() : -1);
  let range: { readonly features: readonly unknown[] } = linesData(initialRange);
  const markers = routeMarkers(data, route);
  const lat = position.latitude;
  const lon = position.longitude;
  // F-25 C-8: only which metrics are offered; no classes until the viewer picks one.
  const offered = offeredMetrics(data);
  performance.measure('wx-map:route-model', { start: modelStart });

  // Step 6a (v1.4): our work and MapLibre's never share one task.
  await whenQuiet();
  if (signal.aborted) {
    settle();
    return inertHandle;
  }

  const inner = document.createElement('div');
  inner.className = 'wx-map';
  container.appendChild(inner);

  let map: MapLibreMap;
  try {
    setWorkerUrl(workerUrl);
    const [west, south, east, north] = route.bounds;
    const point = west === east && south === north;
    const mapStart = performance.now();
    map = new MapLibreMap({
      container: inner,
      style: baseStyle(lines, markers, mapColours(container), initialRange, classColours(container)),
      ...(point
        ? { center: [west, south] as [number, number], zoom: 15 }
        : { bounds: [west, south, east, north] as [number, number, number, number], fitBoundsOptions: { padding: fitPadding(), maxZoom: 16 } }),
      cooperativeGestures: true,
      // Double-tap and double-click zoom off (F-06 v1.2): a short route scrub looks like a tap to
      // MapLibre, which never sees its moves, so two quick scrubs would zoom.
      doubleClickZoom: false,
      dragRotate: false,
      pitchWithRotate: false,
      touchPitch: false,
      maxPitch: 0,
      attributionControl: { compact: true },
      maplibreLogo: false,
      renderWorldCopies: false,
      // v1.4: cheaper start-up. A 3× canvas costs 2.25 times the pixels of 2× in every frame.
      // F-07 D-8: kept up to date below when devicePixelRatio changes.
      pixelRatio: mapPixelRatio(window.devicePixelRatio),
      // Style-spec validation of liberty's 111 layers runs on the main thread at every style
      // application; the tile host's style and our tested base style need no runtime check in
      // production. The dev page still shows validation errors.
      validateStyle: import.meta.env.DEV,
    });
    performance.measure('wx-map:new-map', { start: mapStart });
  } catch (error) {
    // No WebGL2 (GPUInitializationError) or any other throw: RouteMap shows the failure text.
    inner.remove();
    settle();
    throw error;
  }

  // ---- teardown: shared by destroy() and a mount that fails after new Map() ------------
  // (F-06 design v1.3, "Partial mount", QA-06-r1-1). Undoes whatever exists so far, in this
  // order: cursor subscription, listeners (same options), style fetch, map. A failed mount
  // never leaves a WebGL context, a listener or a pending fetch behind.
  let destroyed = false;
  let unsubscribe: (() => void) | null = null;
  let marker: Marker | null = null;
  const listeners: [EventTarget, string, EventListener, AddEventListenerOptions][] = [];
  const listen = (target: EventTarget, type: string, fn: (e: never) => void, opts: AddEventListenerOptions): void => {
    target.addEventListener(type, fn as EventListener, opts);
    listeners.push([target, type, fn as EventListener, opts]);
  };

  // ---- pixel ratio follows devicePixelRatio (F-07 design D-8, F-5; QA-06-r4-1) -----------
  // Desktop zoom or a move to a monitor with another scale factor changes devicePixelRatio;
  // without this the map stays blurry (or over-sampled) until reload. A resolution query
  // matches one exact value, so it is re-armed on every change. Not on the hot path.
  // Checked against maplibre-gl 6.14.0 (dist/maplibre-gl-dev.mjs): Map#setPixelRatio(number)
  // sets _overridePixelRatio and calls resize(), whose _resizeInternal resizes the canvas
  // (_resizeCanvas) and the painter.
  let applied = mapPixelRatio(window.devicePixelRatio);
  let query: MediaQueryList | null = null;
  const onResolution = (): void => {
    arm();
    const next = mapPixelRatio(window.devicePixelRatio);
    if (next !== applied) {
      applied = next;
      map.setPixelRatio(next);
    }
  };
  const arm = (): void => {
    query?.removeEventListener('change', onResolution);
    query = typeof matchMedia === 'function' ? matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`) : null;
    query?.addEventListener('change', onResolution);
  };

  let setScheme: RouteMapHandle['setScheme'] = swallow;
  // The to-be-hidden marks (F-19 E-9): the range geometry of F-17 per range; kept, so a style
  // swap re-applies it on 'style.load' like the range highlight.
  let hidden = linesData({ recorded: [], bridges: [] });
  const applyHidden = (): void => {
    (map.getSource(HIDDEN_SOURCE) as { setData(d: unknown): unknown } | undefined)?.setData(hidden);
  };
  let attribTimer: ReturnType<typeof setTimeout> | undefined;
  let rangeTimer: ReturnType<typeof setTimeout> | undefined;
  let unsubscribeRange: (() => void) | null = null;
  let unsubscribeUnits: (() => void) | null = null;
  // Route colouring (F-25 C-7): the latest wx-colour data, re-applied on 'style.load' like the range.
  let colour = colourData([]);
  // The current metric's class colour index c per sample, no class as NO_VALUE (null = plain route), and the
  // recorded edges' ends: kept, so a range update only partitions the range's edges, never recomputes values or bounds.
  let classes: Uint8Array | null = null;
  let ends: Uint32Array | null = null;
  // The range highlight's data: with colouring on, its edges by class (F-25 C-9). O(log N + range).
  const rangeOf = (a: number, b: number): typeof range => {
    const l = rangeLines(a, b);
    if (classes === null || ends === null || a < 0) return linesData(l);
    const inRange = ends.subarray(lowerBound(ends, a + 1), lowerBound(ends, b + 1));
    return rangeData(classLines(data, inRange, classes, NO_VALUE + 1), l.bridges);
  };
  // F-25 C-9: the route outside a coloured range fades; re-applied with the range and on a scheme change.
  let emphasised = false;
  const applyEmphasis = (): void => emphasise(map, mapColours(container), emphasised);
  const applyColour = (): void => {
    (map.getSource(COLOUR_SOURCE) as { setData(d: unknown): unknown } | undefined)?.setData(colour);
  };
  const teardown = (): void => {
    destroyed = true;
    clearTimeout(attribTimer);
    clearTimeout(rangeTimer);
    unsubscribe?.();
    unsubscribe = null;
    unsubscribeRange?.();
    unsubscribeRange = null;
    unsubscribeUnits?.();
    unsubscribeUnits = null;
    for (const [target, type, fn, opts] of listeners) target.removeEventListener(type, fn, opts);
    listeners.length = 0;
    query?.removeEventListener('change', onResolution);
    query = null;
    settle();
    marker?.remove();
    map.remove();
    inner.remove();
  };

  try {
    arm();
    // Tile and style errors: no console flood while offline, and no tile URLs (which encode
    // the map area) in the console. A failed tile leaves its square blank.
    map.on('error', swallow);
    map.touchZoomRotate.disableRotation();
    // One-finger tap-and-drag zoom off too (F-06 v1.2, ADR-015 rev. 2). In maplibre-gl 6.14.0
    // it is TapDragZoomHandler, enabled by default through touchZoomRotate.enable(); no map
    // option switches it, so it is disabled through `_tapDragZoom` (a public field in the
    // 6.14.0 .d.ts despite the underscore). Pinch zoom stays on. Caveat for F-26: calling
    // touchZoomRotate.enable() again re-enables it.
    map.touchZoomRotate._tapDragZoom.disable();
    map.addControl(new NavigationControl({ showCompass: false }), 'top-right');
    const canvas = map.getCanvas();

    // ---- cursor marker (D-8, D-11): one DOM Marker, moved in the cursor listener ----------
    const first = route.indices[0];
    const at = new LngLat(lon[first], lat[first]); // reused for every move
    const element = document.createElement('div');
    element.className = 'wx-cursor-marker';
    element.style.visibility = 'hidden';
    const m = new Marker({ element, anchor: 'center', subpixelPositioning: true }).setLngLat(at).addTo(map);
    marker = m;
    let shown = false;
    // Hot path: no allocation of our own, no layout read, no repaint (no setData).
    const onCursor = (i: CursorIndex): void => {
      if (i === null || lat[i] !== lat[i]) {
        if (shown) {
          element.style.visibility = 'hidden';
          shown = false;
        }
        return;
      }
      at.lng = lon[i];
      at.lat = lat[i];
      m.setLngLat(at);
      if (!shown) {
        element.style.visibility = 'visible';
        shown = true;
      }
    };
    unsubscribe = cursor.subscribe(onCursor);

    // ---- range highlight (F-17 R-10): one setData after the range settles, never per frame ----
    const applyRange = (): void => {
      (map.getSource(RANGE_SOURCE) as { setData(d: unknown): unknown } | undefined)?.setData(range);
      emphasised = classes !== null && range.features.length > 0;
      applyEmphasis();
    };
    if (selection) {
      let first = true;
      unsubscribeRange = selection.subscribe((a, b) => {
        if (first) {
          first = false; // already in the base style
          return;
        }
        clearTimeout(rangeTimer);
        rangeTimer = setTimeout(() => {
          if (destroyed) return;
          range = rangeOf(a, b);
          applyRange();
        }, RANGE_SETTLE_MS);
      });
      map.on('style.load', applyRange);
    }
    map.on('style.load', applyHidden);
    map.on('style.load', applyColour);

    // ---- route colouring (F-25 C-6, C-8): a select and its legend; work only on a selection ----
    if (offered.length > 0) {
      const sport = data.meta.sport;
      const box = document.createElement('div');
      box.className = 'maplibregl-ctrl wx-colour';
      const select = document.createElement('select');
      select.setAttribute('aria-label', 'Route colour');
      select.add(new Option('Plain route', ''));
      for (const m of offered) select.add(new Option(metricLabel(m, sport), m));
      const legendBox = document.createElement('div');
      legendBox.className = 'wx-legend';
      legendBox.hidden = true;
      box.append(select, legendBox);
      let shown: { scale: ReturnType<typeof colourScale>; missing: boolean } | null = null;
      const span = (className: string, text = ''): HTMLSpanElement => {
        const el = document.createElement('span');
        el.className = className;
        el.textContent = text;
        return el;
      };
      const renderLegend = (): void => {
        legendBox.hidden = shown === null;
        if (shown === null) return;
        const { scale, missing } = shown;
        const offset = scale.metric === 'gradient' ? DIVERGING : 0;
        const { labels, unit } = legend(scale, sport, options.units?.get() ?? 'metric');
        const swatches = document.createElement('div');
        swatches.className = 'wx-swatches';
        for (let k = 0; k < CLASS_COUNT; k++) swatches.append(span('wx-c' + (offset + k)));
        const row = document.createElement('div');
        row.className = 'wx-labels';
        if (scale.metric === 'gradient') {
          labels.forEach((t, k) => row.append(span('wx-l' + (k + 1), t)));
          row.append(span('wx-l7', unit));
        } else {
          row.append(span('wx-l0', labels[0]), span('wx-l7', labels[1] + ' ' + unit));
        }
        legendBox.replaceChildren(swatches, row);
        if (missing) {
          const none = document.createElement('div');
          none.className = 'wx-nodata';
          none.append(span('wx-cn'), 'no data');
          legendBox.append(none);
        }
      };
      // Viewer-caused, off the scrub path: values, scale, classes and geometry, then one setData (C-8).
      const choose = (metric: ColourMetric | ''): void => {
        if (metric === '') {
          shown = null;
          classes = null;
          colour = colourData([]);
        } else {
          ends ??= recordedEdgeEnds(data, route);
          const values = metricValues(data, metric);
          const scale = colourScale(metric, values, ends, sport);
          const offset = metric === 'gradient' ? DIVERGING : 0;
          const cs = classify(values, scale).map((k) => (k < CLASS_COUNT ? offset + k : NO_VALUE));
          classes = cs;
          shown = { scale, missing: ends.some((i) => cs[i] === NO_VALUE) };
          colour = colourData(classLines(data, ends, cs, NO_VALUE));
        }
        applyColour();
        renderLegend();
        if (selection) {
          clearTimeout(rangeTimer);
          range = rangeOf(selection.start(), selection.end());
          applyRange();
        }
      };
      select.addEventListener('change', () => choose(select.value as ColourMetric | ''));
      if (options.units) unsubscribeUnits = options.units.subscribe(renderLegend);
      map.addControl({ onAdd: () => box, onRemove: () => box.remove() }, 'bottom-left');
    }

    // ---- route interaction (D-9, D-10, ADR-015) --------------------------------------------
    let state: RouteGestureState = ROUTE_IDLE;
    const input: MutableInput = { kind: 'move', pointerType: '', pointerId: 0, isPrimary: true, buttons: 0, hit: -1, current: null };
    /** Cached rectangle of MapLibre's container; null = stale. */
    let rect: DOMRect | null = null;
    const xy: [number, number] = [0, 0];

    /** Sample index of the route under the pointer, or −1. Over a map control (zoom buttons,
     *  attribution) nothing is hit: only the map surface (the canvas) counts. */
    const hitAt = (e: PointerEvent, kind: RouteGestureKind): number => {
      if (e.target !== canvas) return -1;
      if (rect === null) rect = inner.getBoundingClientRect();
      xy[0] = e.clientX - rect.left;
      xy[1] = e.clientY - rect.top;
      const ll = map.unproject(xy);
      const unit = 1 / (WORLD_PX * 2 ** map.getZoom()); // Mercator units per CSS px
      return index.nearest(mercatorX(ll.lng), mercatorY(ll.lat), hitRadiusPx(state, e.pointerType, kind) * unit, TIE_PX * unit);
    };
    const handle = (kind: RouteGestureKind, e: PointerEvent): void => {
      input.kind = kind;
      input.pointerType = e.pointerType;
      input.pointerId = e.pointerId;
      input.isPrimary = e.isPrimary;
      input.buttons = e.buttons;
      // No hit test the reducer would discard (v1.3, QA-06-r1-4).
      input.hit = wantsHit(state, kind, e.pointerType, e.pointerId, e.isPrimary, e.buttons) ? hitAt(e, kind) : -1;
      input.current = cursor.get();
      const r = routeStep(state, input);
      state = r.state;
      if (r.action.kind === 'set') cursor.set(r.action.index);
    };

    const onEnter = (): void => {
      rect = inner.getBoundingClientRect();
    };
    const onDown = (e: PointerEvent): void => {
      rect = inner.getBoundingClientRect();
      handle('down', e);
    };
    const onMove = (e: PointerEvent): void => handle('move', e);
    const onUp = (e: PointerEvent): void => handle('up', e);
    const onCancel = (e: PointerEvent): void => handle('cancel', e);
    const onLeave = (e: PointerEvent): void => handle('leave', e);
    const onScroll = (): void => {
      rect = null;
    };
    // The route-scrub lock (ADR-015 decision 3): while one finger scrubs, the page does not
    // scroll and MapLibre never sees the move. Like the chart lock: no allocation, no layout
    // read, never set.
    const onTouchMove = (e: TouchEvent): void => {
      if (interceptsTouch(state, e.touches.length)) {
        if (e.cancelable) e.preventDefault();
        e.stopPropagation();
      }
    };

    const capturePassive: AddEventListenerOptions = { passive: true, capture: true };
    const captureBlocking: AddEventListenerOptions = { passive: false, capture: true };
    listen(container, 'pointerenter', onEnter, capturePassive);
    listen(container, 'pointerdown', onDown, capturePassive);
    listen(container, 'pointermove', onMove, capturePassive);
    listen(container, 'pointerup', onUp, capturePassive);
    listen(container, 'pointercancel', onCancel, capturePassive);
    listen(container, 'pointerleave', onLeave, capturePassive);
    // Added here, at mount, never during a touch: iOS WebKit decides at touch start whether a
    // blocking listener exists (WebKit bug 184251).
    listen(container, 'touchmove', onTouchMove, captureBlocking);
    listen(window, 'scroll', onScroll, capturePassive);
    map.on('resize', onScroll); // the rectangle is stale after a resize too
    // Refit until moved (F-13 design D-20): the box changes size by CSS (pinning, rotation), and
    // MapLibre resizes itself. While the viewer has not moved the camera, the route stays fitted;
    // afterwards a resize keeps centre and zoom. Events without originalEvent are not the viewer's.
    const [west, south, east, north] = route.bounds;
    if (!(west === east && south === north)) {
      let moved = false;
      const onViewerMove = (e: { originalEvent?: unknown }): void => {
        if (e.originalEvent) moved = true;
      };
      map.on('movestart', onViewerMove);
      map.on('zoomstart', onViewerMove);
      map.on('resize', () => {
        if (!moved && !destroyed) {
          map.fitBounds([west, south, east, north], { padding: fitPadding(), maxZoom: 16, animate: false });
        }
      });
    }

    // ---- remote style, merged in after load (D-5) ------------------------------------------
    let loaded = false;
    let remote: StyleSpecification | null = null;
    let applying = false;
    let appliedStyle: StyleSpecification | null = null;
    const applyRemote = async (): Promise<void> => {
      if (applying || destroyed || !loaded || remote === null || remote === appliedStyle) return;
      applying = true;
      await whenQuiet();
      applying = false;
      if (destroyed || remote === null || remote === appliedStyle) return;
      appliedStyle = remote;
      try {
        const styleStart = performance.now();
        // diff: false (v1.4): a diff from the small base style to liberty's 111 layers fails
        // anyway and falls back to a full rebuild; this skips serialising and diffing both.
        // The merge carries wx-range as serialized; 'style.load' then applies the latest range.
        map.setStyle(remote, { diff: false, transformStyle: mergeStyle });
        performance.measure('wx-map:set-style', { start: styleStart });
        // The first idle after the remote style: tiles, glyphs and sprite drawn (User Timing, v1.4).
        map.once('idle', () => performance.mark('wx-map:idle'));
      } catch {
        // The base style stays: route and markers on a plain background.
      }
    };
    // Phones (< 1024 px): the attribution stays expanded for 5 s after its text first appears, or
    // until the first interaction with the map, whichever comes first, then collapses to its (i)
    // button; a tap on (i) opens it again and it stays open (F-13 D-20, QA-13-r2-4). The credits
    // appear late (after the style), so the clock starts at the first idle that finds text.
    if (typeof matchMedia === 'function' && matchMedia('(max-width: 1023.98px)').matches) {
      const attrib = inner.querySelector<HTMLElement>('.maplibregl-ctrl-attrib');
      let state: 'waiting' | 'timing' | 'done' = 'waiting';
      const collapse = (): void => {
        if (state === 'done') return;
        state = 'done';
        clearTimeout(attribTimer);
        attrib?.classList.remove('maplibregl-compact-show');
        attrib?.removeAttribute('open');
      };
      if (attrib) {
        // A tap on (i) is the viewer's choice: leave it as they set it. Other pointer input on the map collapses.
        const onTouch = (e: Event): void => {
          if (attrib.contains(e.target as Node)) state = 'done';
          else collapse();
        };
        for (const type of ['pointerdown', 'wheel'] as const) {
          inner.addEventListener(type, onTouch, { capture: true });
          listeners.push([inner, type, onTouch, { capture: true }]);
        }
        map.on('idle', () => {
          if (state !== 'waiting' || !attrib.textContent?.trim()) return;
          state = 'timing';
          attribTimer = setTimeout(collapse, 5000);
        });
      }
    }
    map.on('load', () => {
      performance.mark('wx-map:load');
      loaded = true;
      void applyRemote();
    });
    void remoteStyle.then((style) => {
      remote ??= style;
      void applyRemote();
    });
    setScheme = (next: Scheme): void => {
      if (next === scheme || destroyed) return;
      scheme = next;
      // Our layers first, so the merge below carries the new colours over; the base style
      // keeps drawing if the other style never arrives ("where available", question 28).
      recolour(map, mapColours(container), classColours(container));
      applyEmphasis();
      if (options.styleUrl !== undefined) return; // an override has no other scheme
      void fetchStyle(styleUrlFor(next)).then((style) => {
        if (style === null || scheme !== next || destroyed) return;
        remote = style;
        void applyRemote();
      });
    };
  } catch (error) {
    teardown();
    throw error;
  }

  return {
    setScheme: (next) => setScheme(next),
    setHidden(ranges): void {
      if (destroyed) return;
      const lines = { recorded: [] as [number, number][][], bridges: [] as [[number, number], [number, number]][] };
      for (const [a, b] of ranges) {
        const l = routeLinesIn(data, route, a, b);
        lines.recorded.push(...l.recorded);
        lines.bridges.push(...l.bridges);
      }
      hidden = linesData(lines);
      applyHidden();
    },
    destroy(): void {
      if (!destroyed) teardown();
    },
  };
}
