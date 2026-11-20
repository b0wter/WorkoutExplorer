// The owner pages' glue (F-19 design E-1, E-4 to E-6, E-9; F-20 C-1 to C-8). LAZY: reached only
// through main.ts's import() on edit.html, which serves /edit/* and /published/*. The edit page
// loads settings and draft.bin through the owner client in parallel, mounts the D-9 root and the
// panel into its slot, then runs the viewing page's glue on it with the decoded draft (one glue,
// F-13 D-19). The published page loads the settings only. Never sees the edit token: lib/owner
// holds it (the published view gets only lib/owner's edit link); its one other exit, owner.token,
// goes only into the My workouts list (F-53 MW-3, lib/mine) in `list`.
// Only mount() from svelte: flushSync, unmount and the like would join the shared activity entry.

import { t } from './texts';
import { serverText } from '../lib/errors/texts';

/** F-54 LG-6: an owner failure's text, by its code; the server's message for an unlisted code. */
const said = (r: { message: string; code?: string }): string => serverText({ error: r.code, message: r.message });
import { t as tc } from '../components/page/texts';
import { onLanguage } from '../lib/i18n';
import { langState } from '../page/map-props.svelte';
import { mount } from 'svelte';
import './edit.css';
import ActivityPage from '../components/page/ActivityPage.svelte';
import EditPanel from '../components/edit/EditPanel.svelte';
import PublishedView from '../components/edit/PublishedView.svelte';
import { loadDataFile, type Summary, type WorkoutMeta, type Zones } from '../lib/datafile';
import { summaryItems } from '../lib/page';
import type { ChartDecorator } from '../components/charts/uplot-options';
import type { Look } from '../page/map-props.svelte';
import { openOwner, type OwnerClient, type OwnerLocation, type OwnerResult, type Settings, type SettingsPatch } from '../lib/owner';
import { impressum } from '../lib/page/impressum';
import { buildPageViewModel, defaultTitle } from '../lib/page/view';
import { enhanceExpiry } from '../page/enhance';
import { loadErrorText } from '../page/load-error';
import { startPage } from '../page/start';
import { routePresence } from '../lib/route';
import { basemapJobs } from './basemap-job';
import { HIDE_STEP_M, hidingRanges, trimIndex } from '../components/edit/fields';
import { startPinEditor, type PinEditor } from './pins';
import type { ZoneSource } from '../zones';
import * as mine from '../lib/mine';

type Ranges = readonly (readonly [number, number])[];

/** F-47 TR-9, F-55 HR-8: the trimmed and to-be-hidden ranges shaded in one chart (MS-7's seam), as
 *  on the map. Replaces what it drew before; nothing without a range. */
function shade(ranges: Ranges): ChartDecorator {
  return (over, x) => {
    over.querySelector('.wx-trim')?.remove();
    if (ranges.length === 0) return;
    const doc = over.ownerDocument;
    const box = doc.createElement('div');
    box.className = 'wx-trim';
    for (const [i, j] of ranges) {
      const el = box.appendChild(doc.createElement('div'));
      el.style.left = x(i) + 'px';
      el.style.width = x(j) - x(i) + 'px';
    }
    over.prepend(box);
  };
}

/** F-58 SE-4, F-55 HR-9: the SAVED start and end hiding radius in metres, 0 = not hidden; the one
 *  source of the dialog's radius text. */
const radii = (s: Settings): [number, number] => [s.hide_start_m, s.hide_end_m];

/** The generic page's content (generic.html, E-4), in this page's theme. No workout data. */
export function showGeneric(doc: Document, text = t('gone'), upload = ''): void {
  doc.title = 'Workout Explorer';
  const link = doc.getElementById('wx-impressum'); // kept: the other footers copy from it (F-38 IM-7)
  doc.body.innerHTML = `<main class="generic"><h1>Workout Explorer</h1><p></p>${upload}</main><footer class="foot generic-foot"><a href="/about"></a></footer>`;
  doc.querySelector('main.generic > p')!.textContent = text; // texts are never HTML (LG-7)
  doc.querySelector('footer a')!.textContent = tc('about');
  if (link) doc.body.prepend(link);
  impressum(doc.querySelector('footer')!);
}

/** The deleted screen (F-20 C-8): E-4's teardown with a fixed sentence and the upload link. */
const showDeleted = (doc: Document): void => {
  showGeneric(doc, t('deleted'), '<p><a href="/"></a></p>');
  doc.querySelector('main a[href="/"]')!.textContent = t('uploadWorkout');
};

