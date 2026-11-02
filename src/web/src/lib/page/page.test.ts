// The view model builder and the committed samples (F-13 design D-3, D-4, D-5, D-7).
// `UPDATE_SAMPLES=1 npm test` rewrites src/server/testdata/page/; otherwise a stale or missing
// sample fails (the contract with F-15's Go builder).
import { afterEach, describe, expect, it, vi } from 'vitest';
import { listSamples, readSample, removeSample, updating, writeSample } from '../../../test-support/samples.mjs';
import { decodeDataFile, type WorkoutData } from '../datafile';
import { loadGolden } from '../../../test-support/golden.mjs';
import { readoutFields } from '../charts';
import { distanceAdvances, lapsGroup, summaryItems } from '.';
import { buildPageViewModel, defaultTitle } from './view';
import { EXPIRES_AT, SAMPLES, buildSample, sampleText, withoutHeartRate } from './samples';
import type { PageViewModel } from './model';

const byName = (name: string): PageViewModel => {
  const spec = SAMPLES.find((s) => s.name === name);
  if (!spec) throw new Error(name);
  return buildSample(spec);
};
const item = (m: PageViewModel, key: string) => m.Summary.find((i) => i.Key === key);

describe('samples', () => {
  it('match the committed files', () => {
    if (updating()) {
      for (const s of SAMPLES) writeSample(s.name, sampleText(buildSample(s)));
      for (const n of listSamples()) if (!SAMPLES.some((s) => s.name === n)) removeSample(n);
    }
    expect(listSamples()).toEqual(SAMPLES.map((s) => s.name).sort());
    for (const s of SAMPLES) {
      expect(readSample(s.name), `${s.name} is stale: run UPDATE_SAMPLES=1 npm test`).toBe(sampleText(buildSample(s)));
    }
  });
});

describe('summary (AP-2, D-4)', () => {
  it('a ride has every metric in AP-2 order, with maximums as a second value (OQ-61)', () => {
    const m = byName('ride-30min');
    expect(m.Summary.map((i) => i.Key)).toEqual([
      'distance', 'moving_time', 'elapsed_time', 'ascent', 'speed', 'heart_rate', 'power', 'cadence', 'temperature', 'calories',
    ].filter((k) => item(m, k)));
    for (const k of ['speed', 'heart_rate', 'power', 'cadence', 'temperature']) {
      expect(item(m, k)?.Max, k).not.toBe('');
    }
    expect(item(m, 'distance')?.Max).toBe('');
    expect(item(m, 'speed')?.Unit).toBe('km/h');
    expect(item(m, 'cadence')?.Unit).toBe('rpm');
  });

  it('AC 3: a run without power has no power cell; a file without heart rate shows no heart-rate value or 0 bpm', () => {
    expect(item(byName('run-30min'), 'power')).toBeUndefined();
    const noHr = byName('ride-no-hr');
    expect(item(noHr, 'heart_rate')).toBeUndefined();
    expect(JSON.stringify(noHr)).not.toMatch(/bpm|Heart rate|HR/);
    expect(noHr.Laps?.Columns.map((c) => c.Label)).not.toContain('HR (bpm)');
  });

  it('pace for running and swimming, with the sport-dependent cadence unit (DS-2)', () => {
    const run = byName('run-30min');
    expect(item(run, 'speed')).toMatchObject({ Label: 'Avg pace', Unit: '/km' });
    expect(item(run, 'speed')?.Value).toMatch(/^\d+:\d\d$/);
    expect(item(run, 'cadence')?.Unit).toBe('spm');
    expect(item(byName('pool-swim'), 'speed')?.Unit).toBe('/100 m');
    expect(item(byName('triathlon'), 'speed')?.Unit).toBe('km/h');
  });

  it('a missing metric is absent, never zero or a dash', () => {
    const indoor = byName('indoor-unknown-zone');
    expect(indoor.Summary.map((i) => i.Key)).not.toContain('distance');
    for (const i of indoor.Summary) expect(i.Value).not.toMatch(/^(0|–|)$/);
  });

  it('a pace from a speed of 0 is hidden', () => {
    const golden = loadGolden().find((g) => g.name === 'run-30min')!;
    const r = decodeDataFile(golden.bin);
    if (!r.ok) throw new Error();
    const data: WorkoutData = { ...r.data, meta: { ...r.data.meta, summary: { avg_speed: 0, max_speed: 0, distance: 1000 } } };
    const m = buildPageViewModel(data, { title: '', description: '', expiresAt: EXPIRES_AT, publicId: 'x', dataFile: 'data.0.bin' });
    expect(m.Summary.map((i) => i.Key)).toEqual(['distance']);
  });
});

