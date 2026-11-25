// TEST-ONLY helper for RangePanel.yards.dom.test.ts (F-72): the panel's `view` prop in $state, imperial units.
import type { WorkoutData } from '../../lib/datafile';
import type { SelectionStore } from '../../lib/selection';
import type { ChartView } from '../charts/uplot-options';

export function viewProps(data: WorkoutData, selection: SelectionStore) {
  let view = $state.raw<ChartView>(null);
  return {
    setView: (v: ChartView) => (view = v),
    props: {
      data,
      selection,
      units: 'imperial' as const,
      get view() {
        return view;
      },
    },
  };
}
