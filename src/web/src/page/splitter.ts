// The splitter between map and content in both two-column layouts (user decision 28 Oct 2026; F-13
// D-17, F-17 layout note). Bundle-only, like the pin toggle (D-19): a sibling of RouteMap's
// container inside #map-slot, so map-view's listeners never see its events and the charts' gestures
// are elsewhere. It writes one custom property, --map-w, on the layout wrapper; CSS owns every size,
// and MapLibre and the charts follow through their own ResizeObservers. A double click or double tap
// removes the property (back to the layout's default). The dragged split is remembered per layout
// (desktop, phone in landscape) through lib/prefs (AP-17, decision log 29 Oct 2026); a reset clears
// it. restoreSplit applies the stored split before the load, addSplitter adds the handle after it.
// No URL state.

import type { SplitLayout, ViewerPrefs } from '../lib/prefs';

/** page.css's short-landscape two-column query; everything else with the splitter is desktop. */
export const LANDSCAPE_QUERY = '(max-width: 1023.98px) and (orientation: landscape) and (max-height: 500px)';

/** Map width as a share of the layout, while dragging. */
export const MIN_SHARE = 0.3;
export const MAX_SHARE = 0.7;
/** Two taps within this time without a drag reset the ratio. */
const DOUBLE_MS = 350;
/** Movement below this many px is a tap, not a drag. */
const SLOP_PX = 4;

export interface SplitterStorage {
  prefs: Pick<ViewerPrefs, 'split' | 'setSplit' | 'clearSplit'>;
  /** Matches LANDSCAPE_QUERY; null: always the desktop split. */
  landscape: MediaQueryList | null;
}

const layoutOf = (storage?: SplitterStorage): SplitLayout => (storage?.landscape?.matches ? 'landscape' : 'wide');

/** Applies the stored split of the current layout, or the default, now and whenever the layout
 *  changes. Called by the page glue before its first await (QA-17-r1-2, F-13 D-10), so that the
 *  split does not wait for the data file and the splitter element. */
export function restoreSplit(root: ParentNode, storage: SplitterStorage): void {
  const layout = root.querySelector<HTMLElement>('.layout');
  if (!layout) return;
  const restore = (): void => {
    const pct = storage.prefs.split(layoutOf(storage));
    if (pct !== null && pct >= MIN_SHARE * 100 && pct <= MAX_SHARE * 100) layout.style.setProperty('--map-w', pct.toFixed(2) + '%');
    else layout.style.removeProperty('--map-w');
  };
  restore();
  storage.landscape?.addEventListener('change', restore);
}

export function addSplitter(root: ParentNode, slot: HTMLElement, storage?: SplitterStorage): HTMLElement | null {
  const layout = root.querySelector<HTMLElement>('.layout');
  if (!layout || slot.querySelector('.splitter')) return null;
  const el = slot.ownerDocument.createElement('div');
  el.className = 'splitter';
  let id = -1;
  let x0 = 0;
  let share0 = 0;
  let width = 1;
  let moved = false;
  let lastTap = -Infinity;
  let frame = 0;
  let next = 0;

  el.addEventListener('pointerdown', (e) => {
    if (id !== -1 || e.button !== 0) return;
    e.preventDefault(); // no text selection, no emulated mouse events
    id = e.pointerId;
    el.setPointerCapture(id);
    width = layout.getBoundingClientRect().width || 1;
    // From the box as laid out, so a share clamped by the columns' minimum widths starts no dead zone.
    share0 = slot.getBoundingClientRect().width / width;
    x0 = e.clientX;
    moved = false;
  });
  el.addEventListener('pointermove', (e) => {
    if (e.pointerId !== id) return;
    const dx = e.clientX - x0;
    if (!moved && Math.abs(dx) < SLOP_PX) return;
    moved = true;
    next = Math.min(MAX_SHARE, Math.max(MIN_SHARE, share0 + dx / width)) * 100;
    if (frame === 0) {
      frame = requestAnimationFrame(() => {
        frame = 0;
        layout.style.setProperty('--map-w', next.toFixed(2) + '%');
      });
    }
  });
  const end = (e: PointerEvent): void => {
    if (e.pointerId !== id) return;
    id = -1;
    if (moved || e.type === 'pointercancel') {
      lastTap = -Infinity;
      if (moved) storage?.prefs.setSplit(layoutOf(storage), next);
    } else if (e.timeStamp - lastTap < DOUBLE_MS) {
      lastTap = -Infinity;
      layout.style.removeProperty('--map-w');
      storage?.prefs.clearSplit(layoutOf(storage));
    } else {
      lastTap = e.timeStamp;
    }
  };
  el.addEventListener('pointerup', end);
  el.addEventListener('pointercancel', end);
  slot.appendChild(el);
  return el;
}
