// Splits and lengths in the browser (F-46 LS-4 to LS-9, F-48 PL-8; AP-8, AP-13, AP-20). LAZY: reached
// only through the page glue's import(), after the charts' first paint, when the distance advances or
// the workout has lengths. Fills the laps heading slot (the Laps / Splits / Lengths switch when two
// or more exist, the split-distance drop-down in the splits view) and builds the splits and lengths
// tables on demand; on a page without laps it shows the first view at once and reveals the module.
// Under the tables, the comparison chart of the shown view's rows (F-51, ./chart). A multisport
// workout's splits and charts are per leg (F-64).

import './splits.css';
import type { Lap, Length, Totals, WorkoutData } from '../lib/datafile';
import type { formatElapsed, formatSpeedOrPace, isPaceSport, speedOrPaceUnit, Units } from '../lib/format';
import type { codeWord, lapsGroup } from '../lib/page';
import { hasStrokeCounts, lengthRows } from '../lib/lengths';
import { createRangeStats, emptyValues, type RangeStats } from '../lib/range';
import { partialLast, poolDistance, presetBounds, splitChoice, type Pool, type Preset, type SplitChoice } from '../lib/splits';
import { sectionRanges, sections } from '../lib/sections';
import type { UnitsStore } from '../lib/units';
import type { LapsSeams } from '../page/laps';
import { createChart, type Chart, type Slot } from './chart';
import { t } from './texts';
import { onLanguage } from '../lib/i18n';

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
function lengthsTable([lapsGroup, codeWord, formatElapsed, formatSpeedOrPace]: Fmt, doc: Document, lengths: readonly Length[], units: Units): HTMLTableElement {
  const counts = hasStrokeCounts(lengths);
  const lap = lapsGroup('swimming', [{ totals: { elapsed_time: 1, avg_speed: 1 }, stroke: 'freestyle' }], units)!.Columns;
  const head = (k: string): string => lap.find((c) => c.Key === k)!.Label;
  const heads: [string, string][] = [['number', '#'], ['stroke', head('stroke')], ['elapsed_time', head('elapsed_time')], ['avg_speed', head('avg_speed')]];
  if (counts) heads.push(['strokes', t('colStrokes')], ['swolf', t('colSwolf')]);
  const rows = lengthRows(lengths).map((r, i) => {
    const l = lengths[i];
    const time = formatElapsed(r.time); // NaN: '–'
    if (l.rest) return [t('rest'), '', time, '', ...(counts ? ['', ''] : [])];
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
    group?.Columns.map((c, j) => [c.Key, j === dj ? t('poolDistance', { u: pool!.unit }) : c.Label] as const) ?? [],
    group?.Rows.map((row, i) => row.map((t, j) => (j === dj ? String(Math.round(poolDistance(totals[i].totals.distance!, preset, pool!))) : t))) ?? [],
  );
}

type View = 'Laps' | 'Splits' | 'Lengths';
const viewName = (v: View): string => t(`view${v}`);

/** A comparison chart with the slots and caption it draws (F-51; one per sport leg table, F-64 LL-7). */
interface Drawn {
  readonly chart: Chart;
  slots: Slot[];
  caption(): string;
}

/** One block of a multisport splits view (F-64 LL-5): a section's window [a, b] (MS-6) and heading;
 *  a sport leg whose distance advances also its drop-down, table and chart. */
interface Leg {
  readonly sport: string;
  readonly a: number;
  readonly b: number;
  readonly block: HTMLElement;
  readonly name: Text;
  /** LL-6: the leg's list and default, made again on every build (preset names follow the language). */
  readonly pick: (() => SplitChoice) | null;
  readonly select: HTMLSelectElement | null;
  readonly note: HTMLElement | null;
  wrap?: HTMLElement;
  drawn?: Drawn;
  /** The heading's range, then each split's, flat. */
  pairs: number[];
}

/** The chunk's entry. `seams` are the laps enhancer's (LS-8): the rows of the shown splits or lengths
 *  table tap through its one listener; the charts mirror its mark (F-51 SC-7). `splits`: the
 *  distance advances (F-18 L-6). A multisport workout (F-42 MS-1) gets its laps and splits per leg
 *  (F-64 LL-4 to LL-7). */
export function startSplits(root: ParentNode, data: WorkoutData, units: UnitsStore, seams: Pick<LapsSeams, 'splits' | 'marked'>, fmt: Fmt, splits: boolean): void {
  const section = root.querySelector<HTMLElement>('section.laps');
  const slot = section?.querySelector('#laps-slot');
  if (!section || !slot || slot.firstChild) return;
  const doc = section.ownerDocument;
  const secs = sections(data.meta);
  const server = section.querySelector<HTMLElement>(secs ? '.laps-legs' : '.laps-scroll');
  const lengths = data.meta.lengths;
  const stats = splits ? createRangeStats(data) : null;
  const d = stats?.distance;
  // PL-8: one view per kind the workout has, in this order; the page opens on the first (LS-7, not remembered).
  const views = (['Laps', 'Splits', 'Lengths'] as const).filter((v) => (v === 'Laps' ? server : v === 'Splits' ? d : lengths.length > 0));
  if (views.length === 0) return;
  let current: View = views[0];
  const boxes: Partial<Record<View, HTMLElement>> = server ? { Laps: server } : {};
  const pairs: Partial<Record<View, number[]>> = {};
  // LL-5: a multisport workout has no workout-wide drop-down.
  let choice = d && !secs ? splitChoice(data.meta, d[d.length - 1] - d[0]) : null;
  let preset = choice?.presets[choice.index];
  const seam = seams.splits;
  // SC-7, LL-7: the enhancer's one mark notification, fanned out to every chart (a chart without the
  // row's bar clears its highlight).
  const marks: ((row: Element | null) => void)[] = [];
  let markedRow: Element | null = null;
  seams.marked((row) => {
    markedRow = row;
    for (const f of marks) f(row);
  });
  const newChart = (parent: HTMLElement, sport: string): Chart =>
    createChart(parent, sport, [fmt[3], fmt[4], fmt[5]], (f) => {
      marks.push(f);
      f(markedRow);
    });
  // SC-4: a single-sport workout has one chart, last in the section, redrawn per view; a multisport
  // one a chart per sport leg table (LL-7). Slots from the rows each table is built from (SC-3).
  const chart = secs ? null : newChart(section, data.meta.sport);
  const charts: Partial<Record<View, Drawn[]>> = {};
  // F-51 SC-6: elevation gain per lap or split row (samples a … b inclusive), as the range panel shows it (AP-7).
  const gs = data.metrics.altitude ? (stats ?? createRangeStats(data)) : null;
  const gv = emptyValues();
  const net = (a: number, b: number): { elevation?: number } => (gs?.fields.includes('gain') ? { elevation: gs.query(a, b, gv).gain } : {});
  const lapSlots = (rows: Element[], laps: readonly Lap[]): Slot[] =>
    rows.length === laps.length ? rows.map((row, i) => ({ row, totals: laps[i].totals, label: String(i + 1), ...net(laps[i].start_index, laps[i].end_index - 1) })) : [];
  const lapsCaption = (): string => viewName('Laps');
  if (server && chart) charts.Laps = [{ chart, slots: lapSlots([...server.querySelectorAll('tbody tr')], data.meta.laps), caption: lapsCaption }];
  if (server && secs) {
    // LL-2, LL-7: each leg block holds the next r laps (L-2's DOM order); no chart for transitions.
    let k = 0;
    charts.Laps = [...server.querySelectorAll<HTMLElement>('.laps-leg')].flatMap((block) => {
      const rows = [...block.querySelectorAll('tbody tr')];
      const laps = data.meta.laps.slice(k, (k += rows.length));
      const sport = block.dataset.sport ?? '';
      return sport === 'transition' || !rows.length ? [] : [{ chart: newChart(block, sport), slots: lapSlots(rows, laps), caption: lapsCaption }];
    });
  }
  const drawOne = (c: Drawn): void => c.chart.draw(c.slots, c.caption(), units.get(), current);
  const draw = (): void => charts[current]?.forEach(drawOne);

  // LL-4 to LL-6: the multisport splits view, one block per section in order, built on its first show.
  const ranges = secs ? sectionRanges(secs, data.sampleCount) : [];
  const sorted = [...data.meta.sessions].sort((x, y) => x.start - y.start); // MS-2's order
  let names = secs?.map((s) => s.name) ?? [];
  const out = emptyValues();
  const legs: Leg[] = (secs ?? []).map((s, k) => {
    const [a, b] = [ranges[2 * k], ranges[2 * k + 1]];
    const block = doc.createElement('div');
    block.className = 'laps-leg';
    block.dataset.sport = s.sport;
    const h3 = block.appendChild(doc.createElement('h3'));
    const name = h3.appendChild(doc.createTextNode(''));
    // A sport leg splits when its distance advances over its window; the pool rule only for a lap swim (PS-1).
    const advance = d && b > a ? d[b] - d[a] : 0;
    const summary = sorted[k]?.sub_sport === 'lap_swimming' ? data.meta.summary : {};
    const pick = s.sport !== 'transition' && advance > 0 ? () => splitChoice({ sport: s.sport, summary }, advance) : null;
    const select = pick ? h3.appendChild(doc.createElement('select')) : null;
    const note = !pick && s.sport !== 'transition' ? block.appendChild(doc.createElement('p')) : null;
    const leg: Leg = { sport: s.sport, a, b, block, name, pick, select, note, pairs: [] };
    if (note) note.className = 'no-splits';
    if (pick && select) {
      const c = pick();
      select.className = 'split-select';
      select.append(...c.presets.map((p, i) => new Option(p.label, String(i))));
      select.value = String(c.index);
      // LL-6: rebuilds this leg's table and chart only; the selection store is untouched.
      select.addEventListener('change', () => {
        buildLeg(leg, k);
        pairs.Splits = legs.flatMap((l) => l.pairs);
        seam(boxes.Splits!, pairs.Splits);
        drawOne(leg.drawn!);
      });
    }
    return leg;
  });
  /** One leg's heading text and, for a sport leg with splits, its table and chart (LL-5, LL-7). */
  const buildLeg = (leg: Leg, k: number): void => {
    const { a, b } = leg;
    // OQ-297: a transition reads "T1 · 1:12", the elapsed time of the window it selects.
    leg.name.data = leg.sport === 'transition' && b > a ? names[k] + ' · ' + fmt[2](stats!.query(a, b, out).elapsed) : names[k];
    if (leg.note) leg.note.textContent = t('noSplits');
    leg.pairs = [a, b];
    if (!leg.pick || !leg.select) return;
    const c = leg.pick();
    leg.select.setAttribute('aria-label', t('splitDistance'));
    c.presets.forEach((p, i) => (leg.select!.options[i].text = p.label));
    const p = c.presets[+leg.select.value];
    const bounds = presetBounds(d!, p, c.pool, a, b);
    const totals = splitTotals(stats!, bounds);
    const wrap = doc.createElement('div');
    wrap.className = 'laps-scroll splits';
    wrap.appendChild(splitsTable(fmt[0], doc, leg.sport, totals, units.get(), p, c.pool));
    if (leg.wrap) leg.wrap.replaceWith(wrap);
    else leg.block.appendChild(wrap);
    leg.wrap = wrap;
    leg.pairs.push(...bounds.slice(1).flatMap((x, i) => [bounds[i], x]));
    const partial = partialLast(d!, bounds, p, c.pool);
    const rows = wrap.querySelectorAll('tbody tr');
    const slots = totals.map((x, i) => ({ row: rows[i], totals: x.totals, label: String(i + 1), partial: partial && i === totals.length - 1, ...net(bounds[i], bounds[i + 1]) }));
    leg.drawn ??= { chart: newChart(leg.block, leg.sport), slots, caption: () => '' }; // SC-9: last in its block
    leg.drawn.slots = slots;
    leg.drawn.caption = () => t('splitsCaption', { preset: p.label });
  };

  // LS-9, PL-8: built only on arrival, a change, a switch to it or a units change; the selection is untouched (LS-7).
  const build = (view: View): void => {
    let wrap: HTMLElement;
    if (view === 'Splits' && secs) {
      legs.forEach(buildLeg);
      wrap = boxes.Splits ?? doc.createElement('div');
      if (!wrap.firstChild) {
        wrap.className = 'laps-legs splits';
        wrap.append(...legs.map((l) => l.block));
      }
      pairs.Splits = legs.flatMap((l) => l.pairs);
      charts.Splits = legs.flatMap((l) => (l.drawn ? [l.drawn] : []));
    } else if (view === 'Splits') {
      wrap = doc.createElement('div');
      wrap.className = 'laps-scroll splits';
      const bounds = presetBounds(d!, preset!, choice!.pool);
      const totals = splitTotals(stats!, bounds);
      wrap.appendChild(splitsTable(fmt[0], doc, data.meta.sport, totals, units.get(), preset!, choice!.pool));
      pairs.Splits = bounds.slice(1).flatMap((b, i) => [bounds[i], b]);
      const partial = partialLast(d!, bounds, preset!, choice!.pool);
      const rows = wrap.querySelectorAll('tbody tr');
      const slots = totals.map((t, i) => ({ row: rows[i], totals: t.totals, label: String(i + 1), partial: partial && i === totals.length - 1, ...net(bounds[i], bounds[i + 1]) }));
      charts.Splits = [{ chart: chart!, slots, caption: () => t('splitsCaption', { preset: preset!.label }) }];
    } else {
      wrap = doc.createElement('div');
      wrap.className = 'laps-scroll splits lengths';
      wrap.appendChild(lengthsTable(fmt, doc, lengths, units.get()));
      // L-2: a length selects start_index … end_index − 1, rest lengths too.
      pairs.Lengths = lengths.flatMap((l) => [l.start_index, l.end_index - 1]);
      // SC-4: rest lengths take no slot; no lengths chart for a multisport workout (LL-7: legs only).
      const rows = wrap.querySelectorAll('tbody tr');
      const slots = lengthRows(lengths).flatMap((r, i) => (lengths[i].rest ? [] : [{ row: rows[i], totals: { avg_speed: lengths[i].avg_speed }, label: String(r.number) }]));
      charts.Lengths = chart ? [{ chart, slots, caption: () => viewName('Lengths') }] : [];
    }
    wrap.hidden = view !== current;
    const old = boxes[view];
    if (old && old !== wrap) old.replaceWith(wrap);
    else if (!old) section.insertBefore(wrap, section.querySelector(':scope > .compare')); // SC-9: the chart stays last
    boxes[view] = wrap;
    // After the swap: the enhancer unmarks a row only when it left the page (LL-6).
    if (view === current) seam(wrap, pairs[view]!);
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
    buttons.forEach((b, i) => b.setAttribute('aria-pressed', String(views[i] === view)));
    draw();
  };
  if (views.length > 1) {
    const control = doc.createElement('span');
    control.className = 'laps-switch';
    for (const v of views) {
      const b = doc.createElement('button');
      b.type = 'button';
      b.textContent = viewName(v);
      b.addEventListener('click', () => show(v));
      buttons.push(b);
    }
    control.append(...buttons);
    slot.appendChild(control);
  }
  if (choice) {
    select.className = 'split-select';
    select.setAttribute('aria-label', t('splitDistance'));
    select.append(...choice.presets.map((p, i) => new Option(p.label, String(i))));
    select.value = String(choice.index);
    select.addEventListener('change', () => {
      preset = choice!.presets[+select.value];
      build('Splits');
      draw();
    });
    slot.appendChild(select);
  }
  // Boundaries do not depend on units; the cells do (pace only, for lengths). The language (F-54):
  // the cells' decimal separators and every label.
  const redraw = (): void => {
    if (boxes.Splits) build('Splits');
    if (boxes.Lengths) build('Lengths');
    draw();
  };
  units.subscribe(redraw);
  // LS-3: no device laps: the module, headed by its first view, is revealed on it.
  const heading = (): void => void (!server && (section.querySelector('h2')!.textContent = viewName(views[0])));
  onLanguage(() => {
    heading();
    buttons.forEach((b, i) => (b.textContent = viewName(views[i])));
    select.setAttribute('aria-label', t('splitDistance'));
    // Preset names follow the language ('Half marathon'): the choice is made again, the index kept.
    if (choice && d) {
      choice = splitChoice(data.meta, d[d.length - 1] - d[0]);
      choice.presets.forEach((p, i) => (select.options[i].text = p.label));
      preset = choice.presets[+select.value];
    }
    names = sections(data.meta)?.map((s) => s.name) ?? []; // leg names (sportName follows the language)
    redraw();
  });
  heading();
  show(current);
  section.hidden = false;
}
