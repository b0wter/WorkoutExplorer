// lib/format additions of F-13 (design D-6, D-7): pace, cadence unit, sport names, dates, time left.
import { describe, expect, it } from 'vitest';
import {
  cadenceUnit, distanceUnit, elevationUnit, formatDistance, formatFixed, formatPace,
  formatSpeedOrPace, formatTimeLeft, isPaceSport, paceSeconds, speedOrPaceUnit, toSpeed,
  temperatureUnit, toElevation, toTemperature,
} from './index';
import { formatClock, formatDate, formatLocalStart, sportIcon, sportName } from './dates';

describe('speed and pace (DS-2)', () => {
  it.each([
    ['cycling', 10, '36.0', 'km/h'],
    ['toString', 10, '36.0', 'km/h'],
    ['running', 3, '5:33', '/km'],
    ['walking', 1.5, '11:07', '/km'],
    ['swimming', 1, '1:40', '/100 m'],
    ['rowing', 4, '2:05', '/500 m'],
    ['multisport', 5, '18.0', 'km/h'],
    ['', 5, '18.0', 'km/h'],
  ])('%s at %s m/s → %s %s', (sport, v, text, unit) => {
    expect(formatSpeedOrPace(sport, v)).toBe(text);
    expect(speedOrPaceUnit(sport)).toBe(unit);
  });
  it('rounds a pace to whole seconds and carries into the minute', () => {
    expect(formatSpeedOrPace('running', 1000 / 359.6)).toBe('6:00');
  });
  it.each([0, -1, NaN, Infinity])('a pace or speed from %s is hidden', (v) => {
    expect(formatSpeedOrPace('running', v)).toBe('');
    expect(formatSpeedOrPace('cycling', v)).toBe('');
  });
  it('pace sports', () => {
    expect(['running', 'walking', 'hiking', 'swimming', 'rowing'].every(isPaceSport)).toBe(true);
    expect(['cycling', 'multisport', 'training', 'toString'].some(isPaceSport)).toBe(false);
  });
});

describe('cadence unit', () => {
  it.each([['running', 'spm'], ['hiking', 'spm'], ['cycling', 'rpm'], ['training', 'rpm'], ['multisport', '/min']])('%s → %s', (s, u) => {
    expect(cadenceUnit(s)).toBe(u);
  });
});

describe('sport name and icon', () => {
  it.each([
    ['cycling', 'Ride', 'cycling'], ['running', 'Run', 'running'], ['multisport', 'Multisport', 'generic'],
    ['cross_country_skiing', 'Cross country skiing', 'generic'], ['', 'Workout', 'generic'], ['constructor', 'Constructor', 'generic'],
  ])('%s → %s, icon %s', (s, n, i) => {
    expect(sportName(s)).toBe(n);
    expect(sportIcon(s)).toBe(i);
  });
});

describe('dates in the zone at the start position (D-7)', () => {
  it('shifts by the offset and prints with UTC getters', () => {
    expect(formatDate('2026-06-12T23:30:00Z', 7200)).toBe('13 Jun 2026');
    expect(formatClock('2026-06-12T23:30:00Z', 7200)).toBe('01:30');
    expect(formatClock('2026-06-12T05:30:00Z', -18000)).toBe('00:30');
    expect(formatClock('2026-06-12T05:30:00Z', 19800)).toBe('11:00');
  });
  it('unknown zone: UTC with a label', () => {
    expect(formatLocalStart('2026-06-12T03:00:00Z', { known: false, offset_seconds: 0 })).toBe('12 Jun 2026, 03:00 UTC');
    expect(formatLocalStart('2026-06-12T03:00:00Z', { known: true, offset_seconds: 7200 })).toBe('12 Jun 2026, 05:00');
  });
});

describe('time left (UP-7, OQ-62)', () => {
  const at = Date.parse('2026-10-30T09:15:00Z');
  it.each([
    [0, 'Expires in 6 days', 6 * 86400 + 3600],
    [0, 'Expires in 2 days', 2 * 86400],
    [0, 'Expires in 47 hours', 47 * 3600],
    [0, 'Expires in 1 hour', 3600 + 59],
    [0, 'Expires in 5 minutes', 5 * 60],
    [0, 'Expires in 1 minute', 60],
    [0, 'Expires in less than a minute', 59],
    [0, 'Expires now', 0],
    [0, 'Expires now', -5],
  ])('%s: %s', (_n, text, secondsLeft) => {
    expect(formatTimeLeft(at, at - secondsLeft * 1000)).toBe(text);
  });
});

