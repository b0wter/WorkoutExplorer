// @vitest-environment jsdom
// The laps enhancer (F-18 design L-2 to L-7; AC 1, 3 to 9) with the splits chunk (F-46 LS-3 to
// LS-8), wired as the page glue does. The page is the edit page's components rendered from a view
// model built from a synthetic golden, so rows and data agree.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import ActivityPage from '../components/page/ActivityPage.svelte';
import { decodeDataFile, type Lap, type WorkoutData } from '../lib/datafile';
import { buildPageViewModel } from '../lib/page/view';
import { createSelectionStore, type SelectionStore } from '../lib/selection';
import { createUnitsStore, type UnitsStore } from '../lib/units';
import { loadGolden } from '../../test-support/golden.mjs';
import { enhanceLaps } from './laps';
import { enhanceUnits } from './enhance';
import { startSplits } from '../splits';
import { distanceAdvances, lapsGroup } from '../lib/page';
import { sectionRanges, sections } from '../lib/sections';
import { legWithoutLaps } from '../lib/page/samples';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  if (!g) throw new Error(`no golden ${name}`);
  const r = decodeDataFile(g.bin);
  if (!r.ok) throw new Error(r.error.message);
  return r.data;
}

function withLaps(data: WorkoutData, laps: Lap[]): WorkoutData {
  return { ...data, meta: { ...data.meta, laps } };
}

interface Page {
  readonly data: WorkoutData;
  readonly selection: SelectionStore;
  readonly units: UnitsStore;
  readonly section: HTMLElement | null;
  /** Runs the pending selection flush, like the next animation frame. */
  frame(): void;
  rows(selector?: string): HTMLTableRowElement[];
  /** The sections chunk's call (F-42 MS-6). */
  legs(pairs: readonly number[]): void;
  button(text: string): HTMLButtonElement | undefined;
  /** The split-distance drop-down, or null. */
  select(): HTMLSelectElement | null;
  /** Chooses a split distance by its label, as a viewer does. */
  choose(label: string): void;
}

let cleanup: (() => void) | null = null;
beforeEach(() => {
  window.scrollBy = () => {}; // jsdom has no layout; the L-7 test spies on it
});
afterEach(() => {
  cleanup?.();
  cleanup = null;
  vi.restoreAllMocks();
  localStorage.clear();
});

function open(data: WorkoutData, units: 'metric' | 'imperial' = 'metric', withUnits = false, enhanceWith = data): Page {
  const model = buildPageViewModel(data, { title: '', description: '', expiresAt: '2026-10-30T09:15:00Z', publicId: 'x', dataFile: 'data.0.bin' });
  const target = document.createElement('div');
  document.body.appendChild(target);
  const app = mount(ActivityPage, { target, props: { model, report: false } });
  flushSync();
  let pending: (() => void) | null = null;
  const selection = createSelectionStore(data.sampleCount, (f) => {
    pending = f;
  });
  const store = createUnitsStore(units);
  if (withUnits) enhanceUnits(target, data.meta, store);
  const laps = enhanceLaps(target, enhanceWith, selection);
  laps.taps();
  if (distanceAdvances(enhanceWith)) startSplits(target, enhanceWith, store, laps.splits, lapsGroup);
  cleanup = () => {
    unmount(app);
    target.remove();
  };
  return {
    data,
    selection,
    units: store,
    section: target.querySelector('section.laps'),
    frame() {
      const f = pending;
      pending = null;
      f?.();
    },
    legs: laps.legs,
    rows: (sel = '.laps-scroll:not(.splits)') => [...target.querySelectorAll<HTMLTableRowElement>(sel + ' tbody tr')],
    button: (text) => [...target.querySelectorAll<HTMLButtonElement>('.laps-switch button')].find((b) => b.textContent === text),
    select: () => target.querySelector<HTMLSelectElement>('#laps-slot select'),
    choose(label) {
      const sel = target.querySelector<HTMLSelectElement>('#laps-slot select')!;
      sel.value = [...sel.options].find((o) => o.text === label)!.value;
      sel.dispatchEvent(new Event('change'));
    },
  };
}

