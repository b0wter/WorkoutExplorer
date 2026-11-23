// Added lap boundaries, pure (F-65 AL-3, AL-4; ADR-060): the edit page's mirror of Go's
// workout/addedlaps.go over draft.bin (the same samples, device laps only). Sample indices of the
// whole recording. Owner chunk only; never in the activity entry.

import type { Lap, Totals, WorkoutData, WorkoutMeta } from '../datafile';
import { emptyValues, type RangeStats } from '../range';

/** AL-2's cap. */
export const MAX_ADDED_LAPS = 100;

/** A lap that can be split: device lap `lap` (-1: implicit) over the inclusive samples [first, last]. */
interface Span {
  readonly first: number;
  readonly last: number;
  readonly lap: number;
}

/** The device laps with a sample, then the implicit laps: every run of samples no device lap
 *  covers, widened by one sample each side (without device laps: the whole recording). */
function spans(laps: readonly Lap[], n: number): Span[] {
  const out: Span[] = [];
  const covered = new Uint8Array(n);
  laps.forEach((l, i) => {
    if (l.end_index - l.start_index < 1) return;
    out.push({ first: l.start_index, last: l.end_index - 1, lap: i });
    covered.fill(1, l.start_index, l.end_index);
  });
  for (let k = 0; k < n; ) {
    if (covered[k]) {
      k++;
      continue;
    }
    const p = k;
    while (k < n && !covered[k]) k++;
    out.push({ first: Math.max(p - 1, 0), last: Math.min(k, n - 1), lap: -1 });
  }
  return out;
}

/** The first and last sample of every device and implicit lap: where a boundary is refused. */
export function lapEnds(laps: readonly Lap[], n: number): Set<number> {
  return new Set(spans(laps, n).flatMap((s) => [s.first, s.last]));
}

/** AL-4: sample i inside a pool-swim session with lengths (of overlapping sessions the
 *  latest-starting) moves to the nearest last sample of that session's lengths, by time, ties to
 *  the earlier; otherwise i. */
export function snapLapBoundary(meta: Pick<WorkoutMeta, 'sessions' | 'lengths'>, time: Float64Array, i: number): number {
  let sf = -1;
  let st = 0;
  let pool = false;
  for (const s of meta.sessions)
    if (i >= s.start_index && i < s.end_index && s.start_index >= sf) [sf, st, pool] = [s.start_index, s.end_index, s.sub_sport === 'lap_swimming'];
  let best = -1;
  let bestD = 0;
  if (pool)
    for (const l of meta.lengths) {
      const last = l.end_index - 1;
      if (l.end_index - l.start_index < 1 || last < sf || last >= st) continue;
      const d = Math.abs(time[last] - time[i]);
      if (best < 0 || d < bestD || (d === bestD && last < best)) [best, bestD] = [last, d];
    }
  return best < 0 ? i : best;
}

/** Go's completeTotals on cleared totals: range values over the samples [a, b], nothing below two
 *  samples; no speed, power or cadence for a lap starting in a transition (F-18 L-1). */
function rangeTotals(meta: Pick<WorkoutMeta, 'sessions'>, stats: RangeStats, a: number, b: number, start: number): Totals {
  if (b <= a) return {};
  const v = stats.query(a, b, emptyValues());
  const o: Record<string, number> = { elapsed_time: v.elapsed };
  const put = (k: string, x: number): void => void (x === x && (o[k] = x));
  put('distance', v.distance);
  put('ascent', v.gain);
  put('avg_heart_rate', v.heartRate);
  if (meta.sessions.find((s) => s.start <= start && start <= s.end)?.sport !== 'transition') {
    put('avg_speed', v.speed);
    put('avg_power', v.power);
    put('avg_cadence', v.cadence);
  }
  return o;
}

/** AL-3: every sample of `at` strictly inside (lo, hi) that is not a lap end splits the lap that
 *  holds it (of overlapping laps, the later-starting). Parts share the split sample; their totals
 *  are range values (F-18 L-1, no speed, power or cadence for a part starting in a transition);
 *  every part but the last gets 'added', the last keeps the lap's trigger ('' for an implicit lap).
 *  Unsplit laps are returned as they were, all in time order. Anything else in `at` is skipped. */
