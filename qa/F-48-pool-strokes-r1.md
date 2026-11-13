# QA F-48 Pool swims: lengths and stroke type - round 1

- **Date:** 2026-11-13
- **Verdict:** ready - no Blocker or Major. Four Minor findings, below.
- **Inputs:** `features/F-48-pool-strokes.md`; `architect/F-48-pool-strokes.md` (PL-1 to PL-9, OQ-185); ADR-048; F-03 §5.8 (v1.8); ADR-017 rev. 9; F-47 TR-5; ADR-011 item 4; decisions OQ-180 to OQ-185 (as suggested). Commits 92d0b1f (design) and 98b424e (code); diff `8e5c76c..98b424e`; working tree clean.

## Build and test results
- `make ci`: exit 0. gofmt, `go vet`: clean. `go test -count=1 ./...`: all packages ok. Vitest: 69 files, 1,788 tests passed. Build ok. Size: entry `index.html` / `edit.html` 56,651 / 60,000 gzip PASS (+259 over 56,392; PL-9 ceiling 300). caddy-check and docker-check ok.
- `go test -race -count=1` on `workout`, `datafile`, `page`, `publish`: ok.
- Entry bundle `main-*.js` holds no `SWOLF`; the Lengths view is in `splits-*.js` (2,100 gzip, lazy).
- Mutants (scratch copy of `src/server`, all killed): cut length kept (`TestTrimLengths`, `TestTrimKeepsTrimmedLengthsOut` leak check reports 4322, 55.555, backstroke in `data.*.bin`); lengths outside a pool swim kept (`TestLengthsScope`, `TestListenerLengthsAndStrokes`); trimmed `lengths` recount removed; rest lengths counted after a trim. SWOLF from the unrounded time is killed by `lengths.test.ts` (21.6 + 14 = 36).
- Golden data files decoded: `pool-swim.bin` 41 lengths (40 active, a rest of 0 s at the end), summary lengths 40, stroke freestyle; `pool-swim-mixed.bin` 16 lengths (2 rests), no summary stroke, lap strokes `freestyle, "", backstroke, drill, mixed`; `pool-swim-mixed-cut-80.bin` no session, 13 lengths kept (OQ-185), no `avg_speed`; `triathlon.bin` no lengths and no lap stroke even though the file records freestyle.

## Findings

### Minor

- **ID:** QA-48-r1-1
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/internal/page/page.go:105` (Go `strokeWords`) vs `src/web/src/lib/page/build.ts:78-83` (TS `codeWord`)
- **Relates to:** PL-7, ADR-026 (both builders agree cell for cell); AC 2 "each FIT stroke maps to its word"
- **Evidence:** I changed Go's `"Breaststroke"` to `"Breast"` and ran `go test ./internal/page/`, which still passes. The Go page test pins only IM and Butterfly. The page samples pin Freestyle, Backstroke, Drill and Mixed. Removing `mixed` does fail. The TS test pins all seven words.
- **Consequence:** A typo or an omission in the Go breaststroke word would go unnoticed. The no-JS page would then differ from the JS and edit pages. To fix it, make the Go test assert all seven codes, matching `page.test.ts:190`, or add a breaststroke lap to a page sample. The `codeWord` change itself is fine.

- **ID:** QA-48-r1-2
- **Severity / kind:** Minor / defect (docs)
- **Route:** frontend
- **Where:** `architect/ADR-011-activity-page-bundle-allocation.md` item 4, F-48 row
- **Relates to:** PL-9, ADR-011 item 4
- **Evidence:** The row says "measured at the build, from entry 56,392", but no measured value is recorded, and the reserve line is not updated. The measured value is 56,651 (+259).
- **Consequence:** The next feature cannot see the reserve. Record +259 and entry 56,651, and update the reserve.

- **ID:** QA-48-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/tools/corpusgen/f08.go` (cap file, 10,000 lengths); `src/server/internal/publish/trim_test.go:367`
- **Relates to:** PL-9 verification row "Budget"; ADR-048 Consequences (about 1.1 MB metadata at the cap); AC 11 "the edit page lists every length"
- **Evidence:** (a) The 10,000 cap-file lengths sit inside cycling and running sessions, so `Normalise` drops them all. The stop rule therefore measures the parse only. Encoding, publishing and the browser parse of a pool swim that keeps 10,000 lengths are never measured. (b) No test asserts that `draft.bin` or the edit page still holds every length after a trim. This depends only on F-47's existing guarantee that `draft.bin` is untrimmed.
- **Consequence:** Both paths are correct by reading the code, but nothing pins them. A cheap fix: make one cap-file session `lap_swimming`, and add one assertion on `draft.bin`'s length count in `TestTrimKeepsTrimmedLengthsOut`.

