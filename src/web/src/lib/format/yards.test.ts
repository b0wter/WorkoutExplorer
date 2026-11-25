// F-72 swim distance in yards: the one rule (YD-1), formatting per language (YD-5), whole-yard
// axis ticks (YD-6), metric output unchanged (YD-4), the surfaces that take the scope sport (YD-2, YD-3).
import { afterEach, describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile, type WorkoutData } from '../datafile';
import { langSource, resetI18n, type Lang } from '../i18n';
import { lapsGroup, summaryItems } from '../page';
import { readoutFields } from '../charts';
import { xScale } from '../../components/charts/uplot-options';
import { distanceUnit, formatDistance } from '.';

const use = (l: Lang): void => langSource(() => l, () => {});
afterEach(resetI18n);
const golden = (name: string): WorkoutData => {
  const r = decodeDataFile(loadGolden().find((x) => x.name === name)!.bin);
  if (!r.ok) throw new Error(name);
  return r.data;
};
const YD = 0.9144;

describe('the rule (YD-1)', () => {
  it.each([
    ['imperial', 'swimming', 'yd'],
    ['imperial', 'running', 'mi'],
    ['imperial', 'cycling', 'mi'],
    ['imperial', 'multisport', 'mi'],
    ['imperial', undefined, 'mi'],
    ['metric', 'swimming', 'km'],
    ['metric', 'running', 'km'],
  ] as const)('%s + %s -> %s', (u, sport, unit) => expect(distanceUnit(u, sport)).toBe(unit));

  it('metric output is byte-identical with and without the sport', () => {
    for (const m of [0, 999.4, 1500, 42195, NaN]) {
      expect(formatDistance(m, 'metric', 'swimming')).toBe(formatDistance(m, 'metric'));
      expect(formatDistance(m, 'imperial', 'running')).toBe(formatDistance(m, 'imperial'));
    }
    expect(formatDistance(1500, 'metric', 'swimming')).toBe('1.50');
  });
});

describe('format per language (YD-5, F72-Q3)', () => {
  const at = (n: number): number => n * YD;
  it('English groups with a comma', () => {
    expect(['999', '1,000', '1,650', '10,000'].map((s) => formatDistance(at(Number(s.replace(',', ''))), 'imperial', 'swimming'))).toEqual(['999', '1,000', '1,650', '10,000']);
  });
  it('1,500 m is 1,640 yd; 66 lengths of 22.86 m is 1,650 yd; NaN is a dash', () => {
    expect(formatDistance(1500, 'imperial', 'swimming')).toBe('1,640');
    expect(formatDistance(66 * 22.86, 'imperial', 'swimming')).toBe('1,650');
    expect(formatDistance(100, 'imperial', 'swimming')).toBe('109');
    expect(formatDistance(NaN, 'imperial', 'swimming')).toBe('–');
  });
  it.each([
    ['de', ['999', '1.000', '1.650', '10.000']],
    ['es', ['999', '1650', '1650', '10.000']], // CLDR: no grouping below 10,000 (F72-Q3); 1,000 yd is "1000"
    ['fr', ['999', '1 000', '1 650', '10 000']],
  ] as const)('%s', (l, want) => {
    use(l);
    const got = [999, 1000, 1650, 10000].map((n) => formatDistance(at(n), 'imperial', 'swimming'));
    expect(got.map((s) => s.normalize('NFKC'))).toEqual(want.map((s) => s.normalize('NFKC')).map((s, i) => (l === 'es' && i === 1 ? '1000' : s)));
  });
  it('a language switch re-formats', () => {
    expect(formatDistance(at(10000), 'imperial', 'swimming')).toBe('10,000');
    use('de');
    expect(formatDistance(at(10000), 'imperial', 'swimming')).toBe('10.000');
  });
});

describe('axis (YD-6)', () => {
  const axis = { kind: 'distance', min: 0, max: 1500, x: new Float64Array(0), at: new Float64Array(0), keys: new Float64Array(0), index: new Uint32Array(0) } as never;
  it('steps through whole yards only, labels like a summary value', () => {
    const s = xScale(axis, 'imperial', 'swimming');
    for (const incr of s.incrs) expect(Number.isInteger(Math.round((incr / YD) * 1e6) / 1e6)).toBe(true);
    expect(s.label(1000 * YD, 100 * YD)).toBe('1,000 yd');
    expect(s.label(10000 * YD, 1000 * YD)).toBe('10,000 yd');
    expect(s.label(250 * YD, 50 * YD)).toBe('250 yd');
  });
  it('other sports and metric keep km and mi', () => {
    expect(xScale(axis, 'imperial', 'running').label(1609.344, 1609.344)).toBe('1 mi');
    expect(xScale(axis, 'imperial').label(1609.344, 1609.344)).toBe('1 mi');
    expect(xScale(axis, 'metric', 'swimming').label(2000, 1000)).toBe('2 km');
  });
});

describe('surfaces (YD-2, YD-3)', () => {
  it('a pool swim: summary and laps in yards, pool cell and pace unchanged; metric km', () => {
    const d = golden('pool-swim');
    const sum = summaryItems(d.meta.sport, d.meta.summary, 'imperial');
    expect(sum.find((i) => i.Key === 'distance')?.Unit).toBe('yd');
    expect(sum.find((i) => i.Key === 'distance')?.Value).toMatch(/^[\d,]+$/);
    expect(sum.find((i) => i.Key === 'pool')?.Unit).toBe('m');
    expect(sum.find((i) => i.Key === 'speed')?.Unit).toBe('/100 yd');
    expect(summaryItems(d.meta.sport, d.meta.summary, 'metric').find((i) => i.Key === 'distance')?.Unit).toBe('km');
    expect(lapsGroup(d.meta.sport, d.meta.laps, 'imperial')?.Columns.map((c) => c.Label)).toContain('Distance (yd)');
    expect(lapsGroup(d.meta.sport, d.meta.laps, 'metric')?.Columns.map((c) => c.Label)).toContain('Distance (km)');
  });
  it('a run and a ride stay in miles', () => {
    for (const n of ['run-30min', 'ride-30min']) {
      const d = golden(n);
      expect(summaryItems(d.meta.sport, d.meta.summary, 'imperial').find((i) => i.Key === 'distance')?.Unit).toBe('mi');
    }
  });
  it('multisport: whole view miles; the swim leg table yards; bike leg miles', () => {
    const d = golden('triathlon');
    expect(summaryItems(d.meta.sport, d.meta.summary, 'imperial').find((i) => i.Key === 'distance')?.Unit).toBe('mi');
    expect(lapsGroup('swimming', d.meta.laps.slice(0, 1), 'imperial')?.Columns.map((c) => c.Label)).toContain('Distance (yd)');
    expect(lapsGroup('cycling', d.meta.laps.slice(0, 1), 'imperial')?.Columns.map((c) => c.Label)).toContain('Distance (mi)');
  });
  it('readout: distance unit follows the view sport, not the cursor section', () => {
    const d = golden('triathlon');
    const unit = (view?: string): string | undefined => readoutFields(d, 'imperial', view, view ?? d.meta.sport).find((f) => f.key === 'distance')?.unit;
    expect(unit()).toBe('mi');
    expect(unit('swimming')).toBe('yd');
    expect(unit('cycling')).toBe('mi');
    // Whole view scrubbing over the swim: pace per 100 yd but distance in miles (YD-2, F72-Q2).
    const f = readoutFields(d, 'imperial', 'swimming');
    expect(f.find((x) => x.key === 'distance')?.unit).toBe('mi');
    expect(f.find((x) => x.key === 'speed')?.unit).toBe('/100 yd');
  });
});
