// The preview basemap job (F-36 MC-8, ADR-041; OQ-112). Asks the server for the public route's
// frame, renders the basemap for it and sends it under the frame's key. Nothing is shown and
// nothing is logged: any failure leaves the server-made card (SH-12). The basemap module is a
// lazy chunk reached only from here.

import type { OwnerClient } from '../lib/owner';
import type { Frame } from '../components/map/basemap';

/** MC-8: one budget for GET, render and PUT. */
export const BASEMAP_BUDGET_MS = 10_000;

export type RenderBasemap = (frame: Frame, signal: AbortSignal) => Promise<Blob>;

const lazyRender: RenderBasemap = (frame, signal) => import('../components/map/basemap').then((m) => m.renderBasemap(frame, signal));

/** Returns the starter of jobs: at most one runs per page, a newer one aborts the older. Each
 *  job's promise resolves when it ended, at the latest after the budget; it never rejects. */
export function basemapJobs(owner: Pick<OwnerClient, 'previewFrame' | 'putPreviewMap'>, render: RenderBasemap = lazyRender): () => Promise<void> {
  let current: AbortController | null = null;
  return () => {
    current?.abort();
    const c = (current = new AbortController());
    const timer = setTimeout(() => c.abort(), BASEMAP_BUDGET_MS);
    const ended = new Promise<void>((resolve) => c.signal.addEventListener('abort', () => resolve()));
    const work = (async () => {
      const got = await owner.previewFrame(c.signal);
      // No position in the public route, or a stored map already fits: MapLibre is not loaded.
      if (!got.ok || got.value.frame === null || got.value.current) return;
      const png = await render(got.value.frame, c.signal);
      if (!c.signal.aborted) await owner.putPreviewMap(got.value.key, png, c.signal); // refusals are ignored
    })().catch(() => {});
    return Promise.race([work, ended]).finally(() => clearTimeout(timer));
  };
}
