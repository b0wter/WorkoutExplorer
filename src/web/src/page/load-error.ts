// What the viewer sees when the data file cannot be loaded (F-07 design D-3, F-3). Pure.
// OQ-10 was accepted by the user on 15 Oct 2026: "The workout data could not be loaded."
// No technical detail on the page: the code and message go to the console only.

import type { DataFileErrorCode } from '../lib/datafile';

import { t } from './texts';

/** 'unsupported-version' → the load error and "Reload the page."; every other code and 'bad-name' → the load error. */
export function loadErrorText(code: DataFileErrorCode | 'bad-name'): string {
  return code === 'unsupported-version' ? t('loadError') + ' ' + t('reload') : t('loadError');
}