const ride = (): WorkoutData => golden('ride-30min');

describe('row taps (L-2, L-3; AC 7)', () => {
  it('a tap writes (start_index, end_index − 1) and marks the row; the mark follows the store', () => {
    const p = open(ride());
    const laps = p.data.meta.laps;
    expect(laps).toHaveLength(6);
    p.rows()[2].querySelector('td')?.click();
    expect([p.selection.start(), p.selection.end()]).toEqual([laps[2].start_index, laps[2].end_index - 1]);
    p.frame();
    expect(p.rows().map((r) => r.classList.contains('on'))).toEqual([false, false, true, false, false, false]);
    p.rows()[4].click();
    p.frame();
    expect([p.selection.start(), p.selection.end()]).toEqual([laps[4].start_index, laps[4].end_index - 1]);
    expect(p.rows().filter((r) => r.classList.contains('on'))).toEqual([p.rows()[4]]);
    // A drag or a clear elsewhere removes the mark.
    p.selection.set(10, 20);
    p.frame();
    expect(p.section?.querySelector('tr.on')).toBeNull();
    p.selection.set(laps[4].start_index, laps[4].end_index - 1);
    p.frame();
    expect(p.section?.querySelector('tr.on')).toBeNull(); // only taps mark (O(1) per flush)
  });

  it('a tap on the row already selected changes nothing; rows never clear', () => {
    const p = open(ride());
    p.rows()[1].click();
    p.frame();
    const set = vi.spyOn(p.selection, 'set');
    const clear = vi.spyOn(p.selection, 'clear');
    p.rows()[1].click();
    expect(set).not.toHaveBeenCalled();
    expect(clear).not.toHaveBeenCalled();
    expect(p.rows()[1].classList.contains('on')).toBe(true);
  });

  it('a lap of fewer than 2 samples is shown but writes nothing; the header row writes nothing', () => {
    const d = ride();
    const one = { ...d.meta.laps[0], start_index: 5, end_index: 6 };
    const p = open(withLaps(d, [one, ...d.meta.laps.slice(1)]));
    p.rows()[0].click();
    p.section?.querySelector('thead th')?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    expect(p.selection.start()).toBe(-1);
  });

  it('rows that do not match meta.laps are not selectable', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {});
    const d = ride();
    const p = open(d, 'metric', false, withLaps(d, d.meta.laps.slice(1)));
    p.rows()[0].click();
    expect(p.selection.start()).toBe(-1);
    expect(console.warn).toHaveBeenCalled();
  });
});

describe('the tapped row stays in place (L-7; AC 8)', () => {
  it('no range → range: the window scrolls by the row’s move, read after the slot listener', () => {
    const p = open(ride());
    const row = p.rows()[3];
    let top = 400;
    vi.spyOn(row, 'getBoundingClientRect').mockImplementation(() => ({ top }) as DOMRect);
    const scroll = vi.spyOn(window, 'scrollBy').mockImplementation(() => {});
    // The glue's slot listener (subscribed at load) shows the panel: the row moves down by 120.
    p.selection.subscribe((a) => {
      if (a >= 0) top = 520;
    });
    row.click();
    expect(scroll).not.toHaveBeenCalled();
    p.frame();
    expect(scroll).toHaveBeenCalledExactlyOnceWith(0, 120);
    // range → range: the panel is already there; nothing is read or scrolled.
    p.rows()[1].click();
    p.frame();
    expect(scroll).toHaveBeenCalledTimes(1);
  });
});

const dist = (rows: HTMLTableRowElement[]): string[] => rows.map((r) => r.cells[2].textContent ?? '');

