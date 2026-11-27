// @vitest-environment jsdom
// The edit page, part 2: a split of the former edit.dom.test.ts so Vitest runs the parts in parallel; shared setup in testing/edit-dom.ts.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { $, EDIT_LINK, FIT, GENERIC, ID, N, ON, RANGES, RIDE, TOKEN, VIEW_LINK, WARNING, btn, click, clipboard, expectNoTokenOutsideTheFragment, field, fresh, golden, hideAt, methods, open, openPublished, patches, s, save, settle, shows, type } from './testing/edit-dom';
import { stubLog, resetStubLog } from '../page/testing/stub-log';
import type { Settings } from '../lib/owner';
import { decodeDataFile } from '../lib/datafile';
import { formatElapsed } from '../lib/format';
import editHtml from '../../edit.html?raw';

describe('F-20 C-5, AC 5, 6: the published page', () => {
  beforeEach(() => {
    s.settings = fresh({ visibility: 'unlisted', title: 'Secret loop', description: 'Through the park' });
  });

  it('the settings only; both links, the warning, Go there in each box and Delete; no workout data', async () => {
    await openPublished();
    expect(s.calls.map((c) => c.url)).toEqual(['/api/workouts/' + ID]);
    expect(stubLog.calls).toEqual([]);
    expect(document.title).toBe('Workout Explorer');
    expect($('main.published h1')?.textContent).toBe('Workout published');
    expect($('main.published .edit-share .edit-link')?.textContent).toBe(VIEW_LINK());
    expect($('main.published [translate="no"]')?.textContent).toBe(EDIT_LINK());
    // Two boxes, blue then red: each with its header, its link and its Copy.
    const boxes = [...document.querySelectorAll('main.published .link-box')];
    expect(boxes.map((b) => b.querySelector('h2')?.textContent)).toEqual(['Viewing link', 'Edit link']);
    expect(boxes[0].classList.contains('view-box')).toBe(true);
    expect(boxes[0].querySelector('a[data-copy-link]')).not.toBeNull();
    const go = boxes[0].querySelector<HTMLAnchorElement>('a.go');
    expect([go?.textContent, go?.getAttribute('href')]).toEqual(['Go there', '/w/' + ID]);
    expect(boxes[1].classList.contains('edit-warn')).toBe(true);
    expect(boxes[1].querySelector('[translate="no"]')?.textContent).toBe(EDIT_LINK());
    expect(boxes[1].querySelector('[data-act="copy-edit"]')).not.toBeNull();
    expect(boxes[1].textContent).toContain(WARNING);
    expect($('main.published')?.textContent).toContain(WARNING);
    const back = boxes[1].querySelector('[data-act="back"]');
    expect([back?.tagName, back?.textContent]).toEqual(['BUTTON', 'Go there']);
    expect($('main.published')?.textContent).not.toContain('Back to edit');
    expect(btn('delete')).not.toBeNull();
    expect($('main.published .edit-delete-hint + .edit-row')?.previousElementSibling?.textContent).toBe('Uploaded accidentally?');
    expect($('footer a[href="/about"]')).not.toBeNull();
    const html = document.documentElement.outerHTML;
    for (const gone of ['Secret loop', 'Through the park']) expect(html).not.toContain(gone);
    expect($('.edit-panel, .summary, #map-slot, #charts-slot, table, header')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });

  it('copy puts exactly each link on the clipboard', async () => {
    const writeText = clipboard();
    await openPublished();
    $<HTMLAnchorElement>('main.published a[data-copy-link]')!.click();
    await settle();
    await click('copy-edit');
    expect(writeText.mock.calls).toEqual([[VIEW_LINK()], [EDIT_LINK()]]);
    expect($('.edit-copied')?.textContent).toBe('Edit link copied.');
    expectNoTokenOutsideTheFragment();
  });

  it('a refused clipboard: select-by-hand text for the edit link, the viewing link in D-14\'s field', async () => {
    clipboard(() => Promise.reject(new Error('denied')));
    await openPublished();
    await click('copy-edit');
    expect($('.edit-copied')?.textContent).toBe('Copying did not work. Select the link above and copy it by hand.');
    $<HTMLAnchorElement>('main.published a[data-copy-link]')!.click();
    await settle();
    expect([...document.querySelectorAll<HTMLInputElement>('input')].map((i) => i.value)).toEqual([VIEW_LINK()]);
    expectNoTokenOutsideTheFragment();
  });

  it('the edit link\'s Go there: assign to the edit link', async () => {
    await openPublished();
    await click('back');
    expect(s.navs).toEqual(['assign ' + EDIT_LINK()]);
  });

  it('a reload shows it again', async () => {
    await openPublished();
    await openPublished();
    expect($('main.published [translate="no"]')?.textContent).toBe(EDIT_LINK());
  });

  it('a draft: replace to the edit page (Back does not bounce), nothing shown', async () => {
    s.settings = fresh();
    await openPublished();
    expect(s.navs).toEqual(['replace ' + EDIT_LINK()]);
    expect($('main.published')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });

  it.each([
    ['#' + TOKEN.slice(0, 21) + 'x0', '/published/' + ID],
    ['', '/published/' + ID],
    ['#' + TOKEN, '/published/' + ID.slice(2)],
  ])('a malformed link (%s on %s): the generic page and no request', async (hash, path) => {
    await open(hash, path);
    expect(s.calls).toEqual([]);
    expect(document.body.textContent).toContain(GENERIC);
  });

  it('a 404: the generic page; a 500: the message', async () => {
    s.onGet = () => new Response('', { status: 404 });
    await openPublished();
    expect(document.body.textContent).toContain(GENERIC);
    expectNoTokenOutsideTheFragment();
    s.host = document.createElement('div');
    document.body.replaceChildren(s.host);
    s.onGet = () => new Response('', { status: 500 });
    await openPublished();
    expect(document.body.textContent).toContain('Something went wrong on the server.');
    expect(document.body.textContent).not.toContain(GENERIC);
    expect($('main.published')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });
});

describe.each([
  ['edit', () => open()],
  ['published', () => openPublished()],
])('F-20 C-7, C-8, AC 9: delete on the %s page', (_, go) => {
  beforeEach(() => {
    s.settings = fresh({ visibility: 'unlisted', title: 'Secret loop', description: 'Through the park' });
  });

  it('Delete only asks: the confirmation is announced and Cancel has the focus; Cancel sends nothing', async () => {
    await go();
    await click('delete');
    expect(methods()).toEqual([]);
    expect($('.edit-confirm')?.getAttribute('role')).toBe('alert');
    expect($('.edit-confirm')?.textContent).toContain('permanently');
    expect($('.edit-confirm')?.textContent).toContain('Both the viewing link and the edit link stop working');
    expect(document.activeElement).toBe(btn('cancel'));
    await click('cancel');
    expect($('.edit-confirm')).toBeNull();
    expect(methods()).toEqual([]);
    expect(btn('delete')).not.toBeNull();
  });

  it('confirm: DELETE, then the deleted screen with no workout data', async () => {
    await go();
    await click('delete');
    await click('confirm');
    expect(methods()).toEqual(['DELETE ']);
    expect(document.title).toBe('Workout Explorer');
    expect($('main.generic p')?.textContent).toBe('This workout has been deleted.');
    expect($<HTMLAnchorElement>('main.generic a')?.getAttribute('href')).toBe('/');
    expect($('footer a[href="/about"]')).not.toBeNull();
    const html = document.documentElement.outerHTML;
    for (const gone of [ID, TOKEN, 'Secret loop', 'Through the park', '/w/']) expect(html).not.toContain(gone);
    expect(document.body.textContent).not.toMatch(/\d/);
    expect($('.edit-panel, main.published, .summary, #map-slot, #charts-slot, table')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });

  it('a 404 on delete: the generic page', async () => {
    s.onDelete = () => new Response('', { status: 404 });
    await go();
    await click('delete');
    await click('confirm');
    expect(document.body.textContent).toContain(GENERIC);
  });

  it('a 500 on delete: the message; the page stays and a retry is possible', async () => {
    s.onDelete = () => new Response('', { status: 500 });
    await go();
    await click('delete');
    await click('confirm');
    expect($('.edit-status')?.textContent).toBe('Something went wrong on the server. Please try again later.');
    expect(document.body.textContent).not.toContain(GENERIC);
    expect(btn('confirm')?.disabled).toBe(false);
  });
});

describe('F-20 C-6, AC 10: download original', () => {
  const BLOB = 'blob:' + window.location.origin + '/0d1e';
  let anchors: HTMLAnchorElement[];
  let blobs: Blob[];
  beforeEach(() => {
    anchors = [];
    blobs = [];
    Object.assign(URL, {
      createObjectURL: vi.fn((b: Blob) => (blobs.push(b), BLOB)),
      revokeObjectURL: vi.fn(),
    });
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      anchors.push(this.cloneNode() as HTMLAnchorElement);
      expect(this.isConnected).toBe(true);
    });
  });

  it('offered draft and published; the bytes as served, the server\'s name, exactly the blob: URL', async () => {
    for (const visibility of ['unlisted', 'draft'] as const) {
      s.settings = fresh({ visibility });
      await open();
      expect(btn('download'), visibility).not.toBeNull();
    }
    const timers = vi.spyOn(globalThis, 'setTimeout');
    await click('download');
    const revoke = timers.mock.calls.find((c) => c[1] === 40_000);
    expect(URL.revokeObjectURL).not.toHaveBeenCalled(); // not at once: Safari would drop the file
    (revoke![0] as () => void)();
    expect(s.calls.at(-1)?.url).toBe('/api/workouts/' + ID + '/original.fit');
    expect(anchors).toHaveLength(1);
    expect(anchors[0].getAttribute('href')).toBe(BLOB); // nothing added, so no token (QA-20-r1-2 b)
    expect(anchors[0].download).toBe('workout-2026-10-29.fit');
    expect(new Uint8Array(await blobs[0].arrayBuffer())).toEqual(FIT);
    expect($('a[download]')).toBeNull(); // removed again
    expect(URL.revokeObjectURL).toHaveBeenCalledWith(BLOB);
    expectNoTokenOutsideTheFragment();
  });

  it('C-2: a running download disables Save, Publish and Delete (QA-20-r1-2 c)', async () => {
    let release: () => void = () => {};
    s.original = () => new Promise<Response>((r) => (release = () => r(new Response(FIT)))) as unknown as Response;
    await open();
    await click('download');
    for (const b of [$<HTMLButtonElement>('.edit-save'), btn('publish'), btn('delete'), btn('download')]) expect(b?.disabled).toBe(true);
    expect($('.edit-status')?.textContent).toBe('Downloading…');
    btn('publish')!.dispatchEvent(new MouseEvent('click', { bubbles: true })); // forced
    await settle();
    expect(methods()).toEqual([]);
    release();
    await settle();
    expect(btn('publish')?.disabled).toBe(false);
    expect(anchors[0].getAttribute('href')).toBe(BLOB);
  });

  it('no Content-Disposition: workout.fit', async () => {
    s.original = () => new Response(FIT);
    await open();
    await click('download');
    expect(anchors[0].download).toBe('workout.fit');
  });

  it('404: the generic page; 500: the message', async () => {
    s.original = () => new Response('', { status: 500 });
    await open();
    await click('download');
    expect($('.edit-status')?.textContent).toBe('Something went wrong on the server. Please try again later.');
    expect(anchors).toEqual([]);
    s.original = () => new Response('', { status: 404 });
    await click('download');
    expect(document.body.textContent).toContain(GENERIC);
  });
});

describe('F-38 IM-7: the Impressum link in the browser-drawn footers', () => {
  const LINK = '<a href="/impressum">Impressum</a>';
  const withTemplate = (filled: string): void => {
    const t = document.createElement('template');
    t.id = 'wx-impressum';
    t.innerHTML = filled;
    document.body.prepend(t);
  };
  const links = (): string[] => [...document.querySelectorAll('footer a:not(.gh)')].map((a) => a.getAttribute('href')!);
  it('the page and published footers link the repository', async () => {
    await open();
    expect(document.querySelector('footer a.gh')?.getAttribute('href')).toBe('https://github.com/b0wter/WorkoutExplorer');
  });

  for (const [name, filled, want] of [
    ['set', LINK, ['/about', '/impressum']],
    ['unset', '', ['/about']],
    ['the unfilled dev marker', '<!--wx:impressum-->', ['/about']],
  ] as const) {
    describe(name, () => {
      beforeEach(() => withTemplate(filled));

      it('the page footer', async () => {
        await open();
        expect(links()).toEqual(want);
        expect(document.querySelectorAll('footer')).toHaveLength(1);
      });

      it('the published view', async () => {
        s.settings = fresh({ visibility: 'unlisted' });
        await openPublished();
        expect($('main.published')).not.toBeNull();
        expect(links()).toEqual(want);
      });

      it('the not-found view, also when drawn twice (delete, then the deleted screen)', async () => {
        await open('');
        expect($('main.generic')).not.toBeNull();
        expect(links()).toEqual(want);
        s.settings = fresh({ visibility: 'unlisted' });
        s.host = document.createElement('div');
        document.body.append(s.host);
        await openPublished();
        await click('delete');
        await click('confirm');
        expect($('main.generic p')?.textContent).toBe('This workout has been deleted.');
        expect(links()).toEqual(want);
      });
    });
  }

  it('the edit.html template holds exactly the marker, before #wx-edit', () => {
    expect(editHtml).toContain('<template id="wx-impressum"><!--wx:impressum--></template>\n    <div id="wx-edit"></div>');
  });
});

describe('the token stays in lib/owner (E-2 source scan)', () => {
  const sources = import.meta.glob(['../**/*.ts', '../**/*.svelte', '../../*.html', '!../**/*.test.ts', '!../**/testing/**', '!../**/*.test-helper.*'], {
    query: '?raw',
    import: 'default',
    eager: true,
  }) as Record<string, string>;
  const code = (text: string): string => text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/.*$/gm, '$1');

  it('only lib/owner reads the fragment, builds Authorization, changes history or navigates (F-20 C-1)', () => {
    expect(Object.keys(sources).length).toBeGreaterThan(40);
    for (const [file, text] of Object.entries(sources)) {
      if (file === '../lib/owner/index.ts') continue;
      // The upload page turns the edit link the API returns into the address it opens (F-11 U-9):
      // the token's way into the fragment, before any edit page code runs.
      if (file === '../lib/upload/rules.ts') continue;
      // F-37 DK-11: the delete link's page reads its own key and sends it as Bearer; it never touches history.
      if (file.startsWith('../report-delete/')) {
        expect(code(text), file).not.toMatch(/replaceState|pushState|location\.(assign|replace|href)|console\.|localStorage|sessionStorage|innerHTML/);
        continue;
      }
      // F-26 (ADR-038 item 3): the fullscreen map's one history entry, with no URL argument.
      let c = file === '../components/map/map-view.ts' ? code(text).replace("history.pushState(null, '')", '') : code(text);
      // F-40 SR-4, SR-5: the viewing page (no edit seam) reads its range link once and drops it
      // without a history entry; the edit seam reads nothing.
      if (file === '../page/start.ts') c = c.replace("edit ? '' : win.location.hash", '').replace("win.history.replaceState(win.history.state, '', win.location.pathname + win.location.search)", '');
      expect(c, file).not.toMatch(/\.hash\b|Authorization|Bearer|replaceState|pushState|location\.href/);
      // The upload page opens the edit link it checked (F-11 U-9).
      if (file !== '../upload/main.ts') expect(c, file).not.toMatch(/location\.(assign|replace)\b/);
    }
    const owner = sources['../lib/owner/index.ts'];
    expect(owner).toContain("Authorization: 'Bearer ' + token");
    expect(owner).toContain("redirect: 'error'");
    expect(code(owner)).not.toMatch(/console\.|localStorage|sessionStorage|indexedDB|cookie|caches\.|location\.href|innerHTML|textContent/);
    expect(owner).not.toMatch(/^import .* from '\./m); // imports nothing of ours
  });

  it('the edit page has the head of the template, no inline script but the theme script, no inline style, no data preload', () => {
    const edit = sources['../../edit.html'].replace(/<!--[\s\S]*?-->/g, '');
    expect(edit).toContain('<meta name="referrer" content="no-referrer"');
    expect(edit).toContain('<meta name="robots" content="noindex, nofollow, noarchive"');
    expect(edit).toContain('<script type="module" src="/src/main.ts"></script>');
    expect(edit).toContain('<div id="wx-edit"></div>');
    expect(edit).not.toMatch(/<style[\s>]|\sstyle\s*=|\son[a-z]+\s*=|preload/i);
    expect(edit.match(/<script(?![^>]*\ssrc=)[^>]*>/gi)).toEqual(['<script>']); // DS-4, theme-script.test.ts
    const sprite = (html: string): string => html.slice(html.indexOf('<svg class="sprite"'), html.indexOf('</defs></svg>'));
    expect(sprite(edit)).toBe(sprite(sources['../../index.html'])); // the copy of the template's icons
    expect(sprite(edit)).toContain('sport-generic');
  });
});

describe('F-47 TR-9: the trim slider (AC 31, 32, 41)', () => {
  const decoded = decodeDataFile(RIDE.slice(0));
  if (!decoded.ok) throw new Error('ride-4h does not decode');
  const T = decoded.data.time;
  const S = Date.parse(decoded.data.meta.start_time) / 1000;
  const at = (i: number): string => new Date((S + T[i]) * 1000).toISOString().slice(0, 19) + 'Z';
  const drag = (name: 'trim_a' | 'trim_b', i: number): void => type(name, String(i));
  const handles = (): [number, number] => [Number(field('trim_a').value), Number(field('trim_b').value)];
  /** What the charts' decoration draws into a fresh overlay: [left, width] per shaded part. */
  const shaded = (): number[][] => {
    const over = document.createElement('div');
    (stubLog.chartStack[0].decorate as (o: HTMLElement, x: (i: number) => number) => void)(over, (i) => i);
    return [...over.querySelectorAll<HTMLElement>('.wx-trim > div')].map((d) => [parseFloat(d.style.left), parseFloat(d.style.width)]);
  };
  const keys = (): string[] => [...document.querySelectorAll('.summary > .metric')].map((m) => m.getAttribute('data-k')!);

  it('the handles shade the charts and grey the map live; Save sends sample instants, null at the ends', async () => {
    await open();
    expect(handles()).toEqual([0, N - 1]);
    expect(shaded()).toEqual([]);
    drag('trim_a', 100);
    drag('trim_b', N - 200);
    expect(shaded()).toEqual([
      [0, 100],
      [N - 200, 199],
    ]);
    expect(stubLog.routeMap[0].hidden).toEqual([
      [0, 100],
      [N - 200, N - 1],
    ]);
    expect($('.edit-panel')?.textContent).toContain(formatElapsed(T[100] - T[0]) + ' – ' + formatElapsed(T[N - 200] - T[0])); // elapsed from the recording start
    await save();
    expect(patches()).toEqual([{ trim_start_at: at(100), trim_end_at: at(N - 200) }]);
    expect(handles()).toEqual([100, N - 200]); // refilled from the response
    drag('trim_a', 0);
    drag('trim_b', N - 1);
    expect(stubLog.routeMap[0].hidden).toEqual([]);
    await save();
    expect(patches()[1]).toEqual({ trim_start_at: null, trim_end_at: null });
    expectNoTokenOutsideTheFragment();
  });

  it('with hiding on, the trim marks come first, then the stored hidden ranges', async () => {
    s.settings = fresh({ ...ON });
    await open();
    drag('trim_b', N - 500);
    expect(stubLog.routeMap[0].hidden).toEqual([[N - 500, N - 1], ...RANGES]);
  });

  it('QA-55-r1-3: an unsaved trim with hiding on says the shading waits for Save', async () => {
    s.settings = fresh({ ...ON, trim_start_at: at(100), trim_end_at: at(N - 300) });
    await open();
    const note = () => $('.edit-stale')?.textContent;
    expect(note()).toBeUndefined();
    drag('trim_a', 0);
    expect(note()).toBe('Save to update the hidden sections.');
    hideAt(0, 0);
    expect(note()).toBeUndefined(); // nothing hidden, nothing stale
    hideAt(300, 0);
    expect(note()).toBe('Save to update the hidden sections.');
    drag('trim_a', 100);
    expect(note()).toBeUndefined(); // back on the saved trim
  });

  it('at least 60 s remain (AC 41): the moved handle is held back', async () => {
    await open();
    drag('trim_a', 1000);
    drag('trim_b', 1000);
    const [a, b] = handles();
    expect(a).toBe(1000);
    expect(T[b] - T[a]).toBeGreaterThanOrEqual(60);
    expect(T[b - 1] - T[a]).toBeLessThan(60);
  });

  it('F-51 AC 8: with a stored trim the comparison chart shows the same full rows as the edit page table', async () => {
    s.settings = fresh({ trim_start_at: at(50), trim_end_at: at(N - 300), trimmed: { sport: 'cycling', summary: { distance: 12_345, elapsed_time: 600 } } });
    await open();
    stubLog.chartStack[0].onready?.(); // the charts' first paint starts the lazy chunks
    await vi.waitFor(() => expect($('section.laps .compare rect.bar')).not.toBeNull());
    const rows = [...s.host.querySelectorAll('section.laps .laps-scroll:not(.splits) tbody tr')];
    expect(rows.length).toBeGreaterThan(1);
    expect(s.host.querySelectorAll('section.laps .compare rect.bar')).toHaveLength(rows.length);
  });

  it('a stored trim: handles from it, only the summary trimmed, in the chosen units; header and laps stay full', async () => {
    const full = fresh();
    s.settings = full;
    await open();
    const head = $('h1')?.textContent;
    const laps = $('section.laps')?.textContent;
    const fullKeys = keys();
    document.body.replaceChildren(s.host);
    s.host.replaceChildren();
    resetStubLog();
    s.settings = fresh({ trim_start_at: at(50), trim_end_at: at(N - 300), trimmed: { sport: 'cycling', summary: { distance: 12_345, elapsed_time: 600 } } });
    await open();
    expect(handles()).toEqual([50, N - 300]);
    expect(stubLog.routeMap[0].hidden).toEqual([
      [0, 50],
      [N - 300, N - 1],
    ]);
    expect(keys()).toEqual(['distance', 'elapsed_time']);
    expect(fullKeys).not.toEqual(keys());
    expect($('h1')?.textContent).toBe(head);
    expect($('section.laps')?.textContent).toBe(laps);
    const units = $<HTMLSelectElement>('#units-slot select')!;
    for (const [u, n, unit] of [['imperial', '7.67', 'mi'], ['metric', '12.35', 'km'], ['imperial', '7.67', 'mi']]) {
      units.value = u;
      units.dispatchEvent(new Event('change', { bubbles: true }));
      flushSync();
      expect($('.summary [data-k="distance"] .n')?.textContent).toBe(n);
      expect($('.summary [data-k="distance"] .u')?.textContent).toBe(unit);
      expect($('section.laps thead')?.textContent).toContain('Distance (' + unit + ')'); // the laps still follow the units
    }
    expect(keys()).toEqual(['distance', 'elapsed_time']);
  });

  it('a stored trim whose trimmed values failed (null): no summary values, never the full totals', async () => {
    s.settings = fresh({ trim_end_at: at(N - 300), trimmed: null });
    await open();
    expect(keys()).toEqual([]);
  });

  it('the save response replaces the strip: trimmed values after a trim save, full ones after removing it', async () => {
    await open();
    const fullKeys = keys();
    s.onPatch = (body) => Response.json((s.settings = { ...s.settings, ...body, trimmed: body.trim_start_at ? { sport: 'cycling', summary: { elapsed_time: 900 } } : null } as Settings));
    drag('trim_a', 300);
    await save();
    expect(keys()).toEqual(['elapsed_time']);
    drag('trim_a', 0);
    await save();
    expect(keys()).toEqual(fullKeys);
  });

  it('multisport: the leg lines and the trim shading are both drawn', async () => {
    s.draft = () => new Response(golden('triathlon').slice(0), { status: 200 });
    s.settings = fresh({ hiding_steps: null });
    await open();
    const charts = stubLog.chartStack[0];
    charts.onready?.();
    await vi.waitFor(() => {
      const over = document.createElement('div');
      (charts.decorate as (o: HTMLElement, x: (i: number) => number) => void)(over, (i) => i);
      expect(over.querySelector('.wx-legs')).not.toBeNull();
    });
    drag('trim_a', 10);
    const over = document.createElement('div');
    (charts.decorate as (o: HTMLElement, x: (i: number) => number) => void)(over, (i) => i);
    expect([!!over.querySelector('.wx-legs'), over.querySelectorAll('.wx-trim > div').length]).toEqual([true, 1]);
  });

  it('indoor: the slider works without a map; no error', async () => {
    s.draft = () => new Response(golden('indoor-no-gps').slice(0), { status: 200 });
    s.settings = fresh({ hiding_steps: null });
    await open();
    drag('trim_a', 50);
    await save();
    expect(Object.keys(patches()[0])).toEqual(['trim_start_at']);
    expect(stubLog.routeMap).toEqual([]);
    expect(s.consoleOut.filter((a) => a.some((x) => x instanceof Error))).toEqual([]);
  });

  it('an undecodable draft: no slider, the stored trim goes back unchanged', async () => {
    s.draft = () => new Response(new Uint8Array([1, 2, 3]), { status: 200 });
    s.settings = fresh({ trim_start_at: '2026-10-29T10:00:00Z' });
    await open();
    expect(field('trim_a')).toBeNull();
    type('title', 'x');
    await save();
    expect(patches()).toEqual([{ title: 'x' }]);
  });

  it('MC-8(b): a trim save on a published workout runs the basemap job; not on a draft', async () => {
    s.onFrame = () => Response.json({ frame: { lon: 11.5, lat: 48.1, zoom: 12 }, key: '0123456789abcdef', current: false });
    const frameGets = () => s.calls.filter((c) => !c.init.method && c.url.endsWith('/preview-map')).length;
    await open();
    drag('trim_a', 100);
    await save();
    expect(frameGets()).toBe(0);
    document.body.replaceChildren(s.host);
    s.host.replaceChildren();
    resetStubLog();
    s.settings = fresh({ visibility: 'unlisted' });
    await open();
    drag('trim_b', N - 100);
    await save();
    expect(frameGets()).toBe(1);
  });
});

