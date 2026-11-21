// @vitest-environment jsdom
// F-62 LC-1, OQ-287: the location controls only on the viewing page; the edit seam (draft and
// published owner pages) passes none.
import { expect, it } from 'vitest';
import { createCursorStore } from '../lib/cursor';
import { createSelectionStore } from '../lib/selection';
import { createUnitsStore } from '../lib/units';
import { positioned } from '../lib/route/testing/fixtures';
import { createLook, createMapProps } from './map-props.svelte';

it('locate: true without the edit seam, false with it', () => {
  const data = positioned([47, 47], [8, 8.01]);
  const look = createLook(createUnitsStore('metric'), null);
  const prefs = { mapLayer: () => 'map' as const, setMapLayer: () => {} };
  const make = (edit?: { hidden: [] }) =>
    createMapProps(data, createCursorStore(data.sampleCount, (f) => f()), createSelectionStore(data.sampleCount, (f) => f()), look, () => {}, edit, prefs).props.locate;
  expect(make()).toBe(true);
  expect(make({ hidden: [] })).toBe(false);
});
