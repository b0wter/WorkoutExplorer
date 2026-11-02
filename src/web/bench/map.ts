// Dev-only page for F-06's browser checks (MD1–MD12, MP1–MP10) and its two measurements:
// scrubbing with the map visible (MD10, MP6) and the start-up window (MD11, MP1).
// architect/F-06-route-map-linked-to-cursor.md, "Dev page", "Manual checks".
// Never part of the production build; nothing in src/ imports it.
//
// Synthetic golden data files only (ADR-004 item 7). Two variants are built here from
// ride-4h: start and end hidden (positions NaN for the first and last 300 samples) and all
// positions hidden.

import { mount, unmount } from 'svelte';
import { decodeDataFile, type WorkoutData } from '../src/lib/datafile';
import { createCursorStore, type CursorIndex, type CursorStore } from '../src/lib/cursor';
import { presentCharts } from '../src/lib/charts';
import { routePresence } from '../src/lib/route';
import ChartStack from '../src/components/charts/ChartStack.svelte';
import RouteMap from '../src/components/map/RouteMap.svelte';
import { benchMapProps } from './map-props.svelte';
import ride12h from '../../server/testdata/datafile/synthetic/ride-12h.bin?url';
import ride4h from '../../server/testdata/datafile/synthetic/ride-4h.bin?url';
import gpsDropout from '../../server/testdata/datafile/synthetic/gps-dropout.bin?url';
import pauseHours from '../../server/testdata/datafile/synthetic/pause-hours.bin?url';
import brickRunBike from '../../server/testdata/datafile/synthetic/brick-run-bike.bin?url';
import noDistanceWithGps from '../../server/testdata/datafile/synthetic/no-distance-with-gps.bin?url';
import indoorNoGps from '../../server/testdata/datafile/synthetic/indoor-no-gps.bin?url';

interface Choice {
  readonly label: string;
  readonly url: string;
  readonly transform?: (d: WorkoutData) => WorkoutData;
}

const HIDDEN_ENDS = 300;

function hidePositions(d: WorkoutData, hide: (i: number) => boolean): WorkoutData {
  const p = d.position;
  if (!p) throw new Error('no position stream');
  const latitude = Float64Array.from(p.latitude);
  const longitude = Float64Array.from(p.longitude);
  for (let i = 0; i < d.sampleCount; i++) {
    if (hide(i)) {
      latitude[i] = Number.NaN;
      longitude[i] = Number.NaN;
    }
  }
  return { ...d, position: { latitude, longitude } };
}

const CHOICES: readonly Choice[] = [
  { label: 'ride-12h (24 laps, repeated loops; MD10/MD11 file)', url: ride12h },
  { label: 'ride-4h', url: ride4h },
  { label: 'gps-dropout (no positions 25:00–30:00)', url: gpsDropout },
  { label: 'pause-hours', url: pauseHours },
  { label: 'brick-run-bike (two sessions)', url: brickRunBike },
  { label: 'no-distance-with-gps', url: noDistanceWithGps },
  { label: 'indoor-no-gps (hint, no MapLibre)', url: indoorNoGps },
  {
    label: `ride-4h, start and end hidden (${HIDDEN_ENDS} samples each)`,
    url: ride4h,
    transform: (d) => hidePositions(d, (i) => i < HIDDEN_ENDS || i >= d.sampleCount - HIDDEN_ENDS),
  },
  { label: 'ride-4h, all positions hidden (nothing shown)', url: ride4h, transform: (d) => hidePositions(d, () => true) },
];

/** `.invalid` never resolves (RFC 2606): the tile host is unreachable (MD7, MP8). */
const BLOCKED_STYLE_URL = 'https://tiles.invalid/styles/liberty';

const SCRUB_MS = 10_000;
const EVENT_EVERY_MS = 8;
const MAX_SAMPLES = 8192; // frames or flushes in 10 s, with room for 240 Hz displays
const FRAME_OK_MS = 20;
const STARTUP_WINDOW_MS = 5_000;
const LONG_FRAME_MS = 50;
/** v1.4: idle frame-rate baseline right after the start-up window. */
const BASELINE_MS = 2_000;
/** A baseline median at or above this means rAF is capped at 30 Hz (iOS Low Power Mode). */
const CAPPED_MEDIAN_MS = 25;
/** User Timing names written by RouteMap and map-view (F-06 v1.4). */
const STEP_PREFIX = 'wx-map:';