describe('laps / splits switch (L-4 to L-6; F-46 LS-4, LS-7)', () => {
  it('a ride opens its splits on 5 km; 1 km gives 14 splits; imperial keeps 1 km in miles; the selection stays', () => {
    const p = open(ride(), 'metric', true);
    const server = p.section?.querySelector<HTMLElement>('.laps-scroll:not(.splits)');
    expect(p.button('Laps')?.getAttribute('aria-pressed')).toBe('true');
    expect(p.section?.querySelector('.splits')).toBeNull(); // never built at load
    expect(p.select()?.hidden).toBe(true); // drop-down in the splits view only
    p.button('Splits')?.click();
    expect(server?.hidden).toBe(true);
    expect(p.select()?.hidden).toBe(false);
    expect([...p.select()!.options].map((o) => o.text)).toEqual(['1 km', '2 km', '2.5 km', '5 km', '10 km']); // 13.46 km
    expect(p.select()?.selectedOptions[0].text).toBe('5 km');
    expect(p.rows('.splits')).toHaveLength(3);
    expect(dist(p.rows('.splits'))[2]).toBe('3.46');
    p.choose('1 km');
    const splits = p.rows('.splits');
    expect(splits).toHaveLength(14);
    // A split ends at the first sample at or beyond k km: about 7.5 m per sample here.
    expect(dist(splits).slice(0, 13).every((t) => t === '1.00' || t === '1.01')).toBe(true);
    expect(dist(splits)[13]).toBe('0.46');
    expect([...(p.section?.querySelectorAll('.splits th') ?? [])].map((t) => t.textContent)).toEqual([
      '#', 'Time', 'Distance (km)', 'Speed (km/h)', 'Gain (m)', 'HR (bpm)', 'Power (W)', 'Cadence (rpm)',
    ]);
    splits[13].click();
    p.frame();
    const range = [p.selection.start(), p.selection.end()];
    expect(range[1]).toBe(p.data.sampleCount - 1);
    expect(splits[13].classList.contains('on')).toBe(true);
    // AC 40: units keep the chosen distance; 1 km shows as 0.62 mi.
    p.units.set('imperial');
    const mi = p.rows('.splits');
    expect(mi).toHaveLength(14);
    expect(dist(mi).slice(0, 13).every((t) => t === '0.62' || t === '0.63')).toBe(true);
    expect(p.select()?.selectedOptions[0].text).toBe('1 km');
    expect([p.selection.start(), p.selection.end()]).toEqual(range);
    expect(p.section?.querySelector('tr.on')).toBeNull();
    expect(p.section?.querySelectorAll('.splits')).toHaveLength(1);
    // The server table is still rewritten in place (F-14), the splits table did not confuse it.
    expect([...(server?.querySelectorAll('th') ?? [])].map((t) => t.textContent)).toContain('Distance (mi)');
    expect(p.section?.querySelector<HTMLElement>('.splits')?.hidden).toBe(false);
    p.button('Laps')?.click();
    expect(server?.hidden).toBe(false);
    expect(p.select()?.hidden).toBe(true);
    expect(p.section?.querySelector<HTMLElement>('.splits')?.hidden).toBe(true);
  });

  it('AC 37: with a range selected, a new distance keeps the range and unmarks a split row; rows of the new table select', () => {
    const p = open(ride());
    p.button('Splits')?.click();
    p.rows('.splits')[1].click();
    p.frame();
    const range = [p.selection.start(), p.selection.end()];
    const set = vi.spyOn(p.selection, 'set');
    const clear = vi.spyOn(p.selection, 'clear');
    p.choose('10 km');
    p.frame();
    expect(set).not.toHaveBeenCalled();
    expect(clear).not.toHaveBeenCalled();
    expect([p.selection.start(), p.selection.end()]).toEqual(range);
    expect(p.section?.querySelector('tr.on')).toBeNull();
    expect(p.rows('.splits')).toHaveLength(2);
    p.rows('.splits')[1].click();
    expect(p.selection.end()).toBe(p.data.sampleCount - 1);
  });

  it('AC 3: one device lap shows one row, and the splits still work', () => {
    const d = ride();
    const all = { ...d.meta.laps[0], start_index: 0, end_index: d.sampleCount, totals: d.meta.summary };
    const p = open(withLaps(d, [all]));
    expect(p.rows()).toHaveLength(1);
    p.button('Splits')?.click();
    expect(p.rows('.splits')).toHaveLength(3);
    p.rows()[0].click();
    expect([p.selection.start(), p.selection.end()]).toEqual([0, d.sampleCount - 1]);
  });

  it('AC 4: without distance there is the laps table but no switch', () => {
    const p = open(golden('indoor-no-distance-no-gps'));
    expect(p.rows().length).toBeGreaterThan(0);
    expect(p.section?.querySelector('.laps-switch')).toBeNull();
    expect(p.section?.querySelector('#laps-slot')?.childNodes).toHaveLength(0);
  });

  it('AC 4: a distance that never advances gives no switch', () => {
    const d = ride();
    const flat = Float64Array.from(d.metrics.distance ?? [], () => 1234.5);
    const p = open({ ...d, metrics: { ...d.metrics, distance: flat } });
    expect(p.section?.querySelector('.laps-switch')).toBeNull();
  });

  it('LS-3: distance without device laps: the module is revealed on its splits, no switch, the drop-down shown', () => {
    const p = open(golden('ride-no-laps'));
    expect(p.section?.hidden).toBe(false);
    expect(p.section?.querySelector('h2')?.textContent).toBe('Splits');
    expect(p.section?.querySelector('.laps-switch, .laps-none')).toBeNull();
    expect(p.select()?.hidden).toBe(false);
    expect(p.select()?.selectedOptions[0].text).toBe('5 km');
    expect(p.rows('.splits').length).toBeGreaterThan(0);
    p.rows('.splits')[0].click();
    expect(p.selection.start()).toBe(0);
  });

  it('AC 9, AC 38: every load starts on laps and the default; nothing is stored', () => {
    const d = ride();
    const p = open(d);
    p.button('Splits')?.click();
    p.choose('1 km');
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
    expect(location.hash).toBe('');
    cleanup?.();
    const q = open(d);
    expect(q.button('Laps')?.getAttribute('aria-pressed')).toBe('true');
    expect(q.section?.querySelector('.splits')).toBeNull();
    expect(q.select()?.selectedOptions[0].text).toBe('5 km');
  });
});

