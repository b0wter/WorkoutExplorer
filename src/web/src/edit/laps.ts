// Owner-added lap boundaries on the edit page (F-65 AL-8, AL-9; AP-8, OQ-301 to OQ-305). Owner
// chunk. Holds the set (draft.bin sample indices), merges it over draft.bin's device laps into
// data.meta.laps, so every reader of the laps (laps view, row taps, map lap markers, F-51 chart,
// F-64 blocks) shows it before Save; "Add lap" for the "+" disclosure; the set Save sends. Mirrors
// the server's rules (lib/addedlaps); the server alone decides.

import type { Lap, WorkoutData } from '../lib/datafile';
import { addLaps, lapEnds, MAX_ADDED_LAPS, snapLapBoundary } from '../lib/addedlaps';
import { createRangeStats } from '../lib/range';
import { lapsBySection, sections } from '../lib/sections';
import { indexAt, instantAt, removed } from '../pins';
import type { PlusItem } from './plus';
import { t } from './texts';

/** One lap row's × (LapsTable's `remove`): its boundary, name and dimmed reason; null = no ×. */
export type LapRemoval = { readonly at: number; readonly name: string; readonly dim: string } | null;

export interface AddedLaps {
  /** Per lap of data.meta.laps, in order. */
  rows(): LapRemoval[];
  /** The × of a row: removes the boundary at sample `at`. */
  remove(at: number): void;
  /** "Add lap" at the cursor or both range ends (OQ-304), for the "+" disclosure. */
  item(cursor: () => number, range: () => readonly [number, number]): PlusItem;
  /** `{ added_lap_boundaries }` when the set to send differs from the stored one, else {} (E-6). */
  patch(stored: readonly string[]): { added_lap_boundaries?: string[] };
  /** A settings response: the stored set is the truth. */
  saved(stored: readonly string[]): void;
}

/** `cut()`: the live trim window [a, b] (TR-9). `onchange`: the set changed (data.meta.laps is the new merge). */
export function createAddedLaps(data: WorkoutData, startS: number, stored: readonly string[], cut: () => readonly [number, number], onchange: () => void): AddedLaps {
  const { time, sampleCount: n } = data;
  const device = data.meta.laps;
  const ends = lapEnds(device, n);
  const stats = createRangeStats(data);
  const snap = (i: number): number => snapLapBoundary(data.meta, time, i);
  /** AL-2's resolution of the stored set: nearest sample, the snap; an entry that is no longer a
   *  valid boundary (outside, a lap end, a repeat) is skipped and so never sent again (AL-3). */
  const load = (at: readonly string[]): number[] => {
    const out: number[] = [];
    for (const s of at) {
      const x = Date.parse(s) / 1000 - startS;
      const k = snap(indexAt(time, startS, s));
      if (n > 0 && x >= time[0] && x <= time[n - 1] && !ends.has(k) && !out.includes(k)) out.push(k);
    }
    return out.sort((a, b) => a - b);
  };
  let set = load(stored);
  // The merge over the whole recording: a boundary in the live-trimmed part keeps its lap, dimmed
  // with "Removed on Save" (AL-8), and is left out of the sent set.
  const merge = (): void => {
    (data.meta as { laps: readonly Lap[] }).laps = addLaps({ ...data, meta: { ...data.meta, laps: device } }, stats, set, 0, n - 1);
  };
  merge();
  const change = (): void => {
    merge();
    onchange();
  };
  const kept = (k: number): boolean => k > cut()[0] && k < cut()[1];

  return {
    rows: () => {
      // The number the row shows: per leg block on a multisport workout (F-64 LL-2), else overall.
      const legs = lapsBySection(sections(data.meta), data.meta.laps);
      const shown = legs.length ? legs.flatMap((ls) => ls.map((_, k) => k + 1)) : data.meta.laps.map((_, i) => i + 1);
      return data.meta.laps.map((l, i) => (l.trigger === 'added' && set.includes(l.end_index - 1) ? { at: l.end_index - 1, name: t('removeLap', { n: shown[i] }), dim: kept(l.end_index - 1) ? '' : removed() } : null));
    },
    remove: (at) => {
      set = set.filter((k) => k !== at);
      change();
    },
    item: (cursor, range) => {
      /** The new boundaries, or the reason there are none. */
      const fresh = (): number[] | string => {
        const [a, b] = range();
        const ts = (a >= 0 ? [a, b] : cursor() >= 0 ? [cursor()] : []).map(snap);
        const [lo, hi] = cut();
        if (ts.some((k) => k < lo || k > hi)) return t('lapTrimmed');
        const add = [...new Set(ts.filter((k) => k !== lo && k !== hi && !ends.has(k) && !set.includes(k)))];
        if (add.length === 0) return t('lapHere');
        // Only what Save would send counts: boundaries the live trim removes do not (AL-8).
        return set.filter(kept).length + add.length > MAX_ADDED_LAPS ? t('lapMax') : add;
      };
      return {
        label: () => t('addLap'),
        state: () => {
          const f = fresh();
          return typeof f === 'string' ? { off: true, why: f } : { off: false, why: '' };
        },
        run: () => {
          const f = fresh();
          if (typeof f === 'string') return;
          set = [...set, ...f].sort((x, y) => x - y);
          change();
        },
      };
    },
    patch: (stored) => {
      const send = set.filter(kept).map((k) => instantAt(time, startS, k));
      return JSON.stringify(send) === JSON.stringify(stored) ? {} : { added_lap_boundaries: send };
    },
    saved: (stored) => {
      const next = load(stored);
      if (next.join() === set.join()) return;
      set = next;
      change();
    },
  };
}
