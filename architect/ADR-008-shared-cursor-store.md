# ADR-008 Shared cursor store: a framework-free sample-index store, delivered once per frame

- **Status:** accepted
- **Date:** 11 Oct 2026; revision 1 (decision 5)
- **History:** [architect/history/ADR-008-shared-cursor-store.md](history/ADR-008-shared-cursor-store.md)
- **Features:** F-05 (introduces it); F-06 and F-25 (map marker, route hover), F-07, F-13, F-17, F-18 (readouts, selection)
- **Supersedes:** none
- **Depends on:** [ADR-007](ADR-007-decoded-workout-data-in-the-browser.md) (sample indices into `WorkoutData`)
- **Requirements:** AP-6; TD Front end, Implementation notes ("one shared cursor store (a sample index) drives charts, map marker and value readouts; no component talks to another directly"); NFR Performance (60 fps scrubbing on a 12-hour workout)

## Context

The TD fixes *what* the cursor is: one store holding a sample index. It does not fix *how* it delivers changes. The cursor is the hottest value in the viewer: pointer events fire up to 120+ times per second, and every change moves a line in up to six charts, the readout and (F-06) a map marker. Three questions follow:

1. **Svelte store or plain store?** Svelte's `writable` plus `$cursor` in markup runs Svelte's reactive graph and DOM diffing on every move.
2. **Synchronous or batched delivery?** Synchronous delivery runs every subscriber per pointer event, which can be several times per frame.
3. **What value?** An index, a time or a pixel. An index is the only one that every view (time axis, distance axis in F-17, map, laps) can turn into its own coordinate without a search.

## Decision

1. **Value:** a sample index into the `WorkoutData` arrays, or `null` for no cursor. Never a time, distance or pixel.
2. **A plain TypeScript store in `lib/cursor`** with `get`, `set` and `subscribe`, with no Svelte or DOM imports. It satisfies Svelte's store contract by shape, but components never use `$cursor` or feed it into `$state`/`$derived`. Subscribers write to the DOM themselves.
3. **Coalesced delivery:** `set` stores the value and schedules at most one flush through an injected `schedule` function (production: `requestAnimationFrame`). The flush delivers the latest value once, only if it differs from the last delivered one. `subscribe` calls the listener synchronously with the current value, so late subscribers (the map, loaded after the charts) are in sync immediately.
4. **One store per `WorkoutData`**, created by the page and passed down. Writers are input handlers only; listeners never call `set`.
5. **(rev. 1) Listener errors are isolated.** The flush calls each listener in its own `try`/`catch`. A throw goes to an injectable `report` function, and the other listeners still get the value. The default rethrows in a microtask, so the console and `window.onerror` see it unchanged. Listeners must not throw (review rule). The catch only stops one broken view, such as the map in F-06, from freezing the charts and readout, which would otherwise stay stale until the next move.

API and rules: [F-05 design](F-05-stacked-charts-with-shared-cursor.md), "Cursor store".

## Consequences

- Scrubbing costs one listener pass per frame, however many pointer events arrive, and no framework work. This is the precondition for the 60 fps target (F-05 AC 7, F-07 on the phone).
- New views join by subscribing (F-06 map, F-17 selection readouts) without touching chart code, which meets "no component talks to another directly".
- **Cost:** subscribers write to the DOM imperatively, outside Svelte's model. That code must be small, kept in the listener, and reviewed against the hot-path rules (F-05 D-10).
- Delivery is up to one frame late relative to `set`; invisible in practice, because pointer events are dispatched before that frame's animation callbacks.
- The store is testable in Node with a manual scheduler.

## Would choose differently if

- A view needs every intermediate value, not just the latest one per frame (for example a recorded scrub trail). Then add a separate synchronous event channel for it; do not change this store.
- Several workouts are shown side by side with a shared cursor (not in the spec). Then the value becomes a time and each workout maps it to its own index, in a superseding ADR.

## Reversibility

Cheap inside F-05. After F-06 and F-17 subscribe, the value type (index) is a published interface; changing it touches every view.
