# QA F-19 Edit page: draft view and settings panel - round 1

- **Date:** 2026-10-29
- **Verdict:** ready with conditions. No Blocker or Major finding. Conditions: (1) the operator's browser check of AC 1 (network log), AC 2 (reload), AC 7 (two columns, phone pin) and AC 8 (indoor one column), because no browser is available here; (2) the user reviews the look of the grey to-be-hidden marks once (E-9, OQ-63 rule).
- **Inputs:** `features/F-19-edit-page-and-settings.md`; `architect/F-19-edit-page-and-settings.md` (v0.1), ADR-030, ADR-020 (items 7, 9, rev. 1), ADR-025, ADR-011, ADR-002, ADR-019, F-12 design; decision log OQ-81, OQ-82. Commits 961f78a (design), c9f4695 (B-1), 23a6381 (frontend) on `master`, clean working tree.

## Build and test results

| Command | Result |
| --- | --- |
| `make -k ci` | exit 0. gofmt and `go vet` clean; svelte-check 309 files, 0 errors, 0 warnings; `go test ./...` all ok; vitest 51 files, **1,286 passed**; build ok |
| `npm run size` (in `make ci`) | `index.html` **56,360 / 60,000 B** gzip PASS; `edit.html` 56,360 (same counted set); `upload.html` 2,275 / 8,000 PASS; upload entry no storage API, no shared chunk PASS. F-19's growth over F-18 r1 (55,986): **+374 of 400**. Owner chunk `start.svelte` 5,363 + CSS 617 = **5,980 B** gzip (limit 15,000) |
| `go test -race -count=1 ./...` | all 14 packages ok |
| `make corpus` | 55 ok, 0 diff, 0 fail; M/G invariants ok; 6 local real files |
| Caddy 2 (`deploy/Caddyfile`, scratch data dir, port 18099) | `/edit/{id}`, `/edit/`, `/edit/x/y?z=1`: 200, `edit.html`, `Cache-Control: no-store`, `Referrer-Policy: no-referrer`. `/edit.html` and `/edit`: 404. No access log configured |
| Size check probe | a scratch build dir whose manifest lists only the shared chunk, with a 73 KB gzip chunk behind `index.html`: `entry index.html: 73246 / 60000 ... FAIL`, exit 1. The HTML-entry path still fails over budget; a missing `index.html` entry fails (unit test) |
| Real files (scratch test on a copy, never in the repo) | `checkHidden` with the new ADR-030 assertion on the 6 local real files: all pass; 2 ranges each, up to 45,400 samples |
| Mutants, front end (scratch copy, `vitest run src/edit src/lib/owner src/components/edit src/page/hygiene.test.ts`) | **29 / 29 killed**: the 7 of the design (token in the query, header dropped, generic page on 500, redirect followed, storage write of the token (local and session), marker kept, marks computed in TypeScript) plus token in `history.state`, console, error message and 500 message; `credentials: 'include'`; `cache` dropped; token not validated; `firstOpen` always false; marks ignore the unsaved toggle; draft 404 and save 404 not generic; header from input instead of response; tab kept; C1 controls kept; no code-point cut; lifetime `>=` and 31 days; patch sends all fields; visibility sent on a draft; notice hours capped at 24; expiry in UTC |
| Mutants, back end (scratch copy, 6 packages) | **9 / 9 killed**: start range +1, end range −1, full range short, store drops pair 2, scan reads pair 1 only, API drops ranges, API encodes `null`, upload stores none, gap-filled samples counted as boundary |

## Findings

### Minor

- **ID:** QA-19-r1-1
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect
- **Where:** `architect/ADR-011-activity-page-bundle-allocation.md`, `architect/ADR-002-bundle-size-budget-measurement.md`, `architect/F-19-edit-page-and-settings.md` E-1/E-9/module table, `architect/INDEX.md:35`
- **Relates to:** F-19 hand-offs (ADR-011 revision "at F-19 QA"), ADR-002, "one home per fact"
- **Evidence:** No ADR-011 revision records F-19's share (400, measured +374 against F-18 r1), F-18's measured 1,067 or the new reserve (about 3,640). ADR-002 still says the size check counts the manifest's entry, but `check-bundle-size.mjs` now rebuilds `index.html` and `edit.html` from the built HTML's module script, because Vite 8 no longer lists either as an entry when they share one module (the manifest confirms this). The design does not record the engineers' choices that a later reader would question: a separate `wx-hidden` source instead of "a style over the existing route source" (E-9), `lib/page/view.ts` and `lib/format/dates.ts` imported by path to keep the builder out of the entry, and the address dropping every parameter after the token, not only `&new` (ADR-020 rev. 1 says unknown parameters are ignored). `architect/INDEX.md` still lists F-19 as "OQ-81, OQ-82 open".
- **Consequence:** The next feature that spends from the reserve (F-20, F-28) has no current ledger to check against. A later Vite upgrade that changes the manifest again is harder to diagnose without the ADR-002 note.

