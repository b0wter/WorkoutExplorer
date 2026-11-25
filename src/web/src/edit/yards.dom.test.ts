// @vitest-environment jsdom
// F-72 YD-3, YD-8 (QA-72-r1-1): the pins lists (viewing and edit page) and the added-laps table
// pass the workout's sport, so a pool swim reads yards on an imperial page and km on a metric one.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../lib/datafile';
import { createCursorStore } from '../lib/cursor';
import { createSelectionStore } from '../lib/selection';
import { createUnitsStore } from '../lib/units';
import { startPins, type PinsView } from '../pins';
import { startPinEditor } from './pins';
import { createAddedLaps } from './laps';
import { startAddedTable } from './addedtable';

function golden(name: string): WorkoutData {
  const r = decodeDataFile(loadGolden().find((x) => x.name === name)!.bin);
  if (!r.ok) throw new Error(name);
  return r.data;
}
const swim = golden('pool-swim');
const mark = Math.floor(swim.sampleCount / 2);
const tick = (): Promise<void> => new Promise((r) => setTimeout(r, 0));
afterEach(() => {
  vi.unstubAllGlobals();
  document.body.innerHTML = '';
});

function env(units: 'metric' | 'imperial') {
  document.body.innerHTML = '<div class="pins-slot" data-url="/w/x/pins.0123456789abcdef.json" data-count="1"></div>';
  const slot = document.querySelector<HTMLElement>('.pins-slot')!;
  const run = (f: () => void): void => f();
  return { slot, cursor: createCursorStore(swim.sampleCount, run), selection: createSelectionStore(swim.sampleCount, run), units: createUnitsStore(units) };
}

describe('F-72 pins and added laps in yards', () => {
  it.each([['imperial', ' yd'], ['metric', ' km']] as const)('viewing pins list, %s', async (u, unit) => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify([{ type: 'danger', title: 'x', note: '', from: mark }]), { status: 200 })));
    const e = env(u);
    startPins({ ...e, data: swim, look: { decorator: () => {} }, map: () => {} });
    await tick();
    await tick();
    expect(e.slot.querySelector('.pin-row')?.textContent).toContain(unit);
  });

  it.each([['imperial', 'yd'], ['metric', 'km']] as const)('edit pins list ("at …"), %s', async (u, unit) => {
    const e = env(u);
    let view!: PinsView;
    startPins({ ...e, data: swim, look: { decorator: () => {} }, map: () => {}, edit: { onpins: (v) => (view = v) } });
    await tick();
    const item = { label: () => 'Add lap', short: () => 'Lap', state: () => ({ off: false, why: '' }), run: () => {} };
    startPinEditor({ slot: e.slot, view, data: swim, startS: Date.parse(swim.meta.start_time) / 1000, shade: () => ({}) as never, onedit: () => {}, items: [item] }, []);
    e.cursor.set(mark);
    e.slot.querySelector<HTMLButtonElement>('.plus-button')!.click();
    const why = [...e.slot.querySelectorAll('.plus-row .edit-small')].map((x) => x.textContent ?? '');
    expect(why.some((w) => w.endsWith(' ' + unit))).toBe(true);
  });

  it.each([['imperial', 'yd'], ['metric', 'km']] as const)('added-laps table, %s', (u, unit) => {
    const e = env(u);
    const startS = Date.parse(swim.meta.start_time) / 1000;
    // The first sample whose boundary the laps accept (not an existing lap end).
    let added!: ReturnType<typeof createAddedLaps>;
    for (let k = 3; k < swim.sampleCount; k++) {
      added = createAddedLaps(swim, startS, [new Date((startS + swim.time[k]) * 1000).toISOString()], false, () => [0, swim.sampleCount - 1], () => {});
      if (added.rows().some(Boolean)) break;
    }
    startAddedTable(e.slot, swim, added, e.units).refresh();
    expect(added.rows().filter(Boolean).length).toBeGreaterThan(0);
    const cells = [...e.slot.querySelectorAll('.added-row .pin-d')].map((x) => x.textContent ?? '');
    expect(cells.some((c) => c.endsWith(' ' + unit))).toBe(true);
  });
});