describe('pool splits (LS-6; AC 36)', () => {
  it('a 33 m pool (32.5 m recorded) splits every 3 lengths as "100 m", in metres in both unit systems', () => {
    const d = golden('pool-swim-33m');
    expect(d.meta.summary.pool_length_m).toBe(33);
    const p = open(d, 'imperial');
    p.button('Splits')?.click();
    expect([...p.select()!.options].map((o) => o.text)).toEqual(['100 m', '200 m', '400 m', '500 m', '1000 m']);
    expect(p.select()?.selectedOptions[0].text).toBe('100 m');
    const heads = [...(p.section?.querySelectorAll('.splits th') ?? [])].map((t) => t.textContent);
    expect(heads[2]).toBe('Distance (m)');
    const rows = p.rows('.splits');
    const full = dist(rows).slice(0, -1);
    expect(full.every((t) => t === '100')).toBe(true);
    expect(rows.length).toBe(Math.ceil(d.meta.summary.lengths! / 3));
    // Each split spans 3 recorded lengths in the stream.
    const stream = d.metrics.distance!;
    const pairs = rows.slice(0, -1).map((r) => {
      r.click();
      return stream[p.selection.end()] - stream[p.selection.start()];
    });
    expect(pairs.every((m) => Math.abs(m - 3 * d.meta.summary.pool_length_recorded_m!) < 1)).toBe(true);
  });

  it('a 25 yd pool splits by 100 yd, 4 lengths, in yards on a metric page', () => {
    const p = open(golden('pool-swim-25yd'));
    p.button('Splits')?.click();
    expect(p.select()?.selectedOptions[0].text).toBe('100 yd');
    expect(p.section?.querySelectorAll('.splits th')[2].textContent).toBe('Distance (yd)');
    expect(dist(p.rows('.splits')).slice(0, -1).every((t) => t === '100')).toBe(true);
  });
});

