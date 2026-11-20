// The summary and laps parts of the page view model (F-13 design D-2 to D-5, ADR-026 items 2, 3);
// the whole model is view.ts. Pure. Renderers select and format; they never compute: every number is the data file's
// metadata as written. The Go builder (F-15) must reproduce the committed samples exactly.

import type { Lap, Summary, SummaryKey, TotalKey, WorkoutData } from '../datafile';
import {
  MISSING_TEXT,
  cadenceUnit,
  distanceUnit,
  heartRateUnit,
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
import { en, t } from './texts';

type Totals = Readonly<Partial<Record<SummaryKey, number>>>;


function has(t: Totals, k: SummaryKey): number | undefined {
  const v = t[k];
  return v === undefined || !Number.isFinite(v) ? undefined : v;
}

/** One summary cell with an average and an optional maximum (OQ-61). Only a maximum: the maximum is the value.
 *  `label` names the pair's two texts, 'avg…' and 'max…'. */
function pair(
  key: string,
  label: 'Pace' | 'Speed' | 'HeartRate' | 'Power' | 'Cadence' | 'Temperature',
  unit: string,
  avg: number | undefined,
  max: number | undefined,
  fmt: (v: number) => string,
): SummaryItem | null {
  const a = avg === undefined ? '' : fmt(avg);
  const m = max === undefined ? '' : fmt(max);
  if (a !== '') return { Key: key, Label: t(`avg${label}`), Value: a, Unit: unit, Max: m };
  if (m !== '') return { Key: key, Label: t(`max${label}`), Value: m, Unit: unit, Max: '' };
  return null;
}

/** AP-2 summary: the metrics the summary has, in AP-2 order; none as 0 or a dash. */
export function summaryItems(sport: string, s: Summary, units: Units = 'metric'): SummaryItem[] {
  const items: (SummaryItem | null)[] = [];
  const one = <T>(key: string, label: keyof typeof en, v: T | undefined, unit: string, fmt: (v: T) => string): void => {
    if (v !== undefined) items.push({ Key: key, Label: t(label), Value: fmt(v), Unit: unit, Max: '' });
  };
  const int = (v: number): string => formatFixed(v, 0);
  one('distance', 'distance', has(s, 'distance'), distanceUnit(units), (v) => formatDistance(v, units));
  one('moving_time', 'movingTime', has(s, 'moving_time'), '', formatElapsed);
  one('elapsed_time', 'elapsedTime', has(s, 'elapsed_time'), '', formatElapsed);
  one('calories', 'calories', has(s, 'calories'), 'kcal', int);
  // F-44 PS-5: whole metres or yards as recorded, the same in both unit systems (DS-1).
  one('pool', 'pool', has(s, 'pool_length_m'), 'm', int);
  one('pool', 'pool', has(s, 'pool_length_yd'), 'yd', int);
  one('lengths', 'lengths', has(s, 'lengths'), '', int);
  // F-48 PL-7: the one stroke of the whole swim, untinted.
  one('stroke', 'stroke', codeWord('stroke', s.stroke) || undefined, '', (w) => w);
  const speed = (v: number): string => formatSpeedOrPace(sport, v, units);
  const temp = (v: number): string => int(toTemperature(v, units));
  // A pace from a speed <= 0 is '' and so hidden (D-4). For a pace the best (fastest) value is the max.
  items.push(pair('speed', isPaceSport(sport) ? 'Pace' : 'Speed', speedOrPaceUnit(sport, units), has(s, 'avg_speed'), has(s, 'max_speed'), speed));
  items.push(pair('heart_rate', 'HeartRate', heartRateUnit(), has(s, 'avg_heart_rate'), has(s, 'max_heart_rate'), int));
  items.push(pair('power', 'Power', 'W', has(s, 'avg_power'), has(s, 'max_power'), int));
  items.push(pair('cadence', 'Cadence', cadenceUnit(sport), has(s, 'avg_cadence'), has(s, 'max_cadence'), int));
  one('ascent', 'elevationGain', has(s, 'ascent'), elevationUnit(units), (v) => int(toElevation(v, units)));
  items.push(pair('temperature', 'Temperature', temperatureUnit(units), has(s, 'avg_temperature'), has(s, 'max_temperature'), temp));
  return items.filter((i): i is SummaryItem => i !== null);
}

/** The known codes of the laps table's word columns: AP-8 triggers (F-46 LS-2), AP-13 strokes (ADR-048 item 2). */
const CODES = {
  trigger: ['button', 'time', 'distance', 'position', 'stop', 'equipment'],
  stroke: ['freestyle', 'backstroke', 'breaststroke', 'butterfly', 'drill', 'mixed', 'im'],
};
/** A known code's word ('Button', 'IM'); any other code is an empty cell. */
export const codeWord = (kind: keyof typeof CODES, code = ''): string => (!CODES[kind].includes(code) ? '' : t(`${kind}_${code}` as keyof typeof en));

/** AP-8 laps from the lap totals as written: the device's values, completed by the server where the
 *  device recorded none (D-5, F-18 L-1). Columns provisional (OQ-28). Null without laps. The
 *  browser's splits go through it too (F-18 L-5), as range values mapped to lap totals, without a
 *  trigger or stroke and so without those columns (LS-2: trigger last, when any lap has a known
 *  trigger; F-48 PL-7: stroke before it, when any lap has a known stroke). */
export function lapsGroup(sport: string, laps: readonly (Pick<Lap, 'totals'> & { readonly trigger?: string; readonly stroke?: string })[], units: Units = 'metric'): LapsGroup | null {
  if (laps.length === 0) return null;
  const speedLabel = t(isPaceSport(sport) ? 'colPace' : 'colSpeed', { u: speedOrPaceUnit(sport, units) });
  const cols: { key: string; label: string; cell: (t: Totals) => string }[] = [
    { key: 'elapsed_time', label: t('colTime'), cell: (o) => cellOf(o, 'elapsed_time', formatElapsed) },
    { key: 'distance', label: t('colDistance', { u: distanceUnit(units) }), cell: (o) => cellOf(o, 'distance', (v) => formatDistance(v, units)) },
    { key: 'avg_speed', label: speedLabel, cell: (o) => cellOf(o, 'avg_speed', (v) => formatSpeedOrPace(sport, v, units)) },
    { key: 'ascent', label: t('colGain', { u: elevationUnit(units) }), cell: (o) => cellOf(o, 'ascent', (v) => formatFixed(toElevation(v, units), 0)) },
    { key: 'avg_heart_rate', label: t('colHeartRate', { u: heartRateUnit() }), cell: (o) => cellOf(o, 'avg_heart_rate', (v) => formatFixed(v, 0)) },
    { key: 'avg_power', label: t('colPower'), cell: (o) => cellOf(o, 'avg_power', (v) => formatFixed(v, 0)) },
    { key: 'avg_cadence', label: t('colCadence', { u: cadenceUnit(sport) }), cell: (o) => cellOf(o, 'avg_cadence', (v) => formatFixed(v, 0)) },
  ];
  // A column appears when at least one lap has its value; a cell without one shows '–'.
  const shown = cols.filter((c) => laps.some((l) => c.cell(l.totals) !== MISSING_TEXT));
  // Word columns after the numbers, headed by their capitalised key, when at least one lap has a known word.
  const words = (['stroke', 'trigger'] as const).filter((k) => laps.some((l) => codeWord(k, l[k]) !== ''));
  return {
    Columns: [{ Key: 'number', Label: '#' }, ...shown.map((c) => ({ Key: c.key, Label: c.label })), ...words.map((k) => ({ Key: k, Label: t(k) }))],
    Rows: laps.map((l, i) => [String(i + 1), ...shown.map((c) => c.cell(l.totals)), ...words.map((k) => codeWord(k, l[k]))]),
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
