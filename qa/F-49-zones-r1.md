# QA F-49 Heart-rate and power zones - round 1

- **Date:** 2026-11-13
- **Verdict:** ready. No Blocker or Major. Four Minor findings, listed below. The ZN-9 entry overrun (+265 gzip against a 150 ceiling) is a user decision that is still open. I found no cheaper option that keeps F-03 §6 rule 4, so it is not a finding here (measurements are under Build and test results).
- **Inputs:** `features/F-49-zones.md`; `architect/F-49-zones.md` (ZN-1 to ZN-9); ADR-049; F-03 §5.1, §5.9 and §6 rule 4 (v1.9); F-15 P-7 (zone fields on the allow-list); ADR-011 item 4; migration `0007_zones.sql`; `git diff 8e0f05b..d84a8dd`. Working tree clean at d84a8dd. OQ-195 was built as suggested.

## Build and test results

- `make ci`: exit 0. It covers lint, Go and web tests, build, size, caddy-check and docker-check.
- `npx vitest run`: 71 files, 1,862 tests passed.
- `npm run size`: entry `index.html` 56,916 / 60,000 bytes gzip, PASS. That is +265 over F-48's 56,651. ZN-9's ceiling is 150.
  - I split the cost in a scratch copy: the `zones` validator in `meta.ts` costs about 141, the `import()` glue in `page/start.ts` about 95, the rest (key list, types) about 29.
  - Moving the validator out of the entry would break F-03 §6 rule 4 ("rejected by both readers" at decode). I found no cheaper compliant option.
  - ZN-9 asks for the measurement to be recorded in ADR-011 item 4. It is not recorded yet; that waits for the user's decision.
- `go test -race ./...`: all ok.
  - `internal/publish` took 427 s, of which F-49's three tests take 11 s.
  - A first run timed out at the 600 s default because it shared the CPU with my mutant runs. The package was already slow before F-49, so this is not an F-49 finding.
- **Mutants**, run in a scratch copy of d84a8dd. All 9 were killed:
  - time_in_zone and zone messages read the wrong way round (tops as bottoms);
  - the cap kept as a zone;
  - a kind written without its stream (encoder);
  - a percent list not converted;
  - samples counted instead of moving time;
  - the selection ignored;
  - the edit page's trim window ignored;
  - the sharer's basis ignored on the edit page;
  - the TS reader accepting both `from` and `basis`.
  - The "`session.max_heart_rate` as basis" mutant has no code path to mutate: ingest never reads it for zones. `TestZonePrivacy` pins the expected `from`, so a basis of 201 would fail it.
- **Range drag on `ride-12h`:** run in Node only, not in a browser. I used the 43,200-sample golden, a sharer basis for both kinds (HR 185, FTP 250) and the moving-time stream, over the full range. One update (both kinds) took 1.6 ms median, 2.0 ms p95, 2.9 ms max. The probe is in the scratchpad (`bench/bench.test.ts`).

## Findings

### Minor

- **ID:** QA-49-r1-1
- **Severity / kind:** Minor / defect (document)
- **Route:** architect
- **Where:** `architect/F-49-zones.md:45` (Verification, Privacy row)
- **Relates to:** NFR Privacy, AP-2, OQ-186
- **Evidence:** The row says `session.max_heart_rate` must appear in no file. But `session.max_heart_rate` is the summary's `max_heart_rate` total (F-03 §5.2). AP-2 shows it, and it is in `draft.bin`, the public data file, `index.html` and `summary_json`.
  - The backend's reading is the right one: it is never a zone value. `TestZonePrivacy` leaves 201 out of the markers and pins `zones` to `[114,133,152,171]`.
  - The same row also asks for checks of decompressed `.br`, every public file and the database row. The test checks only the metadata of the public data file and of `draft.bin`. For resting heart rate, threshold heart rate and the device totals this is enough by construction: `ingest` never reads them (ZN-1), and `index.html` and `summary_json` come from the same metadata.
- **Consequence:** A later reader may "fix" the code to keep the workout maximum out, or think the test is incomplete. Reword it to: "`session.max_heart_rate` is never a zone value", and "no marker in any data file's metadata (draft and public); the other files are derived from it".

