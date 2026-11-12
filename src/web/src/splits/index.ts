// Splits in the browser (F-46 LS-4 to LS-9; AP-8, AP-20). LAZY: reached only through the page glue's
// import(), after the charts' first paint, when the distance advances. Fills the laps heading slot
// (laps / splits switch on a page with laps, the split-distance drop-down in the splits view) and
// builds the splits table; on a page without laps it shows the splits at once and reveals the module.

import './splits.css';
import type { Totals, WorkoutData } from '../lib/datafile';
import type { Units } from '../lib/format';
import type { lapsGroup } from '../lib/page';
import { createRangeStats, emptyValues, type RangeStats } from '../lib/range';
import { poolDistance, presetBounds, splitChoice, type Pool, type Preset } from '../lib/splits';
import type { UnitsStore } from '../lib/units';

/** One splits table (F-18 L-5): range values over each split, formatted by lib/page's lapsGroup.
 *  NaN values reach lapsGroup as non-finite totals, which it treats as absent. A pool split's
 *  distance is in the pool's unit, header included (LS-6). */
function splitsTable(laps: typeof lapsGroup, doc: Document, sport: string, stats: RangeStats, bounds: number[], units: Units, preset: Preset, pool: Pool | null): HTMLTableElement {
  const out = emptyValues();
  const totals: { totals: Totals }[] = [];
  for (let i = 1; i < bounds.length; i++) {
    const v = stats.query(bounds[i - 1], bounds[i], out);
    totals.push({
      totals: {
        elapsed_time: v.elapsed,
        distance: v.distance,
        avg_speed: v.speed,
        ascent: v.gain,
        avg_heart_rate: v.heartRate,
        avg_power: v.power,
        avg_cadence: v.cadence,
      },
    });
  }
  const group = laps(sport, totals, units);
  const table = doc.createElement('table');
  const head = table.createTHead().insertRow();
  const body = table.createTBody();
  const dj = pool ? (group?.Columns.findIndex((c) => c.Key === 'distance') ?? -1) : -1;
  group?.Columns.forEach((c, j) => {
    const th = doc.createElement('th');
    th.dataset.k = c.Key;
    th.textContent = j === dj ? 'Distance (' + pool!.unit + ')' : c.Label;
    head.appendChild(th);
  });
  group?.Rows.forEach((row, i) => {
    const tr = body.insertRow();
    row.forEach((t, j) => (tr.insertCell().textContent = j === dj ? String(Math.round(poolDistance(totals[i].totals.distance!, preset, pool!))) : t));
  });
  return table;
}

/** The chunk's entry. `seam` is the laps enhancer's (LS-8): the rows tap through its one listener.
 *  `laps` is lib/page's lapsGroup, passed in: imported here, Vite splits lib/format out of the entry
 *  into a second script of the page. */
export function startSplits(root: ParentNode, data: WorkoutData, units: UnitsStore, seam: (container: Element, pairs: readonly number[]) => void, laps: typeof lapsGroup): void {
  const section = root.querySelector<HTMLElement>('section.laps');
  const slot = section?.querySelector('#laps-slot');
  if (!section || !slot || slot.firstChild) return;
  const stats = createRangeStats(data);
  const d = stats.distance;
  if (!d) return;
  const doc = section.ownerDocument;
  const server = section.querySelector<HTMLElement>('.laps-scroll');
  // LS-7: the choice lives here only; every load starts on the sport's default.
  const choice = splitChoice(data.meta, d[d.length - 1] - d[0]);
  let preset = choice.presets[choice.index];
  let splits: HTMLElement | null = null;
  // LS-9: only on arrival, a change, a switch to splits or a units change; the selection is untouched (LS-7).
  const build = (): void => {
    const bounds = presetBounds(d, preset, choice.pool);
    const wrap = doc.createElement('div');
    wrap.className = 'laps-scroll splits';
    wrap.hidden = splits?.hidden ?? true;
    wrap.appendChild(splitsTable(laps, doc, data.meta.sport, stats, bounds, units.get(), preset, choice.pool));
    seam(wrap, bounds.slice(1).flatMap((b, i) => [bounds[i], b]));
    if (splits) splits.replaceWith(wrap);
    else section.appendChild(wrap);
    splits = wrap;
  };
  const select = doc.createElement('select');
  select.className = 'split-select';
  select.setAttribute('aria-label', 'Split distance');
  select.innerHTML = choice.presets.map((p, i) => `<option value="${i}">${p.label}</option>`).join('');
  select.value = String(choice.index);
  select.addEventListener('change', () => {
    preset = choice.presets[+select.value];
    build();
  });
  // Boundaries do not depend on units; the cells do.
  units.subscribe(() => {
    if (splits) build();
  });
  if (!server) {
    // LS-3: no device laps: splits only, no switch.
    build();
    splits!.hidden = false;
    slot.appendChild(select);
    section.hidden = false;
    return;
  }
  const control = doc.createElement('span');
  control.className = 'laps-switch';
  control.innerHTML = '<button type="button" aria-pressed="true">Laps</button><button type="button" aria-pressed="false">Splits</button>';
  const [lapsButton, splitsButton] = control.children;
  const show = (onSplits: boolean): void => {
    if (onSplits && !splits) build();
    server.hidden = onSplits;
    select.hidden = !onSplits;
    if (splits) splits.hidden = !onSplits;
    lapsButton.setAttribute('aria-pressed', String(!onSplits));
    splitsButton.setAttribute('aria-pressed', String(onSplits));
  };
  lapsButton.addEventListener('click', () => show(false));
  splitsButton.addEventListener('click', () => show(true));
  select.hidden = true;
  slot.append(control, select);
}