/** How long the original's object URL lives after the click (C-6). */
// shortcut: a fixed 40 s (FileSaver.js's value) because Safari can drop a download whose URL is
// revoked at once; upgrade if a slow phone still loses one.
const REVOKE_MS = 40_000;

/** A server fault is not "not found" (E-4): its message, never the generic page. */
function showMessage(host: HTMLElement, message: string): void {
  const p = host.ownerDocument.createElement('p');
  p.className = 'generic';
  p.textContent = message;
  host.replaceChildren(p);
}

/** F-53 MW-3, MW-6: the owner pages' My workouts writes; storage failures are contained in lib/mine. */
function list(win: Window, owner: OwnerClient) {
  const store = () => win.localStorage;
  return {
    /** Not found or deleted: removes the entry only when it holds this address's token (P-12). */
    remove: () => mine.remove(store, owner.publicId, owner.token()),
    upsert: (s: Settings, meta: Pick<WorkoutMeta, 'sport' | 'start_time' | 'time_zone'> | null) =>
      mine.upsert(store, {
        id: owner.publicId,
        token: owner.token(),
        ...fields(s),
        // The header's sport and local start date (MW-2); without a decoded draft the stored ones stay.
        sport: meta?.sport,
        date: meta ? new Date(Date.parse(meta.start_time) + (meta.time_zone.known ? meta.time_zone.offset_seconds : 0) * 1000).toISOString().slice(0, 10) : undefined,
      }),
    /** The published page: an existing entry only (MW-3). */
    update: (s: Settings) => mine.update(store, owner.publicId, fields(s)),
  };
}
/** MW-2: the stored title when not blank, else the suggestion ('' = the default title applies). */
const fields = (s: Settings) => ({ title: s.title.trim() !== '' ? s.title : s.suggested_title, created: s.created_at, expires: s.expires_at, published: s.visibility === 'unlisted' });

/** One owner action at a time (C-2), shared by both pages; Delete (C-7, C-8). */
function actions(doc: Document, owner: OwnerClient, mineList: ReturnType<typeof list>) {
  /** '' or the running action's text. */
  let busy = $state('');
  let status = $state('');
  /** The busy state outlives its job (a navigation, C-3). */
  let kept = false;
  const ui = {
    get busy() {
      return busy;
    },
    get status() {
      return status;
    },
    set status(s: string) {
      status = s;
    },
    /** Runs one action unless another is running; `true` from the job keeps the busy state
     *  until the page unloads (a navigation, C-3). */
    run: async (text: string, job: () => Promise<boolean | void>): Promise<void> => {
      if (busy) return;
      busy = text;
      let keep = false;
      try {
        keep = (await job()) === true;
      } finally {
        if (!keep) busy = '';
        kept = keep;
      }
    },
    /** Ends a kept busy state: the page is shown again after its navigation. */
    release: (): void => {
      if (kept) {
        kept = false;
        busy = '';
      }
    },
    remove: (): Promise<void> =>
      ui.run(t('deleting'), async () => {
        const r = await owner.delete();
        if (r.ok || r.notFound) mineList.remove(); // MW-3, AC 46, 48
        if (r.ok) showDeleted(doc);
        else if (r.notFound) showGeneric(doc);
        else status = said(r); // O-10: it may still exist; trying again is safe
      }),
  };
  return ui;
}