const select = document.getElementById('file') as HTMLSelectElement;
const tiles = document.getElementById('tiles') as HTMLSelectElement;
const scrubButton = document.getElementById('scrub') as HTMLButtonElement;
const second = document.getElementById('second') as HTMLDivElement;
const times = document.getElementById('times') as HTMLPreElement;
const startup = document.getElementById('startup') as HTMLPreElement;
const info = document.getElementById('info') as HTMLPreElement;
const out = document.getElementById('out') as HTMLPreElement;
const mapTarget = document.getElementById('map') as HTMLDivElement;
const stackTarget = document.getElementById('stack') as HTMLDivElement;

CHOICES.forEach((c, k) => select.add(new Option(c.label, String(k))));

// Preallocated measurement buffers (no allocation per frame or flush).
const frameIntervals = new Float64Array(MAX_SAMPLES);
let frameCount = 0;
const flushTimes = new Float64Array(MAX_SAMPLES);
let flushCount = 0;
let flushStart = 0;
let measuring = false;

const startupIntervals = new Float64Array(MAX_SAMPLES);
/** End time (rAF timestamp) of startupIntervals[k]; the interval is [end − length, end]. */
const startupEnds = new Float64Array(MAX_SAMPLES);
let startupCount = 0;
const baselineIntervals = new Float64Array(MAX_SAMPLES);
let baselineCount = 0;

// The dev server serves many modules; keep the MapLibre chunk's entry in the buffer (v1.4).
performance.setResourceTimingBufferSize(2000);

interface Current {
  readonly apps: ReturnType<typeof mount>[];
  readonly cursor: CursorStore;
  readonly data: WorkoutData;
  readonly offs: (() => void)[];
  readonly choice: Choice;
  chartsReadyAt: number;
  mapReadyAt: number;
  startupDone: boolean;
}
let current: Current | null = null;
/** The first show after a page load is the cold one (MapLibre chunk not yet loaded). */
let shows = 0;

async function load(choice: Choice): Promise<WorkoutData> {
  const response = await fetch(choice.url);
  if (!response.ok) throw new Error(`HTTP ${response.status} for ${choice.url}`);
  const result = decodeDataFile(await response.arrayBuffer());
  if (!result.ok) throw new Error(result.error.message);
  return choice.transform ? choice.transform(result.data) : result.data;
}

function teardown(): void {
  if (!current) return;
  for (const off of current.offs) off();
  for (const app of current.apps) void unmount(app);
  current = null;
}

const ms = (v: number) => (Number.isNaN(v) ? '–' : v.toFixed(0));

function showTimes(c: Current): void {
  times.textContent =
    `charts onready: ${ms(c.chartsReadyAt)} ms · onmapready: ${ms(c.mapReadyAt)} ms (since navigation start)` +
    (Number.isNaN(c.mapReadyAt) || Number.isNaN(c.chartsReadyAt) ? '' : ` · map ${ms(c.mapReadyAt - c.chartsReadyAt)} ms after the charts`);
}

async function show(k: number): Promise<void> {
  scrubButton.disabled = true;
  teardown();
  const choice = CHOICES[k];
  const cold = shows === 0;
  shows++;
  clearSteps();
  info.textContent = `Loading ${choice.label}…`;
  startup.textContent = 'Start-up window: waiting for the charts…';
  const data = await load(choice);
  const cursor = createCursorStore(data.sampleCount, (f) => requestAnimationFrame(() => f()));
  const offs: (() => void)[] = [];
  const blocked = tiles.value === 'blocked';
  const hasMap = routePresence(data) === 'route';

  // Subscribed BEFORE ChartStack and the map: marks the start of the flush.
  offs.push(
    cursor.subscribe(() => {
      if (measuring) flushStart = performance.now();
    }),
  );

  const c: Current = {
    apps: [],
    cursor,
    data,
    offs,
    choice,
    chartsReadyAt: Number.NaN,
    mapReadyAt: Number.NaN,
    startupDone: false,
  };
  current = c;

  const endSubscriber = (): void => {
    // Subscribed AFTER the charts, the readout and the map marker: marks the end of the flush.
    offs.push(
      cursor.subscribe(() => {
        if (measuring && flushCount < MAX_SAMPLES) flushTimes[flushCount++] = performance.now() - flushStart;
      }),
    );
    offs.push(
      cursor.subscribe((i: CursorIndex) => {
        second.textContent = i === null ? 'second subscriber: –' : 'second subscriber: index ' + i;
      }),
    );
    scrubButton.disabled = false;
  };

  const mapProps = benchMapProps({
    data,
    cursor,
    load: false,
    styleUrl: blocked ? BLOCKED_STYLE_URL : undefined,
    onmapready: () => {
      if (current !== c) return;
      c.mapReadyAt = performance.now();
      showTimes(c);
      endSubscriber();
    },
  });
  c.apps.push(mount(RouteMap, { target: mapTarget, props: mapProps.props }));
  c.apps.push(
    mount(ChartStack, {
      target: stackTarget,
      props: {
        data,
        cursor,
        onready: () => {
          if (current !== c) return;
          c.chartsReadyAt = performance.now();
          showTimes(c);
          mapProps.setLoad(true); // the wiring of the design: load = chartsReady
          monitorStartup(c, cold, blocked, hasMap);
          if (!hasMap) endSubscriber(); // no map: scrubbing still measurable
        },
      },
    }),
  );
  info.textContent = describe(data, choice, blocked);
}

