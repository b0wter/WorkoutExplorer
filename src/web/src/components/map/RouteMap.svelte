<script lang="ts">
  // The route map's static glue (F-06 design, "Route presence", "Loading and start-up";
  // ADR-013). STATIC: only routePresence, the hint, the container, the touch-quiet gate and
  // the import() of the lazy adapter. MapLibre is reached only through import('./map-view').
  //
  // Wiring (F-07, bench/map.html): <RouteMap {data} {cursor} load={chartsReady} /> with the
  // same cursor store as ChartStack; `load` turns true from ChartStack's onready (AC 1).
  import { untrack } from 'svelte';
  import type { Attachment } from 'svelte/attachments';
  import type { WorkoutData } from '../../lib/datafile';
  import type { CursorStore } from '../../lib/cursor';
  import type { SelectionStore } from '../../lib/selection';
  import { routePresence } from '../../lib/route';
  import { createTouchQuiet } from './touch-quiet';
  import type { Scheme } from './map-style';

  interface Props {
    /** One map per data value: a new value destroys the old map and builds a new one. */
    data: WorkoutData;
    /** The same store as the charts (one per WorkoutData, ADR-008). */
    cursor: CursorStore;
    /** The range selection (F-17 R-10): highlighted on the route; the map never writes it. */
    selection?: SelectionStore;
    /** false until the page wants the map; MapLibre is requested only after it turns true.
     *  Turning false again never unloads the map. */
    load?: boolean;
    /** Container height in CSS px, default 320; read once per data value. Ignored with `fill`. */
    height?: number;
    /** The container fills its parent, whose CSS owns the height (F-13 design D-20). */
    fill?: boolean;
    /** Colour scheme (F-14 D-10, D-12): picks the style at load; a change repaints and swaps the style in place. */
    scheme?: Scheme;
    /** Overrides the scheme's remote style URL (tests, bench). */
    styleUrl?: string;
    /** Called once per data value, after the map exists and the marker follows the cursor. */
    onmapready?: () => void;
    /** Inclusive sample ranges marked as to-be-hidden (F-19 E-9, ADR-030); edit page only. */
    hidden?: readonly (readonly [number, number])[];
  }

  let { data, cursor, selection, load = false, height, fill = false, scheme = 'light', styleUrl, onmapready, hidden }: Props = $props();

  const MAP_HEIGHT = 320;

  // A wiring bug, not a data problem: one cursor store per WorkoutData (as ChartStack).
  $effect.pre(() => {
    if (cursor.sampleCount !== data.sampleCount) {
      throw new Error(`RouteMap: cursor store for ${cursor.sampleCount} samples, data has ${data.sampleCount}`);
    }
  });

  const presence = $derived(routePresence(data));

  // Runs once per container, i.e. once per data value ({#key data}). Everything it reads is
  // untracked, so only the removal of the container runs its cleanup; the nested effect
  // tracks `load` alone and never tears anything down.
  const attachMap: Attachment<HTMLDivElement> = (node) => {
    const d = untrack(() => data);
    const c = untrack(() => cursor);
    if (!untrack(() => fill)) node.style.cssText = 'height:' + (untrack(() => height) ?? MAP_HEIGHT) + 'px';
    // Created now, so it has seen every touch by the time the map may start (D-4).
    const quiet = createTouchQuiet(window);
    const abort = new AbortController();
    const signal = abort.signal;
    let handle: { setScheme(s: Scheme): void; setHidden(r: readonly (readonly [number, number])[]): void; destroy(): void } | null = null;
    let started = false;

    const start = async (): Promise<void> => {
      try {
        await quiet.whenQuiet();
        if (signal.aborted) return;
        const view = await import('./map-view');
        performance.mark('wx-map:imported'); // start-up table on bench/map.html (v1.4)
        if (signal.aborted) return;
        const h = await view.mountRouteMap({
          container: node,
          data: d,
          cursor: c,
          selection: untrack(() => selection),
          whenQuiet: quiet.whenQuiet,
          styleUrl: untrack(() => styleUrl),
          scheme: untrack(() => scheme),
          signal,
        });
        if (signal.aborted) {
          h.destroy();
          return;
        }
        handle = h;
        // A scheme change while the map was loading.
        h.setScheme(untrack(() => scheme));
        h.setHidden(untrack(() => hidden) ?? []);
        untrack(() => onmapready)?.();
      } catch (error) {
        if (signal.aborted) return;
        // No WebGL2, a failed chunk or a throw during mount: one line in the map's place
        // (OQ-16); the charts are untouched. Console only; the error holds no coordinates.
        node.textContent = 'The map could not be loaded.';
        queueMicrotask(() => {
          throw error;
        });
      }
    };

    $effect(() => {
      const next = scheme; // read first: the effect depends on it even while there is no map yet
      handle?.setScheme(next);
    });
    $effect(() => {
      const next = hidden ?? [];
      handle?.setHidden(next);
    });
    $effect(() => {
      if (load && !started) {
        started = true;
        void start();
      }
    });

    return () => {
      abort.abort();
      handle?.destroy();
      handle = null;
      quiet.dispose();
    };
  };
</script>

{#key data}
  {#if presence === 'route'}
    <div class="route-map" class:fill {@attach attachMap}></div>
  {:else if presence === 'no-location-data'}
    <p class="map-hint">No location data in this workout.</p>
  {/if}
{/key}

<style>
  .route-map {
    position: relative;
    overflow: hidden;
    background: var(--map-bg, #e8eaed);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--muted, #555);
    font-size: 14px;
  }
  .route-map.fill {
    height: 100%;
  }
  .map-hint {
    margin: 8px 0;
    color: var(--muted, #555);
    font-size: 14px;
  }
</style>
