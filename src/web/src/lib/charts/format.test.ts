// Formatting (F-05 design, Tests): elapsed time, distance, each spec's unit and decimals.

import { describe, expect, it } from 'vitest';
import { CHART_ORDER, CHART_SPECS, chartSpecs, formatDistance, formatElapsed, formatFixed } from './index';

describe('formatElapsed', () => {
  it.each([
    [0, '0:00'],
    [59, '0:59'],
    [60, '1:00'],
    [3599, '59:59'],
    [3600, '1:00:00'],
    [3661, '1:01:01'],
    [43199, '11:59:59'],
    [360000, '100:00:00'],
  ])('%s s → %s', (s, text) => {
    expect(formatElapsed(s)).toBe(text);
  });

  it('rounds down to whole seconds', () => {
    expect(formatElapsed(59.9)).toBe('0:59');
  });

  it('shows – for NaN', () => {
    expect(formatElapsed(Number.NaN)).toBe('–');
  });
});

describe('formatDistance', () => {
  it.each([
    [0, '0.00'],
    [12345.6, '12.35'],
    [999, '1.00'],
    [Number.NaN, '–'],
  ])('%s m → %s', (m, text) => {
    expect(formatDistance(m)).toBe(text);
  });
});

describe('formatFixed', () => {
  it('never shows a negative zero', () => {
    expect(formatFixed(-0.3, 0)).toBe('0');
    expect(formatFixed(-0.04, 1)).toBe('0.0');
    expect(formatFixed(-0, 0)).toBe('0');
    expect(formatFixed(-1.6, 0)).toBe('-2');
  });
});

describe('chart specs', () => {
  it('converts speed from m/s to km/h', () => {
    expect(CHART_SPECS.speed.toDisplay(10)).toBe(36);
    expect(CHART_SPECS.speed.unit).toBe('km/h');
  });

  it('uses identity for every other metric', () => {
    for (const m of CHART_ORDER) {
      if (m === 'speed') continue;
      expect(CHART_SPECS[m].toDisplay(123.4)).toBe(123.4);
    }
  });

  it.each([
    ['altitude', 'Elevation', 'm', 1234.56, '1235'],
    ['speed', 'Speed', 'km/h', 36.04, '36.0'],
    ['heart_rate', 'Heart rate', 'bpm', 142, '142'],
    ['power', 'Power', 'W', 251.4, '251'],
    ['cadence', 'Cadence', 'rpm', 88, '88'],
    ['temperature', 'Temperature', '°C', -3, '-3'],
  ] as const)('%s: label %s, unit %s, format(%s) = %s', (metric, label, unit, value, text) => {
    const spec = CHART_SPECS[metric];
    expect(spec.metric).toBe(metric);
    expect(spec.label).toBe(label);
    expect(spec.unit).toBe(unit);
    expect(spec.format(value)).toBe(text);
  });

  it.each([
    ['altitude', 0],
    ['speed', 1],
    ['heart_rate', 0],
    ['power', 0],
    ['cadence', 0],
    ['temperature', 0],
  ] as const)('%s has %s decimals', (metric, decimals) => {
    expect(CHART_SPECS[metric].decimals).toBe(decimals);
  });

  it('each spec formats with spec.decimals places', () => {
    for (const m of CHART_ORDER) {
      const spec = CHART_SPECS[m];
      for (const v of [0, 1.23456, 47.55, 1234.5, -12.349]) {
        const text = spec.format(v);
        expect(text).toBe(formatFixed(v, spec.decimals));
        const dot = text.indexOf('.');
        expect(dot < 0 ? 0 : text.length - dot - 1).toBe(spec.decimals);
      }
    }
  });

  it('formats NaN as – and never as 0, for every spec', () => {
    for (const m of CHART_ORDER) {
      const spec = CHART_SPECS[m];
      expect(spec.format(Number.NaN)).toBe('–');
      expect(spec.format(spec.toDisplay(Number.NaN))).toBe('–');
    }
  });
});

describe('cadence chart unit follows the sport (QA-14-r1-4)', () => {
  it.each([['running', 'spm'], ['cycling', 'rpm'], ['multisport', '/min']])('%s → %s', (s, u) => {
    expect(chartSpecs(s, 'metric').cadence.unit).toBe(u);
  });
});
