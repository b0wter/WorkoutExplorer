// @vitest-environment jsdom
// The edit page, part 1: a split of the former edit.dom.test.ts so Vitest runs the parts in parallel; shared setup in testing/edit-dom.ts.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { $, DRAFT_LINE, FIELDS, GENERIC, ID, N, ON, PUBLISHED_PAGE, RANGES, SH10, STEPS, TOKEN, VIEW_LINK, btn, click, clipboard, expectNoTokenOutsideTheFragment, field, fresh, golden, hideAt, methods, open, openPublished, patches, renders, s, save, settle, shows, type } from './testing/edit-dom';
import { stubLog } from '../page/testing/stub-log';
import type { Settings } from '../lib/owner';
import { LIST_KEY } from '../lib/mine';
import { en as errorTexts } from '../lib/errors/texts';

describe('AC 1: a fresh upload, the token only in a header', () => {
  it('loads settings and draft.bin in parallel, renders the page with the panel first, leaves the address alone', async () => {
    await open('#' + TOKEN + '&x');
    expect(window.location.hash).toBe('#' + TOKEN + '&x'); // E-3: nothing rewrites it
    expect(s.calls.map((c) => c.url).sort()).toEqual(['/api/workouts/' + ID, '/api/workouts/' + ID + '/draft.bin']);
    expect($('main.published')).toBeNull();
    expect(window.location.pathname).toBe('/edit/' + ID);
    const layout = $('.page.has-map > .layout')!;
    expect(layout.firstElementChild?.id).toBe('edit-slot');
    expect(layout.firstElementChild?.firstElementChild?.matches('form.edit-panel')).toBe(true);
    expect(layout.children[1].matches('header.head')).toBe(true);
    expect($('h1')?.textContent).toMatch(/^Ride, /);
    expect(document.title).toBe($('h1')?.textContent);
    expect(field('title').placeholder).toBe($('h1')?.textContent);
    expect($('footer button[data-report]')).toBeNull();
    expect($<HTMLSelectElement>('header #theme-slot select')?.value).toBe('system'); // DS-4
    expect(stubLog.calls).toEqual(['RouteMap', 'ChartStack']);
    expectNoTokenOutsideTheFragment();
  });

  it("F-39 AC 9: today's hint, even after the viewer's hint is done; the How to use bar is there (as on the viewing page)", async () => {
    document.documentElement.dataset.hint = 'done';
    try {
      await open();
      expect(stubLog.chartStack[0].hint).toBe('plain');
      expect($('.howto-row:has([data-howto]) + .howto-row [data-summary-info]')).not.toBeNull();
    } finally {
      delete document.documentElement.dataset.hint;
    }
  });

  it('F-57: no GPX button (GX-5); the viewer download is stated under the hiding sliders (OQ-271)', async () => {
    await open();
    expect($('[data-gpx]')).toBeNull();
    const field = $('input[name="hide_end_m"]')!.closest('.edit-field')!;
    expect(field.lastElementChild?.textContent).toBe('Viewers can download the visible route as a GPX file: the track and its public pins, without times, heart rate or any other data.');
  });

  it.each(['triathlon', 'indoor-no-gps'])('F-57 QA-57-r1-1: no GPX line for %s (multisport or no positions)', async (name) => {
    s.draft = () => new Response(golden(name).slice(0), { status: 200 });
    await open();
    expect($('input[name="hide_end_m"]')).not.toBeNull();
    expect($('.edit-panel')?.textContent).not.toContain('GPX');
  });

  it('the copy link copies the view path, never the address', async () => {
    const writeText = vi.fn(async () => {});
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
    s.settings = fresh({ visibility: 'unlisted' });
    await open();
    expect($<HTMLAnchorElement>('header a[data-copy-link]')!.hidden).toBe(false);
    $('header a[data-copy-link]')!.click();
    await settle();
    expect(writeText).toHaveBeenCalledExactlyOnceWith(window.location.origin + '/w/' + ID);
  });

  it('a draft has no copy link (no /w/ page yet); it follows the visibility the server returns', async () => {
    const writeText = vi.fn(async () => {});
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
    await open();
    const link = () => $<HTMLAnchorElement>('header a[data-copy-link]')!;
    expect(link().hidden).toBe(true);
    s.onPatch = (body) => Response.json((s.settings = { ...s.settings, ...body, visibility: 'unlisted' } as Settings));
    type('title', 'a');
    await save();
    expect(link().hidden).toBe(false);
    link().click();
    await settle();
    expect(writeText).toHaveBeenCalledExactlyOnceWith(window.location.origin + '/w/' + ID);
    s.onPatch = (body) => Response.json((s.settings = { ...s.settings, ...body, visibility: 'draft' } as Settings));
    type('title', 'b');
    await save();
    expect(link().hidden).toBe(true);
  });

  it('AC 4: hiding is off on a new upload; no download toggle (F-24 discarded); visibility is "Draft" with no control', async () => {
    await open();
    expect([field('hide_start_m').value, field('hide_end_m').value]).toEqual(['0', '0']);
    expect(field('allow_download')).toBeNull();
    expect(field('visibility')).toBeNull();
    expect($('.edit-panel')?.textContent).toContain('Draft');
  });
});

