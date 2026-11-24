// @vitest-environment jsdom
// F-68 XS-9, AP-7: the range panel's dynamics cells: means over present samples, from the chunk's spec list.
import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile, DYNAMICS_ORDER, type WorkoutData } from '../lib/datafile';
import { createSelectionStore } from '../lib/selection';
import { rangeCells } from './range';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  if (!g) throw new Error(name);
  const r = decodeDataFile(g.bin);
  if (!r.ok) throw new Error(r.error.message);
  return r.data;
}

function panel(): HTMLElement {
  const root = document.createElement('div');
  root.innerHTML = '<div class="cell">fixed</div><div class="actions"></div>';
  return root;
}

const texts = (root: HTMLElement): string[][] =>
  [...root.querySelectorAll('.cell')].slice(1).map((c) => ['.label', '.value', '.unit'].map((q) => c.querySelector(q)?.textContent ?? ''));

describe('rangeCells', () => {
  const d = golden('run-dynamics');
  const present = DYNAMICS_ORDER.filter((n) => d.metrics[n]?.some((v) => v === v));

  it('one cell per present stream before the actions, in XS-1 order; dashes without a range', () => {
    const root = panel();
    const sel = createSelectionStore(d.sampleCount, (f) => f());
    rangeCells(root, d, sel, 'metric');
    expect(present.length).toBeGreaterThan(0);
    expect(texts(root)).toHaveLength(present.length);
    expect(root.lastElementChild?.className).toBe('actions');
    expect(texts(root).every(([, v]) => v === '–')).toBe(true);
  });

  it('the value is the mean of the present samples in the range; imperial converts the unit and the value', () => {
    const root = panel();
    const sel = createSelectionStore(d.sampleCount, (f) => f());
    const cells = rangeCells(root, d, sel, 'metric');
    const [a, b] = [Math.floor(d.sampleCount / 4), Math.floor((d.sampleCount * 3) / 4)];
    sel.set(a, b);
    const mean = (n: (typeof DYNAMICS_ORDER)[number]): number => {
      const v = (d.metrics[n] as Float64Array).subarray(a, b + 1).filter((x) => x === x);
      return v.reduce((s, x) => s + x, 0) / v.length;
    };
    const metric = texts(root);
    const decimals = { vertical_oscillation: 1, ground_contact_time: 0, stride_length: 2, vertical_ratio: 1 };
    present.forEach((n, k) => expect(Number(metric[k][1])).toBeCloseTo(mean(n), decimals[n]));
    cells.update('imperial');
    const imperial = texts(root);
    present.forEach((n, k) => {
      if (n === 'vertical_oscillation') expect([imperial[k][2], Number(imperial[k][1])]).toEqual(['in', Number((mean(n) / 2.54).toFixed(1))]);
      if (n === 'stride_length') expect([imperial[k][2], Number(imperial[k][1])]).toEqual(['ft', Number((mean(n) * 3.28084).toFixed(2))]);
    });
  });

  it('destroy removes the cells and the listeners', () => {
    const root = panel();
    const sel = createSelectionStore(d.sampleCount, (f) => f());
    const cells = rangeCells(root, d, sel, 'metric');
    cells.destroy();
    expect(texts(root)).toHaveLength(0);
    sel.set(1, 5); // no listener left: nothing throws
  });
});
