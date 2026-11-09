// Viewer preferences on the device (decision log 29 Oct 2026; DS-1, AP-5, AP-17): one entry per
// preference, unknown values ignored, every storage failure silent.
import { describe, expect, it } from 'vitest';
import { PREF_KEYS, createViewerPrefs, type KeyValueStore } from './index';

function memory(): KeyValueStore & { map: Map<string, string> } {
  const map = new Map<string, string>();
  return {
    map,
    getItem: (k) => map.get(k) ?? null,
    setItem: (k, v) => void map.set(k, v),
    removeItem: (k) => void map.delete(k),
  };
}

describe('createViewerPrefs', () => {
  it('reads nothing from empty storage and writes nothing by reading', () => {
    const s = memory();
    const p = createViewerPrefs(() => s);
    expect([p.units(), p.axis(), p.split('wide'), p.split('landscape')]).toEqual([null, null, null, null]);
    expect(s.map.size).toBe(0);
  });

  it('round-trips each preference in its own entry', () => {
    const s = memory();
    const p = createViewerPrefs(() => s);
    p.setUnits('imperial');
    expect([...s.map]).toEqual([[PREF_KEYS.units, 'imperial']]);
    p.setAxis('distance');
    p.setSplit('wide', 42.123);
    p.setSplit('landscape', 55);
    expect([p.units(), p.axis(), p.split('wide'), p.split('landscape')]).toEqual(['imperial', 'distance', 42.12, 55]);
    p.clearSplit('wide');
    expect(p.split('wide')).toBeNull();
    expect(p.split('landscape')).toBe(55);
    expect(s.map.size).toBe(3);
  });

  it('theme: System is the default and is stored as no entry (DS-4)', () => {
    const s = memory();
    const p = createViewerPrefs(() => s);
    expect(p.theme()).toBe('system');
    p.setTheme('dark');
    expect([...s.map]).toEqual([[PREF_KEYS.theme, 'dark']]);
    expect(p.theme()).toBe('dark');
    p.setTheme('light');
    expect(p.theme()).toBe('light');
    p.setTheme('system');
    expect(s.map.size).toBe(0);
    for (const v of ['system', 'Dark', 'auto', '']) {
      s.map.set(PREF_KEYS.theme, v);
      expect(p.theme(), v).toBe('system');
    }
  });

  it.each([
    ['wx.units', 'furlongs'],
    ['wx.units', 'Imperial'],
    ['wx.axis', 'pace'],
    ['wx.split.wide', '50%'],
    ['wx.split.wide', 'NaN'],
    ['wx.split.wide', '-40'],
    ['wx.split.wide', ''],
    ['wx.split.wide', '1e2'],
  ])('ignores an unknown value: %s = %j', (key, value) => {
    const s = memory();
    s.map.set(key, value);
    const p = createViewerPrefs(() => s);
    expect([p.units(), p.axis(), p.split('wide')]).toEqual([null, null, null]);
  });

  it('is silent when storage is missing, the getter throws, or every call throws', () => {
    const boom = (): never => {
      throw new Error('SecurityError');
    };
    const throwing: KeyValueStore = { getItem: boom, setItem: boom, removeItem: boom };
    for (const get of [() => null, () => undefined, boom, () => throwing]) {
      const p = createViewerPrefs(get);
      expect(() => {
        p.setUnits('metric');
        p.setAxis('time');
        p.setSplit('wide', 40);
        p.clearSplit('landscape');
        p.setTheme('dark');
        p.setTheme('system');
        p.setHowtoOpened();
        p.setHintDone();
      }).not.toThrow();
      expect([p.units(), p.axis(), p.split('wide'), p.theme(), p.howtoOpened()]).toEqual([null, null, null, 'system', false]);
    }
  });

  it('F-39 flags: "1" once, any other value reads as not yet (HU-1)', () => {
    const s = memory();
    const p = createViewerPrefs(() => s);
    expect(p.howtoOpened()).toBe(false);
    for (const v of ['0', 'true', '1 ', '']) {
      s.map.set(PREF_KEYS.howto, v);
      expect(p.howtoOpened()).toBe(false);
    }
    s.map.clear();
    p.setHowtoOpened();
    p.setHintDone();
    expect([...s.map]).toEqual([
      ['wx.howto.opened', '1'],
      ['wx.hint.done', '1'],
    ]);
    expect(p.howtoOpened()).toBe(true);
  });
});