describe('AC 2, E-6: saving', () => {
  it('F-45 ST-7: the suggestion is header, tab and placeholder while the title is empty; it follows the save response and is never sent', async () => {
    s.settings = fresh({ suggested_title: 'Morning ride near Köln' });
    s.onPatch = (body) => Response.json((s.settings = { ...s.settings, ...body, suggested_title: body.hide_start_m ? 'Morning ride near Bonn' : s.settings.suggested_title } as Settings));
    await open();
    expect($('h1')?.textContent).toBe('Morning ride near Köln');
    expect(document.title).toBe('Morning ride near Köln');
    expect(field('title').placeholder).toBe('Morning ride near Köln');
    expect(field('title').value).toBe('');
    hideAt(300, 300);
    flushSync();
    expect(field('title').placeholder).toBe('Morning ride near Köln'); // not from an unsaved toggle
    await save();
    expect(patches()).toEqual([{ ...ON }]);
    expect($('h1')?.textContent).toBe('Morning ride near Bonn');
    expect(document.title).toBe('Morning ride near Bonn');
    expect(field('title').placeholder).toBe('Morning ride near Bonn');
    type('title', 'Loop');
    await save();
    expect($('h1')?.textContent).toBe('Loop');
    type('title', '');
    await save();
    expect($('h1')?.textContent).toBe('Morning ride near Bonn');
    expect(field('title').value).toBe('');
    for (const p of patches()) expect(p).not.toHaveProperty('suggested_title');
  });

  it('sends only the changed field, and the header re-renders from the response', async () => {
    await open();
    type('title', 'Evening <b>loop</b>');
    expect($('.edit-status')?.textContent).toBe('');
    await save();
    expect(patches()).toEqual([{ title: 'Evening <b>loop</b>' }]);
    expect($('h1')?.textContent).toBe('Evening <b>loop</b>');
    expect($('h1 b')).toBeNull();
    expect(document.title).toBe('Evening <b>loop</b>');
    expect($('.edit-status')?.textContent).toBe('Saved');
    expect($('time[data-expires]')?.textContent).toBe('Expires in 29 days');
    type('description', 'x');
    expect($('.edit-status')?.textContent).toBe('');
    expectNoTokenOutsideTheFragment();
  });

  it('the panel refills from the stored settings (the server trims)', async () => {
    s.onPatch = (body) => {
      s.settings = { ...s.settings, ...body, title: String(body.title).trim() } as Settings;
      return Response.json(s.settings);
    };
    await open();
    type('title', '  Trimmed  ');
    await save();
    expect(field('title').value).toBe('Trimmed');
  });

  it('a pasted tab is never sent; the rest of the description stays (E-7)', async () => {
    await open();
    type('description', 'one\ttwo\nthree');
    await save();
    expect(patches()).toEqual([{ description: 'one two\nthree' }]);
    expect($('.desc')?.textContent).toBe('one two\nthree');
  });

  it('nothing changed: no request', async () => {
    await open();
    await save();
    expect(patches()).toEqual([]);
    expect($('.edit-status')?.textContent).toBe('No changes');
  });

  it('one save at a time', async () => {
    let release: () => void = () => {};
    s.onPatch = (body) => {
      s.settings = { ...s.settings, ...body } as Settings;
      return new Promise<Response>((r) => (release = () => r(Response.json(s.settings)))) as unknown as Response;
    };
    await open();
    type('title', 'A');
    $<HTMLFormElement>('form.edit-panel')!.dispatchEvent(new Event('submit', { cancelable: true }));
    await settle();
    expect($<HTMLButtonElement>('.edit-save')?.disabled).toBe(true);
    $<HTMLFormElement>('form.edit-panel')!.dispatchEvent(new Event('submit', { cancelable: true }));
    await settle();
    expect(patches()).toHaveLength(1);
    release();
    await settle();
    expect($<HTMLButtonElement>('.edit-save')?.disabled).toBe(false);
  });

  it('400: the text of the error code (F-54 LG-6, P-14), the form keeps the input, the header the stored text', async () => {
    s.onPatch = () => Response.json({ error: 'invalid_settings', message: 'The title is too long.' }, { status: 400 });
    await open();
    const before = $('h1')?.textContent;
    type('title', 'Mine');
    await save();
    expect($('.edit-status')?.textContent).toBe(errorTexts.invalid_settings);
    expect(field('title').value).toBe('Mine');
    expect($('h1')?.textContent).toBe(before);
  });

  it('F-20 C-11: a published workout has no visibility control; no request carries visibility', async () => {
    s.settings = fresh({ visibility: 'unlisted' });
    await open();
    expect(field('visibility')).toBeNull();
    expect($('.edit-panel select:not([name="lifetime"])')).toBeNull();
    expect($('.edit-panel')?.textContent).toContain('Unlisted');
    expect($('.edit-panel')?.textContent).not.toMatch(/private/i);
    type('title', 'x');
    await save();
    expect(patches()).toEqual([{ title: 'x' }]);
  });

});

