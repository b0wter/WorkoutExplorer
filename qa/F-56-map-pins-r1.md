# QA F-56 Map pins - round 1

- **Date:** 2026-11-20
- **Verdict:** not ready. One Major (QA-56-r1-1): the check that lets a pins-only Save skip the rebuild (PN-8) runs before the hides-more rule (P-7). Only one untested part of that check keeps it from skipping the remove-first build when a radius is raised in the same Save. Without that part, the newly hidden start stays public (probe below). The behaviour is correct today, and the fix is one test. Apart from this, the privacy model held in every probe: no dimmed or trimmed pin reached any public file, `Count` or the file name, across 728 corpus builds and an end-to-end owner-API flow.
- **Inputs:** `features/F-56-map-pins.md` (AC 33 to 46, OQ-234 to OQ-240, OQ-272 to OQ-277, decisions of 20 Nov 2026); `architect/F-56-map-pins.md` v0.2 (PN-1 to PN-13, Verification); ADR-057 rev. 1; ADR-011 (F-56 row); ADR-028; F-12 and F-15 rows from `4ebce16`; TD changes `c17c3fd`; product spec PIN-1 to PIN-8, SH-4, SH-9, SH-10, UP-6, UP-7, AP-15. Code: backend `6317245`, frontend `b814240`, HEAD `b814240`, clean working tree.

## Build and test results

- `make ci` at HEAD: **exit 0**. Go: every package ok (`publish` 57 s). Vitest: 79 files, **2,027 tests passed**. svelte-check clean. Size: `entry index.html: 57858 / 60000 bytes gzip PASS`, and the same for `edit.html`. Lazy `pins-*.js` 2,954 gzip and `pins-*.css` 1,045 gzip, listed as dynamic imports (not counted). caddy-check, docker-check, update harness 50/50 and sign 12/12 ok.
- `go test -race -count=1` on `internal/lifecycle`, `internal/httpapi`, `internal/store`, and `internal/publish -run 'Pin|Hides'`: all ok.
- **Entry size per commit** (scratch `git archive` plus `vite build` plus `check-bundle-size.mjs`): `9ec6db3` (before F-56) **57,601**; `6317245` (backend, `lib/page` view model) **57,609**; HEAD **57,858**. F-56 therefore adds **+257** against its 250 ceiling (QA-56-r1-2).
- **Corpus privacy sweep** (scratch copy of `src/server`, real `ingest` parser, real `Area.WorkoutChanged` and template). Inputs: every parseable synthetic corpus file with at least 20 samples; radii 0/0, 50/0, 0/300, 300/300, 2000/0, 50/1950 and 2000/2000; no trim, and a trim of 10 % at each end. Each build had a point pin about every n/120 samples and a section from each point to the point three steps on, each with a unique marker title. That gave **728 builds, 110,521 public and 69,043 non-public pin checks, 0 failures**. The oracle was the public data file of the same build. A pin's title is in some public file (brotli included) exactly when every sample it is anchored to (its `from`, and its `to` for a section) has a position there. Every `from` and `to` in `pins.{hash}.json` has a position and points at the pin's own sample. The file has no keys beyond the five. `data-count` equals the number of public pins. With no public pin there is no file and no slot.
- **End-to-end owner-API flow** (scratch, `cmd/workout-explorer` harness, `ride-30min.fit`). All steps pass:
  - pins placed, then published;
  - a radius raised: the pin stays in `GET` and leaves `public/`;
  - a new pin at a neighbouring hidden sample: `400`;
  - a dimmed-only edit: every file under `public/` has the same size and mtime;
  - a radius raise and a dimmed edit in one Save: rebuilt;
  - the radius lowered: the pin is public again;
  - a trim past a pin: the pin leaves the row and `public/`, and undoing the trim does not restore it;
  - a point, and a section reaching into the trim, sent while the trim is set: `400`;
  - no pin text in the logs.
