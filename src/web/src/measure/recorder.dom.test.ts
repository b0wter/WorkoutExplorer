// @vitest-environment jsdom
// The scrub recorder's wiring (F-07 design F-4, F-6; ADR-016; ADR-012): listener order,
// readiness, a full recording with stubbed frames and touches, no DOM mutation during the
// window, and listener cleanup. Fake timers drive the countdown; requestAnimationFrame is a
// manual queue. Timing values are not asserted (jsdom is not a browser). Synthetic data only.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { loadGolden } from '../../test-support/golden.mjs';
import { createCursorStore, type CursorStore } from '../lib/cursor';
import { decodeDataFile, type WorkoutData } from '../lib/datafile';
import { createRecorder } from './recorder';

function ride(): WorkoutData {
  const g = loadGolden().find((x) => x.name === 'ride-4h');
  if (!g) throw new Error('no golden ride-4h');
  const r = decodeDataFile(g.bin);
  if (!r.ok) throw new Error(r.error.message);
  return r.data;
}
const data = ride();

// ---- environment -------------------------------------------------------------------------

let frames: FrameRequestCallback[] = [];
type Call = { type: string; fn: unknown; capture: boolean };
let added: Call[] = [];
let removed: Call[] = [];

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  frames = [];
  vi.stubGlobal('requestAnimationFrame', (f: FrameRequestCallback): number => {
    frames.push(f);
    return frames.length;
  });
  added = [];
  removed = [];
  const capture = (o?: boolean | EventListenerOptions): boolean => (typeof o === 'boolean' ? o : !!o?.capture);
  const add = window.addEventListener.bind(window);
  const remove = window.removeEventListener.bind(window);
  vi.spyOn(window, 'addEventListener').mockImplementation((type: string, fn: EventListenerOrEventListenerObject, o?: boolean | AddEventListenerOptions) => {
    added.push({ type, fn, capture: capture(o) });
    add(type, fn, o);
  });
  vi.spyOn(window, 'removeEventListener').mockImplementation((type: string, fn: EventListenerOrEventListenerObject, o?: boolean | EventListenerOptions) => {
    removed.push({ type, fn, capture: capture(o) });
    remove(type, fn, o);
  });
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  document.body.innerHTML = '';
});

interface Harness {
  cursor: CursorStore;
  /** Runs the pending cursor flush, if any. */
  flush: () => void;
  /** Names of the listeners in subscription order, as the store calls them. */
  order: string[];
  subscriptions: () => number;
}

/** A cursor store with a manual flush, counting subscriptions; 'charts' and 'marker' stand-ins
 *  can be subscribed between the recorder's listeners. */
function harness(): Harness {
  const queue: (() => void)[] = [];
  const store = createCursorStore(data.sampleCount, (f) => queue.push(f));
  let count = 0;
  const subscribe = store.subscribe.bind(store);
  const cursor: CursorStore = {
    sampleCount: store.sampleCount,
    get: store.get,
    set: store.set,
    subscribe: (l) => {
      count++;
      return subscribe(l);
    },
  };
  return {
    cursor,
    flush: () => {
      for (const f of queue.splice(0)) f();
    },
    order: [],
    subscriptions: () => count,
  };
}

const button = (): HTMLButtonElement => {
  const b = document.querySelector('button');
  if (!b) throw new Error('no button');
  return b;
};
const output = (): string => document.querySelector('pre.wx-measure')?.textContent ?? '';

/** A touch event as a plain Event with `touches` (jsdom has no Touch; ADR-012 rev. 2). */
function touch(target: EventTarget, type: string, fingers: number): void {
  const ev = new Event(type, { bubbles: true });
  Object.defineProperty(ev, 'touches', { value: { length: fingers } });
  target.dispatchEvent(ev);
}

/** Runs the queued animation frames once, at time t. */
function frameAt(t: number): void {
  for (const f of frames.splice(0)) f(t);
}

// ---- tests -------------------------------------------------------------------------------

