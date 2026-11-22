// The owner client (F-19 design E-2, E-3; F-20 C-1, C-5; ADR-020 item 7 and rev. 2, ADR-025; F-12
// endpoints). CRITICAL: the only module that reads the URL fragment or holds the edit token. The
// token goes out only as the Authorization header of same-origin requests to
// /api/workouts/{public_id}..., in the published page's edit link, in the two navigations to the
// owner pages, and through `token` to src/edit's My workouts list write (F-53 MW-7, ADR-058); it is
// never logged or put into history.state. Imports nothing of ours.

/** A pin as the API carries it (F-56 PN-2): instants RFC 3339 UTC whole seconds; end_at null for a point. */
export interface ApiPin {
  readonly type: 'point_of_interest' | 'danger' | 'difficult_section' | 'water_food' | 'photo_spot' | 'note';
  readonly title: string;
  readonly note: string;
  readonly start_at: string;
  readonly end_at: string | null;
}

/** The settings object (F-12 "Settings object"). */
export interface Settings {
  readonly title: string;
  readonly description: string;
  /** F-55 HR-1: start and end hiding radius in metres, 0 to 2,000 in 50 m steps; 0 = not hidden. */
  readonly hide_start_m: number;
  readonly hide_end_m: number;
  /** Read-only (SH-2, F-20 C-11): publishing is the only change. */
  readonly visibility: 'draft' | 'unlisted';
  readonly lifetime_days: number;
  /** RFC 3339 UTC. */
  readonly created_at: string;
  readonly expires_at: string;
  /** UP-3: seconds since the start after which data is missing; null for a complete file. */
  readonly data_end_s: number | null;
  readonly view_url: string;
  /** Read-only, ADR-056 (F-55 HR-4): per end, entry k for radius 50·k m, the first (start) or last
   *  (end) draft.bin sample that stays public, -1 when none does; null without positions. */
  readonly hiding_steps: { readonly start: readonly number[]; readonly end: readonly number[] } | null;
  /** Read-only, F-45 ST-6: the UP-5 suggestion for an empty title; '' = none. */
  readonly suggested_title: string;
  /** F-47 TR-3: RFC 3339 UTC instants of the first and last remaining sample; null = that end not trimmed. */
  readonly trim_start_at: string | null;
  readonly trim_end_at: string | null;
  /** Read-only, F-47 TR-8: the trimmed workout's sport and summary (draft.bin's summary shape, `stroke` its only string); null without a trim. */
  readonly trimmed: { readonly sport: string; readonly summary: Readonly<Record<string, number | string>> } | null;
  /** F-49 ZN-4: the sharer's max heart rate (bpm) and threshold power (W); null = the file's. */
  readonly zones_max_heart_rate: number | null;
  readonly zones_threshold_power: number | null;
  /** F-56 PN-2: the whole stored set, dimmed pins included, canonical order. */
  readonly pins: readonly ApiPin[];
  /** F-65 AL-2: owner-added lap boundaries, RFC 3339 UTC whole seconds, ascending; [] without any. */
  readonly added_lap_boundaries: readonly string[];
}

/** F-36 MC-2, MC-4: the server's preview frame for the public route, its key and whether a stored
 *  basemap already matches it; `frame` null when the public route has no position. */
export type PreviewFrame =
  | { readonly frame: null }
  | { readonly frame: { readonly lon: number; readonly lat: number; readonly zoom: number }; readonly key: string; readonly current: boolean };

/** The fields a PATCH may carry (F-12). */
export type SettingsPatch = Partial<Pick<Settings, 'title' | 'description' | 'hide_start_m' | 'hide_end_m' | 'lifetime_days' | 'trim_start_at' | 'trim_end_at' | 'zones_max_heart_rate' | 'zones_threshold_power' | 'pins' | 'added_lap_boundaries'>>;

/** 'not-found' is ADR-025's single answer (the generic page, E-4); 'error' carries the server's
 *  `error` code and message ('' without one: UP-10's sentence); the caller picks the text by code
 *  (F-54 LG-6, lib/errors). */
export type OwnerResult<T> = { ok: true; value: T } | { ok: false; notFound: true } | { ok: false; notFound: false; message: string; code?: string };

export interface OwnerClient {
  readonly publicId: string;
  /** Which owner page the address is (F-20 C-1). */
  readonly kind: 'edit' | 'published';
  /** The edit link (F-20 C-5), only on the published page; otherwise null. Apart from `token`
   *  (F-53), the only way the token leaves this module as a value. */
  readonly editLink: string | null;
  /** F-53 MW-7: the checked edit token, read only by src/edit for the My workouts list write
   *  (lib/mine). Nothing else may read it. A function, so that serialising the client never shows it. */
  token(): string;
  /** location.assign to the published page (C-3, OQ-86). */
  toPublished(): void;
  /** To the edit page: location.replace from a draft's published page (C-5), else assign. */
  toEdit(replace: boolean): void;
  settings(): Promise<OwnerResult<Settings>>;
  save(patch: SettingsPatch): Promise<OwnerResult<Settings>>;
  /** F-15 P-1: draft → unlisted; the stored settings. */
  publish(): Promise<OwnerResult<Settings>>;
  /** AC-4: deletes everything; ok on 204. */
  delete(): Promise<OwnerResult<true>>;
  /** SH-8: the stored original and the server's file name (C-6). */
  original(): Promise<OwnerResult<{ blob: Blob; name: string }>>;
  /** F-36 MC-4: GET …/preview-map. A malformed answer is an error. */
  previewFrame(signal: AbortSignal): Promise<OwnerResult<PreviewFrame>>;
  /** F-36 MC-4: PUT …/preview-map/{key} with the PNG; ok on 204. A key that is not 16 lowercase
   *  hex sends nothing. */
  putPreviewMap(key: string, png: Blob, signal: AbortSignal): Promise<OwnerResult<true>>;
  /** The draft data file response, for lib/datafile's loader; rejects on a network failure. */
  draft(): Promise<Response>;
}