- **Delete with 50 maximal pins** (500-character notes, so the row spills into overflow pages): before the delete the marker texts are in the WAL and the pins file. After the owner's delete, and again after close, `dataLeft` and a byte walk of the data directory find nothing.
- **PATCH body:** the largest valid body has every code point escaped as a surrogate pair (12 bytes) and every other settable field set. It is **355,227 bytes**, under the cap of 368,640 (360 KiB), and is accepted with 50 pins. A body one byte over the cap is refused. Probes: duplicate keys in a pin are accepted, the last one winning; `.000Z` and `+02:00` are accepted and normalised, the same as the trim; a key in a different case (`"Type"`) gets `400`.
- **Mutants** (scratch copies, one at a time):
  - **Go, 29 run: 24 killed, 2 equivalent, 3 survived.**
    - Killed: positioned ignores NaN; one section end checked; publish uses kept instead of public; `Count` counts every pin; `samePublicPins` always false; `samePublicPins` ignores content; the DeepEqual guard removed (identical rows skipped); the `index.html` existence check removed; ties resolved to the later sample; no trim deletion; deletion on a radius raise; a new anchor in a hidden part accepted; no re-parse on a trim change; no 50 limit; no title control check; no title trim; `Outside` skipped when pins are sent; pins judged against the old row instead of the patched one; output not sorted; note controls allowed; a `null` title parsed as `""`; extra keys allowed; fractional seconds allowed; `"pins":null` accepted.
    - Equivalent: the visibility check in `samePublicPins` (it is implied by `sameRecord`, and lifecycle calls the hook only for non-drafts); the grid built from `before` instead of `after` (the records are equal apart from pins).
    - **Survived:** M5, `!sameRecord(b2, after)` removed (QA-56-r1-1); L4, `sameAnchor` compares start only; M13, a section whose ends resolve to one sample is no longer "merged" (both QA-56-r1-5). Separately, a body cap of 128 KiB passes the test suite (QA-56-r1-3).
  - **Web, 12 run: 10 killed, 2 survived.**
    - Killed: dimmed pins dropped from the sent set; removed count always 0; `place` without the hidden check; `textContent` changed to `innerHTML`; `readPins` without `to > from`; Add pin with no visible route; the dimmed reason checks one section end only; Move without the check; wrong slot reserve; chunk never loaded.
    - **Survived:** F7, a section's end not snapped or checked, and F8, the dim check on a new section removed (QA-56-r1-6).

## Findings

### Major

- **ID:** QA-56-r1-1
- **Severity / kind:** Major / risk (an untested privacy guard)
- **Route:** backend
- **Where:** `src/server/internal/publish/pins.go:192` (`samePublicPins`), called at `src/server/internal/publish/publish.go:250` before the P-7 branch
- **Relates to:** PN-8, ADR-057 item 5, F-15 P-7, PIN-7, SH-4, SH-10; the architecture's mutant list ("`pins` fully on P-7's allow-list")
- **Evidence:** `samePublicPins` returns early from `WorkoutChanged`, before `HidesMore` is consulted. It may skip the build only when the records differ in `Pins` alone, and the conjunct `!sameRecord(b2, after)` is what enforces that. Removing that conjunct (M5) leaves the whole `internal/publish` suite green. A scratch probe, `TestQARadiusRaisePlusDimmedEdit`, publishes `trimTrack` with a pin at sample 20. It then applies one change: `HideStartM` 0 → 300 plus a new title for the pin that 300 m now dims. The probe passes on HEAD. Under M5 it fails: "raising the radius together with a dimmed-pin edit left the old positions public". The projection test sees the same public pins under the new grid, so no rebuild runs, and the old data file with the formerly visible start stays served.
- **Consequence:** Today the code is correct. But the only thing between the normal edit-page flow (drag a radius slider, retitle a pin, Save) and a hidden start left public is one untested conjunct. That is worse than the mutant the design requires to be killed (`pins` on the allow-list), because this one bypasses P-7 for every field. A refactor that simplifies the condition would pass CI. Add a test like the probe (radius or trim change plus a projection-neutral pin edit in one change: remove-first, new data file) and include M5 in the mutant set.

### Minor

- **ID:** QA-56-r1-2
- **Severity / kind:** Minor / defect (budget record)
- **Route:** frontend (record); architect (accept or trim)
- **Where:** `architect/ADR-011-activity-page-bundle-allocation.md`, F-56 row and reserve
- **Relates to:** ADR-011 item 4, F-56 Cost ("ceiling 250 bytes gzip … above: stop and report"), AC 45
- **Evidence:** The row records "+249 gzip (entry 57,858 − 57,609 at `6317245`)". But `6317245` is F-56's own backend commit, and its `lib/page` view-model change (`Pins` group, PN-7) already added +8 to the entry: 57,601 at `9ec6db3`, then 57,609. F-56's static cost is **+257**, which is 7 B over the ceiling. The reserve is 2,142 against the true figure, not the 2,150 the measurement implies.
- **Consequence:** The ceiling was exceeded without the stop-and-report step, and the record understates it. There is no budget risk (the entry is 57,858 of 60,000). Correct the row, and either accept 257 or trim 7 B.

