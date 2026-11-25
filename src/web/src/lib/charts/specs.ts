// Which charts exist, in which order, and how their values read (F-05 design,
// "Which charts, in which order", "Readout"; AP-4). All labels, units, colours
// and formatters live here and in format.ts, so F-14 changes one place.

import type { DynamicsName, WorkoutData } from '../datafile';
import type { SelectionStore } from '../selection';
import {
  MISSING_TEXT,
  distanceUnit,
  elevationUnit,
  formatDistance,
  formatElapsed,
  formatFixed,
  formatPace,
  isPaceSport,
  paceSeconds,
  STOP_SPEED,
  speedOrPaceUnit,
  cadenceUnit,
  temperatureUnit,
  toElevation,
  toSpeed,
  toTemperature,
  type Units,
} from '../format';
import { heartRateUnit } from '../format';
import { t } from './texts';

type BaseMetric = 'altitude' | 'speed' | 'heart_rate' | 'power' | 'cadence' | 'temperature';
export type ChartMetric = BaseMetric | DynamicsName;

/** AP-4 order. Differs from METRIC_ORDER (cadence and power swap; distance is not charted). */
export const CHART_ORDER: readonly BaseMetric[] = ['altitude', 'speed', 'heart_rate', 'power', 'cadence', 'temperature'];

export interface ChartSpec {
  readonly metric: ChartMetric;
  /** 'Elevation', 'Speed', 'Heart rate', 'Power', 'Cadence', 'Temperature'. */
  readonly label: string;
  /** Readout and range panel label where the chart's is too long (F-68); default `label`. */
  readonly short?: string;
  /** Display unit: 'm' or 'ft', 'km/h' or 'mph' or a pace unit, 'bpm', 'W', 'rpm' or 'spm' or '/min', '°C' or '°F'. */
  readonly unit: string;
  /** F-03 §2 unit → display unit (speed × 3.6, feet, °F; a pace is seconds per pace unit, unclipped; NaN for a missing sample or a stop). */
  readonly toDisplay: (v: number) => number;
  /** Display precision of format(); also the finest y tick step, 10^−decimals (D-13). */
  readonly decimals: number;
  /** Display value → text with `decimals` places; NaN → '–'. */
  readonly format: (v: number) => string;
  /** Fallback CSS colour; the charts read `--c-{metric}` from the theme (F-14 D-9, D-10). */
  readonly colour: string;
  /** The y axis runs faster-up: a pace (OQ-66). */
  readonly invert: boolean;
  /** Y tick steps; default the 1-2-5 sequence from 10^−decimals. */
  readonly incrs?: readonly number[];
}

const identity = (v: number): number => v;
/** Y tick steps of a pace axis in seconds (round minutes and seconds). */
const PACE_INCRS = [1, 2, 5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600];

function spec(
  metric: ChartMetric,
  label: string,
  unit: string,
  toDisplay: (v: number) => number,
  decimals: number,
  colour: string,
  format: (v: number) => string = (v) => formatFixed(v, decimals),
): ChartSpec {
  return { metric, label, unit, toDisplay, decimals, format, colour, invert: false };
}

/** The six chart specs for a sport and unit system (F-14 D-11). The speed chart of a pace sport is a pace chart. */
export function chartSpecs(sport: string, units: Units): Readonly<Record<BaseMetric, ChartSpec>> {
  const speedSpec = isPaceSport(sport)
    ? (() => {
        // A stop has no pace (NaN: a gap, '–'). The real pace is returned; the chart clips it (chartValues).
        const toDisplay = (v: number): number => (v >= STOP_SPEED ? paceSeconds(sport, units, v) : NaN);
        return { ...spec('speed', t('chartPace'), speedOrPaceUnit(sport, units), toDisplay, 0, '#1f6fc5', formatPace), invert: true, incrs: PACE_INCRS };
      })()
    : spec('speed', t('chartSpeed'), speedOrPaceUnit(sport, units), (v) => toSpeed(v, units), 1, '#1f6fc5');
  return {
    altitude: spec('altitude', t('chartElevation'), elevationUnit(units), (v) => toElevation(v, units), 0, '#5b8c3a'),
    speed: speedSpec,
    heart_rate: spec('heart_rate', t('chartHeartRate'), heartRateUnit(), identity, 0, '#d0312d'),
    power: spec('power', t('chartPower'), 'W', identity, 0, '#8a3ffc'),
    cadence: spec('cadence', t('chartCadence'), cadenceUnit(sport), identity, 0, '#0f8b8d'),
    temperature: spec('temperature', t('chartTemperature'), temperatureUnit(units), (v) => toTemperature(v, units), 0, '#d97706'),
  };
}

