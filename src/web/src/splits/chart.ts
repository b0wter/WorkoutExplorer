// The lap and split comparison chart (F-51 SC-1, SC-6 to SC-10; AP-23). Part of the lazy splits
// chunk: plain SVG under the laps and splits table, built from the shown view's rows only (on a view
// switch, a split-distance, units or overlay change), never on a cursor flush. Rules in lib/compare.

import '../theme/zones.css';
import type { formatSpeedOrPace, isPaceSport, speedOrPaceUnit, Units } from '../lib/format';
import { compare, overlay, type CompareRow, type OverlayKey } from '../lib/compare';

/** One slot: its table row (taps and hover go through it) and the number the table shows. */
export interface Slot extends CompareRow {
  readonly row: Element;
  readonly label: string;
}

export interface Chart {
  /** `noun`: a row's name in the accessible label ("Lap", "Split", "Length"). */
  draw(slots: readonly Slot[], caption: string, units: Units, noun: string): void;
}

/** The entry's formatters (see ../splits Fmt). */
export type Fmt = readonly [typeof formatSpeedOrPace, typeof speedOrPaceUnit, typeof isPaceSport];

const PLOT = 110; // px, plus the row numbers below: about 190 px with header, gaps and legend
const PAD = 4;
const NAMES: Readonly<Record<OverlayKey, readonly [label: string, colour: string]>> = {
  avg_heart_rate: ['Heart rate', 'heart_rate'],
  avg_power: ['Power', 'power'],
  avg_cadence: ['Cadence', 'cadence'],
  elevation: ['Elevation', 'altitude'],
};

const yp = (f: number): number => PAD + f * (PLOT - 2 * PAD);
const r2 = (x: number): number => Math.round(x * 100) / 100;

function el<K extends keyof HTMLElementTagNameMap>(doc: Document, tag: K, cls: string, text = ''): HTMLElementTagNameMap[K] {
  const e = doc.createElement(tag);
  e.className = cls;
  e.textContent = text;
  return e;
}

