// F-65 AL-8: the owner's added lap boundaries as a small table under the pins list (AP-8). Owner
// chunk. Same classes as the pins list; the × is the laps table's `remove` (both tables follow the set).

import type { WorkoutData } from '../lib/datafile';
import type { UnitsStore } from '../lib/units';
import { distanceUnit, formatDistance, formatElapsed } from '../lib/format';
import { onLanguage } from '../lib/i18n';
import type { AddedLaps } from './laps';
import { t } from './texts';

export interface AddedTable {
  /** Rebuilds the rows (set, live trim, units or language changed). */
  refresh(): void;
}

export function startAddedTable(slot: HTMLElement, data: WorkoutData, added: AddedLaps, units: UnitsStore): AddedTable {
  const doc = slot.ownerDocument;
  const dist = data.metrics.distance;
  const make = <K extends keyof HTMLElementTagNameMap>(tag: K, cls: string, text = ''): HTMLElementTagNameMap[K] => {
    const e = doc.createElement(tag);
    e.className = cls;
    e.textContent = text;
    return e;
  };
  const box = make('div', 'added-laps');
  const head = box.appendChild(make('h2', 'pins-head'));
  const list = box.appendChild(make('ul', 'pin-list'));
  slot.append(box);
  let focus = -1; // the index of the × just used: focus goes to the one now at its place

  const refresh = (): void => {
    const rows = added.rows().flatMap((r) => (r ? [r] : []));
    box.hidden = rows.length === 0;
    head.textContent = t('addedLaps');
    const u = units.get();
    const line = (cols: string[], cls: string, dim = false): HTMLLIElement => {
      const li = make('li', 'pin-item ' + cls);
      const d = li.appendChild(make('div', 'pin-row added-row'));
      cols.forEach((c, i) => d.append(make('span', (i === 0 ? 'pin-t' : 'pin-d') + (dim ? ' dim' : ''), c)));
      return li;
    };
    list.replaceChildren(
      ...(rows.length ? [line([t('addedColLap'), t('addedColTime'), ...(dist ? [t('addedColDistance')] : [])], 'added-head')] : []),
      ...rows.map((r) => {
        const li = line([String(r.n), formatElapsed(data.time[r.at] - data.time[0]), ...(dist ? [formatDistance(dist[r.at], u, data.meta.sport) + ' ' + distanceUnit(u, data.meta.sport)] : [])], '', !!r.dim);
        const d = li.firstElementChild!;
        const b = make('button', 'added-x-btn', '×');
        b.type = 'button';
        b.setAttribute('aria-label', r.name);
        b.title = r.name;
        b.addEventListener('click', () => {
          focus = [...list.querySelectorAll('.added-x-btn')].indexOf(b);
          added.remove(r.at);
        });
        d.append(b);
        if (r.dim) d.append(make('span', 'edit-small', r.dim));
        return li;
      }),
    );
    if (focus >= 0) {
      const xs = list.querySelectorAll<HTMLElement>('.added-x-btn');
      const h = doc.querySelector<HTMLElement>('section.laps h2');
      h?.setAttribute('tabindex', '-1');
      (xs[Math.min(focus, xs.length - 1)] ?? h ?? doc.querySelector<HTMLElement>('.plus-button:not([hidden]), .plus-item'))?.focus();
      focus = -1;
    }
  };
  units.subscribe(refresh);
  onLanguage(refresh);
  refresh();
  return { refresh };
}
