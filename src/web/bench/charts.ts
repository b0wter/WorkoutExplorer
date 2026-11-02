// Dev-only page for F-05's browser checks and the 60 fps measurement (AC 7).
// architect/F-05-stacked-charts-with-shared-cursor.md, "Dev page", "Performance".
// Never part of the production build; nothing in src/ imports it.
//
// Synthetic golden data files only (ADR-004 item 7). The heart-rate dropout
// variant is ride-4h with heart_rate set to NaN at indices 3,600–4,199, built here.

import { mount, unmount } from 'svelte';
import { decodeDataFile, type WorkoutData } from '../src/lib/datafile';
import { createCursorStore, type CursorIndex, type CursorStore } from '../src/lib/cursor';
import { presentCharts } from '../src/lib/charts';
import ChartStack from '../src/components/charts/ChartStack.svelte';
import ride12h from '../../server/testdata/datafile/synthetic/ride-12h.bin?url';
import ride4h from '../../server/testdata/datafile/synthetic/ride-4h.bin?url';
import indoorNoGps from '../../server/testdata/datafile/synthetic/indoor-no-gps.bin?url';
import noDistanceWithGps from '../../server/testdata/datafile/synthetic/no-distance-with-gps.bin?url';
import brickRunBike from '../../server/testdata/datafile/synthetic/brick-run-bike.bin?url';
import pauseHours from '../../server/testdata/datafile/synthetic/pause-hours.bin?url';

interface Choice {
  readonly label: string;
  readonly url: string;
  readonly transform?: (d: WorkoutData) => WorkoutData;
}

function heartRateDropout(d: WorkoutData): WorkoutData {
  const hr = d.metrics.heart_rate;
  if (!hr) throw new Error('ride-4h has no heart_rate');
  const copy = Float64Array.from(hr);
  copy.fill(Number.NaN, 3600, 4200);
  return { ...d, metrics: { ...d.metrics, heart_rate: copy } };
}

const CHOICES: readonly Choice[] = [
  { label: 'ride-12h (all six charts; AC 7 file)', url: ride12h },
  { label: 'ride-4h', url: ride4h },
  { label: 'ride-4h with heart-rate dropout (3,600–4,199)', url: ride4h, transform: heartRateDropout },
  { label: 'indoor-no-gps (no altitude)', url: indoorNoGps },
  { label: 'no-distance-with-gps (no speed, no power)', url: noDistanceWithGps },
  { label: 'brick-run-bike (power missing in the run part)', url: brickRunBike },
  { label: 'pause-hours (multi-hour gap)', url: pauseHours },
];

const SCRUB_MS = 10_000;
const EVENT_EVERY_MS = 8;
const MAX_SAMPLES = 8192; // frames or flushes in 10 s, with room for 240 Hz displays
const FRAME_OK_MS = 20;

const select = document.getElementById('file') as HTMLSelectElement;
const scrubButton = document.getElementById('scrub') as HTMLButtonElement;
const second = document.getElementById('second') as HTMLSpanElement;
const info = document.getElementById('info') as HTMLPreElement;
const out = document.getElementById('out') as HTMLPreElement;
const target = document.getElementById('stack') as HTMLDivElement;

CHOICES.forEach((c, k) => select.add(new Option(c.label, String(k))));

// Preallocated measurement buffers (no allocation per frame or flush).
const frameIntervals = new Float64Array(MAX_SAMPLES);
let frameCount = 0;
const flushTimes = new Float64Array(MAX_SAMPLES);
let flushCount = 0;
let flushStart = 0;
let measuring = false;

let current: { app: ReturnType<typeof mount>; cursor: CursorStore; data: WorkoutData; offs: (() => void)[] } | null = null;
let ready = false;

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
  void unmount(current.app);
  current = null;
}

async function show(k: number): Promise<void> {
  scrubButton.disabled = true;
  ready = false;
  teardown();
  const choice = CHOICES[k];
  info.textContent = `Loading ${choice.label}…`;
  const data = await load(choice);
  const cursor = createCursorStore(data.sampleCount, (f) => requestAnimationFrame(() => f()));
  const offs: (() => void)[] = [];

  // Subscribed BEFORE ChartStack: listeners run in subscription order, so this
  // marks the start of the time spent in the charts and the readout.
  offs.push(
    cursor.subscribe(() => {
      if (measuring) flushStart = performance.now();
    }),
  );

  const app = mount(ChartStack, {
    target,
    props: {
      data,
      cursor,
      onready: () => {
        // Every chart has subscribed by now. Subscribed AFTER ChartStack: marks the end.
        offs.push(
          cursor.subscribe(() => {
            if (measuring && flushCount < MAX_SAMPLES) flushTimes[flushCount++] = performance.now() - flushStart;
          }),
        );
        // Stand-in for F-06's map: a late subscriber gets the current index at once.
        offs.push(
          cursor.subscribe((i: CursorIndex) => {
            second.textContent = i === null ? 'second subscriber: –' : 'second subscriber: index ' + i;
          }),
        );
        ready = true;
        scrubButton.disabled = false;
        info.textContent = describe(data);
      },
    },
  });
  current = { app, cursor, data, offs };
}

