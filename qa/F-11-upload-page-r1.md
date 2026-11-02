# QA F-11 Upload page - round 1

- **Date:** 2026-10-23
- **Verdict:** ready. No Blocker or Major finding. Two Minor findings, both about documents and labels: AC numbers drifted after the new AC was added, and the statuses in the index and ADR-020 are stale.
- **Inputs:** `features/F-11-upload-page.md` (8 ACs, including the new AC 7 "disabled while uploading"; AC 1 Android moved to F-32); `architect/F-11-upload-page.md` v1.0 (U-1 to U-11); ADR-024; ADR-020 rev. 1; the F-09 response table and F-10 `GET /api/status`; the decision log entry of 23 Oct 2026 in `features/README.md`. Code at `bcf19fc` (build `76d1f14` plus the drop-zone CSS change), clean working tree.

User-approved engineer choices, not reviewed as findings: a drop anywhere on the page uploads; a 4 s status wait, then the page fails open; text-only progress; only the first of several dropped files is uploaded.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | **Exit 0.** `go vet` clean, govulncheck 0 vulnerabilities, svelte-check 0 errors/0 warnings, Go 12/12 packages ok, Vitest **32 files, 866 tests passed** |
| `npm run size` (inside `make ci`) | `entry index.html: 45328 / 60000 bytes gzip PASS` (F-07 r3: 45,582, so the activity entry did not grow; the small drop comes from removing the module-preload polyfill, ADR-024 item 3). `entry upload.html: 2272 / 8000 bytes gzip PASS` (JS 1,571 + CSS 701). `upload entry: no storage API, no shared chunk PASS` |
| Probe: the built server with `WORKOUT_EXPLORER_STORAGE_LIMIT_MB=1` on a 2 MB scratch data folder | `GET /api/status` gives `200 {"uploads_open":false}` with `Cache-Control: no-store`. `POST /api/workouts` gives `503 {"error":"storage_full","message":"Uploads are paused because storage is full. Please try again later."}`, which matches the page's `SENTENCES.paused` word for word |
| Probe: multipart upload of exactly 20,000,000 bytes with a 244-byte UTF-8 file name | The server accepts the size and parses the file: `422 not_fit`, not `413`. So the 16 KiB framing margin in `maxBodyBytes` holds, and the client and server boundaries agree. 20,000,001 bytes gives `413 too_large` with the same sentence as the client |

Not run: no browser in CI (ADR-012). The page's behaviour in a browser rests on the jsdom tests and the user's manual check of 23 Oct 2026.

## Findings

### Minor

- **ID:** QA-11-r1-1
- **Severity / kind:** Minor / risk (traceability)
- **Route:** architect (design and ADR-020), frontend (test and script labels)
- **Where:** `architect/F-11-upload-page.md` U-9, U-10 and the Verification table; `architect/ADR-020-workout-identifiers-edit-token-and-links.md:28`; `src/web/src/upload/page.dom.test.ts:213,308`; `src/web/scripts/check-upload-entry.mjs:1`
- **Relates to:** F-11 AC 7 (disabled while uploading) and AC 8 (no cookies, no storage)
- **Evidence:** The new AC was added as AC 7, so the privacy AC is now AC 8. The design (U-9, U-10, Verification row "7"), ADR-020 rev. 1 ("F-11 AC 7") and the privacy tests still call the privacy AC "AC 7". The Verification table has no row for the new disabled-state AC. The test block that does cover it is labelled "AC 7 of F-11 scope".
- **Consequence:** The code is correct, but a reader who traces "AC 7" lands on the wrong criterion. The design does not say how the new AC is verified. It is covered: the jsdom test "while an upload runs, the picker is disabled and a second drop or pick starts nothing" and "after an error both picker and drop work again".

