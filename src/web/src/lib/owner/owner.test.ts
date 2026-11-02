// The owner client (F-19 design E-2 to E-4; ADR-020 item 7 and rev. 1, ADR-025). Critical: the
// token is read from the fragment only, sent only as a header, never in a URL or history state.
import { describe, expect, it, vi } from 'vitest';
import { SENTENCES } from '../upload';
import { SERVER_ERROR, openOwner, type Settings } from '.';

const ID = 'AbCdEfGhIjKlMnOpQrStUv';
const TOKEN = 'Zz0123456789abcdefghij';

const SETTINGS: Settings = {
  title: 't',
  description: '',
  hide_ends: false,
  allow_download: false,
  visibility: 'draft',
  lifetime_days: 30,
  created_at: '2026-10-01T10:00:00Z',
  expires_at: '2026-10-31T10:00:00Z',
  data_end_s: null,
  view_url: '/w/' + ID,
  hide_ends_ranges: [],
};

function setup(hash: string, respond: () => Response | Promise<Response> = () => Response.json(SETTINGS), path = '/edit/' + ID) {
  const assign = vi.fn();
  const replace = vi.fn();
  const fetchFn = vi.fn(async (_url: string, _init: RequestInit) => respond());
  const owner = openOwner({ origin: 'https://wx.example', pathname: path, hash, assign, replace }, fetchFn as unknown as typeof fetch);
  return { owner, assign, replace, fetchFn };
}

describe('openOwner: the link', () => {
  it.each([
    ['', 'no fragment'],
    ['#', 'empty fragment'],
    ['#' + TOKEN.slice(1), '21 characters'],
    ['#' + TOKEN + 'x', '23 characters'],
    ['#' + TOKEN.slice(1) + '-', 'not base62'],
    ['#&' + TOKEN, 'token after a parameter'],
  ])('a malformed token (%s, %s) gives null and touches nothing', (hash) => {
    for (const path of ['/edit/' + ID, '/published/' + ID]) {
      const { owner, fetchFn } = setup(hash, undefined, path);
      expect(owner).toBeNull();
      expect(fetchFn).not.toHaveBeenCalled();
    }
  });

  it.each(['/edit/' + ID.slice(1), '/edit/' + ID + '/', '/edit/' + ID + '/x', '/w/' + ID, '/edit/', '/published/' + ID + '/', '/publish/' + ID, '/x/edit/' + ID])(
    'a malformed path (%s) gives null',
    (path) => {
      expect(setup('#' + TOKEN, undefined, path).owner).toBeNull();
    },
  );

  it('the token is the fragment up to the first &; nothing after it is used (ADR-020 item 7)', () => {
    const { owner, fetchFn } = setup('#' + TOKEN + '&new');
    void owner!.settings();
    expect((fetchFn.mock.calls[0][1].headers as Record<string, string>).Authorization).toBe('Bearer ' + TOKEN);
  });

  it('the edit page: kind edit, no edit link, the client object carries no token (F-20 C-5)', () => {
    const { owner } = setup('#' + TOKEN);
    expect(owner?.publicId).toBe(ID);
    expect(owner?.kind).toBe('edit');
    expect(owner?.editLink).toBeNull();
    expect(JSON.stringify(owner)).not.toContain(TOKEN);
    expect(Object.values(owner ?? {}).map(String).join()).not.toContain(TOKEN);
  });

  it('the published page: kind published; the token only in the edit link (C-5)', () => {
    const { owner } = setup('#' + TOKEN + '&x=1', undefined, '/published/' + ID);
    expect(owner?.kind).toBe('published');
    expect(owner?.editLink).toBe('https://wx.example/edit/' + ID + '#' + TOKEN);
    const { editLink: _, ...rest } = owner!;
    expect(JSON.stringify(rest)).not.toContain(TOKEN);
    expect(Object.values(rest).map(String).join()).not.toContain(TOKEN);
  });

  it('navigations: published page with assign; edit page with replace or assign', () => {
    const { owner, assign, replace } = setup('#' + TOKEN);
    owner!.toPublished();
    expect(assign).toHaveBeenLastCalledWith('https://wx.example/published/' + ID + '#' + TOKEN);
    owner!.toEdit(false);
    expect(assign).toHaveBeenLastCalledWith('https://wx.example/edit/' + ID + '#' + TOKEN);
    expect(replace).not.toHaveBeenCalled();
    owner!.toEdit(true);
    expect(replace).toHaveBeenCalledExactlyOnceWith('https://wx.example/edit/' + ID + '#' + TOKEN);
    expect(assign).toHaveBeenCalledTimes(2);
  });
});