- **ID:** QA-56-r1-3
- **Severity / kind:** Minor / defect (test)
- **Route:** backend
- **Where:** `src/server/internal/httpapi/owner_test.go:327`
- **Relates to:** PN-2 (O-12 body cap), AC 41
- **Evidence:** The comment says each code point is "a surrogate pair escaped (12 bytes)". The file holds the raw emoji (bytes `F0 9F 98 80`, 4 bytes), so the "largest valid body" is about 122 KB. With `maxPatchBytes = 128 << 10` the `httpapi` suite passes. The escaped body in my probe is 355,227 bytes and fits the real cap, so the behaviour is right.
- **Consequence:** A cap reduced below the largest valid body would pass CI, and a legitimate maximal Save would then get `400`. Build the body with `😀` escapes.

- **ID:** QA-56-r1-4
- **Severity / kind:** Minor / defect (missing test)
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/expiry_test.go:50`, `:211`
- **Relates to:** AC 44, UP-7, AC-4; the architecture's Verification row "F-21's database byte scan also searches a pin title and note"
- **Evidence:** `dataLeft` is called with `texts` (title and description) only, and the comment still says "phase 2 adds pins". No delete or expiry test sets pins. The probe above shows the behaviour is right (no bytes left, overflow pages included).
- **Consequence:** AC 44's database half rests on construction (same row, `secure_delete`) rather than a test. A future move of pins to a child table, which ADR-057 names as a possible change, would not be caught.

- **ID:** QA-56-r1-5
- **Severity / kind:** Minor / defect (test gaps in PN-5)
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/pins.go:104` (`sameAnchor`); `src/server/internal/publish/pins.go:91` (`Merged`)
- **Relates to:** AC 35 ("moving a pin … into a hidden or trimmed part is still refused"), PN-2 (`end_at > start_at`), PN-6 (`to > from`)
- **Evidence:**
  - L4: when `sameAnchor` compares only the start, a section whose stored start is kept while its end moves into a hidden part counts as "stored" and is accepted. No test fails. It stays unpublished, so nothing leaks, but the AC 35 refusal is gone.
  - M13: with `ei < si` instead of `ei <= si`, a section whose ends resolve to one sample (possible on sparse or smart recording) is stored with `to == from`. `readPins` then rejects the whole public file and viewers see no pins at all. No test fails.
- **Consequence:** Two rules with no test. One of them protects availability for every pin on the page.

