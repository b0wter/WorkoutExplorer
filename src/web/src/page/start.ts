// The activity page's glue (F-13 design D-8, D-12, D-13, D-16): reads the page, loads the data
// file, mounts Svelte into the empty slots only and attaches the enhancers. The server markup
// (header, summary, laps) is never re-rendered (ADR-026 item 4). Static: ChartStack, RouteMap,
// RangePanel (F-17) and the fullscreen readout strip (F-33, ./strip) are the only components; the
// page components (components/page) are never imported here.
//
// The scrub recorder (src/measure) is reached ONLY through import() and only with ?measure
// (ADR-016, ADR-011 rev. 7): nothing here imports it statically, not even a type.

import { mount } from 'svelte';
import { createCursorStore } from '../lib/cursor';
import { createSelectionStore, type SelectionStore } from '../lib/selection';
import { loadDataFile } from '../lib/datafile';
import { loadGroup, onLanguage, pageLang } from '../lib/i18n';
import { routePresence } from '../lib/route';
import ChartStack from '../components/charts/ChartStack.svelte';
import RouteMap from '../components/map/RouteMap.svelte';
import RangePanel from '../components/range/RangePanel.svelte';
import { defaultUnits, createUnitsStore } from '../lib/units';
import { createViewerPrefs, type KeyValueStore } from '../lib/prefs';
import { parseRangeLink, rangeFragment } from '../lib/rangelink';
import { copier, copyField, enhanceCopyLink, viewingUrl, type ClipboardNav, enhanceExpiry, enhanceGpx, enhanceHowto, enhanceReport, enhanceSummaryInfo, enhanceUnits, loadHeader, addLangSelect, addPinToggle, addThemeSelect, addUnitsSelect } from './enhance';
import { enhanceLaps } from './laps';
import { codeWord, distanceAdvances, lapsGroup } from '../lib/page';
import { formatElapsed, formatSpeedOrPace, isPaceSport, speedOrPaceUnit } from '../lib/format';
import { loadErrorText } from './load-error';
import { addSplitter, restoreSplit, LANDSCAPE_QUERY, type SplitterStorage } from './splitter';
import { createLook, createMapProps, type Look } from './map-props.svelte';
import { mountStrip } from './strip';
import type { DataFileResult, WorkoutData, WorkoutMeta } from '../lib/datafile';
import type { ZoneSource } from '../zones';
import type { EffortWindow } from '../efforts';
import type { Facts } from '../summary-info';
import type { PinsView } from '../pins';
import type { RouteMapHandle } from '../components/map/map-view';

type RecorderModule = typeof import('../measure/recorder');

/** The data file URL exactly as the page's preload element carries it; never built (F-04). */
export function dataFileUrl(root: ParentNode): string | null {
  return root.querySelector('link#wx-data')?.getAttribute('href') ?? null;
}

function showLoadError(root: ParentNode, text: () => string): void {
  // Map and chart area both show the text (D-13); with the load failed nothing can shift
  // any more, so the chart area drops its reserved height.
  let hosts = ['#map-slot', '#charts-slot'].map((q) => root.querySelector(q)).filter((h) => h !== null);
  if (hosts.length === 0) hosts = [root.querySelector('#range-slot')].filter((h) => h !== null);
  for (const host of hosts) {
    const p = host.ownerDocument.createElement('p');
    p.className = 'load-error';
    p.textContent = text();
    host.replaceChildren(p);
    host.classList.add('load-failed');
    onLanguage(() => (p.textContent = text())); // F-54: a switch, or the language file arriving after the failure
  }
}

/** The range panel (F-17 R-9): mounted after the charts' first paint into #range-slot. The slot
 *  is hidden while there is no range; only the two transitions touch it, never a frame of a drag. */
function mountRangePanel(root: ParentNode, data: WorkoutData, selection: SelectionStore, look: Look, oncopy?: (button: HTMLButtonElement) => void): void {
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
      oncopy,
    },
  });
}

/** "Copy link to this selection" (F-40 SR-7): the viewing link plus the current range's fragment,
 *  through Copy link's routine. The fallback field never shows another range than the current one. */
function rangeCopy(viewUrl: string, data: WorkoutData, selection: SelectionStore, nav: ClipboardNav): (button: HTMLButtonElement) => void {
  const url = (): string => viewUrl + '#' + rangeFragment(data.time, selection.start(), selection.end());
  let field: HTMLInputElement | null = null;
  selection.subscribe(() => {
    field?.remove();
    field = null;
  });
  let copy: ((url: string) => void) | null = null;
  return (button) => {
    copy ??= copier(button, nav, (u) => {
      if (selection.start() >= 0 && u === url() && button.parentElement) field = copyField(button.parentElement, u);
    });
    copy(url());
  };
}