describe('AC 3: lifetime', () => {
  it('offers 1 to 30 days from upload, only after now, and shows the resulting expiry', async () => {
    s.settings = fresh({ created_at: '2026-10-19T11:00:00Z', expires_at: '2026-11-18T11:00:00Z' });
    await open();
    const days = [...field('lifetime').options].map((o) => Number(o.value));
    expect(days).toEqual(Array.from({ length: 20 }, (_, i) => i + 11));
    expect(field('lifetime').value).toBe('30');
    expect($('.edit-expiry')?.textContent).toMatch(/^Expires 18 Nov 2026, \d\d:00$/);
    field('lifetime').value = '12';
    field('lifetime').dispatchEvent(new Event('change', { bubbles: true }));
    await settle();
    expect($('.edit-expiry')?.textContent).toMatch(/^Expires 31 Oct 2026, \d\d:00$/);
    await save();
    expect(patches()).toEqual([{ lifetime_days: 12 }]);
  });
});

describe('AC 5: the incomplete-file notice', () => {
  it('a truncated file shows the time of the last intact record', async () => {
    s.draft = () => new Response(golden('ride-30min-cut-60').slice(0), { status: 200 });
    s.settings = fresh({ data_end_s: 1800, hiding_steps: null });
    await open();
    expect($('.edit-notice')?.textContent).toBe('The file was incomplete; data after 00:30:00 is missing.');
  });

  it('a complete file shows none', async () => {
    await open();
    expect($('.edit-notice')).toBeNull();
  });
});

describe('AC 6, E-4: not found and errors', () => {
  it.each([
    ['#' + TOKEN.slice(0, 21) + 'x0', '/edit/' + ID],
    ['#' + TOKEN.slice(1), '/edit/' + ID],
    ['', '/edit/' + ID],
    ['#' + TOKEN, '/edit/' + ID.slice(2)],
  ])('a malformed link (%s on %s): the generic page and no request', async (hash, path) => {
    await open(hash, path);
    expect(s.calls).toEqual([]);
    expect(document.body.textContent).toContain(GENERIC);
    expect($('main.generic h1')?.textContent).toBe('Workout Explorer');
    expect($('footer a[href="/about"]')).not.toBeNull();
  });

  it.each(['settings', 'draft.bin'])('a 404 on %s: the generic page', async (which) => {
    if (which === 'settings') s.onGet = () => new Response('{}', { status: 404 });
    else s.draft = () => new Response('', { status: 404 });
    await open();
    expect(document.body.textContent).toContain(GENERIC);
    expect($('form.edit-panel')).toBeNull();
    expectNoTokenOutsideTheFragment();
  });

  it('a 404 on save: the generic page', async () => {
    s.onPatch = () => new Response('', { status: 404 });
    await open();
    type('title', 'x');
    await save();
    expect(document.body.textContent).toContain(GENERIC);
    expect(document.title).toBe('Workout Explorer');
  });

  it('a 500 on settings: the server message, never the generic page', async () => {
    s.onGet = () => Response.json({ error: 'server_error', message: 'Something went wrong on the server. Please try again later.' }, { status: 500 });
    await open();
    expect(document.body.textContent).toContain('Something went wrong on the server.');
    expect(document.body.textContent).not.toContain(GENERIC);
    expectNoTokenOutsideTheFragment();
  });

  it('a 500 on save: the message', async () => {
    s.onPatch = () => new Response('oops', { status: 500 });
    await open();
    type('title', 'x');
    await save();
    expect($('.edit-status')?.textContent).toBe('Something went wrong on the server. Please try again later.');
    expectNoTokenOutsideTheFragment();
  });

  it('draft.bin unreadable: the panel still works, D-13 text, no islands', async () => {
    s.draft = () => new Response(new Uint8Array([1, 2, 3, 4]), { status: 200 });
    await open();
    expect($('.load-error')?.textContent).toBe('The workout data could not be loaded.');
    expect(stubLog.calls).toEqual([]);
    type('title', 'Still');
    await save();
    expect(patches()).toEqual([{ title: 'Still' }]);
    expectNoTokenOutsideTheFragment();
  });
});

