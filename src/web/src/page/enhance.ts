// Framework-free enhancers of server markup (F-13 design D-8, D-12, D-14; ADR-026 item 4). They
// attach to hook attributes, so they work on any markup that carries the hooks: the template's
// and the edit page's components'. Nothing here re-renders anything.

import type { WorkoutMeta } from '../lib/datafile';
import { formatTimeLeft, type Units } from '../lib/format';
import { lapsGroup, summaryItems } from '../lib/page';
import type { UnitsStore } from '../lib/units';
import type { ThemeChoice } from '../lib/prefs';
import type { Facts } from '../summary-info';
import type { GpxInput } from '../lib/gpx';
import { LANGS, LANG_NAMES, lang, loadGroup, onLanguage, type Lang } from '../lib/i18n';
import { t as tp } from '../lib/page/texts';
import { en, t } from './texts';

/** Rewrites the summary and laps text of the server markup (metric) into the store's units (F-14
 *  D-5, ADR-027): the page builders run over the data file's metadata and write strings into the
 *  existing nodes, matched by `data-k` (the view model's Key) and by position. Text only; no node
 *  is added, removed or replaced. A markup that does not match is left untouched and logged.
 *  Runs once for the store's current value when it is not metric, then on every change. */
export function enhanceUnits(root: ParentNode, meta: WorkoutMeta, store: UnitsStore): void {
  const apply = (): void => {
    const units = store.get();
    const writes: [Node, string][] = [];
    // F-54 LG-4: the viewing page's header, device line and leg rows from the metadata (the edit
    // page renders its own); a lazy chunk, loaded once the page is not English.
    if (lang() !== 'en' || headerChunk) void loadHeader().then((m) => m.header(root, meta));
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
      const k = cell.querySelector('.k');
      if (k) writes.push([k, item.Label]);
      if (item.Unit) writes.push([unit[0], item.Unit]);
      if (item.Max) writes.push([max[0], tp('max', { v: item.Max })]);
    });
    // The server tables only, not the bundle's splits (F-18 L-6). F-64 LL-3: a multisport page has
    // one table per leg block, built for the block's sport from its next r laps (a leg name has no unit).
    const blocks = [...root.querySelectorAll('.laps-legs:not(.splits) > .laps-leg')];
    let k = 0;
    const tables = blocks.length
      ? blocks.map((b) => [b.getAttribute('data-sport') ?? '', b, meta.laps.slice(k, (k += b.querySelectorAll('tbody tr').length))] as const)
      : [[meta.sport, root, meta.laps] as const];
    if (blocks.length && k !== meta.laps.length) ok = false;
    for (const [sport, box, ls] of tables) {
      const laps = lapsGroup(sport, ls, units);
      const heads = box.querySelectorAll('.laps-scroll:not(.splits) thead th');
      const rows = box.querySelectorAll('.laps-scroll:not(.splits) tbody tr');
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
    }
    if (!ok) {
      console.warn('page: summary or laps markup does not match the view model; units not applied');
      return;
    }
    for (const [node, t] of writes) node.textContent = t;
  };
  if (store.get() !== 'metric' || lang() !== 'en') apply();
  store.subscribe(apply);
  onLanguage(apply); // F-54 LG-4: labels, words and decimal separators
}

let headerChunk: Promise<typeof import('./header')> | null = null;
/** F-54: the viewing page's language pass for the header (dates, sport names, device line). */
export const loadHeader = (): Promise<typeof import('./header')> => (headerChunk ??= import('./header'));

/** The units drop-down (F-14 D-8): a native select in the header's reserved slot, created only
 *  once the data file is decoded. Never in the template. Wording: plain 'Metric' and 'Imperial'.
 *  `onchoose` hears only the viewer's own choices (DS-1: only those are remembered). */
