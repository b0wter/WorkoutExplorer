// GPX of the visible route (F-57 GX-2 to GX-4; SH-13). Pure: no DOM. Only the lazy chunk src/gpx
// imports it. PRIVACY (GX-2): the inputs are positions, altitude and the header's start; no time
// stream, no other metric. The track is numbers only; the only text are the public pins' (waypoints,
// GX-10), XML-escaped.
import { pinName, type ShownPin } from '../pins';

/** Closed inputs (GX-2): from the public data file only. */
export interface GpxInput {
  /** Degrees; NaN where the sample has no position (hidden, trimmed, dropout). */
  readonly latitude: Float64Array;
  readonly longitude: Float64Array;
  /** Metres; NaN where not recorded. Absent without an altitude stream. */
  readonly altitude?: Float64Array;
  /** `meta.start_time` and `meta.time_zone`: the date the header shows (GX-4). */
  readonly startTime: string;
  readonly zone: { readonly known: boolean; readonly offset_seconds: number };
  /** The public pins file's URL (GX-10); absent without public pins. */
  readonly pinsUrl?: string;
}

const HEAD_A = '<?xml version="1.0" encoding="UTF-8"?>\n<gpx version="1.1" creator="Workout Explorer" xmlns="http://www.topografix.com/GPX/1/1">\n';
const HEAD_B = '<trk><trkseg>\n';
const CHUNK = 1024;
const TAIL = '</trkseg></trk>\n</gpx>\n';

/** `v` at the data file's quantum (10^-digits), as exact decimal text: integer arithmetic, never
 *  float formatting (GX-3: no false precision). */
function fixed(v: number, digits: number, scale: number): string {
  const n = Math.round(v * scale);
  const s = String(Math.abs(n)).padStart(digits + 1, '0');
  return (n < 0 ? '-' : '') + s.slice(0, -digits) + '.' + s.slice(-digits);
}

/** The file as Blob parts (GX-7): head, one line per positioned sample in index order, tail. One
 *  segment, joined across dropouts and pauses (OQ-270); `<ele>` only where altitude is recorded. */
const esc = (s: string): string => s.replace(/[&<>]/g, (c) => (c === '&' ? '&amp;' : c === '<' ? '&lt;' : '&gt;'));

/** One `<wpt>` per public pin end that has a position (GX-10); a section's ends are "(start)" and "(end)". */
function waypoints(pins: readonly ShownPin[], lat: Float64Array, lon: Float64Array): string {
  let out = '';
  const wpt = (i: number, name: string, p: ShownPin): void => {
    if (lat[i] !== lat[i] || lon[i] !== lon[i]) return;
    out += '<wpt lat="' + fixed(lat[i], 6, 1e6) + '" lon="' + fixed(lon[i], 6, 1e6) + '"><name>' + esc(name) + '</name>' + (p.note ? '<desc>' + esc(p.note) + '</desc>' : '') + '<type>' + p.type + '</type></wpt>\n';
  };
  for (const p of pins) {
    const n = pinName(p);
    if (p.to === undefined) wpt(p.from, n, p);
    else {
      wpt(p.from, n + ' (start)', p);
      wpt(p.to, n + ' (end)', p);
    }
  }
  return out;
}

export function gpxParts(input: Pick<GpxInput, 'latitude' | 'longitude' | 'altitude'>, pins?: readonly ShownPin[]): string[] {
  const { latitude: lat, longitude: lon, altitude: alt } = input;
  const parts = [HEAD_A + (pins?.length ? waypoints(pins, lat, lon) : '') + HEAD_B];
  // Lines joined per CHUNK: a Blob from tens of thousands of parts is several times slower (GX-8).
  let chunk = '';
  for (let i = 0; i < lat.length; i++) {
    if (lat[i] !== lat[i] || lon[i] !== lon[i]) continue;
    const pt = '<trkpt lat="' + fixed(lat[i], 6, 1e6) + '" lon="' + fixed(lon[i], 6, 1e6) + '"';
    const a = alt ? alt[i] : NaN;
    chunk += a === a ? pt + '><ele>' + fixed(a, 1, 10) + '</ele></trkpt>\n' : pt + '/>\n';
    if (i % CHUNK === CHUNK - 1) {
      parts.push(chunk);
      chunk = '';
    }
  }
  parts.push(chunk + TAIL);
  return parts;
}

/** `route-YYYY-MM-DD.gpx`: the header's date (start shifted by the zone offset when known, else
 *  UTC, as lib/format/dates), never a sample's time (GX-4); `route.gpx` without a start date. */
export function gpxFileName(input: Pick<GpxInput, 'startTime' | 'zone'>): string {
  const at = Date.parse(input.startTime);
  if (at !== at) return 'route.gpx';
  const d = new Date(at + (input.zone.known ? input.zone.offset_seconds : 0) * 1000);
  return 'route-' + d.toISOString().slice(0, 10) + '.gpx';
}
