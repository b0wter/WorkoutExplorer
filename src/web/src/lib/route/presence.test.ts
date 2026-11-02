// Route presence (F-06 design, Tests; AP-3, AC 5).

import { describe, expect, it } from 'vitest';
import { routePresence } from './index';
import { golden, positioned } from './testing/fixtures';

const N = Number.NaN;

describe('routePresence', () => {
  it('no position stream → no-location-data', () => {
    expect(routePresence(positioned(undefined))).toBe('no-location-data');
  });

  it('position stream with every sample NaN → no-visible-route', () => {
    expect(routePresence(positioned([N, N, N]))).toBe('no-visible-route');
  });

  it('one finite sample anywhere → route', () => {
    expect(routePresence(positioned([47, N, N]))).toBe('route');
    expect(routePresence(positioned([N, 47, N]))).toBe('route');
    expect(routePresence(positioned([N, N, 47]))).toBe('route');
  });

  it('golden files', () => {
    expect(routePresence(golden('indoor-no-gps'))).toBe('no-location-data');
    expect(routePresence(golden('gps-dropout'))).toBe('route');
    expect(routePresence(golden('ride-12h'))).toBe('route');
  });
});
