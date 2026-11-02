// Viewer preferences remembered on the viewer's device (decision log 29 Oct 2026: DS-1 units,
// AP-5 x axis, AP-17 map/content split). The only module that touches browser storage. Each
// preference is its own localStorage entry, written only on an explicit choice by the caller.
// Every access is guarded: unavailable, blocked or throwing storage, and any value that is not
// one this module wrote, read as "nothing stored", silently. No cookie; nothing goes to the server.
// Imports only the Units type. Shared by the viewing page and, later, the edit page (F-19).

import type { Units } from '../format';

/** The part of `Storage` used here (localStorage in the browser, a map in tests). */
export interface KeyValueStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export type AxisChoice = 'time' | 'distance';
/** The two two-column layouts have separate splits (their defaults differ): desktop, phone in landscape. */
export type SplitLayout = 'wide' | 'landscape';

export interface ViewerPrefs {
  units(): Units | null;
  setUnits(units: Units): void;
  axis(): AxisChoice | null;
  setAxis(axis: AxisChoice): void;
  /** The stored map share in percent, any finite number (the splitter decides what it accepts). */
  split(layout: SplitLayout): number | null;
  setSplit(layout: SplitLayout, percent: number): void;
  clearSplit(layout: SplitLayout): void;
}

export const PREF_KEYS = {
  units: 'wx.units',
  axis: 'wx.axis',
  wide: 'wx.split.wide',
  landscape: 'wx.split.landscape',
} as const;

/** `storage` is called on every access, so a getter that throws (blocked site data) is contained too. */
export function createViewerPrefs(storage: () => KeyValueStore | null | undefined): ViewerPrefs {
  const read = (key: string): string | null => {
    try {
      return storage()?.getItem(key) ?? null;
    } catch {
      return null;
    }
  };
  const write = (key: string, value: string | null): void => {
    try {
      const s = storage();
      if (value === null) s?.removeItem(key);
      else s?.setItem(key, value);
    } catch {
      // Not remembered; the choice still applies to this page.
    }
  };
  const oneOf = <T extends string>(key: string, allowed: readonly T[]): T | null => {
    const v = read(key);
    return allowed.find((a) => a === v) ?? null;
  };
  return {
    units: () => oneOf(PREF_KEYS.units, ['metric', 'imperial'] as const),
    setUnits: (units) => write(PREF_KEYS.units, units),
    axis: () => oneOf(PREF_KEYS.axis, ['time', 'distance'] as const),
    setAxis: (axis) => write(PREF_KEYS.axis, axis),
    split(layout) {
      const v = read(PREF_KEYS[layout]);
      if (v === null || !/^\d{1,3}(\.\d{1,2})?$/.test(v)) return null;
      return Number(v);
    },
    setSplit: (layout, percent) => write(PREF_KEYS[layout], percent.toFixed(2)),
    clearSplit: (layout) => write(PREF_KEYS[layout], null),
  };
}