/** The edit page's seam (F-19 E-5): its already-decoded draft instead of the preload, and the
 *  to-be-hidden ranges to mark on the map (E-9; a getter, so the marks follow the panel). The page's
 *  look, handed over once, for the strip's units and the trim shading (F-47 TR-9). */
export interface EditSeam {
  readonly data: DataFileResult;
  readonly hidden: readonly (readonly [number, number])[];
  onlook(look: Look): void;
  /** F-49 ZN-7, ZN-8: the zones chunk's starter, after the charts' first paint; the edit glue
   *  decides when (its saved values) and passes its public window. Resolves to the refresh. */
  onzones(start: (src: ZoneSource) => Promise<(() => void) | void>): void;
  /** F-50 EF-1, EF-7: the efforts chunk's starter, likewise with the window; resolves to the refresh. */
  onefforts(start: (src: EffortWindow) => Promise<(() => void) | void>): void;
  /** F-58 SE-4: the summary dialog's facts, owner extras included, read on every open. */
  explain(): Facts;
  /** F-56 PN-12: the pins view over the draft, once the chunk is in (after the charts' first paint). */
  onpins?(view: PinsView): void;
}

/** Enhances the server markup, then loads and mounts. Never rejects. */
export async function startPage(root: ParentNode, win: Window = window, edit?: EditSeam): Promise<void> {
  // Viewer preferences on this device (lib/prefs): the getter itself may throw (blocked site data).
  const prefs = createViewerPrefs((): KeyValueStore => win.localStorage);
  const split: SplitterStorage = { prefs, landscape: typeof win.matchMedia === 'function' ? win.matchMedia(LANDSCAPE_QUERY) : null };
  // The stored split before any await (QA-17-r1-2, F-13 D-10): it must not wait for the data file.
  restoreSplit(root, split);
  enhanceCopyLink(root, win.location.origin, win.navigator);
  enhanceReport(root); // F-28 RF-2: before any await, so a failing data file still leaves Report working
  enhanceHowto(root, prefs.howtoOpened(), prefs.setHowtoOpened); // F-39 HU-6: likewise; viewing page only (the markup)
  enhanceExpiry(root, Date.now());
  // F-56 PN-7: the pins list's reserved height, only with JavaScript (the template hides the slot):
  // Count one-line rows of 32 px, the 32 px heading and 8 px below (src/pins/pins.css).
  // The viewing page has the slot only with a pins file; the edit page always (ActivityPage).
  const pinSlot = root.querySelector<HTMLElement>('.pins-slot');
  if (pinSlot?.dataset.count) {
    pinSlot.style.minHeight = +pinSlot.dataset.count * 32 + 40 + 'px';
    pinSlot.hidden = false;
  }
  // One units store per page (D-3): a remembered drop-down choice, else the browser language (D-4,
  // DS-1). The server markup is metric: swapped once the data file is decoded (D-5).
  const units = createUnitsStore(prefs.units() ?? defaultUnits(win.navigator.languages, win.navigator.language));
  // F-39 HU-3: the edit page keeps today's hint; the viewing page takes "done" from the head script's
  // attribute, read once, so the hint agrees with the space the CSS reserved.
  const hint = edit ? 'plain' : win.document.documentElement.dataset.hint === 'done' ? 'none' : 'once';
  const look = createLook(units, typeof win.matchMedia === 'function' ? win.matchMedia('(prefers-color-scheme: dark)') : null, prefs.theme(), hint);
  edit?.onlook(look);
  const hintDone = (): void => {
    const html = win.document.documentElement;
    if (look.hint !== 'once' || html.dataset.hint) return;
    // The box fades and its reserve shrinks (page.css 'gone', 200 ms), then goes (user 10 Nov 2026).
    html.dataset.hint = 'gone';
    win.setTimeout(() => {
      look.hint = 'none';
      html.dataset.hint = 'done';
    }, 200);
    prefs.setHintDone();
  };
  // DS-4: the head script applied the stored theme before the first paint; a choice sets the
  // attribute first, so the charts and the map read the new CSS when the scheme prop changes.
  // The color-scheme meta follows too: iOS Safari paints the area under its toolbar from the meta,
  // not the root's CSS color-scheme (white strip under a forced dark page, user 14 Nov 2026).
  const scheme = win.document.querySelector<HTMLMetaElement>('meta[name="color-scheme"]');
  const metaScheme = (theme: string): void => scheme?.setAttribute('content', theme === 'system' ? 'light dark' : theme);
  metaScheme(look.theme);
  addThemeSelect(root, look.theme, (theme) => {
    const html = win.document.documentElement;
    if (theme === 'system') delete html.dataset.theme;
    else html.dataset.theme = theme;
    metaScheme(theme);
    prefs.setTheme(theme);
    look.theme = theme;
  });

  // F-54 LG-1: the drop-down's choice applies to every loaded group and is remembered (DS-1).
  // The loader is a lazy chunk: an English page view never needs it (F-54 bundle budget).
  const i18n = (): Promise<typeof import('../lib/i18n/load')> => import('../lib/i18n/load');
  addLangSelect(root, (l) => {
    prefs.setLang(l);
    void i18n().then((m) => m.setLang(l, win.document));
  });
  // The summary and laps (and on the viewing page the header) follow units and language once a
  // metadata is known: the streamed prefix's or the full decode's, whichever comes first.
  let enhanced = false;
  const enhance = (meta: WorkoutMeta): void => {
    if (enhanced) return;
    enhanced = true;
    // The viewing page only: the edit page builds its strip (F-47 TR-9: it may show the trimmed
    // summary) and its laps (F-65 AL-8) in its units itself.
    if (!edit) enhanceUnits(root, meta, units);
  };

  const url = edit ? '' : dataFileUrl(root);
  // F-54 LG-2, LG-4: the viewing page's first language pass, released only while the head script holds.
  let prefix: Promise<WorkoutMeta | null> = Promise.resolve(null);
  let fromPrefix: ((r: Response) => void) | undefined;
  if (!edit && pageLang(win.document) !== 'en' && url !== null) {
    const reader = import('../lib/datafile/prefix'); // F-54: lazy, with the loader
    // The clone is taken before the full read starts; the reader may arrive later.
    prefix = new Promise((ok) => (fromPrefix = (r) => void ((c) => reader.then((m) => m.streamMeta(c), () => null).then(ok))(r.clone())));
  }
  const english = pageLang(win.document) === 'en';
  const markup = (): void => void i18n().then((m) => m.applyMarkup(root));
  if (!edit) onLanguage(markup);
  void loadGroup('page', win.document); // English: only recorded, for a later switch
  if (url === null) {
    console.warn('page: no data file link');
    if (!english) void i18n().then((m) => m.startLang(['page'], win.document, undefined, markup));
    showLoadError(root, () => loadErrorText('network'));
    return;
  }
  const measure = new URLSearchParams(win.location.search).has('measure');
  performance.mark('wx-page:load-start');
  // In parallel: the data file and, only with ?measure, the recorder chunk.
  const loading = edit ? Promise.resolve(edit.data) : loadDataFile(() => fetch(url), fromPrefix);
  if (!edit && !english) {
    const meta = prefix.then((m) => m ?? loading.then((r) => (r.ok ? r.data.meta : null)));
    void i18n().then((m) =>
      m.startLang(['page'], win.document, Promise.all([meta, loadHeader()]), () => {
        m.applyMarkup(root);
        void meta.then((d) => d && enhance(d));
      }),
    );
  }
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
    showLoadError(root, () => loadErrorText(result.error.code));
    return;
  }
  const data = result.data;
  enhance(data.meta);
  addUnitsSelect(root, units, prefs.setUnits);
  // F-58 SE-1: the viewing page's dialog gets these public metadata fields only, never streams,
  // positions, route state or the title (AC 39); owner facts come only through the edit seam.
  const { sport, summary, sessions, lengths, zones } = data.meta;
  enhanceSummaryInfo(root, units, edit ? edit.explain : () => ({ sport, summary, sessions, lengths, zones: Object.keys(zones).length > 0 }));
  const cursor = createCursorStore(data.sampleCount, (f) => requestAnimationFrame(() => f()));
  const selection = createSelectionStore(data.sampleCount, (f) => requestAnimationFrame(() => f()));
  // F-40 SR-4: the viewing page only (no edit seam) reads the range link, once, before the charts mount.
  const hash = edit ? '' : win.location.hash;
  const linked = parseRangeLink(hash, data.time);
  if (linked) selection.set(linked[0], linked[1]);
  // The link's own range is not the viewer's (SR-5, SR-6). The viewer's first change or Clear drops
  // any fragment, once and without a history entry; every range of the viewer's (drag or lap/split
  // row) marks the hint done (F-39 HU-4).
  const [la, lb] = [selection.start(), selection.end()];
  let viewer = false;
  selection.subscribe((a, b) => {
    if (!viewer) {
      if (a === la && b === lb) return;
      viewer = true;
      if (hash) win.history.replaceState(win.history.state, '', win.location.pathname + win.location.search);
    }
    if (a >= 0) hintDone();
  });
  // F-18: the row mark; row taps once the range panel is mounted (L-7, QA-18-r1-3).
  const laps = enhanceLaps(root, data, selection, win);
  const viewUrl = edit ? null : viewingUrl(win.location.origin, root.querySelector('a[data-copy-link]')?.getAttribute('href') ?? null);
  const hasRoute = routePresence(data) === 'route';
  // F-57 GX-2: viewing page only (the edit seam's data is the unhidden draft); closed inputs.
  // data.position holds latitude and longitude only (lib/datafile decode).
  if (!edit && hasRoute) enhanceGpx(root, { ...data.position!, altitude: data.metrics.altitude, startTime: data.meta.start_time, zone: data.meta.time_zone, pinsUrl: pinSlot?.dataset.url });
  const mapSlot = root.querySelector<HTMLElement>('#map-slot');
  const chartsSlot = root.querySelector<HTMLElement>('#charts-slot');
  const mod = recorderModule ? await recorderModule : null;
  // Before the components mount, so that its first listener comes before the charts' (ADR-016).
  const recorder = mod
    ? mod.createRecorder({ cursor, data, name: url.slice(url.lastIndexOf('/') + 1), hasRoute })
    : null;

  // F-56 PN-10: the map's seam for the pins chunk, once the map is ready.
  let pinMap: RouteMapHandle | null = null;
  let onPinMap: ((m: RouteMapHandle) => void) | null = null;
  const map = hasRoute && mapSlot ? createMapProps(data, cursor, selection, look, (h) => {
    performance.mark('wx-page:map-ready');
    pinMap = h;
    onPinMap?.(h);
    addPinToggle(root, mapSlot);
    const box = mapSlot.querySelector<HTMLElement>('.route-map');
    if (box) mountStrip(box, data, cursor, units); // F-33 CR-2: the fullscreen box holds it
    recorder?.mapReady();
  }, edit, prefs) : null;
  const ready = (): void => {
    laps.taps();
    map?.startLoading();
    // F-56 PN-9: the pins chunk (it fetches the file), only with a pins file or on the edit page.
    if (pinSlot)
      import('../pins').then(
        (m) => m.startPins({ slot: pinSlot, data, cursor, selection, units, look, edit, map: (cb) => (pinMap ? cb(pinMap) : (onPinMap = cb)) }),
        () => {
          pinSlot.hidden = true;
          console.warn('page: pins chunk failed');
        },
      );
    // F-42 MS-1, MS-9: section lines, names and separator ranges are a lazy chunk, for a workout
    // whose sessions hold two or more sports only; failing, the page stays as it is.
    if (new Set(data.meta.sessions.map((s) => s.sport)).size > 1) import('../sections').then((m) => m.startSections(data, look, laps.legs), () => console.warn('page: sections chunk failed'));
    // F-46 LS-4, F-48 PL-8: the switch, the splits with their drop-down and the lengths are a lazy
    // chunk; failing, laps only.
    const splits = distanceAdvances(data);
    if (splits || data.meta.lengths.length) {
      const fmt = [lapsGroup, codeWord, formatElapsed, formatSpeedOrPace, speedOrPaceUnit, isPaceSport] as const;
      import('../splits').then((m) => m.startSplits(root, data, units, laps, fmt, splits), () => console.warn('page: splits chunk failed'));
    }
    // F-49 ZN-8: the zones section is a lazy chunk, here over the whole public data file (ZN-7).
    const zones = (src?: ZoneSource) => import('../zones').then((m) => m.startZones(root, data, selection, formatElapsed, src), () => console.warn('page: zones chunk failed'));
    if (edit) edit.onzones(zones);
    else if (Object.keys(data.meta.zones).length) void zones();
    // F-50 EF-7: best efforts with the same trigger, when power or a pace sport's advancing distance may
    // give rows; the chunk applies the exact rules (moving time, EF-2), a false positive costs a download.
    if (data.metrics.power || (splits && [data.meta, ...data.meta.sessions].some((s) => isPaceSport(s.sport)))) {
      const efforts = (src?: EffortWindow) => import('../efforts').then((e) => e.startEfforts(root, data, selection, units, win, src), () => console.warn('page: efforts chunk failed'));
      if (edit) edit.onefforts(efforts);
      else void efforts();
    }
  };
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
        get hint() {
          return look.hint;
        },
        onhint: hintDone,
        get decorate() {
          return look.decorate;
        },
        onready: () => {
          performance.mark('wx-page:charts-ready');
          recorder?.chartsReady();
          mountRangePanel(root, data, selection, look, viewUrl ? rangeCopy(viewUrl, data, selection, win.navigator) : undefined);
          ready();
        },
      },
    });
  } else {
    // No chart will ever call onready: the map may start right away.
    performance.mark('wx-page:charts-ready');
    recorder?.chartsReady();
    ready();
  }
}
