// @vitest-environment jsdom
// The sections chunk (F-42 MS-6, MS-7): separator ranges and the chart decorator, on a synthetic
// triathlon golden. jsdom has no layout: x is a fake scale and offsetWidth is stubbed.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile, type Session, type WorkoutData } from '../lib/datafile';
import { sectionRanges, sections } from '../lib/sections';
import { createSelectionStore } from '../lib/selection';
import type { ChartView } from '../components/charts/uplot-options';
import { resetI18n, use } from '../lib/i18n';
import { decorator, pickable, startSections } from '.';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  const r = g && decodeDataFile(g.bin);
  if (!r?.ok) throw new Error(`no golden ${name}`);
  return r.data;
}

afterEach(() => {
  vi.restoreAllMocks();
  resetI18n();
});

describe('sections chunk', () => {
  const data = golden('triathlon');
  const secs = sections(data.meta) ?? [];
  const n = data.sampleCount;
  /** One px per sample. */
  const x = (i: number): number => i;
  const draw = (nameWidth = 0, scale = x): HTMLElement => {
    vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(nameWidth);
    const over = document.createElement('div');
    over.appendChild(document.createElement('span')); // the shade, drawn above
    decorator(secs, n)(over, scale);
    return over;
  };
  const marks = (over: HTMLElement, cls: string) => [...over.querySelectorAll<HTMLElement>('.' + cls)];

  it('AC 1: in every chart 4 lines at the leg starts after the first and every leg name, the first at sample 0', () => {
    expect(secs.map((g) => g.name)).toEqual(['Swim', 'T1', 'Ride', 'T2', 'Run']);
    const over = draw();
    expect(over.firstElementChild?.className).toBe('wx-legs');
    expect(marks(over, 'wx-leg-line').map((l) => l.style.transform)).toEqual(secs.slice(1).map((g) => `translateX(${g.start}px)`));
    const names = marks(over, 'wx-leg-name');
    expect(names.map((l) => l.textContent)).toEqual(['Swim', 'T1', 'Ride', 'T2', 'Run']);
    expect(names[0].style.transform).toBe('translateX(0px)');
  });

  it('a name wider than its leg is left out, its line stays; a redraw replaces the marks', () => {
    const t1 = secs[2].start - secs[1].start;
    const over = draw(t1 + 1);
    expect(marks(over, 'wx-leg-name').map((l) => l.textContent)).not.toContain('T1');
    expect(marks(over, 'wx-leg-line')).toHaveLength(4);
    decorator(secs, n)(over, x);
    expect(over.querySelectorAll('.wx-legs')).toHaveLength(1);
  });

  it('a first leg exactly as wide as its name keeps it: no line, so no gap is reserved (real triathlon, 390 px iPhone)', () => {
    // The real file's samples start 776 s into the swim: the swim is ~10 % of the axis, ~30 px of a
    // 294 px plot, and "Swim" at 11 px is ~27 px. The old check added 8 px of gaps and dropped it.
    const swim = secs[1].start;
    const names = marks(draw(27, (i) => (i * 30) / swim), 'wx-leg-name');
    expect(names[0].textContent).toBe('Swim');
    expect(names[0].style.transform).toBe('translateX(0px)');
  });

  it('startSections: separator ranges to the laps, the decorator to the look; nothing without sections', () => {
    const rows = vi.fn();
    const look: { decorate?: unknown; decorator(k: string, f: unknown): void } = { decorator: (k, f) => k === 'legs' && (look.decorate = f) };
    startSections(data, look as never, rows);
    expect(rows).toHaveBeenCalledWith(sectionRanges(secs, n));
    expect(typeof look.decorate).toBe('function');
    const single = golden('ride-30min');
    const none: { decorate?: unknown } = {};
    startSections(single, none as never, rows);
    expect(rows).toHaveBeenCalledTimes(1);
    expect(none.decorate).toBeUndefined();
  });
});

