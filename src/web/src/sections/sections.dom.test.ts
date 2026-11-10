// @vitest-environment jsdom
// The sections chunk (F-42 MS-6, MS-7): separator ranges and the chart decorator, on a synthetic
// triathlon golden. jsdom has no layout: x is a fake scale and offsetWidth is stubbed.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../lib/datafile';
import { sectionRanges, sections } from '../lib/sections';
import { decorator, startSections } from '.';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  const r = g && decodeDataFile(g.bin);
  if (!r?.ok) throw new Error(`no golden ${name}`);
  return r.data;
}

afterEach(() => vi.restoreAllMocks());

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
    const look: { decorate?: unknown } = {};
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
