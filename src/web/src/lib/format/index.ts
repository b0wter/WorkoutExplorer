// The one formatting home (F-13 design D-6): numbers, durations, pace, dates and sport names.
// English: fixed strings, never Intl or toLocale* (their output differs between engines and from
// Go, DS-3). German, Spanish and French: the decimal separator and dates through Intl (F-54 LG-7).
// Every unit-dependent function takes the unit system (F-14 D-1); the factors and unit strings
// live only here. Imports only lib/i18n. Hot-path callers (the readout) use only the first
// functions: no allocations other than the returned string.
import { lang } from '../i18n';
import { t } from './texts';

let sepLang = 'en';
let sep = '.';
let nf: Intl.NumberFormat; // the language's number format (not for English), also the yard grouping
/** The language's decimal separator, from Intl once per language (LG-7); '.' for English. */
function decimal(): string {
  const l = lang();
  if (l !== sepLang) {
    sepLang = l;
    sep = l === 'en' ? '.' : ((nf = new Intl.NumberFormat(l)).formatToParts(1.5).find((p) => p.type === 'decimal')?.value ?? '.');
  }
  return sep;
}

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
  const out = text.charCodeAt(0) === 45 && Number(text) === 0 ? text.slice(1) : text;
  return decimals > 0 ? out.replace('.', decimal()) : out;
}

/** The viewer's unit system (DS-1). */
export type Units = 'metric' | 'imperial';

export const M_PER_MILE = 1609.344;
const M_PER_YARD = 0.9144;
const FEET_PER_M = 1 / 0.3048;
const MPS_TO_MPH = 3600 / M_PER_MILE;

/** F-72 YD-1: the one rule. Imperial swimming is in yards; everything else km or mi. `sport` is the
 *  scope sport of what the surface shows (YD-2); without one the rule gives km or mi as before. */
/** Metres per distance unit: 1000, 1609.344 or, for imperial swimming, 0.9144 (yards). */
export const distanceMetres = (units: Units, sport?: string): number => (units === 'imperial' ? (sport === 'swimming' ? M_PER_YARD : M_PER_MILE) : 1000);

/** Whole yards, grouped: English by hand, de/es/fr through the language's Intl.NumberFormat (YD-5, LG-7). */
const formatYards = (yd: number): string => (decimal(), sepLang === 'en' ? ('' + yd).replace(/\B(?=(\d{3})+$)/g, ',') : nf.format(yd));

/** Metres → kilometres or miles with 2 decimals, or whole yards (YD-5); NaN → '–'. */
export function formatDistance(metres: number, units: Units = 'metric', sport?: string): string {
  const m = distanceMetres(units, sport);
  return m < 1 && metres >= 0 ? formatYards(Math.round(metres / m)) : formatFixed(metres / m, 2); // NaN (never a negative distance) takes formatFixed's '–'
}

/** 'km', 'mi' or 'yd'. */
export const distanceUnit = (units: Units, sport?: string): string => (units === 'metric' ? 'km' : sport === 'swimming' ? 'yd' : 'mi');
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

/** 'bpm', a text (F-54 LG-7). */
export const heartRateUnit = (): string => t('bpm');

/** Cadence unit: spm for the pace sports, the neutral '/min' for a multisport file, else rpm. */
export function cadenceUnit(sport: string): string {
  return t(isPaceSport(sport) ? 'spm' : sport === 'multisport' ? 'perMin' : 'rpm');
}

/** "Expires in 6 days" for the time left until `expiresAt` at `now` (both ms; zone independent). */
export function formatTimeLeft(expiresAtMs: number, nowMs: number): string {
  const left = Math.floor((expiresAtMs - nowMs) / 1000);
  if (!(left > 0)) return t('expiresNow');
  const days = Math.floor(left / 86400);
  const hours = Math.floor(left / 3600);
  const minutes = Math.floor(left / 60);
  if (days >= 2) return t('expires', { when: relative(days, 'day') });
  if (hours >= 1) return t('expires', { when: relative(hours, 'hour') });
  if (minutes >= 1) return t('expires', { when: relative(minutes, 'minute') });
  return t('expiresSoon');
}

/** 'in 6 days': English by hand; other languages through Intl.RelativeTimeFormat (LG-7, MW-9: we pick the unit only). */
export function relative(n: number, unit: 'day' | 'hour' | 'minute'): string {
  const l = lang();
  return l === 'en' ? 'in ' + n + ' ' + unit + (n === 1 ? '' : 's') : new Intl.RelativeTimeFormat(l).format(n, unit);
}
