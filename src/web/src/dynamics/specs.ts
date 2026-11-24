// The running dynamics' chart specs (F-68 XS-1, XS-9, XS-10): the one list that the charts, both
// readouts and the range panel's cells read (installed by ./index).

import type { ChartSpec } from '../lib/charts';
import { formatFixed, toElevation, type Units } from '../lib/format';
import type { DynamicsName } from '../lib/datafile';
import { t } from './texts';

const CM_PER_IN = 2.54;

/** XS-10: cm (1) or in (1); ms (0); m (2) or ft (2); % (1). The stream values are in the §2 units. */
export function dynamicsSpecs(units: Units): readonly ChartSpec[] {
  const imperial = units === 'imperial';
  const row = (metric: DynamicsName, label: string, short: string, unit: string, toDisplay: (v: number) => number, decimals: number, colour: string): ChartSpec => ({
    metric,
    label,
    short,
    unit,
    toDisplay,
    decimals,
    format: (v) => formatFixed(v, decimals),
    colour,
    invert: false,
  });
  const same = (v: number): number => v;
  return [
    row('vertical_oscillation', t('dynVerticalOscillation'), t('dynVerticalOscillationShort'), imperial ? 'in' : 'cm', imperial ? (v) => v / CM_PER_IN : same, 1, '#9c6b00'),
    row('ground_contact_time', t('dynGroundContact'), t('dynGroundContactShort'), 'ms', same, 0, '#4a55c9'),
    row('stride_length', t('dynStrideLength'), t('dynStrideLengthShort'), imperial ? 'ft' : 'm', (v) => toElevation(v, units), 2, '#7a8c00'),
    row('vertical_ratio', t('dynVerticalRatio'), t('dynVerticalRatioShort'), '%', same, 1, '#8d5a3a'),
  ];
}
