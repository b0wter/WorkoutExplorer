// Pure rules of the edit panel (F-19 design E-6 to E-8; F-12 O-4 to O-6). Every server rule has
// one client counterpart here, so the panel never sends what the server refuses.

import { formatClock, formatDate } from '../../lib/format/dates';
import type { Settings, SettingsPatch } from '../../lib/owner';
import { cleanText } from '../../lib/text';

/** What the panel edits. */
export type Form = Required<SettingsPatch>;

export const TITLE_MAX = 100;
export const DESCRIPTION_MAX = 1000;
const DAY_MS = 86_400_000;

export function formOf(s: Settings): Form {
  return {
    title: s.title,
    description: s.description,
    hide_ends: s.hide_ends,
    lifetime_days: s.lifetime_days,
    trim_start_at: s.trim_start_at,
    trim_end_at: s.trim_end_at,
    zones_max_heart_rate: s.zones_max_heart_rate,
    zones_threshold_power: s.zones_threshold_power,
  };
}

/** F-49 OQ-195: the zone fields, their whole-number ranges and the server's sentences (F-12). */
export const ZONE_FIELDS = [
  ['zones_max_heart_rate', 100, 250, 'The max heart rate must be a whole number from 100 to 250 bpm.'],
  ['zones_threshold_power', 50, 1000, 'The threshold power must be a whole number from 50 to 1,000 W.'],
] as const;

/** The first zone field the server would refuse, as its sentence; null when both are empty or valid.
 *  NaN (unparsable input) is refused too. */
export function zoneError(form: Pick<Form, 'zones_max_heart_rate' | 'zones_threshold_power'>): string | null {
  for (const [k, lo, hi, message] of ZONE_FIELDS) {
    const v = form[k];
    if (v !== null && !(Number.isInteger(v) && v >= lo && v <= hi)) return message;
  }
  return null;
}

/** At most `max` code points (F-12 O-4); `maxlength` already holds typing to UTF-16 units. */
const cut = (text: string, max: number): string => [...text].slice(0, max).join('');

/** One PATCH with only the fields that differ from the stored settings (E-6). Never visibility
 *  (F-20 C-11: read-only). */
export function patchOf(stored: Settings, form: Form): SettingsPatch {
  const next: Form = {
    ...form,
    title: cut(cleanText(form.title, false), TITLE_MAX),
    description: cut(cleanText(form.description, true), DESCRIPTION_MAX),
  };
  const patch: Record<string, unknown> = {};
  for (const k of Object.keys(next) as (keyof Form)[]) {
    if (next[k] !== stored[k]) patch[k] = next[k];
  }
  return patch as SettingsPatch;
}

/** Whole days d with 1 ≤ d ≤ 30 whose expiry is after now (O-5). */
export function lifetimeOptions(createdAt: string, nowMs: number): number[] {
  const created = Date.parse(createdAt);
  const out: number[] = [];
  for (let d = 1; d <= 30; d++) if (created + d * DAY_MS > nowMs) out.push(d);
  return out;
}

/** The expiry for a lifetime, as a date and time in the browser's time zone (E-7), e.g.
 *  '30 Oct 2026, 09:15'. lib/format's fixed English output, with the browser's offset. */
export function expiryText(createdAt: string, days: number): string {
  const at = new Date(Date.parse(createdAt) + days * DAY_MS);
  const iso = at.toISOString();
  const offset = -at.getTimezoneOffset() * 60;
  return formatDate(iso, offset) + ', ' + formatClock(iso, offset);
}

const pad2 = (n: number): string => (n < 10 ? '0' : '') + n;

/** UP-3's notice (E-8): elapsed time since the start as hh:mm:ss, hours not capped; null when the
 *  file was complete. */
export function incompleteNotice(dataEndS: number | null): string | null {
  if (dataEndS === null) return null;
  const s = Math.max(0, Math.floor(dataEndS));
  return `The file was incomplete; data after ${pad2(Math.floor(s / 3600))}:${pad2(Math.floor(s / 60) % 60)}:${pad2(s % 60)} is missing.`;
}

/** First index i with time[i] >= t; time.length when none. */
function atOrAfter(time: Float64Array, t: number): number {
  let lo = 0;
  let hi = time.length;
  while (lo < hi) {
    const m = (lo + hi) >> 1;
    if (time[m] < t) lo = m + 1;
    else hi = m;
  }
  return lo;
}

/** Last index i with time[i] <= t; -1 when none. */
function atOrBefore(time: Float64Array, t: number): number {
  const i = atOrAfter(time, t);
  return time[i] === t ? i : i - 1;
}

/** F-47 TR-4, the display mirror of the server's window rule: at least this much remains. */
export const TRIM_MIN_S = 60;

/** A stored trim instant as a draft.bin sample index, rounded inward like the server: the start to
 *  the first sample at or after it, the end to the last at or before it; null is that end. */
export function trimIndex(time: Float64Array, startS: number, at: string | null, end: boolean): number {
  const n = time.length;
  if (at === null) return end ? n - 1 : 0;
  const t = Date.parse(at) / 1000 - startS;
  return Math.max(0, Math.min(n - 1, (end ? atOrBefore : atOrAfter)(time, t)));
}

/** A handle's sample as the instant PATCH takes (start_time + offset, whole seconds), or null at
 *  its end of the recording (TR-3). */
export function trimAt(time: Float64Array, startS: number, i: number, end: boolean): string | null {
  if (i === (end ? time.length - 1 : 0)) return null;
  return new Date((startS + time[i]) * 1000).toISOString().slice(0, 19) + 'Z';
}

/** The handles after one moved: the moved one is held back so that at least TRIM_MIN_S remain
 *  between them (TR-4), so the slider never sends a trim the server refuses. */
export function trimClamp(time: Float64Array, a: number, b: number, endMoved: boolean): [number, number] {
  const n = time.length;
  if (endMoved) return [a, Math.min(n - 1, Math.max(b, atOrAfter(time, time[a] + TRIM_MIN_S)))];
  return [Math.max(0, Math.min(a, atOrBefore(time, time[b] - TRIM_MIN_S))), b];
}