describe('requests (E-2)', () => {
  const expectOwnerRequest = (call: [string, RequestInit], suffix: string, method?: string): void => {
    const [url, init] = call;
    expect(url).toBe('/api/workouts/' + ID + suffix);
    expect(url).not.toMatch(/[?#]/);
    expect(url).not.toContain(TOKEN);
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer ' + TOKEN);
    expect(init).toMatchObject({ cache: 'no-store', credentials: 'omit', referrerPolicy: 'no-referrer', redirect: 'error' });
    expect(init.method).toBe(method);
  };

  it('settings, save and draft: same-origin path, token only in the header, redirect: error', async () => {
    const { owner, fetchFn } = setup('#' + TOKEN);
    await owner!.settings();
    await owner!.save({ title: 'x' });
    await owner!.draft();
    expectOwnerRequest(fetchFn.mock.calls[0], '');
    expectOwnerRequest(fetchFn.mock.calls[1], '', 'PATCH');
    expect(fetchFn.mock.calls[1][1].body).toBe('{"title":"x"}');
    expect((fetchFn.mock.calls[1][1].headers as Record<string, string>)['Content-Type']).toBe('application/json');
    expectOwnerRequest(fetchFn.mock.calls[2], '/draft.bin');
  });

  it('publish, delete and original (F-20 C-1): same rules, no body, no query', async () => {
    const { owner, fetchFn } = setup('#' + TOKEN);
    await owner!.publish();
    await owner!.delete();
    await owner!.original();
    expectOwnerRequest(fetchFn.mock.calls[0], '/publish', 'POST');
    expectOwnerRequest(fetchFn.mock.calls[1], '', 'DELETE');
    expectOwnerRequest(fetchFn.mock.calls[2], '/original.fit');
    for (const [, init] of fetchFn.mock.calls) expect(init.body).toBeUndefined();
  });

  it('publish: 200 the settings, 404 not found, 500 the message', async () => {
    expect(await setup('#' + TOKEN).owner!.publish()).toEqual({ ok: true, value: SETTINGS });
    expect(await setup('#' + TOKEN, () => new Response('', { status: 404 })).owner!.publish()).toEqual({ ok: false, notFound: true });
    const r = await setup('#' + TOKEN, () => Response.json({ message: 'Publishing failed.' }, { status: 500 })).owner!.publish();
    expect(r).toEqual({ ok: false, notFound: false, message: 'Publishing failed.' });
  });

  it('delete: 204 ok, 404 not found, 500 UP-10 sentence', async () => {
    expect(await setup('#' + TOKEN, () => new Response(null, { status: 204 })).owner!.delete()).toEqual({ ok: true, value: true });
    expect(await setup('#' + TOKEN, () => new Response('', { status: 404 })).owner!.delete()).toEqual({ ok: false, notFound: true });
    expect(await setup('#' + TOKEN, () => new Response('x', { status: 500 })).owner!.delete()).toEqual({ ok: false, notFound: false, message: SERVER_ERROR });
  });

  it('original: the bytes unchanged and the name from Content-Disposition; workout.fit without it', async () => {
    const bytes = new Uint8Array([14, 16, 0x98, 0, 0xff, 0x2e, 0x46]);
    const named = await setup('#' + TOKEN, () =>
      new Response(bytes, { headers: { 'Content-Disposition': 'attachment; filename="workout-2026-10-29.fit"' } }),
    ).owner!.original();
    expect(named.ok && named.value.name).toBe('workout-2026-10-29.fit');
    expect(named.ok && new Uint8Array(await named.value.blob.arrayBuffer())).toEqual(bytes);
    const bare = await setup('#' + TOKEN, () => new Response(bytes)).owner!.original();
    expect(bare.ok && bare.value.name).toBe('workout.fit');
    expect(await setup('#' + TOKEN, () => new Response('', { status: 404 })).owner!.original()).toEqual({ ok: false, notFound: true });
  });

  it('200: the settings object', async () => {
    expect(await setup('#' + TOKEN).owner!.settings()).toEqual({ ok: true, value: SETTINGS });
  });

  it('404: not found, whatever the body', async () => {
    const r = await setup('#' + TOKEN, () => new Response('{"message":"x"}', { status: 404 })).owner!.save({});
    expect(r).toEqual({ ok: false, notFound: true });
  });

  it('400 and 500: the server message; without one, UP-10 sentence', async () => {
    const bad = await setup('#' + TOKEN, () => Response.json({ error: 'invalid_settings', message: 'Title too long.' }, { status: 400 })).owner!.save({ title: 'x' });
    expect(bad).toEqual({ ok: false, notFound: false, message: 'Title too long.' });
    const proxy = await setup('#' + TOKEN, () => new Response('<html>Bad gateway</html>', { status: 502 })).owner!.settings();
    expect(proxy).toEqual({ ok: false, notFound: false, message: SERVER_ERROR });
  });

  it('a network failure or a refused redirect (fetch rejects): UP-10 sentence, no detail', async () => {
    const r = await setup('#' + TOKEN, () => Promise.reject(new TypeError('redirect mode is error: ' + TOKEN))).owner!.settings();
    expect(r).toEqual({ ok: false, notFound: false, message: SERVER_ERROR });
  });

  it('the server-error sentence is lib/upload\'s', () => {
    expect(SERVER_ERROR).toBe(SENTENCES.serverError);
  });
});