- **ID:** QA-49-r1-2
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/edit/edit.dom.test.ts:1478`, `src/web/src/zones/zones.dom.test.ts:89`; Verification "Edit equals public" row
- **Relates to:** ZN-5, ZN-7, AC 6 and AC 9 (edit and viewing pages agree under a trim)
- **Evidence:** The edit side is tested: the saved trim window bounds the count, and the mutant that ignores the window is killed. The public side is tested in Go: `TestZonesTrimmed` checks the zone values.
  - No test computes `lib/zones` over a published trimmed data file and compares it with the edit page's window over `draft.bin`.
  - Time is counted from the stored `moving_time` stream. Any difference between the full and the trimmed file's moving-time deltas at the window edges would go unnoticed.
  - I did not prove a difference exists.
- **Consequence:** If moving-time detection at a trim edge ever differs, the edit page and the public page show different zone times and no test fails. Fix: add one golden of a trimmed zone ride (for example `zones-wahoo-ride` with a trim, like `ride-30min-cut-60`), then one vitest comparing both computations.

- **ID:** QA-49-r1-3
- **Severity / kind:** Minor / risk (not verified)
- **Route:** frontend (or main session at the next manual browser check)
- **Where:** `src/web/src/zones/index.ts` (`update`, subscribed to selection flushes)
- **Relates to:** AC 11 (60 fps scrubbing, `ride-12h` unchanged), ZN-8
- **Evidence:** In Node, one update over the whole `ride-12h` took about 1.6 ms for both kinds (see Build and test results). That fits the frame budget, but the range drag already does other work in the same frame.
  - Not checked: a browser run of the `?measure` recorder with the section active.
  - Not checked: LCP.
- **Consequence:** Probably fine. If the browser check regresses, ZN-8 already names the fix: recompute when the drag ends.

- **ID:** QA-49-r1-4
- **Severity / kind:** Minor / preference (misleading comment)
- **Route:** backend
- **Where:** `src/server/internal/publish/publish.go:315-317`
- **Relates to:** F-15 P-7, ZN-4
- **Evidence:** `ZonesMaxHeartRate` and `ZonesThresholdPower` sit in the same `case` as `Title`. That case is followed by the comment "never read by the public build … a build that starts reading one must take it off this list". The public build does read both zone fields (`publicTransform`).
  - The misplacement is older: the comment belongs to the next case (`Sport`, …). The new fields now make it read as an instruction to remove them.
- **Consequence:** Someone following the comment would remove the zone fields from the list. A zone change would then rebuild remove-first: a needless outage of the page, not a privacy problem. Fix: move the comment under the `Sport` case.

## Engineer-reported items (judged)

- **Entry +265 against ZN-9's 150:** no cheaper compliant option found; see the budget measurements under Build and test results. This is the user's decision.
- **`session.max_heart_rate` wording:** the backend's reading is correct. The design's wording is the defect; see QA-49-r1-1.
- **Decoder checks `from` against the full stream range (0–254, 0–65,534):** this matches F-03 §5.9 ("within the stream's valid stored range"). The writer is stricter (≥ 1). Go and TS agree. Fine.
- **`MaxSessions` bounds the kept lists:**
  - `add` caps every basis slice, and `tizHR` and `tizPower` are capped.
  - `sessFTP` relies on the session budget.
  - `hr_zone` and `power_zone` stop at 10 values and drop the list at the 11th.
  - Memory is bounded. Fine.
- **Percent detection ignores invalid values:** a NaN inside the list stays NaN after conversion, and `zoneFrom` rejects it. Trailing NaNs are trimmed first. Fine.
- **`novalidate` on the edit form:** the only other constrained inputs are the trim range sliders, which cannot be invalid. `zoneError` refuses non-integers, out-of-range values and `badInput`, and the server refuses them again (`parsePatch` rejects `150.5`, strings and overflow; `ApplyPatch` checks the range; the migration has `CHECK`). Fine.
- **"Edit equals public" check not built:** see QA-49-r1-2.
- **Range-drag cost:** about 1.6 ms per update for both kinds in Node; no browser check (QA-49-r1-3).

## Previous findings

None (round 1).

## Questions

1. **ZN-9 overrun (+265 against 150):** accept it and book 265 in ADR-011 item 4 (reserve 3,349 → 3,084), or find a remedy under ADR-011 decision 5? Suggestion: accept and record it. The validator is required by F-03 §6, and the glue is the minimum for a lazy chunk. Evidence that would change this: an architect decision to move zone validation into the lazy chunk (a format-contract change).

## Coverage

| AC | Code | Test | Status |
| --- | --- | --- | --- |
| 1 Apple-style, bpm from the configured max, times add up | `ingest/zones.go` hrList, `workout/zones.go` zoneFrom (bottoms, cap) | `TestHRList`, `TestZoneFrom`, `TestZonePrivacy`, `zones.test.ts` AC 1 | traced. The corpus file uses bpm values; the percent path is unit-tested only. Main session's real-file check is consistent |
| 2 Wahoo-style tops | zoneReader hrZone and powerZone, tops branch | `TestSharerZones` (`from` values), `zones.dom.test.ts` AC 2 | traced |
| 3 FTP only: power by model, no HR | pickKind bases | `zones.test.ts` AC 3, 4 | traced |
| 4 `zones_target` | zonesTarget | `zones.test.ts` AC 3, 4; `TestZonesTrimmed` (basis 186 and 240) | traced |
| 5 None (Garmin-style); no power, no power zones | encoder `zonesOf`, `page/start.ts` condition | `zones.dom.test.ts` AC 3, 5; page.dom "no zone values, no zones chunk" | traced |
| 6 Sharer values, edit and view, clear | httpapi, lifecycle, store, `publicTransform`, edit glue | `TestSharerZones`, `TestZoneColumns`, lifecycle and owner tests, edit.dom F-49 block | traced |
| 7 Deleted with the workout, nowhere else | row columns only; no log; not in `draft.bin` | `TestZoneColumns`; no logging in the owner path (read) | traced |
| 8 Range follows the selection | `zones/index.ts` update | `zones.dom.test.ts` AC 8, `zones.test.ts` AC 8, mutant killed | traced |
| 9 Trim on both pages | edit window `trimIndex`, public build after the trim | edit.dom TR-9 test, `TestZonesTrimmed`, mutant killed | traced, with the gap in QA-49-r1-2 |
| 10 Truncated and multisport | pickKind (first session's list) | `zones.test.ts` AC 10, cut-60 corpus golden | traced |
| 11 Not in the entry; size, LCP, scrubbing | lazy `src/zones/`; never recomputes on a cursor flush | page.dom chunk test, `zones.dom.test.ts` cursor case | entry contents traced. The data file grows 8–12 bytes (`ride-12h` +12). LCP and the browser scrubbing check were not run (QA-49-r1-3) |

Not run: a browser frame or LCP measurement; real FIT files (the main session checked them locally).
