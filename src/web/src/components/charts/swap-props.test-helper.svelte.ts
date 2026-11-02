// TEST-ONLY helper for ChartStack.dom.test.ts (F-05 design v1.4, Tests, QA-05-r2-2):
// holds ChartStack's `data` and `cursor` props in $state.raw, so a test can swap them in
// place on the same mounted ChartStack. Never imported by non-test code.

import type { WorkoutData } from '../../lib/datafile';
import type { CursorStore } from '../../lib/cursor';

export interface SwapHarness {
  /** Props for mount(ChartStack, { props }); data and cursor read the state. */
  readonly props: { readonly data: WorkoutData; readonly cursor: CursorStore; readonly onready?: () => void };
  /** Replaces data and cursor together (one state write). */
  swap(data: WorkoutData, cursor: CursorStore): void;
}

export function swapHarness(data: WorkoutData, cursor: CursorStore, onready?: () => void): SwapHarness {
  let current = $state.raw({ data, cursor });
  return {
    props: {
      get data() {
        return current.data;
      },
      get cursor() {
        return current.cursor;
      },
      onready,
    },
    swap(d: WorkoutData, c: CursorStore): void {
      current = { data: d, cursor: c };
    },
  };
}
