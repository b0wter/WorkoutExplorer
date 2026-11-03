// @vitest-environment jsdom
// The report form (F-28 RF-3 to RF-8; AC 2 to AC 5). jsdom has no showModal: a minimal stand-in.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { openReport } from './form';

const ID = 'AbCdEfGhIjKlMnOpQrStUv';
const FALLBACK = 'The report could not be sent. Please try again later.';
let fetchMock: ReturnType<typeof vi.fn>;
let respond: () => Promise<Response>;

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
    this.open = false;
    this.dispatchEvent(new Event('close'));
  };
  respond = async () => new Response(null, { status: 204 });
  fetchMock = vi.fn(() => respond());
  vi.stubGlobal('fetch', fetchMock);
});
afterEach(() => {
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const $ = <T extends Element = HTMLElement>(q: string): T => document.querySelector<T>('dialog.report ' + q)!;
const dialog = (): HTMLDialogElement => document.querySelector('dialog.report')!;
const textarea = (): HTMLTextAreaElement => $('textarea');
const send = (): HTMLButtonElement => $('button[type=submit]');
const settle = async (): Promise<void> => {
  for (let k = 0; k < 5; k++) await new Promise((r) => setTimeout(r, 0));
};

function fill(reason: string | null, message = ''): void {
  if (reason) $<HTMLInputElement>(`input[value=${reason}]`).checked = true;
  textarea().value = message;
  textarea().dispatchEvent(new Event('input'));
}
async function submit(): Promise<void> {
  $<HTMLFormElement>('form').requestSubmit();
  await settle();
}

describe('the form (RF-4, AC 3, AC 4)', () => {
  it('opens a modal dialog with four reasons, none chosen, and an optional message of at most 1,000', () => {
    openReport(document, ID);
    expect(dialog().open).toBe(true);
    const radios = [...document.querySelectorAll<HTMLInputElement>('dialog.report input[type=radio]')];
    expect(radios.map((r) => r.value)).toEqual(['spam', 'illegal_content', 'personal_data', 'other']);
    expect(radios.map((r) => r.parentElement?.textContent?.trim())).toEqual(['Spam', 'Illegal content', "Someone else's personal data", 'Other']);
    expect(radios.some((r) => r.checked)).toBe(false);
    expect(radios[0].required).toBe(true);
    expect(textarea().maxLength).toBe(1000);
    expect(textarea().required).toBe(false);
    expect(textarea().getAttribute('aria-describedby')).toBe('report-count report-note');
    expect($('#report-note').textContent).toBe('Please do not include contact details. Reports get no reply.');
  });

  it('asks for no name, email or other contact detail (AC 4)', () => {
    openReport(document, ID);
    const fields = [...dialog().querySelectorAll('input, textarea, select')];
    expect(fields.map((f) => f.getAttribute('type') ?? f.tagName)).toEqual(['radio', 'radio', 'radio', 'radio', 'TEXTAREA']);
    expect(dialog().textContent).not.toMatch(/name|e-?mail|phone/i);
  });

  it('counts in UTF-16 units, as maxlength does', () => {
    openReport(document, ID);
    expect($('.report-count').textContent).toBe('0 / 1000');
    fill(null, 'ab🚴');
    expect($('.report-count').textContent).toBe('4 / 1000');
  });

  it('one dialog per page, reused; Cancel closes and keeps reason and text (RF-8)', () => {
    openReport(document, ID);
    fill('spam', 'kept');
    $<HTMLButtonElement>('button[type=button]').click();
    expect(dialog().open).toBe(false);
    openReport(document, ID);
    expect(document.querySelectorAll('dialog').length).toBe(1);
    expect(textarea().value).toBe('kept');
    expect($<HTMLInputElement>('input[value=spam]').checked).toBe(true);
  });
});

describe('sending (RF-5, RF-6, AC 2)', () => {
  it('no reason: nothing is sent', async () => {
    openReport(document, ID);
    fill(null, 'text');
    await submit();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('a reason and no message: exactly the three keys, then the confirmation; the next open is empty', async () => {
    openReport(document, ID);
    fill('other');
    await submit();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe('/api/reports');
    expect(init).toEqual({ method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ public_id: ID, reason: 'other', message: '' }) });
    expect($('form').hidden).toBe(true);
    expect($('.report-done').hidden).toBe(false);
    expect($('.report-done').getAttribute('role')).toBe('status');
    expect($('.report-done').textContent).toContain('Thank you. The report has been sent.');
    $<HTMLButtonElement>('.report-done button').click();
    expect(dialog().open).toBe(false);
    openReport(document, ID);
    expect($('form').hidden).toBe(false);
    expect($('.report-done').hidden).toBe(true);
    expect($<HTMLInputElement>('input[value=other]').checked).toBe(false);
    expect($('.report-count').textContent).toBe('0 / 1000');
  });

  it('control characters other than line feed become spaces; nothing stored, nothing logged (RF-6)', async () => {
    const log = vi.spyOn(console, 'log');
    const warn = vi.spyOn(console, 'warn');
    openReport(document, ID);
    fill('spam', 'a\tb\nc\x07d');
    await submit();
    expect(JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string).message).toBe('a b\nc d');
    expect(localStorage.length + sessionStorage.length).toBe(0);
    expect(document.cookie).toBe('');
    expect(log).not.toHaveBeenCalled();
    expect(warn).not.toHaveBeenCalled();
  });

  it('one request at a time: the send button is disabled and reads "Sending…"', async () => {
    let finish: (r: Response) => void = () => {};
    respond = () => new Promise((r) => (finish = r));
    openReport(document, ID);
    fill('spam');
    await submit();
    expect(send().disabled).toBe(true);
    expect(send().textContent).toBe('Sending…');
    finish(new Response(null, { status: 204 }));
    await settle();
    expect(send().disabled).toBe(false);
    expect(send().textContent).toBe('Send report');
  });
});

describe('errors keep the form (RF-7, AC 5)', () => {
  const json = (status: number, body: unknown) => async () => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
  it.each([
    ['429', json(429, { error: 'rate_limited', message: 'Too many reports. Please try again later.' }), 'Too many reports. Please try again later.'],
    ['404', json(404, { error: 'not_found', message: 'This workout has expired or is not available.' }), 'This workout has expired or is not available.'],
    ['400', json(400, { error: 'invalid_report', message: 'Server sentence.' }), 'Server sentence.'],
    ['500', json(500, { error: 'server_error', message: 'Something went wrong.' }), 'Something went wrong.'],
    ['500 without a message string', json(500, { message: 5 }), FALLBACK],
    ['400 with null', json(400, null), FALLBACK],
    ['502 from Caddy, not JSON', async () => new Response('Bad Gateway', { status: 502 }), FALLBACK],
    ['503 with a JSON message', json(503, { message: 'not ours' }), FALLBACK],
    ['200 is not 204', json(200, { message: 'odd' }), FALLBACK],
    ['a network failure', () => Promise.reject(new TypeError('Failed to fetch')), FALLBACK],
  ])('%s', async (_name, answer, text) => {
    respond = answer;
    openReport(document, ID);
    fill('personal_data', 'my text');
    await submit();
    expect($('.report-error').textContent).toBe(text);
    expect($('.report-error').getAttribute('role')).toBe('alert');
    expect($('form').hidden).toBe(false);
    expect(textarea().value).toBe('my text');
    expect($<HTMLInputElement>('input[value=personal_data]').checked).toBe(true);
    expect(send().disabled).toBe(false);
    // A retry clears the old sentence while it sends.
    respond = async () => new Response(null, { status: 204 });
    await submit();
    expect($('.report-done').hidden).toBe(false);
  });
});
