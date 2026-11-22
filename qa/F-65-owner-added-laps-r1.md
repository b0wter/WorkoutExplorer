# QA F-65 Owner-added laps - round 1

- **Date:** 2026-11-22
- **Verdict:** not ready - one Major: the owner chunk is 16,745 gzip, above the 15,000 "stop and report" line of F-19/F-20, with no decision recorded (QA-65-r1-1, architect). The code itself holds: privacy probes, API refusals, Go/TS agreement, migration and rebuild all pass. Once the architect decides, the remaining findings are Minor.
- **Inputs:** `features/F-65-owner-added-laps.md`; `architect/F-65-owner-added-laps.md` 0.1, ADR-060, F-56 PN-12, F-03 §5.3/§9, F-12, F-15 P-7, F-19 and F-20 Cost, ADR-011; TD at main `f28e5dc` (branch merges with it cleanly). Code: branch `worktree-agent-aca7cc8627b50d23a`, backend `1ee448f`, frontend `e2bcbce` (base `571d0a1`).

## Build and test results

- `make -k ci` in the worktree: **exit 0**. gofmt, `go vet`, svelte-check (0 errors, 0 warnings), Go tests all `ok`, vitest **91 files, 2,335 tests passed**, build, size, caddy-check, docker-check and update-check passed. The known load flakes did not occur.
- Size (`npm run size`): activity entry **60,521 / 62,000** gzip PASS. Owner chunk `start.svelte-*.js` **16,745 gzip** (44,439 raw; it is lazy, so the size check does not count it). In the built output, the "+" and × UI exists only in the owner chunk. The entry gains only the `added` code and its word.
- `go test -race -count=1 -run 'AddedLap|CheckEdits|AddLaps|Snap|Patch|QAF65'` over lifecycle, publish, httpapi, workout, store and cmd: all `ok`.
- QA end-to-end probe (scratchpad copy, `TestQAF65Probe`, full HTTP stack with the real publisher, `ride-30min`, 1,800 samples, lap ends 0/300/…/1799): 100 entries accepted. Each of these got `400 invalid_settings` with its sentence and left the row unchanged: 101 entries, a repeat, a device lap end, outside the recording, `null`, a fraction, a non-string, a valid entry mixed with an invalid one, inside the trimmed part, and on the trim window end. Setting a trim deleted every stored entry outside the window (not sent). Undoing the trim restored none. A radius change to 2,000/2,000 kept all entries. Four boundaries in the hidden start and end plus one in the middle were published as 5 `added` laps. Their samples have no public latitude. All columns are byte-equal to a second upload with the same settings and no boundaries. The same preview hash. The `index.html` outside the laps section is identical after normalising IDs and hashes (Open Graph included). No `.fit` in `public/`. No boundary instant in the logs.
- Mutants (scratchpad copies, each reverted):
  - **Go, 16 run:** 13 killed (snap skipped ×2, trim not judged, radius judged, cap ±1 in service and HTTP, last part `added`, part keeps device totals, off the allow-list, repeat not refused, merge after trim, overlap splits the earlier lap, stale entry kept). 3 **survived** across the full publish, cmd, lifecycle and httpapi suites: device-lap-end check skipped at the real port; window-end check skipped; trim deletion of a *sent* stored entry skipped. See QA-65-r1-2.
  - **TS, 14 run:** last part `added`, snap skipped, part keeps device totals, cap ±1, trimmed check skipped, trimmed entries sent, unchanged set still sent, × on device rows, Esc without focus return and a click on an `aria-disabled` item running were all killed. **Survived (4):** window ends not treated as boundaries; focus leaving the group does not close the list; after a change the edit page does not refresh the map's lap markers (`v.map((m) => m.laps())` removed); the splits and comparison chart are not restarted. See QA-65-r1-2.

## Findings

### Major

