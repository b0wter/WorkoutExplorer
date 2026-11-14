// @vitest-environment jsdom
// The Best efforts section (F-50 EF-7 to EF-9): tables, the curve after the slices, row taps select
// the covering range and mark the row, units rewrite pace texts only, no section without rows.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../lib/datafile';
import { createSelectionStore, type SelectionStore } from '../lib/selection';
import { createUnitsStore, type UnitsStore } from '../lib/units';
import { startEfforts } from '.';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  const r = g && decodeDataFile(g.bin);
  if (!r?.ok) throw new Error(`no golden ${name}`);
  return r.data;
}

let slot: HTMLElement;
let flush: (() => void) | null;
let selection: SelectionStore;
let units: UnitsStore;

beforeEach(() => {
  document.body.innerHTML = '<div id="efforts-slot" class="efforts-slot" hidden></div>';
  slot = document.getElementById('efforts-slot')!;
  flush = null;
  window.scrollBy = vi.fn(); // L-7's scroll; not in jsdom
});

function start(data: WorkoutData, src?: () => readonly [number, number]): () => void {
  selection = createSelectionStore(data.sampleCount, (f) => (flush = f));
  units = createUnitsStore('metric');
  return startEfforts(document, data, selection, units, window, src);
}
const tick = (): void => flush?.();
const rows = (): HTMLTableRowElement[] => [...slot.querySelectorAll<HTMLTableRowElement>('tbody tr')];
const column = (t: number, c: number): string[] => [...slot.querySelectorAll('table')[t].querySelectorAll(`tbody td:nth-child(${c})`)].map((e) => e.textContent!);

describe('Best efforts section', () => {
  it('AC 2: a ride with power: the power table, then the curve once computed; no distance table', async () => {
    start(golden('ride-30min'));
    expect(slot.hidden).toBe(false);
    expect(slot.querySelector('h2')?.textContent).toBe('Best efforts');
    expect(slot.querySelectorAll('table')).toHaveLength(1);
    expect([...slot.querySelectorAll('th')].map((h) => h.textContent)).toEqual(['Duration', 'Power', 'Start']);
    expect(column(0, 1)).toEqual(['5 s', '1 min', '5 min', '20 min']);
    expect(column(0, 2).every((t) => / W$/.test(t))).toBe(true);
    expect(slot.querySelector('svg')).toBeNull(); // the curve comes in slices, after the tables
    await vi.waitFor(() => expect(slot.querySelector('svg.curve polyline')).not.toBeNull());
    const svg = slot.querySelector('svg.curve')!;
    expect(svg.getAttribute('role')).toBe('img');
    expect(svg.getAttribute('aria-label')).toMatch(/^Power curve/);
    expect(svg.outerHTML).not.toMatch(/style=/);
    expect([...svg.querySelectorAll('text')].map((t) => t.textContent)).toEqual(expect.arrayContaining(['5 s', '1 min', '5 min', '20 min']));
  });

  it('AC 1, 6: a run: distance rows with pace; imperial rewrites pace per mile, names stay', () => {
    start(golden('run-30min'));
    expect([...slot.querySelectorAll('th')].map((h) => h.textContent)).toEqual(['Distance', 'Time', 'Pace', 'Start']);
    const names = column(0, 1);
    expect(names[0]).toBe('1 km');
    expect(column(0, 3).every((t) => t.endsWith(' /km'))).toBe(true);
    const table = slot.querySelector('table');
    units.set('imperial');
    expect(column(0, 1)).toEqual(names);
    expect(column(0, 3).every((t) => t.endsWith(' /mi'))).toBe(true);
    expect(slot.querySelector('table')).toBe(table); // rewritten in place
  });

  it('AC 2: no section for a ride without power, a swim, or without moving time', () => {
    const r = golden('ride-30min');
    start({ ...r, metrics: { ...r.metrics, power: undefined } });
    expect([slot.hidden, slot.childElementCount]).toEqual([true, 0]);
    start(golden('pool-swim'));
    expect(slot.hidden).toBe(true);
    start({ ...r, movingTime: undefined }); // EF-1: no moving time, no section
    expect(slot.hidden).toBe(true);
  });

  it('AC 4: a tap selects the covering range and marks the row; the same range again does nothing; another range unmarks', () => {
    const d = golden('ride-30min');
    start(d);
    const row = rows()[1];
    row.click();
    tick();
    const [a, b] = [selection.start(), selection.end()];
    expect(b - a).toBeGreaterThanOrEqual(60);
    expect(d.movingTime![b] - d.movingTime![a]).toBeGreaterThanOrEqual(60);
    expect(row.classList.contains('on')).toBe(true);
    flush = null;
    row.click();
    expect(flush).toBeNull(); // no write
    selection.set(0, 5);
    tick();
    expect(rows().some((r) => r.classList.contains('on'))).toBe(false);
  });

  it('EF-1: a window bounds the efforts; refresh recomputes only when it changed; a selection flush never rebuilds', () => {
    const d = golden('ride-30min');
    let w: readonly [number, number] = [0, 400];
    const refresh = start(d, () => w);
    expect(column(0, 1)).toEqual(['5 s', '1 min', '5 min']);
    const table = slot.querySelector('table');
    selection.set(10, 20);
    tick();
    refresh();
    expect(slot.querySelector('table')).toBe(table);
    w = [0, d.sampleCount - 1];
    refresh();
    expect(column(0, 1)).toEqual(['5 s', '1 min', '5 min', '20 min']);
  });

  it('AC 7: a truncated file shows efforts without error', () => {
    start(golden('triathlon-cut-90'));
    expect(slot.hidden).toBe(false);
  });
});