describe('imperial units (F-14 D-1, DS-1, DS-2)', () => {
  it.each([
    ['cycling', 10, '22.4', 'mph'],
    ['multisport', 10, '22.4', 'mph'],
    ['running', 3, '8:56', '/mi'],
    ['hiking', 1.5, '17:53', '/mi'],
    ['swimming', 1, '1:31', '/100 yd'],
    ['rowing', 4, '2:05', '/500 m'],
  ])('%s at %s m/s → %s %s', (sport, v, text, unit) => {
    expect(formatSpeedOrPace(sport, v, 'imperial')).toBe(text);
    expect(speedOrPaceUnit(sport, 'imperial')).toBe(unit);
  });
  it('distance, elevation and temperature', () => {
    expect(formatDistance(1609.344, 'imperial')).toBe('1.00');
    expect(formatDistance(10000, 'metric')).toBe('10.00');
    expect([distanceUnit('metric'), distanceUnit('imperial')]).toEqual(['km', 'mi']);
    expect([elevationUnit('metric'), elevationUnit('imperial')]).toEqual(['m', 'ft']);
    expect([temperatureUnit('metric'), temperatureUnit('imperial')]).toEqual(['°C', '°F']);
    expect(toElevation(100, 'imperial')).toBeCloseTo(328.084, 3);
    expect(toElevation(100, 'metric')).toBe(100);
    expect(toTemperature(100, 'imperial')).toBe(212);
    expect(toTemperature(-40, 'imperial')).toBe(-40);
    expect(toTemperature(21, 'metric')).toBe(21);
  });
  it('a pace from a speed <= 0 stays hidden in both systems', () => {
    expect(formatSpeedOrPace('running', 0, 'imperial')).toBe('');
    expect(formatSpeedOrPace('cycling', -1, 'imperial')).toBe('');
  });
  it('toSpeed converts m/s to km/h or mph', () => {
    expect(toSpeed(10, 'metric')).toBeCloseTo(36, 9);
    expect(toSpeed(10, 'imperial')).toBeCloseTo(22.369, 3);
  });
  it('paceSeconds and formatPace', () => {
    expect(paceSeconds('running', 'metric', 4)).toBe(250);
    expect(paceSeconds('running', 'metric', 0)).toBe(Infinity);
    expect(formatPace(250)).toBe('4:10');
    expect(formatPace(599.6)).toBe('10:00');
  });
});

// Constructed ties (F-15 design P-11): the same table as internal/page TestJavaScriptRounding in Go,
// which must round as toFixed (an exact binary tie away from zero) and Math.round (ties toward +∞).
describe('rounding ties, held equal with the Go page builder', () => {
  it.each([
    [0.5, 0, '1'], [1.5, 0, '2'], [2.5, 0, '3'], [-2.5, 0, '-3'], [-0.5, 0, '-1'],
    [0.25, 1, '0.3'], [0.125, 2, '0.13'], [2.675, 2, '2.67'], [1.005, 2, '1.00'],
    [0.35, 1, '0.3'], [12.5, 0, '13'], [13.5, 0, '14'], [-0.4, 0, '0'], [-0.04, 1, '0.0'],
    [0, 2, '0.00'], [9.995, 2, '9.99'], [9.96, 1, '10.0'], [(7.5 / 3.6) * 3.6, 1, '7.5'],
    [1e-7, 2, '0.00'], [123456.785, 2, '123456.79'],
  ])('formatFixed(%s, %s) → %s', (v, d, text) => {
    expect(formatFixed(v, d)).toBe(text);
  });

  it.each([
    [299.5, '5:00'], [298.5, '4:59'], [250, '4:10'],
  ])('formatPace(%s) → %s', (s, text) => {
    expect(formatPace(s)).toBe(text);
  });
});
