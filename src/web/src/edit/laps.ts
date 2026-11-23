// Owner-added lap boundaries on the edit page (F-65 AL-8, AL-9; AP-8, OQ-301 to OQ-305). Owner
// chunk. Holds the set (draft.bin sample indices), merges it over draft.bin's device laps into
// data.meta.laps, so every reader of the laps (laps view, row taps, map lap markers, F-51 chart,
// F-64 blocks) shows it before Save; "Add lap" for the "+" disclosure; the set Save sends. Mirrors
// the server's rules (lib/addedlaps); the server alone decides.

import type { Lap, WorkoutData } from '../lib/datafile';
import { addLaps, lapEnds, MAX_ADDED_LAPS, removableBoundaries, removeAutoLaps, snapLapBoundary } from '../lib/addedlaps';
import { createRangeStats } from '../lib/range';
import { lapsBySection, sections } from '../lib/sections';
import { indexAt, instantAt, removed } from '../pins';
import type { PlusItem } from './plus';
import { t } from './texts';

/** A new boundary this close (seconds) to an existing one is no boundary (a split end misses a device lap by a sample or two). */
const LAP_TOLERANCE_S = 2;

/** One lap row's × (LapsTable's `remove`): its boundary, name and dimmed reason; null = no ×. */
export type LapRemoval = { readonly at: number; readonly name: string; readonly dim: string } | null;

export interface AddedLaps {
  /** Per lap of data.meta.laps, in order. */
  rows(): LapRemoval[];
  /** The × of a row: removes the boundary at sample `at`. */
  remove(at: number): void;
  /** "Add lap" at the cursor or both range ends (OQ-304), for the "+" disclosure. */
  item(cursor: () => number, range: () => readonly [number, number]): PlusItem;
  /** The settings to send when they differ from the stored ones, else {} (E-6). */
  patch(stored: readonly string[], removeStored: boolean): { added_lap_boundaries?: string[]; remove_auto_laps?: boolean };
  /** A settings response: the stored values are the truth. */
  saved(stored: readonly string[], removeStored: boolean): void;
  /** F-66 RA-9: the line under the laps table; null = not shown (no removable boundary in the live trim window, setting off). */
  auto(): { readonly on: boolean; readonly label: string } | null;
  /** F-66: switch "Remove auto-laps" on or off; the laps are recomputed. */
  toggle(): void;
}

/** `cut()`: the live trim window [a, b] (TR-9). `onchange`: the set changed (data.meta.laps is the new merge). */
export function createAddedLaps(data: WorkoutData, startS: number, stored: readonly string[], removeStored: boolean, cut: () => readonly [number, number], onchange: () => void): AddedLaps {
  const { time, sampleCount: n } = data;
  const orig = data.meta.laps;
  const stats = createRangeStats(data);
  // F-66 RA-9: the base of the merge is the device laps, or the laps after removing the auto-laps.
  let on = removeStored;
  const based = (): readonly Lap[] => (on ? removeAutoLaps({ ...data, meta: { ...data.meta, laps: orig } }, stats) : orig);
  let device = based();
  let ends = lapEnds(device, n);
  /** F-66 RA-8: entries a restore took off the merge (on a restored device boundary); back when it is switched on again. */
  let held: number[] = [];
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
  const rebase = (): void => {
    device = based();
    ends = lapEnds(device, n);
  };
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
        // Within LAP_TOLERANCE_S of an existing end, added boundary or the cut counts as there already.
        const there = [...ends, ...set, lo, hi].map((k) => time[k]);
        const add = [...new Set(ts.filter((k) => !there.some((x) => Math.abs(x - time[k]) <= LAP_TOLERANCE_S)))];
        if (add.length === 0) return t('lapHere');
        // Only what Save would send counts: boundaries the live trim removes do not (AL-8).
        return set.filter(kept).length + add.length > MAX_ADDED_LAPS ? t('lapMax') : add;
      };
      return {
        label: () => t('addLap'),
        short: () => t('addLapShort'),
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
    patch: (stored, removeStored) => {
      const send = set.filter(kept).map((k) => instantAt(time, startS, k));
      return { ...(JSON.stringify(send) === JSON.stringify(stored) ? {} : { added_lap_boundaries: send }), ...(on === removeStored ? {} : { remove_auto_laps: on }) };
    },
    saved: (stored, removeStored) => {
      const was = on;
      held = []; // the stored state is the truth: what a Save dropped does not come back
      on = removeStored;
      if (on !== was) rebase();
      const next = load(stored);
      if (next.join() === set.join() && on === was) return;
      set = next;
      change();
    },
    auto: () => {
      const n1 = on ? 1 : removableBoundaries({ laps: orig, sessions: data.meta.sessions }, time, n, cut()[0], cut()[1]);
      return n1 === 0 ? null : { on, label: on ? t('restoreAutoLaps') : t('removeAutoLaps', { n: n1 }) };
    },
    toggle: () => {
      on = !on;
      rebase();
      if (on) {
        set = [...set, ...held.filter((k) => !ends.has(k) && !set.includes(k))].sort((a, b) => a - b);
        held = [];
      } else {
        const near = (k: number): boolean => [...ends].some((e) => Math.abs(time[e] - time[k]) <= LAP_TOLERANCE_S);
        held = [...held, ...set.filter(near)];
        set = set.filter((k) => !near(k));
      }
      change();
    },
  };
}