describe('header (AP-1, D-7)', () => {
  afterEach(() => vi.unstubAllEnvs());

  it('AC 2: a UTC+2 ride shows its local start time to a viewer in UTC-5', () => {
    vi.stubEnv('TZ', 'America/New_York');
    expect(new Date(2026, 0, 1, 12).getTimezoneOffset()).toBe(300); // the test really runs at UTC-5
    const golden = loadGolden().find((g) => g.name === 'ride-30min')!;
    const r = decodeDataFile(golden.bin);
    if (!r.ok) throw new Error();
    const data: WorkoutData = { ...r.data, meta: { ...r.data.meta, start_time: '2026-06-12T05:30:00Z', time_zone: { known: true, offset_seconds: 7200 } } };
    const m = buildPageViewModel(data, { title: '', description: '', expiresAt: EXPIRES_AT, publicId: 'x', dataFile: 'data.0.bin' });
    expect(m.Header.Start).toBe('12 Jun 2026, 07:30');
  });

  it('an unknown zone shows UTC with a label', () => {
    expect(byName('indoor-unknown-zone').Header.Start).toMatch(/, \d\d:\d\d UTC$/);
  });

  it('the default title is sport and date (UP-4); a given title is kept', () => {
    expect(byName('ride-12h').Page.Title).toBe('Ride, 12 Jun 2026');
    expect(defaultTitle('running', '2026-10-09T23:30:00Z', 7200)).toBe('Run, 10 Oct 2026');
    expect(byName('ride-30min').Page.Title).toBe('Morning loop');
  });

  it('the expiry estimate is a date in the workout zone, marked as an estimate (OQ-62)', () => {
    const m = byName('ride-12h');
    expect(m.Header.ExpiresText).toBe('Expires around 30 Oct 2026');
    expect(m.Header.ExpiresAt).toBe(EXPIRES_AT);
  });

  it('the data file URL carries the content hash and the view path is the workout page', () => {
    const m = byName('ride-30min');
    expect(m.Page.ViewPath).toBe('/w/ride-30min');
    expect(m.Page.DataURL).toMatch(/^\/w\/ride-30min\/data\.[0-9a-f]{16}\.bin$/);
  });
});

describe('modules (AP-3, D-5, D-10)', () => {
  it('indoor: the hint, no map slot, charts still sized', () => {
    const m = byName('indoor-ride');
    expect(m.Route).toEqual({ Map: '', NoLocation: 'yes', Hidden: '' });
    expect(m.ChartsClass).toMatch(/^charts-\d cells-\d has-distance$/);
    expect(m.Summary.length).toBeGreaterThan(0);
  });

  it('F-17 R-8: has-distance only with a distance stream, for every golden file', () => {
    let without = 0;
    for (const g of loadGolden()) {
      const r = decodeDataFile(g.bin);
      if (!r.ok) continue;
      const m = buildPageViewModel(r.data, { title: '', description: '', expiresAt: EXPIRES_AT, publicId: 'x', dataFile: 'd' });
      if (m.ChartsClass === '') continue;
      expect(m.ChartsClass.endsWith(' has-distance'), g.name).toBe(r.data.metrics.distance !== undefined);
      if (r.data.metrics.distance === undefined) without++;
    }
    expect(without).toBeGreaterThan(0);
    expect(byName('indoor-unknown-zone').ChartsClass).toBe('charts-1 cells-2');
  });

  it('fully hidden (F-16 H-5): the notice only, never with the indoor hint', () => {
    expect(byName('route-hidden').Route).toEqual({ Map: '', NoLocation: '', Hidden: 'yes' });
  });

  it('a ride with a route reserves the map; cells = time + distance + charts', () => {
    const m = byName('ride-12h');
    expect(m.Route).toEqual({ Map: 'yes', NoLocation: '', Hidden: '' });
    expect(m.ChartsClass).toBe('charts-6 cells-8 has-distance');
  });

  it('laps: a column only when a lap has the value; the number column first; 24 rows for ride-12h', () => {
    const laps = byName('ride-12h').Laps!;
    expect(laps.Columns[0]).toEqual({ Key: 'number', Label: '#' });
    expect(laps.Rows).toHaveLength(24);
    expect(laps.Rows[0]).toHaveLength(laps.Columns.length);
    expect(laps.Rows[0][0]).toBe('1');
  });

  it('L-8: without laps but with an advancing distance the module says "No laps recorded."', () => {
    expect(byName('ride-no-laps').Laps).toEqual({ Columns: [], Rows: [], None: 'yes' });
    expect(byName('ride-12h').Laps?.None).toBe('');
  });

  it('L-8: without laps and without an advancing distance there is no laps group', () => {
    const settings = { title: '', description: '', expiresAt: EXPIRES_AT, publicId: 'x', dataFile: 'd' };
    const g = loadGolden().find((x) => x.name === 'ride-no-laps')!;
    const r = decodeDataFile(g.bin);
    if (!r.ok) throw new Error();
    const noDistance = { ...r.data.metrics };
    delete noDistance.distance;
    expect(buildPageViewModel({ ...r.data, metrics: noDistance }, settings).Laps).toBeNull();
    const flat = { ...r.data.metrics, distance: new Float64Array(r.data.sampleCount).fill(5) };
    expect(buildPageViewModel({ ...r.data, metrics: flat }, settings).Laps).toBeNull();
  });
});