describe('E-9, AC 8; F-55 HR-8: to-be-hidden marks from the sliders', () => {
  const shaded = (): number[][] => {
    const over = document.createElement('div');
    (stubLog.chartStack[0].decorate as (o: HTMLElement, x: (i: number) => number) => void)(over, (i) => i);
    return [...over.querySelectorAll<HTMLElement>('.wx-trim > div')].map((d) => [parseFloat(d.style.left), parseFloat(d.style.left) + parseFloat(d.style.width)]);
  };
  const label = (name: string) => field(name).closest('label')!.textContent!.trim();
  beforeEach(() => localStorage.setItem('wx.units', 'metric')); // jsdom's en-US would default to imperial

  it('map and charts follow the unsaved slider values through hiding_steps, once per frame; saved, they stay', async () => {
    await open();
    const map = stubLog.routeMap[0];
    expect(map.hidden).toEqual([]);
    expect(shaded()).toEqual([]);
    // A drag: the value is in the label at once, the shading waits for the frame.
    field('hide_start_m').value = '500';
    field('hide_start_m').dispatchEvent(new Event('input', { bubbles: true }));
    field('hide_start_m').value = '300';
    field('hide_start_m').dispatchEvent(new Event('input', { bubbles: true }));
    flushSync();
    expect(label('hide_start_m')).toBe('Start: hide 300 m');
    expect(s.frames).toHaveLength(1); // one frame for two moves
    expect(map.hidden).toEqual([]);
    s.frames.splice(0).forEach((f) => f(0));
    flushSync();
    expect(map.hidden).toEqual([RANGES[0]]); // start only: 300 m → steps.start[6]
    expect(shaded()).toEqual([RANGES[0]]);
    hideAt(0, 500);
    expect(map.hidden).toEqual([[STEPS.end[10] + 1, N - 1]]); // end only: 500 m → steps.end[10]
    expect(label('hide_start_m')).toBe('Start: not hidden');
    expect(label('hide_end_m')).toBe('End: hide 500 m');
    hideAt(300, 300);
    expect(map.hidden).toEqual(RANGES);
    expect(shaded()).toEqual(RANGES);
    await save();
    expect(patches()).toEqual([{ ...ON }]);
    expect(map.hidden).toEqual(RANGES);
    expect(stubLog.routeMap).toHaveLength(1); // never remounted
  });

  it('sliders: 0 to 2,000 m in 50 m steps, labelled, filled from the saved radii; only changed radii sent', async () => {
    s.settings = fresh({ hide_start_m: 2000, hide_end_m: 150 });
    await open();
    for (const name of ['hide_start_m', 'hide_end_m']) expect([field(name).type, field(name).min, field(name).max, field(name).step]).toEqual(['range', '0', '2000', '50']);
    expect([field('hide_start_m').value, field('hide_end_m').value]).toEqual(['2000', '150']);
    expect(label('hide_start_m')).toBe('Start: hide 2000 m');
    expect(field('hide_end_m').getAttribute('aria-valuetext')).toBe('End: hide 150 m');
    hideAt(2000, 0);
    await save();
    expect(patches()).toEqual([{ hide_end_m: 0 }]);
    expect(Object.keys(localStorage).filter((k) => k !== 'wx.units' && k !== LIST_KEY)).toEqual([]); // OQ-248; F-53's list
  });

  it('OQ-268: imperial labels; the value sent stays metric', async () => {
    localStorage.setItem('wx.units', 'imperial');
    await open();
    hideAt(50, 150);
    expect([label('hide_start_m'), label('hide_end_m')]).toEqual(['Start: hide 160 ft', 'End: hide 490 ft']);
    hideAt(500, 2000);
    expect([label('hide_start_m'), label('hide_end_m')]).toEqual(['Start: hide 0.31 mi', 'End: hide 1.24 mi']);
    await save();
    expect(patches()).toEqual([{ hide_start_m: 500, hide_end_m: 2000 }]);
  });

  it('hiding_steps null (not yet regenerated): nothing shaded, no error', async () => {
    s.settings = fresh({ hiding_steps: null });
    await open();
    hideAt(300, 300);
    expect(stubLog.routeMap[0].hidden).toEqual([]);
    expect(shaded()).toEqual([]);
    expect(s.consoleOut).toEqual([]);
  });

  it('a fully hidden route marks the whole track', async () => {
    s.settings = fresh({ ...ON, hiding_steps: { start: STEPS.start, end: STEPS.end.map(() => -1) } });
    await open();
    expect(stubLog.routeMap[0].hidden).toEqual([[0, N - 1]]);
  });

  it('indoor: one column, no map, the toggle has no effect and no error', async () => {
    s.draft = () => new Response(golden('indoor-no-gps').slice(0), { status: 200 });
    s.settings = fresh({ hiding_steps: null });
    await open();
    expect($('.page')?.classList.contains('has-map')).toBe(false);
    expect($('#map-slot')).toBeNull();
    expect($('.hint')?.textContent).toBe('No location data in this workout.');
    hideAt(300, 300);
    await save();
    expect(patches()).toEqual([{ ...ON }]);
    expect(stubLog.routeMap).toEqual([]);
    expect(s.consoleOut.filter((a) => a.some((x) => x instanceof Error))).toEqual([]);
    expectNoTokenOutsideTheFragment();
  });
});