function describe(data: WorkoutData): string {
  const charts = presentCharts(data).map((s) => s.label);
  return [
    `N: ${data.sampleCount}`,
    `charts (${charts.length}): ${charts.join(', ') || '-'}`,
    `gaps: ${data.gaps.length}`,
    `distance in readout: ${data.metrics.distance ? 'yes' : 'no'}`,
  ].join('\n');
}

function quantile(sorted: Float64Array, q: number): number {
  if (sorted.length === 0) return Number.NaN;
  const at = Math.min(sorted.length - 1, Math.max(0, Math.ceil(q * sorted.length) - 1));
  return sorted[at];
}

function middleOver(): HTMLElement | null {
  const overs = target.querySelectorAll<HTMLElement>('.u-over');
  return overs.length > 0 ? overs[Math.floor((overs.length - 1) / 2)] : null;
}

function autoScrub(): void {
  if (!current || !ready || measuring) return;
  const over = middleOver();
  if (!over) {
    out.textContent = 'No chart to scrub.';
    return;
  }
  const data = current.data;
  scrubButton.disabled = true;
  out.textContent = 'Scrubbing… keep the mouse still and DevTools closed.';
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
      report(data, rect.width);
      scrubButton.disabled = false;
      return;
    }
    // Triangle wave: left → right → left at constant speed over the 10 s.
    const phase = t / SCRUB_MS;
    const p = phase < 0.5 ? phase * 2 : 2 - phase * 2;
    const x = rect.left + p * (rect.width - 1);
    over.dispatchEvent(
      new PointerEvent('pointermove', {
        pointerType: 'mouse',
        isPrimary: true,
        pointerId: 1,
        bubbles: true,
        clientX: x,
        clientY: y,
      }),
    );
  }, EVENT_EVERY_MS);
}

function report(data: WorkoutData, plotWidthCss: number): void {
  const intervals = frameIntervals.slice(0, frameCount).sort();
  const flushes = flushTimes.slice(0, flushCount).sort();
  let ok = 0;
  for (const v of intervals) if (v <= FRAME_OK_MS) ok++;
  const share = intervals.length > 0 ? ok / intervals.length : 0;
  const flushP95 = quantile(flushes, 0.95);
  const flushMax = flushes.length > 0 ? flushes[flushes.length - 1] : Number.NaN;
  const pass = share >= 0.99 && flushP95 <= 2 && flushMax <= 8;
  const ms = (v: number) => v.toFixed(2);
  out.textContent = [
    `Frames: ${intervals.length}`,
    `Interval ms median / p95 / max: ${ms(quantile(intervals, 0.5))} / ${ms(quantile(intervals, 0.95))} / ${ms(intervals[intervals.length - 1] ?? Number.NaN)}`,
    `Share ≤ ${FRAME_OK_MS} ms: ${(share * 100).toFixed(2)} %`,
    `Flushes: ${flushes.length}`,
    `Flush ms median / p95 / max: ${ms(quantile(flushes, 0.5))} / ${ms(flushP95)} / ${ms(flushMax)}`,
    `PASS (≥ 99 % ≤ ${FRAME_OK_MS} ms, flush p95 ≤ 2 ms, max ≤ 8 ms): ${pass ? 'yes' : 'NO'}`,
    '',
    `N: ${data.sampleCount}`,
    `Charts: ${presentCharts(data).length}`,
    `devicePixelRatio: ${window.devicePixelRatio}`,
    `Plot width (CSS px): ${plotWidthCss.toFixed(0)}`,
    `Window width (CSS px): ${window.innerWidth}`,
    `userAgent: ${navigator.userAgent}`,
  ].join('\n');
}

select.addEventListener('change', () => {
  show(Number(select.value)).catch(fail);
});
scrubButton.addEventListener('click', autoScrub);

function fail(e: unknown): void {
  info.textContent = `error: ${e instanceof Error ? e.message : String(e)}`;
}

show(0).catch(fail);