/** rAF monitor from charts onready to STARTUP_WINDOW_MS later (MD11, MP1), then BASELINE_MS
 *  of idle frames for the frame-rate baseline, then the step table (F-06 v1.4). */
function monitorStartup(c: Current, cold: boolean, blocked: boolean, hasMap: boolean): void {
  startupCount = 0;
  baselineCount = 0;
  const start = performance.now();
  let last = start;
  startup.textContent = `Start-up window: measuring for ${STARTUP_WINDOW_MS / 1000} s, then ${BASELINE_MS / 1000} s idle baseline… keep still.`;
  const frame = (now: number): void => {
    if (current !== c) return;
    const elapsed = now - start;
    if (last - start < STARTUP_WINDOW_MS) {
      // An interval that began inside the window counts to it (as before v1.4).
      if (startupCount < MAX_SAMPLES) {
        startupIntervals[startupCount] = now - last;
        startupEnds[startupCount] = now;
        startupCount++;
      }
    } else if (baselineCount < MAX_SAMPLES) {
      baselineIntervals[baselineCount++] = now - last;
    }
    last = now;
    if (elapsed < STARTUP_WINDOW_MS + BASELINE_MS) {
      requestAnimationFrame(frame);
      return;
    }
    c.startupDone = true;
    startup.textContent = startupReport(c, cold, blocked, hasMap);
  };
  requestAnimationFrame(frame);
}

interface Step {
  readonly name: string;
  readonly start: number;
  readonly duration: number;
}

/** Removes the previous show's wx-map entries, so the table shows this show only. */
function clearSteps(): void {
  for (const e of performance.getEntriesByType('mark')) if (e.name.startsWith(STEP_PREFIX)) performance.clearMarks(e.name);
  for (const e of performance.getEntriesByType('measure')) if (e.name.startsWith(STEP_PREFIX)) performance.clearMeasures(e.name);
}

/** The MapLibre JS chunk's Resource Timing entry fetched for this show, or null (warm: already loaded). */
function maplibreChunk(after: number): PerformanceResourceTiming | null {
  let found: PerformanceResourceTiming | null = null;
  for (const e of performance.getEntriesByType('resource') as PerformanceResourceTiming[]) {
    const path = new URL(e.name).pathname;
    if (!/maplibre-gl[^/]*\.m?js$/.test(path) || path.includes('worker')) continue;
    if (e.responseEnd < after) continue;
    if (found === null || e.responseEnd > found.responseEnd) found = e;
  }
  return found;
}

/** Every wx-map mark and measure of this show, plus "import evaluation" when measurable, by start. */
function steps(c: Current): { list: Step[]; chunk: PerformanceResourceTiming | null } {
  const list: Step[] = [];
  for (const e of [...performance.getEntriesByType('mark'), ...performance.getEntriesByType('measure')]) {
    if (e.name.startsWith(STEP_PREFIX) && e.startTime >= c.chartsReadyAt) list.push({ name: e.name, start: e.startTime, duration: e.duration });
  }
  const imported = list.find((s) => s.name === 'wx-map:imported');
  const chunk = maplibreChunk(c.chartsReadyAt);
  if (imported && chunk && chunk.responseEnd <= imported.start) {
    list.push({ name: 'import evaluation', start: chunk.responseEnd, duration: imported.start - chunk.responseEnd });
  }
  list.sort((a, b) => a.start - b.start);
  return { list, chunk };
}

