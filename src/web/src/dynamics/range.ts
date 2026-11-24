// The running dynamics' cells of the range panel (F-68 XS-9, AP-7): the mean of each present stream
// over the range, by prefix sums of value and count (missing samples drop out), so a query is O(1).
// Lazy with the chunk: the entry's range code knows no per-stream rule. The cells are appended to the
// panel's grid before its action row, from the same spec list as the charts and the readout.

import { MISSING_TEXT } from '../lib/charts';
import { onLanguage } from '../lib/i18n';
import type { WorkoutData } from '../lib/datafile';
import type { Units } from '../lib/format';
import type { SelectionStore } from '../lib/selection';
import { dynamicsSpecs } from './specs';

/** Mean of `[a, b]` from the prefix arrays of count and sum (length N + 1), NaN without a sample. */
export function prefixMean(count: Float64Array, sum: Float64Array, a: number, b: number): number {
  const c = count[b + 1] - count[a];
  return c > 0 ? (sum[b + 1] - sum[a]) / c : Number.NaN;
}

export function rangeCells(root: HTMLElement, data: WorkoutData, selection: SelectionStore, units: Units) {
  const doc = root.ownerDocument;
  const rows = dynamicsSpecs(units)
    .filter((s) => data.metrics[s.metric]?.some((v) => v === v))
    .map((s) => {
      const values = data.metrics[s.metric] as Float64Array;
      const sum = new Float64Array(values.length + 1);
      const count = new Float64Array(values.length + 1);
      values.forEach((v, i) => {
        const ok = v === v;
        sum[i + 1] = sum[i] + (ok ? v : 0);
        count[i + 1] = count[i] + (ok ? 1 : 0);
      });
      const el = (tag: string, cls: string, parent?: HTMLElement): HTMLElement => {
        const e = doc.createElement(tag);
        e.className = cls;
        parent?.append(e);
        return e;
      };
      const cell = el('div', 'cell');
      const label = el('div', 'label', cell);
      const val = el('div', 'val', cell);
      const value = el('span', 'value', val);
      val.append('\u00a0');
      const unit = el('span', 'unit', val);
      root.querySelector('.actions')?.before(cell);
      return { metric: s.metric, sum, count, cell, label, value, unit };
    });
  let current = units;
  const write = (): void => {
    const [a, b] = [selection.start(), selection.end()];
    for (const s of dynamicsSpecs(current)) {
      const r = rows.find((x) => x.metric === s.metric);
      if (!r) continue;
      const v = a < 0 ? Number.NaN : s.toDisplay(prefixMean(r.count, r.sum, a, b));
      r.label.textContent = s.short ?? s.label;
      r.unit.textContent = s.unit;
      r.value.textContent = v === v ? s.format(v) : MISSING_TEXT;
    }
  };
  const off = [selection.subscribe(write), onLanguage(write)];
  return {
    update(u: Units): void {
      current = u;
      write();
    },
    destroy(): void {
      off.forEach((f) => f());
      rows.forEach((r) => r.cell.remove());
    },
  };
}
