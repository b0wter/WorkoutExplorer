// What the viewer sees when the data file cannot be loaded (F-07 design D-3, F-3). Pure.
// OQ-10 was accepted by the user on 15 Oct 2026: "The workout data could not be loaded."
// No technical detail on the page: the code and message go to the console only.

import type { DataFileErrorCode } from '../lib/datafile';

export const LOAD_ERROR_TEXT = 'The workout data could not be loaded.';
export const RELOAD_TEXT = 'Reload the page.';

/** 'unsupported-version' → LOAD_ERROR_TEXT + ' ' + RELOAD_TEXT; every other code and 'bad-name' → LOAD_ERROR_TEXT. */
export function loadErrorText(code: DataFileErrorCode | 'bad-name'): string {
  return code === 'unsupported-version' ? LOAD_ERROR_TEXT + ' ' + RELOAD_TEXT : LOAD_ERROR_TEXT;
}
