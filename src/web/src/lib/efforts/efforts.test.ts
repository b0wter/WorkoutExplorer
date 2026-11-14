// lib/efforts (F-50 EF-1 to EF-6; AP-11): hand-computed distance efforts on a synthetic run, power on
// the moving-second grid, legs and the corpus (acceptance criteria 1 to 3, 5, 7), the curve's cost.
import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData, type WorkoutMeta } from '../datafile';
import { bestPower, curveDurations, distanceEfforts, effortLegs, longest, powerEfforts, powerLegs, type Effort } from '.';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  const r = g && decodeDataFile(g.bin);
  if (!r?.ok) throw new Error(`no golden ${name}`);
  return r.data;
}

const meta = (sport: string, sessions: { sport: string; start_index: number; start: number }[] = []): WorkoutMeta =>
  ({ sport, sessions: sessions.map((s) => ({ ...s, sub_sport: '', end: 0, end_index: 0, totals: {} })) }) as unknown as WorkoutMeta;

/** A workout from samples [elapsed s, moving s, distance m, power W]. */
function workout(sport: string, rows: readonly (readonly number[])[], sessions?: Parameters<typeof meta>[1]): WorkoutData {
  const col = (k: number) => Float64Array.from(rows, (r) => r[k] ?? NaN);
  const metrics = rows.some((r) => r.length > 3) ? { distance: col(2), power: col(3) } : { distance: col(2) };
  return { sampleCount: rows.length, time: col(0), gaps: new Uint32Array(), movingTime: col(1), metrics, meta: meta(sport, sessions) } as WorkoutData;
}

const all = (d: WorkoutData) => effortLegs(d, 0, d.sampleCount - 1);
const dist = (d: WorkoutData) => distanceEfforts(d, all(d));
const power = (d: WorkoutData) => powerEfforts(d, powerLegs(d, all(d)));
const byName = (es: readonly Effort[]) => Object.fromEntries(es.map((e) => [e.name, e]));

/** AC 1: a 12 km run, sampled every 1, 2, 3 s in turn, plus a sample on each speed change:
 *  0–3000 m at 3.125 m/s, 3000–4000 m at 5 m/s with a 60 s pause at 3500 m, 4000–5000 m at 4 m/s,
 *  5000–12000 m at 3.5 m/s. */
function run12k(): WorkoutData {
  const legs: [metres: number, speed: number][] = [[3000, 3.125], [500, 5], [0, 60], [500, 5], [1000, 4], [7000, 3.5]];
  const rows: number[][] = [[0, 0, 0]];
  let [t, m, d, step] = [0, 0, 0, 0];
  for (const [metres, speed] of legs) {
    if (metres === 0) {
      // The pause: elapsed time runs, moving time and distance stand.
      for (const end = t + speed; t < end; ) rows.push([(t = Math.min(end, t + 10)), m, d]);
      continue;
    }
    const t1 = t + metres / speed;
    const [t0, d0, m0] = [t, d, m];
    while (t < t1) {
      t = Math.min(t1, t + 1 + (step++ % 3));
      rows.push([t, m0 + (t - t0), (d = d0 + (t - t0) * speed)]);
    }
    m = m0 + (t1 - t0);
  }
  return workout('running', rows);
}

