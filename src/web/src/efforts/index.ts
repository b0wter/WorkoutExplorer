// The Best efforts section (F-50 EF-7 to EF-9; AP-11). LAZY: reached only through the page glue's
// import(), after the charts' first paint; on the edit page through the edit glue, which passes its
// public window (the saved trim, EF-1). Fills #efforts-slot: the tables first, then the power curve
// computed in slices of about 4 ms. Never recomputed on a cursor or selection flush: a selection
// flush only marks rows, a units change rewrites the pace texts.

import './efforts.css';
import type { WorkoutData } from '../lib/datafile';
import type { SelectionStore } from '../lib/selection';
import type { UnitsStore } from '../lib/units';
import { formatElapsed, formatPace, paceSeconds, speedOrPaceUnit, type Units } from '../lib/format';
import { selectRow } from '../page/laps';
import { t } from './texts';
import { onLanguage } from '../lib/i18n';
import { bestPower, effortName, curveDurations, distanceEfforts, effortLegs, longest, powerEfforts, powerLegs, DURATIONS, type Effort, type PowerLeg } from '../lib/efforts';

/** The public window (EF-1): inclusive sample indices. */
export type EffortWindow = () => readonly [number, number];

const SLICE_MS = 4;
const PLOT = 120; // px
const SVG = 'http://www.w3.org/2000/svg';

const paceText = (e: Effort, units: Units): string => formatPace(paceSeconds(e.sport, units, e.metres / e.value)) + ' ' + speedOrPaceUnit(e.sport, units);

