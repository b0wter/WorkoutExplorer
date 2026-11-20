// "Download route (GPX)" (F-57 GX-6, GX-7; SH-13). LAZY: reached only through enhanceGpx's import(),
// on the viewing page only (GX-2: the edit page's data is the unhidden draft). Nothing is fetched:
// the file is built from the already decoded public data and handed over as a blob: link.

import { gpxFileName, gpxParts, type GpxInput } from '../lib/gpx';

/** Long enough for any browser to start the download; a new task, never the click's (GX-7). */
const REVOKE_MS = 10_000;

export function downloadGpx(doc: Document, input: GpxInput): void {
  const url = URL.createObjectURL(new Blob(gpxParts(input), { type: 'application/gpx+xml' }));
  const a = doc.createElement('a');
  a.href = url;
  a.download = gpxFileName(input);
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), REVOKE_MS);
}
