import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { loadDataFile } from './load';

function response(init: { ok?: boolean; status?: number; body?: () => Promise<ArrayBuffer> }): Response {
  return {
    ok: init.ok ?? true,
    status: init.status ?? 200,
    arrayBuffer: init.body ?? (() => Promise.resolve(new ArrayBuffer(0))),
  } as unknown as Response;
}

const golden = loadGolden().find((g) => g.name === 'indoor-no-gps');

describe('loadDataFile', () => {
  it('maps a rejected request to network', async () => {
    const r = await loadDataFile(() => Promise.reject(new TypeError('Failed to fetch https://example.invalid/x')));
    expect(r).toEqual({ ok: false, error: { code: 'network', message: 'data file: request failed (TypeError)' } });
  });

  it('maps a request that throws synchronously to network', async () => {
    const r = await loadDataFile(() => {
      throw new DOMException('aborted', 'AbortError');
    });
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.error.code).toBe('network');
  });

  it('maps a 404 to http-status with the status', async () => {
    const r = await loadDataFile(() => Promise.resolve(response({ ok: false, status: 404 })));
    expect(r.ok).toBe(false);
    if (!r.ok) expect([r.error.code, r.error.status]).toEqual(['http-status', 404]);
  });

  it('maps an unreadable body to network', async () => {
    const r = await loadDataFile(() => Promise.resolve(response({ body: () => Promise.reject(new TypeError('body')) })));
    expect(r.ok).toBe(false);
    if (!r.ok) expect(r.error.code).toBe('network');
  });

  it('decodes a golden file', async () => {
    if (!golden) throw new Error('indoor-no-gps golden missing');
    const r = await loadDataFile(() => Promise.resolve(response({ body: () => Promise.resolve(golden.bin.slice(0)) })));
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.data.position).toBeUndefined();
  });

  it('reports a bad body as not-a-data-file', async () => {
    const body = new TextEncoder().encode('<!doctype html>').buffer as ArrayBuffer;
    const r = await loadDataFile(() => Promise.resolve(response({ body: () => Promise.resolve(body) })));
    expect(r.ok).toBe(false);
    if (!r.ok) expect([r.error.code, r.error.rule]).toEqual(['not-a-data-file', 1]);
  });

  it('never rejects, even for a broken response object', async () => {
    await expect(loadDataFile(() => Promise.resolve(null as unknown as Response))).resolves.toMatchObject({
      ok: false,
      error: { code: 'internal' },
    });
  });
});