describe('F-20 C-10, C-11, AC 1, 2: a draft', () => {
  it('no link anywhere, no Share, no header Copy link; the draft line and SH-10 shown', async () => {
    await open();
    expect(btn('publish')).not.toBeNull();
    expect(btn('share')).toBeNull();
    expect(btn('links')).toBeNull();
    expect($('.edit-share, .edit-warn, main.published')).toBeNull();
    expect($<HTMLAnchorElement>('header a[data-copy-link]')!.hidden).toBe(true);
    expect(document.body.textContent).not.toContain('/w/' + ID);
    expect($('.edit-panel')?.textContent).toContain(DRAFT_LINE);
    expect($('.edit-panel')?.textContent).toContain(SH10);
    expectNoTokenOutsideTheFragment();
  });

  it('published: no draft line, no Publish; SH-10 still shown', async () => {
    s.settings = fresh({ visibility: 'unlisted' });
    await open();
    expect($('.edit-panel')?.textContent).not.toContain(DRAFT_LINE);
    expect(btn('publish')).toBeNull();
    expect($('.edit-panel')?.textContent).toContain(SH10);
  });
});

describe('F-20 C-2, C-3, AC 3, 4: publish', () => {
  it('without changes: only the POST, then assign to the published page; busy until unload, nothing re-rendered', async () => {
    let release: () => void = () => {};
    s.onPublish = () =>
      new Promise<Response>((r) => (release = () => r(Response.json((s.settings = { ...s.settings, visibility: 'unlisted' }))))) as unknown as Response;
    await open();
    const map = stubLog.routeMap[0];
    await click('publish');
    expect(methods()).toEqual(['POST /publish']);
    expect(s.navs).toEqual([]); // never before the 200
    release();
    await settle();
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
    expect(stubLog.routeMap).toEqual([map]);
    expect(btn('share')).toBeNull(); // not re-rendered as published
    expect($('.edit-status')?.textContent).toBe('Publishing…');
    for (const b of [$<HTMLButtonElement>('.edit-save'), btn('publish'), btn('delete'), btn('download')]) expect(b?.disabled).toBe(true);
    expectNoTokenOutsideTheFragment();
  });

  it('OQ-85: unsaved changes are saved first (PATCH before POST), then published', async () => {
    await open();
    hideAt(300, 300);
    type('title', 'Loop');
    await click('publish');
    expect(methods()).toEqual(['PATCH ', 'POST /publish']);
    expect(patches()).toEqual([{ title: 'Loop', ...ON }]);
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
  });

  it('OQ-85: a failed save stops the publish and stays', async () => {
    s.onPatch = () => Response.json({ message: 'The title is too long.' }, { status: 400 });
    await open();
    type('title', 'x');
    await click('publish');
    expect(methods()).toEqual(['PATCH ']);
    expect($('.edit-status')?.textContent).toBe('The title is too long.');
    expect(btn('publish')?.disabled).toBe(false);
    expect(s.navs).toEqual([]);
  });

  it('a 500 on publish: the message, Publish still offered, no navigation; a retry works', async () => {
    s.onPublish = () => Response.json({ message: 'Something went wrong on the server. Please try again later.' }, { status: 500 });
    await open();
    await click('publish');
    expect($('.edit-status')?.textContent).toBe('Something went wrong on the server. Please try again later.');
    expect(btn('publish')?.disabled).toBe(false);
    expect(s.navs).toEqual([]);
    s.onPublish = () => Response.json((s.settings = { ...s.settings, visibility: 'unlisted' }));
    await click('publish');
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
  });

  it('a 404 on publish: the generic page', async () => {
    s.onPublish = () => new Response('', { status: 404 });
    await open();
    await click('publish');
    expect(document.body.textContent).toContain(GENERIC);
    expect(document.title).toBe('Workout Explorer');
    expect(s.navs).toEqual([]);
  });

  it('C-2: one action at a time; Save, Publish, Delete and Download are disabled while one runs', async () => {
    let release: () => void = () => {};
    s.onPublish = () => new Promise<Response>((r) => (release = () => r(new Response('', { status: 500 })))) as unknown as Response;
    await open();
    btn('publish')!.click();
    await settle();
    for (const b of [$<HTMLButtonElement>('.edit-save'), btn('publish'), btn('delete'), btn('download')]) expect(b?.disabled).toBe(true);
    // The fields too: Publish read the form at the click, so a change now would be dropped (QA-20-r2-1).
    for (const f of FIELDS) expect(field(f).matches(':disabled'), f).toBe(true);
    expect($('.edit-status')?.textContent).toBe('Publishing…');
    type('title', 'x');
    await save();
    btn('publish')!.dispatchEvent(new MouseEvent('click', { bubbles: true })); // a disabled button's click, forced
    await settle();
    expect(methods()).toEqual(['POST /publish']);
    release();
    await settle();
    expect($<HTMLButtonElement>('.edit-save')?.disabled).toBe(false);
    for (const f of FIELDS) expect(field(f).matches(':disabled'), f).toBe(false);
  });

  it('C-2: Show links and the edit link\'s Go there are disabled while an action runs (QA-20-r2-3)', async () => {
    let release: () => void = () => {};
    s.original = () => new Promise<Response>((r) => (release = () => r(new Response('', { status: 500 })))) as unknown as Response;
    s.settings = fresh({ visibility: 'unlisted' });
    await open();
    await click('share');
    await click('download');
    expect(btn('links')?.disabled).toBe(true);
    release();
    await settle();
    expect(btn('links')?.disabled).toBe(false);
    s.onDelete = () => new Promise<Response>((r) => (release = () => r(new Response('', { status: 500 })))) as unknown as Response;
    s.host = document.createElement('div');
    document.body.replaceChildren(s.host);
    await openPublished();
    await click('delete');
    await click('confirm');
    expect(btn('back')?.disabled).toBe(true);
    release();
    await settle();
    expect(btn('back')?.disabled).toBe(false);
  });
});