/** Share of moving pace samples the pace axis covers; the slower rest clips to the axis edge. */
const PACE_AXIS_PERCENTILE = 0.95;

/** What a chart draws for a metric's raw column: toDisplay applied once (a pace stop is NaN, so a gap
 *  before decimation). A pace axis is bounded by the workout's own data: samples slower than the
 *  PACE_AXIS_PERCENTILE of the moving ones clip to that bound (the readout keeps the real value). */
export function chartValues(spec: ChartSpec, raw: Float64Array): Float64Array {
  const values = raw.map(spec.toDisplay);
  if (!spec.invert) return values;
  const moving = values.filter((v) => v === v).sort();
  if (moving.length === 0) return values;
  const bound = moving[Math.floor((moving.length - 1) * PACE_AXIS_PERCENTILE)];
  for (let i = 0; i < values.length; i++) if (values[i] > bound) values[i] = bound;
  return values;
}

/** The metric, speed (not pace) specs; tests and bench pages. The page uses chartSpecs. */
export const CHART_SPECS = chartSpecs('', 'metric');

function hasValue(values: Float64Array | undefined): boolean {
  if (values === undefined) return false;
  for (let i = 0; i < values.length; i++) if (values[i] === values[i]) return true;
  return false;
}

/** F-68 XS-9: what the lazy src/dynamics chunk installs (the page awaits it before the charts mount when the
 *  file has one of the streams): the running dynamics' specs, and the range panel's extra cells. Without
 *  the chunk those streams have no chart and no cell. */
export const dynamics: {
  specs(units: Units): readonly ChartSpec[];
  /** Appends the cells before `.actions` in `root`; the result follows a units change or is destroyed. */
  range?(root: HTMLElement, data: WorkoutData, selection: SelectionStore, units: Units): { update(units: Units): void; destroy(): void };
} = { specs: () => [] };

/** Specs in CHART_ORDER for streams present in data with at least one non-NaN value. */
export function presentCharts(data: WorkoutData, units: Units = 'metric', sport = data.meta.sport): readonly ChartSpec[] {
  const specs = chartSpecs(sport, units);
  return [...CHART_ORDER.filter((m) => hasValue(data.metrics[m])).map((m) => specs[m]), ...dynamics.specs(units).filter((s) => hasValue(data.metrics[s.metric]))];
}

export interface ReadoutField {
  readonly key: 'elapsed' | 'distance' | ChartMetric;
  /** 'Time', 'Distance', or the chart's label. */
  readonly label: string;
  /** '' for elapsed, 'km' or 'mi' for distance. */
  readonly unit: string;
  /** Value at a sample index, formatted; '–' for a missing sample. */
  readonly text: (index: number) => string;
}

/** Readout fields in order: elapsed, distance (if the stream is present; in `distSport`'s unit, fixed per instance, F-72 YD-2), then presentCharts(data) for `sport` (default the workout's; F-67 LP-10).
 *  Built once per WorkoutData; text() reads the full-resolution arrays (D-9). */
export function readoutFields(data: WorkoutData, units: Units = 'metric', sport?: string, distSport = data.meta.sport): readonly ReadoutField[] {
  const time = data.time;
  const fields: ReadoutField[] = [{ key: 'elapsed', label: t('axisTime'), unit: '', text: (i) => formatElapsed(time[i]) }];
  const distance = data.metrics.distance;
  if (distance !== undefined) {
    fields.push({ key: 'distance', label: t('axisDistance'), unit: distanceUnit(units, distSport), text: (i) => formatDistance(distance[i], units, distSport) });
  }
  for (const spec of presentCharts(data, units, sport)) {
    const values = data.metrics[spec.metric];
    if (values === undefined) continue; // presentCharts only returns present streams
    const { toDisplay, format } = spec;
    fields.push({
      key: spec.metric,
      label: spec.short ?? spec.label,
      unit: spec.unit,
      text: (i) => {
        const v = values[i];
        const d = toDisplay(v);
        return d === d ? format(d) : MISSING_TEXT;
      },
    });
  }
  return fields;
}
