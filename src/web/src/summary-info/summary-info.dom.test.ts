// @vitest-environment jsdom
// "About these numbers" (F-58; AP-24) through the viewing page's glue: topics per workout (AC 37,
// 38), the trim example, and the critical-lane check (AC 39, SE-1): the same metadata with the
// route whole, its ends hidden or fully hidden, and the matching route markup and a different
// title, gives byte-identical text. Synthetic data only (ADR-004).

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../lib/datafile';
import { STREAM_SPECS } from '../lib/datafile/format';
import { buildDataFile, type Stored } from '../lib/datafile/testing/build-file';
import { startPage } from '../page/start';
import { renderSample, type Fixture } from '../page/testing/fixture';
import { summaryItems } from '../lib/page';
import { openSummaryInfo, type Facts } from './index';

vi.mock('../components/charts/ChartStack.svelte', async () => await import('../page/testing/ChartStackStub.svelte'));
vi.mock('../components/map/RouteMap.svelte', async () => await import('../page/testing/RouteMapStub.svelte'));

const decode = (name: string): WorkoutData => {
  const g = loadGolden().find((x) => x.name === name);
  const r = g && decodeDataFile(g.bin.slice(0));
  if (!r?.ok) throw new Error('no golden ' + name);
  return r.data;
};
const scale = (name: string): number => STREAM_SPECS.find((s) => s.name === name)!.scale;
const ints = (a: Float64Array | undefined, name: string): Stored[] | undefined => a && Array.from(a, (v) => (Number.isNaN(v) ? null : Math.round(v / scale(name))));

type Route = 'whole' | 'ends' | 'none';
/** The golden's metadata and streams with a synthetic route: whole, both ends hidden (the first and
 *  last 30 samples without position) or fully hidden (no position streams), as F-16 H-1 writes them. */
function variant(d: WorkoutData, route: Route): ArrayBuffer {
  const n = d.sampleCount;
  const pos = (base: number): Stored[] => Array.from({ length: n }, (_, i) => (route === 'ends' && (i < 30 || i >= n - 30) ? null : base + i * 10));
  const metrics: Record<string, Stored[] | undefined> = { moving_time: ints(d.movingTime, 'moving_time'), elevation_gain: ints(d.elevationGain, 'elevation_gain') };
  for (const [k, a] of Object.entries(d.metrics)) metrics[k] = ints(a, k);
  for (const k of Object.keys(metrics)) if (!metrics[k]) delete metrics[k];
  const { sport, start_time, time_zone, summary, device, sessions, laps, lengths, zones } = d.meta;
  return buildDataFile({
    time: ints(d.time, 'time_offset')!,
    position: route === 'none' ? undefined : { latitude: pos(50_000_000), longitude: pos(7_000_000) },
    metrics,
    meta: { sport, start_time, time_zone, summary, device, sessions, laps, lengths, zones },
  });
}

let page: Fixture | null = null;
let bin: ArrayBuffer;

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
    this.open = false;
  };
  vi.stubGlobal('fetch', vi.fn(async () => new Response(bin.slice(0), { status: 200 })));
  vi.stubGlobal('requestAnimationFrame', () => 1);
  vi.spyOn(console, 'warn').mockImplementation(() => {});
  localStorage.setItem('wx.units', 'metric');
});

afterEach(() => {
  page?.remove();
  page = null;
  document.querySelectorAll('dialog').forEach((d) => d.remove());
  window.history.replaceState(null, '', '/');
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  localStorage.clear();
});

async function settle(): Promise<void> {
  for (let k = 0; k < 10; k++) {
    await new Promise((r) => setTimeout(r, 0));
    flushSync();
  }
}

