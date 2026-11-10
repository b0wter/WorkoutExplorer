// uPlot options for one metric chart (F-05 design, "Charts"; ADR-009).
// uPlot only draws: cursor, legend and select are off; we own decimation,
// pointer handling and the cursor line. Alignment is explicit (D-6): the same
// fixed x range, y-axis width and padding on every chart, so one pointer x
// means the same time in every chart.
//
// Type-only import of 'uplot', so this file (and its test) runs in Node.

import type uPlot from 'uplot';
import type { WorkoutData } from '../../lib/datafile';
import { formatElapsed, formatFixed, type ChartAxis, type ChartSpec } from '../../lib/charts';
import { distanceUnit, type Units } from '../../lib/format';

// Layout constants (design v1.2): each defined once. CSS that needs one of them
// reads a custom property that ChartStack sets from these values.

/** Fixed y-axis width in CSS px, on every chart (CSS: --y-axis-width). */
export const Y_AXIS_SIZE = 48;
/** Top padding of every plot in CSS px. */
export const PAD_TOP = 8;
/** Top, right, bottom, left in CSS px, on every chart; never uPlot's auto padding. */
export const PADDING: readonly [number, number, number, number] = [PAD_TOP, 16, 0, 0];
/** Height of the x axis (tick labels) on the bottom chart, CSS px; 0 on every other chart. */
export const X_AXIS_SIZE = 28;
/** Height of the HTML title row above each chart, CSS px (CSS: --title-row-height). */
export const TITLE_ROW_HEIGHT = 16;
/** Elapsed-time tick increments in seconds. */
export const X_INCRS: readonly number[] = [60, 300, 600, 900, 1800, 3600, 7200, 10800, 21600];
/** Minimum CSS px between x ticks; "10:30:00" needs about 50 px at uPlot's 12 px font. */
export const X_SPACE = 70;
/** Plot area height in CSS px unless the page sets chartHeight (F-13). */
export const DEFAULT_CHART_HEIGHT = 120;

/** The x scale of every chart (F-17 R-7): range, tick steps and labels. */
export interface XScale {
  readonly min: number;
  readonly max: number;
  readonly incrs: readonly number[];
  /** splits → labels; incr is the tick step uPlot chose. */
  readonly label: (v: number, incr: number) => string;
}

const M_PER_MILE = 1609.344;
const STEPS = [0.1, 0.2, 0.5, 1, 2, 5, 10, 20, 50, 100, 200, 500];

/** The time scale ([time[0], time[N−1]], elapsed labels) or the distance scale (round km or mi). */
export function xScale(axis: ChartAxis, units: Units = 'metric'): XScale {
  if (axis.kind === 'time') return { min: axis.min, max: axis.max, incrs: X_INCRS, label: (v) => formatElapsed(v) };
  const unit = units === 'metric' ? 1000 : M_PER_MILE;
  const name = ' ' + distanceUnit(units);
  return {
    min: axis.min,
    max: axis.max,
    incrs: STEPS.map((k) => k * unit),
    label: (v, incr) => formatFixed(v / unit, incr < unit * 0.99 ? 1 : 0) + name,
  };
}

export interface ChartGeometry {
  /** Full chart width in CSS px (the stack's content width). */
  readonly width: number;
  /** PLOT-AREA height in CSS px (chartHeight), the same on every chart; not the uPlot
   *  height. outerHeight() adds the top padding and, on the bottom chart, the x axis. */
  readonly height: number;
  /** The bottom chart carries the x-axis labels. */
  readonly isBottom: boolean;
}

/** The x scale range of every chart: [time[0], time[N−1]], or [t, t + 1] for N = 1. */
export function xRange(time: Float64Array): [number, number] {
  const first = time[0];
  const last = time[time.length - 1];
  return time.length > 1 ? [first, last] : [first, first + 1];
}

/** Y tick steps: the 1-2-5 sequence from 10^−decimals up to 10^6 (D-13), so a step is
 *  never finer than the display precision (no duplicate labels like "1", "1", "2"). */