export function addUnitsSelect(root: ParentNode, store: UnitsStore, onchoose?: (units: Units) => void): HTMLSelectElement | null {
  return addSelect(root, '#units-slot', 'units', ['metric', 'imperial'], store.get(), (v) => {
    const units: Units = v === 'imperial' ? 'imperial' : 'metric';
    store.set(units);
    onchoose?.(units);
  });
}

/** The theme drop-down (DS-4, user 3 Nov 2026), beside the units one; System by default. */
export function addThemeSelect(root: ParentNode, value: ThemeChoice, onchoose: (theme: ThemeChoice) => void): HTMLSelectElement | null {
  return addSelect(root, '#theme-slot', 'theme', ['system', 'light', 'dark'], value, (v) => onchoose(v as ThemeChoice));
}

/** The language drop-down (F-54 LG-1, OQ-280), beside units and theme: each language named in itself. */
export function addLangSelect(root: ParentNode, onchoose: (l: Lang) => void): HTMLSelectElement | null {
  const select = addSelect(root, '#lang-slot', 'language', [], lang(), (v) => onchoose(v as Lang));
  select?.append(...LANGS.map((l, i) => new Option(LANG_NAMES[i], l)));
  if (select) {
    select.value = lang();
    // The page language is applied after this select exists (async file load): follow it (r: showed English on a German page).
    onLanguage(() => (select.value = lang()));
  }
  return select;
}