describe('createRecorder: listeners and readiness', () => {
  it('one listener at once, one more at chartsReady, one more at mapReady; Record enabled only then', () => {
    const h = harness();
    const r = createRecorder({ cursor: h.cursor, data, name: 'ride-4h', hasRoute: true });
    expect(h.subscriptions()).toBe(1);
    expect(button().textContent).toBe('Record');
    expect(button().disabled).toBe(true);
    expect(output()).toBe('Measurement: waiting for the charts…');
    expect(button().style.cssText).toContain('position: fixed');
    expect(document.querySelector('pre.wx-measure')).not.toBeNull();

    r.chartsReady();
    expect(h.subscriptions()).toBe(2);
    expect(button().disabled).toBe(true);
    expect(output()).toBe('Measurement: waiting for the charts…');

    r.mapReady();
    expect(h.subscriptions()).toBe(3);
    expect(button().disabled).toBe(false);
    const lines = output().split('\n');
    expect(lines[0]).toBe(`Load · ride-4h · N ${data.sampleCount} · streams 10 · laps ${data.meta.laps.length}`);
    expect(lines[lines.length - 1]).toMatch(/^dpr \S+ · window \d+ CSS px · /);
    expect(lines.some((l) => l.startsWith('Since navigation start: data ready '))).toBe(true);
    expect(lines.some((l) => l.endsWith('map ready –') || /map ready [\d.]+$/.test(l))).toBe(true);
  });

  it('no route: chartsReady subscribes two listeners and enables Record', () => {
    const h = harness();
    const r = createRecorder({ cursor: h.cursor, data, name: 'ride-4h', hasRoute: false });
    expect(h.subscriptions()).toBe(1);
    r.chartsReady();
    expect(h.subscriptions()).toBe(3);
    expect(button().disabled).toBe(false);
    expect(output().startsWith('Load · ride-4h')).toBe(true);
  });

  it('a click while not ready does nothing', () => {
    const h = harness();
    createRecorder({ cursor: h.cursor, data, name: 'ride-4h', hasRoute: true });
    button().click();
    vi.advanceTimersByTime(5000);
    expect(frames).toHaveLength(0);
    expect(button().textContent).toBe('Record');
  });
});

