// The preview basemap (F-36 MC-7, ADR-041, ADR-013 note). LAZY: reached only through import() from
// src/edit. The second value importer of 'maplibre-gl'. Renders OpenFreeMap's liberty style alone
// (no wx- source or layer, no route, no text of ours) at exactly the server's frame, offscreen,
// 1200 × 630 at pixel ratio 1, and returns the canvas as PNG. Never a capture of the visible map:
// that one is themed, draws the to-be-hidden sections and the cursor, and keeps no drawing buffer.

import { Map as MapLibreMap, setWorkerUrl } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url';
import './basemap.css';
import { OPENFREEMAP_STYLE_URL, isStyle, mergeStyle } from './map-style';

export interface Frame {
  readonly lon: number;
  readonly lat: number;
  readonly zoom: number;
}

/** Rejects on any failure (no WebGL2, a style, tile, glyph or sprite error, abort, toBlob); the
 *  map and its container are gone on every path. */
export async function renderBasemap(frame: Frame, signal: AbortSignal, styleUrl = OPENFREEMAP_STYLE_URL): Promise<Blob> {
  // Fetched by us, as map-view does (ADR-014): no credentials, no referrer, abortable.
  const response = await fetch(styleUrl, { credentials: 'omit', referrerPolicy: 'no-referrer', signal });
  const style: unknown = response.ok ? await response.json() : null;
  if (!isStyle(style)) throw new Error('basemap: no style');
  signal.throwIfAborted();

  const box = document.createElement('div');
  box.className = 'wx-basemap';
  document.body.appendChild(box);
  let map: MapLibreMap | null = null;
  let stop = (): void => {};
  try {
    setWorkerUrl(workerUrl);
    map = new MapLibreMap({
      container: box,
      style: mergeStyle(undefined, style), // light whatever the theme; Mercator, no terrain or sky
      center: [frame.lon, frame.lat],
      zoom: frame.zoom,
      bearing: 0,
      pitch: 0,
      interactive: false,
      attributionControl: false, // Go draws the attribution (MC-6)
      maplibreLogo: false,
      fadeDuration: 0,
      pixelRatio: 1,
      canvasContextAttributes: { preserveDrawingBuffer: true },
      validateStyle: import.meta.env.DEV,
    });
    const m = map;
    return await new Promise<Blob>((resolve, reject) => {
      stop = () => reject(signal.reason);
      signal.addEventListener('abort', stop);
      // Any error abandons the job; the reason may hold a tile URL, so it is never logged.
      m.on('error', () => reject(new Error('basemap: map error')));
      m.once('idle', () => {
        try {
          m.getCanvas().toBlob((b) => (b ? resolve(b) : reject(new Error('basemap: toBlob'))), 'image/png');
        } catch (e) {
          reject(e); // a tainted canvas (SecurityError): one more fallback
        }
      });
    });
  } finally {
    signal.removeEventListener('abort', stop);
    map?.remove();
    box.remove();
  }
}
