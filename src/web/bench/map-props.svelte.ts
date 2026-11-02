// Dev-only props holder for bench/map.ts (F-06 design, "Dev page"): RouteMap's `load` must
// turn true from ChartStack's onready on a mounted RouteMap, which needs $state. Never
// imported by src/.

import type { WorkoutData } from '../src/lib/datafile';
import type { CursorStore } from '../src/lib/cursor';

export interface BenchMapProps {
  readonly data: WorkoutData;
  readonly cursor: CursorStore;
  readonly load: boolean;
  readonly styleUrl?: string;
  readonly onmapready?: () => void;
}

export function benchMapProps(initial: BenchMapProps): { props: BenchMapProps; setLoad(load: boolean): void } {
  let load = $state(initial.load);
  return {
    props: {
      data: initial.data,
      cursor: initial.cursor,
      get load() {
        return load;
      },
      styleUrl: initial.styleUrl,
      onmapready: initial.onmapready,
    },
    setLoad(value: boolean): void {
      load = value;
    },
  };
}