describe('transition laps (AC 6, L-1)', () => {
  it('a lap in a transition shows "–" for speed, cadence and power; its other columns are filled', () => {
    const p = open(golden('triathlon'));
    const heads = [...(p.section?.querySelectorAll('thead th') ?? [])].map((t) => t.getAttribute('data-k'));
    const col = (k: string): number => heads.indexOf(k);
    const transitions = p.rows().filter((r) => r.cells[col('avg_speed')]?.textContent === '–');
    expect(transitions.length).toBeGreaterThan(0);
    for (const r of transitions) {
      expect(r.cells[col('avg_power')].textContent).toBe('–');
      expect(r.cells[col('avg_cadence')].textContent).toBe('–');
      for (const k of ['elapsed_time', 'distance', 'avg_heart_rate']) expect(r.cells[col(k)].textContent).not.toBe('–');
    }
  });
});

describe('separator rows (F-42 MS-6; AC 2, AC 3)', () => {
  const names = (p: Page): string[] => p.rows().map((r) => (r.className === 'laps-section' ? r.textContent ?? '' : '#' + r.cells[0].textContent));

  it('one before each leg\'s laps, a leg without laps alone; units rewrite the lap rows only', () => {
    const warn = vi.spyOn(console, 'warn');
    const p = open(legWithoutLaps(golden('triathlon')), 'imperial', true);
    expect(names(p)).toEqual(['Swim', '#1', 'T1', '#2', 'Ride', '#3', 'T2', 'Run', '#4']);
    expect(p.rows()[0].cells[0].colSpan).toBe(p.section?.querySelectorAll('thead th').length);
    expect([...(p.section?.querySelectorAll('thead th') ?? [])].map((t) => t.textContent)).toContain('Distance (mi)');
    expect(warn).not.toHaveBeenCalled();
  });

  it('a separator selects nothing until the chunk gives its range; then its leg, marked like a lap', () => {
    const p = open(golden('triathlon'));
    const secs = sections(p.data.meta);
    if (!secs) throw new Error('no sections');
    const sep = p.rows().filter((r) => r.className === 'laps-section');
    expect(sep).toHaveLength(5);
    // QA-42-r1-2, r1-3: one tbody per leg, each headed by its separator.
    expect(sep.map((r) => [r.parentElement?.firstElementChild === r, r.cells[0].getAttribute('scope')])).toEqual(Array(5).fill([true, 'rowgroup']));
    expect(p.section?.querySelectorAll('.laps-scroll tbody')).toHaveLength(5);
    sep[2].click();
    expect(p.selection.start()).toBe(-1);
    expect(p.section?.classList.contains('legs')).toBe(false); // QA-42-r1-4: no pointer before the ranges
    p.legs(sectionRanges(secs, p.data.sampleCount));
    expect(p.section?.classList.contains('legs')).toBe(true);
    sep[2].querySelector('th')?.click();
    expect([p.selection.start(), p.selection.end()]).toEqual([secs[2].start, secs[3].start]);
    p.frame();
    expect(sep[2].classList.contains('on')).toBe(true);
    sep[4].click();
    expect([p.selection.start(), p.selection.end()]).toEqual([secs[4].start, p.data.sampleCount - 1]);
    // A lap row still maps to its lap by DOM order past the separators.
    const laps = p.data.meta.laps;
    const last = p.rows()[p.rows().length - 1];
    last.click();
    expect([p.selection.start(), p.selection.end()]).toEqual([laps[laps.length - 1].start_index, laps[laps.length - 1].end_index - 1]);
  });
});