describe('distanceAdvances (F-18 L-6, L-8; same table as Go TestDistanceAdvances)', () => {
  const of = (v: number[]) => distanceAdvances({ metrics: { distance: Float64Array.from(v) } });
  it('compares the last valid distance with the first', () => {
    expect(of([0, 0.5, 0.9])).toBe(true);
    expect(of([0.4, 0.4, 0.4])).toBe(false);
    expect(of([NaN, 0.3, 0.7, NaN])).toBe(true);
    expect(of([NaN, 0.3, NaN])).toBe(false);
    expect(of([NaN, NaN])).toBe(false);
    expect(distanceAdvances({ metrics: {} })).toBe(false);
  });
});

describe('withoutHeartRate', () => {
  it('removes the stream and every total', () => {
    const g = loadGolden().find((x) => x.name === 'ride-30min')!;
    const r = decodeDataFile(g.bin);
    if (!r.ok) throw new Error();
    const d = withoutHeartRate(r.data);
    expect(d.metrics.heart_rate).toBeUndefined();
    expect(JSON.stringify(d.meta)).not.toContain('heart_rate');
  });
});

describe('reserved readout (D-10)', () => {
  it('the cell count in ChartsClass equals the readout the chart island renders, for every golden file', () => {
    let checked = 0;
    for (const g of loadGolden()) {
      const r = decodeDataFile(g.bin);
      if (!r.ok) continue;
      const m = buildPageViewModel(r.data, { title: '', description: '', expiresAt: EXPIRES_AT, publicId: 'x', dataFile: 'd' });
      const cells = m.ChartsClass.match(/cells-(\d+)/);
      if (cells) {
        expect(Number(cells[1]), g.name).toBe(readoutFields(r.data).length);
        checked++;
      }
    }
    expect(checked).toBeGreaterThan(20);
  });
});

describe('imperial builders (F-14 D-2, D-5; AC 1, 4, 6)', () => {
  const spec = (name: string) => {
    const s = SAMPLES.find((x) => x.name === name);
    if (!s) throw new Error(name);
    const golden = loadGolden().find((g) => g.name === s.golden);
    const r = decodeDataFile(golden!.bin);
    if (!r.ok) throw new Error('decode');
    return r.data;
  };
  it('the same keys and columns in both systems; only text differs', () => {
    for (const name of ['ride-30min', 'run-30min', 'pool-swim', 'triathlon', 'indoor-ride']) {
      const d = spec(name);
      const m = summaryItems(d.meta.sport, d.meta.summary, 'metric');
      const i = summaryItems(d.meta.sport, d.meta.summary, 'imperial');
      expect(i.map((x) => [x.Key, x.Label, x.Unit !== '', x.Max !== ''])).toEqual(m.map((x) => [x.Key, x.Label, x.Unit !== '', x.Max !== '']));
      const lm = lapsGroup(d.meta.sport, d.meta.laps, 'metric');
      const li = lapsGroup(d.meta.sport, d.meta.laps, 'imperial');
      expect(li?.Columns.map((c) => c.Key)).toEqual(lm?.Columns.map((c) => c.Key));
      expect(li?.Rows.map((r) => r.length)).toEqual(lm?.Rows.map((r) => r.length));
    }
  });
  it('a ride shows miles, feet, mph and degrees F; the metric build is unchanged', () => {
    const d = spec('ride-30min');
    const i = summaryItems(d.meta.sport, d.meta.summary, 'imperial');
    const unit = (k: string) => i.find((x) => x.Key === k)?.Unit;
    expect([unit('distance'), unit('ascent'), unit('speed'), unit('temperature')]).toEqual(['mi', 'ft', 'mph', '°F']);
    expect(i.find((x) => x.Key === 'distance')?.Value).toBe('8.36');
    expect(summaryItems(d.meta.sport, d.meta.summary).find((x) => x.Key === 'distance')?.Value).toBe('13.46');
    const laps = lapsGroup(d.meta.sport, d.meta.laps, 'imperial');
    expect(laps?.Columns.map((c) => c.Label)).toContain('Distance (mi)');
    expect(laps?.Columns.map((c) => c.Label)).toContain('Gain (ft)');
    expect(laps?.Columns.map((c) => c.Label)).toContain('Speed (mph)');
  });
  it('a run shows min/mi; a swim shows miles with pace per 100 yd (OQ-67)', () => {
    const r = spec('run-30min');
    expect(summaryItems(r.meta.sport, r.meta.summary, 'imperial').find((x) => x.Key === 'speed')?.Unit).toBe('/mi');
    const s = spec('pool-swim');
    const items = summaryItems(s.meta.sport, s.meta.summary, 'imperial');
    expect(items.find((x) => x.Key === 'distance')?.Unit).toBe('mi');
    expect(items.find((x) => x.Key === 'speed')?.Unit).toBe('/100 yd');
  });
});
