// The laps enhancer (F-18 design L-2, L-3, L-7; AP-7, AP-8). Framework-free, on the server
// markup's hooks, so it runs on the template's and the edit page's laps module alike. It adds row
// taps into the selection store and the mark of the selected row; the server table is adopted,
// never re-rendered (ADR-026 item 4). The switch and the splits table are the lazy splits chunk's
// (F-46 LS-4), whose rows tap through the `splits` seam (LS-8).

import type { WorkoutData } from '../lib/datafile';
import type { SelectionStore } from '../lib/selection';

/** Row ranges as flat pairs [a0, b0, a1, b1, …], sample indices, both inclusive (L-2). */
type Ranges = readonly number[];

export interface LapsSeams {
  /** Makes rows tappable: the glue calls it once the range panel is mounted (or will never be), so
   *  the L-7 flush always runs after the panel's slot listener: a tap before that would move the row
   *  uncompensated (QA-18-r1-3). */
  taps(): void;
  /** Gives the separator rows (F-42 MS-6) their ranges, flat pairs in row order. */
  legs(pairs: Ranges): void;
  /** LS-8: the splits chunk's current table container and its row ranges; a marked row in the
   *  previous one is unmarked. */
  splits(container: Element, pairs: Ranges): void;
}

export function enhanceLaps(root: ParentNode, data: WorkoutData, selection: SelectionStore, win: Window = window): LapsSeams {
  const section = root.querySelector<HTMLElement>('section.laps');
  if (!section) return { taps: () => {}, legs: () => {}, splits: () => {} };
  // L-2: lap rows map to meta.laps by DOM order (a = start_index, b = end_index − 1, the shared
  // boundary sample included), only when the counts agree. A separator row's pair is NaN (not
  // selectable) until legs fills the NaNs in order.
  const meta = data.meta.laps;
  let laps: number[] = [];
  let k = 0;
  section.querySelectorAll('.laps-scroll tbody tr').forEach((r) => {
    const l = r.matches('.laps-section') ? null : meta[k++];
    laps.push(l ? l.start_index : NaN, l ? l.end_index - 1 : NaN);
  });
  if (k !== meta.length) {
    console.warn('page: laps rows do not match the data file; rows not selectable');
    laps = [];
  }
  let j = 0;
  const legs = (pairs: Ranges): void => {
    laps.forEach((a, i) => a !== a && (laps[i] = pairs[j++]));
    // QA-42-r1-4: separator rows look tappable only once they are.
    if (laps.length) section.classList.add('legs');
  };
  let splitsBox: Element | null = null;
  let splitRanges: Ranges = [];
  let marked: Element | null = null;
  let ma = -1;
  let mb = -1;

  const mark = (row: Element | null, a = -1, b = -1): void => {
    marked?.classList.remove('on');
    row?.classList.add('on');
    marked = row;
    ma = a;
    mb = b;
  };
  // L-3: O(1) per flush, a drag frame compares with the marked row only.
  selection.subscribe((a, b) => {
    if (marked && (a !== ma || b !== mb)) mark(null);
  });

  const taps = (): void => {
    section.classList.add('live');
    section.addEventListener('click', (event) => {
      const row = (event.target as Element).closest<HTMLTableRowElement>('tbody tr');
      if (!row) return;
      const ranges = splitsBox?.contains(row) ? splitRanges : laps;
      const i = 2 * (row.rowIndex - 1); // one thead row; a multisport table has one tbody per leg
      const a = ranges[i];
      const b = ranges[i + 1];
      // Not selectable (fewer than 2 samples, or a markup that does not match); the same range again.
      if (!(b > a) || (a === selection.start() && b === selection.end())) return;
      if (selection.start() < 0) {
        // L-7: the panel appears above the laps. Compare the row's top before the write with its top
        // in the flush, after the glue's slot listener (subscribed earlier), and scroll the difference.
        const top = row.getBoundingClientRect().top;
        let first = true;
        const off = selection.subscribe(() => {
          if (first) {
            first = false;
            return;
          }
          off();
          win.scrollBy(0, row.getBoundingClientRect().top - top);
        });
      }
      mark(row, a, b);
      selection.set(a, b);
    });
  };
  const splits = (container: Element, pairs: Ranges): void => {
    if (marked && splitsBox?.contains(marked)) mark(null);
    splitsBox = container;
    splitRanges = pairs;
  };
  return { taps, legs, splits };
}