describe('F-67 leg picker', () => {
  const data = golden('triathlon');
  const n = data.sampleCount;
  const s = (sport: string, start_index: number): Session => ({ sport, sub_sport: '', start: start_index, end: start_index, start_index, end_index: start_index, totals: {} });
  const labels = (sessions: Session[], count = 100) => pickable(sections({ sessions }) ?? [], count).map((p) => p.label);

  it('LP-1: sport legs only, in order; a repeated sport is numbered (OQ-315, OQ-324); no leg with samples, none', () => {
    expect(pickable(sections(data.meta) ?? [], n).map((p) => [p.label, p.leg.start, p.leg.end])).toEqual([
      ['Swim', 0, 600],
      ['Ride', 660, 2460],
      ['Run', 2520, n - 1],
    ]);
    expect(labels([s('running', 0), s('transition', 10), s('cycling', 20), s('transition', 50), s('running', 60)])).toEqual(['Run 1', 'Ride', 'Run 2']);
    expect(labels([s('swimming', 0), s('running', 10), s('swimming', 20), s('running', 30)])).toEqual(['Swim 1', 'Run 1', 'Swim 2', 'Run 2']);
    expect(labels([s('cycling', 0), s('running', 10)])).toEqual(['Ride', 'Run']); // no transitions
    expect(labels([s('transition', 0), s('cycling', 200)], 100)).toEqual([]); // the ride has no samples
  });

  function start(withBar = true, ready = true) {
    const slot = document.createElement('div');
    if (withBar) slot.innerHTML = '<div class="toolbar"><span class="axis-toggle"></span></div>';
    const look: { view: ChartView; decorator(): void } = { view: null, decorator: () => {} };
    const selection = createSelectionStore(n, (f) => f());
    const fits: number[][] = [];
    const maps: ((m: { fit(a: number, b: number, still?: boolean): void }) => void)[] = [];
    startSections(data, look, () => {}, selection, (cb) => maps.push(cb), slot);
    const pick = slot.querySelector<HTMLSelectElement>('select.wx-leg-pick');
    const flush = (): void => maps.splice(0).forEach((cb) => cb({ fit: (a, b, still) => fits.push(still ? [a, b, 1] : [a, b]) }));
    const choose = (k: number): void => {
      pick!.selectedIndex = k;
      pick!.dispatchEvent(new Event('change'));
      if (ready) flush();
    };
    return { slot, look, selection, pick, choose, fits, flush };
  }

  it('AC 1, LP-2: a native select after the axis toggle, "Whole workout · Swim · Ride · Run", opening on the whole workout', () => {
    const { slot, pick, look } = start();
    expect(slot.querySelector('.toolbar')?.lastElementChild).toBe(pick);
    expect([...pick!.options].map((o) => o.text)).toEqual(['Whole workout', 'Swim', 'Ride', 'Run']);
    expect(pick!.selectedIndex).toBe(0);
    expect(pick!.getAttribute('aria-label')).toBe('Leg');
    expect(look.view).toBeNull();
  });

  it('no toolbar row (no distance stream): no picker (LP-2 deviation)', () => {
    expect(start(false).slot.querySelector('select')).toBeNull();
  });

  it('LP-3, LP-9: a pick sets the view and fits the map to the leg; Whole workout clears it and fits the route', () => {
    const { look, choose, fits, selection } = start();
    choose(3);
    expect(look.view).toEqual({ a: 2520, b: n - 1, sport: 'running' });
    expect(fits).toEqual([[2520, n - 1]]);
    expect(selection.start()).toBe(-1); // OQ-316: a view, not a range
    choose(0);
    expect(look.view).toBeNull();
    expect(fits).toEqual([[2520, n - 1], [-1, -1]]);
  });

  it('a leg switch clears the range (units differ per leg, F-72 YD-2a)', () => {
    const { choose, selection } = start();
    choose(1);
    selection.set(10, 50);
    choose(2);
    expect(selection.start()).toBe(-1);
  });

  it('LP-8 (OQ-319): a range inside the leg keeps it; one outside returns to the whole workout without moving the map; clearing keeps it', () => {
    const { look, choose, fits, selection, pick } = start();
    choose(2);
    selection.set(700, 2460);
    expect(look.view?.sport).toBe('cycling');
    selection.clear();
    expect(look.view?.sport).toBe('cycling');
    selection.set(100, 200);
    expect(look.view).toBeNull();
    expect(pick!.selectedIndex).toBe(0);
    expect(fits).toEqual([[660, 2460]]);
  });

  it('QA-67-r1-1: the automatic return resets the map target to the whole route without a move (LP-9), also before the map is ready', () => {
    const { choose, selection, fits, flush } = start(true, false);
    choose(2);
    selection.set(100, 200);
    expect(fits).toEqual([]);
    flush();
    expect(fits).toEqual([[660, 2460], [-1, -1, 1]]);
  });

  it('AC 4: a language switch keeps the chosen leg and view, with translated texts', () => {
    const { look, choose, pick } = start();
    choose(3);
    use('de', { wholeWorkout: 'Ganzes Training' }, document);
    expect(pick!.selectedIndex).toBe(3);
    expect(pick!.options[0].text).toBe('Ganzes Training');
    expect(look.view?.sport).toBe('running');
  });
});
