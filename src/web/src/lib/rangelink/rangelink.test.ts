// F-40 SR-1, SR-2 (AC 4, AC 6, AC 7): the range link format and its hostile-input parse.
import { describe, expect, it } from 'vitest';
import { parseRangeLink, rangeFragment } from '.';

// Offsets with a pause (40 → 100) and a start that is not 0, as after start hiding.
const T = Float64Array.from([10, 11, 12, 20, 30, 40, 100, 101, 102]);

describe('parseRangeLink', () => {
  it('selects the first sample at or after from to the last at or before to', () => {
    expect(parseRangeLink('#range=11-40', T)).toEqual([1, 5]);
    expect(parseRangeLink('#range=13-99', T)).toEqual([3, 5]);
    expect(parseRangeLink('#range=0-9999999', T)).toEqual([0, 8]); // cut to the data
    expect(parseRangeLink('#range=100-200', T)).toEqual([6, 8]);
  });

  it('round-trips a selection exactly on whole-second offsets', () => {
    for (const [a, b] of [[0, 8], [2, 3], [5, 6], [1, 7]] as const) {
      expect(parseRangeLink('#' + rangeFragment(T, a, b), T)).toEqual([a, b]);
    }
    expect(rangeFragment(Float64Array.from([1.5, 2.5]), 0, 1)).toBe('range=1-3');
  });

  it.each([
    '', '#', '#range=', '#range=1-', '#range=-5', 'range=1-5', '#range=5-1', '#range=5-5', '#range=1-5&x=1', '#x=1&range=1-5',
    '#range=1-5#range=1-5', '#range=+1-5', '#range=1.0-5', '#range= 1-5', '#range=1-5 ', '#range=%31-5', '#RANGE=1-5',
    '#range=12345678-12345679', '#range=1e3-2e3', '#range=0x1-5', '#range=1--5', '#range=1-5\n',
    '#range=0-9', // entirely before the data
    '#range=200-300', // entirely after it
    '#range=41-99', // inside the pause: no sample
    '#range=20-29', // one sample only
    '#token-like-AbC123_-',
  ])('ignores %j', (hash) => {
    expect(parseRangeLink(hash, T)).toBeNull();
  });

  it('ignores everything on a one-sample or empty workout', () => {
    expect(parseRangeLink('#range=0-100', Float64Array.from([5]))).toBeNull();
    expect(parseRangeLink('#range=0-100', new Float64Array(0))).toBeNull();
  });
});
