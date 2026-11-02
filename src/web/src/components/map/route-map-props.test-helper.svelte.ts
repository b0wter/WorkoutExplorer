// TEST-ONLY helper for RouteMap.dom.test.ts (ADR-012 rev. 2): holds RouteMap's props in
// $state.raw, so a test can change `load` or swap `data` and `cursor` in place on the same
// mounted RouteMap. Never imported by non-test code.

import type { WorkoutData } from '../../lib/datafile';
import type { CursorStore } from '../../lib/cursor';

export interface RouteMapTestProps {
  data: WorkoutData;
  cursor: CursorStore;
  load?: boolean;
  height?: number;
  styleUrl?: string;
  scheme?: 'light' | 'dark';
  onmapready?: () => void;
  hidden?: readonly (readonly [number, number])[];
}

export interface RouteMapHarness {
  /** Props for mount(RouteMap, { props }); every prop reads the state. */
  readonly props: Readonly<RouteMapTestProps>;
  /** Changes some props in one state write. */
  set(patch: Partial<RouteMapTestProps>): void;
}

export function routeMapHarness(initial: RouteMapTestProps): RouteMapHarness {
  let current = $state.raw({ ...initial });
  return {
    props: {
      get data() {
        return current.data;
      },
      get cursor() {
        return current.cursor;
      },
      get load() {
        return current.load;
      },
      get hidden() {
        return current.hidden;
      },
      get height() {
        return current.height;
      },
      get scheme() {
        return current.scheme;
      },
      get styleUrl() {
        return current.styleUrl;
      },
      get onmapready() {
        return current.onmapready;
      },
    },
    set(patch: Partial<RouteMapTestProps>): void {
      current = { ...current, ...patch };
    },
  };
}
