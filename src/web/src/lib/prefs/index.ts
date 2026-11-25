// Viewer preferences remembered on the viewer's device (decision log 29 Oct 2026: DS-1 units,
// AP-5 x axis, AP-17 map/content split; DS-4 theme, 3 Nov 2026). The only module that touches browser
// storage, apart from the read-only theme script in the page heads (theme-script.test.ts). Each
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
/** DS-4 (user, 3 Nov 2026): 'system' follows prefers-color-scheme and is stored as no entry. */
export type ThemeChoice = 'system' | 'light' | 'dark';
/** The two two-column layouts have separate splits (their defaults differ): desktop, phone in landscape. */
export type SplitLayout = 'wide' | 'landscape';

export interface ViewerPrefs {
  units(): Units | null;
  setUnits(units: Units): void;
  axis(): AxisChoice | null;
  setAxis(axis: AxisChoice): void;
  theme(): ThemeChoice;
  setTheme(theme: ThemeChoice): void;
  /** The stored map share in percent, any finite number (the splitter decides what it accepts). */
  split(layout: SplitLayout): number | null;
  setSplit(layout: SplitLayout, percent: number): void;
  clearSplit(layout: SplitLayout): void;
  /** AP-22 (F-39 HU-1): flags, `"1"` once the event happened; never removed. */
  howtoOpened(): boolean;
  setHowtoOpened(): void;
  setHintDone(): void;
  /** F-76 TC-1: a setter only; the head script reads it (data-chart-size), the bundle never does. Off removes the key. */
  setCharts(tall: boolean): void;
  /** F-54 LG-1: written only by the language drop-down, English included; read only by the head script. */
  setLang(lang: string): void;
  /** F-61 SL-2: the route map's layer; written only by the satellite toggle, read once when the map mounts. */
  mapLayer(): MapLayer;
  setMapLayer(layer: MapLayer): void;
}

export type MapLayer = 'map' | 'satellite';
export type MapLayerPrefs = Pick<ViewerPrefs, 'mapLayer' | 'setMapLayer'>;

export const PREF_KEYS = {
  units: 'wx.units',
  axis: 'wx.axis',
  /** Also read by the inline head script of index.html and edit.html, before the first paint. */
  theme: 'wx.theme',
  wide: 'wx.split.wide',
  landscape: 'wx.split.landscape',
  howto: 'wx.howto.opened',
  /** Also read by the inline head script (sets <html data-hint="done">, F-39 HU-2). */
  hint: 'wx.hint.done',
  /** F-76 TC-1: `tall` or no entry; also read by the head script (sets <html data-chart-size="tall">). */
  charts: 'wx.charts',
  /** F-54 LG-1: also read by the inline head script, which decides the page view's language. */
  lang: 'wx.lang',
  /** F-61 SL-2: never read by the head script or by the preview basemap. */
  map: 'wx.map',
} as const;

/** F-54 LG-1: the language drop-down's write alone, for the upload page's entry (no other key there). */
export function rememberLang(storage: () => KeyValueStore | null | undefined, lang: string): void {
  try {
    storage()?.setItem('wx.lang', lang);
  } catch {
    // Not remembered; the choice still applies to this page.
  }
}

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
    theme: () => oneOf(PREF_KEYS.theme, ['light', 'dark'] as const) ?? 'system',
    setTheme: (theme) => write(PREF_KEYS.theme, theme === 'system' ? null : theme),
    split(layout) {
      const v = read(PREF_KEYS[layout]);
      if (v === null || !/^\d{1,3}(\.\d{1,2})?$/.test(v)) return null;
      return Number(v);
    },
    setSplit: (layout, percent) => write(PREF_KEYS[layout], percent.toFixed(2)),
    clearSplit: (layout) => write(PREF_KEYS[layout], null),
    howtoOpened: () => read(PREF_KEYS.howto) === '1',
    setHowtoOpened: () => write(PREF_KEYS.howto, '1'),
    setHintDone: () => write(PREF_KEYS.hint, '1'),
    setCharts: (tall) => write(PREF_KEYS.charts, tall ? 'tall' : null),
    setLang: (lang) => write(PREF_KEYS.lang, lang),
    mapLayer: () => oneOf(PREF_KEYS.map, ['map', 'satellite'] as const) ?? 'map',
    setMapLayer: (layer) => write(PREF_KEYS.map, layer),
  };
}