/** A native select in an empty reserved header slot (D-8): option values are the keys, labels their texts. */
function addSelect(root: ParentNode, slotQuery: string, label: keyof typeof en, keys: readonly (keyof typeof en)[], value: string, onchange: (value: string) => void): HTMLSelectElement | null {
  const slot = root.querySelector(slotQuery);
  if (!slot || slot.firstChild) return null;
  const select = slot.ownerDocument.createElement('select');
  const name = (): void => {
    select.setAttribute('aria-label', t(label));
    keys.forEach((k, i) => (select.options[i].text = t(k)));
  };
  select.append(...keys.map((k) => new Option('', k)));
  name();
  onLanguage(name);
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

/** The viewing URL to copy: the page's origin plus the link's root-relative path. Never
 *  `location.href` and never the fragment, so an edit token cannot get into the copy (SH-6, AC 6). */
export function viewingUrl(origin: string, href: string | null): string | null {
  if (href === null || !href.startsWith('/') || href.startsWith('//')) return null;
  return origin + href.split(/[?#]/, 1)[0];
}

export type ClipboardNav = { clipboard?: { writeText(t: string): Promise<void> } };

/** SH-6's copy routine for `el`, shared by Copy link and "Copy link to this selection" (F-40 SR-7):
 *  the Clipboard API, then "Link copied" for 2 s; without it (a non-secure context) or when it
 *  refuses, `fallback` shows the URL. No execCommand. */
export function copier(el: HTMLElement, nav: ClipboardNav, fallback: (url: string) => void): (url: string) => void {
  let idle = el.textContent;
  let timer = 0;
  return (url) => {
    const write = nav.clipboard ? nav.clipboard.writeText(url) : Promise.reject(new Error('no clipboard'));
    write.then(
      () => {
        if (!timer) idle = el.textContent; // in the page language (F-54)
        el.textContent = t('linkCopied');
        clearTimeout(timer);
        timer = window.setTimeout(() => {
          el.textContent = idle;
          timer = 0;
        }, 2000);
      },
      () => fallback(url),
    );
  };
}

/** The copy fallback: `url` in a read-only, pre-selected field in `host` (one per host). */
export function copyField(host: Element, url: string): HTMLInputElement {
  let field = host.querySelector<HTMLInputElement>('input.copy-url');
  if (!field) {
    field = host.ownerDocument.createElement('input');
    field.className = 'copy-url';
    field.readOnly = true;
    host.appendChild(field);
  }
  field.value = url;
  field.focus();
  field.select();
  return field;
}

/** Copy-link button (SH-6): the plain viewing link, its fallback field on its own header line. */
export function enhanceCopyLink(root: ParentNode, origin: string, nav: ClipboardNav): void {
  const link = root.querySelector<HTMLAnchorElement>('a[data-copy-link]');
  if (!link) return;
  const url = viewingUrl(origin, link.getAttribute('href'));
  if (url === null) return;
  const head = link.closest('header') ?? link.parentElement;
  const copy = copier(link, nav, (u) => head && copyField(head, u));
  link.addEventListener('click', (event) => {
    event.preventDefault();
    copy(url);
  });
}

/** The Report button (F-28 RF-1, RF-2, RF-5): shown only with JavaScript and a valid public ID,
 *  taken from the copy link's view path, never `location`. The form is a lazy chunk (ADR-011). */
export function enhanceReport(root: ParentNode, load = () => Promise.all([import('../report/form'), loadGroup('report')]).then(([m]) => m)): void {
  const button = root.querySelector<HTMLButtonElement>('button[data-report]');
  const id = root.querySelector('a[data-copy-link]')?.getAttribute('href')?.split('/').pop();
  if (!button || !id || !/^[0-9A-Za-z]{22}$/.test(id)) return;
  button.hidden = false;
  lazyClick(button, load, (m) => m.openReport(button.ownerDocument, id), 'reportFailed');
}

/** A click loads the lazy chunk and opens; on a failed load, one line after `at` (F-28, F-39, F-58),
 *  never a second one. */
function lazyClick<M>(button: HTMLElement, load: () => Promise<M>, open: (m: M) => void, text: 'reportFailed' | 'helpFailed' | 'gpxFailed', at = button): void {
  button.addEventListener('click', () => {
    load().then(open, () => {
      if (at.nextElementSibling?.className === 'report-fail') return;
      const p = at.ownerDocument.createElement('span');
      p.className = 'report-fail';
      p.textContent = t(text);
      at.after(p);
    });
  });
}

/** How to use (F-39 HU-6, HU-7; AP-22): reveals the template's button, emphasised until the dialog
 *  has been shown on this device (`opened`); the dialog is a lazy chunk (ADR-011). */
export function enhanceHowto(root: ParentNode, opened: boolean, onopen: () => void, load = () => Promise.all([import('../howto'), loadGroup('help')]).then(([m]) => m)): void {
  const button = root.querySelector<HTMLButtonElement>('button[data-howto]');
  if (!button) return;
  button.classList.add('ready');
  button.classList.toggle('fresh', !opened);
  lazyClick(
    button,
    load,
    (m) => {
      m.openHowto(button.ownerDocument);
      button.classList.remove('fresh');
      onopen();
    },
    'helpFailed',
  );
}

/** "About these numbers" (F-58 SE-5, SE-6; AP-24): reveals the bar under How to use (the data file is decoded);
 *  the dialog is a lazy chunk, which reads the trim string from the button (SE-2). `facts` is read
 *  on every open. */
export function enhanceSummaryInfo(root: ParentNode, units: UnitsStore, facts: () => Facts, load = () => Promise.all([import('../summary-info'), loadGroup('help')]).then(([m]) => m)): void {
  const button = root.querySelector<HTMLButtonElement>('button[data-summary-info]');
  if (!button) return;
  button.classList.add('ready'); // as How to use: reserved and invisible until now
  lazyClick(button, load, (m) => m.openSummaryInfo(button, facts(), units.get(), summaryItems), 'helpFailed', button.parentElement!);
}

/** "Download route (GPX)" (F-57 GX-5 to GX-7; SH-13): reveals the template's reserved button once the
 *  data file is decoded; the builder is a lazy chunk. Viewing page only: `input` must hold the public
 *  data file's positions (GX-2), never the edit page's draft. */
export function enhanceGpx(root: ParentNode, input: GpxInput, load = () => import('../gpx')): void {
  const button = root.querySelector<HTMLButtonElement>('button[data-gpx]');
  if (!button) return;
  button.classList.add('ready');
  lazyClick(button, load, (m) => m.downloadGpx(button.ownerDocument, input), 'gpxFailed');
}

