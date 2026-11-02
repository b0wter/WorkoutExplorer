// Which charts are shown and what the readout holds (F-05 design, Tests; AP-4, AC 1, AC 2).

import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile, type MetricName, type WorkoutData } from '../datafile';
import { CHART_ORDER, CHART_SPECS, chartValues, decimate, presentCharts, readoutFields } from './index';

/** A WorkoutData built in the test: time 0..n−1, the given metrics. */
function workout(n: number, metrics: Partial<Record<MetricName, number[]>>): WorkoutData {
  const time = new Float64Array(n);
  for (let i = 0; i < n; i++) time[i] = i;
  const m: Partial<Record<MetricName, Float64Array>> = {};
  for (const [k, v] of Object.entries(metrics)) m[k as MetricName] = Float64Array.from(v);
  return {
    sampleCount: n,
    time,
    gaps: new Uint32Array(0),
    metrics: m,
    meta: {
      sport: 'cycling',
      start_time: '2026-06-06T05:00:00Z',
      time_zone: { known: true, offset_seconds: 0 },
      summary: {},
      sessions: [],
      laps: [],
    },
  };
}

const ALL = {
  altitude: [100, 101, 102],
  distance: [0, 5, 10],
  speed: [5, 5.5, 6],
  heart_rate: [120, 121, Number.NaN],
  cadence: [80, 81, 82],
  power: [200, 210, 220],
  temperature: [20, 20, 21],
};

const metricsOf = (data: WorkoutData) => presentCharts(data).map((s) => s.metric);

const golden = new Map<string, WorkoutData>();
for (const g of loadGolden()) {
  const r = decodeDataFile(g.bin);
  if (!r.ok) throw new Error(`golden ${g.name}: ${r.error.message}`);
  golden.set(g.name, r.data);
}
function goldenData(name: string): WorkoutData {
  const d = golden.get(name);
  if (!d) throw new Error(`no golden file ${name}`);
  return d;
}

describe('CHART_ORDER', () => {
  it('is elevation, speed, heart rate, power, cadence, temperature (AP-4)', () => {
    expect(CHART_ORDER).toEqual(['altitude', 'speed', 'heart_rate', 'power', 'cadence', 'temperature']);
  });
});

describe('presentCharts', () => {
  it('gives six specs in order for all streams', () => {
    expect(metricsOf(workout(3, ALL))).toEqual(['altitude', 'speed', 'heart_rate', 'power', 'cadence', 'temperature']);
    expect(presentCharts(workout(3, ALL))[0].metric).toBe('altitude');
  });

  it('gives five specs and no power chart without power', () => {
    const { power: _p, ...rest } = ALL;
    expect(metricsOf(workout(3, rest))).toEqual(['altitude', 'speed', 'heart_rate', 'cadence', 'temperature']);
  });

  it('gives no elevation chart without altitude', () => {
    const { altitude: _a, ...rest } = ALL;
    expect(metricsOf(workout(3, rest))).not.toContain('altitude');
  });

  it('never gives a distance chart', () => {
    expect(metricsOf(workout(3, { distance: [0, 1, 2] }))).toEqual([]);
  });

  it('gives no chart for a present stream that is entirely missing', () => {
    const nan = [Number.NaN, Number.NaN, Number.NaN];
    expect(metricsOf(workout(3, { ...ALL, power: nan }))).not.toContain('power');
  });

  it('gives nothing for time only', () => {
    expect(presentCharts(workout(3, {}))).toEqual([]);
  });

  it('ride-12h → six charts', () => {
    expect(metricsOf(goldenData('ride-12h'))).toEqual(CHART_ORDER);
  });

  it('indoor-no-gps → no elevation chart', () => {
    const m = metricsOf(goldenData('indoor-no-gps'));
    expect(m).not.toContain('altitude');
    expect(m.length).toBeGreaterThan(0);
  });

  it('no-distance-with-gps → no speed, no power', () => {
    const m = metricsOf(goldenData('no-distance-with-gps'));
    expect(m).not.toContain('speed');
    expect(m).not.toContain('power');
    expect(m).toContain('heart_rate');
  });

  it('every golden file gives charts in CHART_ORDER', () => {
    for (const data of golden.values()) {
      const m = metricsOf(data);
      expect(m).toEqual(CHART_ORDER.filter((x) => m.includes(x)));
    }
  });
});

