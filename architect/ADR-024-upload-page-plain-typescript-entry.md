# ADR-024 Upload page: its own entry in plain TypeScript, sharing no chunk with the activity page

- **Status:** accepted (23 Oct 2026, with the F-11 design; see the decision log in `features/README.md`)
- **Date:** 23 Oct 2026
- **Features:** F-11 (introduces it); F-13, F-17 (the activity entry it protects); F-29 (the About page may follow the same rule)
- **Supersedes:** none. Refines TD Tech stack ("Front end: Svelte 5 + Vite + TypeScript") for the upload page only; works within [ADR-002](ADR-002-bundle-size-budget-measurement.md) and [ADR-011](ADR-011-activity-page-bundle-allocation.md) item 3
- **Requirements:** UP-1, UP-8, NFR Performance, NFR Compatibility; TD Front end (Budget)

## Context

ADR-011 item 3 gives the upload page its own HTML entry and its own 60,000 bytes, and forbids `uplot` and `lib/charts` in it. It does not say what the page is written in. The page is small: a status check, a drop zone with a file picker, one upload with progress, a message line, a footer link. About six states and no shared data with the activity page.

In a multi-entry Vite build, every module that both entries import moves into a shared chunk. ADR-002 counts that chunk in both entries. If the upload page used Svelte, the Svelte runtime parts used by both pages would move out of the activity entry into a shared chunk. That costs gzip overhead (two streams instead of one) and adds a request on the activity page. ADR-011 has a reserve of 1,418 bytes, and F-13 and F-17 still have to be measured.

## Decision

1. **Second HTML entry `src/web/upload.html`.** `index.html` stays the activity page entry (F-07 D-1, `tools/protoserve`, ADR-011's CI line).
2. **Plain TypeScript and CSS, no Svelte and no runtime dependency.** Pure logic goes in `src/lib/upload/` (Node tests). DOM glue goes in `src/upload/` (jsdom smoke tests, ADR-012, as `src/prototype/` does).
3. **The two entries share no chunk.** Their counted sets in the Vite manifest are disjoint. The activity entry's CI line does not grow because of F-11. Code that Vite adds to every entry and would then share (the module-preload polyfill is the likely case) is removed or kept per entry. Every browser in the compatibility list supports `modulepreload` natively.
4. **Ceiling: 8,000 bytes gzip for the upload entry** (ADR-002 counting). Above it, the engineer stops and reports. A page of this size in plain TypeScript should be a few KB, so a jump past 8,000 means a framework or dependency has crept in.

## Consequences

- The activity page's budget is not touched by F-11. The upload page downloads a few KB, not the roughly 11 KB of the Svelte runtime, which helps on a phone.
- There are two ways of writing UI in `src/web`. This is accepted for one small page, and the pure/DOM split keeps it testable.
- QA reads the disjoint counted sets and both CI lines from the manifest and the size check output.

## Would choose differently if

- The upload page grows into a real view, for example the "remember my uploads" list (Later). Then use Svelte, measure the shared-chunk cost against ADR-011's reserve at that time, and record it in a superseding ADR.
- The activity page gets clear budget slack (a superseded ADR-002 measuring brotli, or a raised budget). Then consistency may be worth the shared chunk.