describe('a full recording', () => {
  /** Stubbed performance.now (QA-07-r1-2): the charts stand-in takes CHARTS_MS and the marker
   *  stand-in MAP_MS of it, so the recorded split is known exactly. */
  let clock = 0;
  const CHARTS_MS = 3;
  const MAP_MS = 1;

  /** Mounts a recorder with stand-ins for the charts and the marker, ready to record. */
  function ready(hasRoute = true) {
    clock = 0;
    vi.spyOn(performance, 'now').mockImplementation(() => clock);
    const h = harness();
    const r = createRecorder({ cursor: h.cursor, data, name: 'ride-4h', hasRoute });
    h.cursor.subscribe(() => {
      h.order.push('charts');
      clock += CHARTS_MS;
    });
    r.chartsReady();
    if (hasRoute) {
      h.cursor.subscribe(() => {
        h.order.push('marker');
        clock += MAP_MS;
      });
      r.mapReady();
    }
    const map = document.createElement('div');
    map.className = 'route-map';
    const chart = document.createElement('div');
    chart.className = 'chart';
    document.body.append(map, chart);
    return { h, map, chart };
  }

  it('countdown 3-2-1, window ●, touch scrub with a flush per frame, block appended; no DOM mutation in the window; listeners removed', () => {
    const { h, chart } = ready();
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(800);
    vi.spyOn(document.querySelector('.route-map') as HTMLElement, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 375, 320));

    button().click();
    expect(button().disabled).toBe(true);
    expect(button().textContent).toBe('3');
    // The finger goes down during the countdown, on a chart.
    touch(chart, 'touchstart', 1);
    vi.advanceTimersByTime(1000);
    expect(button().textContent).toBe('2');
    vi.advanceTimersByTime(1000);
    expect(button().textContent).toBe('1');
    expect(frames).toHaveLength(0);
    vi.advanceTimersByTime(1000);
    expect(button().textContent).toBe('●');
    expect(frames).toHaveLength(1);

    const observer = new MutationObserver(() => {});
    observer.observe(document.body, { subtree: true, childList: true, attributes: true, characterData: true });

    const T0 = 1000;
    frameAt(T0); // first frame: the window starts
    observer.takeRecords();
    let k = 1;
    for (; T0 + k * 16 < T0 + 10_000; k++) {
      h.cursor.set(k % data.sampleCount);
      h.flush();
      frameAt(T0 + k * 16);
    }
    expect(observer.takeRecords()).toEqual([]); // nothing written between the first and the last frame
    h.cursor.set(k % data.sampleCount);
    h.flush();
    frameAt(T0 + k * 16); // the last frame ends the window
    observer.disconnect();
    expect(frames).toHaveLength(0);

    // Listener order in a flush: A, charts, M, marker, Z (only the stand-ins are visible).
    expect(h.order.slice(-2)).toEqual(['charts', 'marker']);

    const text = output();
    const block = text.slice(text.indexOf('Scrub record'));
    const lines = block.split('\n');
    expect(lines[0]).toBe(`Scrub record · ride-4h · N ${data.sampleCount} · input touch · started on chart`);
    expect(lines[1]).toMatch(/^Frames 625 · interval ms median \/ p95 \/ max 16\.00 \/ 16\.00 \/ 16\.00 · ≤ 20 ms 100\.00 %$/);
    // The charts/map split (ADR-016 decision 1): A → charts → M → marker → Z.
    expect(lines[2]).toBe('Flush ms (625) total p95 / max 4.00 / 4.00 · charts p95 / max 3.00 / 3.00 · map p95 / max 1.00 / 1.00');
    // Each frame follows a flush here, so every frame saw a change.
    expect(lines[3]).toBe('Cursor changed in 100.00 % of frames · frames without a finger 0 · map in view at start yes');
    expect(lines[4]).toBe('Counts: yes');
    expect(lines[5]).toMatch(/^PASS: (yes|NO) \(≥ 99 % ≤ 20 ms, flush p95 ≤ 2 ms, max ≤ 8 ms\)$/);
    expect(lines[6]).toMatch(/^dpr \S+ · window \d+ CSS px · /);
    expect(text.startsWith('Load · ride-4h')).toBe(true);

    expect(button().textContent).toBe('Record');
    expect(button().disabled).toBe(false);

    // Every window listener added during the run is removed with the same capture flag.
    expect(added.length).toBe(3);
    for (const a of added) {
      expect(a.capture).toBe(true);
      expect(removed.some((r) => r.type === a.type && r.fn === a.fn && r.capture === a.capture)).toBe(true);
    }
    // After the run, touches no longer reach the recorder: a second run starts clean.
    touch(chart, 'touchend', 0);
  });

  it('a lifted finger and a route start are recorded; the map out of view is reported', () => {
    const { h, map } = ready();
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(800);
    vi.spyOn(map, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, -400, 375, 320)); // scrolled away

    button().click();
    touch(map, 'touchstart', 1);
    vi.advanceTimersByTime(3000);
    frameAt(0);
    for (let k = 1; k <= 700 && frames.length > 0; k++) {
      if (k === 100) touch(map, 'touchend', 0);
      if (k === 110) touch(map, 'touchstart', 1);
      h.cursor.set(k % data.sampleCount);
      h.flush();
      frameAt(k * 16);
    }
    const block = output().slice(output().indexOf('Scrub record')).split('\n');
    expect(block[0]).toContain('input touch · started on route');
    expect(block[3]).toMatch(/frames without a finger 10 · map in view at start no$/);
    expect(block[4]).toBe('Counts: NO: no finger down in 10 frames; map not in view');
  });

  it('mouse run without a map: input mouse, started on –, no map', () => {
    const { h } = ready(false);
    button().click();
    vi.advanceTimersByTime(3000);
    frameAt(0);
    for (let k = 1; k <= 700 && frames.length > 0; k++) {
      h.cursor.set(k % data.sampleCount);
      h.flush();
      frameAt(k * 16);
    }
    const block = output().slice(output().indexOf('Scrub record')).split('\n');
    expect(block[0]).toBe(`Scrub record · ride-4h · N ${data.sampleCount} · input mouse · started on –`);
    // No marker: the whole flush is the charts part.
    expect(block[2]).toBe('Flush ms (625) total p95 / max 3.00 / 3.00 · charts p95 / max 3.00 / 3.00 · map p95 / max 0.00 / 0.00');
    expect(block[3]).toMatch(/frames without a finger – · map in view at start no map$/);
    expect(block[4]).toBe('Counts: yes');
  });

  it('flushes outside a recording are not recorded', () => {
    const { h, map } = ready();
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(800);
    vi.spyOn(map, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 375, 320));
    for (let k = 1; k < 50; k++) {
      h.cursor.set(k);
      h.flush();
    }
    button().click();
    vi.advanceTimersByTime(3000);
    frameAt(0);
    for (let k = 1; k <= 700 && frames.length > 0; k++) frameAt(k * 16); // cursor still
    const block = output().slice(output().indexOf('Scrub record')).split('\n');
    expect(block[2]).toBe('Flush ms (0) total p95 / max – / – · charts p95 / max – / – · map p95 / max – / –');
    expect(block[4]).toBe('Counts: NO: cursor still in more than 25 % of frames');
  });

  /** One recording with the map in view: optional touch during the countdown, `at(k)` before
   *  frame k, a flush before frame k when `flushIf(k)`. Returns the result block's lines. */
  function record(o: { countdownTouch?: Element; at?: (k: number) => void; flushIf?: (k: number) => boolean } = {}): string[] {
    const { h, map } = ready();
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(800);
    vi.spyOn(map, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 375, 320));
    button().click();
    if (o.countdownTouch) touch(o.countdownTouch, 'touchstart', 1);
    vi.advanceTimersByTime(3000);
    frameAt(0);
    for (let k = 1; k <= 700 && frames.length > 0; k++) {
      o.at?.(k);
      if (o.flushIf?.(k) ?? true) {
        h.cursor.set(k % data.sampleCount);
        h.flush();
      }
      frameAt(k * 16);
    }
    return output().slice(output().indexOf('Scrub record')).split('\n');
  }

  it('cursor still in half the frames: the changed share is per frame and the run does not count (QA-07-r1-2)', () => {
    const block = record({ countdownTouch: document.body, flushIf: (k) => k % 2 === 0 });
    // 312 flushes before 625 frames; a frame counts as changed only after a flush.
    expect(block[2]).toMatch(/^Flush ms \(312\) /);
    expect(block[3]).toMatch(/^Cursor changed in 49\.92 % of frames · frames without a finger 0 · /);
    expect(block[4]).toBe('Counts: NO: cursor still in more than 25 % of frames');
  });
});

