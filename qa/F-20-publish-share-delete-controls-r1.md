# QA F-20 Publish, Share and delete controls - round 1

- **Date:** 2026-10-30
- **Verdict:** ready with conditions. No Blocker or Major finding. Conditions: (1) the operator runs the manual checks of the design's Verification table: the network log on desktop and phone (the token only in request headers), publishing a ride with hiding on and opening `/w/…` in another browser, both links showing the generic page through Caddy after a delete, and the original saving with the server's name on iOS (Orion) and desktop. No browser is available here. (2) The user reviews the new texts once they are visible (OQ-63 rule): the first-open notice, the copy-failure text, the delete confirmation, the deleted screen and SH-10.
- **Inputs:** `features/F-20-publish-share-delete-controls.md`; `architect/F-20-publish-share-delete-controls.md` (v0.1, C-1 to C-9); decision log of 30 Oct 2026 (OQ-83 to OQ-85, the publish-failure edge case); F-19 design E-2 to E-10; ADR-020 (items 7 and 8, rev. 1), ADR-025, ADR-028, ADR-011 rev. 10; F-12 owner API. Commits 0c7a08b (design) and 5bf3611 (front end, plus the clock pin in `page.dom.test.ts`) on `master`, clean working tree.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. `go vet` clean. svelte-check: 309 files, 0 errors, 0 warnings. `go test ./...`: all ok. vitest: 51 files, **1,313 passed**. Build ok |
| `npm run size` (in `make ci`) | `index.html` and `edit.html`: **56,362 / 60,000 B** gzip, PASS. That is **+2 B** over the 56,360 measured at F-20 start, within the 100 B ceiling (the engineer reported +1). The growth is the lazy chunk's new hash in the entry. `upload.html`: 2,275 / 8,000, PASS. Owner chunk: `start.svelte` 6,836 + CSS 793 = **7,629 B** gzip, limit 15,000 (F-19: 5,980, so F-20 adds 1,649) |
| `go test -race -count=1 ./...` | all 14 packages ok. No Go code changed in F-20 |
| End to end, `lib/owner` against the real Go server (scratch vitest on a copy; binary built from HEAD; port 18093; scratch data dir; 2 uploads of `ride-30min.fit`) | Pass. Workout A ran the panel's sequence: `save({title, hide_ends:true})` on a draft wrote nothing under `public/`. Then `publish()` returned unlisted, with `view_url` `/w/{id}` and 2 ranges. Every sample in those ranges has no position in A's public data file, but has one in B's (published with hiding off, as the control). `original()` returned bytes equal to `private/…/original.fit`, named `workout-YYYY-MM-DD.fit`. `delete()` returned `{ok, true}`. After that, `public/{id}` was gone and `private/` had one folder fewer. `settings`, `publish`, `delete`, `original` and `save` each returned `notFound`, and B was untouched. No request URL carried `?` or `#`. The server log has no token and no `public_id` |
| Mutants (scratch copy; `vitest run src/edit src/lib/owner src/components/edit src/page/hygiene.test.ts`) | **26 of 30 meaningful mutants killed.** All 9 design mutants were tried, and 8 were killed in their direct form: token in the Share text or copy; edit link without the marker; edit link in a `title` attribute; publish without the pending `PATCH`; publish after a failed `PATCH`; `POST` before `PATCH`; delete without confirmation; title kept on the deleted screen; two writes at once; token in the original's request query. Also killed: Share or header Copy link while private; SH-10 only when published or only on a draft; no deleted screen; generic page on a 500 delete; deleted screen on a 404 delete; name from `Content-Disposition` ignored; revoke at once; no revoke; Share not opened automatically; Cancel sends `DELETE`; navigation after publish; Publish hidden after a 500; token in the `DELETE` body. **Survived:** (a) the edit link as an `<input>` value; (b) the token in the download anchor's `href`, by way of `editLink` (a design mutant); (c) download outside the busy lock; (d) Confirm enabled while busy. (d) is equivalent, because `run()` refuses anyway. (a) to (c) are QA-20-r1-2 |
| Probe: first open with the settings request failing (scratch copy of `edit.dom.test.ts`) | With `GET` answering 500 on `…#token&new`, the body shows only the server message, there is no notice, and the address is already `#token`. A reload with `GET` fine shows no notice. See QA-20-r1-1 |
| Not run | `make corpus` (no Go or format change since F-19 QA r1, which ran it); Caddy (no `deploy/` change; the AC 5 path through Caddy is a manual condition) |