describe('readoutFields', () => {
  it('orders elapsed, distance, then the present charts', () => {
    const keys = readoutFields(workout(3, ALL)).map((f) => f.key);
    expect(keys).toEqual(['elapsed', 'distance', 'altitude', 'speed', 'heart_rate', 'power', 'cadence', 'temperature']);
  });

  it('labels and units of elapsed and distance', () => {
    const [elapsed, distance] = readoutFields(workout(3, ALL));
    expect([elapsed.label, elapsed.unit]).toEqual(['Time', '']);
    expect([distance.label, distance.unit]).toEqual(['Distance', 'km']);
  });

  it('has no distance field without distance', () => {
    const { distance: _d, ...rest } = ALL;
    expect(readoutFields(workout(3, rest)).map((f) => f.key)).not.toContain('distance');
  });

  it('has only the elapsed field for time only', () => {
    expect(readoutFields(workout(3, {})).map((f) => f.key)).toEqual(['elapsed']);
  });

  it('text(i) shows the formatted full-resolution value at i, and – for a missing sample', () => {
    const fields = readoutFields(workout(3, ALL));
    const byKey = new Map(fields.map((f) => [f.key, f]));
    expect(byKey.get('elapsed')?.text(2)).toBe('0:02');
    expect(byKey.get('distance')?.text(2)).toBe('0.01');
    expect(byKey.get('speed')?.text(1)).toBe('19.8');
    expect(byKey.get('heart_rate')?.text(1)).toBe('121');
    expect(byKey.get('heart_rate')?.text(2)).toBe('–');
    expect(byKey.get('heart_rate')?.text(2)).not.toBe('0');
  });

  it('on ride-12h: elapsed, distance and six chart fields, each equal to the full-resolution value', () => {
    const data = goldenData('ride-12h');
    const fields = readoutFields(data);
    expect(fields.map((f) => f.key)).toEqual(['elapsed', 'distance', ...CHART_ORDER]);
    // Indices that a decimated series would not hold (between bucket extremes).
    for (const i of [0, 1, 12345, 30001, data.sampleCount - 1]) {
      expect(fields[0].text(i)).toBe(formatElapsedRef(data.time[i]));
      for (const f of fields.slice(2)) {
        const spec = CHART_SPECS[f.key as keyof typeof CHART_SPECS];
        const v = data.metrics[spec.metric]?.[i] ?? Number.NaN;
        expect(f.text(i)).toBe(spec.format(spec.toDisplay(v)));
      }
    }
  });
});

/** Reference h:mm:ss / m:ss written independently of format.ts. */
function formatElapsedRef(s: number): string {
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = Math.floor(s % 60);
  const two = (x: number) => String(x).padStart(2, '0');
  return h > 0 ? `${h}:${two(m)}:${two(sec)}` : `${m}:${two(sec)}`;
}

