// Split presets and rules (F-46 LS-5, LS-6; F-64 LL-4, LL-6; AP-20). Pure, lazy (the splits chunk
// only, so the boundary rule stays out of the entry). Lists and defaults are metric in both unit systems; a pool swim splits in its pool's unit.

import type { WorkoutMeta } from '../datafile';

export interface Pool {
  /** Shown length P in the pool's unit; 33 is taken as exactly 100/3 (OQ-165). */
  readonly shown: number;
  readonly unit: 'm' | 'yd';
  /** The device's recorded length in metres: the step of the distance stream (ADR-046). */
  readonly recorded: number;
}

export interface Preset {
  readonly label: string;
  /** Split length in the distance stream, metres. */
  readonly length: number;
  /** Pool only: whole lengths per split and the distance a whole split shows, in the pool's unit. */
  readonly lengths?: number;
  readonly shown?: number;
  /** F-69: a time split; `length` is then moving seconds, over the moving-time stream. */
  readonly time?: true;
}

export interface SplitChoice {
  /** The offered presets, in list order: those not longer than the workout, and always the default. */
  readonly presets: readonly Preset[];
  /** Index of the sport's default in `presets`. */
  readonly index: number;
  readonly pool: Pool | null;
}

const KM = [1000, 5000, 10000, 20000];
// [list, default]; every other sport and multisport take KM (OQ-167: e-biking is cycling).
const CYCLING: [number[], number] = [[1000, 2000, 2500, 5000, 10000, 15000, 20000, 25000, 30000, 40000, 50000, 100000], 5000];
const LISTS: Readonly<Record<string, [number[], number]>> = {
  running: [[400, 1000, 5000, 10000, 20000, 21097.5], 1000],
  rowing: [[250, 500, 1000], 500],
  cycling: CYCLING,
  e_biking: CYCLING,
  swimming: [[50, 100, 200, 500, 1000], 100],
};
const TIME_MIN = [1, 5, 10, 15, 30, 60]; // F-69 TS-3
const POOL = [100, 200, 400, 500, 1000];

import { t } from './texts';

function label(metres: number, swim: boolean): string {
  if (metres === 21097.5) return t('halfMarathon');
  return swim || metres < 1000 ? metres + ' m' : metres / 1000 + ' km';
}

/** The summary's pool, when a swim has one shared pool length (F-44 PS-3); else null (open water). */
export function poolOf(meta: Pick<WorkoutMeta, 'sport' | 'summary'>): Pool | null {
  const s = meta.summary;
  const recorded = s.pool_length_recorded_m;
  const len = s.pool_length_m ?? s.pool_length_yd;
  if (meta.sport !== 'swimming' || !recorded || !len) return null;
  return { shown: len === 33 ? 100 / 3 : len, unit: s.pool_length_m === undefined ? 'yd' : 'm', recorded };
}

/** One pool preset (LS-6): n whole lengths, at least 1; shown as the target unless n·P is more than 1 % off. */
function poolPreset(target: number, pool: Pool): Preset {
  const n = Math.max(1, Math.round(target / pool.shown));
  const actual = n * pool.shown;
  const shown = Math.abs(actual - target) > 0.01 * target ? actual : target;
  return { label: Math.round(shown) + ' ' + pool.unit, length: n * pool.recorded, lengths: n, shown };
}

/** F-69 TS-3: the window's moving time for the offer rule; 0 (no time group) without the stream or for a lap swim. */
export function movingOf(m: Float64Array | undefined, a: number, b: number, lapSwim: boolean): number {
  return m && !lapSwim && b > a ? m[b] - m[a] : 0;
}

/** The drop-down's presets for the workout (LS-5): `advance` is d̄[N−1] − d̄[0] in metres. `moving`
 *  (F-69 TS-3) is the window's moving time in seconds: the time presets not longer than it follow the
 *  distance ones, from 60 s and never for a pool swim. */
export function splitChoice(meta: Pick<WorkoutMeta, 'sport' | 'summary'>, advance: number, moving = 0): SplitChoice {
  const pool = poolOf(meta);
  const [list, def] = Object.hasOwn(LISTS, meta.sport) ? LISTS[meta.sport] : [KM, 1000];
  const all = pool ? POOL.map((t) => poolPreset(t, pool)) : list.map((m) => ({ label: label(m, meta.sport === 'swimming'), length: m }));
  const d = pool ? 0 : list.indexOf(def);
  const presets = all.filter((p, i) => i === d || p.length <= advance + 1e-6);
  const time: Preset[] = pool || moving < 60 ? [] : TIME_MIN.filter((m) => m * 60 <= moving + 1e-6).map((m) => ({ label: m + ' min', length: m * 60, time: true }));
  return { presets: [...presets, ...time], index: presets.indexOf(all[d]), pool };
}

/** Split boundaries over d̄ (F-18 L-4) for splits of `length` metres within the sample window
 *  [a, b] (F-64 LL-4; the whole stream by default): boundary 0 is a, boundary k the first sample
 *  with d̄ − d̄[a] ≥ k·length − tol (k·length per k, never accumulated), and split k is
 *  [boundary k−1, boundary k]. A last partial split ends at b when its distance is > tol; a tail
 *  without advance is in no split. A sample that crosses several multiples ends one split only, so
 *  no split is empty. O(b − a). The default tol absorbs float error only, far below the 0.1 m
 *  quantisation; a pool split passes 1 % of a recorded length (F-46 LS-6). */
export function splitBounds(d: Float64Array, length: number, tol = 1e-6, a = 0, b = d.length - 1): number[] {
  const out = [a];
  const d0 = d[a];
  let k = 1;
  for (let i = a + 1; i <= b; i++) {
    if (d[i] - d0 < k * length - tol) continue;
    out.push(i);
    while (d[i] - d0 >= k * length - tol) k++;
  }
  if (d[b] - d[out[out.length - 1]] > tol) out.push(b);
  return out;
}

/** Split boundaries for a preset (F-18 L-4; LS-6: a pool split ends within 1 % of a recorded length), within [a, b]. */
export function presetBounds(d: Float64Array, p: Preset, pool: Pool | null, a?: number, b?: number): number[] {
  return splitBounds(d, p.length, tolerance(pool), a, b);
}

const tolerance = (pool: Pool | null): number => (pool ? 0.01 * pool.recorded : 1e-6);

/** True when the last split of `bounds` is partial (F-51 SC-4): it reaches no further multiple of
 *  the preset than the boundary before it, so splitBounds ended it at the window's end. */
export function partialLast(d: Float64Array, bounds: readonly number[], p: Preset, pool: Pool | null): boolean {
  const n = bounds.length;
  const tol = tolerance(pool);
  const multiples = (i: number): number => Math.floor((d[i] - d[bounds[0]] + tol) / p.length);
  return n > 1 && multiples(bounds[n - 1]) <= multiples(bounds[n - 2]);
}

/** A pool split's distance in the pool's unit (LS-6): a whole split shows the preset's distance,
 *  a partial one its whole lengths × P. `metres` is the split's distance in the stream. */
export function poolDistance(metres: number, p: Preset, pool: Pool): number {
  const n = Math.round(metres / pool.recorded);
  return n === p.lengths ? p.shown! : n * pool.shown;
}
