# QA F-62 Show my location on the map - round 1

- **Date:** 2026-11-21
- **Verdict:** ready with conditions - no Blocker or Major. Conditions: the browser and phone hand checks below (real network log, Copy link through Caddy, a stationary watch, the dot on the phone) and the r1-1 test gap (a pan alone is not tested to stop follow mode) in the small lane.
- **Inputs:** `features/F-62-show-my-location.md`; `architect/F-62-show-my-location.md` (LC-1 to LC-12); F-22 H-2, H-5 notes; ADR-011, ADR-013, ADR-038; spec AP-25, AP-3, AP-15, AP-19, SH-5, SH-12, DS-6, INF-1, NFR Privacy; branch `worktree-agent-a38203a31be5117b4` at 98aac9f (diff 39f4ac7..98aac9f), clean tree.

## Build and test results

- `make ci` in the worktree: exit 0. Go: every package `ok`. `svelte-check`: 0 errors, 0 warnings. Vitest: 90 files, 2,278 tests passed. `caddy-check` against Caddy v2.11.9: `ok`. `docker-check: ok`. `update-check`: ok.
- Size: activity entry 60,293 / 62,000 bytes gzip (+12 against the design's 60,281; ceiling 50), reserve 1,707. Location chunk `location-*.js` 1,120 bytes gzip (above LC-1's 0.5 KB fold trigger, so a separate chunk is justified); map chunk 8,572 gzip, map CSS 1,050 gzip. Map texts land in `lang-page-*.json` (2,443 to 2,510 / 5,120).
- Built location chunk scanned: no `fetch`, `console`, storage, `cookie`, `history`, `sendBeacon`, `postMessage`, `import(` or URL literal; no import of another chunk.
- Mutants (in a scratch copy of `src/web`, F-62 test files run each time, baseline 64/64 green):

| Mutant | Result |
| --- | --- |
| `movestart` listener removed (a pan no longer stops follow) | **survived** (r1-1) |
| `zoomstart` listener removed | killed |
| resize or our own move stops follow (`originalEvent` check dropped) | killed |
| watch kept while hidden | killed |
| `pagehide` only clears the watch (dot and state stay) | killed |
| "one watch" guard dropped | killed |
| one `import()` per tap (two quick taps, two instances) | killed |
| insecure-context check dropped | killed |
| `console.debug` in the error path | killed |
| first centring does not end the D-20 refit | killed |
| controls on every page (`if (options.locate)` forced) | killed |
| `locate: !edit` forced `true` (edit page) | killed |
| teardown does not stop location | killed |
| message never clears | killed |
| `visibilitychange` / `pagehide` listeners not removed on stop | **survived** (r1-2) |
| `destroyed` guard after the chunk loads dropped | **survived** (r1-2) |
| location buttons not relabelled on a language change | **survived** (r1-2) |

Caddy: the H-12 check compares every `securitySet` header as exactly one value equal to `geolocation=(self)` on every route class, so a second feature or a second header fails (read, not mutated). The structural LC-10 check (comment-stripped source regex) covers storage, `lib/prefs` and console mutants by construction.

No real browser is available here, so the design's "QA in a real browser with an emulated, moving position" check was not run; it is a hand check below.

## Findings

### Minor