describe('units and pace charts (F-14 D-11, OQ-66)', () => {
  const run = (sport: string) => {
    const d = workout(3, ALL);
    return { ...d, meta: { ...d.meta, sport } };
  };
  it('a ride in imperial: feet, mph, degrees F, miles in the readout', () => {
    const d = workout(3, ALL);
    const specs = presentCharts(d, 'imperial');
    const by = Object.fromEntries(specs.map((s) => [s.metric, s]));
    expect(by.altitude.unit).toBe('ft');
    expect(by.altitude.toDisplay(100)).toBeCloseTo(328.084, 3);
    expect(by.speed.unit).toBe('mph');
    expect(by.speed.invert).toBe(false);
    expect(by.temperature.unit).toBe('°F');
    expect(by.temperature.toDisplay(20)).toBe(68);
    const fields = readoutFields(d, 'imperial');
    expect(fields.find((f) => f.key === 'distance')?.unit).toBe('mi');
    expect(fields.find((f) => f.key === 'distance')?.text(2)).toBe('0.01');
  });
  it.each([
    ['running', 'metric', 'Pace', '/km'],
    ['running', 'imperial', 'Pace', '/mi'],
    ['hiking', 'metric', 'Pace', '/km'],
    ['swimming', 'imperial', 'Pace', '/100 yd'],
    ['rowing', 'imperial', 'Pace', '/500 m'],
    ['cycling', 'metric', 'Speed', 'km/h'],
    ['multisport', 'metric', 'Speed', 'km/h'],
    ['inline_skating', 'imperial', 'Speed', 'mph'],
  ] as const)('%s in %s: speed chart is %s in %s', (sport, units, label, unit) => {
    const spec = presentCharts(run(sport), units).find((s) => s.metric === 'speed');
    expect(spec?.label).toBe(label);
    expect(spec?.unit).toBe(unit);
    expect(spec?.invert).toBe(label === 'Pace');
  });
  it('a run: pace values, faster is a smaller number, a stop is a gap, the readout is unclipped', () => {
    const d = run('running');
    const speed = presentCharts(d, 'metric').find((s) => s.metric === 'speed');
    if (!speed) throw new Error('no speed chart');
    expect(speed.toDisplay(4)).toBe(250);
    expect(speed.toDisplay(5)).toBeLessThan(speed.toDisplay(4));
    expect(speed.toDisplay(0)).toBeNaN();
    expect(speed.toDisplay(-1)).toBeNaN();
    expect(speed.toDisplay(0.29)).toBeNaN();
    expect(speed.toDisplay(NaN)).toBeNaN();
    expect(speed.toDisplay(0.3)).toBeCloseTo(3333.3, 1); // real pace, not clipped
    expect(speed.format(speed.toDisplay(4))).toBe('4:10');
    const field = readoutFields(d, 'metric').find((f) => f.key === 'speed');
    expect(field?.label).toBe('Pace');
    expect(field?.text(0)).toBe('3:20'); // 5 m/s
  });
  it('the drawn pace: a stop is a gap through decimation, slow outliers clip to the data-derived bound', () => {
    const n = 200;
    const raw = new Float64Array(n).fill(4); // 4:10/km
    for (let i = 50; i < 80; i++) raw[i] = 0; // a stop
    for (let i = 190; i < n; i++) raw[i] = 0.5; // 10 of 170 moving samples (6 %) far slower
    const speed = presentCharts(run('running'), 'metric').find((s) => s.metric === 'speed');
    if (!speed) throw new Error('no speed chart');
    const values = chartValues(speed, raw);
    expect(values[60]).toBeNaN();
    expect(values[0]).toBe(250);
    expect(values[195]).toBeCloseTo(2000, 6); // 6 % are slow, so the 95th percentile is already 0.5 m/s
    const time = Float64Array.from({ length: n }, (_, i) => i);
    const dec = decimate(time, values, new Uint32Array(0), 0, n - 1, n);
    const at = dec.x.findIndex((t) => t > 49 && t < 80);
    expect(dec.y[at]).toBeNull(); // one break between the samples before and after the stop
    expect(dec.x.filter((t) => t > 50 && t < 80).length).toBe(0 + (dec.x[at] > 50 ? 1 : 0));
    expect(dec.y.some((v) => v !== null && Number.isNaN(v))).toBe(false);
  });
  it('the pace axis bound follows the workout, not a fixed pace', () => {
    const speed = presentCharts(run('running'), 'metric').find((s) => s.metric === 'speed');
    if (!speed) throw new Error('no speed chart');
    const raw = new Float64Array(100).fill(3); // 5:33/km
    raw[99] = 0.4; // one very slow moment: 41:40/km
    const values = chartValues(speed, raw);
    expect(Math.max(...values)).toBeCloseTo(1000 / 3, 6); // clipped to the 95th percentile
    expect(values[99]).toBeCloseTo(1000 / 3, 6);
    expect(speed.toDisplay(0.4)).toBe(2500); // the readout stays real
    // a genuinely slow walker keeps his own scale
    const slow = chartValues(speed, new Float64Array(100).fill(0.5));
    expect(slow[0]).toBe(2000);
    // all stops: no bound, all gaps
    expect(chartValues(speed, new Float64Array(5)).every(Number.isNaN)).toBe(true);
    // a speed chart is not clipped
    const ride = presentCharts(run('cycling'), 'metric').find((s) => s.metric === 'speed');
    expect(chartValues(ride as never, Float64Array.of(1, 100))[1]).toBeCloseTo(360, 9);
  });
  it('the imperial pace is per mile', () => {
    const speed = presentCharts(run('running'), 'imperial').find((s) => s.metric === 'speed');
    expect(speed?.toDisplay(0)).toBeNaN();
    expect(speed?.toDisplay(1609.344 / 600)).toBeCloseTo(600, 6);
    expect(speed?.format(1440)).toBe('24:00');
  });
});
