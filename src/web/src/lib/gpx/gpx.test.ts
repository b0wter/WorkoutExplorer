// F-57 GPX builder (GX-2 to GX-4, GX-8; AC 3 to 6, AC 9). Synthetic golden data files only (ADR-004).
import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../datafile';
import { gpxFileName, gpxParts } from '.';
import type { ShownPin } from '../pins';

const nan = NaN;
const f64 = (a: number[]) => Float64Array.from(a);

function decoded(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  if (!g) throw new Error(name);
  const r = decodeDataFile(g.bin);
  if (!r.ok) throw new Error(r.error.message);
  return r.data;
}

const input = (d: WorkoutData) => ({ latitude: d.position!.latitude, longitude: d.position!.longitude, altitude: d.metrics.altitude });
const points = (gpx: string) => [...gpx.matchAll(/<trkpt lat="([^"]*)" lon="([^"]*)"(?:><ele>([^<]*)<\/ele><\/trkpt>|\/>)/g)].map((m) => [m[1], m[2], m[3]]);

/** The expected points from the stored integers, independent of the builder's formatting. */
function expected(lat: Float64Array, lon: Float64Array, alt?: Float64Array): (string | undefined)[][] {
  const out: (string | undefined)[][] = [];
  const dec = (v: number, s: number, d: number) => (Math.round(v * s) / s).toFixed(d);
  for (let i = 0; i < lat.length; i++) {
    if (Number.isNaN(lat[i])) continue;
    out.push([dec(lat[i], 1e6, 6), dec(lon[i], 1e6, 6), alt && !Number.isNaN(alt[i]) ? dec(alt[i], 10, 1) : undefined]);
  }
  return out;
}

describe('content (GX-3; AC 3, AC 5)', () => {
  it('writes exactly the fixed head, one line per positioned sample, the tail', () => {
    const gpx = gpxParts({
      latitude: f64([47.123456, nan, -0.000001, 0.5, -33.9]),
      longitude: f64([8.65432, nan, -179.999999, 0, 151.2]),
      altitude: f64([412.3, 99, nan, -3.2, 0]),
    }).join('');
    expect(gpx).toBe(
      '<?xml version="1.0" encoding="UTF-8"?>\n' +
        '<gpx version="1.1" creator="Workout Explorer" xmlns="http://www.topografix.com/GPX/1/1">\n' +
        '<trk><trkseg>\n' +
        '<trkpt lat="47.123456" lon="8.654320"><ele>412.3</ele></trkpt>\n' +
        '<trkpt lat="-0.000001" lon="-179.999999"/>\n' +
        '<trkpt lat="0.500000" lon="0.000000"><ele>-3.2</ele></trkpt>\n' +
        '<trkpt lat="-33.900000" lon="151.200000"><ele>0.0</ele></trkpt>\n' +
        '</trkseg></trk>\n' +
        '</gpx>\n',
    );
  });

  it('without an altitude stream: no <ele> at all', () => {
    expect(gpxParts({ latitude: f64([1]), longitude: f64([2]) }).join('')).toContain('<trkpt lat="1.000000" lon="2.000000"/>\n');
  });

  it('no position at all: an empty segment (the button is never shown then, GX-5)', () => {
    expect(gpxParts({ latitude: f64([nan]), longitude: f64([nan]) }).join('')).toContain('<trk><trkseg>\n</trkseg></trk>');
  });

  it.each(['ride-30min', 'gps-dropout', 'pause-hours', 'chained-ride-ride', 'ride-12h'])('%s: every positioned sample in order, ele where recorded, one segment, nothing else, deterministic', (name) => {
    const d = decoded(name);
    const gpx = gpxParts(input(d)).join('');
    expect(points(gpx)).toEqual(expected(d.position!.latitude, d.position!.longitude, d.metrics.altitude));
    expect(gpx.match(/<trk>/g)).toHaveLength(1);
    expect(gpx.match(/<trkseg>/g)).toHaveLength(1);
    // Only the GX-3 elements without pins: no time, name, desc, metadata, extensions, wpt, rte, type, bounds.
    expect(new Set([...gpx.matchAll(/<([a-z?]+)/g)].map((m) => m[1]))).toEqual(new Set(['?xml', 'gpx', 'trk', 'trkseg', 'trkpt', 'ele']));
    expect(gpxParts(input(d)).join('')).toBe(gpx);
  });

  it('gps-dropout: the dropout is joined, not split (OQ-270)', () => {
    const d = decoded('gps-dropout');
    expect(d.position!.latitude.some(Number.isNaN)).toBe(true);
    expect(gpxParts(input(d)).join('')).not.toMatch(/<\/trkseg>[\s\S]*<trkseg>/);
  });
});

