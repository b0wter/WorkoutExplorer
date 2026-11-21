// F-62 "Show my location" (AP-25; design LC-1 to LC-10). LAZY: loaded by map-view's import() on the
// first location or follow tap, viewing page only. MapLibre as types only (ADR-013): map-view hands
// over the map and its Marker class.
//
// LC-10: the position lives only in this module. It never leaves it: no export, event, store or
// callback carries it; this file and location-state.ts use no network, storage, URL, history,
// messaging or console API (checked by location.test.ts). The only exposure is the tile requests
// for the area the map centres on (About page).

import type { Map as MapLibreMap, Marker } from 'maplibre-gl';
import { LOC_OFF, locStep, type LocEvent } from './location-state';

export interface LocationParts {
  readonly map: MapLibreMap;
  readonly Marker: typeof Marker;
  /** The location toggle (aria-pressed) and the follow button (hidden while not on). */
  readonly toggle: HTMLButtonElement;
  readonly follow: HTMLButtonElement;
  /** Shows a message of the map texts in the map box (texts stay in the map chunk: no import back into it). */
  readonly say: (key: 'locDenied' | 'locFailed') => void;
  /** Ends F-13 D-20's "refit until moved" (LC-5). */
  readonly centred: () => void;
}

export interface LocationHandle {
  tap(): void;
  follow(): void;
  /** Fully off and every listener gone (map teardown). */
  stop(): void;
}

/** Zoom when centring, at least (walking a route); a closer zoom of the viewer's stays. */
const ZOOM = 16;
/** Equator length in metres (Web Mercator) / 512 CSS px world at zoom 0. */
const M_PER_PX_Z0 = 40075016.686 / 512;
const OPTIONS: PositionOptions = { enableHighAccuracy: true, maximumAge: 10_000, timeout: 30_000 };

export function startLocation({ map, Marker, toggle, follow, say, centred }: LocationParts): LocationHandle {
  const geo = window.isSecureContext ? navigator.geolocation : undefined;
  let s = LOC_OFF;
  let watch: number | null = null;
  let fix: GeolocationCoordinates | null = null;
  let shown = false;
  const el = (className: string): HTMLDivElement => {
    const d = document.createElement('div');
    d.className = className;
    return d;
  };
  const ringEl = el('wx-loc-ring');
  const ring = new Marker({ element: ringEl, anchor: 'center', subpixelPositioning: true });
  const dot = new Marker({ element: el('wx-loc-dot'), anchor: 'center', subpixelPositioning: true });

  const unwatch = (): void => {
    if (watch !== null) geo?.clearWatch(watch);
    watch = null;
  };
  // At most one watch, never while the page is hidden (LC-7, LC-9).
  const rewatch = (): void => {
    if (s.phase === 'off' || watch !== null || !geo || document.visibilityState === 'hidden') return;
    watch = geo.watchPosition(
      (p) => send('fix', p.coords),
      (e) => send(e.code === 1 ? 'denied' : e.code === 3 ? 'timeout' : 'failed'), // 1: PERMISSION_DENIED, 3: TIMEOUT
      OPTIONS,
    );
  };
  // The accuracy circle: its radius in CSS px at the position's latitude and the current zoom (LC-6).
  const size = (): void => {
    if (!fix) return;
    const d = Math.min((2 * fix.accuracy * 2 ** map.getZoom()) / (M_PER_PX_Z0 * Math.cos((fix.latitude * Math.PI) / 180)), 4096);
    ringEl.style.width = ringEl.style.height = d + 'px';
  };

  const send = (e: LocEvent, coords?: GeolocationCoordinates): void => {
    const r = locStep(s, e);
    s = r.state;
    if (s.phase === 'off') {
      unwatch();
      fix = null;
      ring.remove();
      dot.remove();
      shown = false;
    } else {
      if (coords) fix = coords;
      rewatch();
      if (fix && s.phase === 'on') {
        const at = { lng: fix.longitude, lat: fix.latitude };
        ring.setLngLat(at);
        dot.setLngLat(at);
        if (!shown) {
          ring.addTo(map); // below the dot
          dot.addTo(map);
          shown = true;
        }
        if (r.centre) {
          centred();
          map.jumpTo({ center: at, zoom: Math.max(map.getZoom(), ZOOM) });
        }
        size(); // after the centring's zoom
      }
    }
    toggle.setAttribute('aria-pressed', String(s.phase !== 'off'));
    follow.hidden = s.phase !== 'on';
    follow.setAttribute('aria-pressed', String(s.following));
    if (r.message) say(r.message === 'denied' ? 'locDenied' : 'locFailed'); // our text, never error.message
  };

  // LC-4: a viewer's move carries originalEvent; our jumpTo and a resize do not.
  const onMove = (e: { originalEvent?: unknown }): void => {
    if (e.originalEvent && s.following) send('move');
  };
  const onVisibility = (): void => (document.visibilityState === 'hidden' ? unwatch() : rewatch());
  const onPageHide = (): void => send('off');
  map.on('movestart', onMove);
  map.on('zoomstart', onMove);
  map.on('zoom', size);
  document.addEventListener('visibilitychange', onVisibility);
  window.addEventListener('pagehide', onPageHide);

  return {
    tap(): void {
      send('tap');
      if (!geo && s.phase === 'starting') send('failed'); // insecure context or no API
    },
    follow: () => send('follow'),
    stop(): void {
      send('off');
      map.off('movestart', onMove);
      map.off('zoomstart', onMove);
      map.off('zoom', size);
      document.removeEventListener('visibilitychange', onVisibility);
      window.removeEventListener('pagehide', onPageHide);
    },
  };
}
