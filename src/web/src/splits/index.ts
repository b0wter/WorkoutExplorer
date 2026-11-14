// Splits and lengths in the browser (F-46 LS-4 to LS-9, F-48 PL-8; AP-8, AP-13, AP-20). LAZY: reached
// only through the page glue's import(), after the charts' first paint, when the distance advances or
// the workout has lengths. Fills the laps heading slot (the Laps / Splits / Lengths switch when two
// or more exist, the split-distance drop-down in the splits view) and builds the splits and lengths
// tables on demand; on a page without laps it shows the first view at once and reveals the module.
// Under the tables, the comparison chart of the shown view's rows (F-51, ./chart).

import './splits.css';
import type { Length, Totals, WorkoutData } from '../lib/datafile';
import type { formatElapsed, formatSpeedOrPace, isPaceSport, speedOrPaceUnit, Units } from '../lib/format';
import type { codeWord, lapsGroup } from '../lib/page';
import { hasStrokeCounts, lengthRows } from '../lib/lengths';
import { netElevation } from '../lib/compare';
import { createRangeStats, emptyValues, type RangeStats } from '../lib/range';
import { partialLast, poolDistance, presetBounds, splitChoice, type Pool, type Preset } from '../lib/splits';
import type { UnitsStore } from '../lib/units';
import type { LapsSeams } from '../page/laps';
import { createChart, type Slot } from './chart';

/** The entry's formatters, passed in: imported here, Vite splits lib/format out of the entry into a
 *  second script of the page. A tuple, as property names would survive minification in the entry. */
export type Fmt = readonly [
  lapsGroup: typeof lapsGroup,
  codeWord: typeof codeWord,
  formatElapsed: typeof formatElapsed,
  formatSpeedOrPace: typeof formatSpeedOrPace,
  speedOrPaceUnit: typeof speedOrPaceUnit,
  isPaceSport: typeof isPaceSport,
];

/** A table from a header and cell texts. */
function table(doc: Document, heads: readonly (readonly [key: string, label: string])[], rows: readonly (readonly string[])[]): HTMLTableElement {
  const t = doc.createElement('table');
  const head = t.createTHead().insertRow();
  const body = t.createTBody();
  for (const [k, label] of heads) {
    const th = doc.createElement('th');
    th.dataset.k = k;
    th.textContent = label;
    head.appendChild(th);
  }
  for (const row of rows) {
    const tr = body.insertRow();
    for (const c of row) tr.insertCell().textContent = c;
  }
  return t;
}

/** The lengths table (F-48 PL-8): number, stroke, time, pace, and strokes and SWOLF when any active
 *  length has a stroke count. A rest row shows "Rest" and its time only. Pace always per 100 m or yd
 *  (lengths exist in pool swims only). */
function lengthsTable([, codeWord, formatElapsed, formatSpeedOrPace, speedOrPaceUnit]: Fmt, doc: Document, lengths: readonly Length[], units: Units): HTMLTableElement {
  const counts = hasStrokeCounts(lengths);
  const heads: [string, string][] = [['number', '#'], ['stroke', 'Stroke'], ['elapsed_time', 'Time'], ['avg_speed', 'Pace (' + speedOrPaceUnit('swimming', units) + ')']];
  if (counts) heads.push(['strokes', 'Strokes'], ['swolf', 'SWOLF']);
  const rows = lengthRows(lengths).map((r, i) => {
    const l = lengths[i];
    const time = formatElapsed(r.time); // NaN: '–'
    if (l.rest) return ['Rest', '', time, '', ...(counts ? ['', ''] : [])];
    const cells = [String(r.number), codeWord('stroke', l.stroke), time, formatSpeedOrPace('swimming', l.avg_speed ?? 0, units) || '–'];
    if (counts) cells.push(l.strokes === undefined ? '–' : String(l.strokes), r.swolf === r.swolf ? String(r.swolf) : '–');
    return cells;
  });
  return table(doc, heads, rows);
}

/** Range values over each split, as lap totals (one source for the table and the chart, F-51 SC-3). */
function splitTotals(stats: RangeStats, bounds: number[]): { totals: Totals }[] {
  const out = emptyValues();
  const totals: { totals: Totals }[] = [];
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
  return totals;
}

/** One splits table (F-18 L-5): the split totals, formatted by lib/page's lapsGroup. NaN values
 *  reach lapsGroup as non-finite totals, which it treats as absent. A pool split's distance is in
 *  the pool's unit, header included (LS-6). */
function splitsTable(laps: Fmt[0], doc: Document, sport: string, totals: { totals: Totals }[], units: Units, preset: Preset, pool: Pool | null): HTMLTableElement {
  const group = laps(sport, totals, units);
  const dj = pool ? (group?.Columns.findIndex((c) => c.Key === 'distance') ?? -1) : -1;
  return table(
    doc,
    group?.Columns.map((c, j) => [c.Key, j === dj ? 'Distance (' + pool!.unit + ')' : c.Label] as const) ?? [],
    group?.Rows.map((row, i) => row.map((t, j) => (j === dj ? String(Math.round(poolDistance(totals[i].totals.distance!, preset, pool!))) : t))) ?? [],
  );
}

type View = 'Laps' | 'Splits' | 'Lengths';

/** The chunk's entry. `seams` are the laps enhancer's (LS-8): the rows of the shown splits or lengths
 *  table tap through its one listener; the chart mirrors its mark (F-51 SC-7). `splits`: the
 *  distance advances (F-18 L-6). */
