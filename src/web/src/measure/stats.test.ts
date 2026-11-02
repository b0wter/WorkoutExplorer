// Scrub record statistics (F-07 design F-4, F-6; ADR-016 decisions 2 and 3). Node.
import { describe, expect, it } from 'vitest';
import {
  CAPPED_MEDIAN_MS,
  COUNTDOWN_S,
  FLUSH_MAX_MS,
  FLUSH_P95_MS,
  FRAME_OK_MS,
  MAX_SAMPLES,
  MIN_CHANGED_SHARE,
  PASS_SHARE,
  RECORD_MS,
  formatScrub,
  quantile,
  scrubStats,
  type ScrubInput,
} from './stats';

const filled = (n: number, value: number): Float64Array => new Float64Array(n).fill(value);

/** A passing, counting touch run: 100 frames of 16.5 ms, a flush in every frame. */
function input(patch: Partial<ScrubInput> = {}): ScrubInput {
  return {
    intervals: filled(100, 16.5),
    frames: 100,
    total: filled(100, 1),
    charts: filled(100, 0.75),
    map: filled(100, 0.25),
    flushes: 100,
    changedFrames: 100,
    noFingerFrames: 0,
    touch: true,
    mapInView: true,
    ...patch,
  };
}

/** n intervals, of which `slow` are 33 ms and the rest 16 ms. */
function intervals(n: number, slow: number): Pick<ScrubInput, 'intervals' | 'frames' | 'changedFrames'> {
  const a = filled(n, 16);
  for (let k = 0; k < slow; k++) a[k] = 33;
  return { intervals: a, frames: n, changedFrames: n };
}

/** 100 flushes with total = `base`, one of them `p95` at rank 95 and the max `max`. */
function flushes(p95: number, max: number): Pick<ScrubInput, 'total'> {
  const t = filled(100, 0.5);
  for (let k = 0; k < 6; k++) t[k] = p95; // ranks 95..100 when sorted, if p95 ≥ 0.5
  t[0] = max;
  return { total: t };
}

describe('constants (ADR-016)', () => {
  it('as in the design', () => {
    expect([RECORD_MS, COUNTDOWN_S, MAX_SAMPLES]).toEqual([10_000, 3, 8192]);
    expect([FRAME_OK_MS, PASS_SHARE, FLUSH_P95_MS, FLUSH_MAX_MS]).toEqual([20, 0.99, 2, 8]);
    expect([CAPPED_MEDIAN_MS, MIN_CHANGED_SHARE]).toEqual([25, 0.75]);
  });
});

describe('quantile (nearest rank)', () => {
  it('n = 1', () => {
    const a = Float64Array.of(7);
    expect(quantile(a, 0.5)).toBe(7);
    expect(quantile(a, 0.95)).toBe(7);
  });

  it('n = 4', () => {
    const a = Float64Array.of(1, 2, 3, 4);
    expect(quantile(a, 0.5)).toBe(2);
    expect(quantile(a, 0.95)).toBe(4);
  });

  it('n = 100', () => {
    const a = Float64Array.from({ length: 100 }, (_, k) => k + 1);
    expect(quantile(a, 0.5)).toBe(50);
    expect(quantile(a, 0.95)).toBe(95);
    expect(quantile(a, 1)).toBe(100);
  });

  it('empty → NaN', () => {
    expect(quantile(new Float64Array(0), 0.5)).toBeNaN();
  });
});

describe('scrubStats: pass rule (inclusive bounds)', () => {
  it('the base run passes and counts', () => {
    const s = scrubStats(input());
    expect(s.pass).toBe(true);
    expect(s.counts).toBe(true);
    expect(s.reasons).toEqual([]);
  });

  it('share exactly 0.99 passes; 0.9899 fails', () => {
    expect(scrubStats(input(intervals(100, 1))).okShare).toBe(0.99);
    expect(scrubStats(input(intervals(100, 1))).pass).toBe(true);
    const s = scrubStats(input(intervals(10_000, 101)));
    expect(s.okShare).toBe(0.9899);
    expect(s.pass).toBe(false);
  });

  it('an interval of exactly 20 ms is ok', () => {
    expect(scrubStats(input({ intervals: filled(100, 20) })).okShare).toBe(1);
  });

  it('flush p95 2.00 passes; 2.01 fails', () => {
    const ok = scrubStats(input(flushes(2, 2)));
    expect(ok.totalP95).toBe(2);
    expect(ok.pass).toBe(true);
    const bad = scrubStats(input(flushes(2.01, 2.01)));
    expect(bad.totalP95).toBe(2.01);
    expect(bad.pass).toBe(false);
  });

  it('flush max 8.00 passes; 8.01 fails', () => {
    const ok = scrubStats(input(flushes(1, 8)));
    expect([ok.totalP95, ok.totalMax]).toEqual([1, 8]);
    expect(ok.pass).toBe(true);
    const bad = scrubStats(input(flushes(1, 8.01)));
    expect(bad.totalMax).toBe(8.01);
    expect(bad.pass).toBe(false);
  });

  it('no flushes: flush values NaN, no pass (and the cursor-still rule keeps it from counting)', () => {
    const s = scrubStats(input({ flushes: 0, changedFrames: 0 }));
    expect([s.totalP95, s.totalMax, s.chartsP95, s.mapMax]).toEqual([NaN, NaN, NaN, NaN]);
    expect(s.pass).toBe(false);
    expect(s.counts).toBe(false);
  });

  it('uses only the first `frames` and `flushes` entries of preallocated buffers', () => {
    const iv = filled(8192, 99);
    iv.fill(16, 0, 50);
    const total = filled(8192, 99);
    total.fill(1, 0, 40);
    const s = scrubStats(input({ intervals: iv, frames: 50, changedFrames: 50, total, charts: filled(8192, 0), map: filled(8192, 0), flushes: 40 }));
    expect([s.intervalMax, s.totalMax, s.okShare]).toEqual([16, 1, 1]);
    expect(s.pass).toBe(true);
  });

  it('charts and map parts are reported separately', () => {
    const s = scrubStats(input());
    expect([s.chartsP95, s.chartsMax, s.mapP95, s.mapMax]).toEqual([0.75, 0.75, 0.25, 0.25]);
  });
});

