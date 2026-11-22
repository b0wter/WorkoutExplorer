// @vitest-environment jsdom
// The pins chunk (F-56 PN-9 to PN-11; AC 37 to 41, 45): the fetched file, list, legend, map seam,
// chart marks and the one selection store, on the synthetic ride-30min golden. jsdom has no layout:
// the chart x scale is one px per sample.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../lib/datafile';
import { createCursorStore, type CursorStore } from '../lib/cursor';
import { createSelectionStore, type SelectionStore } from '../lib/selection';
import { createUnitsStore, type UnitsStore } from '../lib/units';
import type { ChartDecorator } from '../components/charts/uplot-options';
import { startPins, type PinMap, type PinsView } from '.';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  const r = g && decodeDataFile(g.bin);
  if (!r?.ok) throw new Error(`no golden ${name}`);
  return r.data;
}
const data = golden('ride-30min');
const HOSTILE = '<img src=x onerror="window.pwned=1">';
const FILE = [
  { type: 'danger', title: HOSTILE, note: '</script><b>x</b>\nline two', from: 100 },
  { type: 'difficult_section', title: '', note: '', from: 200, to: 400 },
  { type: 'water_food', title: 'Fountain', note: '', from: 50 },
];

let slot: HTMLElement;
let flushes: (() => void)[];
let cursor: CursorStore;
let selection: SelectionStore;
let units: UnitsStore;
let decorate: ChartDecorator | undefined;
let mapCb: ((m: PinMap) => void) | null;
let respond: () => Response;

