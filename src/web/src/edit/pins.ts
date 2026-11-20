// The pin editor (F-56 PN-12; PIN-1, PIN-3 to PIN-5, PIN-7, PIN-8; OQ-234, OQ-272, OQ-273, OQ-275).
// Owner chunk. Display goes through the lazy src/pins view over draft.bin; this module holds the
// owner's pins (sample indices of the draft), "Add pin", the form, and the set Save sends. It
// mirrors the server's rules so a refusal never shows in normal use; the server alone decides.
// PN-11: owner text only through .value and textContent, never as HTML.

import type { WorkoutData } from '../lib/datafile';
import type { PinsView } from '../pins';
// Through src/pins (not lib/pins): this static import puts the view and lib/pins in one lazy chunk
// with the owner chunk's dependency, instead of a third shared chunk in the viewing entry's preload list.
import { MAX_PINS, PIN_NOTE_MAX, PIN_TITLE_MAX, PIN_TYPES, REMOVED, TYPE_NAMES, dimReason, fromApi, hasVisibleRoute, snap, sortPins, toApi, type ApiPin, type PinType, type Shade, type ShownPin } from '../pins';

/** The server's sentence (lifecycle.MsgPinHidden): the browser shows it before sending. */
export const PIN_REFUSED = 'This pin is in a hidden or trimmed part of the route.';

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

  // ---- tools: Add pin, the armed hint with Cancel, one message line ----
  const tools = make('div', 'edit-row pin-tools');
  const add = tools.appendChild(button('Add pin'));
  const hint = tools.appendChild(make('span', 'edit-small', 'Tap the map or a chart to place the pin.'));
  const cancel = tools.appendChild(button('Cancel'));
  const msg = tools.appendChild(make('span', 'edit-status'));
  msg.setAttribute('role', 'status');

  // ---- the form of the selected pin ----
  const form = make('div', 'pin-form');
  const field = (label: string, input: HTMLElement): void => {
    const l = form.appendChild(make('label', 'edit-field', label));
    l.append(input);
  };
  const type = make('select', '');
  for (const t of PIN_TYPES) type.add(new Option(TYPE_NAMES[t], t));
  field('Type', type);
  const title = make('input', '');
  title.maxLength = PIN_TITLE_MAX;
  field('Title', title);
  const note = make('textarea', '');
  note.maxLength = PIN_NOTE_MAX;
  note.rows = 2;
  field('Note', note);
  const row = form.appendChild(make('div', 'edit-row'));
  const sectionLabel = row.appendChild(make('label', 'edit-small', ' Section (the selected range)'));
  const section = make('input', '');
  section.type = 'checkbox';
  sectionLabel.prepend(section);
  const move = row.appendChild(button('Move'));
  const del = row.appendChild(button('Delete pin'));
  const done = row.appendChild(button('Done'));
  slot.prepend(tools, form);

  let armed: '' | 'add' | 'move' = '';
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
    add.hidden = armed !== '' || list.length >= MAX_PINS || !hasVisibleRoute(env.shade(), lat);
    hint.hidden = cancel.hidden = armed === '';
    form.hidden = k === null;
    if (k === null) return;
    const p = list[k];
    section.checked = p.to !== undefined;
    sectionLabel.hidden = p.to === undefined && selection.start() < 0;
    move.hidden = p.to !== undefined;
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
      title.placeholder = TYPE_NAMES[p.type];
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
      title.placeholder = TYPE_NAMES[p.type];
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
    if (s < 0 || dimReason({ from: s }, env.shade(), lat)) {
      say(PIN_REFUSED);
      return -1;
    }
    return s;
  };
  /** OQ-273: the selected range as the section's ends, each snapped; refused as a point would be. */
  const toSection = (p: Pin, a: number, b: number): void => {
    const [sa, sb] = [place(a), place(b)];
    if (sa < 0 || sb < 0) return;
    if (sa >= sb) return say(PIN_REFUSED);
    p.from = sa;
    p.to = sb;
    changed(list.indexOf(p));
  };
  section.addEventListener('change', () => {
    say('');
    const k = view.selected();
    if (k === null) return;
    const p = list[k];
    if (section.checked && selection.start() >= 0) toSection(p, selection.start(), selection.end());
    else if (!section.checked) {
      delete p.to;
      changed(list.indexOf(p));
    } else controls();
  });
  selection.subscribe((a, b) => {
    const k = view.selected();
    const p = k === null ? null : list[k];
    if (p?.to !== undefined && a >= 0 && (a !== p.from || b !== p.to)) toSection(p, a, b);
    else controls();
  });

  // ---- arming: the next tap on the map or a chart places (OQ-272) ----
  const arm = (mode: typeof armed): void => {
    armed = mode;
    say('');
    controls();
  };
  add.addEventListener('click', () => arm('add'));
  move.addEventListener('click', () => arm('move'));
  cancel.addEventListener('click', () => arm(''));
  // Esc disarms before it would leave the fullscreen map (the map listens on the document).
  doc.defaultView?.addEventListener(
    'keydown',
    (e) => {
      if (armed && e.key === 'Escape') {
        e.stopPropagation();
        arm('');
      }
    },
    true,
  );
  const charts = doc.getElementById('charts-slot');
  const mapSlot = doc.getElementById('map-slot');
  doc.addEventListener(
    'click',
    (e) => {
      if (!armed) return;
      const t = e.target as Element;
      // As the cursor would (ADR-015 on the map, x on a chart); a miss leaves it armed.
      const i = mapSlot?.contains(t) ? (t.tagName === 'CANVAS' ? view.sampleAt(e.clientX, e.clientY) : -2) : charts?.contains(t) && t.closest('.chart-host') ? (cursor.get() ?? -1) : -2;
      if (i === -2) return;
      say('');
      const s = place(i);
      if (s < 0) return;
      const k = view.selected();
      let p: Pin;
      if (armed === 'move' && k !== null) {
        p = list[k];
        p.from = s;
      } else list.push((p = { type: 'point_of_interest', title: '', note: '', from: s }));
      armed = '';
      changed(list.indexOf(p));
      view.select(list.indexOf(p));
    },
    true,
  );

  render(null);

  const send = (): ApiPin[] => {
    const s = env.shade();
    return list.filter((p) => dimReason(p, s, lat) !== REMOVED).map((p) => toApi(p, time, startS));
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
      arm('');
      render(null);
      say(removed ? `${removed} ${removed === 1 ? 'pin' : 'pins'} in the trimmed part removed.` : '');
    },
  };
}