/** Never rejects. `loc` is the window's location (a test passes its own, to record navigations). */
export async function startEdit(host: HTMLElement, win: Window = window, loc: OwnerLocation = win.location): Promise<void> {
  langState(); // F-54: before the first mount, so every component's texts follow a switch
  const doc = host.ownerDocument;
  const owner = openOwner(loc);
  if (!owner) return showGeneric(doc);
  const mineList = list(win, owner);
  /** ADR-025's not-found: the generic page, and the list entry goes (F-53 MW-3, P-12). */
  const gone = (): void => {
    mineList.remove();
    showGeneric(doc);
  };
  if (owner.kind === 'published') return startPublished(host, owner, loc.origin, mineList, gone);
  const [got, result] = await Promise.all([owner.settings(), loadDataFile(owner.draft)]);
  if (!got.ok && got.notFound) return gone();
  if (!result.ok && result.error.code === 'http-status' && result.error.status === 404) return gone();
  if (!got.ok) return showMessage(host, said(got));
  const data = result.ok ? result.data : null;

  let settings = $state.raw(got.value);
  const ui = actions(doc, owner, mineList);
  /** F-55 HR-8: the sliders' radii in metres, unsaved too; set at most once per animation frame. */
  let ends = $state.raw<readonly [number, number]>(radii(got.value));
  let pending: readonly [number, number] | null = null;
  /** F-56 PN-12: the pin editor, once the pins chunk is in. */
  let pinEditor: PinEditor | null = null;
  const onhide = (startM: number, endM: number): void => {
    if (pending === null)
      requestAnimationFrame(() => {
        ends = pending!;
        pending = null;
        pinEditor?.refresh();
      });
    pending = [startM, endM];
  };
  /** Apart from settings: a restore updates it without refilling the form (C-2). */
  let visibility = $state(got.value.visibility);
  const build = (s: Settings) =>
    data &&
    buildPageViewModel(data, {
      title: s.title,
      description: s.description,
      expiresAt: s.expires_at,
      publicId: owner.publicId,
      dataFile: '',
      suggestedTitle: s.suggested_title,
      // F-58 SE-2: from the saved ends, as the Go builder does (non-null = trimmed).
      trim: s.trim_start_at !== null ? (s.trim_end_at !== null ? 'both' : 'start') : s.trim_end_at !== null ? 'end' : '',
    });
  const first = build(got.value);
  let model = $state.raw(first);
  // F-54: the view model's strings (header, laps, device, leg names) in the new language.
  onLanguage(() => {
    const next = build(settings);
    if (!next || !model) return;
    model = next;
    doc.title = next.Page.Title;
    setTimeout(() => enhanceExpiry(doc, Date.now())); // after Svelte's update, as on a save
  });
  /** F-47 TR-9: the trim handles [a, b] as sample indices, live, unsaved too. */
  const last = (data?.sampleCount ?? 1) - 1;
  let cut = $state.raw<readonly [number, number]>([0, last]);
  /** The page's look (startPage's seam): the strip's units. */
  let look = $state.raw<Look>();
  /** The strip's source (TR-9): with a trim stored, the server's trimmed values only, never the
   *  draft's full totals; without them (an owner-view failure) no values at all. */
  const strip = (s: Settings, meta: WorkoutMeta): Pick<WorkoutMeta, 'sport' | 'summary'> =>
    s.trim_start_at === null && s.trim_end_at === null ? meta : s.trimmed ? { sport: s.trimmed.sport, summary: s.trimmed.summary as Summary } : { sport: meta.sport, summary: {} };

  /** F-53 MW-3: the list entry from saved settings, with the header's (trimmed) sport. */
  const record = (s: Settings): void => mineList.upsert(s, data && { ...data.meta, sport: strip(s, data.meta).sport });
  record(got.value);

  /** F-49 ZN-7: what the public build writes, from the SAVED settings only (E-6): the sharer's
   *  value as a basis before the draft's zone values, over the saved trim window (TR-9). */
  const startS = data ? Date.parse(data.meta.start_time) / 1000 : 0;
  const zoneSource: ZoneSource = {
    zones: (): Zones => {
      const hr = settings.zones_max_heart_rate;
      const ftp = settings.zones_threshold_power;
      return { ...data?.meta.zones, ...(hr !== null && { heart_rate: { basis: hr } }), ...(ftp !== null && { power: { basis: ftp } }) };
    },
    window: () => (data ? [trimIndex(data.time, startS, settings.trim_start_at, false), trimIndex(data.time, startS, settings.trim_end_at, true)] : [0, 0]),
  };
  /** The page's zones starter (startPage's seam) and, once started, the section's refresh. */
  let startZones: ((src: ZoneSource) => Promise<(() => void) | void>) | undefined;
  let zonesRefresh: Promise<(() => void) | void> | undefined;
  /** F-50 EF-1: the efforts section's refresh, per save; it recomputes when the saved window changed. */
  let effortsRefresh: Promise<(() => void) | void> | undefined;
  /** ZN-8: the chunk once a kind has zone values and its stream; afterwards a refresh per save. */
  const zones = (): void => {
    if (zonesRefresh) void zonesRefresh.then((refresh) => refresh?.());
    else if (startZones && data && Object.keys(zoneSource.zones()).some((k) => data.metrics[k as keyof Zones])) zonesRefresh = startZones(zoneSource);
  };

  /** Any owner result: 404 the generic page, an error its message, settings the new truth.
   *  True for settings. */
  const apply = (r: OwnerResult<Settings>): r is { ok: true; value: Settings } => {
    if (!r.ok) {
      if (r.notFound) gone();
      else ui.status = said(r);
      return false;
    }
    // The response is the truth (E-6): the panel refills from it, the header re-renders from it.
    settings = r.value;
    record(r.value);
    pinEditor?.saved(r.value.pins ?? []);
    zones();
    void effortsRefresh?.then((refresh) => refresh?.());
    visibility = r.value.visibility;
    const next = build(r.value);
    if (next && model) {
      model = { ...model, Page: next.Page, Header: next.Header, Trim: next.Trim };
      doc.title = next.Page.Title;
      // After Svelte's update: the re-render put the server's estimate back (F-13 D-7).
      setTimeout(() => enhanceExpiry(doc, Date.now()));
    }
    return true;
  };

  /** F-36 MC-8: the preview basemap job; never on the published page. */
  const basemap = basemapJobs(owner);

  /** F-56 PN-12: the pins go with every Save, when they changed. */
  const withPins = (patch: SettingsPatch): SettingsPatch => ({ ...patch, ...pinEditor?.patch(settings.pins ?? []) });
  const save = (panel: SettingsPatch): Promise<void> | void => {
    if (ui.busy) return;
    const patch = withPins(panel);
    if (Object.keys(patch).length === 0) {
      ui.status = t('noChanges');
      return;
    }
    return ui.run(t('saving'), async () => {
      if (!apply(await owner.save(patch))) return;
      ui.status = t('saved');
      // MC-8(b): hiding or trim changed on a published workout; in the background, nothing awaited or shown.
      if (('hide_start_m' in patch || 'hide_end_m' in patch || 'trim_start_at' in patch || 'trim_end_at' in patch) && visibility !== 'draft') void basemap();
    });
  };

  // C-3, OQ-85: the panel's unsaved changes first; no publish unless that save succeeded. On 200
  // the published page, with the busy state kept and nothing re-rendered. F-36 MC-8(a), OQ-112: the
  // basemap job starts after the save, so its frame reflects the saved hiding, and runs alongside
  // publish; the published page opens once both ended (the job: at most 10 s).
  const publish = (panel: SettingsPatch): Promise<void> =>
    ui.run(t('publishing'), async () => {
      const patch = withPins(panel);
      if (Object.keys(patch).length > 0 && !apply(await owner.save(patch))) return;
      const job = basemap();
      const r = await owner.publish();
      if (!r.ok) return void apply(r);
      record(r.value); // AC 46: Published, with the viewing link
      await job;
      owner.toPublished();
      return true;
    });

  const download = (): Promise<void> =>
    ui.run(t('downloading'), async () => {
      const r = await owner.original();
      if (!r.ok) {
        if (r.notFound) gone();
        else ui.status = said(r);
        return;
      }
      const url = URL.createObjectURL(r.value.blob); // blob: holds no token (C-6)
      const a = doc.createElement('a');
      a.href = url;
      a.download = r.value.name;
      doc.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(url), REVOKE_MS);
    });

  if (first) {
    doc.title = first.Page.Title;
    mount(ActivityPage, {
      target: host,
      props: {
        get model() {
          const m = model!;
          const s = strip(settings, data!.meta);
          // Built here, in the look's units: enhanceUnits leaves the edit page's strip alone.
          return { ...m, Summary: summaryItems(s.sport, s.summary, look?.units) };
        },
        report: false,
        edit: true,
        get copy() {
          return visibility !== 'draft'; // as Share (C-4)
        },
      },
    });
  } else {
    // The draft could not be decoded (E-5): the panel still works; D-13's text below it.
    host.innerHTML = '<div class="page"><div class="layout"><div id="edit-slot" class="edit-slot"></div><p class="load-error"></p></div></div>';
    host.querySelector('.load-error')!.textContent = loadErrorText(result.ok ? 'internal' : result.error.code);
  }
  const zone = data?.meta.time_zone;
  mount(EditPanel, {
    target: host.querySelector('#edit-slot')!,
    props: {
      get settings() {
        return settings;
      },
      get visibility() {
        return visibility;
      },
      get busy() {
        return ui.busy;
      },
      get status() {
        return ui.status;
      },
      // F-45 ST-7: follows the saved settings, so a save that changes the suggestion updates it.
      get placeholder() {
        return settings.suggested_title || (data && zone ? defaultTitle(data.meta.sport, data.meta.start_time, zone.known ? zone.offset_seconds : 0) : '');
      },
      now: Date.now(),
      origin: loc.origin,
      onsave: (patch: SettingsPatch) => void save(patch),
      onpublish: (patch: SettingsPatch) => void publish(patch),
      ondelete: () => void ui.remove(),
      ondownload: () => void download(),
      onlinks: () => owner.toPublished(),
      onhide,
      get units() {
        return look?.units ?? 'metric';
      },
      onedit: () => (ui.status = ''),
      time: data?.time ?? null,
      startTime: data?.meta.start_time ?? '',
      ontrim: (a: number, b: number) => {
        cut = [a, b];
        queueMicrotask(() => pinEditor?.refresh()); // outside the panel's fill effect, which writes cut
      },
      zones: data?.meta.zones ?? {},
      gpx: !!data && data.meta.sport !== 'multisport' && routePresence(data) === 'route',
    },
  });
  // C-2: a back-forward cache restore (also an app resume on mobile WebKit) never reloads, which
  // would drop unsaved changes. A draft published meanwhile (Back from the published page) shows
  // as published; the form keeps what the owner typed.
  win.addEventListener('pageshow', (e) => {
    if (!e.persisted || visibility !== 'draft') return;
    void owner.settings().then((r) => {
      if (r.ok) record(r.value);
      if (r.ok && r.value.visibility !== 'draft') {
        visibility = r.value.visibility;
        ui.release();
      }
    });
  });
  /** E-9's marks from the sliders (F-55 HR-8) plus the trimmed parts (F-47 TR-9), greyed alike on
   *  the map and the charts: neither will be public. */
  const hidden = (): Ranges => {
    const [a, b] = cut;
    const marks = hidingRanges(settings.hiding_steps, ends[0], ends[1], last);
    return [...(a > 0 ? [[0, a] as const] : []), ...(b < last ? [[b, last] as const] : []), ...marks];
  };
  if (result.ok) {
    await startPage(doc, win, {
      data: result,
      get hidden() {
        return hidden();
      },
      onzones: (start) => {
        startZones = start;
        zones();
      },
      onefforts: (start) => (effortsRefresh = start(zoneSource.window)),
      // F-56 PN-12: the editor over the pins view; the shading as the map's and charts' (E-9).
      onpins: (view) => {
        const steps = (): Settings['hiding_steps'] => settings.hiding_steps;
        pinEditor = startPinEditor(
          {
            slot: view.slot,
            view,
            data: result.data,
            startS,
            shade: () => {
              const st = steps();
              return { a: cut[0], b: cut[1], from: st ? st.start[ends[0] / HIDE_STEP_M] : 0, to: st ? st.end[ends[1] / HIDE_STEP_M] : last };
            },
            onedit: () => (ui.status = ''),
          },
          settings.pins ?? [],
        );
      },
      // F-58 SE-4: the saved settings' facts; sessions, lengths and zones as the rest of the page.
      explain: () => {
        const s = strip(settings, result.data.meta);
        const meta = result.data.meta;
        return {
          sport: s.sport,
          summary: s.summary,
          sessions: meta.sessions,
          lengths: meta.lengths,
          zones: Object.keys(zoneSource.zones()).some((k) => result.data.metrics[k as keyof Zones]),
          owner: { radii: radii(settings), trimAt: [settings.trim_start_at, settings.trim_end_at], zone: meta.time_zone, untrimmed: meta.summary },
        };
      },
      onlook: (l: Look) => {
        look = l;
        // The chunks' decorators (legs, pins) and the shading; the charts get all (MS-7 has one seam).
        const chunks = Object.getOwnPropertyDescriptor(l, 'decorate')!.get!;
        Object.defineProperty(l, 'decorate', {
          get: (): ChartDecorator => {
            const t = shade(hidden());
            const g: ChartDecorator | undefined = chunks();
            return g ? (over, x) => (g(over, x), t(over, x)) : t;
          },
        });
      },
    });
  }
}

/** The published page (C-5): the settings only; a draft goes to the edit page with replace. */
async function startPublished(host: HTMLElement, owner: OwnerClient, origin: string, mineList: ReturnType<typeof list>, gone: () => void): Promise<void> {
  const doc = host.ownerDocument;
  const got = await owner.settings();
  if (!got.ok) return got.notFound ? gone() : showMessage(host, said(got));
  if (got.value.visibility === 'draft') return owner.toEdit(true);
  mineList.update(got.value);
  const ui = actions(doc, owner, mineList);
  doc.title = 'Workout Explorer';
  host.replaceChildren();
  mount(PublishedView, {
    target: host,
    props: {
      origin,
      viewUrl: got.value.view_url,
      editLink: owner.editLink!,
      get busy() {
        return ui.busy;
      },
      get status() {
        return ui.status;
      },
      onback: () => owner.toEdit(false),
      ondelete: () => void ui.remove(),
    },
  });
}