describe('touch validity (design v1.2, QA-07-r1-1)', () => {
  // Same harness as above, without stand-in timing.
  function readyRun(o: { countdownTouch?: boolean; touchAt?: number; mouse?: boolean }): string[] {
    const h = harness();
    const r = createRecorder({ cursor: h.cursor, data, name: 'ride-4h', hasRoute: true });
    r.chartsReady();
    r.mapReady();
    const map = document.createElement('div');
    map.className = 'route-map';
    const chart = document.createElement('div');
    document.body.append(map, chart);
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(800);
    vi.spyOn(map, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 375, 320));
    button().click();
    if (o.countdownTouch) touch(chart, 'touchstart', 1);
    vi.advanceTimersByTime(3000);
    frameAt(0);
    for (let k = 1; k <= 700 && frames.length > 0; k++) {
      if (k === o.touchAt) touch(chart, 'touchstart', 1);
      h.cursor.set(k % data.sampleCount);
      h.flush();
      frameAt(k * 16);
    }
    return output().slice(output().indexOf('Scrub record')).split('\n');
  }

  it('late finger: touchstart about 2.3 s into the window → the frames before it count, Counts: NO', () => {
    const block = readyRun({ touchAt: 144 }); // 144 × 16 ms ≈ 2.3 s
    expect(block[0]).toContain('input touch · started on chart');
    expect(block[1]).toMatch(/≤ 20 ms 100\.00 %$/);
    expect(block[3]).toMatch(/frames without a finger 143 · map in view at start yes$/);
    expect(block[4]).toBe('Counts: NO: no finger down in 143 frames');
  });

  it('finger down during the countdown and held to the end → 0, and the reason absent', () => {
    const block = readyRun({ countdownTouch: true });
    expect(block[3]).toMatch(/frames without a finger 0 · /);
    expect(block[4]).toBe('Counts: yes');
  });

  it('mouse run (no touch events) → –, and the reason absent', () => {
    const block = readyRun({ mouse: true });
    expect(block[0]).toContain('input mouse · started on –');
    expect(block[3]).toMatch(/frames without a finger – · /);
    expect(block[4]).toBe('Counts: yes');
  });

  it('two runs in a row: a finger still down at the end of run 1 does not carry over into run 2 (QA-07-r2-1)', () => {
    const h = harness();
    const r = createRecorder({ cursor: h.cursor, data, name: 'ride-4h', hasRoute: true });
    r.chartsReady();
    r.mapReady();
    const map = document.createElement('div');
    map.className = 'route-map';
    const chart = document.createElement('div');
    document.body.append(map, chart);
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(800);
    vi.spyOn(map, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 375, 320));
    const scrub = (touchAt: number): string[] => {
      button().click();
      if (touchAt === 0) touch(chart, 'touchstart', 1); // during the countdown
      vi.advanceTimersByTime(3000);
      frameAt(0);
      for (let k = 1; k <= 700 && frames.length > 0; k++) {
        if (k === touchAt) touch(chart, 'touchstart', 1);
        h.cursor.set(k % data.sampleCount);
        h.flush();
        frameAt(k * 16);
      }
      const text = output();
      return text.slice(text.lastIndexOf('Scrub record')).split('\n');
    };

    // Run 1: finger down from the countdown, still down when the window ends (no touchend).
    const first = scrub(0);
    expect(first[3]).toMatch(/frames without a finger 0 · /);
    expect(first[4]).toBe('Counts: yes');

    // Run 2: the finger lands about 2.3 s into the window.
    const second = scrub(144);
    expect(second[3]).toMatch(/frames without a finger 143 · /);
    expect(second[4]).toBe('Counts: NO: no finger down in 143 frames');
  });
});
