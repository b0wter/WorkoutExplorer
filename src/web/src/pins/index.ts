// Map pins on the page (F-56 PN-9 to PN-11; PIN-1 to PIN-6, AP-15). LAZY: reached only through the
// page glue's import(), after the charts' first paint, when the page links a pins file (viewing
// page) or the owner edits pins (edit page). Owns the list, the map markers, legend and card, the
// chart marks and the one pin selection store; nothing here runs per cursor frame.
// PN-11: owner text reaches the DOM only through textContent and setAttribute, never as HTML.

import './pins.css';
import type { WorkoutData } from '../lib/datafile';
import type { CursorStore } from '../lib/cursor';
import type { SelectionStore } from '../lib/selection';
import type { UnitsStore } from '../lib/units';
import type { ChartDecorator } from '../components/charts/uplot-options';
import { distanceUnit, formatDistance } from '../lib/format';
import { PIN_TYPES, TYPE_NAMES, pinName, readPins, type PinType, type ShownPin } from '../lib/pins';

// The pin editor's helpers (owner chunk), re-exported: see src/edit/pins.ts.
export { MAX_PINS, PIN_NOTE_MAX, PIN_TITLE_MAX, PIN_TYPES, REMOVED, TYPE_NAMES, dimReason, fromApi, hasVisibleRoute, snap, sortPins, toApi } from '../lib/pins';
export type { ApiPin, PinType, Shade, ShownPin } from '../lib/pins';

/** The lazy map module's seam for pins (ADR-013): nothing pin-specific lives there. */
export interface PinMap {
  /** A DOM marker at sample i's position; returns its removal. */
  marker(el: HTMLElement, i: number): () => void;
  /** Section lines (inclusive sample ranges), above the route, below the range. */
  marks(ranges: readonly (readonly [number, number])[]): void;
  /** An element inside the map box (stays in fullscreen), bottom left. */
  control(el: HTMLElement): void;
  /** The route sample near a client point, or -1. */
  sampleAt(x: number, y: number): number;
}

export interface PinsEnv {
  readonly slot: HTMLElement;
  readonly data: WorkoutData;
  readonly cursor: CursorStore;
  readonly selection: SelectionStore;
  readonly units: UnitsStore;
  /** The charts' decoration seam (F-42 MS-7). */
  readonly look: { decorator(key: string, f: ChartDecorator): void };
  /** Calls back once with the map's seam, when (if ever) the map is ready. */
  map(cb: (m: PinMap) => void): void;
  /** The edit page's seam: it gets the view and no file is read; a new range never deselects a
   *  section there (its editor moves it, OQ-273). */
  readonly edit?: { onpins?(view: PinsView): void };
}

export interface PinsView {
  readonly slot: HTMLElement;
  readonly cursor: CursorStore;
  readonly selection: SelectionStore;
  /** Replaces the pins; `keep` stays selected (an index into the new list). */
  set(pins: readonly ShownPin[], keep?: number | null): void;
  /** The selected pin's index, or null. */
  selected(): number | null;
  /** Selects (null: none) without the toggle a tap has. */
  select(i: number | null): void;
  /** Called on every selection change. */
  onselect(fn: (i: number | null) => void): void;
  /** The map's route sample near a client point, or -1 (no map yet: -1). */
  sampleAt(x: number, y: number): number;
}

const SVG = 'http://www.w3.org/2000/svg';
/** Fixed glyphs, white on the type's colour (PIN-2); never mixed with owner text. */
const GLYPHS: Readonly<Record<PinType, string>> = {
  point_of_interest: 'M12 6l1.8 3.8 4.2.5-3.1 2.9.8 4.1L12 15.2l-3.7 2.1.8-4.1L6 10.3l4.2-.5z',
  danger: 'M12 6.5v7M12 16.5v1',
  difficult_section: 'M6 15l3-5 3 4 3-5 3 6',
  water_food: 'M12 6c3 3.8 4.5 6 4.5 8a4.5 4.5 0 0 1-9 0c0-2 1.5-4.2 4.5-8z',
  photo_spot: 'M6.5 9h3l1-2h3l1 2h3v8h-11zM14 13a2 2 0 1 1-4 0 2 2 0 0 1 4 0',
  note: 'M8 8.5h8M8 12h8M8 15.5h5',
};

