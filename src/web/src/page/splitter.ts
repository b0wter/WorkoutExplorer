// The splitter between map and content in both two-column layouts (user decision 28 Oct 2026; F-13
// D-17, F-17 layout note). Bundle-only, like the pin toggle (D-19): a sibling of RouteMap's
// container inside #map-slot, so map-view's listeners never see its events and the charts' gestures
// are elsewhere. It writes one custom property, --map-w, on the layout wrapper; CSS owns every size,
// and MapLibre and the charts follow through their own ResizeObservers. A double click or double tap
// removes the property (back to the layout's default). The dragged split is remembered per layout
// (desktop, phone in landscape) through lib/prefs (AP-17, decision log 29 Oct 2026); a reset clears
// it. restoreSplit applies the stored split before the load; addSplitter (map-glue.ts, in the map-view chunk,
// ADR-011 decision 7 T-2) adds the handle once the map is ready.
// No URL state.

import type { SplitLayout, ViewerPrefs } from '../lib/prefs';

/** page.css's short-landscape two-column query; everything else with the splitter is desktop. */
/** The page's two-column layout (page.css), the one switch for everything that depends on it. */
export const TWO_COLUMN_QUERY = '(min-width: 1024px), (max-width: 1023.98px) and (orientation: landscape) and (max-height: 500px)';
export const LANDSCAPE_QUERY = '(max-width: 1023.98px) and (orientation: landscape) and (max-height: 500px)';

/** Map width as a share of the layout, while dragging. */
export const MIN_SHARE = 0.3;
export const MAX_SHARE = 0.7;

export interface SplitterStorage {
  prefs: Pick<ViewerPrefs, 'split' | 'setSplit' | 'clearSplit'>;
  /** Matches LANDSCAPE_QUERY; null: always the desktop split. */
  landscape: MediaQueryList | null;
}

export const layoutOf = (storage?: SplitterStorage): SplitLayout => (storage?.landscape?.matches ? 'landscape' : 'wide');

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
  // F-73 RS-3: the head script's root value (first paint) hands over to .layout, so a reset really resets.
  layout.ownerDocument.documentElement.style.removeProperty('--map-w');
  storage.landscape?.addEventListener('change', restore);
}
