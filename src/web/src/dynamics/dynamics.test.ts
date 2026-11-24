// F-68 XS-9, XS-10: the lazy dynamics chunk's specs feed charts and readout from one list; the range
// panel's means come from the same list. Synthetic corpus files only (golden fixtures).
import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../test-support/golden.mjs';
import { decodeDataFile, DYNAMICS_ORDER, type WorkoutData } from '../lib/datafile';
import { presentCharts, readoutFields } from '../lib/charts';
import { buildPageViewModel } from '../lib/page/view';
import { dynamicsSpecs } from './specs';
import './index';

function golden(name: string): WorkoutData {
  const g = loadGolden().find((x) => x.name === name);
  if (!g) throw new Error(name);
  const r = decodeDataFile(g.bin);
  if (!r.ok) throw new Error(r.error.message);
  return r.data;
}

describe('specs (XS-10)', () => {
  it('metric: cm 1, ms 0, m 2, % 1', () => {
    const s = dynamicsSpecs('metric');
    expect(s.map((x) => [x.metric, x.unit, x.decimals])).toEqual([
      ['vertical_oscillation', 'cm', 1],
      ['ground_contact_time', 'ms', 0],
      ['stride_length', 'm', 2],
      ['vertical_ratio', '%', 1],
    ]);
    expect(s.map((x) => x.metric)).toEqual([...DYNAMICS_ORDER]);
    expect(s[0].format(s[0].toDisplay(8.46))).toBe('8.5');
    expect(s[2].format(s[2].toDisplay(1.2345))).toBe('1.23');
  });

  it('imperial: in and ft convert, ms and % do not', () => {
    const s = dynamicsSpecs('imperial');
    expect(s.map((x) => x.unit)).toEqual(['in', 'ms', 'ft', '%']);
    expect(s[0].toDisplay(2.54)).toBeCloseTo(1, 9);
    expect(s[2].toDisplay(1)).toBeCloseTo(3.28084, 5);
    expect(s[1].toDisplay(240)).toBe(240);
    expect(s[3].toDisplay(7.5)).toBe(7.5);
  });
});

describe('run-dynamics', () => {
  const d = golden('run-dynamics');

  it('has one chart and one readout field per present stream, below temperature, in XS-1 order', () => {
    const present = DYNAMICS_ORDER.filter((n) => d.metrics[n]);
    expect(present.length).toBeGreaterThan(0);
    const charts = presentCharts(d).map((c) => c.metric);
    expect(charts.slice(-present.length)).toEqual(present);
    expect(charts.indexOf('temperature') === -1 || charts.indexOf('temperature') < charts.indexOf(present[0])).toBe(true);
    const fields = readoutFields(d).map((f) => f.key);
    expect(fields.slice(-present.length)).toEqual(present);
  });

  it('QA-68-r1-6: the page sample (Go-compared) reserves exactly the charts and cells the client draws', () => {
    const m = buildPageViewModel(d, { title: '', description: '', expiresAt: '2026-10-30T09:15:00Z', publicId: 'x', dataFile: 'd' });
    const n = presentCharts(d).length;
    expect(m.ChartsClass).toBe('charts-' + n + ' cells-' + readoutFields(d).length + ' has-distance');
    expect(n).toBeGreaterThanOrEqual(DYNAMICS_ORDER.length);
  });

  it('the sentinel is a gap: no value is the raw 65535 or 0 (XS-1)', () => {
    for (const n of DYNAMICS_ORDER) for (const v of d.metrics[n] ?? []) if (v === v) expect(v).toBeGreaterThan(0);
  });
});

describe('files without dynamics', () => {
  it('stryd power: the power chart only, no dynamics chart', () => {
    const d = golden('stryd-power');
    expect(DYNAMICS_ORDER.every((n) => !d.metrics[n])).toBe(true);
    expect(presentCharts(d).some((c) => c.metric === 'power')).toBe(true);
  });
});