- **ID:** QA-62-r1-1
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/components/map/location.dom.test.ts:149`; code `src/web/src/components/map/location.ts:113`
- **Relates to:** AC "a pan, drag or zoom by the viewer stops follow mode"; LC-4
- **Evidence:** deleting `map.on('movestart', onMove)` leaves all 64 F-62 tests green. The test stops follow with `zoomstart` first, then fires a viewer `movestart` while follow is already off, so the pan path is never asserted on its own.
- **Consequence:** a drag or pan (the commonest gesture on a phone) could stop stopping follow mode without a failing test. The code is correct today. Fix: one test where only a `movestart` with `originalEvent` stops follow mode.

- **ID:** QA-62-r1-2
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `location.dom.test.ts:260`; `location.ts:130-131`; `map-view.ts:449`; `map-view.ts` `onLanguage` (`locLabel()`)
- **Relates to:** LC-9 (teardown, "no listener is left"), LC-7 (one watch), LC-12 (relabel)
- **Evidence:** three mutants survive: (a) the teardown test's assertion cannot fail, because after `stop()` the phase is `off` and `rewatch` returns early whether or not the listeners are still attached; (b) removing `if (destroyed) return null` is not caught: a tap whose chunk resolves after unmount would start a watch that nothing clears (`stopLocation` is assigned after teardown ran); (c) a language change no longer relabelling the two buttons is not caught.
- **Consequence:** (b) is the one with a privacy edge: a watch (and the browser's location indicator) left running after the map is gone. Code is correct now; the guards are unprotected.

- **ID:** QA-62-r1-3
- **Severity / kind:** Minor / risk
- **Route:** frontend
- **Where:** `location.ts:36` (`timeout: 30_000` on `watchPosition`)
- **Relates to:** LC-7, LC-8; AC "an error after the dot is shown gives the same message once"
- **Evidence:** read only, not reproduced (no browser here). Engines differ on whether a watch's `timeout` re-arms after each fix; where it does, a viewer standing still for 30 s gets a TIMEOUT error.
- **Consequence:** the viewer sees "Your location is not available." while the dot is fine. Handled gracefully (once per turning on, dot and watch kept), so only a misleading message. Settled by the hand check "stand still"; if it shows, drop `timeout` after the first fix or ignore code 3 once a fix exists.

- **ID:** QA-62-r1-4
- **Severity / kind:** Minor / defect (accessibility)
- **Route:** frontend
- **Where:** `src/web/src/components/map/map-view.ts:436-441`
- **Relates to:** LC-8 (`role="status"`)
- **Evidence:** the message element is created with `role="status"` and its text in the same step and then inserted. Live regions are announced reliably only when they exist before their content changes; a region inserted already filled is often silent (NVDA, VoiceOver).
- **Consequence:** a screen-reader user may not hear "Location access is not allowed." Fix: keep one empty status element in the map box and set its text.

- **ID:** QA-62-r1-5
- **Severity / kind:** Minor / wording
- **Route:** frontend (user owns the wording, AB-2)
- **Where:** `src/web/src/i18n/de.json` `ab_ifYouTurn`; `fr.json` `ab_ifYouTurn`
- **Relates to:** INF-1, DS-6
- **Evidence:** German says "auf der Karte eines Workouts"; every other About text says "Training" (24 times). French says "d'une séance"; the rest says "entraînement" (23 times).
- **Consequence:** inconsistent terms on one page. Suggest "eines Trainings" and "d'un entraînement" when the user approves the wording.

- **ID:** QA-62-r1-6
- **Severity / kind:** Minor / documentation hand-off
- **Route:** architect (requirements engineer for the TD)
- **Where:** `spec/Workout Explorer — Technical Design.md:464` (Headers); `architect/ADR-011-*.md` item 4
- **Relates to:** F-62 design Hand-offs and Cost rows
- **Evidence:** the TD headers list does not name `Permissions-Policy: geolocation=(self)`; ADR-011 has no F-62 measured row yet.
- **Consequence:** the system-wide header list is stale. Values for the ADR-011 row: entry 60,293, F-62 share +12, reserve 1,707.

## Engineer's deviations

| Deviation | Judgement |
| --- | --- |
| Chunk returns `{tap, follow, stop}` instead of only `stop` | Accepted. None of the three returns or carries a position; the seam back to map-view stays position-free (LC-10). |
| `say(key)` callback for messages | Accepted. Only a key crosses the seam; texts stay in the map text group; never `error.message`. |
| Two message texts, cleared after 4 s | Accepted (denied vs everything else matches LC-8); see r1-4 for the live region. |
| `jumpTo` with zoom `max(current, 16)` | Accepted: no animation to interrupt, no `flyTo`, a closer viewer zoom is kept. A follow tap after zooming out returns to 16; user checks on the phone. |
| `enableHighAccuracy`, `maximumAge` 10 s, `timeout` 30 s | Accepted with r1-3 (timeout while stationary) as a hand check. |
| About wording | Engineer's draft; user approves. See r1-5. |
| protoserve lacks the header | Accepted: without a `Permissions-Policy` the geolocation default allowlist is already `self`, so behaviour is the same. Consequence: Copy link with the header must be checked through Caddy, not on :18080. |
| ADR-011 measured row deferred to QA close | Fine; numbers in r1-6. |

## Previous findings

None (round 1).

## Questions

None that block. r1-3 is settled by the stationary hand check.

## Coverage

| Acceptance criterion | Code | Test / check |
| --- | --- | --- |
| Button on the viewing page and fullscreen; none on edit or without a map; follow only while on | `map-props.svelte.ts` `locate: !edit`; `map-view.ts` `if (options.locate)`; group inside the map container | `map-props.dom.test.ts`, `RouteMap.dom.test.ts`, `location.dom.test.ts` (controls, follow hidden). Fullscreen by structure (same container, ADR-038); no-map cases by AP-3 (no map mounted). Hand check for fullscreen |
| Off at every load; no Geolocation call before the tap | lazy `import('./location')` on click | `location.dom.test.ts` (no `watchPosition`/`getCurrentPosition` before tap); built entry has no location code |
| Tap: dot with accuracy, centre, follow | `location.ts` `send`, `size` | dot, circle size formula, centre, follow on a moving fix: tested |
| Viewer pan/drag/zoom stops follow; own moves do not | `onMove` on `movestart`/`zoomstart` with `originalEvent` | zoom and "own move/resize" tested; pan alone not (r1-1) |
| Follow re-centres every tap | `locStep` `follow` | tested (twice in a row, zoom kept) |
| Second tap: off, no dot, no follow button, map stays | `locStep` `tap` | tested incl. late callback |
| Centres only on turn-on, follow tap, follow mode | `locStep` `centre` | exhaustive state x event table test |
| Hidden/closed stops updates | `onVisibility`, `onPageHide` | tested (clearWatch, resume, pagehide off, teardown) |
| Errors: message, off, map unchanged; after a fix once | `locStep` `denied`/`failed`; `say` | tested codes 1, 2, 3, insecure, chunk import failure, once rule, self-clearing |
| No position to server, storage, URL, range link, console | LC-10 boundaries | jsdom run-time test (fetch, Storage, history, cookie, href, console spies) and source check; built chunk scanned. Real network log: hand check |
| Preview images without position | `basemap.ts` and owner glue never import location; SH-5 server side | structural test |
| `Permissions-Policy: geolocation=(self)` on every response, nothing else | `deploy/Caddyfile` security snippet | `caddy-check` passed (exact single value on every route class). Copy link: hand check through Caddy |
| About sentence; no hint on the viewing page | `about.html` `ab_ifYouTurn` in en, de, es, fr | i18n completeness test in `make ci`; read; no text on the viewing page besides button names and messages |
| Texts in four languages | `map-texts.ts`, `de/es/fr.json` | LG-8 completeness test passes; relabel untested (r1-2) |
| LCP and entry unchanged; 60 fps scrubbing with location on | entry +12 bytes; markers outside the scrub path | size check passes; 60 fps not measured here: hand check |
| Phone check | - | user |

Not run: a real browser (none installed here), the 60 fps scrub measurement, the corpus (no parser change).

### Hand checks for the user

1. Through Caddy (not protoserve) on the phone: turn location on, walk; DevTools or a proxy shows only tile requests and one `/assets/location-*.js` request with no query; storage and cookies empty; URL unchanged; console empty; Copy link still copies.
2. Stand still with location on for over a minute: no "location not available" message (r1-3).
3. Pan with one gesture (no zoom): follow stops; tap follow: re-centres and follows again. Same in the fullscreen map, portrait and landscape; enter and leave fullscreen keeps dot and follow.
4. Switch apps and come back: the dot resumes without a tap; reload: location off.
5. Dot visible over the route in light and dark theme and on Satellite; scrubbing the 12-hour ride stays smooth with location on.