describe('F-36 MC-8, OQ-112: the preview basemap job', () => {
  const KEY = '0123456789abcdef';
  const FRAME = { lon: 11.5, lat: 48.1, zoom: 12.25 };
  const withFrame = (current = false) => (s.onFrame = () => Response.json({ frame: FRAME, key: KEY, current }));
  const frameGets = () => s.calls.filter((c) => !c.init.method && c.url.endsWith('/preview-map')).length;

  it('publish: GET the frame after the save, render it, PUT under its key, then the published page', async () => {
    withFrame();
    await open();
    hideAt(300, 300);
    await click('publish');
    expect(methods()).toEqual(['PATCH ', 'POST /publish', 'PUT /preview-map/' + KEY]);
    const order = s.calls.map((c) => c.url.replace('/api/workouts/' + ID, ''));
    expect(order.indexOf('/preview-map')).toBeGreaterThan(order.indexOf('')); // after the save's 200
    expect(renders.map(([f]) => f)).toEqual([FRAME]);
    const put = s.calls.find((c) => c.init.method === 'PUT')!;
    expect(put.init.body).toBeInstanceOf(Blob);
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
    expectNoTokenOutsideTheFragment();
  });

  it('the published page opens only once the job ended', async () => {
    withFrame();
    let done: (b: Blob) => void = () => {};
    s.render = () => new Promise<Blob>((r) => (done = r));
    await open();
    await click('publish');
    expect(methods()).toEqual(['POST /publish']);
    expect(s.navs).toEqual([]);
    done(new Blob([new Uint8Array([1])]));
    await settle();
    expect(methods()).toEqual(['POST /publish', 'PUT /preview-map/' + KEY]);
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
  });

  it('a render that never ends: the published page opens after 10 s, nothing sent', async () => {
    withFrame();
    s.render = () => new Promise<Blob>(() => {});
    await open();
    vi.useFakeTimers();
    try {
      btn('publish')!.click();
      await vi.advanceTimersByTimeAsync(9_999);
      expect(s.navs).toEqual([]);
      await vi.advanceTimersByTimeAsync(1);
      expect(s.navs).toEqual([PUBLISHED_PAGE()]);
      expect(renders[0][1].aborted).toBe(true);
    } finally {
      vi.useRealTimers();
    }
    expect(methods()).toEqual(['POST /publish']);
  });

  it.each([
    ['no WebGL2', () => (s.render = () => Promise.reject(new Error('GPUInitializationError')))],
    ['a tile error', () => (s.render = () => Promise.reject(new Error('basemap: map error')))],
    ['the PUT refused', () => (s.onPut = () => Response.json({ error: 'invalid_image', message: 'x' }, { status: 400 }))],
    ['the PUT lost', () => (s.onPut = () => Promise.reject(new TypeError('network')) as unknown as Response)],
    ['the GET failed', () => (s.onFrame = () => new Response('', { status: 500 }))],
  ])('%s: publish succeeds, the published page opens, nothing shown', async (_name, fail) => {
    withFrame();
    fail();
    await open();
    await click('publish');
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
    expect($('.edit-status')?.textContent).toBe('Publishing…');
    expect(s.consoleOut).toEqual([]);
  });

  it.each([
    ['no frame (indoor, fully hidden)', { frame: null }],
    ['a current map', { frame: FRAME, key: KEY, current: true }],
  ])('%s: nothing rendered, no PUT', async (_name, body) => {
    s.onFrame = () => Response.json(body);
    await open();
    await click('publish');
    expect(frameGets()).toBe(1);
    expect(renders).toEqual([]);
    expect(methods()).toEqual(['POST /publish']);
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
  });

  it('a failed publish shows its message; the job does not open anything', async () => {
    withFrame();
    s.onPublish = () => Response.json({ message: 'Something broke.' }, { status: 500 });
    await open();
    await click('publish');
    await settle();
    expect(s.navs).toEqual([]);
    expect($('.edit-status')?.textContent).toBe('Something broke.');
  });

  it('MC-8(b): a save that changes hiding on a published workout runs the job in the background', async () => {
    s.settings = fresh({ visibility: 'unlisted' });
    withFrame();
    await open();
    hideAt(300, 300);
    await save();
    expect($('.edit-status')?.textContent).toBe('Saved');
    expect(frameGets()).toBe(1);
    expect(methods()).toEqual(['PATCH ', 'PUT /preview-map/' + KEY]);
    expect(s.navs).toEqual([]);
    expectNoTokenOutsideTheFragment();
  });

  it('MC-8(b): no job for a title-only save', async () => {
    s.settings = fresh({ visibility: 'unlisted' });
    withFrame();
    await open();
    type('title', 'Loop');
    await save();
    expect(frameGets()).toBe(0);
    expect(renders).toEqual([]);
  });

  it('MC-8(b): no job for a save of a draft', async () => {
    withFrame();
    await open();
    hideAt(300, 300);
    await save();
    expect(frameGets()).toBe(0);
  });

  it('never on the published page', async () => {
    s.settings = fresh({ visibility: 'unlisted' });
    withFrame();
    await openPublished();
    await settle();
    expect(frameGets()).toBe(0);
  });
});

