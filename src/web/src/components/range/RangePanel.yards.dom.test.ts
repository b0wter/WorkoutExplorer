// @vitest-environment jsdom
// F-72 YD-2, YD-7: the range panel's distance unit follows the leg view (a swim leg: yards), the
// whole view stays in miles, and a view change rewrites unit and value.
import { afterEach, describe, expect, it } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile } from '../../lib/datafile';
import { createSelectionStore } from '../../lib/selection';
import RangePanel from './RangePanel.svelte';
import { viewProps } from './view-props.test-helper.svelte';

const r = decodeDataFile(loadGolden().find((x) => x.name === 'triathlon')!.bin);
if (!r.ok) throw new Error('no golden triathlon');
const data = r.data;
let app: ReturnType<typeof mount> | null = null;
afterEach(() => {
  if (app) void unmount(app);
  app = null;
});

describe('RangePanel distance unit (F-72)', () => {
  it('whole view: miles; swim leg view: yards; another leg: miles', () => {
    const target = document.createElement('div');
    const selection = createSelectionStore(data.sampleCount, (f) => f());
    const props = viewProps(data, selection);
    app = mount(RangePanel, { target, props: props.props });
    selection.set(10, 500);
    flushSync();
    const cell = () => [...target.querySelectorAll('.cell')].find((c) => c.querySelector('.label')?.textContent === 'Distance')!;
    const unit = (): string | null | undefined => cell().querySelector('.unit')?.textContent;
    expect(unit()).toBe('mi'); // swim range in the whole view stays miles (F72-Q2)
    props.setView({ a: 0, b: 600, sport: 'swimming' });
    flushSync();
    expect(unit()).toBe('yd');
    expect(cell().querySelector('.value')?.textContent).toMatch(/^[\d,]+$/);
    props.setView({ a: 660, b: 2460, sport: 'cycling' });
    flushSync();
    expect(unit()).toBe('mi');
  });
});
