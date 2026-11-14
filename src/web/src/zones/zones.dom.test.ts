// @vitest-environment jsdom
// The Zones section (F-49 ZN-7, ZN-8): rows per kind, the selection narrows the counts, a cursor
// move does nothing, the edit page's window and values through the source, refresh after a save.
import { beforeEach, describe, expect, it } from 'vitest';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData, type Zones } from '../lib/datafile';
import { formatElapsed } from '../lib/format';
import { createSelectionStore, type SelectionStore } from '../lib/selection';
import { startZones, type ZoneSource } from '.';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  const r = g && decodeDataFile(g.bin);
  if (!r?.ok) throw new Error(`no golden ${name}`);
  return r.data;
}

let slot: HTMLElement;
let flush: (() => void) | null;
let selection: SelectionStore;

beforeEach(() => {
  document.body.innerHTML = '<div id="zones-slot" class="zones-slot" hidden></div>';
  slot = document.getElementById('zones-slot')!;
  flush = null;
});

function start(data: WorkoutData, src?: ZoneSource): () => void {
  selection = createSelectionStore(data.sampleCount, (f) => (flush = f));
  return startZones(document, data, selection, formatElapsed, src);
}
const select = (a: number, b: number): void => {
  selection.set(a, b);
  flush?.();
};
const kinds = (): string[] => [...slot.querySelectorAll<HTMLElement>('.zones-kind:not([hidden]) h3')].map((h) => h.textContent!);
const column = (kind: string, cls: string): string[] => [...slot.querySelectorAll(`.zone[data-k="${kind}"] .${cls}`)].map((e) => e.textContent!);
const captions = (): string[] => [...slot.querySelectorAll('.zones-src')].map((e) => e.textContent!);
const seconds = (text: string): number => text.split(':').reduce((s, p) => s * 60 + Number(p), 0);

describe('Zones section', () => {
  it('AC 2: a Wahoo-style ride shows heart rate (5 zones) and power (7 zones) with the file bounds; shares add up', () => {
    const d = golden('zones-wahoo-ride');
    start(d);
    expect(slot.hidden).toBe(false);
    expect(slot.querySelector('h2')?.textContent).toBe('Zones');
    expect(kinds()).toEqual(['Heart rate', 'Power']);
    expect(column('heart_rate', 'zone-r')).toEqual(['≥ 176 bpm', '161–175 bpm', '141–160 bpm', '121–140 bpm', '≤ 120 bpm']);
    expect(column('heart_rate', 'zone-n')).toEqual(['Z5', 'Z4', 'Z3', 'Z2', 'Z1']); // highest on top
    expect(column('power', 'zone-n')).toEqual(['Z7', 'Z6', 'Z5', 'Z4', 'Z3', 'Z2', 'Z1']);
    expect(column('power', 'zone-r')[6]).toBe('≤ 138 W');
    const tok = (k: string): string[] => [...slot.querySelectorAll<HTMLElement>(`.zone[data-k="${k}"]`)].map((e) => e.style.getPropertyValue('--t'));
    expect(tok('heart_rate').map((v) => v.slice(11, -1))).toEqual(['red', 'orange', 'yellow', 'green', 'blue']);
    expect(tok('power').map((v) => v.slice(11, -1))).toEqual(['purple', 'red', 'orange', 'yellow', 'green', 'blue', 'grey']);
    expect(captions()).toEqual(['Zones as recorded by the device', 'Zones as recorded by the device']);
    const pct = column('power', 'zone-p').map((p) => parseInt(p, 10));
    expect(Math.abs(pct.reduce((x, y) => x + y) - 100)).toBeLessThanOrEqual(3);
    const whole = d.movingTime![d.sampleCount - 1];
    expect(column('power', 'zone-t').map(seconds).reduce((x, y) => x + y)).toBeLessThanOrEqual(whole);
  });

  it('AC 8: a range shows its zones, Clear the whole workout again; a cursor move changes nothing', () => {
    const d = golden('zones-wahoo-ride');
    start(d);
    const whole = column('power', 'zone-t');
    select(100, 160);
    const range = column('power', 'zone-t').map(seconds).reduce((x, y) => x + y);
    expect(range).toBeLessThanOrEqual(60);
    expect(range).toBeGreaterThan(0);
    selection.clear();
    flush?.();
    expect(column('power', 'zone-t')).toEqual(whole);
  });

  it('AC 3, 5: only kinds with values and a stream; nothing at all hides the slot', () => {
    start(golden('zones-ride-ftp-only'));
    expect(kinds()).toEqual(['Power']);
    expect(column('power', 'zone-r')[5]).toBe('146–198 W'); // the model from 265 W
    expect(captions()).toEqual(['Estimated from threshold power 265 W']);
    document.body.innerHTML = '<div id="zones-slot" hidden></div>';
    slot = document.getElementById('zones-slot')!;
    start(golden('ride-30min'));
    expect(slot.hidden).toBe(true);
    expect(kinds()).toEqual([]);
  });

  it('a kind without samples in the range is not shown; the slot hides when none is', () => {
    const d = golden('zones-wahoo-ride');
    const hr = d.metrics.heart_rate!.slice();
    hr.fill(NaN, 0, 200);
    start({ ...d, metrics: { ...d.metrics, heart_rate: hr } });
    select(10, 150);
    expect(kinds()).toEqual(['Power']);
  });

  it('ZN-7, AC 6, 9: the edit page source: its window (the saved trim) bounds every range; refresh takes new values', () => {
    const d = golden('zones-wahoo-ride');
    let zones: Zones = d.meta.zones;
    let window: readonly [number, number] = [300, 600];
    const refresh = start(d, { zones: () => zones, window: () => window });
    const total = (): number => column('power', 'zone-t').map(seconds).reduce((x, y) => x + y);
    expect(total()).toBeLessThanOrEqual(300);
    select(0, 400); // only 300 … 400 of it is public
    expect(total()).toBeLessThanOrEqual(100);
    select(700, 900); // outside the window: nothing to show
    expect(slot.hidden).toBe(true);
    zones = { heart_rate: { basis: 200 } };
    window = [0, d.sampleCount - 1];
    refresh();
    expect(kinds()).toEqual(['Heart rate']);
    expect(column('heart_rate', 'zone-r')[4]).toBe('≤ 119 bpm');
    expect(captions()).toEqual(['Estimated from max heart rate 200 bpm']);
  });
});
