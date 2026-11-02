// The units store and the default rule (F-14 design D-3, D-4; DS-1; AC 1, 3, 9).
import { describe, expect, it, vi } from 'vitest';
import { createUnitsStore, defaultUnits } from './index';

describe('defaultUnits', () => {
  it.each([
    [['en-US'], 'imperial'],
    [['en-us'], 'imperial'],
    [['es_US'], 'imperial'],
    [['en-Latn-US'], 'imperial'],
    [['en-US', 'de-DE'], 'imperial'],
    [['de-DE'], 'metric'],
    [['en-GB'], 'metric'],
    [['en'], 'metric'],
    [['en-GB', 'en-US'], 'metric'],
    [['en-CA'], 'metric'],
    [[''], 'metric'],
    [['!!'], 'metric'],
    [[], 'metric'],
  ] as const)('%j → %s', (languages, expected) => {
    expect(defaultUnits(languages, undefined)).toBe(expected);
  });
  it('falls back to navigator.language, then metric', () => {
    expect(defaultUnits(undefined, 'en-US')).toBe('imperial');
    expect(defaultUnits(undefined, undefined)).toBe('metric');
  });
});

describe('createUnitsStore', () => {
  it('get, set, subscribe with synchronous delivery; the same value is a no-op; no call on subscribe', () => {
    const store = createUnitsStore('metric');
    const seen: string[] = [];
    const off = store.subscribe((u) => seen.push(u));
    expect(seen).toEqual([]);
    store.set('imperial');
    store.set('imperial');
    expect(store.get()).toBe('imperial');
    expect(seen).toEqual(['imperial']);
    off();
    store.set('metric');
    expect(seen).toEqual(['imperial']);
  });
  it('a throwing listener is reported and does not stop the others', () => {
    const report = vi.fn();
    const store = createUnitsStore('metric', report);
    const seen: string[] = [];
    store.subscribe(() => {
      throw new Error('x');
    });
    store.subscribe((u) => seen.push(u));
    store.set('imperial');
    expect(report).toHaveBeenCalledTimes(1);
    expect(seen).toEqual(['imperial']);
  });
});
