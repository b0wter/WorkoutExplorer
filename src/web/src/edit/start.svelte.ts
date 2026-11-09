// The owner pages' glue (F-19 design E-1, E-4 to E-6, E-9; F-20 C-1 to C-8). LAZY: reached only
// through main.ts's import() on edit.html, which serves /edit/* and /published/*. The edit page
// loads settings and draft.bin through the owner client in parallel, mounts the D-9 root and the
// panel into its slot, then runs the viewing page's glue on it with the decoded draft (one glue,
// F-13 D-19). The published page loads the settings only. Never sees the edit token: lib/owner
// holds it (the published view gets only lib/owner's edit link).
// Only mount() from svelte: flushSync, unmount and the like would join the shared activity entry.

import { mount } from 'svelte';
import './edit.css';
import ActivityPage from '../components/page/ActivityPage.svelte';
import EditPanel from '../components/edit/EditPanel.svelte';
import PublishedView from '../components/edit/PublishedView.svelte';
import { loadDataFile } from '../lib/datafile';
import { openOwner, type OwnerClient, type OwnerLocation, type OwnerResult, type Settings, type SettingsPatch } from '../lib/owner';
import { buildPageViewModel, defaultTitle } from '../lib/page/view';
import { enhanceExpiry } from '../page/enhance';
import { loadErrorText } from '../page/load-error';
import { startPage } from '../page/start';
import { basemapJobs } from './basemap-job';

const NONE: readonly (readonly [number, number])[] = [];

/** The generic page's content (generic.html, E-4), in this page's theme. No workout data. */
export function showGeneric(doc: Document, text = 'This workout has expired or is not available.', upload = ''): void {
  doc.title = 'Workout Explorer';
  doc.body.innerHTML = `<main class="generic"><h1>Workout Explorer</h1><p>${text}</p>${upload}</main><footer class="foot generic-foot"><a href="/about">About</a></footer>`;
}

/** The deleted screen (F-20 C-8): E-4's teardown with a fixed sentence and the upload link. */
const showDeleted = (doc: Document): void => showGeneric(doc, 'This workout has been deleted.', '<p><a href="/">Upload a workout</a></p>');

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

/** One owner action at a time (C-2), shared by both pages; Delete (C-7, C-8). */
function actions(doc: Document, owner: OwnerClient) {
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
      ui.run('Deleting…', async () => {
        const r = await owner.delete();
        if (r.ok) showDeleted(doc);
        else if (r.notFound) showGeneric(doc);
        else status = r.message; // O-10: it may still exist; trying again is safe
      }),
  };
  return ui;
}

