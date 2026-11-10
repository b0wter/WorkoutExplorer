// Multisport sections (F-42 MS-1 to MS-3, AP-12). Pure; imports only lib/datafile and
// lib/format. Mirrors Go internal/page sections.go: the committed page samples pin both.

import type { Lap, WorkoutMeta } from '../datafile';
import { sportName } from '../format/dates';

/** One section: its name and first sample. */
export interface Section {
  readonly name: string;
  readonly start: number;
}

/** A separator row of the laps table and the lap rows under it (MS-3, MS-6): name and column span
 *  as strings, as the template prints them. */
export interface LapsSection {
  readonly Name: string;
  readonly Span: string;
  readonly Rows: readonly (readonly string[])[];
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

/** MS-3: lap rows (one per lap) grouped by section, every section in order, a section without laps
 *  with no rows; a lap out of order stays under the separator above it. Empty without sections or
 *  laps. */
export function lapSections(
  secs: readonly Section[] | null,
  laps: readonly Lap[],
  rows: readonly (readonly string[])[],
  span: number,
): LapsSection[] {
  const out: { Name: string; Span: string; Rows: (readonly string[])[] }[] = [];
  if (!secs || laps.length === 0) return out;
  const upTo = (k: number) => {
    while (out.length <= k) out.push({ Name: secs[out.length].name, Span: String(span), Rows: [] });
  };
  laps.forEach((l, i) => {
    upTo(sectionOf(secs, l));
    out[out.length - 1].Rows.push(rows[i]);
  });
  upTo(secs.length - 1);
  return out;
}