- **ID:** QA-56-r1-6
- **Severity / kind:** Minor / defect (test gap)
- **Route:** frontend
- **Where:** `src/web/src/edit/pins.ts:172-197` (`toSection`, the Section checkbox, range moves)
- **Relates to:** AC 34 (OQ-273), PN-12
- **Evidence:** F7 (a section's end taken unsnapped and unchecked) and F8 (the dim check on a new section removed) both survive the web suites. No editor test makes a section from a range, moves its ends with a new range, or is refused at a hidden end.
- **Consequence:** The server still refuses (tested), so there is no leak. But AC 34's editor behaviour is untested, and a regression would show the owner a server sentence that should "never show in normal use" (F-12).

- **ID:** QA-56-r1-7
- **Severity / kind:** Minor / defect (verification not done)
- **Route:** frontend
- **Where:** `src/web/bench/` (no pins case)
- **Relates to:** AC 45 ("with 50 pins on the synthetic 12-hour ride, cursor scrubbing keeps 60 fps"); the architecture's Verification row "50 pins on `ride-12h` pass ADR-016's scrub gate"
- **Evidence:** No bench or measurement includes pins. The only evidence is the structural test "nothing pin-related runs on a cursor move" (`pins.dom.test.ts:185`).
- **Consequence:** The 60 fps part of AC 45 is not measured. The structural test makes a regression unlikely but does not prove the frame budget with 50 DOM markers on MapLibre.

## Previous findings

None (round 1).

## Questions

1. **Architect, ADR-011:** F-56 measures +257 against a 250 ceiling (QA-56-r1-2). Accept it and record 257 and a reserve of 2,142? Suggestion: accept. The 7 B buys nothing worth a code change.
2. **Architect, PN-8 shape:** should `samePublicPins` stay ahead of the P-7 branch, guarded by `sameRecord`? The alternative is to fold it into the rule: "`HidesMore(before with after's pins, after)` is false and the projection is unchanged". Suggestion: keep the code and add the QA-56-r1-1 test. The present shape is fine once that test exists.

## Coverage

| AC | Code | Test / evidence |
| --- | --- | --- |
| 33 Add pin, snap, form, empty title | `edit/pins.ts`, `lib/pins` `snap`, `pinName` | `edit.dom.test` (refused hidden, placed visible, whole set sent), `lib/pins` tests. **Esc and Cancel, and a tap on a chart, not tested:** hand check |
| 34 Sections from a range | `edit/pins.ts` `toSection` | Not tested in the editor (QA-56-r1-6); server side tested |
| 35 Refused in hidden or trimmed parts, server too | `lifecycle/pins.go` `resolvePins`, `publish/pins.go` `judge` | `TestPinsHiddenAndTrimmed`, `TestPinPredicatesAgreeWithTheDataFile`, `TestPinFirstVisibleFixAndDropout`; corpus sweep; E2E probe. Gap: QA-56-r1-5 |
| 36 Trim deletes with a count; radius dims and keeps; lowering republishes | `resolvePins`, `samePublicPins`, `edit/pins.ts` `saved`, `dimReason` | `TestPinsHiddenAndTrimmed`, `TestPinsDimmedNeverPublic`, `edit.dom.test` count line; E2E probe. Gap: QA-56-r1-1 |
| 37 Views, list placement, linking | `pins/index.ts`, `page.css` | `pins.dom.test` selection and map; placement in one and two columns: **hand check** |
| 38 Legend | `pins/index.ts` | `pins.dom.test` legend |
| 39 Fullscreen card | `pins/index.ts` (map controls) | `pins.dom.test` card; real fullscreen: **hand check** |
| 40 Edit after publish, old place gone | P-7 remove-first, `samePublicPins` | `TestPinsFile` (old file removed), `TestPinsDimmedNeverPublic` (remove-first steps); E2E probe |
| 41 Limits, text rules, hostile text | `httpapi/owner.go` `parsePins`, `checkPinShape`; `textContent` only | `TestPinsRules`, httpapi refusals, `pins.dom.test` hostile, hygiene scan; mutants killed. Gap: QA-56-r1-3 |
| 42 No visible route | `hasVisibleRoute`; server refuses every new anchor | `edit.dom.test` (no Add pin, dimmed list), `TestPinFirstVisibleFixAndDropout` (indoor); sweep (fully hidden gives no file) |
| 43 No pins in preview | preview and card take no pin input (`grep Pins` in `page`, `preview`, `publish`) | By construction (code read); no dedicated test |
| 44 Nothing remains after delete or expiry | Row column, `Remove` | `TestPinsFileRemovedWithFolder`; delete probe. Gap: QA-56-r1-4 |
| 45 Nothing loads without pins; budget; 60 fps | `page/start.ts` slot guard | `page.dom.test` (no slot, no request; chunk after charts); size 57,858 (QA-56-r1-2). 60 fps not measured (QA-56-r1-7) |
| 46 Themes, phone, touch | `pins.css` | **Hand check** |

Also checked and found correct:
- **PN-4, file shape:** the predicates read the transform's output, never the radii or `hiding_steps`. `Hide` keeps sample count and alignment, so the index arithmetic holds.
- **PN-6, file and slot:** the pins file is linked as `/w/{id}/pins.{hash}.json`, so the sweep's `referenced` keeps it. Caddy serves it with the hashed-name rule. `pinsFile`/`Count` come only from public pins.
- **Owner API:** the token is checked before the body is read. The migration is v10, a nullable `json_valid` column, and the round-trip and `UpdateDerived` tests show regenerate does not touch pins. About and How to use lines are present.
- **Front end:** the slot reserve is set through CSSOM (no inline `style` in markup, so the CSP holds), and no listener runs per cursor frame.

Not run: browser or device checks, LCP, and the scrub gate.

**What the user should check by hand:**
- the Add pin flow with Esc and Cancel, in the map and in fullscreen;
- a tap on a chart;
- sections from a range;
- list placement in one and two columns;
- the legend and the fullscreen card;
- icons and colours in both themes;
- phone, touch and landscape;
- whether hiding a type in the legend leaves the slot's reserved `min-height` as empty space under the list;
- 60 fps scrubbing with 50 pins on `ride-12h`.
