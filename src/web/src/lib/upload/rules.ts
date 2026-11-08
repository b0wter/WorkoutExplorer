// Pure rules of the upload page (F-11 design U-4 to U-9). No DOM, no imports.

/** The 20 MB cap in bytes of file content (ADR-021 item 1; F-09 D-5). */
export const MAX_UPLOAD_BYTES = 20_000_000;

/** The only sentences the page holds itself (U-8); every other text comes from the server. */
export const SENTENCES = {
  tooLarge: 'This file is larger than 20 MB.',
  unreadable: 'The upload could not be read. Please try again.',
  serverError: 'Something went wrong on the server. Please try again later.',
  paused: 'Uploads are paused because storage is full. Please try again later.',
  // Zip archives (F-35); the last four are keyed by lib/zip's refusal reasons.
  opening: 'Opening the archive…',
  noZip: 'This browser cannot open zip archives. Please unzip it and upload the FIT file.',
  noFit: 'This archive contains no FIT file.',
  several: 'This archive contains more than one FIT file. Please upload one at a time.',
  fitTooLarge: 'The FIT file in this archive is larger than 20 MB.',
  damaged: 'This archive could not be opened. Please unzip it and upload the FIT file.',
} as const;

/** The page's states (U-5, U-7; F-35 Z-11: `opening` reads the file and opens an archive). Only `ready` accepts a file. */
export type PageState = 'checking' | 'ready' | 'opening' | 'uploading' | 'processing' | 'paused';

export function canPick(state: PageState): boolean {
  return state === 'ready';
}

/** True when the file is refused in the browser (U-6). Exactly 20,000,000 is sent. */
export function tooLarge(size: number): boolean {
  return size > MAX_UPLOAD_BYTES;
}

/** Whole percent of an upload, 0 to 100. */
export function percent(loaded: number, total: number): number {
  if (!(total > 0)) return 0;
  return Math.max(0, Math.min(100, Math.floor((loaded * 100) / total)));
}

/** Reads the `GET /api/status` body: true or false, or null when it is not the expected shape (U-5). */
export function parseStatus(text: string): boolean | null {
  const v = parseJson(text);
  if (v && typeof v === 'object' && typeof (v as { uploads_open?: unknown }).uploads_open === 'boolean') {
    return (v as { uploads_open: boolean }).uploads_open;
  }
  return null;
}

const BASE62_22 = /^[0-9A-Za-z]{22}$/;

/**
 * The edit link to open after an upload (U-9, ADR-020 items 2, 3, 7 and rev. 2): same origin,
 * path /edit/ plus 22 base62 characters, no query, fragment of 22 base62 characters. Returns the
 * absolute address, unchanged (no marker), or null when the shape is wrong.
 */
export function editTarget(editUrl: unknown, origin: string): string | null {
  if (typeof editUrl !== 'string') return null;
  let u: URL;
  try {
    u = new URL(editUrl, origin);
  } catch {
    return null;
  }
  if (u.origin !== origin || u.search !== '' || u.username !== '' || u.password !== '') return null;
  if (!u.pathname.startsWith('/edit/') || !BASE62_22.test(u.pathname.slice(6))) return null;
  if (!u.hash.startsWith('#') || !BASE62_22.test(u.hash.slice(1))) return null;
  return `${origin}${u.pathname}${u.hash}`;
}

/** What the page does with an upload response (U-8, U-9, table "Response handling"). */
export type Outcome =
  | { kind: 'navigate'; url: string }
  | { kind: 'error'; message: string }
  | { kind: 'paused'; message: string };

export function outcomeOf(status: number, body: string, origin: string): Outcome {
  const v = parseJson(body);
  if (status === 201) {
    const url = editTarget(v && typeof v === 'object' ? (v as { edit_url?: unknown }).edit_url : undefined, origin);
    return url ? { kind: 'navigate', url } : { kind: 'error', message: SENTENCES.serverError };
  }
  let message: string = SENTENCES.serverError;
  let code: unknown;
  if (v && typeof v === 'object') {
    const m = (v as { message?: unknown }).message;
    if (typeof m === 'string' && m !== '') message = m;
    code = (v as { error?: unknown }).error;
  }
  if (status === 503 && code === 'storage_full') return { kind: 'paused', message };
  return { kind: 'error', message };
}

function parseJson(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}