## Findings

### Minor

- **ID:** QA-20-r1-1
- **Severity / kind:** Minor / risk
- **Route:** architect
- **Where:** `src/web/src/lib/owner/index.ts:73` (marker removed before any request); `src/web/src/edit/start.svelte.ts:42-49` (a settings error renders only the message, with no panel and no notice)
- **Relates to:** AC-1, feature AC 3, C-5, F-19 E-3, OQ-84
- **Evidence:** The probe above. When the first `GET /api/workouts/{id}` fails with a 500 or a network failure, the page shows the server message. `&new` is already gone, so no later load shows the "save this link, shown only once" warning. The link itself is still in the address bar.
- **Consequence:** If a request fails right after a successful upload (for example a phone switching networks), the owner is never warned that the address is the only way to edit or delete the workout. They may close the tab and lose the edit link. One way to settle it: show the notice on the error screen as well (`editLink` is known without the server), or keep the marker until the panel has rendered. Either choice changes E-3 or C-5, so the architect should decide.

- **ID:** QA-20-r1-2
- **Severity / kind:** Minor / risk (test gap, critical lane)
- **Route:** frontend
- **Where:** `src/web/src/edit/edit.dom.test.ts:121` (`expectNoTokenOutsideTheFragment`), `:479`, `:682`
- **Relates to:** C-5 ("never … an input"), C-6, the design's mutant "the token in the download URL", C-2
- **Evidence:** Three mutants survive. (a) `<input readonly value={editLink} />` in the notice: Svelte sets `value` as a property, and the scan reads only `outerHTML` and attributes. (b) `a.href = url + '#' + token` taken from `owner.editLink`: the anchor checks run only on a later open, where `editLink` is null. The first-open test that downloads does not look at the anchor, and the anchor is removed before the scan. (c) A download outside `run()`: no test asserts that Save, Publish and Delete are disabled while a download runs.
- **Consequence:** The code is correct today. A regression on the two token paths that C-5 and C-6 forbid would pass CI. Fixes: scan `.value` of every `input` and `textarea`; record the anchor in the first-open download test; add one busy assertion for the download.

- **ID:** QA-20-r1-3
- **Severity / kind:** Minor / defect (accessibility)
- **Route:** frontend
- **Where:** `src/web/src/components/edit/EditPanel.svelte:159`, `:168-173`
- **Relates to:** C-7
- **Evidence:** I read the code. Delete only sets `confirming`. The confirmation appears below the share block with no `role`, no live region and no focus move, and focus stays on Delete. Not checked with a screen reader.
- **Consequence:** A keyboard or screen-reader user presses Delete and hears nothing. They have to find the confirmation without being told it is there. Fix: move focus to the confirmation (or to Cancel), or announce it.

- **ID:** QA-20-r1-4
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect
- **Where:** `architect/F-20-publish-share-delete-controls.md` C-2; `architect/ADR-011-activity-page-bundle-allocation.md` (ledger)
- **Relates to:** C-2, C-6, the ADR-011 hand-off "at F-20 QA"
- **Evidence:** C-2 says that Save, Publish and Delete share the busy state. The code also puts the original download under the same lock (`start.svelte.ts:121`), which is the engineer's declared deviation. The design does not mention it. ADR-011 has no F-20 entry: measured **+2 B** (entry 56,362), reserve **3,638**.
- **Consequence:** A later reader cannot tell whether blocking writes for up to 10 minutes on a slow download (`ownerDownloadTimeout`) was intended, and the reserve ledger goes stale.

