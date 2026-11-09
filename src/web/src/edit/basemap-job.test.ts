// The preview basemap job (F-36 MC-8; AC 37, 38): no frame or a current map loads nothing; the
// rendered PNG goes out under the frame's key; every failure ends quietly; one 10 s budget; a
// newer job aborts the older.

import { afterEach, describe, expect, it, vi } from 'vitest';
import type { OwnerResult, PreviewFrame } from '../lib/owner';
import { BASEMAP_BUDGET_MS, basemapJobs, type RenderBasemap } from './basemap-job';

const KEY = '0123456789abcdef';
const FRAME = { lon: 11.5, lat: 48.1, zoom: 12.25 };
const PNG = new Blob([new Uint8Array([1])]);
const ok = <T>(value: T): OwnerResult<T> => ({ ok: true, value });

function setup(frame: OwnerResult<PreviewFrame>, render: RenderBasemap = async () => PNG, put: () => Promise<OwnerResult<true>> = async () => ok(true as const)) {
  const owner = {
    previewFrame: vi.fn(async (_s: AbortSignal) => frame),
    putPreviewMap: vi.fn(async (_k: string, _b: Blob, _s: AbortSignal) => put()),
  };
  const renderFn = vi.fn(render);
  return { owner, render: renderFn, start: basemapJobs(owner, renderFn) };
}

afterEach(() => vi.useRealTimers());

describe('basemapJobs', () => {
  it('renders the server frame and sends it under its key', async () => {
    const { owner, render, start } = setup(ok({ frame: FRAME, key: KEY, current: false }));
    await start();
    expect(render).toHaveBeenCalledWith(FRAME, expect.any(AbortSignal));
    expect(owner.putPreviewMap).toHaveBeenCalledWith(KEY, PNG, expect.any(AbortSignal));
  });

  it.each([
    ['no frame (indoor, fully hidden)', ok<PreviewFrame>({ frame: null })],
    ['a current map', ok<PreviewFrame>({ frame: FRAME, key: KEY, current: true })],
    ['a failed GET', { ok: false, notFound: false, message: 'x' } as const],
    ['not found', { ok: false, notFound: true } as const],
  ])('%s: nothing rendered or sent', async (_name, frame) => {
    const { owner, render, start } = setup(frame);
    await start();
    expect(render).not.toHaveBeenCalled();
    expect(owner.putPreviewMap).not.toHaveBeenCalled();
  });

  it('a render failure (no WebGL2, tile error) ends quietly with no PUT', async () => {
    const { owner, start } = setup(ok({ frame: FRAME, key: KEY, current: false }), async () => {
      throw new Error('no WebGL2');
    });
    await expect(start()).resolves.toBeUndefined();
    expect(owner.putPreviewMap).not.toHaveBeenCalled();
  });

  it('a refused or failed PUT ends quietly', async () => {
    for (const put of [async () => ({ ok: false, notFound: false, message: 'refused' }) as const, () => Promise.reject(new Error('net'))]) {
      await expect(setup(ok({ frame: FRAME, key: KEY, current: false }), undefined, put).start()).resolves.toBeUndefined();
    }
  });

  it('ends after the budget even if the render never settles, and aborts it', async () => {
    vi.useFakeTimers();
    let seen: AbortSignal | null = null;
    const { owner, start } = setup(ok({ frame: FRAME, key: KEY, current: false }), (_f, s) => ((seen = s), new Promise<Blob>(() => {})));
    let done = false;
    void start().then(() => (done = true));
    await vi.advanceTimersByTimeAsync(BASEMAP_BUDGET_MS - 1);
    expect(done).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(done).toBe(true);
    expect(seen!.aborted).toBe(true);
    expect(owner.putPreviewMap).not.toHaveBeenCalled();
  });

  it('a newer job aborts the older', async () => {
    const signals: AbortSignal[] = [];
    let release = (): void => {};
    const { start } = setup(ok({ frame: FRAME, key: KEY, current: false }), (_f, s) => {
      signals.push(s);
      return signals.length === 1 ? new Promise<Blob>(() => {}) : new Promise<Blob>((r) => (release = () => r(PNG)));
    });
    const first = start();
    await vi.waitFor(() => expect(signals).toHaveLength(1));
    const second = start();
    await first;
    expect(signals[0].aborted).toBe(true);
    await vi.waitFor(() => expect(signals).toHaveLength(2));
    release();
    await second;
    expect(signals[1].aborted).toBe(false);
  });
});