const ID = /^[0-9A-Za-z]{22}$/;
/** F-36 MC-2: the key goes into the request path, so nothing else is ever sent. */
const KEY = /^[0-9a-f]{16}$/;

/** The GET …/preview-map body, checked; null when malformed. */
function asFrame(b: unknown): PreviewFrame | null {
  const v = b as { frame?: unknown; key?: unknown; current?: unknown } | null;
  if (typeof v !== 'object' || v === null) return null;
  if (v.frame === null) return { frame: null };
  const f = v.frame as { lon?: unknown; lat?: unknown; zoom?: unknown } | undefined;
  const finite = (n: unknown): n is number => typeof n === 'number' && Number.isFinite(n);
  if (typeof f !== 'object' || f === null || !finite(f.lon) || !finite(f.lat) || !finite(f.zoom)) return null;
  if (typeof v.key !== 'string' || !KEY.test(v.key) || typeof v.current !== 'boolean') return null;
  return { frame: { lon: f.lon, lat: f.lat, zoom: f.zoom }, key: v.key, current: v.current };
}

export type OwnerLocation = Pick<Location, 'origin' | 'pathname' | 'hash' | 'assign' | 'replace'>;

/**
 * Reads `/edit/{public_id}#{token}` or `/published/{public_id}#{token}` (E-2); the token is the
 * fragment up to the first `&` (ADR-020 item 7). Null, and no request, when either part is
 * malformed. Never rewrites the address (E-3).
 */
export function openOwner(location: OwnerLocation, fetchFn: typeof fetch = fetch): OwnerClient | null {
  const path = /^\/(edit|published)\/([^/]*)$/.exec(location.pathname);
  const fragment = location.hash.slice(1);
  const amp = fragment.indexOf('&');
  const token = amp < 0 ? fragment : fragment.slice(0, amp);
  if (!path || !ID.test(path[2]) || !ID.test(token)) return null;
  const kind = path[1] as 'edit' | 'published';
  const publicId = path[2];
  const link = (page: string): string => location.origin + '/' + page + '/' + publicId + '#' + token;

  const request = (suffix: string, init: RequestInit = {}): Promise<Response> =>
    fetchFn('/api/workouts/' + publicId + suffix, {
      ...init,
      headers: { ...init.headers, Authorization: 'Bearer ' + token },
      cache: 'no-store',
      credentials: 'omit',
      referrerPolicy: 'no-referrer',
      // A redirect, even a misconfigured same-origin one, must never carry the header on (E-2).
      redirect: 'error',
    });

  /** One owner request: 404 is not found; another failure carries the server's message or UP-10's
   *  sentence; `read` turns a success into the value (null or a throw: UP-10's sentence). */
  const send = async <T>(suffix: string, init: RequestInit, read: (r: Response) => Promise<T | null>): Promise<OwnerResult<T>> => {
    const failed = { ok: false, notFound: false, message: '' } as const;
    let response: Response;
    try {
      response = await request(suffix, init);
    } catch {
      return failed;
    }
    if (response.status === 404) return { ok: false, notFound: true };
    if (response.ok) {
      const value = await read(response).catch(() => null);
      return value === null ? failed : { ok: true, value };
    }
    const body = (await response.json().catch(() => null)) as { message?: unknown; error?: unknown } | null;
    const message = typeof body?.message === 'string' ? body.message : '';
    return typeof body?.error === 'string' ? { ok: false, notFound: false, message, code: body.error } : { ok: false, notFound: false, message };
  };
  const asSettings = (r: Response): Promise<Settings | null> =>
    r.json().then((b: unknown) => (typeof b === 'object' && b !== null ? (b as Settings) : null));

  return {
    publicId,
    kind,
    editLink: kind === 'published' ? link('edit') : null,
    token: () => token,
    toPublished: () => location.assign(link('published')),
    toEdit: (replace) => (replace ? location.replace(link('edit')) : location.assign(link('edit'))),
    settings: () => send('', {}, asSettings),
    save: (patch) => send('', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(patch) }, asSettings),
    publish: () => send('/publish', { method: 'POST' }, asSettings),
    delete: () => send('', { method: 'DELETE' }, async () => true as const),
    original: () =>
      send('/original.fit', {}, async (r) => ({
        blob: await r.blob(),
        // The server's name (OQ-59); its rule allows no quote.
        name: /filename="([^"]+)"/.exec(r.headers.get('Content-Disposition') ?? '')?.[1] ?? 'workout.fit',
      })),
    previewFrame: (signal) => send('/preview-map', { signal }, (r) => r.json().then(asFrame)),
    putPreviewMap: (key, png, signal) =>
      KEY.test(key)
        ? send('/preview-map/' + key, { method: 'PUT', headers: { 'Content-Type': 'image/png' }, body: png, signal }, async () => true as const)
        : Promise.resolve({ ok: false, notFound: false, message: '' }),
    draft: () => request('/draft.bin'),
  };
}
