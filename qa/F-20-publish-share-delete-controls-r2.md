# QA F-20 Publish, Share and delete controls - round 2

- **Date:** 2026-10-30
- **Verdict:** ready with conditions. No Blocker or Major finding. Conditions: (1) the operator runs the manual checks in the design's Verification table, because no browser is available here. These are the network log on desktop and phone (token only in request headers); page translation offered on the published page leaving the edit link untouched; through Caddy, a reload of the published page and Back from it to an edit page that shows the published state; a hiding-on ride opened at `/w/…` in another browser; and the iOS download. (2) The user reviews the new texts (OQ-63 rule): the heading "Workout published", the save warning, the copy-failure text, the draft line, the delete confirmation and the deleted screen.
- **Inputs:** `features/F-20-publish-share-delete-controls.md` (11 ACs) and the F-11, F-12, F-15 and F-19 feature changes; `architect/F-20-publish-share-delete-controls.md` v0.2 (C-1 to C-11, B-1); ADR-019 rev. 2 (item 8), ADR-020 rev. 2 (item 7), ADR-028 rev. 1, ADR-011 rev. 11, F-12 O-6, F-19 E-3 and E-7; spec AC-1, AC-4, AC-5, AC-6, SH-2, SH-6, SH-10, UP-4, UP-7; decision log of 30 Oct 2026 (all four blocks). This round covers the diff `f3a4106..9a2ae13` (c174c56, 5788e58, a57edc4, 91e57e5, 3f993fd B-1, b0e2281, 9a2ae13 frontend) on `master`, with a clean working tree.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci`, run as its steps | I did not run `make ci` itself, because its `npm ci` would reinstall `node_modules` under the running Vite dev server on :18080. Each step ran separately instead: `gofmt -l` empty; `go vet ./...` clean; `go test -count=1 ./...` 14 packages ok; `npm run check` 311 files, 0 errors, 0 warnings; `npm run test` 51 files, **1,328 passed**; `npm run build` ok; `npm run size` PASS. `build/web` was rebuilt and then restored byte for byte, so the dev server on :18081 is unchanged |
| `npm run size` | `index.html` and `edit.html` **56,367 / 60,000 B** gzip, PASS. That is **+7** over the F-20 start (56,360) and +5 over r1 (56,362), within the 100 B ceiling. The growth is the lazy chunk's new hash and name in the entry, with no new static code. `upload.html` 2,272 / 8,000 PASS. Owner chunk `start.svelte` 7,301 + CSS 856 = **8,157 B** gzip, limit 15,000 (r1: 7,629). The engineer's figures (56,367 and 8,157) are confirmed. Against the stated 56,361 baseline they say +5 for F-20; the design's baseline is 56,360, so F-20's total is **+7** |
| `go test -race -count=1 ./...` | 14 packages ok |
| `make corpus` | 55 ok, 0 diff, 0 fail; 6 local real files ok |
| Migration probe on real binaries (scratch data directory) | I built the pre-B-1 binary from `f3a4106` and uploaded 4 workouts: A stayed a draft, B was published, C was published and then made private, and E was published and then made private with hiding on (old API). On disk that gave `user_version` 2, 4 folders in `private/` and only B in `public/`. The HEAD binary then started on the same data: `user_version` 3, `CHECK (visibility IN ('draft', 'unlisted'))`, rows A (draft) and B (unlisted) unchanged, C and E gone, `private/` down to 2 folders (recovery: `orphans_removed: 2`), `public/` still only B. Results: `GET` A and B 200, C and E 404; `PATCH {"visibility":…}` 400 on both draft and unlisted, with the generic `invalid_settings` message; `PATCH {"title"}` 200; `POST publish` for C 404. Neither log has a token or a `public_id` |
| End to end through Caddy 2.10.2 (`deploy/Caddyfile`, scratch Go server :18095, Caddy :18096, HEAD web build in scratch) | `curl`: `/published/{id}`, `/edit/{id}`, `/published/` and `/published/x/y` return `edit.html` with 200, `Cache-Control: no-store` and `Referrer-Policy: no-referrer`; `/published` returns 404. A scratch vitest ran the real `startEdit` in jsdom against Caddy, and it **passed**. On the draft: no links, the draft line shown, `/w/{id}` 404. I ticked hiding and changed the title, then pressed Publish. The navigation went to `assign …/published/{id}#{token}` only after `/w/{id}` was 200 with the new title, and no sample in `hide_ends_ranges` has a position in the public data file. On the published page, the edit link is exactly one `translate="no"` text node, the token appears once in the HTML, the viewing link is `origin + /w/{id}`, the title is not shown, and Back to edit assigns the edit link. On the edit page once published: Share and Show links work (`assign` to the published page); no Publish button and no draft line. The published page of a second workout that is still a draft does `replace` to its edit page and renders nothing. A wrong token gives the generic page. Delete from the published page gives the deleted screen; after it, `/w/{id}` through Caddy is 404 with the generic text, both owner pages are generic, and the other workout is untouched. No request URL carries `#`, `?` or a token. The server log has internal IDs only |
| Mutants, front end (scratch copy; `vitest run src/edit/edit.dom.test.ts src/lib/owner src/components/edit src/upload src/lib/upload`) | **34 of 38 killed; 2 of the 4 survivors are equivalent.** All 12 front-end mutants in the design were killed, some in several forms: token in the viewing-link copy or the Share text; the edit link exposed on the edit page; the edit link in an attribute, an input value or a link; `translate="no"` dropped; publish without the pending `PATCH` or after a failed one; navigation before the publish `200` or after a 500; `assign` instead of `replace` on a draft, or the published page shown on a draft; delete without confirmation; the title left on the deleted screen; download outside the busy state; the token in the download request. My own mutants that were also killed: Back to edit with `replace`; publish navigating to the edit page; Share, the draft line (always or never) or the header Copy link on the wrong state; the published page loading `draft.bin`; a 500 shown as the generic page; the settings title used as the document title; copy-edit writing the viewing link; reload on every `pageshow`; `&new` kept by the upload page; `/w/` accepted as an owner path; `visibility` sent in a `PATCH`; Show links as an `href` with the token; the deleted screen on a 404; focus not moved to Cancel. **Survived:** the Share block condition without `!draft` and the edit link written as `{a}#{b}`, both equivalent (Svelte emits one text node). Also Back to edit and Show links enabled while busy, which is QA-20-r2-3 |
| Mutants, back end (scratch copy; `go test` on store, httpapi, publish, lifecycle, cmd) | **6 of 6 killed**: private rows copied as unlisted (design mutant), the `CHECK` still allowing private, the expiry index not recreated, `visibility` silently ignored in `PATCH`, `HidesMore` allowing any visibility change in place, a draft keeping its public folder |
| Probe: a change during a publish (scratch vitest) | With the publish request held, the "Hide start and end" checkbox is still enabled. Ticking it and then letting the publish answer sends no `PATCH`, and the page navigates to the published page. See QA-20-r2-1 |

