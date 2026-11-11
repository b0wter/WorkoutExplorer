# QA F-45 Suggested title - round 1

- **Date:** 2026-11-11
- **Verdict:** ready. No Blocker or Major. The owner-side town and the public town agree in every case I tried (96/96 corpus cases, 800/800 random tracks). Nothing logs the town or title, and nothing sends them anywhere. Five Minor findings remain.
- **Inputs:** `features/F-45-suggested-title.md`; UP-4, UP-5 (product spec); `architect/F-45-suggested-title.md` (ST-1 to ST-10); ADR-045; ADR-030; decision log OQ-154 to OQ-163 (`features/README.md`). Code: commits bb5338e (backend) and dcc2d31 (frontend). Working tree clean at dcc2d31. Commits fa55ea7 and 3e21ba7 are out of scope.

## Build and test results

- `make ci`: exit 0. Includes `go vet`, `svelte-check` (0 errors, 0 warnings), `go test -count=1 ./...` (all ok), vitest (67 files, 1,699 tests passed), the web build, the size check, caddy-check and docker-check.
- Size check: `edit.html` and `index.html` entries are 56,661 / 60,000 bytes gzip, PASS. `upload.html` is 3,785 / 8,000.
- `go test -race -count=1` on places, page, lifecycle, publish, httpapi and datafile: all ok.
- Probes in the scratchpad, run with `go test -overlay` so that `src/` stays unchanged:
  - **Agreement.** For each case I compared two first points: the owner side, `FirstPositionAt(draft.bin, HiddenRanges if hide)`, and the public side, `HideEnds`, then Encode, Decode, `FirstPosition`. I compared the full `Suggest` string too.
    - Corpus, every synthetic and local real file, hiding on and off: 96/96 cases agree. This includes both triathlons, brick, chained files, gps-dropout, pool and indoor.
    - Random tracks: 800/800 agree. They include starting GPS gaps, gap-filled pauses, routes that come back to the start, short fully hidden routes and two-session multisport.
  - **Cost** (8 cores, load average about 4.5):
    - `places.Parse`: 31,821 towns, 2,157,496 bytes of heap kept (the limit is 4 MB), about 16 ms once per process.
    - `Nearest`: about 3.7 ms per lookup with 0 allocations, at three different points.
    - `FirstPositionAt` on ride-12h (43,200 samples), fully hidden: 3.9 to 4.3 ms, 86,880 allocations and 390 KB per call. With no skip: about 0.4 to 0.6 ms.
  - **Binary size:** 24,625,525 bytes at bb5338e^ and 25,550,606 bytes at HEAD, so +925,081. `places.tsv` is 896,834 bytes.
  - **Entry bundle:** I built `src/web` at 3e21ba7 into the scratchpad and compared its `main-*.js` with HEAD's. The raw size is the same (135,152 bytes). The only differences are four chunk-hash strings, for `start.svelte-*.js` (twice) and `map-view-*.js` (twice). The 2 extra gzip bytes come from those hashes. No code was added to the viewing page, which matches ST-7.
  - **Mutant:** `d <= best && (!ok || d < best)` changed to `d < best` in `places.Nearest`. `go test ./internal/places/` still passes.

## Findings

### Minor

- **ID:** QA-45-r1-1
- **Severity / kind:** Minor / risk
- **Route:** backend (or architect, if the threshold should change)
- **Where:** `src/server/internal/places/places.go:63-77`
- **Relates to:** F-45 ST-2 (shortcut: "add one only if a lookup measures above 2 ms"), ST-9
- **Evidence:** `testing.Benchmark` of `Nearest` on the embedded table: 3.68 to 3.77 ms per lookup, 0 allocations. That is about 115 ns per haversine (eight trig and root calls each). The design assumed about 1 ms.
- **Consequence:** The design's own trigger for adding a spatial index has been reached on this machine. The cost is small: one lookup per public build and one per owner GET, PATCH or publish. A latitude-band check before the haversine, or a grid, would bring it under the trigger. The other option is to record a new threshold. One of the two should be done.