- **ID:** QA-48-r1-4
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/tools/corpusgen/main.go` (pool lengths); `src/server/testdata/corpus/synthetic/pool-swim.fit`
- **Relates to:** AC 6 ("numbering continues after [the rest row]"), PL-3 fallback, OQ-183
- **Evidence:** The rest length in `pool-swim.fit` is the last length and lasts 0 s (index 1200-1201, so it cannot be selected). On this file, "continues after it" cannot be observed; `pool-swim-mixed.fit` covers it instead. No corpus file writes a length `avg_speed`, so the "recorded average speed when there is no pool length" path is tested only by the unit test `TestLengthAvgSpeedFallback`. The truncated file shows pace "–" for every length.
- **Consequence:** The ACs are met through the other file, so this is acceptable. If `pool-swim-mixed.fit` is regenerated, giving its lengths `avg_speed` would exercise the fallback end to end in the truncated case.

## Engineer-reported deviations (judged)

| Deviation | Judgement |
| --- | --- |
| pool-swim.fit rest length at the end, 0:00 | Accepted. See QA-48-r1-4. |
| Triathlon open-water swim records freestyle (negative case) | Good. The golden data file shows the stroke cleared. |
| Length time 0 counts as recorded | Accepted. FIT treats 0 as a valid value. Pace is absent (`Time > 0` guard), so there is no division by zero. |
| Overlapping sessions: the latest-starting session takes the length | Accepted. It is documented in `sessionLookup`. Overlapping sessions only occur in malformed files. |
| Golden ingest report not extended | Accepted. The lengths are pinned byte-for-byte through the data-file `.bin` goldens and `TestListenerLengthsAndStrokes`. |
| New `-update` flag for Go samples | Accepted. |
| Truncated file without pool length: pace "–" | Matches OQ-183. |
| Formatters passed to the chunk as a tuple | Accepted. The reason (minification, Vite split) is in the comment. |
| `codeWord` replaces the TS trigger map | Accepted. The TS test pins all codes of both kinds, including `toString`. Go drift: QA-48-r1-1. |
| Lengths table column rules (Rest in the # column; pace header per 100 m or 100 yd) | Matches the Scope section. Tested in `laps.dom.test.ts`. |
| Module heading without device laps is the first view | Matches LS-3 and PL-7. Tested. |
| Switching views drops the row highlight | Accepted. The range itself stays, as LS-7 requires, and this is tested. |
| Owner trimmed-summary type widened to `number \| string` | Accepted. The only consumer casts to `Summary`, which now includes `stroke`. The cast is unchecked but was already there before. |
| ADR-011 item 4 numbers not yet recorded | QA-48-r1-2. |

## Previous findings
None (first round).

## Questions
None blocking. One for the user: real devices often write sparse `record` messages in pool swims. A length that covers fewer than 2 samples cannot be selected, and the tap does nothing without any message. The answer that settles it is "accepted (no real-device promise)" or "show it as not tappable". I suggest accepting it.

## Coverage
- **AC 1** (Stroke column and summary cell Freestyle, untinted): `testdata/page/pool-swim.json` sample, Go and TS page tests. Untinted: `stroke` is not in the `page.css` tint selectors.
- **AC 2** (mixed swim, empty cell, word map): `pool-swim-mixed` sample. FIT enum to code: `convert_test.go:283`, all ten values. Code to word: TS covers all seven, Go partly (QA-48-r1-1).
- **AC 3** (no Stroke column without strokes; triathlon, rides and runs): `pool-swim-no-lengths` sample, triathlon golden, `laps.dom.test.ts` "rides and open-water swims".
- **AC 4** (with JS off and on the edit page): Go and TS builders produce identical samples (ADR-026).
- **AC 5** (Laps, Splits, Lengths; opens on Laps; no view without lengths): `laps.dom.test.ts`.
- **AC 6** (40 numbered rows plus Rest): `laps.dom.test.ts` pool-swim case. Numbering continuing after a rest: mixed case and `lengths.test.ts`.
- **AC 7** (pace per 100 m and per 100 yd; SWOLF 36): `laps.dom.test.ts`, `lengths.test.ts`.
- **AC 8** (drill shows "–"; no Strokes and SWOLF columns without counts): `laps.dom.test.ts`.
- **AC 9** (tap selects the length): `laps.dom.test.ts`.
- **AC 10** (Lengths cell = `num_active_lengths`): `TestTrimLengths` (device count 4); goldens (40, 14).
- **AC 11** (trim): `TestTrimLengths`, `TestTrimWithoutLengthMessages`, `TestTrimKeepsTrimmedLengthsOut` (markers, 9 lengths, owner summary = public). Edit page lists every length: not asserted (QA-48-r1-3).
- **AC 12** (truncated swim): `pool-swim-mixed-cut-80` golden, `laps.dom.test.ts` truncated case.
- **AC 13** (corpus regenerated to v5): golden `.bin` files at `format_version` 5; `make ci` green.
- **AC 14** (bundle, size, performance): bundle checked by the size script and the grep above. `ride-12h.bin` grows by 300 bytes (1,522,800 to 1,523,100, +0.02 %), within noise. LCP and 60 fps scrubbing were not measured; the hot path is unchanged by reading (the table is built only on arrival, switch or units change).
- **PL-1 to PL-9:** traced. `MaxLengths` and `limitLengths` are in `limits.go`, with a flood test and a budget test in `hostile_test.go`.
- **Not run:** browser LCP and fps measurement, real-device FIT files (none committed, by policy).