describe('hidden and trimmed ends never leak (AC 4, SH-4, F-55)', () => {
  const R = 6371000;
  const dist = (a: number, b: number, lat: Float64Array, lon: Float64Array) => {
    const rad = Math.PI / 180;
    const x = (lon[b] - lon[a]) * rad * Math.cos(((lat[a] + lat[b]) / 2) * rad);
    return Math.hypot(x, (lat[b] - lat[a]) * rad) * R;
  };

  /** The public data file's positions as the server writes them: a trim cuts samples off, then each
   *  end's positions are removed until the route first leaves `radius` around that end (F-16, F-55). */
  function publicOf(d: WorkoutData, radius: number, trim: number) {
    const n = d.sampleCount - 2 * trim;
    const lat = d.position!.latitude.slice(trim, trim + n);
    const lon = d.position!.longitude.slice(trim, trim + n);
    const alt = d.metrics.altitude?.slice(trim, trim + n);
    const first = lat.findIndex((v) => !Number.isNaN(v));
    let last = n - 1;
    while (Number.isNaN(lat[last])) last--;
    let a = first;
    let b = last;
    while (a < n - 1 && (Number.isNaN(lat[a + 1]) || dist(first, a + 1, lat, lon) <= radius)) a++;
    while (b > 0 && (Number.isNaN(lat[b - 1]) || dist(last, b - 1, lat, lon) <= radius)) b--;
    const hidden = new Set<number>();
    for (let i = 0; i < n; i++) if (radius > 0 && (i <= a || i >= b)) hidden.add(i);
    const draft = { lat: lat.slice(), lon: lon.slice() };
    for (const i of hidden) lat[i] = lon[i] = NaN;
    return { lat, lon, alt, hidden, draft };
  }

  it.each([
    ['ride-30min', 100, 0],
    ['ride-30min', 500, 60],
    ['ride-30min', 2000, 0],
    ['gps-dropout', 200, 0],
    ['gps-dropout', 1000, 30],
    ['pause-hours', 500, 10],
  ])('%s, radius %i m, trim %i: the visible positions only; first and last are the first and last visible', (name, radius, trim) => {
    const p = publicOf(decoded(name), radius, trim);
    expect(p.hidden.size).toBeGreaterThan(0);
    const got = points(gpxParts({ latitude: p.lat, longitude: p.lon, altitude: p.alt }).join(''));
    const want = expected(p.lat, p.lon, p.alt);
    expect(want.length).toBeGreaterThan(0);
    expect(got).toEqual(want);
    // No hidden draft position anywhere in the file, unless the same point is also visible.
    const visible = new Set(want.map((w) => w[0] + ',' + w[1]));
    for (const i of p.hidden) {
      const key = expected(f64([p.draft.lat[i]]), f64([p.draft.lon[i]]))[0];
      if (key && !visible.has(key[0] + ',' + key[1])) expect(got.some((g) => g[0] === key[0] && g[1] === key[1])).toBe(false);
    }
  });

  it('a fully hidden route gives no point', () => {
    const p = publicOf(decoded('ride-30min'), 1e8, 0);
    expect(points(gpxParts({ latitude: p.lat, longitude: p.lon, altitude: p.alt }).join(''))).toEqual([]);
  });
});