describe('distance efforts (EF-4; AC 1, 3)', () => {
  const d = run12k();
  const e = byName(dist(d));
  const at = (metres: number) => d.metrics.distance!.findIndex((x) => x >= metres - 1e-9);

  it('a 12 km run has 1 km, 1 mi, 5 km and 10 km, not half marathon or marathon', () => {
    expect(Object.keys(e)).toEqual(['1 km', '1 mi', '5 km', '10 km']);
  });

  it('1 km: the fast kilometre, its 60 s pause excluded (moving time); starts at 960 s', () => {
    expect(e['1 km'].value).toBeCloseTo(200, 9);
    expect(e['1 km'].start).toBeCloseTo(960, 9);
    expect([e['1 km'].a, e['1 km'].b]).toEqual([at(3000), at(4000)]);
  });

  it('1 mi: the fast kilometre and 609.344 m at 4 m/s, the end interpolated between samples', () => {
    expect(e['1 mi'].value).toBeCloseTo(200 + 609.344 / 4, 9);
    expect(e['1 mi'].start).toBeCloseTo(960, 9);
    // Covering range: the end lies strictly inside [b − 1, b].
    const D = d.metrics.distance!;
    const b = e['1 mi'].b;
    expect(D[b - 1]).toBeLessThan(4609.344);
    expect(D[b]).toBeGreaterThan(4609.344);
  });

  it('5 km: 3000–8000 m; 10 km: 2000–12000 m, the start interpolated', () => {
    expect(e['5 km'].value).toBeCloseTo(200 + 250 + 3000 / 3.5, 9);
    expect(e['5 km'].start).toBeCloseTo(960, 9);
    expect(e['10 km'].value).toBeCloseTo(1000 / 3.125 + 450 + 2000, 9);
    expect(e['10 km'].start).toBeCloseTo(640, 9);
    expect(e['10 km'].b).toBe(d.sampleCount - 1);
    const D = d.metrics.distance!;
    const a = e['10 km'].a;
    expect(D[a]).toBeLessThanOrEqual(2000);
    expect(D[a + 1]).toBeGreaterThan(2000);
  });

  it('a stretch starts after a pause and ends before one; ties go to the earliest start', () => {
    // Two equal kilometres at 4 m/s, 1 s samples; a pause at the start of the second.
    const rows: number[][] = [];
    for (let t = 0; t <= 250; t++) rows.push([t, t, t * 4]);
    for (let t = 251; t <= 300; t++) rows.push([t, 250, 1000]); // pause
    for (let t = 301; t <= 550; t++) rows.push([t, t - 50, 1000 + (t - 300) * 4]);
    const w = workout('running', rows);
    const k = byName(dist(w))['1 km'];
    expect([k.value, k.start, k.a, k.b]).toEqual([250, 0, 0, 250]);
    // The second at 5 m/s, then a pause: it starts at the pause's last sample and ends at the next one's first.
    const fast = rows.slice(0, 301);
    for (let t = 301; t <= 500; t++) fast.push([t, t - 50, 1000 + (t - 300) * 5]);
    for (let t = 501; t <= 520; t++) fast.push([t, 450, 2000]);
    const k2 = byName(dist(workout('running', fast)))['1 km'];
    expect([k2.value, k2.start, k2.a, k2.b]).toEqual([200, 300, 300, 500]);
  });

  it('only run, walk and hike legs (not ride, rowing, swim); a leg too short has none', () => {
    const rows = run12k();
    for (const sport of ['walking', 'hiking']) expect(dist({ ...rows, meta: meta(sport) })).toHaveLength(4);
    for (const sport of ['cycling', 'rowing', 'swimming']) expect(dist({ ...rows, meta: meta(sport) })).toEqual([]);
    expect(Object.keys(byName(distanceEfforts(rows, effortLegs(rows, 0, rows.metrics.distance!.findIndex((x) => x >= 1500)))))).toEqual(['1 km']);
  });
});

describe('power on the moving-second grid (EF-3, EF-5)', () => {
  it('missing samples count as 0 W; a pause adds no time', () => {
    // 1 s samples of 100 W, one missing, then a 10 s pause at 900 W (no moving time), then 200 W.
    const rows: number[][] = [[0, 0, 0, NaN]];
    for (let t = 1; t <= 10; t++) rows.push([t, t, 0, t === 5 ? NaN : 100]);
    for (let t = 11; t <= 20; t++) rows.push([t, 10, 0, 900]);
    for (let t = 21; t <= 30; t++) rows.push([t, t - 10, 0, 200]);
    const w = workout('cycling', rows);
    const g = powerLegs(w, all(w));
    expect(longest(g)).toBe(20);
    expect(bestPower(g, 20)![0]).toBeCloseTo((9 * 100 + 10 * 200) / 20, 9);
    expect(bestPower(g, 10)![0]).toBe(200);
    // 12 s: the 200 W block and the 2 s before it.
    expect(bestPower(g, 12)![0]).toBeCloseTo((2 * 100 + 10 * 200) / 12, 9);
    const p = byName(power(w))['5 s'];
    expect([p.value, p.start, p.a, p.b]).toEqual([200, 20, 20, 25]);
    expect(bestPower(g, 21)).toBeNull();
  });

  it('irregular samples spread their energy over whole seconds; ties go to the earliest', () => {
    // A 3 s sample of 300 W, then 1 s samples of 100 W, then a 3 s sample of 300 W again.
    const w = workout('cycling', [[0, 0, 0, NaN], [3, 3, 0, 300], [4, 4, 0, 100], [5, 5, 0, 100], [8, 8, 0, 300]]);
    const g = powerLegs(w, all(w));
    expect([...g[0].S]).toEqual([0, 300, 600, 900, 1000, 1100, 1400, 1700, 2000]);
    expect(bestPower(g, 3)).toEqual([300, 0, 0]);
    expect(bestPower(g, 2)).toEqual([300, 0, 0]);
  });

  it('table durations only when some leg has that much moving time; values match the grid', () => {
    const d = golden('ride-30min');
    const p = power(d);
    expect(p.map((e) => e.name)).toEqual(['5 s', '1 min', '5 min', '20 min']);
    const g = powerLegs(d, all(d));
    for (const [e, s] of [[p[0], 5], [p[1], 60]] as const) expect(e.value).toBe(bestPower(g, s)![0]);
    // Covering range: the whole effort is inside [a, b].
    const M = d.movingTime!;
    for (const e of p) {
      expect(e.b).toBeGreaterThan(e.a);
      expect(M[e.b] - M[e.a]).toBeGreaterThanOrEqual(Number(e.name.split(' ')[0]) * (e.name.endsWith('min') ? 60 : 1));
    }
  });
});