## Findings

### Minor

- **ID:** QA-20-r2-1
- **Severity / kind:** Minor / risk (privacy, unlikely timing)
- **Route:** frontend
- **Where:** `src/web/src/components/edit/EditPanel.svelte:110-131` (the form fields have no `disabled` while busy); `src/web/src/edit/start.svelte.ts:143-151` (`publish` reads the patch at click time, then navigates)
- **Relates to:** C-2, C-3, OQ-85, F-19 E-9, UP-4
- **Evidence:** The probe above. Publish captures the form once. While the `POST` runs, every field stays editable, and E-9 draws the to-be-hidden marks as soon as the checkbox is ticked. On `200` the page leaves for the published page, and the change is dropped without a message. In r1 the panel stayed on screen and refilled from the response, so the lost change was visible. Now it is not.
- **Consequence:** An owner who ticks hiding while Publish is still running (a long file, a slow phone link) sees the grey marks and then lands on the published page. They have no sign that the start and end are public. Fix: disable the form's fields while `busy` (one `<fieldset disabled>` or the existing `busy` prop).

- **ID:** QA-20-r2-2
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect
- **Where:** `architect/ADR-011-activity-page-bundle-allocation.md:35`, `:60-61` (ledger)
- **Relates to:** ADR-011, the design's Budget table ("measured … in each QA round")
- **Evidence:** The ledger records F-20 at +2 (entry 56,362, reserve 3,638). Measured now: entry **56,367**, F-20 **+7** of its 100, reserve **3,633**.
- **Consequence:** The reserve that F-28 plans against is 5 B too high. This is a small figure, but the ledger is the one home of the number.