- **ID:** QA-45-r1-2
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/internal/datafile/position.go:66-93`
- **Relates to:** ST-6 ("memory does not grow with the sample count")
- **Evidence:** On a fully hidden 43,200-sample draft, the probe measured 86,880 allocations and 390 KB per call. That is two heap allocations per sample: the `[4]byte` passed to `io.ReadFull` escapes, plus the closure call. For `[[0, N-1]]` the loop reads both columns to the end, although the skip ranges already show that no sample is left. Retained memory stays constant.
- **Consequence:** Each owner settings read of a long, fully hidden workout creates about 0.4 MB of garbage and takes about 4 ms. The design's claim holds only for retained memory.

- **ID:** QA-45-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:**
  - `src/server/internal/places/places_test.go:39-63`
  - `src/server/internal/publish/suggest_test.go:38-47`
  - no test for the design's "Agreement" row
- **Relates to:** design section Verification (mutants, Agreement, AC 35), AC 34, AC 35
- **Evidence:**
  - (a) The `<` for `≤` mutant at 25 km survives. The bisection point `lo` is strictly inside the radius, so no test hits the exact boundary.
  - (b) No test in the suite compares `suggested_title` with the public title across the corpus with hiding on and off. My probe shows they agree today, but nothing protects that.
  - (c) AC 35 (off to on on an already published workout, remove-first) is not tested directly. The publish test builds a new fixture for each case.
  - (d) The "Aville in no public file" byte scan cannot fail for `.br` files (compressed) or the preview PNG (pixels).
- **Consequence:** A change to `FirstPositionAt`, `HideEnds` or the radius test could reach `main` with the suite still green.

- **ID:** QA-45-r1-4
- **Severity / kind:** Minor / deviation
- **Route:** backend
- **Where:** `src/server/internal/places/places.go:56-59`
- **Relates to:** ST-2 ("reports unavailable, logged once without data")
- **Evidence:** `Default()` drops the `Parse` error (`def, _ = Parse(table)`) and logs nothing.
- **Consequence:** If the embedded table were broken, every suggestion would quietly fall back to sport and date. `make ci` would catch it first, so the risk is low.

- **ID:** QA-45-r1-5
- **Severity / kind:** Minor / risk (licence)
- **Route:** backend (`sources.json`)
- **Where:** `src/server/tools/attributions/sources.json` (GeoNames entry); rendered at `build/web/about.html:113`
- **Relates to:** AC 40, INF-1, ST-10
- **Evidence:** The rendered line has the right name, the "(modified)" note and "CC BY 4.0". But the licence anchor points to `https://www.geonames.org/`, not to the licence. CC BY 4.0 §3(a)(1)(C) asks for the licence URI or a hyperlink to it.
- **Consequence:** The attribution is weaker than the licence asks. The fix is to link to `https://creativecommons.org/licenses/by/4.0/`, or to keep the GeoNames link and add the licence link. The "(modified)" note does not mention the dropped PPLX rows. That is acceptable, because "indicate changes" does not ask for detail.

## Previous findings
None (round 1).

## Questions

1. **AC 41: the binary grew by 925,081 bytes, but the table is 896,834 bytes.** Read literally, "no more than the size of the bundled place data" is exceeded by 28 KB, which is the new code. Suggestion: read the AC as "about the table's size", the wording ST-9 and ADR-045 already use. It is settled if the requirements engineer confirms this or rewords the AC.
2. **`triathlon-cut-90` is called "multisport".** Truncation drops the run session, so the page also shows only swim and bike sections. The title agrees with the page, and I think that is correct. It is settled if the user agrees that a lost run session means "not a triathlon".

## Coverage

| AC | Code | Test | Result |
| --- | --- | --- | --- |
| 1 Köln 07:30, header, tab, card, placeholder, field empty | `page/suggest.go`, `page.Build`, `view.ts`, `start.svelte.ts` | `TestSuggest`, `TestBuildUsesSuggestionOnlyForEmptyTitle`, `TestEmbeddedTable` (Köln), edit.dom "F-45 ST-7", cmd e2e og:title | traced |
| 2 Time-of-day boundaries | `timeOfDay` | `TestSuggestTimeBoundaries` | traced |
| 3 Workout offset, else UTC, no start time | `Suggest` | `TestSuggest` | traced |
| 4 Town of the first public point (A/B) | `publish.build`, `lifecycle.SuggestedTitle` | publish and lifecycle `TestSuggestedTitle*`; my probe (agreement) | traced |
| 5 Hiding toggle within SH-10 | existing ADR-028 rebuild | none direct (r1-3c) | traced to code only |
| 6 Indoor, pool, fully hidden, beyond 25 km; 24.9 km | `Nearest`, `FirstPosition*` | `TestSuggestedTitleWithoutTown`, `TestNearestRadius` | traced (r1-3a) |
| 7 Triathlon vs multisport | `suggestSport` | `TestSuggest`; my probe on synthetic triathlon and brick | traced |
| 8 No third-party request | `places` has no net import; `refresh-places.sh` is manual only | none | reviewed by reading; no network log run |
| 9 Typed title wins, clearing restores | `page.Build`, `view.ts` | publish test, edit.dom test | traced |
| 10 PPLX gives the city | `refresh-places.sh` filter | `TestEmbeddedTable` (Köln, Dubai) | traced |
| 11 Delete confirmation keeps sport and date | `RowTitle` unchanged | `TestRowTitleKeepsSportAndDate` | traced |
| 12 About lists GeoNames, CC BY 4.0 | `sources.json` | rendered `about.html` checked | traced (r1-5) |
| 13 Corpus, sizes, binary | no data file or schema change | `make ci` corpus and goldens; my binary probe | traced (Q1); LCP not measured |

Other checks I made, all correct:

- `PATCH {"suggested_title":"x"}` gets 400 (`httpapi/owner_test.go:196`).
- No log line in places, publish, lifecycle, httpapi or cmd includes the town or the suggestion. Pushover and `delete.go` use only the typed title.
- `places` imports only the standard library and `workout`.
- ST-1's checks on the table run in `make ci`: more than 20,000 rows, UTF-8, no Cc or Cf characters, names of at most 60 runes, so the suggestion stays within 100 code points.
- The suggestion is not stored in the row or the data file.

Not done:

- No manual browser network log.
- No LCP measurement.
- No server-hardware timing. All figures above are from the dev machine.