- **ID:** QA-65-r1-1
- **Severity / kind:** Major / risk (a documented budget gate was crossed with no decision)
- **Route:** architect
- **Where:** `architect/F-19-edit-page-and-settings.md:47`, `architect/F-20-publish-share-delete-controls.md:59`, `architect/F-65-owner-added-laps.md` Cost, `architect/ADR-011-activity-page-bundle-allocation.md:97`
- **Relates to:** F-19/F-20 Cost ("Above 15,000 gzip: stop and report"), F-65 Cost (estimate 2,000 "within F-19's chunk ceiling"), NFR Performance
- **Evidence:** `make ci` size output: `assets/start.svelte-CzjxM6B4.js: 44439 raw, 16745 gzip`. ADR-011's new row records owner chunk 13,888 → 16,745 (+2,857) under "Lazy, not counted" and does not mention the 15,000 line. The design's own estimate (13,888 + 2,000 = 15,888) would also have crossed it, so the design never checked the ceiling it cites.
- **Consequence:** The edit page now loads 1,745 bytes gzip above the agreed limit, and the next owner feature has no line to measure against. Whether the cost is acceptable is the architect's call: raise the ceiling with a reason, or split the laps mirror and the "+" disclosure into a lazy chunk loaded on the first "+" open or when stored boundaries exist. I would accept a raised ceiling with a one-line reason. The code needs no change for that.

### Minor

- **ID:** QA-65-r1-2
- **Severity / kind:** Minor / test gap
- **Route:** backend (Go), frontend (TS)
- **Where:** `src/server/internal/publish/laps.go:90` (`OnBoundary = onEnd || k == lo || k == hi`), `src/server/internal/lifecycle/laps.go:68` (`case !c.Kept && stored`), `src/web/src/edit/laps.ts` (`k !== lo && k !== hi`)
- **Relates to:** AC 5 ("the server refuses the same cases when sent directly"), AL-2, AL-5, design Mutants row ("the device-boundary or window-end check skipped; trim deletion skipped")
- **Evidence:** These mutants survive every test in their packages:
  - Go: dropping `onEnd` at the real port. `TestCheckEditsLaps` uses a track without device laps, where every lap end is also a window end, and the end-to-end test refuses sample 0, which is both.
  - Go: dropping `k == lo || k == hi`. The trim case asserts only `Kept`.
  - Go: dropping the "stored and now outside, sent anyway" deletion. The lifecycle test sends a new entry in the same patch, so the refusal hides the mutant.
  - TS: dropping the window-end filter in "Add lap".
  - TS: dropping the `focusout` close.
  - TS: removing `relaps()`'s map refresh or its splits and chart restart. The map's `laps()` is unit-tested, but nothing checks that the edit page calls it or restarts the F-51 chart and row taps after an add or a ×.

  The behaviour itself is correct: my HTTP probe refused sample 300 (an interior device lap end) and the trim start, both with "There is a lap boundary here already.".
- **Consequence:** A regression in any of these checks would go unnoticed. The device-end case is the one AC 5 names. Add one real-port case on an interior device lap end, one on a trim end, one PATCH that sends a stored entry the same patch's trim leaves outside, and on the edit page in jsdom: a case at the live trim end, focus moving out of the group, and after "Add lap" the map markers, the chart slots and a tap on the new row.

- **ID:** QA-65-r1-3
- **Severity / kind:** Minor / defect (accessibility)
- **Route:** frontend
- **Where:** `src/web/src/edit/laps.ts` `rows()` (`t('removeLap', { n: i + 1 })`)
- **Relates to:** AL-8 ("its accessible name includes the lap"), F-64 LL-2 ("rows numbered from 1" per leg), F-64 note "lap numbers are per-table display text"
- **Evidence:** `i` indexes `data.meta.laps` across the whole workout, but in a multisport workout each leg's table numbers its rows from 1 (`lapsGroup` per section). A × on the second Bike lap of a triathlon is announced as "… end of lap 7" while the row shows "2". The tests cover only a single-sport ride.
- **Consequence:** Screen-reader users get a lap number that matches no visible row on multisport workouts. Use the number shown in the row's own table.

- **ID:** QA-65-r1-4
- **Severity / kind:** Minor / defect (keyboard focus), found by code reading; jsdom cannot show it
- **Route:** frontend
- **Where:** `src/web/src/edit/plus.ts` (item click: `close(false); item.run()`), `src/web/src/components/page/ActivityPage.svelte` (`{#key lapsKey}` around `LapsTable`)
- **Relates to:** AC 1 and AC 13 (keyboard), AL-9
- **Evidence:** Choosing an item hides the list while the focused item is inside it, so a browser moves focus to `<body>`. A × click bumps `lapsKey`, which destroys and rebuilds the whole laps table, so the focused × disappears as well.
- **Consequence:** After "Add lap" from the keyboard, or after removing a lap with ×, focus jumps to the top of the page. Return focus to "+" after a choice, as Esc already does, and to a neighbouring row's × or the laps heading after a removal. Check by hand in a real browser.

