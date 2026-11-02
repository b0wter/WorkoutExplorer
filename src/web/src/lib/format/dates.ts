// Part of lib/format (F-13 design D-6): sport names and dates, imported by path ('lib/format/dates').
// Not re-exported by index.ts, so that the activity entry does not carry them: in the bundle only
// the edit page's owner chunk uses them (F-19 bundle share). Same rules as index.ts: fixed English, never Intl.

function pad2(n: number): string {
  return n < 10 ? '0' + n : '' + n;
}

const SPORT_NAMES: Readonly<Record<string, string>> = {
  cycling: 'Ride',
  running: 'Run',
  walking: 'Walk',
  hiking: 'Hike',
  swimming: 'Swim',
  rowing: 'Row',
  multisport: 'Multisport',
};

/** 'cycling' → 'Ride'; an unknown sport → its name with a capital and spaces ('cross_country_skiing' → 'Cross country skiing'); '' → 'Workout'. */
export function sportName(sport: string): string {
  if (Object.hasOwn(SPORT_NAMES, sport)) return SPORT_NAMES[sport];
  const words = sport.replaceAll('_', ' ').trim();
  return words === '' ? 'Workout' : words.charAt(0).toUpperCase() + words.slice(1);
}

/** Icon key: the sport when it has its own icon, else 'generic'. */
export function sportIcon(sport: string): string {
  return Object.hasOwn(SPORT_NAMES, sport) && sport !== 'multisport' ? sport : 'generic';
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

/** The wall clock at the start position: the instant shifted by the zone offset, read with UTC getters only. */
function shifted(rfc3339: string, offsetSeconds: number): Date {
  return new Date(Date.parse(rfc3339) + offsetSeconds * 1000);
}

/** '9 Oct 2026' for an instant in a zone given by its offset; never the viewer's zone. */
export function formatDate(rfc3339: string, offsetSeconds: number): string {
  const d = shifted(rfc3339, offsetSeconds);
  return d.getUTCDate() + ' ' + MONTHS[d.getUTCMonth()] + ' ' + d.getUTCFullYear();
}

/** '07:30', 24-hour, same rules as formatDate. */
export function formatClock(rfc3339: string, offsetSeconds: number): string {
  const d = shifted(rfc3339, offsetSeconds);
  return pad2(d.getUTCHours()) + ':' + pad2(d.getUTCMinutes());
}

/** Local start of a workout: '9 Oct 2026, 07:30' in the zone at the start position; ', 07:30 UTC' when the zone is unknown (question 13). */
export function formatLocalStart(rfc3339: string, zone: { known: boolean; offset_seconds: number }): string {
  const offset = zone.known ? zone.offset_seconds : 0;
  return formatDate(rfc3339, offset) + ', ' + formatClock(rfc3339, offset) + (zone.known ? '' : ' UTC');
}
