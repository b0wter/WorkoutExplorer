# ADR-038 Fullscreen map as a CSS overlay with one history entry, never the Fullscreen API

- **Status:** accepted (3 Nov 2026), with the F-26 design v0.1
- **Date:** 3 Nov 2026
- **Features:** F-26 (introduces it); AP-19 (phase 2) must respect item 3
- **Supersedes:** none. Applies [ADR-015](ADR-015-map-interaction-hover-touch-nearest-sample.md) item 1's F-26 note
- **Depends on:** [ADR-013](ADR-013-maplibre-behind-a-lazy-adapter.md), [ADR-011](ADR-011-activity-page-bundle-allocation.md) item 2, [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md) (token in the fragment)
- **Requirements:** AP-15, AP-18 (pinned state survives), NFR Compatibility (iOS Safari), NFR Accessibility (Esc)

## Context

AP-15 wants the map over the whole screen, left by a button, Esc or the phone's back gesture, back at the same scroll position. iPhone Safari has no element Fullscreen API, so a CSS path is needed there anyway. Where the API exists (Android Chrome, desktop, iPad), it is a second path with different exits: the browser consumes Esc (no `keydown` reaches the page), and Android's back gesture leaves browser fullscreen first, so back behaves differently per platform. The page itself never uses history today.

## Decision

1. **One mechanism everywhere: a CSS overlay.** The map's own container is laid over the viewport (`position: fixed` over the whole viewport, no viewport units) through MapLibre's `FullscreenControl` with `pseudo: true`. `requestFullscreen` is never called, on any platform.
2. **The page under it is left alone.** Nothing is removed or resized: the rest of the document is hidden by `visibility` (inherited from `body`, restored on the map's container) and the document's scrolling is locked. No reflow means no scroll change, no chart resize and no change to pinned, sticky or split state.
3. **Back gesture = one history entry.** Entering pushes one same-document entry from inside the button's click (user activation, so Chrome's back-button intervention does not skip it), with no URL argument, so the URL and its fragment (edit token) are unchanged and the state holds nothing about the workout. `popstate` while fullscreen exits. Every other exit removes that entry with one `history.back()`; the entry is removed exactly once whichever way the viewer leaves.

## Consequences

- One layout path to test on every browser; Esc and back are under our control on all of them.
- **Cost:** on Android and desktop the browser's address bar and the system bars stay visible (OQ-100). The map fills the browser's viewport, not the physical screen.
- A reload while fullscreen leaves the extra entry in history: one back press then does nothing visible. Accepted.
- AP-19 (shareable range, phase 2) must write its fragment with `replaceState`, never `pushState`, or back stops matching item 3.

## Reversibility

Cheap: lazy map code only, nothing stored, no URL. Adding the browser API later (item "would choose differently") changes no layout.

## Would choose differently if

- The operator wants the bars gone on Android (OQ-100): request fullscreen on `document.documentElement` (not on the map) as an extra layer where available, keep this overlay as the only layout, and treat the browser's own fullscreen exit as one more exit path under item 3.
