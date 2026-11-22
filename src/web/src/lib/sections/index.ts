// Multisport sections (F-42 MS-1 to MS-3, AP-12). Pure; imports only lib/datafile and
// lib/format. Mirrors Go internal/page sections.go: the committed page samples pin both.

import type { Lap, WorkoutMeta } from '../datafile';
import { sportName } from '../format/dates';

/** One section: its name, first sample and session sport. */
export interface Section {
  readonly name: string;
  readonly start: number;
  readonly sport: string;
}

/** MS-1, MS-2: null unless the sessions hold two or more distinct sports; else one section per
 *  session in start order (file order on ties), the first starting at sample 0, transitions
 *  named T1, T2, … in order. */
export function sections(meta: Pick<WorkoutMeta, 'sessions'>): Section[] | null {
  if (new Set(meta.sessions.map((s) => s.sport)).size < 2) return null;
  let t = 0;
  return [...meta.sessions]
    .sort((a, b) => a.start - b.start)
    .map((s, i) => ({
      name: s.sport === 'transition' ? 'T' + ++t : sportName(s.sport),
      start: i === 0 ? 0 : s.start_index,
      sport: s.sport,
    }));
}

/** MS-3: the last section whose start ≤ the lap's start_index. */
export function sectionOf(secs: readonly Section[], lap: Pick<Lap, 'start_index'>): number {
  let k = 0;
  secs.forEach((s, j) => {
    if (s.start <= lap.start_index) k = j;
  });
  return k;
}

/** F-64 LL-2 (MS-3): the laps grouped by section, every section in order, a section without laps
 *  with none; a lap out of order stays in the block above it. Empty without sections or laps. */
export function lapsBySection<T extends Pick<Lap, 'start_index'>>(secs: readonly Section[] | null, laps: readonly T[]): T[][] {
  if (!secs || laps.length === 0) return [];
  const out: T[][] = secs.map(() => []);
  let cur = 0;
  for (const l of laps) cur = Math.max(cur, sectionOf(secs, l)), out[cur].push(l);
  return out;
}

/** MS-6: each section's range as flat pairs [a, b] in section order: a = its start, b = the next
 *  section's start (the shared boundary sample, as adjacent laps) or N − 1, never past N − 1.
 *  Selectable when b > a. */
export function sectionRanges(secs: readonly Section[], n: number): number[] {
  return secs.flatMap((s, k) => [s.start, Math.min(secs[k + 1]?.start ?? n - 1, n - 1)]);
}

/** A section that has samples, with its range end (sectionRanges' b). */
export interface Leg extends Section {
  readonly end: number;
}

/** MS-2, MS-7, MS-8: the sections that have samples (start < N and before the next start); only
 *  these get a chart line, a name on the chart and a map marker. */
export function legs(secs: readonly Section[], n: number): Leg[] {
  const ranges = sectionRanges(secs, n);
  return secs.flatMap((s, k) => (s.start < Math.min(secs[k + 1]?.start ?? n, n) ? [{ ...s, end: ranges[2 * k + 1] }] : []));
}
