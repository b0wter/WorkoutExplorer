# QA F-57 GPX route download - round 2

- **Date:** 2026-11-21
- **Verdict:** not ready, on one point only: the code writes no `<sym>`, but PIN-9 and the feature (Scope, AC 4) require "symbol and type = pin type". GX-10 dropped `<sym>` in the same commit (QA-57-r2-1). This needs one user answer and no privacy work. The privacy side is clean. Only the public pins file can reach the GPX, positions come from the public data, escaping holds, and the output without pins is byte-identical.
- **Inputs:** `features/F-57-gpx-route-download.md` (updated 21 Nov); `architect/F-57-gpx-route-download.md` GX-10 (GX-2, GX-3, GX-6, GX-7); spec SH-13, PIN-9, INF-1; `src/server/internal/publish/pins.go` (`publicPins`), `src/server/internal/lifecycle/pins.go` (text rules), `deploy/Caddyfile` (`/w/` cache headers). Diff only: `git show 763fe0a` on a clean tree.

## Build and test results

- `make ci` at 763fe0a: **exit 0**. Go: every package ok. Vitest: 90 files, **2,294 passed**. Size: `entry index.html: 60408 / 62000 bytes gzip PASS`. `assets/gpx-*.js` is 1,652 raw, **975 gzip** (r1: 651), under "dynamic imports (not counted)". caddy-check, docker-check and update-check ok.
- Scratch builds of `src/web` at `763fe0a~1` and `763fe0a` (from `git archive`; the repo is untouched): entry `main-*.js` **56,198 → 56,263 gzip** (+65), CSS unchanged. The minified diff is analysed under QA-57-r2-3.
- Scratch probe through `readPins` and `gpxParts`. The pins file was written as Go writes it (`<` and similar escapes), with an injection title `</name><wpt lat="0" lon="0"/> &amp; ]]> "q" 'a'`, a note with a line feed and `<!-- -->`, a section with an empty title, a non-ASCII title with an emoji, and an end at a NaN sample. Result: all markup is escaped, no extra element, the NaN end is skipped, and the empty title gives "Difficult section (start)/(end)". `xmllint --schema` with r1's GPX 1.1 transcription, `wptType` extended with the official `name, cmt, desc, src, link, sym, type, extensions` order: **validates**, both with and without pins. A title with U+FFFF fails (QA-57-r2-2).
- Mutant in the scratch copy: removing `...pinsUrl(root)` from `startPage`. **Survives:** 2,283 passed. The 1 failure, plus 1 file that failed to load, are environmental: the scratch copy has no `deploy/`, and the baseline shows the same.

## Findings

### Major

- **ID:** QA-57-r2-1
- **Severity / kind:** Major / defect (unresolved deviation from a requirement)
- **Route:** user decision via main session; then architect (GX-10) or requirements (PIN-9, F-57 Scope and AC 4), and frontend only if `<sym>` is to be added
- **Where:** `src/web/src/lib/gpx/index.ts:39` (`waypoints`); `architect/F-57-gpx-route-download.md` GX-10; spec PIN-9; `features/F-57-gpx-route-download.md` Scope bullet 4 and AC 4
- **Relates to:** PIN-9, F-57 AC 4 ("`<name>` …, `<sym>` and `<type>` (pin type)")
- **Evidence:** the same commit writes "symbol and type = pin type" into PIN-9 and the feature, and "no `<sym>` … an unknown `<sym>` can show a wrong icon" into GX-10. The code follows GX-10. No test asserts `<sym>` either way, and the DOM test pins the exact string `<name>Hi</name><type>note</type>`.
- **Consequence:** AC 4 cannot be traced as written. The spec, the source of truth, and the code disagree on what the public file contains. GX-10's reason is sound: Garmin and other apps map `<sym>` to their own symbol names, and `danger` or `water_food` are not among them. **Suggestion:** keep the code and remove "symbol" from PIN-9, Scope and AC 4. This is a small-lane text change, after which this finding closes without another QA round.

