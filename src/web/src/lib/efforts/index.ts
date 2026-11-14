// Best efforts and the power curve (F-50 EF-1 to EF-6; AP-11). Pure; lazy with src/efforts, never
// in the entry. Time is moving time (ADR-029): sample i covers M[i] − M[i−1] with its own value.

import type { WorkoutData } from '../datafile';
import { carriedDistance } from '../range';
import { legs as sectionLegs, sections } from '../sections';

/** EF-4: distance efforts, metres and name (names stay in both unit systems, DS-1). */
export const DISTANCES: readonly (readonly [number, string])[] = [
  [1000, '1 km'],
  [1609.344, '1 mi'],
  [5000, '5 km'],
  [10000, '10 km'],
  [21097.5, 'Half marathon'],
  [42195, 'Marathon'],
];

/** EF-5: power durations, seconds and name. */
export const DURATIONS: readonly (readonly [number, string])[] = [
  [5, '5 s'],
  [60, '1 min'],
  [300, '5 min'],
  [1200, '20 min'],
  [3600, '60 min'],
];

const DISTANCE_SPORTS = ['running', 'walking', 'hiking'];
const NO_POWER_SPORTS = ['transition', 'swimming'];

/** A leg inside the window: samples [a, b], b > a, and its sport. */
export interface Leg {
  readonly a: number;
  readonly b: number;
  readonly sport: string;
}

/** One table row: `value` is moving seconds (distance) or W (power); `start` the elapsed time of
 *  its start on the charts' axis; [a, b] the covering sample range (EF-8). */
export interface Effort {
  readonly name: string;
  readonly metres: number;
  readonly value: number;
  readonly sport: string;
  readonly start: number;
  readonly a: number;
  readonly b: number;
}

/** EF-5: a leg with power on the moving-second grid; S[k] = the energy (J) of its first k moving
 *  seconds, so S.length − 1 = L, its whole moving seconds. m0 = M[a]. */
export interface PowerLeg extends Leg {
  readonly m0: number;
  readonly S: Float64Array;
}

/** EF-1, EF-2: the F-42 legs (or the window as one leg with meta.sport) inside [lo, hi]. */
export function effortLegs(data: Pick<WorkoutData, 'sampleCount' | 'meta'>, lo: number, hi: number): Leg[] {
  const secs = sections(data.meta);
  const all = secs ? sectionLegs(secs, data.sampleCount).map((g) => ({ a: g.start, b: g.end, sport: g.sport })) : [{ a: lo, b: hi, sport: data.meta.sport }];
  return all.map((g) => ({ a: Math.max(g.a, lo), b: Math.min(g.b, hi), sport: g.sport })).filter((g) => g.b > g.a);
}

/** True when `c` is faster than `best`; ties go to the earliest start (EF-2). */
const better = (c: Effort, best: Effort | null): boolean => !best || c.value < best.value || (c.value === best.value && c.start < best.start);

/** EF-4: the fastest stretch of exactly `metres` in each eligible leg, best over the legs. Per leg
 *  two pointers in both orientations: start on a sample with the end interpolated, and the reverse.
 *  A sample that repeats its predecessor's distance and moving time (a pause) is never an end:
 *  the stretch starts after a pause and ends before one. */
export function distanceEfforts(data: Pick<WorkoutData, 'time' | 'movingTime' | 'metrics'>, legs: readonly Leg[]): Effort[] {
  const M = data.movingTime;
  const D = carriedDistance(data.metrics.distance);
  const T = data.time;
  if (!M || !D) return [];
  const out: Effort[] = [];
  for (const [metres, name] of DISTANCES) {
    let best: Effort | null = null;
    for (const { a, b, sport } of legs) {
      if (!DISTANCE_SPORTS.includes(sport) || D[b] - D[a] < metres) continue;
      const add = (value: number, start: number, x: number, y: number): void => {
        const c = { name, metres, value, sport, start, a: x, b: y };
        if (better(c, best)) best = c;
      };
      // Start on sample i, end between j − 1 and j.
      for (let i = a, j = a; i < b; i++) {
        const target = D[i] + metres;
        if (D[i + 1] === D[i] && M[i + 1] === M[i]) continue;
        while (j < b && D[j] < target) j++;
        if (D[j] < target) break;
        const f = (target - D[j - 1]) / (D[j] - D[j - 1]);
        add(M[j - 1] + f * (M[j] - M[j - 1]) - M[i], T[i], i, j);
      }
      // End on sample j, start between i and i + 1.
      for (let j = a + 1, i = a; j <= b; j++) {
        const target = D[j] - metres;
        if (target < D[a] || (D[j - 1] === D[j] && M[j - 1] === M[j])) continue;
        while (i + 1 < j && D[i + 1] <= target) i++;
        const f = (target - D[i]) / (D[i + 1] - D[i]);
        add(M[j] - (M[i] + f * (M[i + 1] - M[i])), T[i] + f * (T[i + 1] - T[i]), i, j);
      }
    }
    if (best) out.push(best);
  }
  return out;
}

