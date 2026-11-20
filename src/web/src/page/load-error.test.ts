// Load failure text, OQ-10 default (F-07 design D-3, F-3, F-6). Node.
import { describe, expect, it } from 'vitest';
import type { DataFileErrorCode } from '../lib/datafile';
import { loadErrorText } from './load-error';
import { en } from './texts';

// Every DataFileErrorCode: a missing one fails the type check (Record over the union).
const CODES: Record<DataFileErrorCode, true> = {
  'not-a-data-file': true,
  'unsupported-version': true,
  corrupt: true,
  'unsupported-platform': true,
  network: true,
  'http-status': true,
  internal: true,
};

describe('loadErrorText', () => {
  it('the texts', () => {
    expect(en.loadError).toBe('The workout data could not be loaded.');
    expect(en.reload).toBe('Reload the page.');
  });

  it.each([...(Object.keys(CODES) as DataFileErrorCode[]), 'bad-name' as const])('%s', (code) => {
    expect(loadErrorText(code)).toBe(
      code === 'unsupported-version' ? 'The workout data could not be loaded. Reload the page.' : 'The workout data could not be loaded.',
    );
  });
});