### Minor

- **ID:** QA-57-r2-2
- **Severity / kind:** Minor / defect
- **Route:** backend (or frontend escape)
- **Where:** `src/server/internal/lifecycle/pins.go:65-77`; `src/web/src/lib/gpx/index.ts:36` (`esc`)
- **Relates to:** AC 2 (valid GPX 1.1)
- **Evidence:** the server refuses only `unicode.IsControl` characters. The XML 1.0 `Char` production also excludes U+FFFE and U+FFFF, which pass both the server check and `esc`. In the probe, a title with U+FFFF makes `xmllint` reject the whole file as not well-formed.
- **Consequence:** an owner who pastes such a character breaks the GPX for every viewer of that workout. Strict importers refuse the file, and nothing tells anyone why. The chance is tiny and only the owner can cause it. The fix is either to refuse U+FFFE and U+FFFF next to the control-character rule, or to replace them in `esc`.

- **ID:** QA-57-r2-3
- **Severity / kind:** Minor / risk (size), with a partial preference
- **Route:** frontend
- **Where:** `src/web/src/page/start.ts:121-124, 272`; `src/web/src/gpx/index.ts:6`
- **Relates to:** AC 8 (entry within budget), GX-6, GX-10 ("no pins file means … no pins code")
- **Evidence:** the +65 gzip in the entry has two causes, shown in the minified diff:
  1. The `pinsUrl` helper and its call, which add a second `querySelector('.pins-slot')`. `pinSlot` from line 139 already holds that element. Replacing the helper with `pinsUrl: pinSlot?.dataset.url` measured **56,236** (−27 gzip). Reading the URL inside the chunk from `button.ownerDocument` would remove those bytes too.
  2. The gpx chunk now imports `lib/pins` statically, so its `import()` changed from `Ji(..., [])` to `__vite__mapDeps([16,17,10])`, with two new names in the entry's preload list (about 38 gzip). This part is intrinsic and acceptable.

  The same static import means that on a workout **without** a pins file, a click also loads the shared `pins-*.js` chunk (1,093 gzip) and `text-*.js`. No pin code runs, but the code does load, contrary to GX-10's wording. Accept it and fix the wording, or load `lib/pins` with `import()` only when `pinsUrl` is set.
- **Consequence:** 27 avoidable entry bytes. The entry has 1,592 bytes of headroom.

- **ID:** QA-57-r2-4
- **Severity / kind:** Minor / defect (document)
- **Route:** requirements (feature file)
- **Where:** `features/F-57-gpx-route-download.md` AC 7
- **Relates to:** AC 7, GX-10
- **Evidence:** AC 7 still says "Downloading requests no workout data (… only the download code itself may load …)". The click now calls `fetch(pinsUrl)`. In practice the hashed `pins.{hash}.json` is `private, max-age=31536000, immutable` (`deploy/Caddyfile:105`) and the pins chunk fetched it at page load, so the request is answered from the browser cache and never reaches the server. That is still a request, and the AC does not mention it.
- **Consequence:** AC 7 contradicts GX-10 as written. Add "except the public pins file, normally from the browser cache".

