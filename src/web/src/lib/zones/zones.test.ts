// lib/zones (F-49 ZN-5): the default model, time in zone from the moving-time stream, windows, and
// the zone values of the synthetic corpus (AP-10 acceptance criteria 1 to 5, 8 to 10).
import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../datafile';
import { boundText, timeInZone, zoneStarts } from '.';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  const r = g && decodeDataFile(g.bin);
  if (!r?.ok) throw new Error(`no golden ${name}`);
  return r.data;
}

/** The time with a sample of `values` in (a, b], counted directly: moving time per step. */
function timeWithSamples(d: WorkoutData, values: Float64Array, a: number, b: number): number {
  let s = 0;
  for (let i = a + 1; i <= b; i++) if (!Number.isNaN(values[i])) s += d.movingTime![i] - d.movingTime![i - 1];
  return s;
}

/** Time in zone over [a, b] with fresh output. */
function tiz(starts: readonly number[], values: Float64Array, d: Pick<WorkoutData, 'time' | 'movingTime'>, a: number, b: number): { total: number; out: number[] } {
  const out = new Float64Array(starts.length + 1);
  const total = timeInZone(starts, values, d, a, b, out);
  return { total, out: [...out] };
}

describe('model (OQ-188)', () => {
  it('heart rate: 5 zones from 60, 70, 80, 90 % of max, half away from zero', () => {
    expect(zoneStarts('heart_rate', { basis: 186 })).toEqual([112, 130, 149, 167]); // 111.6, 130.2, 148.8, 167.4
    expect(zoneStarts('heart_rate', { basis: 185 })).toEqual([111, 130, 148, 167]); // 111, 129.5, 148, 166.5
  });

  it('power: 7 zones from 55, 75, 90, 105, 120, 150 % of threshold (Coggan)', () => {
    expect(zoneStarts('power', { basis: 265 })).toEqual([146, 199, 239, 278, 318, 398]); // 145.75, 198.75, 238.5, 278.25, 318, 397.5
  });

  it('a file list is used as written', () => {
    expect(zoneStarts('power', { from: [139, 189] })).toEqual([139, 189]);
  });

  it('bounds: zone 1 its top only, the last zone its bottom only, whole values between', () => {
    const s = [114, 133, 152, 171];
    expect([0, 1, 2, 3, 4].map((z) => boundText(s, z, 'bpm'))).toEqual(['≤ 113 bpm', '114–132 bpm', '133–151 bpm', '152–170 bpm', '≥ 171 bpm']);
  });
});

describe('time in zone (ZN-5)', () => {
  // 6 samples, 1 s apart; moving time stands still from 2 to 3 (a pause inside the samples).
  const time = new Float64Array([0, 1, 2, 3, 4, 5]);
  const movingTime = new Float64Array([0, 1, 2, 2, 3, 4]);
  const hr = new Float64Array([100, 99, 120, 150, NaN, 200]);
  const starts = [120, 150];

  it('a value on a start belongs to the zone above; below the first start counts in zone 1; missing samples count nowhere', () => {
    // i = 1: 99 → z1 (+1); 2: 120 → z2 (+1); 3: 150 → z3 (+0, paused); 4: missing; 5: 200 → z3 (+1).
    expect(tiz(starts, hr, { time, movingTime }, 0, 5)).toEqual({ total: 3, out: [1, 1, 1] });
  });

  it('counts moving time, not samples: the pause adds nothing', () => {
    const { out } = tiz(starts, hr, { time, movingTime }, 2, 3);
    expect(out).toEqual([0, 0, 0]);
  });

  it('without a moving-time stream: the time step, 0 across a gap (5 s or more)', () => {
    const t = new Float64Array([0, 1, 2, 7, 8, 9]);
    expect(tiz(starts, hr, { time: t }, 0, 5)).toEqual({ total: 3, out: [1, 1, 1] });
  });

  it('the window narrows the count to (a, b]; reuses its output without allocating', () => {
    expect(tiz(starts, hr, { time, movingTime }, 1, 2)).toEqual({ total: 1, out: [0, 1, 0] });
    const out = new Float64Array(3).fill(9);
    timeInZone(starts, hr, { time, movingTime }, 4, 5, out);
    expect([...out]).toEqual([0, 0, 1]);
  });
});

describe('corpus (AP-10 acceptance criteria)', () => {
  it('AC 1: Apple-style, 5 and 6 boundaries: the same 5 zones from the configured max; times add up to the time with heart rate', () => {
    for (const name of ['zones-apple-run-5', 'zones-apple-run-6']) {
      const d = golden(name);
      expect(d.meta.zones, name).toEqual({ heart_rate: { from: [114, 133, 152, 171] } });
      const starts = zoneStarts('heart_rate', d.meta.zones.heart_rate!);
      // Not from the session's own maximum, which is above the configured one.
      expect(starts).not.toEqual(zoneStarts('heart_rate', { basis: d.meta.summary.max_heart_rate! }));
      const hr = d.metrics.heart_rate!;
      const n = d.sampleCount - 1;
      const { total, out } = tiz(starts, hr, d, 0, n);
      expect(total).toBe(timeWithSamples(d, hr, 0, n));
      expect(out.reduce((x, y) => x + y)).toBe(total);
      expect(out.filter((v) => v > 0).length).toBeGreaterThan(1);
    }
  });

  it("AC 2: Wahoo-style ride: the messages' heart-rate and power bounds", () => {
    expect(golden('zones-wahoo-ride').meta.zones).toEqual({ heart_rate: { from: [121, 141, 161, 176] }, power: { from: [139, 189, 226, 264, 301, 376] } });
  });

  it('AC 3, 4: threshold power only gives power zones by the model; zones_target both kinds by the model', () => {
    expect(golden('zones-ride-ftp-only').meta.zones).toEqual({ power: { basis: 265 } });
    expect(golden('zones-ride-target').meta.zones).toEqual({ heart_rate: { basis: 186 }, power: { basis: 240 } });
  });

  it('AC 5: a file without zone values has none', () => {
    expect(golden('ride-30min').meta.zones).toEqual({});
  });

  it('AC 10: a truncated file and a multisport workout with a sharer basis count without error', () => {
    const cut = golden('zones-wahoo-ride-cut-60');
    const p = cut.metrics.power!;
    expect(tiz(zoneStarts('power', cut.meta.zones.power!), p, cut, 0, cut.sampleCount - 1).total).toBe(timeWithSamples(cut, p, 0, cut.sampleCount - 1));
    // OQ-191: one set for the whole workout; OQ-190: the transitions count where heart rate exists.
    const tri = golden('triathlon');
    const hr = tri.metrics.heart_rate!;
    const n = tri.sampleCount - 1;
    const { total } = tiz(zoneStarts('heart_rate', { basis: 190 }), hr, tri, 0, n);
    expect(total).toBe(timeWithSamples(tri, hr, 0, n));
    const t1 = tri.meta.sessions[1];
    expect(timeWithSamples(tri, hr, t1.start_index, t1.end_index - 1)).toBeGreaterThan(0);
  });

  it('AC 8: a range counts its own samples only; two adjacent ranges add up to the whole', () => {
    const d = golden('zones-wahoo-ride');
    const starts = zoneStarts('power', d.meta.zones.power!);
    const p = d.metrics.power!;
    const n = d.sampleCount - 1;
    const m = 400;
    const whole = tiz(starts, p, d, 0, n);
    const left = tiz(starts, p, d, 0, m);
    const right = tiz(starts, p, d, m, n);
    expect(left.total + right.total).toBe(whole.total);
    expect(left.out.map((v, z) => v + right.out[z])).toEqual(whole.out);
  });
});