/** The chart box, last in the laps section (SC-9). `marked` is the laps enhancer's mark notification (SC-7). */
export function createChart(
  section: HTMLElement,
  sport: string,
  [speedText, unitText, paceSport]: Fmt,
  marked: (f: (row: Element | null) => void) => void,
): Chart {
  const doc = section.ownerDocument;
  const box = el(doc, 'div', 'compare');
  box.hidden = true;
  // The header persists across draws, so the focused overlay drop-down keeps its focus (QA-51-r1-2).
  const head = el(doc, 'div', 'compare-head');
  // Median and best as two stat blocks (label over value) that never wrap into the plot.
  const headText = el(doc, 'span', 'compare-stats');
  const label = el(doc, 'label', 'compare-overlay', 'Overlay ');
  const select = el(doc, 'select', 'split-select');
  const body = el(doc, 'div', '');
  label.appendChild(select);
  head.append(headText, label);
  box.append(head, body);
  section.appendChild(box);
  let offered = '';
  const pace = paceSport(sport);
  let choice: OverlayKey | '' = ''; // SC-6: per visit, in memory only
  let markedRow: Element | null = null;
  let barOf = new Map<Element, Element>();
  let selBar: Element | undefined;
  let last: Parameters<Chart['draw']> | null = null;

  const highlight = (): void => {
    selBar?.classList.remove('sel');
    selBar = markedRow ? barOf.get(markedRow) : undefined;
    selBar?.classList.add('sel');
  };
  marked((row) => {
    markedRow = row;
    highlight();
  });

  select.addEventListener('change', () => {
    choice = select.value as OverlayKey | '';
    if (last) draw(...last);
  });

  const draw = (slots: readonly Slot[], caption: string, units: Units, noun: string): void => {
    last = [slots, caption, units, noun];
    const c = compare(slots, pace);
    box.hidden = !c;
    barOf = new Map();
    selBar = undefined;
    if (!c) return void body.replaceChildren();
    if (choice && !c.overlays.includes(choice)) choice = '';
    const unit = ' ' + unitText(sport, units);
    const text = (v: number): string => speedText(sport, v, units);

    // Header: reference and best, and the overlay drop-down when there is a metric to offer.
    const stats: [string, string][] = [['Median', text(c.reference) + unit], ['Best', text(c.best) + unit]];
    headText.replaceChildren(
      ...stats.map(([k, v]) => {
        const s = el(doc, 'span', 'compare-stat', ' ' + v);
        s.prepend(el(doc, 'small', '', k));
        return s;
      }),
    );
    const headLabel = stats.map((kv) => kv.join(' ')).join(' · ');
    label.hidden = !c.overlays.length;
    if (offered !== (offered = c.overlays.join())) {
      select.replaceChildren();
      for (const [v, t] of [['', 'None'], ...c.overlays.map((k) => [k, NAMES[k][0]])]) select.add(new Option(t, v));
    }
    select.value = choice;

    // The plot: the band, the reference line, one bar per slot with a speed, the row numbers, the overlay.
    const n = slots.length;
    const w = 100 / n;
    const bw = Math.min(0.7 * w, 6);
    const step = [1, 2, 5, 10, 20, 50].find((s) => Math.ceil(n / s) <= 12)!;
    let svg = `<rect class="band" x="0" width="100%" y="${r2(yp(c.band[0]))}" height="${r2(yp(c.band[1]) - yp(c.band[0]))}"/><line class="ref" x1="0" x2="100%" y1="${yp(0.5)}" y2="${yp(0.5)}"/>`;
    c.bars.forEach((b, i) => {
      const mid = (i + 0.5) * w;
      if (b) {
        let top = yp(b.y0);
        let bot = yp(b.y1);
        if (bot - top < 2) (b.y0 < 0.5 ? (top = bot - 2) : (bot = top + 2));
        svg += `<rect class="bar ${b.cls}" data-i="${i}" x="${r2(mid - bw / 2)}%" width="${r2(bw)}%" y="${r2(top)}" height="${r2(bot - top)}"/>`;
        // SC-11: a clipped bar ends at the plot edge with a slanted break near its end (a shape, not a colour).
        if (b.clipped) {
          const e = b.y0 < 0.5 ? top + 8 : bot - 12;
          svg += `<line class="cut" x1="${r2(mid - bw / 2 - 0.3)}%" x2="${r2(mid + bw / 2 + 0.3)}%" y1="${r2(e + 4)}" y2="${r2(e)}"/>`;
        }
      }
      if ((i + 1) % step === 0 || i === 0) svg += `<text x="${r2(mid)}%" y="${PLOT + 15}">${slots[i].label}</text>`;
    });
    const ov = choice ? overlay(slots, c, choice) : null;
    if (ov) {
      let g = '';
      ov.ys.forEach((y, i) => {
        if (y !== y) return;
        const next = ov.ys[i + 1];
        if (next === next && i + 1 < n) g += `<line x1="${r2((i + 0.5) * w)}%" x2="${r2((i + 1.5) * w)}%" y1="${r2(yp(y))}" y2="${r2(yp(next))}"/>`;
        g += `<circle cx="${r2((i + 0.5) * w)}%" cy="${r2(yp(y))}" r="2.5"/>`;
      });
      svg += `<g class="ov ov-${NAMES[choice as OverlayKey][1]}">${g}</g>`;
    }
    const plot = doc.createElementNS('http://www.w3.org/2000/svg', 'svg');
    plot.setAttribute('height', String(PLOT + 18));
    plot.setAttribute('role', 'img');
    const beyond = slots.flatMap((s, i) => (c.bars[i]?.clipped ? [noun + ' ' + s.label + ' beyond the scale'] : []));
    plot.setAttribute('aria-label', [headLabel + ', ' + caption, ...beyond].join('. '));
    plot.innerHTML = svg;
    plot.querySelectorAll<SVGElement>('rect[data-i]').forEach((r) => barOf.set(slots[+r.dataset.i!].row, r));

    // Taps and hover: the slot under the pointer; a tap goes through the row's tap path (SC-7).
    const slotAt = (e: MouseEvent): Slot | undefined => {
      const r = plot.getBoundingClientRect();
      const s = slots[Math.floor(((e.clientX - r.left) / r.width) * n)];
      return s && barOf.has(s.row) ? s : undefined;
    };
    let hover: Slot | undefined;
    const setHover = (s: Slot | undefined): void => {
      if (s === hover) return;
      for (const h of hover ? [hover.row, barOf.get(hover.row)!] : []) h.classList.remove('hover');
      hover = s;
      for (const h of s ? [s.row, barOf.get(s.row)!] : []) h.classList.add('hover');
    };
    plot.addEventListener('click', (e) => (slotAt(e)?.row as HTMLElement | undefined)?.click());
    plot.addEventListener('pointermove', (e) => e.pointerType === 'mouse' && setHover(slotAt(e)));
    plot.addEventListener('pointerleave', () => setHover(undefined));

    // Elevation in m or ft with its unit (inlined, not lib/format: that would grow the entry).
    const ovText = (v: number): string =>
      choice === 'elevation' ? Math.round(units === 'metric' ? v : v / 0.3048) + (units === 'metric' ? ' m' : ' ft') : String(Math.round(v));
    const axis = (cls: string, texts: readonly string[]): HTMLElement => {
      const a = el(doc, 'div', cls);
      a.append(...texts.map((t) => el(doc, 'span', '', t)));
      return a;
    };
    const values = el(doc, 'div', 'compare-plot');
    values.append(axis('compare-axis', [text(c.top), text(c.reference), text(c.bottom)]), plot, axis('compare-axis right', ov ? [ovText(ov.max), '', ovText(ov.min)] : []));

    const foot = el(doc, 'div', 'compare-foot', caption);
    const legend = el(doc, 'span', 'compare-legend');
    for (const [cls, t] of [['faster', 'Faster'], ['pace', 'On pace'], ['slower', 'Slower']]) {
      const k = el(doc, 'span', '', t);
      k.prepend(el(doc, 'i', cls));
      legend.appendChild(k);
    }
    foot.appendChild(legend);
    body.replaceChildren(values, foot);
    // The old rows lose a hover they had; the marked row's bar is highlighted again.
    section.querySelectorAll('tr.hover').forEach((r) => r.classList.remove('hover'));
    highlight();
  };
  return { draw };
}