/** Steps that overlap the frame interval ending at `end` (measures by span, marks inside it). */
function overlapping(list: readonly Step[], end: number, length: number): string {
  const from = end - length;
  const names = list.filter((s) => (s.duration > 0 ? s.start < end && s.start + s.duration > from : s.start > from && s.start <= end)).map((s) => s.name);
  return names.length > 0 ? names.join(', ') : 'none of ours (MapLibre self-scheduled work or other)';
}

function startupReport(c: Current, cold: boolean, blocked: boolean, hasMap: boolean): string {
  let longest = 0;
  let longestAt = -1;
  let long = 0;
  for (let k = 0; k < startupCount; k++) {
    const v = startupIntervals[k];
    if (v > longest) {
      longest = v;
      longestAt = k;
    }
    if (v > LONG_FRAME_MS) long++;
  }
  const baseline = quantile(baselineIntervals.slice(0, baselineCount).sort(), 0.5);
  const capped = baseline >= CAPPED_MEDIAN_MS;
  const { list, chunk } = steps(c);
  const rel = (t: number) => (t - c.chartsReadyAt).toFixed(1);
  const lines = [
    `Start-up window (${STARTUP_WINDOW_MS / 1000} s from charts onready), ${cold ? 'COLD (first show after a page load)' : 'warm (reload the page for a cold row)'}`,
    `File: ${c.choice.label} · Tiles: ${blocked ? 'blocked' : 'normal'}${hasMap ? '' : ' · no map for this file'}`,
    `Baseline median interval ms (${BASELINE_MS / 1000} s idle after the window, ${baselineCount} intervals): ${baseline.toFixed(2)}`,
    ...(capped ? [`30 Hz cap: the run does not count (baseline median ≥ ${CAPPED_MEDIAN_MS} ms; Low Power Mode on?)`] : []),
    `Time to onmapready ms (from charts onready): ${ms(c.mapReadyAt - c.chartsReadyAt)}`,
    `Longest frame interval ms: ${longest.toFixed(1)}`,
    `Intervals > ${LONG_FRAME_MS} ms: ${long} (of ${startupCount})`,
    `Step at the longest interval: ${longestAt < 0 ? '–' : overlapping(list, startupEnds[longestAt], longest)}`,
    `Trigger (phone, Low Power Mode off, no 30 Hz cap): longest interval > 300 ms (OQ-21 accepted), or MP1 fails`,
    `devicePixelRatio: ${window.devicePixelRatio} · userAgent: ${navigator.userAgent}`,
    '',
    `Steps (start ms after charts onready / duration ms):`,
  ];
  if (list.length === 0) lines.push('  (none: no map for this file, or the map did not start)');
  for (const s of list) lines.push(`  ${s.name}: ${rel(s.start)} / ${s.duration.toFixed(1)}`);
  if (!chunk) lines.push('  import evaluation: – (MapLibre chunk not fetched in this show: warm run, or not found in Resource Timing)');
  lines.push('', `Frame intervals > ${LONG_FRAME_MS} ms (end ms after charts onready, length ms: overlapping steps):`);
  let any = false;
  for (let k = 0; k < startupCount; k++) {
    const v = startupIntervals[k];
    if (v <= LONG_FRAME_MS) continue;
    any = true;
    lines.push(`  ${rel(startupEnds[k])}, ${v.toFixed(1)}: ${overlapping(list, startupEnds[k], v)}`);
  }
  if (!any) lines.push('  none');
  return lines.join('\n');
}

function describe(data: WorkoutData, choice: Choice, blocked: boolean): string {
  const charts = presentCharts(data).map((s) => s.label);
  return [
    `File: ${choice.label}`,
    `N: ${data.sampleCount} · laps: ${data.meta.laps.length} · sessions: ${data.meta.sessions.length}`,
    `route presence: ${routePresence(data)}`,
    `tiles: ${blocked ? `blocked (${BLOCKED_STYLE_URL})` : 'normal (liberty)'}`,
    `charts (${charts.length}): ${charts.join(', ') || '-'}`,
  ].join('\n');
}

function quantile(sorted: Float64Array, q: number): number {
  if (sorted.length === 0) return Number.NaN;
  const at = Math.min(sorted.length - 1, Math.max(0, Math.ceil(q * sorted.length) - 1));
  return sorted[at];
}

function middleOver(): HTMLElement | null {
  const overs = stackTarget.querySelectorAll<HTMLElement>('.u-over');
  return overs.length > 0 ? overs[Math.floor((overs.length - 1) / 2)] : null;
}

