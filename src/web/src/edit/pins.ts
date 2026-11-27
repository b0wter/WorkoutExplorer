// The pin editor (F-56 PN-12; PIN-1, PIN-3 to PIN-5, PIN-7, PIN-8; OQ-234, OQ-272, OQ-273, OQ-275).
// Owner chunk. Display goes through the lazy src/pins view over draft.bin; this module holds the
// owner's pins (sample indices of the draft), "Add pin", the pin dialog, and the set Save sends. It
// mirrors the server's rules so a refusal never shows in normal use; the server alone decides.
// PN-11: owner text only through .value and textContent, never as HTML.

import type { WorkoutData } from '../lib/datafile';
import type { PinsView } from '../pins';
// Through src/pins (not lib/pins): this static import puts the view and lib/pins in one lazy chunk
// with the owner chunk's dependency, instead of a third shared chunk in the viewing entry's preload list.
import { MAX_PINS, PIN_NOTE_MAX, PIN_TITLE_MAX, PIN_TYPES, removed as removedText, typeName, dimReason, fromApi, hasVisibleRoute, pinName, snap, sortPins, toApi, type ApiPin, type PinType, type Shade, type ShownPin } from '../pins';

import { legSport } from '../lib/sections/order';
import { distanceUnit, formatDistance, formatElapsed } from '../lib/format';
import { onLanguage } from '../lib/i18n';
import { t } from './texts';
import { TWO_COLUMN_QUERY } from '../page/splitter';
import { createPlus, type PlusItem } from './plus';
import { startMapMenu } from './mapmenu';

export interface PinEditorEnv {
  /** The pins slot: the tools go first, the view's list after them. */
  readonly slot: HTMLElement;
  readonly view: PinsView;
  readonly data: WorkoutData;
  /** start_time in Unix seconds (TR-4's arithmetic). */
  readonly startS: number;
  /** The live trim window and hiding limits (TR-9, HR-8). */
  shade(): Shade;
  /** Any pin change (the panel's status clears). */
  onedit(): void;
  /** F-65 AL-9: the "+" items before Add pin (Add lap). */
  readonly items?: readonly PlusItem[];
}

export interface PinEditor {
  /** The shading changed (sliders, trim) or the laps: dimmed states and the "+" items follow. */
  refresh(): void;
  /** `{ pins }` when the set to send differs from the stored one, else {} (E-6). */
  patch(stored: readonly ApiPin[]): { pins?: ApiPin[] };
  /** A settings response: the stored set is the truth; one line when a trim removed pins (AC 36). */
  saved(stored: readonly ApiPin[]): void;
}

type Pin = { -readonly [K in keyof ShownPin]: ShownPin[K] };

