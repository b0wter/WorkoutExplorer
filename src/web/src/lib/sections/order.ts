// MS-1, MS-2 without names (F-67 LP-10): the one ordering rule, small enough for the viewing entry
// (the readout and the range panel); lib/sections' index builds on it. Imports nothing but types.

import type { WorkoutMeta } from '../datafile';

/** A section's first sample and session sport. */
export interface Start {
  readonly start: number;
  readonly sport: string;
}

/** MS-1, MS-2: null unless the sessions hold two or more distinct sports; else one entry per
 *  session in start order (file order on ties), the first starting at sample 0. */
export function ordered(meta: Pick<WorkoutMeta, 'sessions'>): Start[] | null {
  if (new Set(meta.sessions.map((s) => s.sport)).size < 2) return null;
  return [...meta.sessions].sort((a, b) => a.start - b.start).map((s, i) => ({ start: i ? s.start_index : 0, sport: s.sport }));
}

/** The sport whose units apply to samples a..b: their leg on a multisport workout (YD-8), else the workout's. */
export function legSport(meta: Pick<WorkoutMeta, 'sessions' | 'sport'>, a: number, b = a): string {
  const s = ordered(meta);
  return (s && sportIn(s, a, b)) || meta.sport;
}

/** The sport of the section holding samples a..b (the last whose start ≤ a), or '' when the range
 *  reaches past that section's end (the next start, F-42 MS-6). No allocation (readout flush). */
export function sportIn(secs: readonly Start[], a: number, b = a): string {
  let k = 0;
  for (let j = 1; j < secs.length; j++) if (secs[j].start <= a) k = j;
  return b > (secs[k + 1]?.start ?? b) ? '' : secs[k].sport;
}