describe('scrubStats: validity (ADR-016 decision 3)', () => {
  it('no frames', () => {
    const s = scrubStats(input({ frames: 0, intervals: new Float64Array(0), changedFrames: 0, flushes: 0 }));
    expect(s.reasons).toEqual(['no frames']);
    expect(s.counts).toBe(false);
    expect(s.pass).toBe(false);
  });

  it('median 25 does not count; 24.99 counts', () => {
    expect(scrubStats(input({ intervals: filled(100, 25) })).reasons).toEqual(['30 Hz cap (median ≥ 25 ms; Low Power Mode on?)']);
    expect(scrubStats(input({ intervals: filled(100, 24.99) })).counts).toBe(true);
  });

  it('touch with one no-finger frame does not count; not touch ignores no-finger frames', () => {
    expect(scrubStats(input({ noFingerFrames: 1 })).reasons).toEqual(['no finger down in 1 frames']);
    const mouse = scrubStats(input({ noFingerFrames: 5, touch: false }));
    expect(mouse.counts).toBe(true);
    expect(mouse.noFingerFrames).toBeNull();
  });

  it('changed 0.75 counts; 0.7499 does not', () => {
    expect(scrubStats(input({ changedFrames: 75 })).counts).toBe(true);
    const s = scrubStats(input({ intervals: filled(10_000, 16), frames: 10_000, changedFrames: 7499 }));
    expect(s.reasons).toEqual(['cursor still in more than 25 % of frames']);
  });

  it('map not in view does not count; no map (null) counts', () => {
    expect(scrubStats(input({ mapInView: false })).reasons).toEqual(['map not in view']);
    expect(scrubStats(input({ mapInView: null })).counts).toBe(true);
  });

  it('several reasons, in table order', () => {
    const s = scrubStats(input({ intervals: filled(100, 33), noFingerFrames: 3, changedFrames: 10, mapInView: false }));
    expect(s.reasons).toEqual([
      '30 Hz cap (median ≥ 25 ms; Low Power Mode on?)',
      'no finger down in 3 frames',
      'cursor still in more than 25 % of frames',
      'map not in view',
    ]);
  });
});

describe('formatScrub', () => {
  it('exact lines for a fixed input (several reasons joined with "; ")', () => {
    const iv = Float64Array.of(16, 17, 18, 40);
    const s = scrubStats({
      intervals: iv,
      frames: 4,
      total: Float64Array.of(1, 1.5, 3),
      charts: Float64Array.of(0.5, 1, 2),
      map: Float64Array.of(0.5, 0.5, 1),
      flushes: 3,
      changedFrames: 2,
      noFingerFrames: 1,
      touch: true,
      mapInView: false,
    });
    expect(formatScrub(s, { name: 'ride-12h', n: 43200, startedOn: 'route', input: 'touch' }).split('\n')).toEqual([
      'Scrub record · ride-12h · N 43200 · input touch · started on route',
      'Frames 4 · interval ms median / p95 / max 17.00 / 40.00 / 40.00 · ≤ 20 ms 75.00 %',
      'Flush ms (3) total p95 / max 3.00 / 3.00 · charts p95 / max 2.00 / 2.00 · map p95 / max 1.00 / 1.00',
      'Cursor changed in 50.00 % of frames · frames without a finger 1 · map in view at start no',
      'Counts: NO: no finger down in 1 frames; cursor still in more than 25 % of frames; map not in view',
      'PASS: NO (≥ 99 % ≤ 20 ms, flush p95 ≤ 2 ms, max ≤ 8 ms)',
    ]);
  });

  it('a passing mouse run without a map', () => {
    const s = scrubStats(input({ touch: false, mapInView: null }));
    expect(formatScrub(s, { name: 'ride-4h', n: 14400, startedOn: '–', input: 'mouse' }).split('\n')).toEqual([
      'Scrub record · ride-4h · N 14400 · input mouse · started on –',
      'Frames 100 · interval ms median / p95 / max 16.50 / 16.50 / 16.50 · ≤ 20 ms 100.00 %',
      'Flush ms (100) total p95 / max 1.00 / 1.00 · charts p95 / max 0.75 / 0.75 · map p95 / max 0.25 / 0.25',
      'Cursor changed in 100.00 % of frames · frames without a finger – · map in view at start no map',
      'Counts: yes',
      'PASS: yes (≥ 99 % ≤ 20 ms, flush p95 ≤ 2 ms, max ≤ 8 ms)',
    ]);
  });

  it('values that do not exist print as –', () => {
    const s = scrubStats(input({ flushes: 0 }));
    expect(formatScrub(s, { name: 'x', n: 1, startedOn: 'chart', input: 'touch' }).split('\n')[2]).toBe(
      'Flush ms (0) total p95 / max – / – · charts p95 / max – / – · map p95 / max – / –',
    );
  });
});