/** The viewing page for `data` with `sample`'s markup; returns the dialog's text after a click on ⓘ. */
async function dialogText(data: ArrayBuffer, sample = 'ride-30min', route: Route = 'whole', title = ''): Promise<string> {
  bin = data;
  window.history.replaceState(null, '', '/w/x');
  page = renderSample(sample, route === 'whole' ? {} : { route: { Map: '', NoLocation: route === 'none' ? '' : 'yes', Hidden: route === 'none' ? 'yes' : '' } as never });
  if (title) document.querySelector('h1')!.textContent = title;
  const button = document.querySelector<HTMLButtonElement>('button[data-summary-info]')!;
  expect(button.classList.contains('ready')).toBe(false); // reserved, invisible until the data file is decoded (SE-5)
  await startPage(document);
  await settle();
  expect(button.classList.contains('ready')).toBe(true);
  button.click();
  await vi.waitFor(() => expect(document.querySelector<HTMLDialogElement>('dialog.summary-dlg')?.open).toBe(true));
  const text = [...document.querySelectorAll('dialog.summary-dlg .dlg-body p:not(.dlg-actions)')].map((p) => p.textContent).join('\n');
  page.remove();
  page = null;
  document.querySelectorAll('dialog').forEach((d) => d.remove());
  return text;
}

const topics = (text: string): string[] => text.split('\n').map((l) => l.slice(0, l.indexOf('. ')));

describe('AC 39 (SE-1): no trace of hiding on the viewing page', () => {
  it.each(['ride-30min', 'triathlon', 'pool-swim', 'zones-wahoo-ride', 'indoor-no-gps', 'run-30min'])('%s: whole, ends hidden and fully hidden give the same text', async (name) => {
    const d = decode(name);
    const whole = await dialogText(variant(d, 'whole'));
    expect(await dialogText(variant(d, 'ends'), 'ride-30min', 'ends', 'Ride near Köln')).toBe(whole);
    expect(await dialogText(variant(d, 'none'), 'ride-30min', 'none', 'Ride')).toBe(whole);
    expect(await dialogText(variant(d, 'whole'), 'ride-trimmed', 'none')).not.toBe(whole); // the check can see a difference
    // The hiding line is on every workout, without an example (SE-3, OQ-253).
    expect(whole).toMatch(/^Hidden start and end\. The sharer may hide the route near the start and end; that changes the map, never the totals\.$/m);
    expect(whole).not.toMatch(/hidden within|not hidden|recomputed:|dropped:/);
  });
});

describe('topics (AC 37, 38; SE-3)', () => {
  it('a ride: the always-topics and averages; no pool, transition, trim or zones line', async () => {
    const text = await dialogText(variant(decode('ride-30min'), 'whole'));
    expect(topics(text)).toEqual(['Totals', 'Moving time', 'Elapsed time', 'Averages and maximums', 'Speed spikes', 'Range panel and laps', 'Hidden start and end']);
    expect(text).not.toMatch(/transition/i);
    expect(text).toContain('This workout: 13.46 km in 29:59.');
    expect(text).toContain('This workout: 29:59 moving of 29:59 elapsed.');
  });

  it('a trimmed ride: the trim line with its example', async () => {
    const text = await dialogText(variant(decode('ride-30min'), 'whole'), 'ride-trimmed');
    expect(topics(text)).toContain('Trim');
    expect(text).toContain('Trim. After a trim, the totals are recomputed for the remaining part; calories are dropped.This workout: trimmed at the start and the end, totals recomputed.');
  });

  it('a triathlon: the transition sentences', async () => {
    const text = await dialogText(variant(decode('triathlon'), 'whole'));
    expect(text).toMatch(/including pauses, transitions and gaps between legs\.This workout: [0-9:]+, including 2 transitions\./);
    expect(text).toContain('Heart rate includes the transitions; speed, cadence and power do not.');
  });

  it('AP-24 (F-74): the speed spike line when the summary has a speed', async () => {
    expect(topics(await dialogText(variant(decode('ride-30min'), 'whole')))).toContain('Speed spikes');
  });

  it('a pool swim: the pool line; zones: the zones line', async () => {
    expect(topics(await dialogText(variant(decode('pool-swim'), 'whole')))).toContain('Pool swims');
    expect(topics(await dialogText(variant(decode('zones-wahoo-ride'), 'whole')))).toContain('Zones');
  });

  it('a failed chunk load: one line after the bar, never a second', async () => {
    const { enhanceSummaryInfo } = await import('../page/enhance');
    const { createUnitsStore } = await import('../lib/units');
    document.body.innerHTML = '<div class="howto-row"><button data-summary-info></button></div><section class="summary"></section>';
    enhanceSummaryInfo(document, createUnitsStore('metric'), () => ({ sport: 'cycling', summary: {}, sessions: [], lengths: [], zones: false }), () => Promise.reject(new Error('x')));
    const b = document.querySelector<HTMLButtonElement>('button')!;
    b.click();
    b.click();
    await settle();
    expect([...document.querySelectorAll('.report-fail')].map((e) => e.previousElementSibling?.className)).toEqual(['howto-row']);
    document.body.innerHTML = '';
  });
});