- **ID:** QA-65-r1-5
- **Severity / kind:** Minor / defect (edge)
- **Route:** frontend
- **Where:** `src/web/src/edit/laps.ts` `fresh()` (`set.length + add.length > MAX_ADDED_LAPS`)
- **Relates to:** AL-9 ("when the set would exceed 100"), AL-8 (live-trimmed boundaries are left out of the sent set)
- **Evidence:** `set` still holds boundaries in the live-trimmed part (dimmed, "Removed on Save", never sent), and they count towards the cap. With 100 stored boundaries and a live trim that removes some of them, "Add lap" stays off with "There are 100 added laps already.", although the set sent on Save would be below 100 and the server would accept it.
- **Consequence:** A rare false refusal; Save first, then add. Count only the boundaries that would be sent.

- **ID:** QA-65-r1-6
- **Severity / kind:** Minor / documentation
- **Route:** frontend (small lane)
- **Where:** `architect/ADR-011-activity-page-bundle-allocation.md` item 4, Reserve row
- **Relates to:** ADR-011 item 4 and 5, F-65 Cost (entry ceiling 50)
- **Evidence:** Entry +52 against a ceiling of 50. The engineer measured 60,511 to 60,528 for the same code, so the real cost is 42 to 59. I accept it as noise; it needs no architect. The Reserve row still ends at "1,531 measured after F-64" and lacks "**1,479** measured after F-65 (entry 60,521)".
- **Consequence:** The next feature reads a stale reserve. Add the line.

- **ID:** QA-65-r1-7
- **Severity / kind:** Minor / preference
- **Route:** backend
- **Where:** `src/server/internal/publish/laps.go:105` (`CheckPins`)
- **Relates to:** dead code
- **Evidence:** `lifecycle.Trimmer` no longer has `CheckPins`. `(*Area).CheckPins` is now called only from `pins_test.go`.
- **Consequence:** Production code that exists only for tests. Move it into the test file, or call `CheckEdits` from the tests.

## Engineer deviations (judged)

- **Backend 1, `CheckEdits` replaces `CheckPins` on the port:** accepted. One parse serves both checks, and the design's "through the existing port" holds. (The leftover method is QA-65-r1-7.)
- **Backend 2, trim window ends count as boundaries:** accepted. This is AL-3 as written, not a deviation. The owner sees "There is a lap boundary here already." at a trim end, and the edit page mirrors it.
- **Backend 3, re-sort on split:** accepted. `w.Laps` keeps file order, so on a file whose laps are out of time order, the first boundary also reorders unsplit laps. The TS side does the same, malformed files may be refused, and the device values stay byte-equal.
- **Backend 4, pool snap definition** (latest-starting session holding the sample; a length belongs to the session holding its last sample): accepted. It is identical in Go (`SnapLapBoundary`) and TS (`snapLapBoundary`), and the shared sample includes `pool-swim`. Snap mutants are killed on both sides.
- **Backend 5, TD hand-off:** done on main in `f28e5dc` (schema v11, data file v7, Public build step, P-7 allow-list, glossary). It matches the code.
- **Frontend 1, dimmed rows for live-trimmed boundaries:** accepted. This is AL-8 as written.
- **Frontend 2, `enhanceUnits` no longer runs on the edit page:** accepted. The edit page builds its strip and laps in the look's units itself, and the existing "stored trim … in the chosen units" test still passes.
- **Frontend 3, detached subscribers shortcut:** accepted. Each re-render leaves one O(1) selection subscriber and one splits chunk instance per change, and the code carries a `shortcut:` comment. This is a risk only for owners who make hundreds of edits in one visit.
- **Frontend 4, "+" stays on the map when the sliders hide the whole route:** accepted. The map is still shown on the edit page. OQ-305 is about the server's no-location or route-hidden hint, which the jsdom test checks for `indoor-no-gps`.
- **Frontend 5, × handled in a capture listener on the host:** accepted. It runs before the row's tap and works for keyboard activation, since Enter on a button fires a click.

## Previous findings

None (round 1).

## Questions

None that block. QA-65-r1-1 needs an architect decision, not a product one.

## Coverage