export function yIncrs(decimals: number): number[] {
  const out: number[] = [];
  // Divide for negative exponents so 0.1, 0.2, 0.5 are the nearest doubles.
  for (let e = -decimals; e < 6; e++) for (const m of [1, 2, 5]) out.push(e < 0 ? m / 10 ** -e : m * 10 ** e);
  out.push(1e6);
  return out;
}

/** Height of the chart area in CSS px (D-16, v1.4): per chart (title row + chartHeight +
 *  top padding) + the bottom chart's x axis; 0 for 0 charts. The readout is not included. */
export function chartAreaHeight(chartCount: number, chartHeight: number): number {
  return chartCount > 0 ? chartCount * (TITLE_ROW_HEIGHT + chartHeight + PAD_TOP) + X_AXIS_SIZE : 0;
}

/** Plot area width in CSS px for a chart of the given full width. */
export function plotWidth(width: number): number {
  return width - Y_AXIS_SIZE - PADDING[1] - PADDING[3];
}

/** Full uPlot height in CSS px: plot area + vertical padding + x axis (bottom chart only). */
export function outerHeight(plotHeight: number, isBottom: boolean): number {
  return plotHeight + PADDING[0] + PADDING[2] + (isBottom ? X_AXIS_SIZE : 0);
}

/** Draws extra marks into one chart's plot overlay (F-42 MS-7), on layout only: after the first
 *  layout, a width change, an axis switch and a new decorator; never on a cursor or range flush.
 *  `x` gives a sample's x in CSS px on the current axis. It must replace what it drew before, and
 *  what it draws takes no pointer events. */
export type ChartDecorator = (over: HTMLElement, x: (index: number) => number) => void;

/** Colours uPlot cannot take from CSS: read from the theme's custom properties when a chart is built (F-14 D-10). */
export interface ChartInk {
  /** The series line. */
  readonly line: string;
  /** Axis labels and ticks. */
  readonly axis: string;
  /** Grid lines. */
  readonly grid: string;
}

export function buildOptions(spec: ChartSpec, data: WorkoutData, geometry: ChartGeometry, ink?: ChartInk, x?: XScale): uPlot.Options {
  const range = xRange(data.time);
  const lo = x ? x.min : range[0];
  const hi = x ? x.max : range[1];
  const label = x ? x.label : formatElapsed;
  const { isBottom } = geometry;
  const format = spec.format;
  const incrs = spec.incrs ? [...spec.incrs] : yIncrs(spec.decimals);
  const axisInk = ink && { stroke: ink.axis };

  return {
    width: geometry.width,
    height: outerHeight(geometry.height, isBottom),
    padding: [PADDING[0], PADDING[1], PADDING[2], PADDING[3]],
    cursor: { show: false },
    legend: { show: false },
    select: { show: false, left: 0, top: 0, width: 0, height: 0 },
    scales: {
      // Identical on every chart, also for a stream that starts late.
      x: { time: false, range: () => [lo, hi] },
      y: spec.invert ? { auto: true, dir: -1 } : { auto: true },
    },
    series: [{}, { stroke: ink?.line ?? spec.colour, width: 1.5, points: { show: false }, spanGaps: false }],
    axes: [
      {
        scale: 'x',
        side: 2,
        size: isBottom ? X_AXIS_SIZE : 0,
        space: X_SPACE,
        incrs: [...(x ? x.incrs : X_INCRS)],
        values: isBottom
          ? (_u: uPlot, splits: number[], _a: number, _s: number, incr: number) => splits.map((v) => label(v, incr))
          : (_u: uPlot, splits: number[]) => splits.map(() => ''),
        ...axisInk,
        ticks: { show: isBottom, ...(ink && { stroke: ink.axis }) },
        grid: { show: true, ...(ink && { stroke: ink.grid }) },
      },
      {
        scale: 'y',
        side: 3,
        size: Y_AXIS_SIZE,
        incrs,
        values: (_u: uPlot, splits: number[]) => splits.map((v) => format(v)),
        ...axisInk,
        ...(ink && { ticks: { stroke: ink.grid }, grid: { stroke: ink.grid } }),
      },
    ],
  };
}
