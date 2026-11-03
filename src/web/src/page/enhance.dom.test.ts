// @vitest-environment jsdom
// The enhancers (F-13 design D-8, D-14; AC 1 expiry, AC 6 copy link). Synthetic markup only.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { COPIED_TEXT, COPY_TEXT, enhanceCopyLink, enhanceExpiry, enhanceReport, viewingUrl } from './enhance';

const ORIGIN = 'https://example.test';
let root: HTMLElement;

function markup(href = '/w/abc123'): void {
  root = document.createElement('div');
  root.innerHTML =
    `<a class="copy" href="${href}" data-copy-link>Copy link</a>` +
    '<time data-expires datetime="2026-10-30T09:15:00Z">Expires around 30 Oct 2026</time>';
  document.body.appendChild(root);
}
const link = (): HTMLAnchorElement => root.querySelector('a')!;

beforeEach(() => markup());
afterEach(() => {
  root.remove();
  vi.useRealTimers();
  vi.unstubAllEnvs();
});

describe('expiry (AC 1, UP-7, OQ-62)', () => {
  const now = Date.parse('2026-10-24T08:15:00Z'); // 6 days and 1 hour before

  it('"Expires around …" becomes "Expires in …", computed from the UTC time', () => {
    enhanceExpiry(root, now);
    expect(root.querySelector('time')?.textContent).toBe('Expires in 6 days');
  });

  it.each(['America/New_York', 'Asia/Tokyo', 'UTC'])('the same text for a viewer in %s', (zone) => {
    vi.stubEnv('TZ', zone);
    enhanceExpiry(root, now);
    expect(root.querySelector('time')?.textContent).toBe('Expires in 6 days');
  });

  it('an unreadable time keeps the server text; no element is fine', () => {
    root.querySelector('time')!.setAttribute('datetime', 'soon');
    enhanceExpiry(root, now);
    expect(root.querySelector('time')?.textContent).toBe('Expires around 30 Oct 2026');
    enhanceExpiry(document.createElement('div'), now);
  });
});

describe('viewingUrl (SH-6)', () => {
  it.each([
    ['/w/abc', ORIGIN + '/w/abc'],
    ['/w/abc#token', ORIGIN + '/w/abc'],
    ['/w/abc?t=1', ORIGIN + '/w/abc'],
  ])('%s', (href, url) => {
    expect(viewingUrl(ORIGIN, href)).toBe(url);
  });
  it.each([null, '', 'w/abc', '//evil.test/x', 'https://evil.test/x', 'javascript:alert(1)'])('%s is refused', (href) => {
    expect(viewingUrl(ORIGIN, href)).toBeNull();
  });
});

describe('copy link (AC 6, D-14)', () => {
  it('on an edit location with a token in the fragment, the copy is origin + view path, never the token', async () => {
    window.history.replaceState(null, '', '/edit/abc123#SECRET-TOKEN');
    const writeText = vi.fn(async () => {});
    enhanceCopyLink(root, window.location.origin, { clipboard: { writeText } });
    link().click();
    await Promise.resolve();
    await Promise.resolve();
    expect(writeText).toHaveBeenCalledTimes(1);
    const copied = (writeText.mock.calls[0] as unknown as [string])[0];
    expect(copied).toBe(window.location.origin + '/w/abc123');
    expect(copied).not.toContain('SECRET');
    expect(copied).not.toContain('#');
    expect(root.innerHTML).not.toContain('SECRET');
    window.history.replaceState(null, '', '/');
  });

  it('the button says "Link copied", then returns', async () => {
    vi.useFakeTimers();
    enhanceCopyLink(root, ORIGIN, { clipboard: { writeText: async () => {} } });
    link().click();
    await vi.advanceTimersByTimeAsync(0);
    expect(link().textContent).toBe(COPIED_TEXT);
    await vi.advanceTimersByTimeAsync(2000);
    expect(link().textContent).toBe(COPY_TEXT);
  });

  it('the click never navigates', () => {
    enhanceCopyLink(root, ORIGIN, { clipboard: { writeText: async () => {} } });
    const e = new MouseEvent('click', { bubbles: true, cancelable: true });
    link().dispatchEvent(e);
    expect(e.defaultPrevented).toBe(true);
  });

  it.each([
    ['no Clipboard API', {}],
    ['a refusing Clipboard API', { clipboard: { writeText: () => Promise.reject(new Error('denied')) } }],
  ])('%s: the URL appears in a read-only, selected field', async (_n, nav) => {
    enhanceCopyLink(root, ORIGIN, nav);
    link().click();
    await new Promise((r) => setTimeout(r, 0));
    const field = root.querySelector<HTMLInputElement>('input.copy-url');
    expect(field?.value).toBe(ORIGIN + '/w/abc123');
    expect(field?.readOnly).toBe(true);
    expect(link().textContent).toBe(COPY_TEXT);
    link().click(); // a second click reuses the field
    await new Promise((r) => setTimeout(r, 0));
    expect(root.querySelectorAll('input.copy-url')).toHaveLength(1);
  });

  it('a link that is not a root-relative path is not enhanced', () => {
    root.remove();
    markup('https://evil.test/x');
    const writeText = vi.fn(async () => {});
    enhanceCopyLink(root, ORIGIN, { clipboard: { writeText } });
    link().click();
    expect(writeText).not.toHaveBeenCalled();
  });
});

describe('report button (F-28 RF-1, RF-2, RF-5, RF-9)', () => {
  const ID = 'AbCdEfGhIjKlMnOpQrStUv';
  const footer = (href: string): HTMLButtonElement => {
    markup(href);
    root.insertAdjacentHTML('beforeend', '<footer class="foot"><a href="/about">About</a> <button type="button" data-report hidden>Report</button></footer>');
    return root.querySelector('button[data-report]')!;
  };
  const flush = (): Promise<void> => new Promise((r) => setTimeout(r, 0));

  it('hidden in the markup; shown with a 22-character public ID; the chunk loads only on click', async () => {
    const button = footer('/w/' + ID);
    expect(button.hidden).toBe(true); // no JavaScript, no Report control (OQ-103)
    const openReport = vi.fn();
    const load = vi.fn(async () => ({ openReport }));
    enhanceReport(root, load);
    expect(button.hidden).toBe(false);
    expect(load).not.toHaveBeenCalled();
    button.click();
    await flush();
    expect(load).toHaveBeenCalledTimes(1);
    expect(openReport).toHaveBeenCalledWith(document, ID);
  });

  it.each(['/w/abc123', '/w/' + ID + 'x', '/w/' + ID.slice(1) + '-', ''])('stays hidden for the view path %j', (href) => {
    const button = footer(href);
    enhanceReport(root, vi.fn());
    expect(button.hidden).toBe(true);
  });

  it('stays hidden without a copy link', () => {
    const button = footer('/w/' + ID);
    root.querySelector('a[data-copy-link]')!.remove();
    enhanceReport(root, vi.fn());
    expect(button.hidden).toBe(true);
  });

  it('a failed chunk shows one fixed line beside the button, however often it fails', async () => {
    const button = footer('/w/' + ID);
    enhanceReport(root, () => Promise.reject(new Error('offline')));
    button.click();
    button.click();
    await flush();
    const lines = root.querySelectorAll('.report-fail');
    expect(lines.length).toBe(1);
    expect(button.nextElementSibling).toBe(lines[0]);
    expect(lines[0].textContent).toBe('The report form could not be loaded. Reload the page and try again.');
  });
});
