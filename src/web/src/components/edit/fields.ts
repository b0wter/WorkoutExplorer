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
    allow_download: s.allow_download,
    lifetime_days: s.lifetime_days,
  };
}

export { cleanText };

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
