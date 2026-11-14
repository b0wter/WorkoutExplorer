// The Zones section (F-49 ZN-7, ZN-8; AP-10, AP-7). LAZY: reached only through the page glue's
// import(), after the charts' first paint: on the viewing page when the data file has zone values,
// on the edit page through the edit glue, which also passes its public window (the saved trim) and
// zone values (the saved sharer value first). Fills #zones-slot; recomputes on a selection flush
// only, never on a cursor move. Units do not apply (bpm and W).

import '../theme/zones.css';
import './zones.css';
import type { WorkoutData, ZoneKind, Zones } from '../lib/datafile';
import type { formatElapsed } from '../lib/format';
import type { SelectionStore } from '../lib/selection';
import { boundText, timeInZone, zoneStarts } from '../lib/zones';

/** What the section reads (ZN-7, F-50's "public window"): inclusive sample indices. */
export interface ZoneSource {
  zones(): Zones;
  window(): readonly [number, number];
}

const KINDS: readonly (readonly [ZoneKind, string, string])[] = [
  ['heart_rate', 'Heart rate', 'bpm'],
  ['power', 'Power', 'W'],
];

const POWER = ['grey', 'blue', 'green', 'yellow', 'orange', 'red', 'purple'];
const RAMP = ['blue', 'green', 'yellow', 'orange', 'red'];

/** Zone colour by position: the 7-colour power ramp for 7 zones, else the 5-colour ramp spread over n. */
const zoneColour = (z: number, n: number): string => (n === 7 ? POWER[z] : RAMP[n < 2 ? 0 : Math.round((z * 4) / (n - 1))]);

/** One kind's block: built once per zone values, updated in place. */
interface Block {
  readonly box: HTMLElement;
  readonly starts: readonly number[];
  readonly values: Float64Array;
  readonly out: Float64Array;
  readonly bars: HTMLElement[];
  readonly times: HTMLElement[];
  readonly shares: HTMLElement[];
}

/** The chunk's entry. Returns `refresh`, for the edit glue after a save (new window or values). */
export function startZones(root: ParentNode, data: WorkoutData, selection: SelectionStore, fmt: typeof formatElapsed, src?: ZoneSource): () => void {
  const slot = root.querySelector<HTMLElement>('#zones-slot');
  if (!slot) return () => {};
  const source: ZoneSource = src ?? { zones: () => data.meta.zones, window: () => [0, data.sampleCount - 1] };
  const doc = slot.ownerDocument;
  const el = (parent: Element, tag: string, cls = '', text = ''): HTMLElement => {
    const e = parent.appendChild(doc.createElement(tag));
    if (cls) e.className = cls;
    if (text) e.textContent = text;
    return e;
  };
  let blocks: Block[] = [];

  const build = (): void => {
    slot.replaceChildren();
    blocks = [];
    const section = el(slot, 'section', 'zones');
    el(section, 'h2', '', 'Zones');
    const zones = source.zones();
    for (const [kind, label, unit] of KINDS) {
      const entry = zones[kind];
      const values = data.metrics[kind];
      if (!entry || !values) continue; // a kind needs zone values and its stream (OQ-189)
      const starts = zoneStarts(kind, entry);
      const box = el(section, 'div', 'zones-kind');
      el(box, 'h3', '', label);
      // ZN-7: the caption says what the zones rest on; a basis alone means the default model, a
      // basis next to `from` only names the device's own value.
      const basis = `${kind === 'power' ? 'threshold power' : 'max heart rate'} ${entry.basis} ${unit}`;
      el(box, 'p', 'zones-src', entry.from ? 'Zones as recorded by the device' + (entry.basis ? ` (${basis})` : '') : 'Estimated from ' + basis);
      const block: Block = { box, starts, values, out: new Float64Array(starts.length + 1), bars: [], times: [], shares: [] };
      for (let z = starts.length; z >= 0; z--) { // highest zone on top
        const row = el(box, 'div', 'zone');
        row.dataset.k = kind;
        row.style.setProperty('--t', `var(--zone-${zoneColour(z, starts.length + 1)})`);
        el(row, 'span', 'zone-n', 'Z' + (z + 1));
        el(row, 'span', 'zone-r', boundText(starts, z, unit));
        block.bars[z] = el(el(row, 'span', 'zone-bar'), 'span');
        block.times[z] = el(row, 'span', 'zone-t');
        block.shares[z] = el(row, 'span', 'zone-p');
      }
      blocks.push(block);
    }
  };

  /** ZN-7: the public window, narrowed to the range; a kind without time there is not shown. */
  const update = (a: number, b: number): void => {
    const [lo, hi] = source.window();
    const x = a < 0 ? lo : Math.max(a, lo);
    const y = a < 0 ? hi : Math.min(b, hi);
    let shown = false;
    for (const k of blocks) {
      const total = x < y ? timeInZone(k.starts, k.values, data, x, y, k.out) : 0;
      k.box.hidden = total === 0;
      if (total === 0) continue;
      shown = true;
      for (let z = 0; z < k.out.length; z++) {
        const share = k.out[z] / total;
        k.bars[z].style.width = share * 100 + '%';
        k.times[z].textContent = fmt(k.out[z]);
        k.shares[z].textContent = Math.round(share * 100) + '%';
      }
    }
    slot.hidden = !shown;
  };

  build();
  selection.subscribe(update);
  return () => {
    build();
    update(selection.start(), selection.end());
  };
}