export function addLaps(data: Pick<WorkoutData, 'meta' | 'time' | 'sampleCount'>, stats: RangeStats, at: readonly number[], lo: number, hi: number): Lap[] {
  const { meta, time } = data;
  const all = spans(meta.laps, data.sampleCount);
  const ends = new Set(all.flatMap((s) => [s.first, s.last]));
  const cuts = new Map<number, number[]>(); // span index → split samples
  for (const k of at) {
    if (k <= lo || k >= hi || ends.has(k)) continue;
    let best = -1;
    all.forEach((s, j) => {
      if (k > s.first && k < s.last && (best < 0 || s.first >= all[best].first)) best = j;
    });
    const c = cuts.get(best) ?? [];
    if (best >= 0 && !c.includes(k)) cuts.set(best, [...c, k]);
  }
  if (cuts.size === 0) return [...meta.laps];
  const totals = (a: number, b: number, start: number): Totals => rangeTotals(meta, stats, a, b, start);
  const parts = (j: number, base: Lap): Lap[] => {
    const ks = cuts.get(j)!.sort((x, y) => x - y);
    let start = base.start;
    let from = base.start_index;
    return [...ks, -1].map((k) => {
      const last = k < 0;
      const p: Lap = { ...base, start, start_index: from, end: last ? base.end : time[k], end_index: last ? base.end_index : k + 1, trigger: last ? base.trigger : 'added', totals: {} };
      start = time[k];
      from = k;
      return { ...p, totals: totals(p.start_index, p.end_index - 1, p.start) };
    });
  };
  const laps: Lap[] = [];
  const spanOf = new Map(all.map((s, j) => [s.lap, j]));
  meta.laps.forEach((l, i) => {
    const j = spanOf.get(i);
    laps.push(...(j !== undefined && cuts.has(j) ? parts(j, l) : [l]));
  });
  all.forEach((s, j) => {
    if (s.lap < 0 && cuts.has(j)) laps.push(...parts(j, { start: time[s.first], end: time[s.last], start_index: s.first, end_index: s.last + 1, trigger: '', stroke: '', totals: {} }));
  });
  return laps.sort((x, y) => x.start - y.start); // stable
}

/** F-66 OQ-313: a device lap ended by distance, time, position, or by nothing the parser knows. */
export const isAutoLap = (trigger: string): boolean => trigger === 'distance' || trigger === 'time' || trigger === 'position' || trigger === '';

/** MS-3: the last session starting at or before the lap's first sample. */
const legOf = (meta: Pick<WorkoutMeta, 'sessions'>, time: Float64Array, n: number, l: Lap): number => {
  const at = l.start_index < n ? time[l.start_index] : l.start;
  let k = -1;
  meta.sessions.forEach((s, i) => void (s.start <= at && (k = i)));
  return k;
};

/** F-66 RA-3, RA-5: the mirror of Go's workout.RemoveAutoLaps over the device laps. The end of every
 *  auto-lap with a next lap in the same leg goes; a run of those laps and the lap after it become one:
 *  first start, latest end, trigger of the last part, common stroke or 'mixed', range values. One lap
 *  left after a removal: none (OQ-307). */
export function removeAutoLaps(data: Pick<WorkoutData, 'meta' | 'time' | 'sampleCount'>, stats: RangeStats): Lap[] {
  const { meta, time, sampleCount: n } = data;
  const laps = meta.laps;
  const out: Lap[] = [];
  let removed = false;
  for (let i = 0; i < laps.length; ) {
    let j = i;
    while (j + 1 < laps.length && isAutoLap(laps[j].trigger) && legOf(meta, time, n, laps[j]) === legOf(meta, time, n, laps[j + 1])) j++;
    if (j === i) {
      out.push(laps[i++]);
      continue;
    }
    removed = true;
    let [end, end_index, stroke] = [laps[i].end, laps[i].end_index, ''];
    for (const p of laps.slice(i, j + 1)) {
      if (p.end > end) [end, end_index] = [p.end, p.end_index];
      stroke = p.stroke === '' || p.stroke === stroke ? stroke : stroke === '' ? p.stroke : 'mixed';
    }
    const m: Lap = { ...laps[i], end, end_index, trigger: laps[j].trigger, stroke, totals: rangeTotals(meta, stats, laps[i].start_index, end_index - 1, laps[i].start) };
    out.push(m);
    i = j + 1;
  }
  return !removed ? [...laps] : out.length === 1 ? [] : out;
}

/** F-66 RA-9: the number of removable auto-lap boundaries strictly inside the samples (lo, hi). */
export function removableBoundaries(meta: Pick<WorkoutMeta, 'laps' | 'sessions'>, time: Float64Array, n: number, lo: number, hi: number): number {
  const { laps } = meta;
  let c = 0;
  for (let i = 0; i + 1 < laps.length; i++) {
    const k = laps[i].end_index - 1;
    if (isAutoLap(laps[i].trigger) && legOf(meta, time, n, laps[i]) === legOf(meta, time, n, laps[i + 1]) && k > lo && k < hi) c++;
  }
  return c;
}
