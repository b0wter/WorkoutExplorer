// RouteMap's props as reactive state, for mounting it from plain TypeScript (ADR-026 item 4):
// `load` turns true once, from ChartStack's onready (F-06 "Wiring"). The only reactive state of
// the viewing-page glue.

import type { CursorStore } from '../lib/cursor';
import type { SelectionStore } from '../lib/selection';
import type { WorkoutData } from '../lib/datafile';
import type { Units } from '../lib/format';
import type { UnitsStore } from '../lib/units';
import type { MapLayerPrefs, ThemeChoice } from '../lib/prefs';
import type { HintMode } from '../lib/charts';
import type { ChartDecorator, ChartView } from '../components/charts/uplot-options';
import type { RouteMapHandle } from '../components/map/map-view';
import { langSource, type Lang } from '../lib/i18n';

let installed = false;
/** F-54: the language as Svelte state, so a template that reads a text re-renders on a change.
 *  Once per page; the edit glue calls it before its first mount. */
export function langState(): void {
  if (installed) return;
  installed = true;
  let lg = $state<Lang>('en');
  langSource(
    () => lg,
    (next) => (lg = next),
  );
}

/** The viewer's units and colour scheme as reactive state (F-14 D-10): the one scheme listener
 *  of the page. The glue passes both to ChartStack and RouteMap. A forced theme (DS-4) wins over
 *  the system's scheme; the caller sets <html data-theme> first, so the CSS already matches. */
export interface Look {
  readonly units: Units;
  readonly scheme: 'light' | 'dark';
  theme: ThemeChoice;
  /** The range hint under the charts (F-39 HU-3), set by the page. */
  hint: HintMode;
  /** The store itself, for consumers that subscribe (the map's colouring legend, F-25 C-6). */
  readonly store: UnitsStore;
  /** The charts' decoration (F-42 MS-7): every chunk's decorator, in the order first set. */
  readonly decorate?: ChartDecorator;
  /** Sets one chunk's decorator by key (sections, F-56 pins); the charts redraw. */
  decorator(key: string, f: ChartDecorator): void;
  /** F-67 LP-3: the charts' leg view, memory only; written by the sections chunk. */
  view: ChartView;
}

export function createLook(units: UnitsStore, dark: MediaQueryList | null, theme: ThemeChoice = 'system', hint: HintMode = 'plain'): Look {
  let u = $state(units.get());
  let h = $state(hint);
  let isDark = $state(dark?.matches ?? false);
  let t = $state(theme);
  let d = $state.raw<ChartDecorator>();
  let v = $state.raw<ChartView>(null);
  const chunks: Record<string, ChartDecorator> = {};
  units.subscribe((next) => (u = next));
  langState();
  dark?.addEventListener('change', (e) => (isDark = e.matches));
  return {
    get units() {
      return u;
    },
    get scheme() {
      return t === 'system' ? (isDark ? 'dark' : 'light') : t;
    },
    get theme() {
      return t;
    },
    set theme(next) {
      t = next;
    },
    get hint() {
      return h;
    },
    set hint(next) {
      h = next;
    },
    get decorate() {
      return d;
    },
    decorator(key, f) {
      chunks[key] = f;
      const fs = Object.values(chunks);
      d = (over, x) => fs.forEach((g) => g(over, x)); // a write only, also when called inside an effect
    },
    get view() {
      return v;
    },
    set view(next) {
      v = next;
    },
    store: units,
  };
}

export interface MapProps {
  readonly data: WorkoutData;
  readonly cursor: CursorStore;
  readonly selection: SelectionStore;
  readonly load: boolean;
  /** The container fills #map-slot, whose CSS owns the height (D-20). */
  readonly fill: boolean;
  readonly scheme: 'light' | 'dark';
  readonly units: UnitsStore;
  readonly onmapready: (handle: RouteMapHandle) => void;
  /** F-19 E-9: sample ranges marked as to-be-hidden; edit page only. */
  readonly hidden: readonly (readonly [number, number])[] | undefined;
  /** F-61 SL-2: the viewer's map layer choice. */
  readonly prefs: MapLayerPrefs;
  /** F-62 LC-1: the location controls, viewing page only (no edit seam). */
  readonly locate: boolean;
}

export function createMapProps(
  data: WorkoutData,
  cursor: CursorStore,
  selection: SelectionStore,
  look: Look,
  onmapready: (handle: RouteMapHandle) => void,
  edit: { readonly hidden: readonly (readonly [number, number])[] } | undefined,
  prefs: MapLayerPrefs,
): { props: MapProps; startLoading(): void } {
  let load = $state(false);
  return {
    props: {
      data,
      cursor,
      selection,
      fill: true,
      units: look.store,
      onmapready,
      prefs,
      locate: !edit,
      get scheme() {
        return look.scheme;
      },
      get load() {
        return load;
      },
      get hidden() {
        return edit?.hidden;
      },
    },
    startLoading(): void {
      load = true;
    },
  };
}
