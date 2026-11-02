// The activity page's glue (F-13 design D-8, D-12, D-13, D-16): reads the page, loads the data
// file, mounts Svelte into the empty slots only and attaches the enhancers. The server markup
// (header, summary, laps) is never re-rendered (ADR-026 item 4). Static: ChartStack, RouteMap and
// RangePanel (F-17) are the only components; the page components (components/page) are never
// imported here.
//
// The scrub recorder (src/measure) is reached ONLY through import() and only with ?measure
// (ADR-016, ADR-011 rev. 7): nothing here imports it statically, not even a type.

import { mount } from 'svelte';
import { createCursorStore } from '../lib/cursor';
import { createSelectionStore, type SelectionStore } from '../lib/selection';
import { loadDataFile } from '../lib/datafile';
import { routePresence } from '../lib/route';
import ChartStack from '../components/charts/ChartStack.svelte';
import RouteMap from '../components/map/RouteMap.svelte';
import RangePanel from '../components/range/RangePanel.svelte';
import { defaultUnits, createUnitsStore } from '../lib/units';
import { createViewerPrefs, type KeyValueStore } from '../lib/prefs';
import { enhanceCopyLink, enhanceExpiry, enhanceUnits, addPinToggle, addUnitsSelect } from './enhance';
import { enhanceLaps } from './laps';
import { loadErrorText } from './load-error';
import { addSplitter, restoreSplit, LANDSCAPE_QUERY, type SplitterStorage } from './splitter';
import { createLook, createMapProps, type Look } from './map-props.svelte';
import type { DataFileResult, WorkoutData } from '../lib/datafile';

type RecorderModule = typeof import('../measure/recorder');

/** The data file URL exactly as the page's preload element carries it; never built (F-04). */
export function dataFileUrl(root: ParentNode): string | null {
  return root.querySelector('link#wx-data')?.getAttribute('href') ?? null;
}

function showLoadError(root: ParentNode, text: string): void {
  // Map and chart area both show the text (D-13); with the load failed nothing can shift
  // any more, so the chart area drops its reserved height.
  let hosts = ['#map-slot', '#charts-slot'].map((q) => root.querySelector(q)).filter((h) => h !== null);
  if (hosts.length === 0) hosts = [root.querySelector('#range-slot')].filter((h) => h !== null);
  for (const host of hosts) {
    const p = host.ownerDocument.createElement('p');
    p.className = 'load-error';
    p.textContent = text;
    host.replaceChildren(p);
    host.classList.add('load-failed');
  }
}

/** The range panel (F-17 R-9): mounted after the charts' first paint into #range-slot. The slot
 *  is hidden while there is no range; only the two transitions touch it, never a frame of a drag. */
function mountRangePanel(root: ParentNode, data: WorkoutData, selection: SelectionStore, look: Look): void {
  const slot = root.querySelector<HTMLElement>('#range-slot');
  if (!slot) return;
  let shown: boolean | null = null;
  selection.subscribe((a) => {
    const on = a >= 0;
    if (on === shown) return;
    shown = on;
    slot.hidden = !on;
  });
  mount(RangePanel, {
    target: slot,
    props: {
      data,
      selection,
      get units() {
        return look.units;
      },
    },
  });
}

/** The edit page's seam (F-19 E-5): its already-decoded draft instead of the preload, and the
 *  to-be-hidden ranges to mark on the map (E-9; a getter, so the marks follow the panel). */
export interface EditSeam {
  readonly data: DataFileResult;
  readonly hidden: readonly (readonly [number, number])[];
}