- **ID:** QA-11-r1-2
- **Severity / kind:** Minor / risk (document state)
- **Route:** architect
- **Where:** `architect/INDEX.md:27,52,56`; `architect/ADR-020-workout-identifiers-edit-token-and-links.md:3`
- **Relates to:** ADR-020 rev. 1, ADR-024, the F-11 design status
- **Evidence:** The index lists the F-11 design as "draft (user review before build ...)", ADR-024 as "draft (with F-11)", and ADR-020 as "accepted (rev. 1 draft)". The ADR-020 status line still says rev. 1 is "a draft with the F-11 design until the user's review". But the F-11 design and ADR-024 themselves say "accepted (23 Oct 2026)", and the code has been built on rev. 1 (the `&new` marker).
- **Consequence:** F-19 and F-20 must remove the marker as ADR-020 rev. 1 requires. If rev. 1 still looks like a draft, its owner might treat it as open. Either the user accepts rev. 1 and the statuses are updated, or the user has not reviewed it yet and that should be said.

## Previous findings

None (first round).

## Questions

None that block. For QA-11-r1-2: has the user accepted ADR-020 rev. 1 (the `&new` first-open marker)? If yes, it is a status update only.

## Coverage

| AC | Code | Automated test | Manual (user, 23 Oct 2026) |
| --- | --- | --- | --- |
| 1 Drop/pick opens the edit link | `rules.ts` `editTarget` (same origin, `/edit/` plus 22 base62, fragment of 22 base62, no query or user info, `&new` appended); `page.ts` `xhr.onload` calls `env.navigate`; the server sends a root-relative `edit_url` (`lifecycle.EditLink`, ADR-020 item 8) | `rules.test.ts` editTarget; dom test "201 opens the absolute edit link with the marker", "201 with a foreign link navigates nowhere", back-forward-cache reset | Desktop Firefox drop and iPhone Safari pick both land on `/edit/<id>#<token>&new`; the draft folder exists; Back gives a fresh page. Android is in F-32 |
| 2 25 MB refused, no request | `tooLarge`, `pick` | dom test "refuses 25 MB ... sends no request" (pick and drop) | Passed |
| 3 Non-FIT and course file messages, retry | `outcomeOf` passes the server `message` through | dom test with 422 `not_fit` / `unsupported_type`, then a new pick | `not-fit-png.fit`, `course.fit`: passed, retries worked |
| 4 Storage full notice before any pick | `checkStatus`, `parseStatus`; the drop zone is hidden in `checking` and `paused` (CSS) | dom tests: notice shown, input disabled, a pick or drop sends nothing; fails open on an error, a malformed answer, a network error or a timeout | Not tested by hand. Server side probed here (see above); the browser side relies on jsdom |
| 5 Rate limit / busy messages, retry | as AC 3 | dom test with 429 `rate_limited`, 503 `busy` | none (as planned) |
| 6 20,000,000 sent, 20,000,001 refused | `MAX_UPLOAD_BYTES`, `tooLarge` | `rules.test.ts`, dom test; server agreement probed above | none |
| 7 Disabled while uploading | `canPick` only in `ready`; `input.disabled` set in `show`; the drop handler goes through `pick`; greyed drop zone in `uploading`/`processing` (`bcf19fc`) | dom tests "while an upload runs ..." and "after an error both ... work again" | Not listed separately |
| 8 No cookies, no storage | No storage API in the sources | dom test (spy on `Storage.setItem`, `document.cookie`); source scan; build scan `check-upload-entry.mjs` | DevTools check not reported |

Checked against the design and found correct:
- U-3: no inline script, style or `style=`; `lang="en"` and `referrer: no-referrer` are present (the test asserts both).
- U-4: `lib/upload` has no imports and no DOM; `upload/` imports only `lib/upload` and `./page`.
- U-6: no `accept` filter; the input is cleared so the same file can be picked again; `preventDefault` is called on `dragover` and `drop` in every state, so the browser never opens a dropped file.
- U-7: XHR with `FormData`, no hand-set Content-Type, no timeout and no retry; the `generation` guard ignores late callbacks after a bfcache reset.
- U-8: the response table is implemented row by row.
- U-11: `/about`.
- ADR-024: disjoint counted sets, 8,000 ceiling, activity line not grown.
- The server does not read the multipart file name (no `Filename` use in `httpapi`), so a personal file name is not stored.
- Accessibility (live regions for messages) is out of scope by the spec's NFR Accessibility, so it is not reported.
