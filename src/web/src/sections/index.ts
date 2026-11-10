// Multisport sections in the browser (F-42 MS-6, MS-7, MS-9; AP-12). LAZY: reached only through the
// page glue's import(), after the charts' first paint, for a workout whose sessions hold two or more
// sports. Gives the separator rows their ranges and the charts their leg lines and names.

import './sections.css';
import type { WorkoutData } from '../lib/datafile';
import { legs, sectionRanges, sections, type Section } from '../lib/sections';
import type { ChartDecorator } from '../components/charts/uplot-options';

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

/** The chunk's entry: separator ranges into the laps enhancer (MS-6), the decorator into the charts. */
export function startSections(data: WorkoutData, look: { decorate?: ChartDecorator }, rows: (pairs: readonly number[]) => void): void {
  const secs = sections(data.meta);
  if (!secs) return;
  rows(sectionRanges(secs, data.sampleCount));
  look.decorate = decorator(secs, data.sampleCount);
}