describe('F-20 C-4, OQ-86, AC 7, 8: Share on the edit page', () => {
  it('published: Share shows exactly origin + view_url; copy puts exactly that; Show links opens the published page', async () => {
    const writeText = clipboard();
    s.settings = fresh({ visibility: 'unlisted' });
    await open();
    expect($<HTMLAnchorElement>('header a[data-copy-link]')!.hidden).toBe(false);
    expect($('.edit-share')).toBeNull();
    await click('share');
    expect($('.edit-share .edit-link')?.textContent).toBe(VIEW_LINK());
    $<HTMLAnchorElement>('.edit-share a[data-copy-link]')!.click();
    await settle();
    expect(writeText).toHaveBeenCalledExactlyOnceWith(VIEW_LINK());
    expectNoTokenOutsideTheFragment();
    await click('links');
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
    await click('share');
    expect($('.edit-share')).toBeNull();
  });

  it('a refused clipboard on Share: D-14\'s read-only field with the viewing link only', async () => {
    clipboard(() => Promise.reject(new Error('denied')));
    s.settings = fresh({ visibility: 'unlisted' });
    await open();
    await click('share');
    $<HTMLAnchorElement>('.edit-share a[data-copy-link]')!.click();
    await settle();
    expect($<HTMLInputElement>('.edit-share input.copy-url')?.value).toBe(VIEW_LINK());
    expectNoTokenOutsideTheFragment();
  });
});