- **ID:** QA-19-r1-2
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/components/edit/EditPanel.svelte:30-31` (`days` from `now`, once); `src/web/src/edit/start.svelte.ts` (`now: Date.now()` at mount)
- **Relates to:** E-7 ("only days whose expiry is after now ... the 400 sentences never show in normal use"), AC 3
- **Evidence:** I read the code: the options are computed once at mount and not after a save. If the page stays open past `created_at + d·86,400 s` for the smallest offered `d`, that option is still offered and the server answers 400 "The lifetime must end in the future." I did not run this in a browser. The mutant tests pin the rule at a fixed now only.
- **Consequence:** The owner sees a server error for a choice the panel offered. This is rare, because it needs the smallest lifetime and an open tab. The data stays safe because the server refuses.

## Previous findings

None (first round).

## Questions

1. **Copy link on a draft (architect / user).** On the edit page of a draft, the header's "Copy link" (F-13 D-14) copies `/w/{public_id}`, which shows the generic page until the workout is published. Is that intended until F-20's Share exists, or should the control be hidden while the workout is a draft? Settled by a line in the F-19 design (E-10) or the F-20 hand-off.

## Coverage

**Engineers' deviations, judged:**
- Accepted without a finding: the early Caddy `/edit/*` route (correct headers. CSP and `X-Robots-Tag` are F-22's by the hand-off, and `meta robots` is present); `hidden1_*`/`hidden2_*` columns with CHECKs (malformed pairs are refused, and the tests cover 9 bad shapes); always two pairs for a partial cut (consistent with ADR-030, because `from > first` and `to < last` always hold); `HidesMore` treating `HiddenRanges` as unknown (conservative: a change from F-30 regeneration means remove-first, never stale public content); the shared-chunk restructuring; the uncontrolled form via attachments; the `setTimeout` expiry re-enhance (tested: "Expires in 29 days" after save); `#edit-slot`; text cleaning to spaces and code-point cuts (matching `lifecycle.ApplyPatch`: Go `unicode.IsControl` is exactly Cc, the same as the client regex); "No changes"; the error mapping; zero-padded hours; `SERVER_ERROR` duplicated, with a test that holds it equal to lib/upload's; the hygiene exemptions (narrow string replaces); the size check reading HTML (it still fails correctly, see the probe).
- Recorded as r1-1: the documentation not updated.

**Token (critical).** `lib/owner` is the only module that reads `location.hash`, calls `replaceState` or builds `Authorization`. The source scan covers every `.ts`, `.svelte` and `.html` file, and the mutants confirm it is effective. The built owner chunk alone contains `Bearer` with `cache:'no-store', credentials:'omit', referrerPolicy:'no-referrer', redirect:'error'`. Requests are same-origin `/api/workouts/{id}` and `/draft.bin` with no query. The jsdom test checks that the token is absent from `outerHTML`, `document.title`, cookies, both storages, `history.state`, the console and request bodies after the load, save, 400, 500, 404, unreadable-draft and indoor states. The copy link uses the view path. The Go e2e test `TestHiddenRangesCorpus` checks that responses and headers carry no token, and that logs carry neither the token, the `public_id` nor the ranges. Caddy has no access log, and `Authorization` would be redacted by default. Edit page head: `referrer no-referrer` and `robots noindex, nofollow, noarchive`, with no inline script or style in the built `edit.html`. Tile requests from the MapLibre worker can carry at most the site origin, never the page path or fragment. This is inherited from ADR-014 and not new.

**Migration.** `0002_hidden_ranges.sql` runs in the existing single-transaction migrator, together with `user_version`. `TestMigrateV1` migrates a v1 database that has a row, which then reads as "no ranges". A v2 database is refused by an older binary, which is consistent with "workouts short-lived, no compat".

**Acceptance criteria:**

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 Fresh upload, token only in a header | `lib/owner`, `edit/start.svelte.ts` | `edit.dom.test.ts` AC 1 block, `owner.test.ts`, `TestHiddenRangesCorpus` | traced. **Network log in a real browser not run (condition)** |
| 2 Title change, reload | `save`, `patchOf`, F-12 PATCH/GET | `edit.dom.test.ts` E-6 block; F-12 server tests | traced (save and re-render). **Reload in a browser not run (condition)** |
| 3 Lifetime ≤ 30 days, expiry shown | `lifetimeOptions`, `expiryText` | `edit.dom.test.ts` AC 3, `fields.test.ts` | traced (see r1-2) |
| 4 Hiding and download off on new upload | `NewDraft`, panel `fill` | AC 4 test; F-09 tests | traced |
| 5 Truncated notice; none when complete | `incompleteNotice` | AC 5 block (synthetic `ride-30min-cut-60`) | traced. The server's `data_end_s` comes from F-09/F-12 tests |
| 6 Modified token: generic page | `openOwner`, `showGeneric` | AC 6 block (4 malformed links, 404 on each call, on save; 500 is not generic) | traced. **Deployed-page check not run** |
| 7 Two columns ≥ 1024 px; phone pin | D-9 root with `edit` slot, F-13 CSS | panel first in `.layout`, `has-map` | partly traced in jsdom. **Widths and pin in a browser not run (condition)** |
| 8 Indoor: works, one column, toggle harmless | `[]` ranges, no map slot | indoor test (no `has-map`, toggle and save, no error) | traced. **One column at every width not run in a browser (condition)** |

B-1: on the synthetic corpus, the generated tracks (workout and publish tests) and the 6 local real files, the stored ranges equal exactly the samples whose position the public file lacks.

**Not run:** any real browser (no Chromium or Firefox here), Lighthouse, a deployed instance, and the look of the marks.
