// Split presets and rules (F-46 LS-5, LS-6; AP-20). Pure; imports only lib/range's boundary
// function. Lists and defaults are metric in both unit systems; a pool swim splits in its pool's unit.

import type { WorkoutMeta } from '../datafile';
import { splitBounds } from '../range';

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
const POOL = [100, 200, 400, 500, 1000];

function label(metres: number, swim: boolean): string {
  if (metres === 21097.5) return 'Half marathon';
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

/** The drop-down's presets for the workout (LS-5): `advance` is d̄[N−1] − d̄[0] in metres. */
export function splitChoice(meta: Pick<WorkoutMeta, 'sport' | 'summary'>, advance: number): SplitChoice {
  const pool = poolOf(meta);
  const [list, def] = Object.hasOwn(LISTS, meta.sport) ? LISTS[meta.sport] : [KM, 1000];
  const all = pool ? POOL.map((t) => poolPreset(t, pool)) : list.map((m) => ({ label: label(m, meta.sport === 'swimming'), length: m }));
  const d = pool ? 0 : list.indexOf(def);
  const presets = all.filter((p, i) => i === d || p.length <= advance + 1e-6);
  return { presets, index: presets.indexOf(all[d]), pool };
}

/** Split boundaries for a preset (F-18 L-4; LS-6: a pool split ends within 1 % of a recorded length). */
export function presetBounds(d: Float64Array, p: Preset, pool: Pool | null): number[] {
  return splitBounds(d, p.length, pool ? 0.01 * pool.recorded : 1e-6);
}

/** A pool split's distance in the pool's unit (LS-6): a whole split shows the preset's distance,
 *  a partial one its whole lengths × P. `metres` is the split's distance in the stream. */
export function poolDistance(metres: number, p: Preset, pool: Pool): number {
  const n = Math.round(metres / pool.recorded);
  return n === p.lengths ? p.shown! : n * pool.shown;
}