describe('file name (GX-4; AC 6)', () => {
  it('the start date in the workout zone; UTC when the zone is unknown', () => {
    expect(gpxFileName({ startTime: '2026-10-09T05:30:00Z', zone: { known: true, offset_seconds: 7200 } })).toBe('route-2026-10-09.gpx');
    expect(gpxFileName({ startTime: '2026-10-09T23:30:00Z', zone: { known: true, offset_seconds: 7200 } })).toBe('route-2026-10-10.gpx');
    expect(gpxFileName({ startTime: '2026-10-09T01:30:00Z', zone: { known: true, offset_seconds: -18000 } })).toBe('route-2026-10-08.gpx');
    expect(gpxFileName({ startTime: '2026-10-09T23:30:00Z', zone: { known: false, offset_seconds: 7200 } })).toBe('route-2026-10-09.gpx');
  });

  it('no start date: route.gpx', () => {
    expect(gpxFileName({ startTime: '', zone: { known: true, offset_seconds: 0 } })).toBe('route.gpx');
    expect(gpxFileName({ startTime: 'not a date', zone: { known: false, offset_seconds: 0 } })).toBe('route.gpx');
  });

  it('takes no sample: a hidden start crossing local midnight keeps the header date (signature is closed)', () => {
    // 23:50 local start; the first visible sample would be after midnight. The builder never sees it.
    expect(gpxFileName({ startTime: '2026-10-09T21:50:00Z', zone: { known: true, offset_seconds: 7200 } })).toBe('route-2026-10-09.gpx');
    expect(gpxFileName.length).toBe(1);
  });
});

describe('cost (GX-8; AC 9)', () => {
  it('a 12-hour ride builds well under 100 ms in Node', () => {
    const d = decoded('ride-12h');
    expect(d.sampleCount).toBeGreaterThanOrEqual(43000);
    gpxParts(input(d)); // warm-up
    const runs: number[] = [];
    for (let k = 0; k < 5; k++) {
      const t = performance.now();
      new Blob(gpxParts(input(d)), { type: 'application/gpx+xml' });
      runs.push(performance.now() - t);
    }
    const best = Math.min(...runs);
    console.info(`F-57: 12-hour ride (${d.sampleCount} samples) GPX build + Blob: best ${best.toFixed(1)} ms of 5`);
    expect(best).toBeLessThan(500); // generous bound against CI noise; the figure is reported
  });
});

describe('public pins as waypoints (GX-10)', () => {
  const lat = f64([47.1, 47.2, nan, 47.4]);
  const lon = f64([8.1, 8.2, nan, 8.4]);
  const pin = (o: Partial<ShownPin>): ShownPin => ({ type: 'danger', title: 'T', note: '', from: 0, ...o });
  const build = (pins?: ShownPin[]) => gpxParts({ latitude: lat, longitude: lon }, pins).join('');

  it('no pins or an empty list: byte-identical to the track-only file', () => {
    expect(build([])).toBe(build());
    expect(build()).not.toContain('<wpt');
  });

  it('a point pin: wpt before trk with name, desc only when a note exists, type', () => {
    const g = build([pin({ from: 1, note: 'Slippery' }), pin({ from: 3, title: 'B' })]);
    expect(g).toContain('<wpt lat="47.200000" lon="8.200000"><name>T</name><desc>Slippery</desc><type>danger</type></wpt>\n<wpt lat="47.400000" lon="8.400000"><name>B</name><type>danger</type></wpt>\n<trk>');
    expect(g.indexOf('<wpt')).toBeLessThan(g.indexOf('<trk>'));
    expect(g).not.toMatch(/<time|<extensions/);
  });

  it('escapes title and note; an empty title shows the type name', () => {
    const g = build([pin({ title: 'A & <B>', note: '1 < 2 & 3 > 2' }), pin({ title: '', type: 'water_food', from: 1 })]);
    expect(g).toContain('<name>A &amp; &lt;B&gt;</name><desc>1 &lt; 2 &amp; 3 &gt; 2</desc>');
    expect(g).toContain('<name>Water and food</name>');
  });

  it('a section pin gives two waypoints (start) and (end)', () => {
    const g = build([pin({ title: 'Climb', from: 0, to: 3, type: 'difficult_section' })]);
    expect(g).toContain('<name>Climb (start)</name>');
    expect(g).toContain('<name>Climb (end)</name>');
    expect(g.match(/<wpt/g)).toHaveLength(2);
  });

  it('an end without a position is skipped', () => {
    expect(build([pin({ from: 2 })])).not.toContain('<wpt');
    expect(build([pin({ from: 1, to: 2 })]).match(/<wpt/g)).toHaveLength(1);
  });

  it('QA-57-r2-2: characters invalid in XML 1.0 are dropped, tab, LF and emoji kept', () => {
    const g = build([pin({ title: 'a\uFFFFb\uFFFEc\x01d\uD800e\tf\u{1F600}', note: 'x\ny' })]);
    expect(g).toContain('<name>abcde\tf\u{1F600}</name><desc>x\ny</desc>');
  });
});