- **ID:** QA-20-r2-3
- **Severity / kind:** Minor / risk (test gaps)
- **Route:** frontend
- **Where:** `src/web/src/page/hygiene.test.ts:226-227`; `src/web/src/edit/edit.dom.test.ts` (no busy assertion for `back` or `links`)
- **Relates to:** F-19 E-3 (rev.), C-2, the engineer's deviation "Back and Show links disabled while busy"
- **Evidence:** (a) The hygiene scan still removes `history.replaceState(null, '', '#' + token)` from `lib/owner` before it checks for history writes. That code is gone, so the exemption is dead. If someone reintroduces exactly that call, it passes both this scan and the E-2 source scan, which skips `lib/owner` completely. (b) Mutants O14 and O19 (Back to edit, Show links enabled while busy) survive.
- **Consequence:** Today's code is correct. A regression in the critical module (an address rewrite) or in the declared busy behaviour would still pass CI. Fix: drop the exemption, and add `back` and `links` to the existing busy assertions.

## Engineer's deviations, judged

| Deviation | Judgement |
| --- | --- |
| Heading "Workout published" | Acceptable. It needs the user's review (condition 2). After a reload days later it still reads correctly |
| No SH-10 notice on the published page | Acceptable. SH-10 and C-9 place it in the edit panel, and AC 11 asks for the edit panel only. The published page is not that panel |
| Visibility shown as read-only text ("Draft" / "Unlisted") | Matches F-19 E-7 ("shown as text at most, never a control") and C-11. Mutant O17 (visibility sent) is killed |
| Share no longer opens by itself | Acceptable. Publish now navigates away, so the old auto-open has no moment to run. Share stays one click on the edit page |
| Back to edit and Show links disabled while busy | Acceptable; it avoids leaving during a delete. Not pinned by a test (QA-20-r2-3 b) |
| `startEdit` takes a `loc` parameter | Acceptable test seam. It defaults to `win.location`. Only `lib/owner` calls `assign` and `replace` on it (source scan) |
| `.edit-warn` style | Acceptable |
| Back end: `WorkoutChanged` and the recheck still remove the folder of any row that is not unlisted | Acceptable defence in depth. B6 shows a test pins it |
| Back end: generic 400 message for `visibility` | Acceptable. "only the settings that can be changed" is accurate and leaks nothing |
| Back end: `TestMigrationDropsPrivate` reads the 0001 and 0002 SQL files from disk | Acceptable. My probe on the real pre-B-1 binary covers the same path end to end |

## Previous findings