export function startPinEditor(env: PinEditorEnv, stored: readonly ApiPin[]): PinEditor {
  const { slot, view, data, startS } = env;
  const { cursor, selection } = view;
  const doc = slot.ownerDocument;
  const time = data.time;
  const lat = data.position?.latitude;
  const load = (ps: readonly ApiPin[]): Pin[] => ps.map((p) => fromApi(p, time, startS));
  let list = load(stored);

  const make = <K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text = ''): HTMLElementTagNameMap[K] => {
    const e = doc.createElement(tag);
    e.className = cls;
    e.textContent = text;
    return e;
  };
  const button = (text: string): HTMLButtonElement => {
    const b = make('button', 'edit-button', text);
    b.type = 'button';
    return b;
  };

  // ---- tools: "+" with Add lap and Add pin (at the cursor or the selected range), one message line ----
  const tools = make('div', 'edit-row pin-tools');
  const addPin: PlusItem = {
    label: () => t('addPin'),
    short: () => t('addPinShort'),
    // F-56's rules: no Add pin without a visible route or at the cap; its reason when refused.
    state: () => {
      if (list.length >= MAX_PINS || !hasVisibleRoute(env.shade(), lat)) return { hidden: true, off: true, why: '' };
      const sp = spot();
      const twin = sp !== null && list.some((p) => p.from === sp[0] && p.to === sp[1]);
      return { off: sp === null || twin, why: sp === null ? t('pinHiddenSpot') : twin ? t('pinHere') : '' };
    },
    run: () => placePin(),
  };
  // No positions (static per workout): no Add pin item at all, so the one-column box has no blank row.
  const plusItems = lat ? [...(env.items ?? []), addPin] : [...(env.items ?? [])];
  /** A range or point in the units setting: "at 12.4 km", "12.4–13.1 km"; elapsed time without distance. */
  const describe = (r: readonly number[]): string => {
    const dist = data.metrics.distance;
    const u = view.units.get();
    const km = !!dist && r.every((i) => Number.isFinite(dist[i]));
    const ds = legSport(data.meta, r[0], r[r.length - 1]);
    const v = r.map((i) => (km ? formatDistance(dist![i], u, ds) : formatElapsed(time[i] - time[0]))).join('–') + (km ? ' ' + distanceUnit(u, ds) : '');
    return r.length > 1 ? v : t('whereAt', { v });
  };
  /** Where an enabled item acts. */
  const where = (): string => {
    const r = selection.start() >= 0 ? [selection.start(), selection.end()] : [cursor.get() ?? -1];
    return r[0] < 0 ? '' : describe(r);
  };
  const plus = createPlus(doc, plusItems, where);
  tools.append(plus.group);
  const msg = tools.appendChild(make('span', 'edit-status'));
  msg.setAttribute('role', 'status');
  // Two-column layout (the page's own switch): "+" is a control of the map's top-right stack, under the location button (PN-12,
  // AL-9); one column: in the tools row. Same elements, so name, focus order and texts are kept.
  const mapSlot = doc.getElementById('map-slot');
  const onMap = mapSlot ? make('div', 'wx-addpin maplibregl-ctrl') : null;
  const mq = doc.defaultView?.matchMedia?.(TWO_COLUMN_QUERY);
  const dock = (): void => {
    const map = onMap !== null && (mq?.matches ?? true); // no matchMedia (tests): two columns
    if (map) {
      onMap.append(plus.group);
      if (!onMapAdded) (onMapAdded = true), view.map((m) => m.control(onMap, 'top-right')); // empty (CSS: hidden) in one column
    } else tools.prepend(plus.group);
    plus.setFlat(!map);
  };
  let onMapAdded = false;
  mq?.addEventListener?.('change', dock);
  // The same items on a mouse right-click of the route; both layouts, fullscreen included.
  if (mapSlot) startMapMenu({ host: mapSlot, cursor, selection, items: plusItems, where });

  // ---- the dialog (UI-pin-dialog): one for add and edit, built on first use; nothing changes until OK ----
  const form = make('form', 'pin-form');
  form.id = 'wx-pin-form';
  const field = (input: HTMLElement): Text => {
    const l = form.appendChild(make('label', 'edit-field'));
    const text = l.appendChild(doc.createTextNode(''));
    l.append(input);
    return text;
  };
  const type = make('select', '');
  for (const p of PIN_TYPES) type.add(new Option('', p));
  const typeLabel = field(type);
  const title = make('input', '');
  title.maxLength = PIN_TITLE_MAX;
  const titleLabel = field(title);
  const note = make('textarea', '');
  note.maxLength = PIN_NOTE_MAX;
  note.rows = 3;
  const noteLabel = field(note);
  const placeLine = make('p', 'edit-small');
  const del = button('');
  del.classList.add('pin-del');
  const ok = make('button', 'edit-save');
  ok.setAttribute('form', form.id); // the actions sit outside the form: OK submits it (Enter in Title too)
  const cancel = button('');
  slot.prepend(tools);
  /** Every fixed text, again on a language change (F-54). */
  const label = (): void => {
    plus.label();
    PIN_TYPES.forEach((p, i) => (type.options[i].text = typeName(p)));
    typeLabel.data = t('pinType');
    titleLabel.data = t('pinTitle');
    noteLabel.data = t('pinNote');
    del.textContent = t('deletePin');
    ok.textContent = t('ok');
    cancel.textContent = t('cancel');
  };
  label();

  let dims = '';
  const shown = (): ShownPin[] => {
    const s = env.shade();
    return list.map((p) => ({ ...p, dim: dimReason(p, s, lat) }));
  };
  /** Hands the pins to the view; `keep` stays selected. */
  const render = (keep: number | null): void => {
    const ps = shown();
    dims = ps.map((p) => p.dim).join('|');
    view.set(ps, keep);
    controls();
  };
  /** "+" needs a cursor or range; light enough for every cursor step (the item states run only while the list is open). */
  const controls = (): void => plus.refresh(selection.start() >= 0 || (cursor.get() ?? -1) >= 0);
  const say = (text: string): void => {
    msg.textContent = text;
  };
  /** One change to the set: shown at once (OQ-274), saved with Save. */
  const changed = (keep: number | null): void => {
    sortPins(list);
    env.onedit();
    render(keep);
  };

  // A tap on a row or marker selects the pin: the dialog opens; any close deselects, so the next tap opens it again.
  view.onselect((k) => k !== null && void open(k));
  view.onrow((li, k) => {
    li.querySelector('.pin-note')?.remove(); // the list is read-only: no note under the row
    const x = make('button', 'added-x-btn', '×');
    x.type = 'button';
    x.title = t('deletePin') + ': ' + pinName(list[k]);
    x.setAttribute('aria-label', x.title);
    x.addEventListener('click', () => (list.splice(k, 1), changed(null), focusRow(k)));
    li.append(x);
  });

  /** A sample snapped to the nearest one with a position (OQ-234); -1 when refused (PIN-7). */
  const place = (i: number): number => {
    const s = lat && i >= 0 ? snap(lat, i) : -1;
    return s < 0 || dimReason({ from: s }, env.shade(), lat) ? -1 : s;
  };
  /** Add pin's target: the range's snapped ends, else the cursor's sample; null when none or refused. */
  const spot = (): [number, number?] | null => {
    const r = selection.start() >= 0 ? [selection.start(), selection.end()] : [cursor.get() ?? -1];
    const s = r.map(place);
    return s.some((x) => x < 0) || (s.length > 1 && s[0] >= s[1]) ? null : (s as [number, number?]);
  };
  dock(); // after spot(): a flat "+" paints its items at once
  selection.subscribe(controls);
  cursor.subscribe(controls);
  view.units.subscribe(controls); // the reasons' distances

  // ---- the dialog: open, then OK (apply), Delete or any other close (no change) ----
  /** After a delete or a re-render the opener is gone: the row now at `k`, else the heading, else "+" (as addedtable.ts). */
  const focusRow = (k: number): void => {
    const rows = slot.querySelectorAll<HTMLElement>('.pin-item[data-pin] > .pin-row');
    const h = slot.querySelector<HTMLElement>('.pins-head');
    h?.setAttribute('tabindex', '-1');
    const items = [...plus.group.querySelectorAll<HTMLElement>('.plus-item')];
    (rows[Math.min(k, rows.length - 1)] ?? h ?? (plus.button.hidden ? items[items.length - 1] : plus.button)).focus({ preventScroll: true });
  };
  let sess: { p: Pin; k: number | null; from: Element | null; marker: boolean; result: 'ok' | 'del' | null } | null = null;
  /** k: an existing pin; null: the new pin at `fresh`. */
  const open = async (k: number | null, fresh?: Pin): Promise<void> => {
    if (sess) return;
    const p = k === null ? fresh! : list[k];
    const act = doc.activeElement;
    // the opener: a marker (focus returns to its replacement), a row, or for a new pin whatever the add came from
    sess = { p, k, from: act, marker: !!act?.closest('.pin-mk'), result: null };
    const s = sess;
    let openDialog: typeof import('../dialog').openDialog;
    try {
      ({ openDialog } = await import('../dialog'));
    } catch (e) {
      console.warn('pin dialog failed to load', e);
      sess = null;
      view.select(null);
      return;
    }
    type.value = p.type;
    title.value = p.title;
    title.placeholder = typeName(p.type);
    note.value = p.note;
    placeLine.textContent = (k !== null && shown()[k].dim) || describe(p.to === undefined ? [p.from] : [p.from, p.to]);
    del.hidden = k === null;
    const d = openDialog(doc, 'pin-dlg', t(k === null ? 'addPin' : 'editPin'), [placeLine, form], slot, [del, ok, cancel]);
    d.onkeydown = (e) => e.key === 'Escape' && e.stopPropagation(); // Esc closes this dialog only, not the fullscreen map
    d.addEventListener('close', () => close(s), { once: true });
    (k === null ? title : type).focus({ preventScroll: true });
  };
  type.addEventListener('change', () => (title.placeholder = typeName(type.value as PinType)));
  form.addEventListener('submit', (e) => {
    e.preventDefault();
    if (sess) sess.result = 'ok';
    dlg().close();
  });
  del.addEventListener('click', () => {
    if (sess) sess.result = 'del';
    dlg().close();
  });
  cancel.addEventListener('click', () => dlg().close());
  const dlg = (): HTMLDialogElement => ok.closest('dialog')!;
  const close = ({ p, k: k0, from, marker, result }: NonNullable<typeof sess>): void => {
    let k = k0;
    sess = null;
    const gone = k !== null && list.indexOf(p) < 0; // Save replaced the list meanwhile: act on nothing
    if (!gone && result === 'ok') {
      Object.assign(p, { type: type.value, title: title.value, note: note.value });
      if (k === null) list.push(p);
      changed(null);
    } else if (!gone && result === 'del') {
      k = list.indexOf(p);
      list.splice(k, 1);
      changed(null);
    }
    view.select(null); // a section's range stays selected
    if (gone) return;
    if (result === 'del') return focusRow(k!);
    const at = list.indexOf(p);
    const back = at < 0 ? null : (marker ? doc.querySelector<HTMLElement>(`.pin-mk[data-pin="${at}"]`) : doc.querySelector<HTMLElement>(`.pin-item[data-pin="${at}"] .pin-row`)) ?? null;
    // a new pin: the "+" (two columns), "+ Pin" (one column) or, for the right-click, the map the menu returned focus to
    const items = [...plus.group.querySelectorAll<HTMLElement>('.plus-item')];
    const add = !from || from === doc.body || plus.group.contains(from) || !from.isConnected ? (plus.button.hidden ? items[items.length - 1] : plus.button) : (from as HTMLElement);
    (k === null ? add : back ?? (from?.isConnected ? (from as HTMLElement) : add)).focus({ preventScroll: true });
  };

  // ---- placing: at the selected point or range (OQ-272, revised 21 Nov 2026); no waiting mode ----
  const placePin = (): void => {
    const sp = spot();
    if (!sp) return;
    say('');
    void open(null, { type: 'point_of_interest', title: '', note: '', from: sp[0], ...(sp[1] !== undefined && { to: sp[1] }) });
  };

  render(null);
  onLanguage(() => {
    label();
    render(view.selected());
  });

  const send = (): ApiPin[] => {
    const s = env.shade();
    return list.filter((p) => dimReason(p, s, lat) !== removedText()).map((p) => toApi(p, time, startS));
  };
  return {
    refresh: () => {
      if (shown().map((p) => p.dim).join('|') !== dims) render(view.selected());
      else controls();
    },
    patch: (stored) => {
      const pins = send();
      return JSON.stringify(pins) === JSON.stringify(stored) ? {} : { pins };
    },
    saved: (stored) => {
      const key = (p: ApiPin): string => p.start_at + p.end_at;
      const kept = new Set(stored.map(key));
      const removed = list.filter((p) => !kept.has(key(toApi(p, time, startS)))).length;
      list = load(stored);
      render(null);
      say(removed ? t('pinsRemoved', { n: removed }) : '');
    },
  };
}