/** The chunk's entry. Returns `refresh`, for the edit glue after a save: recomputes when the window changed. */
export function startEfforts(root: ParentNode, data: WorkoutData, selection: SelectionStore, units: UnitsStore, win: Window, src?: EffortWindow): () => void {
  const slot = root.querySelector<HTMLElement>('#efforts-slot');
  if (!slot) return () => {};
  const pub: EffortWindow = src ?? (() => [0, data.sampleCount - 1]);
  const doc = slot.ownerDocument;
  const el = (parent: Element, tag: string, cls = '', text = ''): HTMLElement => {
    const e = parent.appendChild(doc.createElement(tag));
    if (cls) e.className = cls;
    if (text) e.textContent = text;
    return e;
  };
  let rows: [HTMLElement, Effort][] = [];
  let paces: [HTMLElement, Effort][] = [];
  let shown = '';
  let run = 0;

  const markRows = (a: number, b: number): void => {
    for (const [r, e] of rows) r.classList.toggle('on', e.a === a && e.b === b);
  };

  const table = (section: Element, heads: readonly string[], efforts: readonly Effort[], cells: (e: Effort, tr: HTMLElement) => void): void => {
    if (!efforts.length) return;
    const tb = el(section, 'table');
    const hr = el(el(tb, 'thead'), 'tr');
    for (const h of heads) el(hr, 'th', '', h);
    const body = el(tb, 'tbody');
    for (const e of efforts) {
      const tr = el(body, 'tr');
      el(tr, 'td', '', effortName(e.name));
      cells(e, tr);
      el(tr, 'td', '', formatElapsed(e.start));
      rows.push([tr, e]);
    }
  };

  /** EF-9: log x from 1 s to the longest leg, linear y from 0 W; geometry in a fixed viewBox. */
  const drawCurve = (box: HTMLElement, ds: readonly number[], ws: Float64Array): void => {
    const max = ds[ds.length - 1];
    const lx = (d: number): number => Math.log(d) / Math.log(max);
    const step = [25, 50, 100, 200, 250, 500, 1000].find((s) => ws[0] / s <= 4) ?? 2000;
    const top = Math.max(step, Math.ceil(ws[0] / step) * step);
    const y = (w: number): number => 4 + (1 - w / top) * (PLOT - 4);
    let svg = '';
    for (let w = step; w <= top; w += step) svg += `<line class="grid" x1="0" x2="100%" y1="${y(w)}" y2="${y(w)}"/><text x="0" y="${y(w) - 3}">${w} W</text>`;
    for (const [d, name] of DURATIONS) {
      if (d > max) continue;
      const x = lx(d) * 100;
      svg += `<line class="grid" x1="${x}%" x2="${x}%" y1="4" y2="${PLOT}"/><text${x > 92 ? ' class="end"' : ''} x="${x}%" y="${PLOT + 14}">${name}</text>`;
    }
    let pts = '';
    ds.forEach((d, i) => (pts += `${Math.round(lx(d) * 10000) / 10} ${Math.round((1 - ws[i] / top) * 1000) / 10} `));
    svg += `<svg y="4" width="100%" height="${PLOT - 4}" viewBox="0 0 1000 100" preserveAspectRatio="none"><polyline points="${pts}"/></svg>`;
    const plot = doc.createElementNS(SVG, 'svg');
    plot.setAttribute('class', 'curve');
    plot.setAttribute('height', String(PLOT + 18));
    plot.setAttribute('role', 'img');
    plot.setAttribute('aria-label', t('curveLabel', { first: Math.round(ws[0]), last: formatElapsed(max), lastW: Math.round(ws[ws.length - 1]) }));
    plot.innerHTML = svg;
    box.appendChild(plot);
  };

  /** EF-7: the curve in slices of about 4 ms, yielding between them; a newer run stops an older one. */
  const curve = (box: HTMLElement, legs: PowerLeg[], id: number, t0: number): void => {
    const ds = curveDurations(longest(legs));
    if (ds.length < 2) return;
    const ws = new Float64Array(ds.length);
    let i = 0;
    const slice = (): void => {
      if (id !== run) return;
      const end = performance.now() + SLICE_MS;
      while (i < ds.length && (i === 0 || performance.now() < end)) ws[i] = bestPower(legs, ds[i++])![0];
      if (i < ds.length) return void win.setTimeout(slice);
      el(box, 'h3', '', t('powerCurve'));
      drawCurve(box, ds, ws);
      performance.measure('wx-efforts:curve', { start: t0 }); // EF-7: from the section's insertion
    };
    win.setTimeout(slice);
  };

  const build = (): void => {
    const [lo, hi] = pub();
    if (shown === (shown = lo + ',' + hi)) return;
    const t0 = performance.now();
    const id = ++run;
    rows = [];
    paces = [];
    slot.replaceChildren();
    const legs = effortLegs(data, lo, hi);
    const dist = distanceEfforts(data, legs);
    const pl = powerLegs(data, legs);
    const power = powerEfforts(data, pl);
    slot.hidden = !dist.length && !power.length;
    if (slot.hidden) return;
    const section = el(slot, 'section', 'efforts');
    el(section, 'h2', '', t('bestEfforts'));
    const u = units.get();
    table(section, [t('colEffortDistance'), t('colEffortTime'), t('colEffortPace'), t('colEffortStart')], dist, (e, tr) => {
      el(tr, 'td', '', formatElapsed(Math.round(e.value)));
      paces.push([el(tr, 'td', '', paceText(e, u)), e]);
    });
    table(section, [t('colEffortDuration'), t('colEffortPower'), t('colEffortStart')], power, (e, tr) => void el(tr, 'td', '', Math.round(e.value) + ' W'));
    markRows(selection.start(), selection.end());
    curve(section, pl, id, t0);
  };

  // EF-8: a row tap selects the effort's covering range (L-3, L-7); the mark follows the store.
  slot.addEventListener('click', (event) => {
    const tr = (event.target as Element).closest('tbody tr');
    const hit = rows.find(([r]) => r === tr);
    if (hit) selectRow(hit[0], hit[1].a, hit[1].b, selection, win);
  });
  selection.subscribe(markRows);
  units.subscribe((u) => {
    for (const [td, e] of paces) td.textContent = paceText(e, u);
  });
  // F-54: a language change rebuilds the section (labels and decimal separators).
  onLanguage(() => {
    shown = '';
    build();
  });
  build();
  return build;
}
