// Framework-free enhancers of server markup (F-13 design D-8, D-12, D-14; ADR-026 item 4). They
// attach to hook attributes, so they work on any markup that carries the hooks: the template's
// and the edit page's components'. Nothing here re-renders anything.

import type { WorkoutMeta } from '../lib/datafile';
import { formatTimeLeft, type Units } from '../lib/format';
import { lapsGroup, summaryItems } from '../lib/page';
import type { UnitsStore } from '../lib/units';
import type { ThemeChoice } from '../lib/prefs';

/** Rewrites the summary and laps text of the server markup (metric) into the store's units (F-14
 *  D-5, ADR-027): the page builders run over the data file's metadata and write strings into the
 *  existing nodes, matched by `data-k` (the view model's Key) and by position. Text only; no node
 *  is added, removed or replaced. A markup that does not match is left untouched and logged.
 *  Runs once for the store's current value when it is not metric, then on every change. */
export function enhanceUnits(root: ParentNode, meta: Pick<WorkoutMeta, 'sport' | 'summary' | 'laps'>, store: UnitsStore): void {
  const apply = (): void => {
    const units = store.get();
    const writes: [Node, string][] = [];
    const items = summaryItems(meta.sport, meta.summary, units);
    const cells = root.querySelectorAll('.summary > .metric');
    let ok = cells.length === items.length;
    items.forEach((item, i) => {
      const cell = cells[i];
      const num = cell?.querySelectorAll('.n');
      const unit = cell?.querySelectorAll('.u');
      const max = cell?.querySelectorAll('.max');
      if (num?.length !== 1 || cell.getAttribute('data-k') !== item.Key || unit.length !== (item.Unit ? 1 : 0) || max.length !== (item.Max ? 1 : 0)) {
        ok = false;
        return;
      }
      writes.push([num[0], item.Value]);
      if (item.Unit) writes.push([unit[0], item.Unit]);
      if (item.Max) writes.push([max[0], 'max ' + item.Max]);
    });
    const laps = lapsGroup(meta.sport, meta.laps, units);
    // The server table only, not the bundle's splits table (F-18 L-6).
    const heads = root.querySelectorAll('.laps-scroll:not(.splits) thead th');
    const rows = root.querySelectorAll('.laps-scroll:not(.splits) tbody tr');
    if (laps === null ? heads.length > 0 : heads.length !== laps.Columns.length || rows.length !== laps.Rows.length) ok = false;
    laps?.Columns.forEach((c, j) => {
      if (heads[j]?.getAttribute('data-k') === c.Key) writes.push([heads[j], c.Label]);
      else ok = false;
    });
    laps?.Rows.forEach((row, i) => {
      const tds = rows[i]?.children;
      if (tds?.length !== row.length) ok = false;
      else row.forEach((t, j) => writes.push([tds[j], t]));
    });
    if (!ok) {
      console.warn('page: summary or laps markup does not match the view model; units not applied');
      return;
    }
    for (const [node, t] of writes) node.textContent = t;
  };
  if (store.get() !== 'metric') apply();
  store.subscribe(apply);
}

/** The units drop-down (F-14 D-8): a native select in the header's reserved slot, created only
 *  once the data file is decoded. Never in the template. Wording: plain 'Metric' and 'Imperial'.
 *  `onchoose` hears only the viewer's own choices (DS-1: only those are remembered). */
export function addUnitsSelect(root: ParentNode, store: UnitsStore, onchoose?: (units: Units) => void): HTMLSelectElement | null {
  return addSelect(root, '#units-slot', 'Units', ['Metric', 'Imperial'], store.get(), (v) => {
    const units: Units = v === 'imperial' ? 'imperial' : 'metric';
    store.set(units);
    onchoose?.(units);
  });
}

/** The theme drop-down (DS-4, user 3 Nov 2026), beside the units one; System by default. */
export function addThemeSelect(root: ParentNode, value: ThemeChoice, onchoose: (theme: ThemeChoice) => void): HTMLSelectElement | null {
  return addSelect(root, '#theme-slot', 'Theme', ['System', 'Light', 'Dark'], value, (v) => onchoose(v as ThemeChoice));
}

/** A native select in an empty reserved header slot (D-8); the option values are the labels in lower case. */
function addSelect(root: ParentNode, slotQuery: string, label: string, labels: string[], value: string, onchange: (value: string) => void): HTMLSelectElement | null {
  const slot = root.querySelector(slotQuery);
  if (!slot || slot.firstChild) return null;
  const select = slot.ownerDocument.createElement('select');
  select.setAttribute('aria-label', label);
  select.innerHTML = labels.map((l) => `<option value="${l.toLowerCase()}">${l}</option>`).join('');
  select.value = value;
  select.addEventListener('change', () => onchange(select.value));
  slot.appendChild(select);
  return select;
}

/** "Expires around 30 Oct 2026" (the server's estimate) becomes "Expires in 6 days", counted from
 *  the UTC expiry time (UP-7): the same text for every viewer, whatever their time zone. */