| Finding | Status |
| --- | --- |
| QA-20-r1-1 (no notice after a failed first settings request) | **Obsolete, closed.** The marker and the notice are gone (ADR-020 rev. 2, F-19 E-3). The edit link is shown only on the published page, which any later open shows again (AC-6) |
| QA-20-r1-2 (token mutants survive; download lock not pinned) | **Fixed.** The scan now reads `.value` of every `input` and `textarea`; the download test checks that the anchor's `href` is exactly the `blob:` URL; a busy test covers the download. Mutants D3b, D11 and D12 are killed |
| QA-20-r1-3 (delete confirmation not announced, no focus) | **Fixed.** `role="alert"`, and focus moves to Cancel (`DeleteControl.svelte`). Tested on both pages; mutant O21 is killed. Not checked with a screen reader |
| QA-20-r1-4 (C-2 lacks the download lock; ADR-011 has no F-20 entry) | **Fixed.** C-2 v0.2 records the lock, and ADR-011 rev. 11 has the F-20 row. The row is now stale again (QA-20-r2-2) |
| r1 Q1 (`translate="no"`) | **Settled** by the user (C-5) and built. Mutant D4 is killed. The browser behaviour is part of condition 1 |

## Questions

None blocking. For the main session: ADR-019 rev. 2, ADR-020 rev. 2, ADR-028 rev. 1 and the F-20 design v0.2 are still marked "draft" although they are built. Mark them accepted when this round closes.

## Coverage

| AC | Code | Test / check |
| --- | --- | --- |
| 1 Fresh draft: no links, no Share, no Copy link, no notice | `EditPanel` `{#if !draft}`, header `copy` getter, `lib/owner` `editLink` null on `/edit/` | "a draft: no link anywhere"; owner unit test; mutants D2, O3, O7; e2e |
| 2 Draft line, gone after publishing | `EditPanel` `{#if draft}` (C-10) | Draft and published tests; mutants O5, O6; e2e |
| 3 Publish: page live, then `/published/…#token`; token only as a header | `start.svelte.ts` `publish`, `lib/owner` `toPublished` | "without changes … assign"; mutants D7, D7b, O2; e2e (`/w/` 200 before navigation; request URLs clean). The browser network log is condition 1 |
| 4 Save first; failure stays | `publish` (OQ-85) | PATCH-before-POST, failed PATCH, 500 and 404 tests; mutants D5, D6; e2e hiding check. QA-20-r2-1 covers edits made during the publish |
| 5 Published page: both links with Copy, warning, Back to edit | `PublishedView.svelte`, `startPublished` | Published-page tests (clipboard exact, fallback, Back `assign`); mutants D1, D3a-c, D4, O1, O11; e2e |
| 6 Reload; later open; draft → edit; bad token → generic | `startPublished`, Caddy `@owner` | Tests for reload, draft `replace`, malformed links, 404 and 500; Caddy `curl`; e2e (draft `replace`, wrong token). A real-browser reload is condition 1 |
| 7 Share on unlisted, hidden on draft; no unpublish control | `EditPanel` share block; B-1 (`visibility` read-only, migration 0003) | Share tests; C-11 test; `TestPatchBody`, `TestOwnerSettings`; mutants B4, O17; migration probe |
| 8 Show links opens the published page; absent on draft | `EditPanel` `links`, `owner.toPublished` | Share test; mutant O18; e2e |
| 9 Download original | `download` (unchanged since r1 apart from the busy object) | Download tests; r1 e2e bytes. iOS is condition 1 |
| 10 Cancel keeps; confirm removes; both links generic | `DeleteControl`, `actions().remove` | `describe.each` on both pages; mutants D9, D10, O20; e2e through Caddy (`/w/` 404 generic, both owner pages generic) |
| 11 SH-10 before and after publishing | `EditPanel` fixed sentence | Draft and published tests |
| B-1 (ADR-019 item 8, O-6, P-7) | `0003_no_private.sql`, `parsePatch`, `HidesMore` | `TestMigrateNoPrivate`, `TestMigrationDropsPrivate`, `TestHidesMore`, `TestNotUnlisted`; back-end mutants 6/6; real-binary migration probe |

Not traced to an automated check: a real browser's network log and its translation behaviour, back-forward cache behaviour in a real browser, the iOS download, and the wording (all conditions). "Unpublished drafts are deleted 48 hours after upload" is only true once F-21 (next in the build order) adds the sweep. The text is the product's, and the deadline belongs to F-21.
