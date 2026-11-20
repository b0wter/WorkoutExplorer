// Map pins, pure (F-56 PN-9, PN-11, PN-12; PIN-1 to PIN-8). The public file check and order for
// the lazy src/pins chunk, and the edit page's mirror of the server's rules (instants, snapping,
// shading). No DOM; never in the activity entry.

import { nearestIndex } from '../cursor/nearest';
import { cleanText } from '../text';
import type { ApiPin } from '../owner';
import { t } from './texts';

export const PIN_TYPES = ['point_of_interest', 'danger', 'difficult_section', 'water_food', 'photo_spot', 'note'] as const;
export type PinType = (typeof PIN_TYPES)[number];

/** PIN-2; an empty title shows its type's name in the page language (OQ-277, F-54). */
export const typeName = (type: PinType): string => t(type);

/** PN-2, OQ-238: the server's limits, in code points. */
export const MAX_PINS = 50;
export const PIN_TITLE_MAX = 60;
export const PIN_NOTE_MAX = 500;

/** One pin as the views show it: sample indices into the shown data file; `to` only for a
 *  section (to > from). `dim`: the edit page's reason it is not public ('' or absent: public). */
export interface ShownPin {
  readonly type: PinType;
  readonly title: string;
  readonly note: string;
  readonly from: number;
  readonly to?: number;
  readonly dim?: string;
}

const codePoints = (s: string): number => [...s].length;
const isIndex = (v: unknown, n: number): v is number => Number.isInteger(v) && (v as number) >= 0 && (v as number) < n;

/** PN-6, PN-11: the public pins file as parsed JSON, checked against a data file of n samples;
 *  null when anything is off (no pins shown). Sorted as the file should be (by from, point before
 *  section, then to, then file order). */
export function readPins(v: unknown, n: number): ShownPin[] | null {
  if (!Array.isArray(v) || v.length > MAX_PINS) return null;
  const out: ShownPin[] = [];
  for (const p of v as Record<string, unknown>[]) {
    if (typeof p !== 'object' || p === null || Array.isArray(p)) return null;
    const keys = Object.keys(p);
    const section = 'to' in p;
    if (keys.length !== (section ? 5 : 4)) return null;
    const { type, title, note, from, to } = p;
    if (!PIN_TYPES.includes(type as PinType) || typeof title !== 'string' || typeof note !== 'string') return null;
    if (codePoints(title) > PIN_TITLE_MAX || codePoints(note) > PIN_NOTE_MAX || !isIndex(from, n)) return null;
    if (section && !(isIndex(to, n) && to > from)) return null;
    out.push({ type: type as PinType, title, note, from, ...(section && { to: to as number }) });
  }
  return sortPins(out);
}

/** The canonical order (ADR-057 item 1), stable. */
export function sortPins<T extends Pick<ShownPin, 'from' | 'to'>>(pins: T[]): T[] {
  return pins.sort((a, b) => a.from - b.from || (a.to ?? -1) - (b.to ?? -1));
}

/** The name a pin shows: its title, else its type's name (OQ-277). */
export const pinName = (p: Pick<ShownPin, 'type' | 'title'>): string => p.title || typeName(p.type);

// ---- edit page (PN-3, PN-5, PN-12) ------------------------------------------------------------

export type { ApiPin };

/** An instant (RFC 3339) as the nearest draft.bin sample, ties to the earlier (PN-3). */
export const indexAt = (time: Float64Array, startS: number, at: string): number => Math.max(0, nearestIndex(time, Date.parse(at) / 1000 - startS));

/** A sample's instant as the API takes it: whole seconds, UTC (TR-4's arithmetic). */
export const instantAt = (time: Float64Array, startS: number, i: number): string => new Date((startS + time[i]) * 1000).toISOString().slice(0, 19) + 'Z';

export const fromApi = (p: ApiPin, time: Float64Array, startS: number): ShownPin => ({
  type: p.type,
  title: p.title,
  note: p.note,
  from: indexAt(time, startS, p.start_at),
  ...(p.end_at !== null && { to: indexAt(time, startS, p.end_at) }),
});

const cut = (s: string, max: number): string => [...s].slice(0, max).join('');

/** As the server stores it (UP-4 rules): title cleaned and trimmed, both cut to their limits. */
export const toApi = (p: ShownPin, time: Float64Array, startS: number): ApiPin => ({
  type: p.type,
  title: cut(cleanText(p.title, false).trim(), PIN_TITLE_MAX),
  note: cut(cleanText(p.note, true), PIN_NOTE_MAX),
  start_at: instantAt(time, startS, p.from),
  end_at: p.to === undefined ? null : instantAt(time, startS, p.to),
});

/** OQ-234: the nearest sample with a position, ties to the earlier; -1 when none has one. */
export function snap(lat: Float64Array, i: number): number {
  for (let d = 0; d < lat.length; d++) {
    if (i - d >= 0 && lat[i - d] === lat[i - d]) return i - d;
    if (i + d < lat.length && lat[i + d] === lat[i + d]) return i + d;
    if (i - d < 0 && i + d >= lat.length) break;
  }
  return -1;
}

/** What the edit page shades (F-47 TR-9, F-55 HR-8): the live trim window [a, b] and the first and
 *  last sample the radii keep public (hiding_steps; -1 when none). */
export interface Shade {
  readonly a: number;
  readonly b: number;
  readonly from: number;
  readonly to: number;
}

/** The reason that wins (trimmed away); compared by edit/pins, so read in the current language. */
export const removed = (): string => t('removedOnSave');

/** PN-12: why a pin is dimmed, '' when it would be public. Trimmed away wins over hidden. */
export function dimReason(p: Pick<ShownPin, 'from' | 'to'>, s: Shade, lat: Float64Array | undefined): string {
  const ends = p.to === undefined ? [p.from] : [p.from, p.to];
  if (ends.some((i) => i < s.a || i > s.b)) return removed();
  const start = ends.some((i) => s.from < 0 || i < s.from);
  const end = ends.some((i) => s.to < 0 || i > s.to);
  if (start || end) return t(start && end ? 'hiddenBoth' : start ? 'hiddenStart' : 'hiddenEnd');
  return lat && ends.every((i) => lat[i] === lat[i]) ? '' : t('hidden');
}

/** OQ-275's mirror: some sample is kept, not hidden and has a position. */
export function hasVisibleRoute(s: Shade, lat: Float64Array | undefined): boolean {
  if (!lat || s.from < 0 || s.to < 0) return false;
  for (let i = Math.max(s.a, s.from); i <= Math.min(s.b, s.to); i++) if (lat[i] === lat[i]) return true;
  return false;
}