/** F-05's procedure, unchanged, with the map above the charts. */
function autoScrub(): void {
  const c = current;
  if (!c || measuring) return;
  const over = middleOver();
  if (!over) {
    out.textContent = 'No chart to scrub.';
    return;
  }
  scrubButton.disabled = true;
  out.textContent = 'Scrubbing… keep the mouse still and DevTools closed. Keep the map in view.';
  frameCount = 0;
  flushCount = 0;
  measuring = true;

  const rect = over.getBoundingClientRect();
  const y = rect.top + rect.height / 2;
  const start = performance.now();
  let lastFrame = 0;

  const frame = (now: number): void => {
    if (!measuring) return;
    if (lastFrame !== 0 && frameCount < MAX_SAMPLES) frameIntervals[frameCount++] = now - lastFrame;
    lastFrame = now;
    requestAnimationFrame(frame);
  };
  requestAnimationFrame(frame);

  const timer = setInterval(() => {
    const t = performance.now() - start;
    if (t >= SCRUB_MS) {
      clearInterval(timer);
      measuring = false;
      report(c, rect.width);
      scrubButton.disabled = false;
      return;
    }
    // Triangle wave: left → right → left at constant speed over the 10 s.
    const phase = t / SCRUB_MS;
    const p = phase < 0.5 ? phase * 2 : 2 - phase * 2;
    const x = rect.left + p * (rect.width - 1);
    over.dispatchEvent(
      new PointerEvent('pointermove', { pointerType: 'mouse', isPrimary: true, pointerId: 1, bubbles: true, clientX: x, clientY: y }),
    );
  }, EVENT_EVERY_MS);
}

function mapVisible(): string {
  const el = mapTarget.querySelector<HTMLElement>('.route-map');
  if (!el) return 'no (no map for this file)';
  const r = el.getBoundingClientRect();
  const visible = r.bottom > 0 && r.top < window.innerHeight;
  return visible ? 'yes' : 'NO (scrolled out of view)';
}

function report(c: Current, plotWidthCss: number): void {
  const intervals = frameIntervals.slice(0, frameCount).sort();
  const flushes = flushTimes.slice(0, flushCount).sort();
  let ok = 0;
  for (const v of intervals) if (v <= FRAME_OK_MS) ok++;
  const share = intervals.length > 0 ? ok / intervals.length : 0;
  const flushP95 = quantile(flushes, 0.95);
  const flushMax = flushes.length > 0 ? flushes[flushes.length - 1] : Number.NaN;
  const pass = share >= 0.99 && flushP95 <= 2 && flushMax <= 8;
  const f = (v: number) => v.toFixed(2);
  out.textContent = [
    `Frames: ${intervals.length}`,
    `Interval ms median / p95 / max: ${f(quantile(intervals, 0.5))} / ${f(quantile(intervals, 0.95))} / ${f(intervals[intervals.length - 1] ?? Number.NaN)}`,
    `Share ≤ ${FRAME_OK_MS} ms: ${(share * 100).toFixed(2)} %`,
    `Flushes: ${flushes.length} (charts, readout and map marker)`,
    `Flush ms median / p95 / max: ${f(quantile(flushes, 0.5))} / ${f(flushP95)} / ${f(flushMax)}`,
    `PASS (desktop; ≥ 99 % ≤ ${FRAME_OK_MS} ms, flush p95 ≤ 2 ms, max ≤ 8 ms): ${pass ? 'yes' : 'NO'}`,
    `Map visible: ${mapVisible()}`,
    '',
    `File: ${c.choice.label}`,
    `N: ${c.data.sampleCount}`,
    `Charts: ${presentCharts(c.data).length}`,
    `devicePixelRatio: ${window.devicePixelRatio}`,
    `Plot width (CSS px): ${plotWidthCss.toFixed(0)}`,
    `Window width (CSS px): ${window.innerWidth}`,
    `userAgent: ${navigator.userAgent}`,
  ].join('\n');
}

function fail(e: unknown): void {
  info.textContent = `error: ${e instanceof Error ? e.message : String(e)}`;
}

select.addEventListener('change', () => {
  show(Number(select.value)).catch(fail);
});
tiles.addEventListener('change', () => {
  show(Number(select.value)).catch(fail);
});
scrubButton.addEventListener('click', autoScrub);

show(0).catch(fail);
