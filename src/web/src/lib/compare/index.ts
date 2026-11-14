// Comparison chart rules (F-51 SC-3 to SC-6; AP-23). Pure, imports only types. Slots, which rows
// count, reference and best, the class of each bar, the scales as fractions of the plot height
// (0 top, 1 bottom), and the overlay offering. Speeds are m/s; the shown quantity (speed or pace)
// is linear in v or in 1/v, so the scales need no units: only labels are converted, by the caller.

import type { Totals } from '../datafile';

/** One slot's row: its values as the table has them; `partial` for the last partial split (SC-4). */
export interface CompareRow {
  readonly totals: Totals;
  readonly partial?: boolean;
}

export type OverlayKey = 'avg_heart_rate' | 'avg_power' | 'avg_cadence';
export const OVERLAYS: readonly OverlayKey[] = ['avg_heart_rate', 'avg_power', 'avg_cadence'];

export type BarClass = 'faster' | 'pace' | 'slower';

export interface Bar {
  /** Top and bottom of the bar, fractions of the plot height. */
  readonly y0: number;
  readonly y1: number;
  readonly cls: BarClass;
}

export interface Comparison {
  /** One per slot; null: an empty slot (no speed). */
  readonly bars: readonly (Bar | null)[];
  /** Median and best of the counted rows, m/s. */
  readonly reference: number;
  readonly best: number;
  /** The on-pace band, fractions. */
  readonly band: readonly [top: number, bottom: number];
  /** Speeds at the plot's top and bottom edge, m/s; for pace the top can be ≤ 0 (no label). */
  readonly top: number;
  readonly bottom: number;
  /** Overlay metrics at least one barred row has, in OVERLAYS order. */
  readonly overlays: readonly OverlayKey[];
}

/** Fewer bars than this, or more slots than MAX_SLOTS: no chart (SC-4). */
export const MIN_BARS = 2;
export const MAX_SLOTS = 200;
/** ±3 % of the reference, inclusive (SC-5). */
export const ON_PACE = 0.03;

const speedOf = (t: Totals): number => {
  const v = t.avg_speed;
  return v !== undefined && Number.isFinite(v) && v > 0 ? v : NaN;
};

const valueOf = (t: Totals, k: OverlayKey): number => {
  const v = t[k];
  return v !== undefined && Number.isFinite(v) ? v : NaN;
};

/** The chart for one view's slots, or null when there is none (SC-4: multisport is the caller's). */
export function compare(rows: readonly CompareRow[], pace: boolean): Comparison | null {
  if (rows.length > MAX_SLOTS) return null;
  const speeds = rows.map((r) => speedOf(r.totals));
  const barred = speeds.filter((v) => v === v);
  const counted = speeds.filter((v, i) => v === v && !rows[i].partial).sort((a, b) => a - b);
  if (barred.length < MIN_BARS || counted.length === 0) return null;
  const m = counted.length >> 1;
  const reference = counted.length % 2 ? counted[m] : (counted[m - 1] + counted[m]) / 2;
  const best = counted[counted.length - 1];
  // Up is faster: q = v for speed, −1/v for pace (linear in the shown pace); the axis is symmetric around q(reference).
  const q = (v: number): number => (pace ? -1 / v : v);
  const qr = q(reference);
  const hi = q(reference * (1 + ON_PACE)) - qr;
  const lo = qr - q(reference * (1 - ON_PACE));
  let h = Math.max(hi, lo);
  for (const v of barred) h = Math.max(h, Math.abs(q(v) - qr));
  const y = (v: number): number => 0.5 - (q(v) - qr) / (2 * h);
  const fromQ = (x: number): number => (pace ? -1 / x : x);
  return {
    bars: speeds.map((v) => {
      if (v !== v) return null;
      const d = (v - reference) / reference;
      const e = ON_PACE + 1e-9; // inclusive despite float error (10.3 vs 10)
      const yv = y(v);
      return { y0: Math.min(yv, 0.5), y1: Math.max(yv, 0.5), cls: d > e ? 'faster' : d < -e ? 'slower' : 'pace' };
    }),
    reference,
    best,
    band: [0.5 - hi / (2 * h), 0.5 + lo / (2 * h)],
    top: fromQ(qr + h),
    bottom: fromQ(qr - h),
    overlays: OVERLAYS.filter((k) => rows.some((r, i) => speeds[i] === speeds[i] && valueOf(r.totals, k) === valueOf(r.totals, k))),
  };
}

export interface Overlay {
  /** One per slot: the dot's position as a fraction, NaN without a dot (no bar or no value). */
  readonly ys: readonly number[];
  /** The values at the plot's top and bottom edge. */
  readonly max: number;
  readonly min: number;
}

/** The overlay's own linear scale (SC-6): the barred rows' values span the plot height. */
export function overlay(rows: readonly CompareRow[], c: Comparison, k: OverlayKey): Overlay {
  const vs = rows.map((r, i) => (c.bars[i] ? valueOf(r.totals, k) : NaN));
  const known = vs.filter((v) => v === v);
  let max = Math.max(...known);
  let min = Math.min(...known);
  if (max === min) {
    max += 1;
    min -= 1;
  }
  return { ys: vs.map((v) => (max - v) / (max - min)), max, min };
}
