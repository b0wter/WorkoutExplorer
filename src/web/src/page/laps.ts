// The laps enhancer (F-18 design L-2 to L-7; AP-7, AP-8, DS-1). Framework-free, on the server
// markup's hooks, so it runs on the template's and the edit page's laps module alike. It adds row
// taps into the selection store, the mark of the selected row, the laps / splits switch in the
// heading slot and the splits table; the server table is adopted, never re-rendered (ADR-026 item 4).

import type { WorkoutData } from '../lib/datafile';
import { M_PER_MILE, type Units } from '../lib/format';
import { distanceAdvances, lapsGroup } from '../lib/page';
import { createRangeStats, emptyValues, splitBounds, type RangeStats } from '../lib/range';
import type { SelectionStore } from '../lib/selection';
import type { UnitsStore } from '../lib/units';

/** Row ranges as flat pairs [a0, b0, a1, b1, …], sample indices, both inclusive (L-2). */
type Ranges = readonly number[];

/** One splits table (L-4, L-5): range values over each split, formatted by lib/page's lapsGroup.
 *  NaN values reach lapsGroup as non-finite totals, which it treats as absent. */
function splitsTable(doc: Document, sport: string, stats: RangeStats, bounds: number[], units: Units): HTMLTableElement {
  const out = emptyValues();
  const totals = [];
  for (let i = 1; i < bounds.length; i++) {
    const v = stats.query(bounds[i - 1], bounds[i], out);
    totals.push({
      totals: {
        elapsed_time: v.elapsed,
        distance: v.distance,
        avg_speed: v.speed,
        ascent: v.gain,
        avg_heart_rate: v.heartRate,
        avg_power: v.power,
        avg_cadence: v.cadence,
      },
    });
  }
  const group = lapsGroup(sport, totals, units);
  const table = doc.createElement('table');
  const head = table.createTHead().insertRow();
  const body = table.createTBody();
  for (const c of group?.Columns ?? []) {
    const th = doc.createElement('th');
    th.dataset.k = c.Key;
    th.textContent = c.Label;
    head.appendChild(th);
  }
  for (const row of group?.Rows ?? []) {
    const tr = body.insertRow();
    for (const t of row) tr.insertCell().textContent = t;
  }
  return table;
}

/** Adds the switch and splits table now. Returns `taps`, which makes rows tappable: the glue
 *  calls it once the range panel is mounted (or will never be), so the L-7 flush always runs after
 *  the panel's slot listener: a tap before that would move the row uncompensated (QA-18-r1-3).
 *  And `legs`, which gives the separator rows (F-42 MS-6) their ranges, flat pairs in row order. */
export function enhanceLaps(
  root: ParentNode,
  data: WorkoutData,
  selection: SelectionStore,
  units: UnitsStore,
  win: Window = window,
): { taps(): void; legs(pairs: Ranges): void } {
  const section = root.querySelector<HTMLElement>('section.laps');
  if (!section) return { taps: () => {}, legs: () => {} };
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
  const legs = (pairs: Ranges): void => laps.forEach((a, i) => a !== a && (laps[i] = pairs[j++]));
  const doc = section.ownerDocument;
  const server = section.querySelector<HTMLElement>('.laps-scroll, .laps-none');

  let splits: HTMLElement | null = null;
  let splitRanges: Ranges = [];
  let stats: RangeStats | null = null;
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
      const ranges = splits?.contains(row) ? splitRanges : laps;
      const i = 2 * row.sectionRowIndex;
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
  // L-6: the switch only when the distance advances; laps first, held in memory only (OQ-76).
  const slot = section.querySelector('#laps-slot');
  if (!slot || !server || slot.firstChild || !distanceAdvances(data)) return { taps, legs };
  const build = (): void => {
    stats ??= createRangeStats(data);
    const u = units.get();
    const d = stats.distance;
    const bounds = d ? splitBounds(d, u === 'metric' ? 1000 : M_PER_MILE) : [0];
    splitRanges = bounds.slice(1).flatMap((b, i) => [bounds[i], b]);
    const wrap = doc.createElement('div');
    wrap.className = 'laps-scroll splits';
    wrap.hidden = splits?.hidden ?? true;
    wrap.appendChild(splitsTable(doc, data.meta.sport, stats, bounds, u));
    if (marked && splits?.contains(marked)) mark(null);
    if (splits) splits.replaceWith(wrap);
    else section.appendChild(wrap);
    splits = wrap;
  };
  const control = doc.createElement('span');
  control.className = 'laps-switch';
  control.innerHTML = '<button type="button" aria-pressed="true">Laps</button><button type="button" aria-pressed="false">Splits</button>';
  const [lapsButton, splitsButton] = control.children;
  const show = (onSplits: boolean): void => {
    if (onSplits && !splits) build();
    server.hidden = onSplits;
    if (splits) splits.hidden = !onSplits;
    lapsButton.setAttribute('aria-pressed', String(!onSplits));
    splitsButton.setAttribute('aria-pressed', String(onSplits));
  };
  lapsButton.addEventListener('click', () => show(false));
  splitsButton.addEventListener('click', () => show(true));
  // km and mi give different rows: a built splits table is rebuilt (L-6).
  units.subscribe(() => {
    if (splits) build();
  });
  slot.appendChild(control);
  return { taps, legs };
}
