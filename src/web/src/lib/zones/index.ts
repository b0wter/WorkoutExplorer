// Heart-rate and power zones (F-49 ZN-5, ADR-049 item 4; AP-10): the one default model and time in
// zone. Pure; imports lib/datafile types only, so it stays out of the entry (ZN-9).

import type { WorkoutData, ZoneEntry, ZoneKind } from '../datafile';

/** OQ-188: where zones 2 … k start, in % of max heart rate (5 zones; 50 % is zone 1's own
 *  bottom, and below it counts in zone 1) or of threshold power (7 zones, Coggan). */
export const MODEL: Readonly<Record<ZoneKind, readonly number[]>> = {
  heart_rate: [60, 70, 80, 90],
  power: [55, 75, 90, 105, 120, 150],
};

/** F-03 §7's gap (GAP_SECONDS), copied: a value import would tie this module to the entry. */
const GAP = 5;

/** The lowest whole bpm or W of zones 2 … k: the file's, or the model's from a basis, half away
 *  from zero (b × p is an exact integer, so a half stays exact). */
export function zoneStarts(kind: ZoneKind, e: ZoneEntry): readonly number[] {
  return e.from ?? MODEL[kind].map((p) => Math.round((e.basis * p) / 100));
}

/** Time in zone over the window [a, b] (ZN-5): each i in (a, b] with a value adds the moving time
 *  between i − 1 and i (ADR-029), or without that stream the time step below a gap, to the zone of
 *  values[i]: 1 + the number of starts ≤ the value. Fills `out` (length starts.length + 1, seconds)
 *  and returns the total. O(b − a), no allocation. */
export function timeInZone(
  starts: readonly number[],
  values: Float64Array,
  data: Pick<WorkoutData, 'time' | 'movingTime'>,
  a: number,
  b: number,
  out: Float64Array,
): number {
  out.fill(0);
  const t = data.time;
  const m = data.movingTime;
  const k = starts.length;
  let total = 0;
  for (let i = a + 1; i <= b; i++) {
    const v = values[i];
    if (v !== v) continue; // missing sample
    let dt = m ? m[i] - m[i - 1] : t[i] - t[i - 1];
    if (!m && dt >= GAP) dt = 0;
    if (dt === 0) continue;
    let z = 0;
    while (z < k && starts[z] <= v) z++;
    out[z] += dt;
    total += dt;
  }
  return total;
}

/** A zone's bounds as printed (ZN-5): zone 1 its upper bound only, the last its lower bound only,
 *  whole values in between. */
export function boundText(starts: readonly number[], z: number, unit: string): string {
  if (z === 0) return '≤ ' + (starts[0] - 1) + ' ' + unit;
  if (z === starts.length) return '≥ ' + starts[z - 1] + ' ' + unit;
  return starts[z - 1] + '–' + (starts[z] - 1) + ' ' + unit;
}
