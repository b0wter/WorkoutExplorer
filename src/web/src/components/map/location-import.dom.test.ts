// @vitest-environment jsdom
// F-62 LC-8: the location chunk fails to load: one message, location stays off, no watch, the map
// unchanged; a later tap tries again. Its own file, as the chunk is mocked to fail.

import { expect, it, vi } from 'vitest';
import { createCursorStore } from '../../lib/cursor';
import { positioned } from '../../lib/route/testing/fixtures';
import { mountRouteMap } from './map-view';
import { recorder } from './testing/maplibre-fake';

vi.mock('maplibre-gl', async () => await import('./testing/maplibre-fake'));
vi.mock('maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url', () => ({ default: '/assets/maplibre-gl-worker-test.js' }));
vi.mock('maplibre-gl/dist/maplibre-gl.css', () => ({}));
vi.mock('./location', () => {
  throw new Error('chunk failed (test)');
});

it('a failed chunk: the message, off, no watch, nothing logged', async () => {
  const watchPosition = vi.fn();
  Object.defineProperty(navigator, 'geolocation', { value: { watchPosition, clearWatch: vi.fn() }, configurable: true });
  vi.stubGlobal('fetch', vi.fn(() => new Promise(() => {})));
  const error = vi.spyOn(console, 'error');
  const data = positioned([47, 47], [8, 8.01]);
  const container = document.createElement('div');
  document.body.appendChild(container);
  const handle = await mountRouteMap({ container, data, cursor: createCursorStore(data.sampleCount, (f) => f()), whenQuiet: () => Promise.resolve(), locate: true, signal: new AbortController().signal });
  const toggle = container.querySelector<HTMLButtonElement>('.wx-loc-group button')!;
  toggle.click();
  await vi.waitFor(() => expect(container.querySelector('.wx-loc-note')?.textContent).toBe('Your location is not available.'));
  expect(toggle.getAttribute('aria-pressed')).toBe('false');
  expect(watchPosition).not.toHaveBeenCalled();
  expect(recorder.maps[0].jumps).toHaveLength(0);
  expect(error).not.toHaveBeenCalled();
  handle.destroy();
});
