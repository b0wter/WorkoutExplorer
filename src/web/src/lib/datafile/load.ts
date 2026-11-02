// loadDataFile: the only async function of lib/datafile (F-04 design D-8).
// The caller supplies the request, so it decides the URL (taken from the page,
// never built) and any headers (edit token for the draft, F-19). Never rejects.

import { decodeDataFile } from './decode';
import { failure } from './meta';
import type { DataFileResult } from './model';

function errorName(e: unknown): string {
  return e instanceof Error ? e.name : typeof e;
}

export async function loadDataFile(request: () => Promise<Response>): Promise<DataFileResult> {
  try {
    let response: Response;
    try {
      response = await request();
    } catch (e) {
      // No exception message: it could name the URL. The name is enough to tell abort from network.
      return failure('network', undefined, `data file: request failed (${errorName(e)})`);
    }
    if (!response.ok) {
      return {
        ok: false,
        error: { code: 'http-status', status: response.status, message: `data file: HTTP status ${response.status}` },
      };
    }
    let body: ArrayBuffer;
    try {
      body = await response.arrayBuffer();
    } catch (e) {
      return failure('network', undefined, `data file: body could not be read (${errorName(e)})`);
    }
    return decodeDataFile(body);
  } catch (e) {
    return failure('internal', undefined, `data file: internal error: ${e instanceof Error ? e.message : String(e)}`);
  }
}
