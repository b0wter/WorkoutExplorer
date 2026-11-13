// Lengths view rules (F-48 PL-8; AP-13). Pure, imports nothing but types. Numbering, rest rows,
// the Strokes and SWOLF columns, and the one whole-second rounding the time cell and SWOLF share.

import type { Length } from '../datafile';

export interface LengthRow {
  /** 1, 2, … across the whole workout; 0 for a rest length, which is never numbered or counted. */
  readonly number: number;
  /** Whole seconds, half away from zero (times are ≥ 0); NaN when not recorded. */
  readonly time: number;
  /** Rounded time + stroke count; NaN for a rest length or without a stroke count or time. */
  readonly swolf: number;
}

export function lengthRows(lengths: readonly Length[]): LengthRow[] {
  let n = 0;
  return lengths.map((l) => {
    const time = l.time === undefined ? NaN : Math.round(l.time);
    return { number: l.rest ? 0 : ++n, time, swolf: l.rest || l.strokes === undefined ? NaN : time + l.strokes };
  });
}

/** The Strokes and SWOLF columns show when at least one active length has a stroke count. */
export function hasStrokeCounts(lengths: readonly Length[]): boolean {
  return lengths.some((l) => !l.rest && l.strokes !== undefined);
}