beforeEach(() => {
  document.body.innerHTML = '<div class="pins-slot" data-url="/w/x/pins.0123456789abcdef.json" data-count="3"></div>';
  slot = document.querySelector('.pins-slot')!;
  flushes = [];
  const schedule = (f: () => void): void => void flushes.push(f);
  cursor = createCursorStore(data.sampleCount, schedule);
  selection = createSelectionStore(data.sampleCount, schedule);
  units = createUnitsStore('metric');
  decorate = undefined;
  mapCb = null;
  respond = () => new Response(JSON.stringify(FILE), { status: 200 });
  vi.stubGlobal('fetch', vi.fn(async () => respond()));
  vi.spyOn(console, 'warn').mockImplementation(() => {});
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const flush = (): void => {
  while (flushes.length) flushes.shift()!();
};
const settle = (): Promise<void> => new Promise((r) => setTimeout(r, 0));
async function start(edit?: { onpins?(v: PinsView): void }): Promise<PinsView> {
  const view = startPins({ slot, data, cursor, selection, units, look: { decorator: (_k, f) => (decorate = f) }, map: (cb) => (mapCb = cb), edit });
  await settle();
  await settle();
  return view;
}
const rows = (): string[] => [...slot.querySelectorAll('.pin-row')].map((r) => r.textContent!);
function fakeMap() {
  const markers: { el: HTMLElement; i: number; removed: boolean }[] = [];
  const controls: HTMLElement[] = [];
  const m: PinMap & { marked: unknown[] } = {
    marked: [],
    marker: (el, i) => {
      const rec = { el, i, removed: false };
      markers.push(rec);
      document.body.append(el);
      return () => {
        rec.removed = true;
        el.remove();
      };
    },
    marks: (r) => void m.marked.push(r),
    laps: () => {},
    control: (el) => void controls.push(el),
    sampleAt: () => 7,
  };
  return { m, markers, controls };
}

describe('viewing page (PN-9, PN-11)', () => {
  it('fetches the slot URL as given; list sorted, a hostile title and note stay text, an empty title is its type name', async () => {
    await start();
    expect(fetch).toHaveBeenCalledWith('/w/x/pins.0123456789abcdef.json');
    const r = rows();
    expect(r).toHaveLength(3);
    expect(slot.style.minHeight).toBe(''); // the reserve is released once the rows exist
    expect(r[0]).toMatch(/^Fountain/);
    expect(r[1]).toContain(HOSTILE);
    expect(r[2]).toMatch(/^Difficult section/);
    expect(document.querySelector('img')).toBeNull();
    expect(document.querySelector('b')).toBeNull();
    expect((window as { pwned?: number }).pwned).toBeUndefined();
    expect(slot.querySelector('.pin-note')?.textContent).toBe('</script><b>x</b>\nline two');
    // Distance from the start (km), a section from–to.
    expect(r[2]).toMatch(/\d\.\d\d–\d\.\d\d km$/);
    units.set('imperial');
    expect(rows()[2]).toMatch(/ mi$/);
  });

  it.each([
    ['a failed fetch', () => new Response('', { status: 404 })],
    ['an invalid file', () => new Response(JSON.stringify([{ ...FILE[0], lat: 1 }]), { status: 200 })],
    ['an index past the data file', () => new Response(JSON.stringify([{ ...FILE[0], from: data.sampleCount }]), { status: 200 })],
    ['no JSON', () => new Response('<html>', { status: 200 })],
  ])('%s: no pins, the slot released, one warning', async (_, r) => {
    respond = r;
    await start();
    expect(slot.hidden).toBe(true);
    expect(rows()).toEqual([]);
    expect(console.warn).toHaveBeenCalledTimes(1);
  });
});

describe('one selection in every view (PN-10)', () => {
  it('a point moves the cursor; a section selects its range, cursor at its start; again clears; another range deselects it', async () => {
    await start();
    const row = (k: number) => slot.querySelectorAll<HTMLButtonElement>('.pin-row')[k];
    row(0).click();
    expect(cursor.get()).toBe(50);
    expect(slot.querySelector('.pin-item.sel')).toBe(row(0).parentElement);
    row(2).click();
    expect(cursor.get()).toBe(200);
    flush();
    expect([selection.start(), selection.end()]).toEqual([200, 400]);
    row(2).click();
    flush();
    expect(selection.start()).toBe(-1);
    expect(slot.querySelector('.sel')).toBeNull();
    row(2).click();
    flush();
    selection.set(10, 20);
    flush();
    expect(slot.querySelector('.sel')).toBeNull();
  });

  it('map: markers at their samples, the section line, legend of present types only; a marker tap selects; the card follows', async () => {
    await start();
    const { m, markers, controls } = fakeMap();
    mapCb!(m);
    expect(markers.map((x) => x.i)).toEqual([50, 100, 200]);
    expect(markers[1].el.getAttribute('aria-label')).toBe('Danger: ' + HOSTILE);
    expect(m.marked.at(-1)).toEqual([[200, 400]]);
    const [legend, card] = controls;
    expect([...legend.querySelectorAll('button')].map((b) => b.getAttribute('aria-label'))).toEqual(['Danger', 'Difficult section', 'Water and food']);
    expect(card.hidden).toBe(true);
    markers[1].el.click();
    expect(cursor.get()).toBe(100);
    expect(card.hidden).toBe(false);
    expect(card.textContent).toContain(HOSTILE);
    expect(card.querySelector('img')).toBeNull();
    expect(markers[1].el.classList.contains('sel')).toBe(true);
    card.querySelector('button')!.click();
    expect(card.hidden).toBe(true);
  });

  it('legend: a hidden type leaves list, map and charts and drops its selection; all shown on start (OQ-239)', async () => {
    await start();
    const { m, markers, controls } = fakeMap();
    mapCb!(m);
    const over = document.body.appendChild(document.createElement('div'));
    decorate!(over, (i) => i);
    expect(over.querySelectorAll('.pin-x')).toHaveLength(2);
    expect((over.querySelector('.pin-band') as HTMLElement).style.width).toBe('200px');
    markers[1].el.click();
    const danger = controls[0].querySelector<HTMLButtonElement>('[aria-label="Danger"]')!;
    danger.click();
    expect(danger.getAttribute('aria-pressed')).toBe('false');
    expect(markers[1].el.hidden).toBe(true);
    expect(slot.querySelector<HTMLElement>('[data-pin="1"]')!.hidden).toBe(true);
    expect(over.querySelector<HTMLElement>('.pin-x[data-pin="1"]')!.hidden).toBe(true);
    expect(slot.querySelector('.sel')).toBeNull();
    decorate!(over, (i) => i); // a later layout keeps it hidden
    expect(over.querySelector<HTMLElement>('.pin-x[data-pin="1"]')!.hidden).toBe(true);
    danger.click();
    expect(markers[1].el.hidden).toBe(false);
  });

  it('nothing pin-related runs on a cursor move', async () => {
    await start();
    const before = slot.innerHTML;
    const d = decorate;
    cursor.set(5);
    flush();
    expect(slot.innerHTML).toBe(before);
    expect(decorate).toBe(d);
  });
});

describe('edit page seam (PN-12)', () => {
  it('no file is read; the view goes to the seam; a new range keeps a section selected', async () => {
    let view: PinsView | null = null;
    await start({ onpins: (v) => (view = v) });
    expect(fetch).not.toHaveBeenCalled();
    expect(view).not.toBeNull();
    view!.set([{ type: 'note', title: 'a', note: '', from: 10, to: 20, dim: 'Hidden by the start radius' }]);
    expect(rows()).toEqual(['aHidden by the start radius']);
    expect(slot.querySelector('.pin-item')!.classList.contains('dim')).toBe(true);
    view!.select(0);
    flush();
    selection.set(30, 40);
    flush();
    expect(view!.selected()).toBe(0);
  });
});
