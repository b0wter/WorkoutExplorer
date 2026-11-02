// TEST-ONLY shared log for Prototype.dom.test.ts (F-07 design F-6; ADR-012 rev. 2): the stub
// components and the mocked recorder write here, so the test sees one call order. Never
// imported by non-test code.

import type { WorkoutData } from '../../lib/datafile';
import type { CursorStore } from '../../lib/cursor';
import type { SelectionStore } from '../../lib/selection';

export interface ChartStackStubProps {
  data: WorkoutData;
  cursor: CursorStore;
  selection?: SelectionStore;
  chartHeight?: number;
  units?: 'metric' | 'imperial';
  scheme?: 'light' | 'dark';
  onready?: () => void;
  initialAxis?: 'time' | 'distance';
  onaxis?: (axis: 'time' | 'distance') => void;
}

export interface RouteMapStubProps {
  data: WorkoutData;
  cursor: CursorStore;
  selection?: SelectionStore;
  load?: boolean;
  height?: number;
  fill?: boolean;
  scheme?: 'light' | 'dark';
  styleUrl?: string;
  onmapready?: () => void;
  hidden?: readonly (readonly [number, number])[];
}

export const stubLog = {
  /** 'createRecorder', 'RouteMap', 'ChartStack', 'chartsReady', 'mapReady', in call order. */
  calls: [] as string[],
  /** The live props object of each stub instance (reads give the current value). */
  chartStack: [] as ChartStackStubProps[],
  routeMap: [] as RouteMapStubProps[],
};

export function resetStubLog(): void {
  stubLog.calls = [];
  stubLog.chartStack = [];
  stubLog.routeMap = [];
}