describe('owner facts (AC 40; SE-4) and units (DS-1)', () => {
  const facts = (over: Partial<Facts>): Facts => ({ sport: 'cycling', summary: { distance: 10000, elapsed_time: 1800 }, sessions: [], lengths: [], zones: false, ...over });
  const open = (f: Facts, units: 'metric' | 'imperial' = 'metric', trim = ''): string => {
    document.body.innerHTML = `<section class="summary"><button data-trim="${trim}"></button></section>`;
    openSummaryInfo(document.querySelector('button')!, f, units, summaryItems);
    const t = document.querySelector('dialog.summary-dlg .dlg-body')!.textContent!;
    document.body.innerHTML = '';
    return t;
  };
  const owner = { radii: [0, 0] as const, trimAt: [null, null] as const, zone: { known: true, offset_seconds: 3600 }, untrimmed: { distance: 12000, elapsed_time: 2000, calories: 300 } };

  it('radii, 0 as not hidden; in the units setting', () => {
    expect(open(facts({ owner }))).toContain('This workout: start not hidden, end not hidden.');
    expect(open(facts({ owner: { ...owner, radii: [300, 300] } }))).toContain('This workout: start hidden within 300 m, end hidden within 300 m.');
    expect(open(facts({ owner: { ...owner, radii: [300, 0] } }), 'imperial')).toContain('This workout: start hidden within 0.19 mi, end not hidden.');
  });

  it('trim ends in local time, recomputed and dropped totals', () => {
    const t = open(facts({ owner: { ...owner, trimAt: ['2026-06-06T05:10:00Z', '2026-06-06T05:40:00Z'] } }), 'metric', 'both');
    expect(t).toContain('This workout: trimmed (now starts 06:10, ends 06:40); recomputed: distance, elapsed time; dropped: calories.');
    expect(open(facts({ owner: { ...owner, zone: { known: false, offset_seconds: 0 }, trimAt: [null, '2026-06-06T05:40:00Z'] } }), 'metric', 'end')).toContain('trimmed (now ends 05:40 UTC)');
  });

  it('a stored trim without trimmed values (empty strip): neither recomputed nor dropped (QA-58-r1-3)', () => {
    const t = open(facts({ summary: {}, owner: { ...owner, trimAt: ['2026-06-06T05:10:00Z', '2026-06-06T05:40:00Z'] } }), 'metric', 'both');
    expect(t).toContain('This workout: trimmed (now starts 06:10, ends 06:40).');
    expect(t).not.toMatch(/recomputed:|dropped:/);
  });

  it('the viewing page names the trimmed end (QA-58-r1-5)', () => {
    expect(open(facts({}), 'metric', 'start')).toContain('This workout: trimmed at the start, totals recomputed.');
    expect(open(facts({}), 'metric', 'end')).toContain('This workout: trimmed at the end, totals recomputed.');
  });

  it('gaps between legs whenever there is more than one session, with or without transitions', () => {
    expect(open(facts({ sessions: [{ sport: 'running' }, { sport: 'cycling' }] }))).toContain('including pauses and gaps between legs.');
    expect(open(facts({ sessions: [{ sport: 'cycling' }] }))).toContain('including pauses.Range panel');
  });

  it('values follow the units setting', () => {
    expect(open(facts({}), 'imperial')).toContain('This workout: 6.21 mi in 30:00.');
    expect(open(facts({}), 'imperial')).toContain('faster than 0.6 mph on a ride or 0.3 mph in other sports');
  });
});
