// Scrub record statistics: validity, pass rule and the result block (F-07 design F-4;
// ADR-016 decisions 2 and 3). Pure: imports nothing, no DOM.

export const RECORD_MS = 10_000,
  COUNTDOWN_S = 3,
  MAX_SAMPLES = 8192;
export const FRAME_OK_MS = 20,
  PASS_SHARE = 0.99,
  FLUSH_P95_MS = 2,
  FLUSH_MAX_MS = 8;
export const CAPPED_MEDIAN_MS = 25,
  MIN_CHANGED_SHARE = 0.75;

/** Nearest rank on a sorted array: sorted[ceil(q·n) − 1]; NaN when empty. */
export function quantile(sorted: Float64Array, q: number): number {
  const n = sorted.length;
  if (n === 0) return Number.NaN;
  return sorted[Math.min(n - 1, Math.max(0, Math.ceil(q * n) - 1))];
}

export interface ScrubInput {
  /** Frame intervals in ms; the first min(frames, length) entries are used. */
  intervals: Float64Array;
  frames: number;
  /** Flush parts in ms (Z − A, M − A, Z − M); the first min(flushes, length) entries are used. */
  total: Float64Array;
  charts: Float64Array;
  map: Float64Array;
  flushes: number;
  changedFrames: number;
  noFingerFrames: number;
  touch: boolean;
  /** null: the workout has no map. */
  mapInView: boolean | null;
}

export interface ScrubStats {
  frames: number;
  intervalMedian: number;
  intervalP95: number;
  intervalMax: number;
  /** Share of frame intervals ≤ FRAME_OK_MS, 0 to 1 (NaN without frames). */
  okShare: number;
  flushes: number;
  totalP95: number;
  totalMax: number;
  chartsP95: number;
  chartsMax: number;
  mapP95: number;
  mapMax: number;
  /** Share of frames in which the cursor changed, 0 to 1 (NaN without frames). */
  changedShare: number;
  /** null when the run was not a touch run. */
  noFingerFrames: number | null;
  mapInView: boolean | null;
  /** The run counts (no validity rule applies, ADR-016 decision 3). */
  counts: boolean;
  reasons: string[];
  /** The pass rule (ADR-016 decision 2); bounds inclusive. */
  pass: boolean;
}

function sorted(values: Float64Array, count: number): Float64Array {
  return values.slice(0, Math.min(Math.max(0, count), values.length)).sort();
}

export function scrubStats(input: ScrubInput): ScrubStats {
  const intervals = sorted(input.intervals, input.frames);
  const total = sorted(input.total, input.flushes);
  const charts = sorted(input.charts, input.flushes);
  const map = sorted(input.map, input.flushes);

  let ok = 0;
  for (let k = 0; k < intervals.length; k++) if (intervals[k] <= FRAME_OK_MS) ok++;
  const okShare = intervals.length === 0 ? Number.NaN : ok / intervals.length;
  const frames = input.frames;
  const changedShare = frames === 0 ? Number.NaN : input.changedFrames / frames;
  const intervalMedian = quantile(intervals, 0.5);
  const totalP95 = quantile(total, 0.95);
  const totalMax = quantile(total, 1);

  const reasons: string[] = [];
  if (frames === 0) reasons.push('no frames');
  if (intervalMedian >= CAPPED_MEDIAN_MS) reasons.push('30 Hz cap (median ≥ 25 ms; Low Power Mode on?)');
  if (input.touch && input.noFingerFrames > 0) reasons.push(`no finger down in ${input.noFingerFrames} frames`);
  if (changedShare < MIN_CHANGED_SHARE) reasons.push('cursor still in more than 25 % of frames');
  if (input.mapInView === false) reasons.push('map not in view');

  return {
    frames,
    intervalMedian,
    intervalP95: quantile(intervals, 0.95),
    intervalMax: quantile(intervals, 1),
    okShare,
    flushes: input.flushes,
    totalP95,
    totalMax,
    chartsP95: quantile(charts, 0.95),
    chartsMax: quantile(charts, 1),
    mapP95: quantile(map, 0.95),
    mapMax: quantile(map, 1),
    changedShare,
    noFingerFrames: input.touch ? input.noFingerFrames : null,
    mapInView: input.mapInView,
    counts: reasons.length === 0,
    reasons,
    // NaN (no frames, no flushes) compares false, so such a run never passes.
    pass: okShare >= PASS_SHARE && totalP95 <= FLUSH_P95_MS && totalMax <= FLUSH_MAX_MS,
  };
}

/** 2 decimals; '–' for a value that does not exist (NaN). */
export function fixed(x: number): string {
  return x === x ? x.toFixed(2) : '–';
}

const percent = (share: number): string => fixed(share * 100);

/** The result block without its last line (dpr · window · user agent), which the recorder
 *  adds because it reads the browser. Lines joined with '\n', no trailing newline. */
export function formatScrub(stats: ScrubStats, context: { name: string; n: number; startedOn: string; input: string }): string {
  const s = stats;
  return [
    `Scrub record · ${context.name} · N ${context.n} · input ${context.input} · started on ${context.startedOn}`,
    `Frames ${s.frames} · interval ms median / p95 / max ${fixed(s.intervalMedian)} / ${fixed(s.intervalP95)} / ${fixed(s.intervalMax)} · ≤ 20 ms ${percent(s.okShare)} %`,
    `Flush ms (${s.flushes}) total p95 / max ${fixed(s.totalP95)} / ${fixed(s.totalMax)} · charts p95 / max ${fixed(s.chartsP95)} / ${fixed(s.chartsMax)} · map p95 / max ${fixed(s.mapP95)} / ${fixed(s.mapMax)}`,
    `Cursor changed in ${percent(s.changedShare)} % of frames · frames without a finger ${s.noFingerFrames === null ? '–' : s.noFingerFrames} · map in view at start ${s.mapInView === null ? 'no map' : s.mapInView ? 'yes' : 'no'}`,
    `Counts: ${s.counts ? 'yes' : 'NO: ' + s.reasons.join('; ')}`,
    `PASS: ${s.pass ? 'yes' : 'NO'} (≥ 99 % ≤ 20 ms, flush p95 ≤ 2 ms, max ≤ 8 ms)`,
  ].join('\n');
}