/** Never rejects. `loc` is the window's location (a test passes its own, to record navigations). */
export async function startEdit(host: HTMLElement, win: Window = window, loc: OwnerLocation = win.location): Promise<void> {
  const doc = host.ownerDocument;
  const owner = openOwner(loc);
  if (!owner) return showGeneric(doc);
  if (owner.kind === 'published') return startPublished(host, owner, loc.origin);
  const [got, result] = await Promise.all([owner.settings(), loadDataFile(owner.draft)]);
  if (!got.ok && got.notFound) return showGeneric(doc);
  if (!result.ok && result.error.code === 'http-status' && result.error.status === 404) return showGeneric(doc);
  if (!got.ok) return showMessage(host, got.message);
  const data = result.ok ? result.data : null;

  let settings = $state.raw(got.value);
  const ui = actions(doc, owner);
  let hide = $state(got.value.hide_ends);
  /** Apart from settings: a restore updates it without refilling the form (C-2). */
  let visibility = $state(got.value.visibility);
  const build = (s: Settings) =>
    data && buildPageViewModel(data, { title: s.title, description: s.description, expiresAt: s.expires_at, publicId: owner.publicId, dataFile: '' });
  const first = build(got.value);
  let model = $state.raw(first);

  /** Any owner result: 404 the generic page, an error its message, settings the new truth.
   *  True for settings. */
  const apply = (r: OwnerResult<Settings>): r is { ok: true; value: Settings } => {
    if (!r.ok) {
      if (r.notFound) showGeneric(doc);
      else ui.status = r.message;
      return false;
    }
    // The response is the truth (E-6): the panel refills from it, the header re-renders from it.
    settings = r.value;
    hide = r.value.hide_ends;
    visibility = r.value.visibility;
    const next = build(r.value);
    if (next && model) {
      model = { ...model, Page: next.Page, Header: next.Header };
      doc.title = next.Page.Title;
      // After Svelte's update: the re-render put the server's estimate back (F-13 D-7).
      setTimeout(() => enhanceExpiry(doc, Date.now()));
    }
    return true;
  };

  /** F-36 MC-8: the preview basemap job; never on the published page. */
  const basemap = basemapJobs(owner);

  const save = (patch: SettingsPatch): Promise<void> | void => {
    if (ui.busy) return;
    if (Object.keys(patch).length === 0) {
      ui.status = 'No changes';
      return;
    }
    return ui.run('Saving…', async () => {
      if (!apply(await owner.save(patch))) return;
      ui.status = 'Saved';
      // MC-8(b): hiding changed on a published workout; in the background, nothing awaited or shown.
      if ('hide_ends' in patch && visibility !== 'draft') void basemap();
    });
  };

  // C-3, OQ-85: the panel's unsaved changes first; no publish unless that save succeeded. On 200
  // the published page, with the busy state kept and nothing re-rendered. F-36 MC-8(a), OQ-112: the
  // basemap job starts after the save, so its frame reflects the saved hiding, and runs alongside
  // publish; the published page opens once both ended (the job: at most 10 s).
  const publish = (patch: SettingsPatch): Promise<void> =>
    ui.run('Publishing…', async () => {
      if (Object.keys(patch).length > 0 && !apply(await owner.save(patch))) return;
      const job = basemap();
      const r = await owner.publish();
      if (!r.ok) return void apply(r);
      await job;
      owner.toPublished();
      return true;
    });

  const download = (): Promise<void> =>
    ui.run('Downloading…', async () => {
      const r = await owner.original();
      if (!r.ok) {
        if (r.notFound) showGeneric(doc);
        else ui.status = r.message;
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
          return model!;
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
      placeholder: data && zone ? defaultTitle(data.meta.sport, data.meta.start_time, zone.known ? zone.offset_seconds : 0) : '',
      now: Date.now(),
      origin: loc.origin,
      onsave: (patch: SettingsPatch) => void save(patch),
      onpublish: (patch: SettingsPatch) => void publish(patch),
      ondelete: () => void ui.remove(),
      ondownload: () => void download(),
      onlinks: () => owner.toPublished(),
      onhide: (on: boolean) => (hide = on),
      onedit: () => (ui.status = ''),
    },
  });
  // C-2: a back-forward cache restore (also an app resume on mobile WebKit) never reloads, which
  // would drop unsaved changes. A draft published meanwhile (Back from the published page) shows
  // as published; the form keeps what the owner typed.
  win.addEventListener('pageshow', (e) => {
    if (!e.persisted || visibility !== 'draft') return;
    void owner.settings().then((r) => {
      if (r.ok && r.value.visibility !== 'draft') {
        visibility = r.value.visibility;
        ui.release();
      }
    });
  });
  if (result.ok) {
    await startPage(doc, win, {
      data: result,
      get hidden() {
        return hide ? settings.hide_ends_ranges : NONE;
      },
    });
  }
}

/** The published page (C-5): the settings only; a draft goes to the edit page with replace. */
async function startPublished(host: HTMLElement, owner: OwnerClient, origin: string): Promise<void> {
  const doc = host.ownerDocument;
  const got = await owner.settings();
  if (!got.ok) return got.notFound ? showGeneric(doc) : showMessage(host, got.message);
  if (got.value.visibility === 'draft') return owner.toEdit(true);
  const ui = actions(doc, owner);
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
