// uPlot options (F-05 design, Tests; ADR-009). Node only: uplot-options.ts
// imports nothing but types from 'uplot'.

import { describe, expect, it } from 'vitest';
import type uPlot from 'uplot';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../../lib/datafile';
import { presentCharts } from '../../lib/charts';
import { CHART_SPECS } from '../../lib/charts';
import {
  TITLE_ROW_HEIGHT,
  buildOptions,
  chartAreaHeight,
  outerHeight,
  plotWidth,
  xRange,
  yIncrs,
} from './uplot-options';

const g = loadGolden().find((x) => x.name === 'ride-12h');
if (!g) throw new Error('golden ride-12h missing');
const decoded = decodeDataFile(g.bin);
if (!decoded.ok) throw new Error(decoded.error.message);
const ride: WorkoutData = decoded.data;

type RangeFn = (...args: unknown[]) => [number, number];
function rangeOf(o: uPlot.Options): [number, number] {
  const r = o.scales?.x?.range;
  if (typeof r !== 'function') throw new Error('x range is not a function');
  // uPlot passes (self, dataMin, dataMax, key); the result must not depend on them.
  return (r as unknown as RangeFn)(null, 12345, 23456, 'x');
}

function withTime(data: WorkoutData, time: number[], heartRate: number[]): WorkoutData {
  return { ...data, sampleCount: time.length, time: Float64Array.from(time), gaps: new Uint32Array(0), metrics: { heart_rate: Float64Array.from(heartRate) } };
}

/** y incrs: strictly increasing, smallest step 10^−decimals, every step a 1-2-5 multiple. */
function checkYIncrs(incrs: unknown, decimals: number): void {
  expect(Array.isArray(incrs)).toBe(true);
  const steps = incrs as number[];
  expect(steps.length).toBeGreaterThan(0);
  for (let k = 1; k < steps.length; k++) expect(steps[k]).toBeGreaterThan(steps[k - 1]);
  expect(steps[0]).toBe(10 ** -decimals);
  expect(steps[steps.length - 1]).toBe(1e6);
  for (const v of steps) {
    const mantissa = v / 10 ** Math.floor(Math.log10(v) + 1e-9);
    expect([1, 2, 5].some((m) => Math.abs(mantissa - m) < 1e-9), `step ${v}`).toBe(true);
  }
}

describe('yIncrs', () => {
  it('decimals 0: 1, 2, 5, 10, … 10^6', () => {
    expect(yIncrs(0).slice(0, 6)).toEqual([1, 2, 5, 10, 20, 50]);
    checkYIncrs(yIncrs(0), 0);
  });

  it('decimals 1: 0.1, 0.2, 0.5, 1, …', () => {
    expect(yIncrs(1).slice(0, 4)).toEqual([0.1, 0.2, 0.5, 1]);
    checkYIncrs(yIncrs(1), 1);
  });

  it('speed (1 decimal) starts at 0.1, every other spec at 1', () => {
    for (const spec of Object.values(CHART_SPECS)) {
      const y = buildOptions(spec, ride, { width: 800, height: 120, isBottom: false }).axes?.[1];
      expect((y?.incrs as number[])[0]).toBe(spec.metric === 'speed' ? 0.1 : 1);
    }
  });
});

describe('buildOptions on ride-12h', () => {
  const specs = presentCharts(ride);

  it('covers six charts', () => {
    expect(specs).toHaveLength(6);
  });

  for (const isBottom of [true, false]) {
    describe(`isBottom ${isBottom}`, () => {
      for (const spec of presentCharts(ride)) {
        it(`${spec.metric}`, () => {
          const o = buildOptions(spec, ride, { width: 1200, height: 120, isBottom });
          expect(o.cursor?.show).toBe(false);
          expect(o.legend?.show).toBe(false);
          expect(o.select?.show).toBe(false);
          expect(o.padding).toEqual([8, 16, 0, 0]);
          expect(o.scales?.x?.time).toBe(false);
          expect(rangeOf(o)).toEqual([ride.time[0], ride.time[ride.sampleCount - 1]]);

          const series = o.series[1];
          expect(series.spanGaps).toBe(false);
          expect(series.points?.show).toBe(false);
          expect(series.stroke).toBe(spec.colour);

          const [x, y] = o.axes ?? [];
          expect(y.size).toBe(48);
          expect(y.side).toBe(3);
          expect(x.side).toBe(2);
          expect(x.size).toBe(isBottom ? 28 : 0);
          expect(x.incrs).toEqual([60, 300, 600, 900, 1800, 3600, 7200, 10800, 21600]);
          expect(x.ticks?.show).toBe(isBottom);
          expect(x.grid?.show).toBe(true);
          expect(x.space).toBe(70);
          checkYIncrs(y.incrs, spec.decimals);

          expect(o.width).toBe(1200);
          // `height` is the plot-area height (design v1.1, "Size"): uPlot's height adds the
          // top padding (8) on every chart and the x axis (28) on the bottom chart only.
          expect(o.height).toBe(120 + 8 + (isBottom ? 28 : 0));
          const bottomOther = buildOptions(spec, ride, { width: 1200, height: 120, isBottom: !isBottom }).height ?? 0;
          expect(Math.abs((o.height ?? 0) - bottomOther)).toBe(28);
        });
      }
    });
  }

  it('x tick labels: elapsed time on the bottom chart, empty elsewhere', () => {
    const spec = CHART_SPECS.heart_rate;
    const values = (isBottom: boolean) => {
      const v = buildOptions(spec, ride, { width: 800, height: 120, isBottom }).axes?.[0].values;
      if (typeof v !== 'function') throw new Error('x values is not a function');
      return v(null as unknown as uPlot, [0, 1800, 3600, 43200], 0, 70, 1800);
    };
    expect(values(true)).toEqual(['0:00', '30:00', '1:00:00', '12:00:00']);
    expect(values(false)).toEqual(['', '', '', '']);
  });

  it('y tick labels use the spec format', () => {
    const v = buildOptions(CHART_SPECS.speed, ride, { width: 800, height: 120, isBottom: false }).axes?.[1].values;
    if (typeof v !== 'function') throw new Error('y values is not a function');
    expect(v(null as unknown as uPlot, [10, 20.25], 1, 30, 10)).toEqual(['10.0', '20.3']);
  });
});

