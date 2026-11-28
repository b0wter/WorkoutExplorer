// @vitest-environment jsdom
// B-30 (F-67, F-72 YD-8): on a multisport file every place that prints a distance takes the unit of
// the leg it points at: yd on the swim, mi on the bike (imperial). One golden triathlon (swim, T1, bike, ...).
// Not covered here: RangePanel (RangePanel.yards.dom.test.ts), and the splits/efforts/colouring/map
// `meta.sport` uses, which are per-leg already or not distance-unit text (see BACKLOG B-30).
import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile } from '../lib/datafile';
import { createCursorStore } from '../lib/cursor';
import { createSelectionStore } from '../lib/selection';
import { createUnitsStore } from '../lib/units';
import { pageLaps } from '../lib/page/view';
import { startPins, type PinsView } from '../pins';
import { startPinEditor } from './pins';
import { createAddedLaps } from './laps';
import { startAddedTable } from './addedtable';
import CursorReadout from '../components/charts/CursorReadout.svelte';

const r = decodeDataFile(loadGolden().find((x) => x.name === 'triathlon')!.bin);
if (!r.ok) throw new Error('no golden triathlon');
const data = r.data;
const SWIM = 300;
const BIKE = 1000;
const tick = (): Promise<void> => new Promise((res) => setTimeout(res, 0));
const run = (f: () => void): void => f();
let app: ReturnType<typeof mount> | null = null;
afterEach(() => {
  if (app) void unmount(app);
  app = null;
  vi.unstubAllGlobals();
  document.body.innerHTML = '';
});

function env() {
  document.body.innerHTML = '<div class="pins-slot" data-url="/w/x/pins.0123456789abcdef.json" data-count="2"></div>';
  const slot = document.querySelector<HTMLElement>('.pins-slot')!;
  return { slot, cursor: createCursorStore(data.sampleCount, run), selection: createSelectionStore(data.sampleCount, run), units: createUnitsStore('imperial') };
}

describe('B-30 distance unit per leg, imperial', () => {
  it('readout and fullscreen strip (the same CursorReadout): view sport sets the distance unit', () => {
    const unitOf = (sport?: string): string | null | undefined => {
      const target = document.createElement('div');
      const cursor = createCursorStore(data.sampleCount, run);
      app = mount(CursorReadout, { target, props: { data, cursor, units: 'imperial', sport } });
      flushSync();
      const u = [...target.querySelectorAll('.cell')].find((c) => c.querySelector('.label')?.textContent === 'Distance')!.querySelector('.unit')?.textContent;
      void unmount(app);
      app = null;
      return u;
    };
    expect(unitOf()).toBe('mi');
    expect(unitOf('swimming')).toBe('yd');
    expect(unitOf('cycling')).toBe('mi');
  });

  it('viewing pins list', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify([{ type: 'danger', title: 'x', note: '', from: SWIM }, { type: 'danger', title: 'y', note: '', from: BIKE }]), { status: 200 })));
    const e = env();
    startPins({ ...e, data, look: { decorator: () => {} }, map: () => {} });
    await tick();
    await tick();
    const rows = [...e.slot.querySelectorAll('.pin-row')].map((x) => x.textContent ?? '');
    expect(rows.find((x) => x.includes(' yd'))).toBeTruthy();
    expect(rows.find((x) => x.includes(' mi'))).toBeTruthy();
  });

  it('edit pins "at ..." and added-laps table', async () => {
    const e = env();
    let view!: PinsView;
    startPins({ ...e, data, look: { decorator: () => {} }, map: () => {}, edit: { onpins: (v) => (view = v) } });
    await tick();
    const item = { label: () => 'Add lap', short: () => 'Lap', state: () => ({ off: false, why: '' }), run: () => {} };
    const startS = Date.parse(data.meta.start_time) / 1000;
    startPinEditor({ slot: e.slot, view, data, startS, shade: () => ({}) as never, onedit: () => {}, items: [item] }, []);
    const why = (i: number): string[] => {
      e.cursor.set(i);
      return [...e.slot.querySelectorAll('.plus-row .edit-small')].map((x) => x.textContent ?? '');
    };
    e.slot.querySelector<HTMLButtonElement>('.plus-button')!.click();
    expect(why(SWIM).some((w) => w.endsWith(' yd'))).toBe(true);
    expect(why(BIKE).some((w) => w.endsWith(' mi'))).toBe(true);

    const iso = (k: number): string => new Date((startS + data.time[k]) * 1000).toISOString();
    const added = createAddedLaps(data, startS, [iso(SWIM), iso(BIKE)], false, () => [0, data.sampleCount - 1], () => {});
    startAddedTable(e.slot, data, added, e.units).refresh();
    const cells = [...e.slot.querySelectorAll('.added-row .pin-d')].map((x) => x.textContent ?? '');
    expect(cells.some((c) => c.endsWith(' yd'))).toBe(true);
    expect(cells.some((c) => c.endsWith(' mi'))).toBe(true);
  });

  it('laps table: the swim leg block in yards, the bike leg in miles', () => {
    const legs = pageLaps(data, 'imperial')?.Legs ?? [];
    const head = (s: string): string => JSON.stringify(legs.find((l) => l.Sport === s)?.Columns);
    expect(head('swimming')).toMatch(/yd/);
    expect(head('cycling')).toMatch(/mi/);
  });
});
