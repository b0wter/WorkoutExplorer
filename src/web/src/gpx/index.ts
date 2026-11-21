// "Download route (GPX)" (F-57 GX-6, GX-7; SH-13). LAZY: reached only through enhanceGpx's import(),
// on the viewing page only (GX-2: the edit page's data is the unhidden draft). Nothing is fetched:
// the file is built from the already decoded public data and handed over as a blob: link.

import { gpxFileName, gpxParts, type GpxInput } from '../lib/gpx';
import type { ShownPin } from '../lib/pins';

/** Long enough for any browser to start the download (40 s as FileSaver.js: iOS Safari's prompt
 *  stays open; QA-57-r1 Q2); a new task, never the click's (GX-7). */
const REVOKE_MS = 40_000;

/** The public pins (GX-10), fetched on click; a failed or invalid file gives the route without waypoints. */
async function publicPins(input: GpxInput): Promise<ShownPin[] | undefined> {
  if (!input.pinsUrl) return undefined;
  try {
    const [r, { readPins }] = await Promise.all([fetch(input.pinsUrl, { signal: AbortSignal.timeout(5000) }), import('../lib/pins')]); // GX-10: no pin code without a pins file
    return (r.ok && readPins(await r.json(), input.latitude.length)) || undefined;
  } catch {
    return undefined;
  }
}

export async function downloadGpx(doc: Document, input: GpxInput): Promise<void> {
  const url = URL.createObjectURL(new Blob(gpxParts(input, await publicPins(input)), { type: 'application/gpx+xml' }));
  const a = doc.createElement('a');
  a.href = url;
  a.download = gpxFileName(input);
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), REVOKE_MS);
}
