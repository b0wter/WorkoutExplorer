// The pin editor (F-56 PN-12; PIN-1, PIN-3 to PIN-5, PIN-7, PIN-8; OQ-234, OQ-272, OQ-273, OQ-275).
// Owner chunk. Display goes through the lazy src/pins view over draft.bin; this module holds the
// owner's pins (sample indices of the draft), "Add pin", the form, and the set Save sends. It
// mirrors the server's rules so a refusal never shows in normal use; the server alone decides.
// PN-11: owner text only through .value and textContent, never as HTML.

import type { WorkoutData } from '../lib/datafile';
import type { PinsView } from '../pins';
// Through src/pins (not lib/pins): this static import puts the view and lib/pins in one lazy chunk
// with the owner chunk's dependency, instead of a third shared chunk in the viewing entry's preload list.
import { MAX_PINS, PIN_NOTE_MAX, PIN_TITLE_MAX, PIN_TYPES, removed as removedText, typeName, dimReason, fromApi, hasVisibleRoute, snap, sortPins, toApi, type ApiPin, type PinType, type Shade, type ShownPin } from '../pins';

import { onLanguage } from '../lib/i18n';
import { t } from './texts';
import { TWO_COLUMN_QUERY } from '../page/splitter';

export interface PinEditorEnv {
  /** The pins slot: the tools and form go first, the view's list after them. */
  readonly slot: HTMLElement;
  readonly view: PinsView;
  readonly data: WorkoutData;
  /** start_time in Unix seconds (TR-4's arithmetic). */
  readonly startS: number;
  /** The live trim window and hiding limits (TR-9, HR-8). */
  shade(): Shade;
  /** Any pin change (the panel's status clears). */
  onedit(): void;
}

export interface PinEditor {
  /** The shading changed (sliders, trim): dimmed states and "Add pin" follow. */
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

  // ---- tools: Add pin (at the cursor or the selected range), one message line ----
  const tools = make('div', 'edit-row pin-tools');
  const add = tools.appendChild(button(''));
  // The reason Add pin is off, in the row (touch has no tooltip); empty it takes no space.
  const why = tools.appendChild(make('span', 'edit-small'));
  const msg = tools.appendChild(make('span', 'edit-status'));
  msg.setAttribute('role', 'status');
  // Two-column layout (the page's own switch): Add pin and its reason sit on the map, bottom left
  // (PN-12); one column: in the tools row. Same elements, so name, focus order and texts are kept.
  const mapSlot = doc.getElementById('map-slot');
  const onMap = mapSlot ? make('div', 'wx-addpin') : null;
  const mq = doc.defaultView?.matchMedia?.(TWO_COLUMN_QUERY);
  const dock = (): void => {
    const map = onMap !== null && !!mq?.matches;
    if (map) {
      onMap.append(add, why);
      if (!onMap.isConnected) mapSlot!.append(onMap);
    } else tools.prepend(add, why);
    tools.classList.toggle('on-map', map);
  };
  dock();
  mq?.addEventListener?.('change', dock);

  // ---- the form of the selected pin ----
  const form = make('div', 'pin-form');
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
  note.rows = 2;
  const noteLabel = field(note);
  const row = form.appendChild(make('div', 'edit-row'));
  const move = row.appendChild(button(''));
  const del = row.appendChild(button(''));
  const done = row.appendChild(button(''));
  slot.prepend(tools, form);
  /** Every fixed text, again on a language change (F-54). */
  const label = (): void => {
    add.textContent = t('addPin');
    PIN_TYPES.forEach((p, i) => (type.options[i].text = typeName(p)));
    typeLabel.data = t('pinType');
    titleLabel.data = t('pinTitle');
    noteLabel.data = t('pinNote');
    move.textContent = t('movePin');
    del.textContent = t('deletePin');
    done.textContent = t('donePin');
    const k = view.selected();
    if (k !== null) title.placeholder = typeName(list[k].type);
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
  const controls = (): void => {
    const k = view.selected();
    // Hidden (not disabled) only when no pin can be added at all; never a layout change while scrubbing.
    add.hidden = list.length >= MAX_PINS || !hasVisibleRoute(env.shade(), lat);
    form.hidden = k === null;
    if (k !== null) move.hidden = list[k].to !== undefined;
    enable();
  };
  /** Add pin and Move need a usable spot; light enough for every cursor step. */
  const enable = (): void => {
    const sp = spot();
    const none = selection.start() < 0 && (cursor.get() ?? -1) < 0;
    const twin = sp !== null && list.some((p) => p.from === sp[0] && p.to === sp[1]);
    add.disabled = sp === null || twin;
    why.textContent = add.hidden || none ? '' : sp === null ? t('pinHiddenSpot') : twin ? t('pinHere') : '';
    const k = view.selected();
    move.disabled = k === null || place(cursor.get() ?? -1) < 0;
  };
  const say = (text: string): void => {
    msg.textContent = text;
  };
  /** One change to the set: shown at once (OQ-274), saved with Save. */
  const changed = (keep: number | null): void => {
    sortPins(list);
    env.onedit();
    render(keep);
  };

  view.onselect((k) => {
    if (k !== null) {
      const p = list[k];
      type.value = p.type;
      title.value = p.title;
      title.placeholder = typeName(p.type);
      note.value = p.note;
    }
    controls();
  });
  const edit = (f: (p: Pin) => void): void => {
    const k = view.selected();
    if (k === null) return;
    f(list[k]);
    env.onedit();
    render(k);
  };
  type.addEventListener('change', () =>
    edit((p) => {
      p.type = type.value as PinType;
      title.placeholder = typeName(p.type);
    }),
  );
  title.addEventListener('input', () => edit((p) => (p.title = title.value)));
  note.addEventListener('input', () => edit((p) => (p.note = note.value)));
  del.addEventListener('click', () => {
    const k = view.selected();
    if (k === null) return;
    list.splice(k, 1);
    changed(null);
  });
  done.addEventListener('click', () => view.select(null));

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
  /** OQ-273: the selected range as the section's ends, each snapped; refused as a point would be. */
  const toSection = (p: Pin, a: number, b: number): void => {
    const [sa, sb] = [place(a), place(b)];
    if (sa < 0 || sb < 0 || sa >= sb) return say(t('pinRefused')), enable();
    p.from = sa;
    p.to = sb;
    changed(list.indexOf(p));
  };
  selection.subscribe((a, b) => {
    const k = view.selected();
    const p = k === null ? null : list[k];
    if (p?.to !== undefined && a >= 0 && (a !== p.from || b !== p.to)) toSection(p, a, b);
    else controls();
  });
  cursor.subscribe(enable);

  // ---- placing: at the selected point or range (OQ-272, revised 21 Nov 2026); no waiting mode ----
  add.addEventListener('click', () => {
    if (add.disabled) return;
    const sp = spot()!;
    say('');
    const p: Pin = { type: 'point_of_interest', title: '', note: '', from: sp[0], ...(sp[1] !== undefined && { to: sp[1] }) };
    list.push(p);
    changed(list.indexOf(p));
    view.select(list.indexOf(p));
  });
  move.addEventListener('click', () => {
    const k = view.selected();
    const s = place(cursor.get() ?? -1);
    if (k === null || s < 0) return;
    say('');
    const p = list[k];
    p.from = s;
    changed(list.indexOf(p));
    view.select(list.indexOf(p));
  });

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