/** EF-2, EF-5: the legs with power (not transition or swimming, at least one power sample) on the
 *  moving-second grid, missing samples as 0 W (AP-7). One pass per leg. */
export function powerLegs(data: Pick<WorkoutData, 'movingTime' | 'metrics'>, legs: readonly Leg[]): PowerLeg[] {
  const M = data.movingTime;
  const P = data.metrics.power;
  if (!M || !P) return [];
  const out: PowerLeg[] = [];
  for (const g of legs) {
    if (NO_POWER_SPORTS.includes(g.sport) || !P.subarray(g.a + 1, g.b + 1).some((p) => p === p)) continue;
    const m0 = M[g.a];
    const L = Math.floor(M[g.b] - m0);
    if (L < 1) continue;
    const S = new Float64Array(L + 1);
    let e = 0;
    let k = 1;
    for (let i = g.a + 1; i <= g.b && k <= L; i++) {
      const t0 = M[i - 1] - m0;
      const t1 = M[i] - m0;
      if (!(t1 > t0)) continue;
      const p = P[i] === P[i] ? P[i] : 0;
      for (; k <= L && k <= t1; k++) S[k] = e + p * (k - t0);
      e += p * (t1 - t0);
    }
    out.push({ ...g, m0, S });
  }
  return out;
}

/** The longest leg with power, in whole moving seconds (0 without any). */
export const longest = (legs: readonly PowerLeg[]): number => legs.reduce((x, g) => Math.max(x, g.S.length - 1), 0);

/** EF-5: the highest mean power over `d` consecutive grid seconds, best over the legs, the earliest
 *  on ties: [W, leg index, first grid second], or null when no leg is that long. No allocation
 *  but the result. */
export function bestPower(legs: readonly PowerLeg[], d: number): [number, number, number] | null {
  let best = -1;
  let leg = -1;
  let at = 0;
  for (let g = 0; g < legs.length; g++) {
    const S = legs[g].S;
    for (let k = 0, end = S.length - 1 - d; k <= end; k++) {
      const s = S[k + d] - S[k];
      if (s > best) {
        best = s;
        leg = g;
        at = k;
      }
    }
  }
  return leg < 0 ? null : [best / d, leg, at];
}

/** EF-5, EF-8: the table's power rows with their covering ranges and starts. */
export function powerEfforts(data: Pick<WorkoutData, 'time' | 'movingTime'>, legs: readonly PowerLeg[]): Effort[] {
  const M = data.movingTime!;
  const T = data.time;
  const out: Effort[] = [];
  for (const [d, name] of DURATIONS) {
    const r = bestPower(legs, d);
    if (!r) continue;
    const g = legs[r[1]];
    const t0 = g.m0 + r[2];
    // a: the last sample at or before the start; b: the first at or after the end.
    let a = g.a;
    while (a + 1 < g.b && M[a + 1] <= t0) a++;
    let b = a + 1;
    while (b < g.b && M[b] < t0 + d) b++;
    const start = T[a] + ((t0 - M[a]) / (M[a + 1] - M[a])) * (T[a + 1] - T[a]);
    out.push({ name, metres: 0, value: r[0], sport: g.sport, start, a, b });
  }
  return out;
}

/** EF-6: the curve's durations: every second up to 30 s, then steps of at most 3 %, plus the
 *  table durations and `max` itself; ascending, all ≤ max. */
export function curveDurations(max: number): number[] {
  const out = new Set<number>();
  for (let d = 1; d <= max; d = d < 30 ? d + 1 : Math.max(d + 1, Math.floor(d * 1.03))) out.add(d);
  for (const [d] of DURATIONS) if (d <= max) out.add(d);
  if (max >= 1) out.add(max);
  return [...out].sort((x, y) => x - y);
}
