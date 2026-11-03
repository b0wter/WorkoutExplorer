// RouteMap's props as reactive state, for mounting it from plain TypeScript (ADR-026 item 4):
// `load` turns true once, from ChartStack's onready (F-06 "Wiring"). The only reactive state of
// the viewing-page glue.

import type { CursorStore } from '../lib/cursor';
import type { SelectionStore } from '../lib/selection';
import type { WorkoutData } from '../lib/datafile';
import type { Units } from '../lib/format';
import type { UnitsStore } from '../lib/units';
import type { ThemeChoice } from '../lib/prefs';

/** The viewer's units and colour scheme as reactive state (F-14 D-10): the one scheme listener
 *  of the page. The glue passes both to ChartStack and RouteMap. A forced theme (DS-4) wins over
 *  the system's scheme; the caller sets <html data-theme> first, so the CSS already matches. */
export interface Look {
  readonly units: Units;
  readonly scheme: 'light' | 'dark';
  theme: ThemeChoice;
  /** The store itself, for consumers that subscribe (the map's colouring legend, F-25 C-6). */
  readonly store: UnitsStore;
}

export function createLook(units: UnitsStore, dark: MediaQueryList | null, theme: ThemeChoice = 'system'): Look {
  let u = $state(units.get());
  let isDark = $state(dark?.matches ?? false);
  let t = $state(theme);
  units.subscribe((next) => (u = next));
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
  readonly onmapready: () => void;
  /** F-19 E-9: sample ranges marked as to-be-hidden; edit page only. */
  readonly hidden: readonly (readonly [number, number])[] | undefined;
}

export function createMapProps(
  data: WorkoutData,
  cursor: CursorStore,
  selection: SelectionStore,
  look: Look,
  onmapready: () => void,
  edit?: { readonly hidden: readonly (readonly [number, number])[] },
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
