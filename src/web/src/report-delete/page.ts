// The delete link's confirmation page (F-37 DK-6, DK-7, DK-11, DK-12; INF-6). Opening never deletes:
// a lookup, then a POST only on a click. The key goes to the server only in the Authorization header.
// The address keeps its fragment, as the edit page (F-19 E-3), so a reload still works.

const KEY = /^[0-9A-Za-z]{22}$/;
const INVALID = 'This delete link is no longer valid: it was already used, has expired, or the workout is gone.';
const SPENT = 'The delete link has been used up, but the deletion failed. The workout may still be online. Please delete it on the server with the `workout-explorer delete` command.';
const FAILED = 'Something went wrong. Please try again later.';

type Fetch = (input: string, init?: RequestInit) => Promise<Response>;
type State = 'loading' | 'ready' | 'deleting' | 'deleted' | 'failed';

interface Lookup {
  title: string;
  start_date: string;
  report_id: number;
  reason: string;
  filed_at: number;
  expires_at: number;
  report_count: number;
}

/** The key: the fragment up to the first `&` (ADR-020 item 7); null when malformed. */
export function keyOf(hash: string): string | null {
  const key = hash.slice(1).split('&')[0];
  return KEY.test(key) ? key : null;
}

const when = (s: number): string =>
  new Date(s * 1000).toLocaleString('en-GB', { dateStyle: 'medium', timeStyle: 'short' });

function valid(v: Partial<Lookup> | null): v is Lookup {
  return (
    !!v &&
    typeof v.title === 'string' &&
    typeof v.start_date === 'string' &&
    typeof v.reason === 'string' &&
    [v.report_id, v.filed_at, v.expires_at, v.report_count].every((n) => typeof n === 'number' && Number.isFinite(n))
  );
}

export function start(doc: Document, hash: string, fetchFn: Fetch): Promise<void> {
  const page = doc.getElementById('page')!;
  const facts = doc.getElementById('facts')!;
  const message = doc.getElementById('message')!;
  const button = doc.getElementById('delete') as HTMLButtonElement;

  const show = (state: State, text = ''): void => {
    page.dataset.state = state;
    message.textContent = text;
    button.disabled = state !== 'ready';
    if (state === 'failed' || state === 'deleted') facts.replaceChildren();
  };

  const key = keyOf(hash);
  if (!key) {
    show('failed', INVALID);
    return Promise.resolve();
  }

  const request = (method: 'GET' | 'POST'): Promise<Response> =>
    fetchFn('/api/report-delete', {
      method,
      headers: { Authorization: 'Bearer ' + key },
      cache: 'no-store',
      credentials: 'same-origin',
      referrerPolicy: 'no-referrer',
      redirect: 'error', // never carry the header on
    });

  /** The server's sentence for a refusal, or FAILED. */
  const refusal = async (res: Response): Promise<string> => {
    const text = ((await res.json().catch(() => null)) as { message?: unknown } | null)?.message;
    return typeof text === 'string' && text !== '' ? text : res.status === 404 ? INVALID : FAILED;
  };

  button.addEventListener('click', async () => {
    if (button.disabled) return;
    show('deleting');
    try {
      const res = await request('POST');
      if (res.status === 204) return show('deleted', 'The workout has been deleted.');
      const text = await refusal(res);
      // 404: link spent or workout gone. 500: the key is already consumed (DK-8), no retry.
      // 429 (before consume) and other errors stay retryable.
      if (res.status === 500) show('failed', SPENT);
      else if (res.status === 404) show('failed', text);
      else show('ready', text);
    } catch {
      show('ready', FAILED);
    }
  });

  return (async () => {
    show('loading', 'Loading…');
    try {
      const res = await request('GET');
      if (res.status !== 200) return show('failed', await refusal(res));
      const v = (await res.json().catch(() => null)) as Partial<Lookup> | null;
      if (!valid(v)) return show('failed', FAILED);
      const rows: [string, string][] = [
        ['Workout', v.title],
        ['Start', v.start_date],
        ['Report', `#${v.report_id}: ${v.reason}`],
        ['Filed', when(v.filed_at)],
        ['Reports', String(v.report_count)],
        ['Expires', when(v.expires_at)],
      ];
      facts.replaceChildren(
        ...rows.flatMap(([k, val]) => {
          const dt = doc.createElement('dt');
          const dd = doc.createElement('dd');
          dt.textContent = k;
          dd.textContent = val;
          return [dt, dd];
        }),
      );
      show('ready');
    } catch {
      show('failed', FAILED);
    }
  })();
}