describe('x range', () => {
  it('is [time[0], time[N−1]] for every chart, including a stream that starts late', () => {
    const nan = Number.NaN;
    const data = withTime(ride, [0, 1, 2, 3, 4, 5], [nan, nan, nan, 120, 121, 122]);
    const o = buildOptions(CHART_SPECS.heart_rate, data, { width: 600, height: 120, isBottom: true });
    expect(rangeOf(o)).toEqual([0, 5]);
  });

  it('is [t, t + 1] for N = 1', () => {
    const data = withTime(ride, [7], [120]);
    const o = buildOptions(CHART_SPECS.heart_rate, data, { width: 600, height: 120, isBottom: false });
    expect(rangeOf(o)).toEqual([7, 8]);
    expect(xRange(Float64Array.from([7]))).toEqual([7, 8]);
  });
});

describe('geometry helpers', () => {
  it('plot width = width − y axis − right and left padding', () => {
    expect(plotWidth(1200)).toBe(1200 - 48 - 16);
  });

  it('outer height = plot height + top and bottom padding + x axis on the bottom chart', () => {
    expect(outerHeight(120, false)).toBe(128);
    expect(outerHeight(120, true)).toBe(156);
  });
});

describe('chartAreaHeight (D-16, v1.4)', () => {
  it('is 0 for 0 charts', () => {
    expect(chartAreaHeight(0, 120)).toBe(0);
  });

  it.each([1, 6])('for %s charts: n × (title + h + 8) + 28, = sum of outerHeight plus n title rows', (n) => {
    for (const h of [120, 80]) {
      expect(chartAreaHeight(n, h)).toBe(n * (TITLE_ROW_HEIGHT + h + 8) + 28);
      let sum = 0;
      for (let k = 0; k < n; k++) sum += TITLE_ROW_HEIGHT + outerHeight(h, k === n - 1);
      expect(chartAreaHeight(n, h)).toBe(sum);
    }
  });
});

describe('pace axis and ink (F-14 D-10, D-11)', () => {
  const run = { ...ride, meta: { ...ride.meta, sport: 'running' } };
  const pace = presentCharts(run, 'metric').find((x) => x.metric === 'speed');
  it('a pace chart inverts the y scale (faster up), uses seconds ticks and m:ss labels', () => {
    if (!pace) throw new Error('no pace chart');
    const o = buildOptions(pace, run, { width: 800, height: 120, isBottom: false });
    expect(o.scales?.y?.dir).toBe(-1);
    expect(o.axes?.[1].incrs).toEqual([...(pace.incrs ?? [])]);
    const values = o.axes?.[1].values as (u: unknown, s: number[]) => string[];
    expect(values(null, [180, 300, 335])).toEqual(['3:00', '5:00', '5:35']);
  });
  it('other charts are not inverted', () => {
    expect(buildOptions(CHART_SPECS.speed, ride, { width: 800, height: 120, isBottom: false }).scales?.y?.dir).toBeUndefined();
  });
  it('ink colours the line, axes and grid; without ink the spec colour stays', () => {
    const ink = { line: '#111111', axis: '#222222', grid: '#333333' };
    const o = buildOptions(CHART_SPECS.heart_rate, ride, { width: 600, height: 120, isBottom: true }, ink);
    expect(o.series[1].stroke).toBe('#111111');
    expect(o.axes?.[0].stroke).toBe('#222222');
    expect(o.axes?.[1].grid?.stroke).toBe('#333333');
    expect(o.axes?.[0].grid?.stroke).toBe('#333333');
    expect(buildOptions(CHART_SPECS.heart_rate, ride, { width: 600, height: 120, isBottom: true }).series[1].stroke).toBe(CHART_SPECS.heart_rate.colour);
  });
});