export function enhanceExpiry(root: ParentNode, nowMs: number): void {
  const el = root.querySelector('time[data-expires]');
  if (!el) return;
  const at = Date.parse(el.getAttribute('datetime') ?? '');
  if (Number.isNaN(at)) return; // keep the server's text
  el.textContent = formatTimeLeft(at, nowMs);
}

export const COPY_TEXT = 'Copy link';
export const COPIED_TEXT = 'Link copied';

/** The viewing URL to copy: the page's origin plus the link's root-relative path. Never
 *  `location.href` and never the fragment, so an edit token cannot get into the copy (SH-6, AC 6). */
export function viewingUrl(origin: string, href: string | null): string | null {
  if (href === null || !href.startsWith('/') || href.startsWith('//')) return null;
  return origin + href.split(/[?#]/, 1)[0];
}

/** Copy-link button (SH-6). Without the Clipboard API (a non-secure context) or when it refuses,
 *  the URL appears in a read-only, pre-selected field next to the button. No execCommand. */
export function enhanceCopyLink(root: ParentNode, origin: string, nav: { clipboard?: { writeText(t: string): Promise<void> } }): void {
  const link = root.querySelector<HTMLAnchorElement>('a[data-copy-link]');
  if (!link) return;
  const url = viewingUrl(origin, link.getAttribute('href'));
  if (url === null) return;
  let timer = 0;
  const showField = (): void => {
    const head = link.closest('header') ?? link.parentElement;
    let field = head?.querySelector<HTMLInputElement>('input.copy-url') ?? null;
    if (!field) {
      field = link.ownerDocument.createElement('input');
      field.className = 'copy-url';
      field.readOnly = true;
      field.value = url;
      head?.appendChild(field);
    }
    field.focus();
    field.select();
  };
  link.addEventListener('click', (event) => {
    event.preventDefault();
    const write = nav.clipboard ? nav.clipboard.writeText(url) : Promise.reject(new Error('no clipboard'));
    write.then(
      () => {
        link.textContent = COPIED_TEXT;
        clearTimeout(timer);
        timer = window.setTimeout(() => {
          link.textContent = COPY_TEXT;
        }, 2000);
      },
      showField,
    );
  });
}

/** The Report button (F-28 RF-1, RF-2, RF-5): shown only with JavaScript and a valid public ID,
 *  taken from the copy link's view path, never `location`. The form is a lazy chunk (ADR-011). */
export function enhanceReport(root: ParentNode, load = () => import('../report/form')): void {
  const button = root.querySelector<HTMLButtonElement>('button[data-report]');
  const id = root.querySelector('a[data-copy-link]')?.getAttribute('href')?.split('/').pop();
  if (!button || !id || !/^[0-9A-Za-z]{22}$/.test(id)) return;
  button.hidden = false;
  button.addEventListener('click', () => {
    load().then(
      (m) => m.openReport(button.ownerDocument, id),
      () => {
        if (button.nextElementSibling?.className === 'report-fail') return;
        const p = button.ownerDocument.createElement('span');
        p.className = 'report-fail';
        p.textContent = 'The report form could not be loaded. Reload the page and try again.';
        button.after(p);
      },
    );
  });
}

const PIN_LABEL = 'Pin map';
const PIN_SVG =
  '<svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><path fill="currentColor" d="M16 3H8v2l1 .5V11l-3 3v2h5v5l1 1 1-1v-5h5v-2l-3-3V5.5L16 5z"/></svg>';

/** The phone's pin toggle (F-13 design D-19, AP-18). Bundle-only: created when the map is ready,
 *  so it never exists without JavaScript, before the map or after a failed map. An icon button in
 *  MapLibre's top-right stack under fullscreen (own control group); without that stack (stubbed
 *  map) it is a plain child of #map-slot. It flips one class on the layout wrapper and
 *  aria-pressed; held in memory only. CSS shows it and applies the class below 1024 px only. */
export function addPinToggle(root: ParentNode, slot: HTMLElement): HTMLButtonElement | null {
  const layout = root.querySelector('.layout');
  if (!layout || slot.querySelector('.map-pin')) return null;
  const doc = slot.ownerDocument;
  const group = doc.createElement('div');
  group.className = 'maplibregl-ctrl maplibregl-ctrl-group map-pin-group';
  const button = doc.createElement('button');
  button.type = 'button';
  button.className = 'map-pin';
  button.title = PIN_LABEL;
  button.setAttribute('aria-label', PIN_LABEL);
  button.setAttribute('aria-pressed', 'false');
  button.innerHTML = PIN_SVG;
  button.addEventListener('click', () => {
    const on = layout.classList.toggle('pinned');
    button.setAttribute('aria-pressed', String(on));
  });
  group.appendChild(button);
  (slot.querySelector('.maplibregl-ctrl-top-right') ?? slot).appendChild(group);
  return button;
}
