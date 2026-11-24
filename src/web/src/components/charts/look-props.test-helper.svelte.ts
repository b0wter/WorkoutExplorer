// TEST-ONLY helper for ChartStack.dom.test.ts (F-14): holds ChartStack's `units` and `scheme`
// props in $state, so a test can change them in place on the same mounted ChartStack.
// Never imported by non-test code.

import type { WorkoutData } from '../../lib/datafile';
import type { CursorStore } from '../../lib/cursor';
import type { Units } from '../../lib/format';
import type { ChartView } from './uplot-options';

export interface LookHarness {
  readonly props: { readonly data: WorkoutData; readonly cursor: CursorStore; readonly units: Units; readonly scheme: 'light' | 'dark'; readonly view: ChartView };
  set(units: Units, scheme: 'light' | 'dark'): void;
  view(v: ChartView): void;
}

export function lookHarness(data: WorkoutData, cursor: CursorStore): LookHarness {
  let units = $state<Units>('metric');
  let scheme = $state<'light' | 'dark'>('light');
  let view = $state.raw<ChartView>(null);
  return {
    view: (v) => (view = v),
    props: {
      data,
      cursor,
      get units() {
        return units;
      },
      get view() {
        return view;
      },
      get scheme() {
        return scheme;
      },
    },
    set(u, s) {
      units = u;
      scheme = s;
    },
  };
}
