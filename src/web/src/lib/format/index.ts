// The one formatting home (F-13 design D-6): numbers, durations, pace, dates and sport names
// as fixed English strings. Never Intl or toLocale*: their output differs between engines and
// from Go (DS-3). Every unit-dependent function takes the unit system (F-14 D-1); the factors and
// unit strings live only here. Imports nothing. Hot-path callers (the readout) use
// only the first functions: no allocations other than the returned string.

/** Missing value: U+2013 EN DASH. Never '0'. */
export const MISSING_TEXT = '–';

function pad2(n: number): string {
  return n < 10 ? '0' + n : '' + n;
}

/** Seconds → 'm:ss' under one hour, else 'h:mm:ss' (hours unbounded). Rounds down to whole seconds. */
export function formatElapsed(seconds: number): string {
  if (!Number.isFinite(seconds)) return MISSING_TEXT;
  const negative = seconds < 0;
  const total = Math.floor(negative ? -seconds : seconds);
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = total % 60;
  const text = h > 0 ? h + ':' + pad2(m) + ':' + pad2(s) : m + ':' + pad2(s);
  return negative ? '-' + text : text;
}

/** v with a fixed number of decimals; NaN → '–'; never '-0'. */
export function formatFixed(v: number, decimals: number): string {
  if (!Number.isFinite(v)) return MISSING_TEXT;
  const text = v.toFixed(decimals);
  // toFixed keeps the sign of values that round to zero ('-0', '-0.0').
  return text.charCodeAt(0) === 45 && Number(text) === 0 ? text.slice(1) : text;
}

/** The viewer's unit system (DS-1). */
export type Units = 'metric' | 'imperial';

export const M_PER_MILE = 1609.344;
const M_PER_YARD = 0.9144;
const FEET_PER_M = 1 / 0.3048;
const MPS_TO_MPH = 3600 / M_PER_MILE;

/** Metres → kilometres or miles with 2 decimals; NaN → '–'. */
export function formatDistance(metres: number, units: Units = 'metric'): string {
  return formatFixed(metres / (units === 'metric' ? 1000 : M_PER_MILE), 2);
}

/** 'km' or 'mi'. */
export const distanceUnit = (units: Units): string => (units === 'metric' ? 'km' : 'mi');
/** 'm' or 'ft'. */
export const elevationUnit = (units: Units): string => (units === 'metric' ? 'm' : 'ft');
/** '°C' or '°F'. */
export const temperatureUnit = (units: Units): string => (units === 'metric' ? '°C' : '°F');
/** Metres → metres or feet. */
export const toElevation = (metres: number, units: Units): number => (units === 'metric' ? metres : metres * FEET_PER_M);
/** °C → °C or °F. */
export const toTemperature = (celsius: number, units: Units): number => (units === 'metric' ? celsius : celsius * 1.8 + 32);

// ---- F-13 additions: the activity page's texts (AP-1, AP-2, AP-8, DS-2, DS-3) ----

/** Sports with a pace instead of a speed (DS-2); others: speed. */
const PACE_SPORTS = ['running', 'walking', 'hiking', 'swimming', 'rowing'];

/** True for the sports that show pace (running, walking, hiking, swimming, rowing). */
export function isPaceSport(sport: string): boolean {
  return PACE_SPORTS.includes(sport);
}

/** The distance one pace unit covers in metres, and its label. Swimming: 100 m or 100 yd; rowing:
 *  500 m in both systems (question 9); the rest: 1 km or 1 mi. */
function paceBasis(sport: string, units: Units): [metres: number, unit: string] {
  const metric = units === 'metric';
  if (sport === 'swimming') return metric ? [100, '/100 m'] : [100 * M_PER_YARD, '/100 yd'];
  if (sport === 'rowing') return [500, '/500 m'];
  return metric ? [1000, '/km'] : [M_PER_MILE, '/mi'];
}

/** Pace in seconds per pace unit for a speed in m/s. Pace sports only. Not rounded; v <= 0 gives Infinity. */
export function paceSeconds(sport: string, units: Units, v: number): number {
  return v > 0 ? paceBasis(sport, units)[0] / v : Infinity;
}

/** Pace, or speed in km/h or mph with one decimal. v is m/s; a pace from v <= 0 or NaN is '' (hidden). */
export function formatSpeedOrPace(sport: string, v: number, units: Units = 'metric'): string {
  if (!(v > 0) || !Number.isFinite(v)) return '';
  if (!isPaceSport(sport)) return formatFixed(toSpeed(v, units), 1);
  return formatPace(paceSeconds(sport, units, v));
}

/** Seconds per pace unit as 'm:ss' (minutes unbounded). */
export function formatPace(seconds: number): string {
  const total = Math.round(seconds);
  return Math.floor(total / 60) + ':' + pad2(total % 60);
}

/** Unit after formatSpeedOrPace's text: 'km/h' or 'mph', '/km' or '/mi', '/100 m' or '/100 yd', '/500 m'. */
export function speedOrPaceUnit(sport: string, units: Units = 'metric'): string {
  return isPaceSport(sport) ? paceBasis(sport, units)[1] : units === 'metric' ? 'km/h' : 'mph';
}

/** Speed in m/s below which a pace sample is a stop: no pace (a gap in the chart, '–' in the readout; OQ-66). */
export const STOP_SPEED = 0.3;

/** m/s → km/h or mph. */
export function toSpeed(v: number, units: Units): number {
  return v * (units === 'metric' ? 3.6 : MPS_TO_MPH);
}

/** Cadence unit: spm for the pace sports, the neutral '/min' for a multisport file, else rpm. */
export function cadenceUnit(sport: string): string {
  return isPaceSport(sport) ? 'spm' : sport === 'multisport' ? '/min' : 'rpm';
}

/** "Expires in 6 days" for the time left until `expiresAt` at `now` (both ms; zone independent). */
export function formatTimeLeft(expiresAtMs: number, nowMs: number): string {
  const left = Math.floor((expiresAtMs - nowMs) / 1000);
  if (!(left > 0)) return 'Expires now';
  const days = Math.floor(left / 86400);
  const hours = Math.floor(left / 3600);
  const minutes = Math.floor(left / 60);
  const plural = (n: number, unit: string): string => n + ' ' + unit + (n === 1 ? '' : 's');
  if (days >= 2) return 'Expires in ' + plural(days, 'day');
  if (hours >= 1) return 'Expires in ' + plural(hours, 'hour');
  if (minutes >= 1) return 'Expires in ' + plural(minutes, 'minute');
  return 'Expires in less than a minute';
}