## Engineer's deviations, judged

| Deviation | Judgement |
| --- | --- |
| The download shares the busy lock with writes | Acceptable. It stops a delete from running during a download, at the cost of no Save, Publish or Delete for as long as a slow download runs. Record it in C-2 (QA-20-r1-4) and pin it with a test (QA-20-r1-2 c) |
| Object URL revoked after 40 s (`shortcut:` comment) | Acceptable. It is tested (not revoked at once, revoked with the same URL), and the `blob:` URL holds no token |
| Share reuses `enhanceCopyLink`; tests select `header a[data-copy-link]` | Acceptable. In production the page-level `enhanceCopyLink(root, …)` (`page/start.ts:86`) takes the first `a[data-copy-link]`. That is the header link, because Share is closed when `startPage` runs. Opening Share before `startPage` would steal the header's handler; nothing does that today |
| Share opens by itself when a response turns draft into published | Matches C-4. It also covers a Save after a publish that answered 500 but committed (the decision-log edge case) |
| Confirm and Cancel stay after a 500 delete | Matches C-7 and O-10 (retry is safe) |
| Wording awaits user review | Condition (2) |
| Red Delete fixed at `#c0392b` in both themes | White on `#c0392b` is about 5.4:1 (AA). It is used only on "Delete permanently". Acceptable |

## Previous findings

None (round 1).

## Questions

1. **Architect or user:** browser page translation (Chrome, Edge) sends page text to the vendor's translation service. The first-open notice is the only DOM text that holds the token. Should `.edit-link` carry `translate="no"`? I did not verify whether Chrome skips sending such nodes or only skips translating them. Two answers would settle it: "accept, as part of the F-19 E-3 exception", or "add `translate="no"` in the small lane".
2. **Architect:** QA-20-r1-1: show the notice on the first-open error screen, or keep the marker until the panel renders?

## Coverage

| Criterion | Code | Test / check |
| --- | --- | --- |
| AC 1: publishing shows the page; the edit page stays and shows published | `start.svelte.ts` `publish`/`apply`; panel `{#if draft}`, `fill` | "Publish without changes…" (no remount, path unchanged, visibility select, header Copy link); server: `TestPublishEndToEnd`, the e2e probe. Through Caddy: manual (condition 1) |
| AC 2: Share shows `/w/{id}`, copy puts exactly that on the clipboard | panel `.edit-share`, `enhanceCopyLink` | Share text and `writeText` exactly `origin + /w/{id}`; fallback field holds the viewing link only; OQ-83 private hides both |
| AC 3: notice after upload; not on reopening | `lib/owner` `editLink`, panel `.edit-first` | Marker / no-marker tests; owner unit tests; mutants killed. Gaps: QA-20-r1-1, QA-20-r1-2 (a) |
| AC 4: download returns the stored original | `owner.original`, `download` | Bytes and name in jsdom; e2e probe bytes equal `original.fit`; `TestOwnerFiles`. iOS: manual |
| AC 5: Cancel keeps; confirm removes; both links generic | `remove`, `showDeleted` | Cancel sends nothing; `DELETE` then deleted screen with no ID, token, title, description, `/w/` or digits; 404 generic; 500 message. Server: `TestOwnerDelete`, e2e probe (folder gone, all endpoints 404). Through Caddy: manual |
| AC 6: SH-10 before and after publishing | panel `.edit-small` (no condition) | Draft, after publish and private; both conditional mutants killed |
| Publish ordering (C-3, OQ-85) | `publish` | `PATCH` before `POST`; no `POST` after a failed `PATCH`; reversed order killed; e2e probe shows hidden samples have no position in the public file |
| Token (C-5, C-6) | `lib/owner`, panel | Scan through load, publish, share, download and delete on first and later open; the E-2 source scan passes. Gaps: QA-20-r1-2 |

Not traced to an automated test: the network log on a real browser, the Caddy paths after delete and publish, the iOS download, and the texts' wording. All are conditions.
