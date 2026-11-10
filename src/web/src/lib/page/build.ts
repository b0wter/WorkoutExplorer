// The summary and laps parts of the page view model (F-13 design D-2 to D-5, ADR-026 items 2, 3);
// the whole model is view.ts. Pure. Renderers select and format; they never compute: every number is the data file's
// metadata as written. The Go builder (F-15) must reproduce the committed samples exactly.

import type { Lap, TotalKey, WorkoutData } from '../datafile';
import {
  MISSING_TEXT,
  cadenceUnit,
  distanceUnit,
  elevationUnit,
  formatDistance,
  formatElapsed,
  formatFixed,
  formatSpeedOrPace,
  isPaceSport,
  speedOrPaceUnit,
  temperatureUnit,
  toElevation,
  toTemperature,
  type Units,
} from '../format';
import type { LapsGroup, SummaryItem } from './model';

type Totals = Readonly<Partial<Record<TotalKey | 'moving_time', number>>>;


function has(t: Totals, k: TotalKey | 'moving_time'): number | undefined {
  const v = t[k];
  return v === undefined || !Number.isFinite(v) ? undefined : v;
}

/** One summary cell with an average and an optional maximum (OQ-61). Only a maximum: the maximum is the value. */
function pair(
  key: string,
  label: string,
  unit: string,
  avg: number | undefined,
  max: number | undefined,
  fmt: (v: number) => string,
): SummaryItem | null {
  const a = avg === undefined ? '' : fmt(avg);
  const m = max === undefined ? '' : fmt(max);
  if (a !== '') return { Key: key, Label: 'Avg ' + label, Value: a, Unit: unit, Max: m };
  if (m !== '') return { Key: key, Label: 'Max ' + label, Value: m, Unit: unit, Max: '' };
  return null;
}

/** AP-2 summary: the metrics the summary has, in AP-2 order; none as 0 or a dash. */
export function summaryItems(sport: string, s: Totals, units: Units = 'metric'): SummaryItem[] {
  const items: (SummaryItem | null)[] = [];
  const one = (key: string, label: string, v: number | undefined, unit: string, fmt: (v: number) => string): void => {
    if (v !== undefined) items.push({ Key: key, Label: label, Value: fmt(v), Unit: unit, Max: '' });
  };
  const int = (v: number): string => formatFixed(v, 0);
  one('distance', 'Distance', has(s, 'distance'), distanceUnit(units), (v) => formatDistance(v, units));
  one('moving_time', 'Moving time', has(s, 'moving_time'), '', formatElapsed);
  one('elapsed_time', 'Elapsed time', has(s, 'elapsed_time'), '', formatElapsed);
  one('calories', 'Calories', has(s, 'calories'), 'kcal', int);
  one('ascent', 'Elevation gain', has(s, 'ascent'), elevationUnit(units), (v) => int(toElevation(v, units)));
  const speed = (v: number): string => formatSpeedOrPace(sport, v, units);
  const temp = (v: number): string => int(toTemperature(v, units));
  // A pace from a speed <= 0 is '' and so hidden (D-4). For a pace the best (fastest) value is the max.
  items.push(pair('speed', isPaceSport(sport) ? 'pace' : 'speed', speedOrPaceUnit(sport, units), has(s, 'avg_speed'), has(s, 'max_speed'), speed));
  items.push(pair('heart_rate', 'heart rate', 'bpm', has(s, 'avg_heart_rate'), has(s, 'max_heart_rate'), int));
  items.push(pair('power', 'power', 'W', has(s, 'avg_power'), has(s, 'max_power'), int));
  items.push(pair('cadence', 'cadence', cadenceUnit(sport), has(s, 'avg_cadence'), has(s, 'max_cadence'), int));
  items.push(pair('temperature', 'temperature', temperatureUnit(units), has(s, 'avg_temperature'), has(s, 'max_temperature'), temp));
  return items.filter((i): i is SummaryItem => i !== null);
}

/** AP-8 laps from the lap totals as written: the device's values, completed by the server where the
 *  device recorded none (D-5, F-18 L-1). Columns provisional (OQ-28). Null without laps. The
 *  browser's splits go through it too (F-18 L-5), as range values mapped to lap totals. */
export function lapsGroup(sport: string, laps: readonly Pick<Lap, 'totals'>[], units: Units = 'metric'): LapsGroup | null {
  if (laps.length === 0) return null;
  const speedLabel = (isPaceSport(sport) ? 'Pace' : 'Speed') + ' (' + speedOrPaceUnit(sport, units) + ')';
  const cols: { key: string; label: string; cell: (t: Totals) => string }[] = [
    { key: 'elapsed_time', label: 'Time', cell: (t) => cellOf(t, 'elapsed_time', formatElapsed) },
    { key: 'distance', label: 'Distance (' + distanceUnit(units) + ')', cell: (t) => cellOf(t, 'distance', (v) => formatDistance(v, units)) },
    { key: 'avg_speed', label: speedLabel, cell: (t) => cellOf(t, 'avg_speed', (v) => formatSpeedOrPace(sport, v, units)) },
    { key: 'ascent', label: 'Gain (' + elevationUnit(units) + ')', cell: (t) => cellOf(t, 'ascent', (v) => formatFixed(toElevation(v, units), 0)) },
    { key: 'avg_heart_rate', label: 'HR (bpm)', cell: (t) => cellOf(t, 'avg_heart_rate', (v) => formatFixed(v, 0)) },
    { key: 'avg_power', label: 'Power (W)', cell: (t) => cellOf(t, 'avg_power', (v) => formatFixed(v, 0)) },
    { key: 'avg_cadence', label: 'Cadence (' + cadenceUnit(sport) + ')', cell: (t) => cellOf(t, 'avg_cadence', (v) => formatFixed(v, 0)) },
  ];
  // A column appears when at least one lap has its value; a cell without one shows '–'.
  const shown = cols.filter((c) => laps.some((l) => c.cell(l.totals) !== MISSING_TEXT));
  return {
    Columns: [{ Key: 'number', Label: '#' }, ...shown.map((c) => ({ Key: c.key, Label: c.label }))],
    Rows: laps.map((l, i) => [String(i + 1), ...shown.map((c) => c.cell(l.totals))]),
    None: '',
  };
}

/** True when the distance stream advances: its last valid value is above its first (F-18 L-6, L-8).
 *  O(1) on a stream that is valid at both ends. Go's internal/page evaluates the same condition. */
export function distanceAdvances(data: Pick<WorkoutData, 'metrics'>): boolean {
  const d = data.metrics.distance;
  if (d === undefined) return false;
  let i = 0;
  while (i < d.length && Number.isNaN(d[i])) i++;
  let j = d.length - 1;
  while (j > i && Number.isNaN(d[j])) j--;
  return j > i && d[j] > d[i];
}

function cellOf(t: Totals, key: TotalKey, fmt: (v: number) => string): string {
  const v = has(t, key);
  if (v === undefined) return MISSING_TEXT;
  const text = fmt(v);
  return text === '' ? MISSING_TEXT : text;
}
