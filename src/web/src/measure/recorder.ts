// The in-page scrub record (F-07 design D-6, F-4; ADR-016 decision 1). LAZY: reached only
// through Prototype.svelte's import('../measure/recorder') when the page URL has ?measure.
// Plain DOM, no Svelte.
//
// Three cursor listeners split every flush (listeners run in subscription order, ADR-008):
//   A  subscribed in createRecorder, before ChartStack mounts;
//   M  subscribed in chartsReady, after the charts and the readout;
//   Z  subscribed in mapReady, after the map marker (or in chartsReady when there is no map).
// charts = M − A, map = Z − M, total = Z − A.
//
// During the 10 s window nothing here writes to the DOM or reads layout, and no listener or
// frame callback allocates (F-05 D-10): every buffer is preallocated.

import type { CursorStore } from '../lib/cursor';
import type { WorkoutData } from '../lib/datafile';
import { COUNTDOWN_S, MAX_SAMPLES, RECORD_MS, fixed, formatScrub, scrubStats } from './stats';

export interface RecorderOptions {
  readonly cursor: CursorStore;
  readonly data: WorkoutData;
  readonly name: string;
  readonly hasRoute: boolean;
}

export interface Recorder {
  /** Charts are ready: subscribe listener M; if !hasRoute also Z, then enable Record and print the Load block. */
  chartsReady(): void;
  /** Map is ready (marker subscribed): subscribe Z, enable Record, print the Load block. */
  mapReady(): void;
}

const RECORD = 'Record';

/** Last line of the Load and result blocks. */
function environmentLine(): string {
  return `dpr ${window.devicePixelRatio} · window ${window.innerWidth} CSS px · ${navigator.userAgent}`;
}

/** startTime of a User Timing mark (ms since navigation start), or NaN. */
function markTime(name: string): number {
  const entries = performance.getEntriesByName(name, 'mark');
  return entries.length > 0 ? entries[entries.length - 1].startTime : Number.NaN;
}

/** The Resource Timing entry of the data file: URL path ending in /data/<name>.bin. */
function dataFileEntry(name: string): PerformanceResourceTiming | null {
  const suffix = '/data/' + name + '.bin';
  const entries = performance.getEntriesByType('resource') as PerformanceResourceTiming[];
  for (let k = entries.length - 1; k >= 0; k--) {
    try {
      if (new URL(entries[k].name).pathname.endsWith(suffix)) return entries[k];
    } catch {
      // not a URL: not ours
    }
  }
  return null;
}

function loadBlock(data: WorkoutData, name: string): string {
  const streams = 1 + (data.position ? 2 : 0) + Object.keys(data.metrics).length;
  const dataReady = markTime('wx-page:data-ready');
  const entry = dataFileEntry(name);
  const lines = [`Load · ${name} · N ${data.sampleCount} · streams ${streams} · laps ${data.meta.laps.length}`];
  if (entry) {
    // "Request to response end": from the HTTP request (requestStart) to the last byte.
    lines.push(
      `Data file: transfer ${entry.transferSize} B · encoded ${entry.encodedBodySize} B · decoded ${entry.decodedBodySize} B · request to response end ${fixed(entry.responseEnd - entry.requestStart)} ms   (0 = not reported by the browser)`,
      `Body read + decode: ${fixed(dataReady - entry.responseEnd)} ms`,
    );
  } else {
    lines.push('Data file: no Resource Timing entry', 'Body read + decode: –');
  }
  lines.push(
    `Since navigation start: data ready ${fixed(dataReady)} · charts ready ${fixed(markTime('wx-page:charts-ready'))} · map ready ${fixed(markTime('wx-page:map-ready'))}`,
    environmentLine(),
  );
  return lines.join('\n');
}

