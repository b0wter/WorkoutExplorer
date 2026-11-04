// F-33 (AP-21; architecture CR-1 to CR-7): the cursor readout as a strip at the bottom of the map's
// container, a second CursorReadout instance on the same cursor store. page.css shows it only in
// fullscreen; here it is hidden while there is no cursor (on the transition only, never per frame)
// and remounted per units value, since the component fixes its fields at mount. Its height
// reaches map-view.css's bottom control corners as --wx-strip (CR-4).

import { mount, unmount } from 'svelte';
import CursorReadout from '../components/charts/CursorReadout.svelte';
import type { CursorStore } from '../lib/cursor';
import type { WorkoutData } from '../lib/datafile';
import type { Units } from '../lib/format';
import type { UnitsStore } from '../lib/units';

export function mountStrip(container: HTMLElement, data: WorkoutData, cursor: CursorStore, units: UnitsStore): void {
  const strip = container.ownerDocument.createElement('div');
  strip.className = 'wx-strip';
  container.appendChild(strip);
  let shown: boolean | null = null;
  cursor.subscribe((i) => {
    if ((i !== null) === shown) return;
    shown = i !== null;
    strip.hidden = !shown;
  });
  const put = (u: Units): object => mount(CursorReadout, { target: strip, props: { data, cursor, units: u } });
  let readout = put(units.get());
  units.subscribe((u) => {
    void unmount(readout);
    readout = put(u);
  });
  if (typeof ResizeObserver === 'function') {
    new ResizeObserver((e) => container.style.setProperty('--wx-strip', e[0].contentRect.height + 'px')).observe(strip);
  }
}