/** Enhances the server markup, then loads and mounts. Never rejects. */
export async function startPage(root: ParentNode, win: Window = window, edit?: EditSeam): Promise<void> {
  // Viewer preferences on this device (lib/prefs): the getter itself may throw (blocked site data).
  const prefs = createViewerPrefs((): KeyValueStore => win.localStorage);
  const split: SplitterStorage = { prefs, landscape: typeof win.matchMedia === 'function' ? win.matchMedia(LANDSCAPE_QUERY) : null };
  // The stored split before any await (QA-17-r1-2, F-13 D-10): it must not wait for the data file.
  restoreSplit(root, split);
  enhanceCopyLink(root, win.location.origin, win.navigator);
  enhanceExpiry(root, Date.now());

  const url = edit ? '' : dataFileUrl(root);
  if (url === null) {
    console.warn('page: no data file link');
    showLoadError(root, loadErrorText('network'));
    return;
  }
  const measure = new URLSearchParams(win.location.search).has('measure');
  performance.mark('wx-page:load-start');
  // In parallel: the data file and, only with ?measure, the recorder chunk.
  const loading = edit ? Promise.resolve(edit.data) : loadDataFile(() => fetch(url));
  const recorderModule: Promise<RecorderModule | null> | null = measure
    ? import('../measure/recorder').catch((error: unknown) => {
        console.warn('page: scrub recorder not loaded', error);
        return null;
      })
    : null;
  const result = await loading;
  performance.mark('wx-page:data-ready');
  if (!result.ok) {
    console.warn(result.error.message);
    showLoadError(root, loadErrorText(result.error.code));
    return;
  }
  const data = result.data;
  // One units store per page (D-3): a remembered drop-down choice, else the browser language (D-4,
  // DS-1). The server markup is metric: swapped now when that is imperial, and on every later change (D-5).
  const units = createUnitsStore(prefs.units() ?? defaultUnits(win.navigator.languages, win.navigator.language));
  enhanceUnits(root, data.meta, units);
  addUnitsSelect(root, units, prefs.setUnits);
  const look = createLook(units, typeof win.matchMedia === 'function' ? win.matchMedia('(prefers-color-scheme: dark)') : null);
  const cursor = createCursorStore(data.sampleCount, (f) => requestAnimationFrame(() => f()));
  const selection = createSelectionStore(data.sampleCount, (f) => requestAnimationFrame(() => f()));
  // F-18: the row mark, the laps / splits switch and the splits table; row taps once the range
  // panel is mounted (L-7, QA-18-r1-3).
  const lapTaps = enhanceLaps(root, data, selection, units, win);
  const hasRoute = routePresence(data) === 'route';
  const mapSlot = root.querySelector<HTMLElement>('#map-slot');
  const chartsSlot = root.querySelector<HTMLElement>('#charts-slot');
  const mod = recorderModule ? await recorderModule : null;
  // Before the components mount, so that its first listener comes before the charts' (ADR-016).
  const recorder = mod
    ? mod.createRecorder({ cursor, data, name: url.slice(url.lastIndexOf('/') + 1), hasRoute })
    : null;

  const map = hasRoute && mapSlot ? createMapProps(data, cursor, selection, look, () => {
    performance.mark('wx-page:map-ready');
    addPinToggle(root, mapSlot);
    recorder?.mapReady();
  }, edit) : null;
  if (map && mapSlot) {
    mount(RouteMap, { target: mapSlot, props: map.props });
    addSplitter(root, mapSlot, split);
  }
  if (chartsSlot) {
    mount(ChartStack, {
      target: chartsSlot,
      props: {
        data,
        cursor,
        selection,
        get units() {
          return look.units;
        },
        get scheme() {
          return look.scheme;
        },
        // AP-5: the remembered axis; a workout without distance shows time and stores nothing.
        initialAxis: prefs.axis() ?? 'time',
        onaxis: prefs.setAxis,
        onready: () => {
          performance.mark('wx-page:charts-ready');
          recorder?.chartsReady();
          mountRangePanel(root, data, selection, look);
          lapTaps();
          map?.startLoading();
        },
      },
    });
  } else {
    // No chart will ever call onready: the map may start right away.
    performance.mark('wx-page:charts-ready');
    recorder?.chartsReady();
    lapTaps();
    map?.startLoading();
  }
}