- **ID:** QA-57-r2-5
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/page/start.ts:272`; `src/web/src/gpx/gpx.dom.test.ts`
- **Relates to:** AC 4, GX-10 (source = the pins slot's `data-url`)
- **Evidence:** the start.ts mutant survives (see above). The tests call `downloadGpx` with a hand-made `pinsUrl` and never go from a real viewing page with a `.pins-slot[data-url]` through `startPage` and `enhanceGpx`. The privacy direction is covered: `!edit` was tested in r1, the edit page's slot has no `data-url`, and only the server's `publicPins` output is in the file. The missing test is the positive one: a pins page whose download contains its waypoints.
- **Consequence:** a refactor of `startPage` can silently drop the waypoints.

- **ID:** QA-57-r2-6
- **Severity / kind:** Minor / risk
- **Route:** frontend
- **Where:** `src/web/src/gpx/index.ts:13-21`
- **Relates to:** AC 9 (under 2 s), GX-10 ("a failed … file gives the route alone")
- **Evidence:** `fetch` has no timeout. It only matters when the cached copy is missing, for example because the page-load fetch failed, and the network then stalls. In that case the download waits for the browser's own timeout, which can be minutes, with no feedback. Not reproduced, from reading the code only.
- **Consequence:** in a rare case the button appears to do nothing for a long time. Use `AbortSignal.timeout(…)`, which already counts as "failed → route only".

- **ID:** QA-57-r2-7
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/web/src/page/start.ts:120-121`
- **Evidence:** the helper was inserted between `/** Enhances the server markup, then loads and mounts. Never rejects. */` and `startPage`, so the doc comment now attaches to `pinsUrl`. QA-57-r2-3(1) removes the helper, which would fix this too.

## Previous findings

This round is diff-only. None of r1's findings (QA-57-r1-1 to r1-6) is touched by 763fe0a, so they were not re-verified. QA-57-r1-1 was fixed before this commit (test `QA-57-r1-1: no GPX line for %s` in `edit.dom.test.ts`).

## Questions

1. **`<sym>` (QA-57-r2-1).** Should the waypoints carry `<sym>` = pin type, as PIN-9 and AC 4 say, or not, as GX-10 says? Suggestion: no `<sym>`, with PIN-9 and AC 4 adjusted to match. The answer settles r2-1.
2. **(a) Async download.** Is the gap between the click and the download new? Not in kind. Before, `lazyClick` already did `load().then(open)`, so the anchor click always came after a promise, and after a network round trip for the chunk on the first click. The new `fetch` adds a second await on every click. It is normally a cache hit of a few milliseconds, so the first click is not materially later than before. Safari's transient-activation window has always been the exposure here. Settle it by hand: iPhone Safari and macOS Safari, a workout with pins, a cold page load, first and second click. Pass means the download, or Safari's download prompt, appears without an "allow downloads" block.

## Coverage

- **Only public pins, never edit-page or dimmed pins:** traced. `startPage` passes `pinsUrl` only under `!edit`, from the viewing page's `.pins-slot[data-url]`; the edit slot has no `data-url` (`ActivityPage.svelte:33-34`). The file is the server's `publicPins` projection (`Public && !Outside && !Merged`, indices relative to the public data). F-56 tested this; it is not part of this diff.
- **Nothing in hidden or trimmed parts:** positions are `lat[i]`/`lon[i]` from the decoded public data file, never from the pin. A NaN end is skipped (tested). `readPins` checks indices against `latitude.length`, the same check the viewer uses against `sampleCount`.
- **XML escaping and injection:** traced and tested (`& < >`). The probe above confirms no element injection. `<type>` is restricted to the `PIN_TYPES` enum by `readPins`. Quotes need no escaping in element content.
- **No times or extensions:** tested (`not.toMatch(/<time|<extensions/)`).
- **Byte-identical without pins:** tested (`build([])` equals `build()`). The r1 GX-3 element-set test still passes.
- **Failure leads to the route alone:** a rejected fetch is tested. A non-ok response and invalid JSON lead to `undefined` by reading the code.
- **No fetch without a pins file:** tested (`fetch` not called without `pinsUrl`). Pin code is still loaded in that case (QA-57-r2-3).
- **(b) Entry +67:** explained in QA-57-r2-3; 27 bytes avoidable.
- **(c) Texts:** English, `about.html`, `edit/texts.ts` and the de, es and fr `gpxNote` and `ab_viewersOfA` say "the track and its public pins" consistently. They use each language's existing term for pins: de "Pins", es "marcadores", fr "repères". No "track only" wording is left. The About and edit tests are updated.
- **Not run:** devices (AC 2 and AC 9 for files with pins, and question 2). The official, untranscribed GPX XSD was not run because that would need an external fetch.
