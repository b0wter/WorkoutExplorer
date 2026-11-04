// @vitest-environment jsdom
// F-33 fullscreen readout strip (architecture CR-1 to CR-4): a second CursorReadout on the same
// cursor store, hidden without a cursor and outside fullscreen, remounted per units value.
// Synthetic golden data only.

import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { loadGolden } from '../../test-support/golden.mjs';
import { readWebFile } from '../../test-support/source.mjs';
import { decodeDataFile, type WorkoutData } from '../lib/datafile';
import { createCursorStore, type CursorStore } from '../lib/cursor';
import { createUnitsStore } from '../lib/units';
import CursorReadout from '../components/charts/CursorReadout.svelte';
import { mountStrip } from './strip';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  const r = g && decodeDataFile(g.bin);
  if (!r || !r.ok) throw new Error(`no golden ${name}`);
  return r.data;
}

const cleanup: (() => void)[] = [];
afterEach(() => {
  while (cleanup.length) cleanup.pop()?.();
  document.documentElement.classList.remove('wx-fs');
});

/** A map box with the strip, plus the page's readout on the same store; flushes run on demand. */
function setup(data = golden('ride-4h')) {
  const queue: (() => void)[] = [];
  const cursor: CursorStore = createCursorStore(data.sampleCount, (f) => queue.push(f));
  const flush = (): void => {
    while (queue.length) queue.shift()?.();
    flushSync();
  };
  const units = createUnitsStore('metric');
  const page = document.createElement('div');
  page.id = 'charts-slot';
  const slot = document.createElement('div');
  slot.id = 'map-slot';
  const box = document.createElement('div');
  box.className = 'route-map';
  slot.append(box);
  document.body.append(page, slot);
  const inst = mount(CursorReadout, { target: page, props: { data, cursor, units: 'metric' } });
  mountStrip(box, data, cursor, units);
  flushSync();
  cleanup.push(() => {
    void unmount(inst);
    page.remove();
    slot.remove();
  });
  const strip = box.querySelector('.wx-strip') as HTMLElement;
  const text = (root: Element): string[] => [...root.querySelectorAll('.value')].map((e) => e.textContent ?? '');
  return { cursor, units, box, strip, page, flush, text };
}

describe('readout strip (F-33)', () => {
  it('lives in the map box and is hidden until the cursor is set; stays once set', () => {
    const { cursor, strip, box, flush } = setup();
    expect(strip.parentElement).toBe(box);
    expect(strip.hidden).toBe(true);
    cursor.set(500);
    flush();
    expect(strip.hidden).toBe(false);
    cursor.set(501);
    flush();
    expect(strip.hidden).toBe(false);
    cursor.set(null);
    flush();
    expect(strip.hidden).toBe(true);
  });

  it('shows the same values as the page readout, also after a units change', () => {
    const { cursor, units, strip, page, flush, text } = setup();
    cursor.set(1234);
    flush();
    expect(text(strip)).toEqual(text(page));
    expect(text(strip).length).toBeGreaterThan(2);
    units.set('imperial');
    flush();
    expect(strip.querySelectorAll('.readout')).toHaveLength(1); // the old instance is gone
    expect(strip.textContent).toContain('mi');
    // The new instance follows the cursor; the old one no longer writes.
    cursor.set(2000);
    flush();
    const imperial = text(strip);
    expect(imperial[0]).toBe(text(page)[0]); // elapsed time does not depend on units
    expect(imperial).not.toEqual(text(page));
  });

  it('page.css hides it outside fullscreen; inside it shows with a cursor', () => {
    const style = document.createElement('style');
    style.textContent = readWebFile('src/page/page.css');
    document.head.append(style);
    cleanup.push(() => style.remove());
    const { cursor, strip, flush } = setup();
    cursor.set(10);
    flush();
    expect(getComputedStyle(strip).display).toBe('none');
    document.documentElement.classList.add('wx-fs');
    expect(getComputedStyle(strip).display).not.toBe('none');
    expect(getComputedStyle(strip).pointerEvents).toBe('none');
  });
});