describe('curve durations (EF-6)', () => {
  it('every second to 30 s, then steps of at most 3 %, the table durations and the longest', () => {
    const ds = curveDurations(43199);
    expect(ds.slice(0, 31)).toEqual(Array.from({ length: 31 }, (_, i) => i + 1));
    for (let i = 1; i < ds.length; i++) expect(ds[i] - ds[i - 1] === 1 || ds[i] / ds[i - 1] <= 1.03, String(ds[i])).toBe(true);
    for (const d of [5, 60, 300, 1200, 3600, 43199]) expect(ds).toContain(d);
    expect(ds.at(-1)).toBe(43199);
    expect(ds.length).toBeLessThan(300);
    expect(curveDurations(0)).toEqual([]);
  });
});

describe('legs and corpus (EF-1, EF-2; AC 2, 3, 5, 7)', () => {
  it('AC 2: a ride without power and a swim have no efforts', () => {
    const r = golden('ride-30min');
    const noPower = { ...r, metrics: { ...r.metrics, power: undefined } };
    expect([dist(noPower), power(noPower)]).toEqual([[], []]);
    const s = golden('pool-swim');
    expect([dist(s), power(s)]).toEqual([[], []]);
  });

  it('AC 3: in triathlon.fit no effort crosses a leg or enters a transition; distance in the run leg only, power not in the swim', () => {
    const d = golden('triathlon');
    const legs = all(d);
    expect(legs.map((g) => g.sport)).toEqual(['swimming', 'transition', 'cycling', 'transition', 'running']);
    const inside = (e: Effort, sport: string) => legs.some((g) => g.sport === sport && g.a <= e.a && e.b <= g.b);
    const ds = dist(d);
    expect(ds.length).toBeGreaterThan(0);
    for (const e of ds) expect(inside(e, 'running'), e.name).toBe(true);
    const ps = power(d);
    expect(ps.length).toBeGreaterThan(0);
    for (const e of ps) expect(inside(e, 'cycling') || inside(e, 'running'), e.name).toBe(true);
    expect(powerLegs(d, legs).map((g) => g.sport)).not.toContain('transition');
  });

  it('AC 5: the window bounds every effort and equals the trimmed file (rebased) value for value', () => {
    const d = golden('ride-30min');
    const [lo, hi] = [300, 1400];
    const inWindow = powerEfforts(d, powerLegs(d, effortLegs(d, lo, hi)));
    for (const e of inWindow) expect(e.a >= lo && e.b <= hi, e.name).toBe(true);
    expect(inWindow.map((e) => e.name)).toEqual(['5 s', '1 min', '5 min']);
    // The public file of a trim: the samples [lo, hi], rebased (ADR-047).
    const cut = (x: Float64Array) => x.slice(lo, hi + 1).map((v) => v - x[lo]);
    const t: WorkoutData = { ...d, sampleCount: hi - lo + 1, time: cut(d.time), movingTime: cut(d.movingTime!), metrics: { distance: cut(d.metrics.distance!), power: d.metrics.power!.slice(lo, hi + 1) } };
    const trimmed = power(t);
    expect(trimmed.map((e) => [e.value, e.a + lo, e.b + lo, e.start + d.time[lo]])).toEqual(inWindow.map((e) => [e.value, e.a, e.b, e.start]));
    const full = powerLegs(d, all(d));
    expect(longest(powerLegs(t, all(t)))).toBeLessThan(longest(full));
  });

  it('AC 7: truncated files give efforts without error', () => {
    for (const name of ['ride-30min-cut-60', 'run-30min-cut-60', 'triathlon-cut-90', 'zones-wahoo-ride-cut-60']) {
      const d = golden(name);
      expect(dist(d).length + power(d).length, name).toBeGreaterThan(0);
    }
  });

  it('ride-12h: the curve over about 280 durations; computation time logged', () => {
    const d = golden('ride-12h');
    const t0 = performance.now();
    const g = powerLegs(d, all(d));
    const ds = curveDurations(longest(g));
    const ws = ds.map((s) => bestPower(g, s)![0]);
    const ms = performance.now() - t0;
    console.info(`ride-12h curve: ${ds.length} durations in ${ms.toFixed(1)} ms`);
    // Not strictly non-increasing (a longer window may hold two peaks), but the 1 s value is the top.
    expect(Math.max(...ws)).toBe(ws[0]);
    expect(ds.length).toBeGreaterThan(250);
  });
});
