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
import { codeWord, distanceAdvances, lapsGroup } from '../lib/page';
import { formatElapsed, formatSpeedOrPace, isPaceSport, speedOrPaceUnit } from '../lib/format';
import { sectionRanges, sections } from '../lib/sections';
import { createRangeStats, emptyValues } from '../lib/range';
import { presetBounds, splitChoice } from '../lib/splits';
import { lapCrossing, legWithoutLaps, runRideRun } from '../lib/page/samples';

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
  const splits = distanceAdvances(enhanceWith);
  if (splits || enhanceWith.meta.lengths.length) startSplits(target, enhanceWith, store, laps, [lapsGroup, codeWord, formatElapsed, formatSpeedOrPace, speedOrPaceUnit, isPaceSport], splits);
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

  it('a tap on the row already selected clears the range and its mark', () => {
    const p = open(ride());
    p.rows()[1].click();
    p.frame();
    const set = vi.spyOn(p.selection, 'set');
    const clear = vi.spyOn(p.selection, 'clear');
    p.rows()[1].click();
    expect(set).not.toHaveBeenCalled();
    expect(clear).toHaveBeenCalledOnce();
    p.frame();
    expect(p.selection.start()).toBe(-1);
    expect(p.section?.querySelector('tr.on')).toBeNull();
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

  it('range → none (a second tap clears): scrolls by the row’s shift too', () => {
    const p = open(ride());
    const row = p.rows()[3];
    row.click();
    p.frame();
    let top = 520;
    vi.spyOn(row, 'getBoundingClientRect').mockImplementation(() => ({ top }) as DOMRect);
    const scroll = vi.spyOn(window, 'scrollBy').mockImplementation(() => {});
    p.selection.subscribe((a) => {
      if (a < 0) top = 400; // the panel disappears: the row moves up by 120
    });
    row.click();
    p.frame();
    expect(scroll).toHaveBeenCalledExactlyOnceWith(0, -120);
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

describe('lengths view (F-48 PL-8; AP-13)', () => {
  const cells = (r: HTMLTableRowElement): string[] => [...r.cells].map((c) => c.textContent ?? '');
  const heads = (p: Page): string[] => [...(p.section?.querySelectorAll('.lengths th') ?? [])].map((t) => t.textContent ?? '');

  it('pool-swim: Laps, Splits and Lengths, opens on laps; 40 numbered rows and a mid-swim Rest row with its time only', () => {
    const p = open(golden('pool-swim'));
    expect([...p.section!.querySelectorAll('.laps-switch button')].map((b) => b.textContent)).toEqual(['Laps', 'Splits', 'Lengths']);
    expect(p.button('Laps')?.getAttribute('aria-pressed')).toBe('true');
    expect(p.section?.querySelector('.lengths')).toBeNull(); // never built at load
    p.button('Lengths')?.click();
    expect(p.button('Lengths')?.getAttribute('aria-pressed')).toBe('true');
    expect(p.section?.querySelector<HTMLElement>('.laps-scroll:not(.splits)')?.hidden).toBe(true);
    expect(p.select()?.hidden).toBe(true);
    const rows = p.rows('.lengths');
    expect(rows).toHaveLength(41);
    expect(rows.map((r) => r.cells[0].textContent).filter((t) => t !== 'Rest')).toEqual(Array.from({ length: 40 }, (_, i) => String(i + 1)));
    expect(cells(rows[20])).toEqual(['Rest', '', '0:30', '', '', '']);
    expect(rows[21].cells[0].textContent).toBe('21'); // numbering continues after the rest
    expect(heads(p)).toEqual(['#', 'Stroke', 'Time', 'Pace (/100 m)', 'Strokes', 'SWOLF']);
    expect(cells(rows[0])).toEqual(['1', 'Freestyle', '0:30', '2:00', '18', '48']);
    p.button('Laps')?.click();
    expect(p.section?.querySelector<HTMLElement>('.lengths')?.hidden).toBe(true);
    expect(p.section?.querySelector<HTMLElement>('.laps-scroll:not(.splits)')?.hidden).toBe(false);
  });

  it('mixed strokes: SWOLF 36 for 0:21.6 and 14 strokes; drill without counts shows "–"; numbering continues after rests; imperial pace per 100 yd', () => {
    const p = open(golden('pool-swim-mixed'));
    p.button('Lengths')?.click();
    const rows = p.rows('.lengths');
    expect(rows).toHaveLength(16);
    expect(cells(rows[0])).toEqual(['1', 'Freestyle', '0:22', '1:26', '14', '36']);
    expect(cells(rows[4])).toEqual(['Rest', '', '0:30', '', '', '']);
    expect(rows[5].cells[0].textContent).toBe('5');
    expect(cells(rows[9]).slice(1)).toEqual(['Drill', '0:40', '2:40', '–', '–']);
    expect(rows.map((r) => r.cells[0].textContent).filter((t) => t !== 'Rest')).toHaveLength(14);
    p.units.set('imperial');
    expect(heads(p)[3]).toBe('Pace (/100 yd)');
    expect(p.rows('.lengths')[0].cells[3].textContent).toBe('1:19');
    expect(p.rows('.lengths')[0].cells[2].textContent).toBe('0:22');
  });

  it('a row tap selects the length (start_index … end_index − 1), rest rows too; switching views keeps the range', () => {
    const d = golden('pool-swim-mixed');
    const p = open(d);
    p.button('Lengths')?.click();
    p.rows('.lengths')[4].click();
    p.frame();
    expect([p.selection.start(), p.selection.end()]).toEqual([d.meta.lengths[4].start_index, d.meta.lengths[4].end_index - 1]);
    expect(p.rows('.lengths')[4].classList.contains('on')).toBe(true);
    p.rows('.lengths')[1].click();
    p.frame();
    expect([p.selection.start(), p.selection.end()]).toEqual([22, 44]);
    p.button('Splits')?.click();
    p.frame();
    expect([p.selection.start(), p.selection.end()]).toEqual([22, 44]);
    p.rows('.splits:not(.lengths)')[0].click();
    p.frame();
    expect(p.selection.start()).toBe(0);
    p.button('Lengths')?.click();
    p.rows('.lengths')[2].click();
    expect([p.selection.start(), p.selection.end()]).toEqual([44, 66]);
  });

  it('a truncated pool swim (UP-3): its recorded lengths, pace from the recorded avg_speed when there is no pool length, no error', () => {
    const p = open(golden('pool-swim-mixed-cut-80'));
    p.button('Lengths')?.click();
    const rows = p.rows('.lengths');
    expect(rows).toHaveLength(13);
    expect(cells(rows[0])).toEqual(['1', 'Freestyle', '0:22', '1:26', '14', '36']);
  });

  it('no stroke counts: no Strokes and SWOLF columns', () => {
    const d = golden('pool-swim-mixed');
    const lengths = d.meta.lengths.map(({ strokes: _, ...l }) => l);
    const p = open({ ...d, meta: { ...d.meta, lengths } });
    p.button('Lengths')?.click();
    expect(heads(p)).toEqual(['#', 'Stroke', 'Time', 'Pace (/100 m)']);
  });

  it('lengths without device laps or distance: the module is revealed on Lengths, no switch, no drop-down', () => {
    const d = golden('pool-swim-mixed');
    const metrics = { ...d.metrics };
    delete metrics.distance;
    const p = open({ ...d, metrics, meta: { ...d.meta, laps: [] } });
    expect(p.section?.hidden).toBe(false);
    expect(p.section?.querySelector('h2')?.textContent).toBe('Lengths');
    expect(p.section?.querySelector('.laps-switch')).toBeNull();
    expect(p.select()).toBeNull();
    expect(p.rows('.lengths')).toHaveLength(16);
  });

  it('rides and open-water swims get no Lengths view', () => {
    expect(open(golden('ride-30min')).button('Lengths')).toBeUndefined();
    cleanup?.();
    expect(open(golden('triathlon')).button('Lengths')).toBeUndefined();
  });
});

/** F-64: a view's leg blocks (the server's laps view, or the splits chunk's). */
const blocks = (p: Page, view: 'laps' | 'splits' = 'laps'): HTMLElement[] => [...(p.section?.querySelectorAll<HTMLElement>(`.laps-legs${view === 'laps' ? ':not(.splits)' : '.splits'} > .laps-leg`) ?? [])];
const legName = (b: HTMLElement): string => b.querySelector('h3')?.firstChild?.textContent ?? '';
const legRows = (b: HTMLElement): HTMLTableRowElement[] => [...b.querySelectorAll<HTMLTableRowElement>('tbody tr')];
const legSelect = (b: HTMLElement): HTMLSelectElement | null => b.querySelector('h3 select');
const pick = (b: HTMLElement, label: string): void => {
  const sel = legSelect(b)!;
  sel.value = [...sel.options].find((o) => o.text === label)!.value;
  sel.dispatchEvent(new Event('change', { bubbles: true }));
};

describe('transition laps (AC 6, L-1; F-64 LL-2)', () => {
  it('a transition is a table of its own: speed, cadence and power absent or "–"; its other columns are filled', () => {
    const p = open(golden('triathlon'));
    const t1 = blocks(p).find((b) => legName(b) === 'T1')!;
    const heads = [...t1.querySelectorAll('thead th')].map((t) => t.getAttribute('data-k'));
    const rows = legRows(t1);
    expect(rows).toHaveLength(1);
    for (const k of ['avg_speed', 'avg_power', 'avg_cadence']) if (heads.includes(k)) expect(rows[0].cells[heads.indexOf(k)].textContent).toBe('–');
    for (const k of ['elapsed_time', 'distance', 'avg_heart_rate']) expect(rows[0].cells[heads.indexOf(k)].textContent).not.toBe('–');
  });
});

describe('laps per leg (F-64 LL-2, LL-3; AC 1, 3, 8, 9)', () => {
  it('one table per leg, headed by its name, a leg without laps heading only; numbers restart; units rewrite each table by its sport', () => {
    const warn = vi.spyOn(console, 'warn');
    const p = open(legWithoutLaps(golden('triathlon')), 'imperial', true);
    expect(blocks(p).map((b) => [legName(b), legRows(b).map((r) => r.cells[0].textContent)])).toEqual([
      ['Swim', ['1']],
      ['T1', ['1']],
      ['Ride', ['1']],
      ['T2', []],
      ['Run', ['1']],
    ]);
    expect(p.section?.querySelector('.laps-section')).toBeNull();
    const speed = (name: string): string | null | undefined => blocks(p).find((b) => legName(b) === name)?.querySelector('th[data-k="avg_speed"]')?.textContent;
    expect([speed('Swim'), speed('Ride'), speed('Run')]).toEqual(['Pace (/100 yd)', 'Speed (mph)', 'Pace (/mi)']);
    expect(blocks(p)[2].querySelector('th[data-k="distance"]')?.textContent).toBe('Distance (mi)');
    expect(warn).not.toHaveBeenCalled();
  });

  it('rows map to their laps by DOM position across the tables; a heading selects nothing until the chunk gives its range, then its leg', () => {
    const p = open(golden('triathlon'));
    const secs = sections(p.data.meta);
    if (!secs) throw new Error('no sections');
    const laps = p.data.meta.laps;
    const ride = blocks(p)[2];
    legRows(ride)[0].click();
    expect([p.selection.start(), p.selection.end()]).toEqual([laps[2].start_index, laps[2].end_index - 1]);
    p.selection.clear();
    const hs = blocks(p).map((b) => b.querySelector<HTMLElement>('h3')!);
    expect(hs).toHaveLength(5);
    hs[2].click();
    expect(p.selection.start()).toBe(-1);
    expect(p.section?.classList.contains('legs')).toBe(false); // QA-42-r1-4: no pointer before the ranges
    p.legs(sectionRanges(secs, p.data.sampleCount));
    expect(p.section?.classList.contains('legs')).toBe(true);
    hs[2].click();
    expect([p.selection.start(), p.selection.end()]).toEqual([secs[2].start, secs[3].start]);
    p.frame();
    expect(hs[2].classList.contains('on')).toBe(true);
    hs[2].click(); // the selected leg again clears
    p.frame();
    expect([p.selection.start(), hs[2].classList.contains('on')]).toEqual([-1, false]);
    hs[4].click();
    expect([p.selection.start(), p.selection.end()]).toEqual([secs[4].start, p.data.sampleCount - 1]);
    const last = p.rows()[p.rows().length - 1];
    p.selection.clear(); // the last lap may equal the last leg, whose second tap would clear
    last.click();
    expect([p.selection.start(), p.selection.end()]).toEqual([laps[laps.length - 1].start_index, laps[laps.length - 1].end_index - 1]);
  });

  it('AC 3: a lap crossing a leg start is listed once, in the leg it starts in', () => {
    const p = open(lapCrossing(golden('triathlon')));
    expect(blocks(p).map((b) => legRows(b).length)).toEqual([1, 1, 1, 1, 1]);
    legRows(blocks(p)[0])[0].click();
    expect(p.selection.end()).toBe(629);
  });
});

describe('splits per leg (F-64 LL-4 to LL-7; AC 2, 4 to 7, 10, 13)', () => {
  const triathlon = (): Page => {
    const p = open(golden('triathlon'));
    p.button('Splits')?.click();
    return p;
  };
  const km = (rows: HTMLTableRowElement[], b: HTMLElement): number[] => {
    const j = [...b.querySelectorAll('thead th')].findIndex((t) => t.getAttribute('data-k') === 'distance');
    return rows.map((r) => Number(r.cells[j].textContent));
  };

  it('Swim, T1 and T2 as headings, Ride, Run in order; each sport leg preselects its AP-20 default; no workout-wide drop-down', () => {
    const p = triathlon();
    const t = p.data.time;
    const secs = sections(p.data.meta)!;
    const el = (k: number): string => formatElapsed(t[secs[k + 1].start] - t[secs[k].start]);
    expect(blocks(p, 'splits').map(legName)).toEqual(['Swim', `T1 · ${el(1)}`, 'Ride', `T2 · ${el(3)}`, 'Run']);
    expect(el(1)).toMatch(/^\d+:\d\d$/);
    expect(blocks(p, 'splits').map((b) => legSelect(b)?.selectedOptions[0].text ?? null)).toEqual(['100 m', null, '5 km', null, '1 km']);
    expect(blocks(p, 'splits').map((b) => !!b.querySelector('table'))).toEqual([true, false, true, false, true]);
    expect(p.select()).toBeNull();
    expect(p.button('Splits')?.getAttribute('aria-pressed')).toBe('true');
  });

  it('no split crosses a leg: each table runs from the leg start to its end, numbers restart, a partial last split, distances sum to the leg', () => {
    const p = triathlon();
    const secs = sections(p.data.meta)!;
    const ranges = sectionRanges(secs, p.data.sampleCount);
    const stats = createRangeStats(p.data);
    for (const k of [0, 2, 4]) {
      const b = blocks(p, 'splits')[k];
      const rows = legRows(b);
      expect(rows[0].cells[0].textContent).toBe('1');
      rows[0].click();
      expect(p.selection.start()).toBe(ranges[2 * k]);
      p.selection.clear();
      rows[rows.length - 1].click();
      expect(p.selection.end()).toBe(ranges[2 * k + 1]);
      p.selection.clear();
      const leg = stats.query(ranges[2 * k], ranges[2 * k + 1], emptyValues()).distance;
      // Shown in km to 2 decimals: within 5 m per row.
      const sum = km(rows, b).reduce((x, y) => x + y, 0) * 1000;
      expect(Math.abs(sum - leg)).toBeLessThan(rows.length * 5);
    }
  });

  it('a heading selects its leg, a transition heading its window; a second tap clears; the drop-down never selects', () => {
    const p = triathlon();
    const secs = sections(p.data.meta)!;
    const [swim, t1] = blocks(p, 'splits');
    t1.querySelector<HTMLElement>('h3')!.click();
    expect([p.selection.start(), p.selection.end()]).toEqual([secs[1].start, secs[2].start]);
    p.frame();
    expect(t1.querySelector('h3')?.classList.contains('on')).toBe(true);
    t1.querySelector<HTMLElement>('h3')!.click();
    expect(p.selection.start()).toBe(-1);
    legSelect(swim)!.click();
    expect(p.selection.start()).toBe(-1);
    swim.querySelector<HTMLElement>('h3')!.click();
    expect([p.selection.start(), p.selection.end()]).toEqual([0, secs[1].start]);
  });

  it('a leg\'s drop-down rebuilds its table only and keeps the range; a marked row elsewhere stays marked', () => {
    const p = triathlon();
    const [, , ride, , run] = blocks(p, 'splits');
    const rideTable = ride.querySelector('table');
    const runTable = run.querySelector('table');
    legRows(ride)[1].click();
    p.frame();
    const range = [p.selection.start(), p.selection.end()];
    const set = vi.spyOn(p.selection, 'set');
    pick(run, '400 m');
    p.frame();
    expect(run.querySelector('table')).not.toBe(runTable);
    expect(ride.querySelector('table')).toBe(rideTable);
    expect(legSelect(ride)?.selectedOptions[0].text).toBe('5 km');
    expect(legRows(run).length).toBeGreaterThan(9);
    expect([p.selection.start(), p.selection.end()]).toEqual(range);
    expect(legRows(ride)[1].classList.contains('on')).toBe(true);
    pick(ride, '1 km');
    p.frame();
    expect(set).not.toHaveBeenCalled();
    expect([p.selection.start(), p.selection.end()]).toEqual(range);
    expect(p.section?.querySelector('tr.on')).toBeNull();
    // Rows of the rebuilt tables select their own splits.
    legRows(run)[0].click();
    expect(p.selection.start()).toBe(sections(p.data.meta)![4].start);
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });

  it('units rebuild every leg with its own choice; pace or speed by the leg\'s sport (OQ-300)', () => {
    const p = triathlon();
    const [swim, , ride, , run] = blocks(p, 'splits');
    pick(run, '400 m');
    const n = legRows(run).length;
    p.units.set('imperial');
    expect(legRows(run)).toHaveLength(n);
    expect(legSelect(run)?.selectedOptions[0].text).toBe('400 m');
    const head = (b: HTMLElement): string | null | undefined => b.querySelector('th[data-k="avg_speed"]')?.textContent;
    expect([head(swim), head(ride), head(run)]).toEqual(['Pace (/100 yd)', 'Speed (mph)', 'Pace (/mi)']);
  });

  it('a comparison chart per sport leg table, last in its block, laps and splits alike; none for transitions or the section', () => {
    const p = open(golden('triathlon'));
    const charts = (bs: HTMLElement[]): boolean[] => bs.map((b) => b.lastElementChild?.classList.contains('compare') ?? false);
    expect(charts(blocks(p))).toEqual([true, false, true, false, true]);
    p.button('Splits')?.click();
    expect(charts(blocks(p, 'splits'))).toEqual([true, false, true, false, true]);
    expect(p.section?.querySelector(':scope > .compare')).toBeNull();
    const ride = blocks(p, 'splits')[2].querySelector<HTMLElement>('.compare')!;
    expect(ride.hidden).toBe(false);
    expect(ride.querySelectorAll('rect.bar')).toHaveLength(legRows(blocks(p, 'splits')[2]).length);
    expect(ride.querySelector('.compare-foot')?.firstChild?.textContent).toBe('5 km splits');
    // SC-7 fanned out: a row tap highlights its bar in its own chart only.
    legRows(blocks(p, 'splits')[2])[0].click();
    p.frame();
    expect(ride.querySelectorAll('rect.sel')).toHaveLength(1);
    expect(p.section?.querySelectorAll('rect.sel')).toHaveLength(1);
  });

  it('a sport leg without distance: heading and "No splits", no drop-down, no chart; its heading selects the leg', () => {
    const data = golden('triathlon');
    const secs = sections(data.meta)!;
    const dist = Float64Array.from(data.metrics.distance!);
    dist.fill(dist[secs[4].start], secs[4].start);
    const p = open({ ...data, metrics: { ...data.metrics, distance: dist } });
    p.button('Splits')?.click();
    const run = blocks(p, 'splits')[4];
    expect([legName(run), run.querySelector('.no-splits')?.textContent, legSelect(run), run.querySelector('table, .compare')]).toEqual(['Run', 'No splits', null, null]);
    run.querySelector<HTMLElement>('h3')!.click();
    expect([p.selection.start(), p.selection.end()]).toEqual([secs[4].start, p.data.sampleCount - 1]);
  });

  it('AC 6: a lap-swimming leg with one pool length splits by whole lengths; without, the open-water list', () => {
    const data = golden('triathlon');
    const lap = { ...data, meta: { ...data.meta, summary: { ...data.meta.summary, pool_length_m: 25, pool_length_recorded_m: 25 }, sessions: data.meta.sessions.map((s, i) => (i ? s : { ...s, sub_sport: 'lap_swimming' })) } };
    const options = (d: WorkoutData): string[] => {
      const p = open(d);
      p.button('Splits')?.click();
      const out = [...legSelect(blocks(p, 'splits')[0])!.options].map((o) => o.text);
      cleanup?.();
      return out;
    };
    expect(options(lap)).toEqual(['100 m', '200 m', '400 m', '500 m']);
    expect(options(golden('triathlon'))).toEqual(['50 m', '100 m', '200 m', '500 m']);
  });

  it('AC 7: run, ride, run gives three tables per view; the two runs are set independently', () => {
    const p = open(runRideRun(golden('triathlon')));
    expect(blocks(p).map(legName)).toEqual(['Run', 'Ride', 'Run']);
    p.button('Splits')?.click();
    const legs = blocks(p, 'splits');
    expect(legs.map(legName)).toEqual(['Run', 'Ride', 'Run']);
    expect(legs.map((b) => legSelect(b)?.selectedOptions[0].text)).toEqual(['1 km', '5 km', '1 km']);
    pick(legs[2], '400 m');
    expect(legs.map((b) => legSelect(b)?.selectedOptions[0].text)).toEqual(['1 km', '5 km', '400 m']);
  });

  it('a ride, ride workout (one sport) keeps its single tables and one drop-down', () => {
    const p = open(golden('chained-ride-ride'));
    expect(p.section?.querySelector('.laps-legs')).toBeNull();
    p.button('Splits')?.click();
    expect(p.select()?.selectedOptions[0].text).toBe('5 km');
    expect(p.section?.querySelector('.laps-legs')).toBeNull();
    expect(p.section?.querySelectorAll(':scope > .compare')).toHaveLength(1);
  });
});

describe('comparison chart (F-51; AP-23)', () => {
  const chart = (p: Page) => p.section?.querySelector<HTMLElement>('.compare') ?? null;
  const bars = (p: Page) => [...(chart(p)?.querySelectorAll<SVGRectElement>('rect.bar') ?? [])];
  const head = (p: Page) => [...(chart(p)?.querySelectorAll('.compare-stat') ?? [])].map((s) => s.textContent).join(' · ');
  const overlay = (p: Page) => chart(p)?.querySelector<HTMLSelectElement>('.compare-overlay:not([hidden]) select') ?? null;
  /** A click at slot i of n, with a 600 px wide plot (jsdom has no layout). */
  const tap = (p: Page, i: number, n: number) => {
    const svg = chart(p)!.querySelector('svg')!;
    svg.getBoundingClientRect = () => ({ left: 0, width: 600 }) as DOMRect;
    svg.dispatchEvent(new MouseEvent('click', { bubbles: true, clientX: (i + 0.5) * (600 / n) }));
  };
  const median = (xs: number[]) => {
    const s = [...xs].sort((a, b) => a - b);
    const m = s.length >> 1;
    return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2;
  };

  it('laps: one bar per lap, last in the section, header from the lap totals; units redraw it', () => {
    const p = open(ride(), 'metric', true);
    const laps = p.data.meta.laps;
    expect(chart(p)?.hidden).toBe(false);
    expect(p.section?.lastElementChild).toBe(chart(p));
    expect(bars(p)).toHaveLength(laps.length);
    const v = laps.map((l) => l.totals.avg_speed!);
    expect(head(p)).toBe('Median ' + formatSpeedOrPace('cycling', median(v)) + ' km/h · Best ' + formatSpeedOrPace('cycling', Math.max(...v)) + ' km/h');
    expect(chart(p)?.querySelector('.compare-foot')?.firstChild?.textContent).toBe('Laps');
    for (const b of bars(p)) {
      const d = (laps[+b.dataset.i!].totals.avg_speed! - median(v)) / median(v);
      expect(b.classList.contains(d > 0.03 ? 'faster' : d < -0.03 ? 'slower' : 'pace')).toBe(true);
    }
    p.units.set('imperial');
    expect(head(p)).toContain(' mph · Best ');
  });

  it('1 km splits: 14 bars, the partial last one left out of median and best; the chart stays last', () => {
    const p = open(ride());
    p.button('Splits')?.click();
    p.choose('1 km');
    expect(bars(p)).toHaveLength(14);
    expect(p.section?.lastElementChild).toBe(chart(p));
    const d = createRangeStats(p.data);
    const c = splitChoice(p.data.meta, d.distance![d.distance!.length - 1] - d.distance![0]);
    const b = presetBounds(d.distance!, c.presets.find((x) => x.label === '1 km')!, null);
    const out = emptyValues();
    const v = b.slice(1).map((e, i) => d.query(b[i], e, out).speed);
    expect(head(p)).toBe('Median ' + formatSpeedOrPace('cycling', median(v.slice(0, 13))) + ' km/h · Best ' + formatSpeedOrPace('cycling', Math.max(...v.slice(0, 13))) + ' km/h');
    expect(chart(p)?.querySelector('.compare-foot')?.firstChild?.textContent).toBe('1 km splits');
    p.button('Laps')?.click();
    expect(bars(p)).toHaveLength(p.data.meta.laps.length);
  });

  it('a bar tap selects as its row tap and highlights the bar; a row tap and a clear move the highlight', () => {
    const p = open(ride());
    const laps = p.data.meta.laps;
    tap(p, 2, laps.length);
    expect([p.selection.start(), p.selection.end()]).toEqual([laps[2].start_index, laps[2].end_index - 1]);
    p.frame();
    expect(p.rows()[2].classList.contains('on')).toBe(true);
    expect(bars(p).map((r) => r.classList.contains('sel'))).toEqual(laps.map((_, i) => i === 2));
    p.rows()[4].click();
    p.frame();
    expect(bars(p).map((r) => r.classList.contains('sel'))).toEqual(laps.map((_, i) => i === 4));
    tap(p, 4, laps.length); // the selected bar again clears
    p.frame();
    expect([p.selection.start(), bars(p).some((r) => r.classList.contains('sel'))]).toEqual([-1, false]);
    expect(p.section?.querySelector('tr.on')).toBeNull();
    p.rows()[4].click();
    p.frame();
    p.selection.set(10, 20);
    p.frame();
    expect(bars(p).some((r) => r.classList.contains('sel'))).toBe(false);
  });

  it('hover highlights the bar and its row, mouse only', () => {
    const p = open(ride());
    const svg = chart(p)!.querySelector('svg')!;
    svg.getBoundingClientRect = () => ({ left: 0, width: 600 }) as DOMRect;
    const move = (pointerType: string) => svg.dispatchEvent(Object.assign(new MouseEvent('pointermove', { clientX: 150 }), { pointerType }));
    move('touch');
    expect(p.section?.querySelector('.hover')).toBeNull();
    move('mouse');
    expect(p.rows()[1].classList.contains('hover')).toBe(true);
    expect(bars(p)[1].classList.contains('hover')).toBe(true);
    svg.dispatchEvent(new MouseEvent('pointerleave'));
    expect(p.section?.querySelector('.hover')).toBeNull();
  });

  it('overlay: only metrics the rows have, None first; one dot per bar; kept on a rebuild that offers it', () => {
    const p = open(ride());
    const sel = overlay(p)!;
    expect(sel.value).toBe('');
    expect([...sel.options].map((o) => o.text)).toEqual(['None', 'Heart rate', 'Power', 'Cadence', 'Elevation']);
    expect(chart(p)?.querySelector('circle')).toBeNull();
    sel.value = 'avg_heart_rate';
    sel.dispatchEvent(new Event('change'));
    expect(chart(p)?.querySelectorAll('.ov-heart_rate circle')).toHaveLength(p.data.meta.laps.filter((l) => l.totals.avg_heart_rate !== undefined).length);
    p.button('Splits')?.click();
    expect(overlay(p)?.value).toBe('avg_heart_rate');
    const bare = ride();
    const noHr = withLaps({ ...bare, metrics: { ...bare.metrics, altitude: undefined } }, bare.meta.laps.map((l) => ({ ...l, totals: { avg_speed: l.totals.avg_speed } })));
    cleanup?.();
    const q = open(noHr, 'metric', false);
    expect(overlay(q)).toBeNull(); // only None left: no drop-down (SC-6)
  });

  it('elevation overlay: gain per lap as the range panel shows it, no dot without valid altitude, m or ft', () => {
    const run = golden('run-30min');
    const n = run.sampleCount;
    const altitude = new Float64Array(n).fill(100);
    for (let i = 20; i < 30; i++) altitude[i] = NaN; // lap 3: no valid altitude, so no dot (never 0)
    // Cumulative gain: lap 1 (0…9) gains 9 m, lap 2 (10…19) 39 − 12 = 27 m.
    const elevationGain = new Float64Array(n).map((_, i) => (i < 10 ? i : i < 20 ? 9 + 3 * (i - 9) : 39));
    const v = [4.5, 4.4, 4.6];
    const laps = v.map((speed, i) => ({ ...run.meta.laps[0], totals: { avg_speed: speed }, start_index: i * 10, end_index: i * 10 + 10 }));
    const p = open(withLaps({ ...run, elevationGain, metrics: { ...run.metrics, altitude } }, laps), 'metric', false);
    const stats = createRangeStats(p.data);
    expect([0, 10].map((a) => stats.query(a, a + 9, emptyValues()).gain)).toEqual([9, 27]);
    const sel = overlay(p)!;
    sel.value = 'elevation';
    sel.dispatchEvent(new Event('change'));
    expect([...chart(p)!.querySelectorAll('.compare-axis.right span')].map((s) => s.textContent)).toEqual(['27 m', '', '9 m']);
    expect(chart(p)?.querySelectorAll('.ov-altitude circle')).toHaveLength(2);
    expect(chart(p)?.querySelector('.ov-altitude line.zero')).toBeNull();
    p.units.set('imperial');
    expect(chart(p)!.querySelector('.compare-axis.right span')?.textContent).toBe('89 ft');
  });

  it('QA-51-r1-2: an overlay change keeps the drop-down and its focus', () => {
    const p = open(ride());
    const sel = overlay(p)!;
    sel.focus();
    sel.value = 'avg_power';
    sel.dispatchEvent(new Event('change'));
    expect(overlay(p)).toBe(sel);
    expect(document.activeElement).toBe(sel);
    expect(chart(p)?.querySelectorAll('.ov-power circle').length).toBeGreaterThan(0);
  });

  it('SC-11: a standing rest lap is clipped with a break mark and named in the label; every axis label is a value', () => {
    const run = golden('run-30min');
    const v = [4.5, 2.5, 4.5, 2.5, 4.5, 0.3];
    // The run's samples with six laps of the given speeds (only the totals matter here).
    const laps = v.map((speed, i) => ({ ...run.meta.laps[0], totals: { avg_speed: speed }, start_index: i * 10, end_index: i * 10 + 10 }));
    const p = open(withLaps(run, laps));
    expect(chart(p)?.querySelectorAll('line.cut')).toHaveLength(1);
    expect(chart(p)?.querySelector('svg')?.getAttribute('aria-label')).toMatch(/, Laps\. Lap 6 beyond the scale$/);
    expect([...chart(p)!.querySelectorAll('.compare-axis:not(.right) span')].every((s) => /^\d+:\d\d$/.test(s.textContent ?? ''))).toBe(true);
  });

  it('AC 8: a trimmed file shows its remaining rows only, as its table', () => {
    const p = open(withLaps(ride(), ride().meta.laps.slice(1, 5)));
    expect(bars(p)).toHaveLength(4);
    expect([...chart(p)!.querySelectorAll('text')].map((t) => t.textContent)).toEqual(p.rows().map((r) => r.cells[0].textContent));
  });

  it('AC 9: a truncated file (UP-3) draws its chart without error', () => {
    const err = vi.spyOn(console, 'error');
    const warn = vi.spyOn(console, 'warn');
    const p = open(golden('ride-30min-cut-60'));
    p.button('Splits')?.click();
    expect(chart(p)?.hidden).toBe(false);
    expect(bars(p).length).toBeGreaterThan(1);
    expect(err).not.toHaveBeenCalled();
    expect(warn).not.toHaveBeenCalled();
  });

  it('run: pace in the header, faster bars point up', () => {
    const p = open(golden('run-30min'));
    if (p.button('Splits')) p.button('Splits')!.click();
    expect(head(p)).toMatch(/^Median \d+:\d\d \/km · Best \d+:\d\d \/km$/);
    for (const b of bars(p).filter((r) => r.classList.contains('faster'))) expect(+b.getAttribute('y')! + +b.getAttribute('height')!).toBeCloseTo(55, 5);
  });

  it('pool swim lengths: one bar per active length, none for rest; no overlay', () => {
    const p = open(golden('pool-swim'));
    p.button('Lengths')?.click();
    expect(bars(p)).toHaveLength(40);
    expect(chart(p)?.querySelectorAll('text')[0].textContent).toBe('1');
    expect(overlay(p)).toBeNull();
    expect(chart(p)?.querySelector('.compare-foot')?.firstChild?.textContent).toBe('Lengths');
  });

  it('a lap without speed keeps an empty slot; fewer than 2 bars: no chart; multisport: none under the section', () => {
    const laps = ride().meta.laps;
    const p = open(withLaps(ride(), laps.map((l, i) => (i === 1 ? { ...l, totals: { ...l.totals, avg_speed: undefined } } : l))));
    expect(bars(p).map((b) => b.dataset.i)).toEqual(['0', '2', '3', '4', '5']);
    cleanup?.();
    const q = open(withLaps(ride(), laps.map((l, i) => (i ? { ...l, totals: {} } : l))));
    expect(chart(q)?.hidden).toBe(true);
    cleanup?.();
    const r = open(golden('triathlon'));
    expect(r.section?.querySelector(':scope > .compare')).toBeNull(); // F-64 LL-7: charts per leg instead
  });
});