export function createRecorder(options: RecorderOptions): Recorder {
  const { cursor, data, name, hasRoute } = options;

  // ---- buffers and state (preallocated; nothing below allocates during the window) -----
  const intervals = new Float64Array(MAX_SAMPLES);
  const total = new Float64Array(MAX_SAMPLES);
  const charts = new Float64Array(MAX_SAMPLES);
  const map = new Float64Array(MAX_SAMPLES);
  let recording = false;
  let flushed = false;
  let tA = 0;
  let tM = 0;
  let flushes = 0;
  let frames = 0;
  let changedFrames = 0;
  let noFingerFrames = 0;
  let firstT = -1;
  let lastT = 0;
  let fingers = 0;
  let input: 'touch' | 'mouse' = 'mouse';
  let startedOn: 'chart' | 'route' | '–' = '–';
  let mapInView: boolean | null = null;
  let ready = false;

  // ---- DOM --------------------------------------------------------------------------------
  const button = document.createElement('button');
  button.type = 'button';
  button.textContent = RECORD;
  button.disabled = true;
  button.style.cssText = 'position:fixed;right:8px;bottom:8px;z-index:10;font-size:16px;padding:8px 12px';
  const out = document.createElement('pre');
  out.className = 'wx-measure';
  out.style.cssText = 'white-space:pre-wrap;word-break:break-word;margin:8px;font-size:12px';
  out.textContent = 'Measurement: waiting for the charts…';
  document.body.appendChild(button);
  document.body.appendChild(out);

  // ---- cursor listeners A, M, Z -----------------------------------------------------------
  cursor.subscribe(() => {
    if (!recording) return;
    tA = performance.now();
    flushed = true;
  });
  const listenerM = (): void => {
    if (recording) tM = performance.now();
  };
  const listenerZ = (): void => {
    if (!recording || flushes >= MAX_SAMPLES) return;
    const now = performance.now();
    total[flushes] = now - tA;
    charts[flushes] = tM - tA;
    map[flushes] = now - tM;
    flushes++;
  };

  // ---- touch tracking (passive, capture, on window; only while armed) ---------------------
  const touchOptions: AddEventListenerOptions = { passive: true, capture: true };
  const onTouch = (e: TouchEvent): void => {
    fingers = e.touches.length;
    if (e.type === 'touchstart') {
      input = 'touch';
      const target = e.target;
      startedOn = target instanceof Element && target.closest('.route-map') ? 'route' : 'chart';
    }
  };
  const touchTypes = ['touchstart', 'touchend', 'touchcancel'] as const;

  // ---- recording ----------------------------------------------------------------------------
  const frame = (t: number): void => {
    if (firstT < 0) {
      firstT = t;
      lastT = t;
      requestAnimationFrame(frame);
      return;
    }
    if (frames < MAX_SAMPLES) intervals[frames] = t - lastT;
    lastT = t;
    frames++;
    // Every frame without a finger, also before the first touch (design v1.2, QA-07-r1-1): a
    // late finger leaves idle frames that must disqualify a touch run. scrubStats ignores the
    // count for mouse runs.
    if (fingers === 0) noFingerFrames++;
    if (flushed) {
      changedFrames++;
      flushed = false;
    }
    if (t - firstT >= RECORD_MS) finish();
    else requestAnimationFrame(frame);
  };

  const startWindow = (): void => {
    if (hasRoute) {
      const el = document.querySelector('.route-map');
      const r = el ? el.getBoundingClientRect() : null;
      mapInView = r !== null && r.bottom > 0 && r.top < window.innerHeight;
    } else {
      mapInView = null;
    }
    button.textContent = '●';
    frames = 0;
    flushes = 0;
    changedFrames = 0;
    noFingerFrames = 0;
    firstT = -1;
    flushed = false;
    recording = true;
    requestAnimationFrame(frame);
  };

  const finish = (): void => {
    recording = false;
    for (const type of touchTypes) window.removeEventListener(type, onTouch, touchOptions);
    const stats = scrubStats({ intervals, frames, total, charts, map, flushes, changedFrames, noFingerFrames, touch: input === 'touch', mapInView });
    const block = formatScrub(stats, { name, n: data.sampleCount, startedOn, input });
    out.textContent += '\n\n' + block + '\n' + environmentLine();
    button.textContent = RECORD;
    button.disabled = false;
  };

  const countdown = (left: number): void => {
    if (left === 0) {
      startWindow();
      return;
    }
    button.textContent = String(left);
    setTimeout(() => countdown(left - 1), 1000);
  };

  button.addEventListener('click', () => {
    if (!ready || button.disabled) return;
    button.disabled = true;
    fingers = 0;
    input = 'mouse';
    startedOn = '–';
    for (const type of touchTypes) window.addEventListener(type, onTouch, touchOptions);
    countdown(COUNTDOWN_S);
  });

  const enable = (): void => {
    ready = true;
    button.disabled = false;
    out.textContent = loadBlock(data, name);
  };

  return {
    chartsReady(): void {
      cursor.subscribe(listenerM);
      if (!hasRoute) {
        cursor.subscribe(listenerZ);
        enable();
      }
    },
    mapReady(): void {
      cursor.subscribe(listenerZ);
      enable();
    },
  };
}