export function startSplits(root: ParentNode, data: WorkoutData, units: UnitsStore, seams: Pick<LapsSeams, 'splits' | 'marked'>, fmt: Fmt, splits: boolean): void {
  const section = root.querySelector<HTMLElement>('section.laps');
  const slot = section?.querySelector('#laps-slot');
  if (!section || !slot || slot.firstChild) return;
  const doc = section.ownerDocument;
  const server = section.querySelector<HTMLElement>('.laps-scroll');
  const lengths = data.meta.lengths;
  const stats = splits ? createRangeStats(data) : null;
  const d = stats?.distance;
  // PL-8: one view per kind the workout has, in this order; the page opens on the first (LS-7, not remembered).
  const views = (['Laps', 'Splits', 'Lengths'] as const).filter((v) => (v === 'Laps' ? server : v === 'Splits' ? d : lengths.length > 0));
  if (views.length === 0) return;
  let current: View = views[0];
  const boxes: Partial<Record<View, HTMLElement>> = server ? { Laps: server } : {};
  const pairs: Partial<Record<View, number[]>> = {};
  const choice = d ? splitChoice(data.meta, d[d.length - 1] - d[0]) : null;
  let preset = choice?.presets[choice.index];
  const seam = seams.splits;
  // F-51 SC-4: no chart for a multisport workout (AP-12). Slots per view, from the rows its table is built from (SC-3).
  const chart = data.meta.sport === 'multisport' ? null : createChart(section, data.meta.sport, [fmt[3], fmt[4], fmt[5]], seams.marked);
  const slots: Partial<Record<View, Slot[]>> = {};
  // F-51 SC-6: net elevation per lap or split row (samples a … b inclusive), when the workout has altitude.
  const alt = data.metrics.altitude;
  const net = (a: number, b: number): { elevation?: number } => (alt ? { elevation: netElevation(alt, a, b) } : {});
  if (server) {
    const rows = [...server.querySelectorAll('tbody tr:not(.laps-section)')];
    const laps = data.meta.laps;
    slots.Laps = rows.length === laps.length ? rows.map((row, i) => ({ row, totals: laps[i].totals, label: String(i + 1), ...net(laps[i].start_index, laps[i].end_index - 1) })) : [];
  }
  const draw = (): void => chart?.draw(slots[current]!, current === 'Splits' ? preset!.label + ' splits' : current, units.get(), current.slice(0, -1));
  // LS-9, PL-8: built only on arrival, a change, a switch to it or a units change; the selection is untouched (LS-7).
  const build = (view: View): void => {
    const wrap = doc.createElement('div');
    wrap.className = view === 'Splits' ? 'laps-scroll splits' : 'laps-scroll splits lengths';
    wrap.hidden = view !== current;
    if (view === 'Splits') {
      const bounds = presetBounds(d!, preset!, choice!.pool);
      const totals = splitTotals(stats!, bounds);
      wrap.appendChild(splitsTable(fmt[0], doc, data.meta.sport, totals, units.get(), preset!, choice!.pool));
      pairs.Splits = bounds.slice(1).flatMap((b, i) => [bounds[i], b]);
      const partial = partialLast(d!, bounds, preset!, choice!.pool);
      const rows = wrap.querySelectorAll('tbody tr');
      slots.Splits = totals.map((t, i) => ({ row: rows[i], totals: t.totals, label: String(i + 1), partial: partial && i === totals.length - 1, ...net(bounds[i], bounds[i + 1]) }));
    } else {
      wrap.appendChild(lengthsTable(fmt, doc, lengths, units.get()));
      // L-2: a length selects start_index … end_index − 1, rest lengths too.
      pairs.Lengths = lengths.flatMap((l) => [l.start_index, l.end_index - 1]);
      // SC-4: rest lengths take no slot.
      const rows = wrap.querySelectorAll('tbody tr');
      slots.Lengths = lengthRows(lengths).flatMap((r, i) => (lengths[i].rest ? [] : [{ row: rows[i], totals: { avg_speed: lengths[i].avg_speed }, label: String(r.number) }]));
    }
    if (view === current) seam(wrap, pairs[view]!);
    const old = boxes[view];
    if (old) old.replaceWith(wrap);
    else section.insertBefore(wrap, section.querySelector('.compare')); // SC-9: the chart stays last
    boxes[view] = wrap;
  };
  const select = doc.createElement('select');
  const buttons: HTMLButtonElement[] = [];
  const show = (view: View): void => {
    current = view;
    const box = boxes[view];
    if (!box) build(view);
    else if (view !== 'Laps') seam(box, pairs[view]!);
    for (const v of views) if (boxes[v]) boxes[v].hidden = v !== view;
    select.hidden = view !== 'Splits';
    buttons.forEach((b) => b.setAttribute('aria-pressed', String(b.textContent === view)));
    draw();
  };
  if (views.length > 1) {
    const control = doc.createElement('span');
    control.className = 'laps-switch';
    for (const v of views) {
      const b = doc.createElement('button');
      b.type = 'button';
      b.textContent = v;
      b.addEventListener('click', () => show(v));
      buttons.push(b);
    }
    control.append(...buttons);
    slot.appendChild(control);
  }
  if (choice) {
    select.className = 'split-select';
    select.setAttribute('aria-label', 'Split distance');
    select.innerHTML = choice.presets.map((p, i) => `<option value="${i}">${p.label}</option>`).join('');
    select.value = String(choice.index);
    select.addEventListener('change', () => {
      preset = choice.presets[+select.value];
      build('Splits');
      draw();
    });
    slot.appendChild(select);
  }
  // Boundaries do not depend on units; the cells do (pace only, for lengths).
  units.subscribe(() => {
    if (boxes.Splits) build('Splits');
    if (boxes.Lengths) build('Lengths');
    draw();
  });
  // LS-3: no device laps: the module, headed by its first view, is revealed on it.
  if (!server) section.querySelector('h2')!.textContent = current;
  show(current);
  section.hidden = false;
}