describe('F-20 C-2: back-forward cache, never a reload', () => {
  const pageshow = (persisted: boolean): void => {
    const e = new Event('pageshow');
    Object.defineProperty(e, 'persisted', { value: persisted });
    window.dispatchEvent(e);
  };
  const gets = () => s.calls.filter((c) => !c.init.method && c.url === '/api/workouts/' + ID).length;

  it('the published page: no reload, no request on a restore (an app resume on mobile WebKit)', async () => {
    s.settings = fresh({ visibility: 'unlisted' });
    await openPublished();
    const before = s.calls.length;
    pageshow(true);
    await settle();
    expect(s.navs).toEqual([]);
    expect(s.calls).toHaveLength(before);
    expect($('main.published')).not.toBeNull();
  });

  it('the edit page, still a draft: re-checks the settings on a restore only; unsaved values stay', async () => {
    await open();
    type('title', 'Typed, not saved');
    pageshow(false);
    await settle();
    expect(gets()).toBe(1);
    pageshow(true);
    await settle();
    expect(gets()).toBe(2);
    expect(s.navs).toEqual([]);
    expect(field('title').value).toBe('Typed, not saved');
    expect(btn('publish')).not.toBeNull();
  });

  it('the edit page, published meanwhile: shows as published, the form keeps the unsaved values', async () => {
    await open();
    type('title', 'Typed, not saved');
    s.settings = { ...s.settings, title: 'Stored elsewhere', visibility: 'unlisted' };
    pageshow(true);
    await settle();
    expect(s.navs).toEqual([]);
    expect(btn('publish')).toBeNull();
    expect(btn('share')).not.toBeNull();
    expect(field('title').value).toBe('Typed, not saved');
    expect(patches()).toEqual([]);
    expect(localStorage.getItem(LIST_KEY)).toContain('"published":true'); // QA-53-r1-3: the list follows
  });

  it('Back from the published page: the edit page shows as published and is no longer locked', async () => {
    await open();
    await click('publish');
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
    expect($('.edit-status')?.textContent).toBe('Publishing…');
    pageshow(true);
    await settle();
    expect(s.navs).toEqual([PUBLISHED_PAGE()]);
    expect(btn('publish')).toBeNull();
    expect($('.edit-status')?.textContent).toBe('');
    expect($<HTMLFieldSetElement>('form.edit-panel fieldset')!.disabled).toBe(false);
    expect($<HTMLButtonElement>('form.edit-panel button[type="submit"]')!.disabled).toBe(false);
    expectNoTokenOutsideTheFragment();
  });
});

