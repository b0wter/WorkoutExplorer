// Server messages chosen by the response's `error` code (F-54 LG-6, ADR-059 item 5): one text per
// code of the API's code list (src/server/testdata/api/error-codes.json), in every language
// English included; the server's English `message` only for a code not listed. The general codes
// (P-14) get one plain sentence each; codes with several server sentences a context-neutral one.
import { texts } from '../i18n';

export const en = {
  bad_request: 'The request was not accepted. Please reload the page and try again.',
  busy: 'The server is busy. Please try again in a minute.',
  canceled: 'The upload was interrupted.',
  internal: 'This file could not be read.',
  invalid_image: 'The preview image was not accepted. Please reload the page and try again.',
  invalid_report: 'The report was not accepted. Please reload the page and try again.',
  invalid_settings: 'The change was not accepted. Please reload the page and try again.',
  limit: 'This file is too large or too complex to read.',
  no_data: 'This file contains no readable workout data.',
  not_fit: 'This file is not a FIT file.',
  not_found: 'This workout has expired or is not available.',
  rate_limited: 'Too many requests. Please try again later.',
  server_error: 'Something went wrong on the server. Please try again later.',
  stale: 'The preview image is out of date. Please reload the page and try again.',
  storage_full: 'Uploads are paused because storage is full. Please try again later.',
  too_large: 'The file is too large.',
  unreadable: 'This file could not be read.',
  unsupported_type: 'Unsupported FIT file type.',
};

export const t = texts(en);

/** The text for an API error body: ours by code; for an unlisted code the server's message, else `fallback`. */
export function serverText(body: unknown, fallback = t('server_error')): string {
  const b = (body && typeof body === 'object' ? body : {}) as { error?: unknown; message?: unknown };
  if (typeof b.error === 'string' && Object.hasOwn(en, b.error)) return t(b.error as keyof typeof en);
  return typeof b.message === 'string' && b.message !== '' ? b.message : fallback;
}