export function icon(doc: Document, type: PinType): SVGSVGElement {
  const svg = doc.createElementNS(SVG, 'svg');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('class', 'pin-ic t-' + type);
  svg.setAttribute('aria-hidden', 'true');
  const c = svg.appendChild(doc.createElementNS(SVG, 'circle'));
  c.setAttribute('cx', '12');
  c.setAttribute('cy', '12');
  c.setAttribute('r', '11');
  svg.appendChild(doc.createElementNS(SVG, 'path')).setAttribute('d', GLYPHS[type]);
  return svg;
}

const el = <K extends keyof HTMLElementTagNameMap>(doc: Document, tag: K, cls: string, text = ''): HTMLElementTagNameMap[K] => {
  const e = doc.createElement(tag);
  e.className = cls;
  e.textContent = text;
  return e;
};

/** The chunk's entry. Viewing page: fetches the slot's pins file (PN-9); a failed or invalid file
 *  (PN-11) shows no pins and releases the slot. Edit page: hands the empty view to the edit seam. */
export function startPins(env: PinsEnv): PinsView {
  const { slot, data, cursor, selection, units } = env;
  const doc = slot.ownerDocument;
  let pins: readonly ShownPin[] = [];
  let sel: number | null = null;
  const off = new Set<PinType>(); // legend: memory only (OQ-239)
  const listeners: ((i: number | null) => void)[] = [];
  const dist = data.metrics.distance;

  const head = el(doc, 'h2', 'pins-head', 'Pins');
  const list = el(doc, 'ul', 'pin-list');
  slot.append(head, list);

  /** Distance from the start in the units setting; a section from–to (OQ-276). */
  const where = (p: ShownPin): string => {
    if (!dist) return '';
    const u = units.get();
    const d = (i: number): string => formatDistance(dist[i], u);
    return (p.to === undefined ? d(p.from) : d(p.from) + '–' + d(p.to)) + ' ' + distanceUnit(u);
  };

  /** Every element of one pin carries data-pin and its type class; selection and legend reach all views through them. */
  const tag = <E extends Element>(e: E, k: number): E => {
    e.setAttribute('data-pin', String(k));
    e.classList.add('t-' + pins[k].type);
    if (pins[k].dim) e.classList.add('dim');
    return e;
  };
  const all = (): NodeListOf<HTMLElement> => doc.querySelectorAll<HTMLElement>('[data-pin]');
  const paint = (): void => {
    for (const e of all()) {
      const k = Number(e.dataset.pin);
      e.classList.toggle('sel', k === sel);
      e.hidden = off.has(pins[k]?.type);
    }
    card.hidden = sel === null;
    if (sel !== null) {
      const p = pins[sel];
      cardName.textContent = pinName(p);
      cardWhere.textContent = where(p);
      cardNote.textContent = p.note;
    }
  };

  /** PN-10: a point moves the cursor; a section selects its range (AP-7), cursor at its start. */
  const select = (i: number | null): void => {
    sel = i;
    if (i !== null) {
      const p = pins[i];
      if (p.to !== undefined) selection.set(p.from, p.to);
      cursor.set(p.from);
    }
    paint();
    for (const f of listeners) f(i);
  };
  /** A tap: selecting the selected pin again clears it, and a section's range with it. */
  const tap = (i: number): void => {
    if (i !== sel) return select(i);
    const p = pins[i];
    if (p.to !== undefined && selection.start() === p.from && selection.end() === p.to) selection.clear();
    select(null);
  };
  // F-18 rows' rule: a range changed elsewhere deselects a section (viewing page).
  selection.subscribe((a, b) => {
    const p = sel === null ? null : pins[sel];
    if (!env.edit && p?.to !== undefined && (a !== p.from || b !== p.to)) select(null);
  });

  // ---- list (OQ-276) ----
  const renderList = (): void => {
    list.replaceChildren(
      ...pins.map((p, k) => {
        const li = tag(el(doc, 'li', 'pin-item'), k);
        const row = li.appendChild(el(doc, 'button', 'pin-row'));
        row.type = 'button';
        row.append(icon(doc, p.type), el(doc, 'span', 'pin-t', pinName(p)), el(doc, 'span', 'pin-d', p.dim || where(p)));
        row.addEventListener('click', () => tap(k));
        if (p.note) li.append(el(doc, 'p', 'pin-note', p.note));
        return li;
      }),
    );
  };
  units.subscribe(() => {
    renderList();
    paint();
  });

  // ---- charts: x-axis marks and section bands (MS-7's seam; layout only) ----
  const decorator: ChartDecorator = (over, x) => {
    over.querySelector('.pin-marks')?.remove();
    const box = el(doc, 'div', 'pin-marks');
    pins.forEach((p, k) => {
      const m = tag(box.appendChild(el(doc, 'div', p.to === undefined ? 'pin-x' : 'pin-band')), k);
      m.hidden = off.has(p.type);
      m.classList.toggle('sel', k === sel);
      m.style.left = x(p.from) + 'px';
      if (p.to !== undefined) m.style.width = x(p.to) - x(p.from) + 'px';
    });
    over.prepend(box);
  };

  // ---- map: markers, section lines, legend, fullscreen card (AP-15) ----
  const card = el(doc, 'div', 'maplibregl-ctrl pin-card');
  card.hidden = true;
  const close = card.appendChild(el(doc, 'button', 'pin-x-btn', '×'));
  const cardName = card.appendChild(el(doc, 'b', 'pin-t'));
  close.type = 'button';
  close.setAttribute('aria-label', 'Close');
  close.addEventListener('click', () => select(null));
  const cardWhere = card.appendChild(el(doc, 'div', 'pin-d'));
  const cardNote = card.appendChild(el(doc, 'p', 'pin-note'));
  const legend = el(doc, 'div', 'maplibregl-ctrl pin-legend');
  let map: PinMap | null = null;
  let removals: (() => void)[] = [];
  const renderMap = (): void => {
    for (const r of removals) r();
    removals = [];
    if (!map) return;
    const m = map;
    pins.forEach((p, k) => {
      const b = tag(el(doc, 'button', 'pin-mk'), k);
      b.type = 'button';
      b.setAttribute('aria-label', TYPE_NAMES[p.type] + ': ' + pinName(p));
      b.append(icon(doc, p.type));
      b.addEventListener('click', () => tap(k));
      removals.push(m.marker(b, p.from));
    });
    m.marks(pins.flatMap((p) => (p.to === undefined || p.dim ? [] : [[p.from, p.to] as const])));
    const types = PIN_TYPES.filter((t) => pins.some((p) => p.type === t));
    legend.hidden = types.length === 0;
    legend.replaceChildren(
      ...types.map((t) => {
        const b = el(doc, 'button', 'pin-key');
        b.type = 'button';
        b.title = TYPE_NAMES[t];
        b.setAttribute('aria-label', TYPE_NAMES[t]);
        b.setAttribute('aria-pressed', String(!off.has(t)));
        b.append(icon(doc, t));
        b.addEventListener('click', () => {
          if (off.has(t)) off.delete(t);
          else off.add(t);
          b.setAttribute('aria-pressed', String(!off.has(t)));
          // A hidden type leaves every view and drops its selection (PN-10).
          if (sel !== null && off.has(pins[sel].type)) select(null);
          else paint();
        });
        return b;
      }),
    );
  };
  env.map((m) => {
    map = m;
    m.control(legend);
    m.control(card);
    renderMap();
    paint();
  });

  const set = (next: readonly ShownPin[], keep: number | null = null): void => {
    pins = next;
    sel = keep;
    renderList();
    renderMap();
    env.look.decorator('pins', decorator.bind(null)); // a new function: the charts redraw
    paint();
  };
  set(pins);

  const view: PinsView = {
    slot,
    cursor,
    selection,
    set,
    selected: () => sel,
    select,
    onselect: (fn) => void listeners.push(fn),
    sampleAt: (x, y) => (map ? map.sampleAt(x, y) : -1),
  };
  if (env.edit) env.edit.onpins?.(view);
  else
    fetch(slot.dataset.url!)
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error('status ' + r.status))))
      .then((file: unknown) => {
        const read = readPins(file, data.sampleCount);
        if (!read) throw new Error('invalid pins file');
        set(read);
      })
      .catch((e: unknown) => {
        slot.hidden = true;
        console.warn('pins: not loaded', e);
      });
  return view;
}