| AC | Code | Test / check | Result |
| --- | --- | --- | --- |
| 1 "+" replaces Add pin, disabled without cursor, menu, no-route placement, Esc/outside/"+" again, keys | `edit/plus.ts`, `edit/pins.ts`, `ActivityPage.svelte` | `edit.dom.test.ts` F-65 block, PN-12 docking test; mutant "Esc no focus return" killed | traced; focus after a choice is QA-65-r1-4; Tab and Enter/Space are native button behaviour (DOM order asserted) |
| 2 Add pin unchanged | `edit/pins.ts` `addPin` item | all F-56 tests pass | traced |
| 3 split at cursor, renumber, triggers, other laps unchanged | `workout/addedlaps.go`, `lib/addedlaps` | `TestAddLapsSplitsDeviceLap`, `added-laps.json` Go/TS agreement (ranges, order, triggers exact; values within quantisation), jsdom "Add lap splits …" | traced |
| 4 range → both ends, end on boundary adds nothing | `edit/laps.ts` `fresh()` | jsdom "with a range" | traced |
| 5 disabled reasons; server refuses the same cases | `edit/laps.ts`, `lifecycle/laps.go`, `publish/laps.go`, `httpapi/owner.go` | lifecycle (stand-in), httpapi parse, QA HTTP probe (all five refusals plus parse errors) | traced; real-port gaps are QA-65-r1-2 |
| 6 hidden start/end accepted, no position published | no position check by design | `TestAddedLapsPrivacyCorpus` (whole synthetic corpus except ride-12h; radii 0/50/300/2,000 combinations; with and without trim; first visible sample and dropouts; ≥100 added laps, ≥20 hidden), `TestAddedLapsEndToEnd`, QA probe (columns, preview, page outside laps byte-equal) | traced |
| 7 no device laps: splits → laps view → splits | `AddLaps` implicit lap, `pageLaps` | `TestAddedLapsRebuildInPlace`, jsdom "without device laps" | traced |
| 8 edit page at once, viewing page after Save, JS on and off, no re-upload | merge in `edit/laps.ts`; server-rendered table | `TestAddedLapsRebuildInPlace` (no-JS table contains "Added"), jsdom | traced |
| 9 × only on added rows, server refuses removing a device boundary | `LapsTable.svelte`, `rows()` | jsdom; mutant "× on device rows" killed. A device boundary cannot be sent, because the set holds only added instants and an entry on a lap end is refused | traced |
| 10 trim deletes on Save; chart, taps, map markers, F-64 tables include added laps | `judgeLaps`, `relaps()` | lifecycle trim tests, QA probe, `map-view.dom.test.ts` `laps()`; on the viewing page the merged laps are the data file's own `laps`, so the existing renderers apply | traced for the public page; on the edit page the wiring of map markers, chart and taps is untested, and so is the "sent and deleted" path (QA-65-r1-2) |
| 11 pool snap, stroke kept | `SnapLapBoundary`, part copy | `TestSnapLapBoundaryAndStroke`, `TestCheckEditsLaps` pool, TS snap test | traced |
| 12 nothing remains after delete or expiry | row column, `secure_delete` | `TestOwnerDeleteLeavesNoBytes`, expiry and draft-expiry tests now search the column text; `TestAddedLapBoundariesColumn` | traced |
| 13 four languages; themes, phone/desktop, touch/mouse | `i18n/{de,es,fr}.json` owner and page groups, `about.html` | language file checks in `make ci` | texts traced; **visual checks not run** |
| 14 viewers load no new code; entry/LCP; 60 fps with 100 boundaries on ride-12h; corpus renders | trigger word only in the entry | size check; all goldens re-encoded at v7; corpus privacy test | entry traced; **LCP and 60 fps scrub with 100 boundaries on ride-12h not measured** (design Verification asks for the scrub gate) |

- **Migration v11 / data file v7:** `0011_added_lap_boundaries.sql` adds a nullable `TEXT` column with a `json_valid` check, so existing rows stay `NULL`. `TestSchema`, `TestMigrationFiles` and `TestAddedLapBoundariesColumn` pass. Format version 7 is written and decoded on both sides, and the `regenerate` test asserts the `[]time.Time` field is never written (RG-4).
- **In-place rebuild and no rebuild when unchanged:** `added_lap_boundaries` is on the P-7 allow-list (mutant killed). `TestAddedLapsRebuildInPlace` sees no remove step. The edit page sends the set only when it differs (mutant "unchanged set still sent" killed).
- **Not run:** browser hand checks (themes, phone and desktop, portrait and landscape, touch, a real screen reader), LCP, the ADR-016 scrub gate with 100 boundaries on `ride-12h`, ride-12h in the corpus privacy test (excluded by the test for time; the rule is O(N) and identical).
