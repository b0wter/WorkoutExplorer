// @vitest-environment jsdom
// F-67 LP-10 (OQ-322): the range panel's speed reads in a section's unit for a range inside it,
// km/h across sections. Synthetic triathlon golden (swim 0–600, T1, ride 660–2460, T2, run 2520–).
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile } from '../../lib/datafile';
import { createSelectionStore } from '../../lib/selection';
import RangePanel from './RangePanel.svelte';

const g = loadGolden().find((x) => x.name === 'triathlon');
const r = g && decodeDataFile(g.bin);
if (!r?.ok) throw new Error('no golden triathlon');
const data = r.data;

let app: ReturnType<typeof mount> | null = null;
afterEach(() => {
  if (app) void unmount(app);
  app = null;
});

describe('RangePanel per leg (F-67 LP-10)', () => {
  it('a range inside one leg uses its unit; a range across legs km/h', () => {
    const target = document.createElement('div');
    const selection = createSelectionStore(data.sampleCount, (f) => f());
    app = mount(RangePanel, { target, props: { data, selection } });
    flushSync();
    const speed = (a: number, b: number) => {
      selection.set(a, b);
      flushSync();
      const cell = [...target.querySelectorAll('.cell')].find((c) => /^Avg (pace|speed)$/.test(c.querySelector('.label')?.textContent ?? ''));
      return [cell?.querySelector('.label')?.textContent, cell?.querySelector('.unit')?.textContent, cell?.querySelector('.value')?.textContent];
    };
    const run = speed(2600, 3000);
    expect(run.slice(0, 2)).toEqual(['Avg pace', '/km']);
    expect(run[2]).toMatch(/^\d+:\d\d$/);
    expect(speed(0, 600).slice(0, 2)).toEqual(['Avg pace', '/100 m']); // the swim leg's own heading range
    expect(speed(700, 2000).slice(0, 2)).toEqual(['Avg speed', 'km/h']);
    expect(speed(500, 3000).slice(0, 2)).toEqual(['Avg speed', 'km/h']); // across legs: the workout's
  });
});
