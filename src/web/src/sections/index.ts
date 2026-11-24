// Multisport sections in the browser (F-42 MS-6, MS-7, MS-9; AP-12). LAZY: reached only through the
// page glue's import(), after the charts' first paint, for a workout whose sessions hold two or more
// sports. Gives the laps view's leg headings their ranges (F-64 LL-3) and the charts their leg lines and names.

import './sections.css';
import type { WorkoutData } from '../lib/datafile';
import { legs, sectionRanges, sections, type Leg, type Section } from '../lib/sections';
import { onLanguage } from '../lib/i18n';
import type { SelectionStore } from '../lib/selection';
import type { ChartDecorator, ChartView } from '../components/charts/uplot-options';
import { t } from './texts';

/** MS-7: in every chart a line at each leg start after the first and each leg's name from its left
 *  edge (the first leg's at sample 0), left out when wider than the leg. Replaces what it drew before. */
export function decorator(secs: readonly Section[], n: number): ChartDecorator {
  const drawn = legs(secs, n);
  return (over, x) => {
    const doc = over.ownerDocument;
    over.querySelector('.wx-legs')?.remove();
    const box = doc.createElement('div');
    box.className = 'wx-legs';
    over.prepend(box);
    for (const g of drawn) {
      const left = x(g.start);
      const add = (cls: string): HTMLElement => {
        const el = box.appendChild(doc.createElement('div'));
        el.className = cls;
        el.style.transform = `translateX(${left}px)`;
        return el;
      };
      if (g.start > 0) add('wx-leg-line');
      const name = add('wx-leg-name');
      name.textContent = g.name;
      // offsetWidth holds the gap after a line (CSS); the first leg has none, so a narrow one keeps its name.
      if (name.offsetWidth > x(g.end) - left) name.remove();
    }
  };
}

/** F-67 LP-1: the picker's sport legs (no transitions); a repeated sport is numbered ("Run 1"). */
export function pickable(secs: readonly Section[], n: number): { leg: Leg; label: string }[] {
  const gs = legs(secs, n).filter((g) => g.sport !== 'transition');
  return gs.map((g) => {
    const same = gs.filter((h) => h.sport === g.sport);
    return { leg: g, label: same.length > 1 ? g.name + ' ' + (same.indexOf(g) + 1) : g.name };
  });
}

/** The map seam the picker uses (LP-9). */
export type FitMap = (cb: (m: { fit(a: number, b: number): void }) => void) => unknown;

/** The chunk's entry: leg heading ranges into the laps enhancer (MS-6, F-64 LL-3), the decorator into
 *  the charts and, in the charts' axis toolbar row (`slot`), the leg picker (F-67 LP-2). */
export function startSections(
  data: WorkoutData,
  look: { decorator(key: string, f: ChartDecorator): void; view?: ChartView },
  rows: (pairs: readonly number[]) => void,
  selection?: SelectionStore,
  map?: FitMap,
  slot?: HTMLElement | null,
): void {
  const secs = sections(data.meta);
  if (!secs) return;
  const n = data.sampleCount;
  rows(sectionRanges(secs, n));
  look.decorator('legs', decorator(secs, n));
  // LP-2: no row without a distance stream, no picker without a sport leg (deviation, LP-1).
  const bar = slot?.querySelector('.toolbar');
  const picks = pickable(secs, n);
  const pick = bar && picks.length ? bar.appendChild(bar.ownerDocument.createElement('select')) : null;
  const fill = (s: readonly Section[]): void => {
    if (!pick) return;
    const k = pick.selectedIndex;
    pick.replaceChildren(new Option(t('wholeWorkout')), ...pickable(s, n).map((p) => new Option(p.label)));
    pick.selectedIndex = k < 0 ? 0 : k;
    pick.setAttribute('aria-label', t('legPick'));
  };
  if (pick) {
    pick.className = 'wx-leg-pick';
    fill(secs);
    // LP-3, LP-9: a view only (no range, OQ-316); the map fits once to the leg or the whole route.
    pick.onchange = () => {
      const g = picks[pick.selectedIndex - 1]?.leg;
      look.view = g ? { a: g.start, b: g.end, sport: g.sport } : null;
      map?.((m) => (g ? m.fit(g.start, g.end) : m.fit(-1, -1)));
    };
    // LP-8 (OQ-319): a new range outside the shown leg returns to the whole workout; the map stays.
    selection?.subscribe((a, b) => {
      const v = look.view;
      if (a >= 0 && v && (a < v.a || b > v.b)) {
        look.view = null;
        pick.selectedIndex = 0;
      }
    });
  }
  // F-54: leg names follow the language (the leg headings are the language pass's); LP-12 the picker too.
  onLanguage(() => {
    const s = sections(data.meta)!;
    look.decorator('legs', decorator(s, n));
    fill(s);
  });
}
