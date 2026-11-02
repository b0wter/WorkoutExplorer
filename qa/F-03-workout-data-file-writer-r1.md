# QA F-03 Workout data file writer - round 1

- **Date:** 2026-10-11
- **Verdict:** ready with conditions.
  - No Blocker or Major findings. ACs 1 to 5 are traced to code and passing tests.
  - AC 6 is met on the OQ-8 basis. The normative format section, `Decode` and the golden fixtures agree, and an independent decoder written from the document alone reproduces all 18 fixtures exactly. The cross-language check in TypeScript completes in F-04 QA (condition).
  - Three Minor findings are open, all routed to backend:
    - `Decode` panics on one crafted input.
    - `Encode` can write a file that its own `Decode` rejects.
    - One `Decode` rule has no test.
- **Inputs:**
  - Feature and decisions: `features/F-03-workout-data-file-writer.md`, `features/README.md`.
  - Specs: TD section "Workout data file" (Layout, Size, Versioning); Product spec NFR Performance (500 KB).
  - Architecture: `architect/F-03-workout-data-file-writer.md` v1.1 (Format §1–§9, W-1 to W-10, Tests, change log), ADR-005, ADR-006 rev. 1, ADR-004 rev. 3 item 7, `architect/overview.md` v1.4, `architect/INDEX.md`.
  - Code reviewed: the working tree on top of `f8a6c24` (`git diff f8a6c24` plus untracked files):
    - `internal/datafile`, `internal/precompress`, `internal/corpustest`;
    - `cmd/workout-explorer/parse.go` and `parse_test.go`;
    - `scripts/corpus.sh`, `Makefile`, `tools/corpusgen`;
    - `testdata/datafile/`, `testdata/corpus/MEASUREMENTS.md`.
  - Real files: three are present locally and git-ignored (manufacturers `development` ×2 and `wahoo_fitness`).

**Privacy note for this report.** It names manufacturers, sports, durations rounded to 0.5 h, sample counts, stream names and byte sizes for the real files. It contains no coordinates, times, dates, places or file names.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (exit code captured directly) | **exit 0**. `go vet` clean, `svelte-check` 0 errors and 0 warnings. `go test ./...`: 8 of 8 packages `ok`. Vitest 23/23 passed. `CGO_ENABLED=0` build OK. Bundle `10613 / 60000 bytes gzip PASS`. `npm ci` warns EBADENGINE (Node 22 locally, Node ≥ 24 required; known F-01 condition) |
| `go test -race -count=1 ./...` | all 8 packages `ok` (datafile 10.1 s) |
| `go test -count=1 -v -run 'TestSizeBudget4h\|TestRoundTripCorpus\|TestDataFileCorpus' ./internal/datafile` | All pass. `real: 3 files (local only)`. 21 round-trip subtests (3 real, 18 synthetic), 18 golden subtests. Log line: `ride-4h: data.bin 396156 bytes, data.bin.br 106951 bytes (budget 500000)` |
| `make corpus` | exit 0, `real: 3 files (local only)`, `corpus: 21 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown`. Every synthetic `bin`/`br` size equals the quality-9 rows in `MEASUREMENTS.md` |
| `go test ./internal/datafile -run '^$' -bench 12h -benchtime 10x -count 2` (i7-4770T) | `BenchmarkEncode12h` 10.0 to 12.8 ms/op, 8.8 MB/op. `BenchmarkBrotli12h` 117 to 119 ms/op, 51.9 to 52.3 MB/op. Matches the engineer's report (about 120 ms, 52 MB) |
| `go test ./internal/datafile -run '^$' -fuzz FuzzDecode -fuzztime 60s` | PASS, 2.44 M executions, no crash. It does **not** find QA-03-r1-1, which needs a crafted JSON field |
| `go list -deps -f '{{if not .Standard}}…' ./cmd/... ./internal/...` | Only `workout-explorer/...`, `github.com/muktihari/fit/...` and `github.com/andybalholm/brotli{,/matchfinder}` |
| Packages that import brotli in non-test code | only `internal/precompress`. Test-only importers: `cmd/workout-explorer`, `internal/datafile`, `internal/precompress` (allowed by ADR-006 item 2) |
| `go list -deps ./cmd/...` contains `corpustest`? | no |
| `go version -m build/workout-explorer` | `CGO_ENABLED=0`, `andybalholm/brotli v1.2.6` |
| `go mod tidy -diff`, `gofmt -l .`, `go vet ./...` | clean |
| `git status --porcelain` before and after the round | identical. The fuzz run wrote no `testdata/fuzz` corpus |

### Independent decoder (AC 6, OQ-8)

I wrote a throwaway Python decoder from the **format section and the Golden fixtures section only**, without reading the Go code first. It covers:
- the §1 header;
- the §2 table;
- the §3 delta reader;
- the §4 and §6 checks, including zero padding, column order, offsets, trailing bytes, `time_offset` strictly increasing and index ranges;
- the §5.4 index ranges, recomputed from `time_offset` and compared with the metadata;
- the fixture digest.

| Input | Result |
| --- | --- |
| All 18 `testdata/datafile/synthetic/*.bin` | **18 of 18 fixtures reproduced exactly**, compared as JSON values against the committed `.json`. This covers counts, first, last, min, max, digest, `byte_offset`, `byte_length` and `metadata_length` |
| The 3 real-file outputs in `build/corpus/real/` | All three decode and pass every check |
| §8 worked example, by hand | Header bytes (`91 01 00 00` for L = 401), quantisation, deltas (`542`, `49`), padding, offsets 416/436/456/476/484 and length 489 are all consistent. `TestWorkedExample` asserts the same column bytes |
| Metadata schema (keys and their order: top level, `time_zone`, `summary` and totals as an ordered subset of §5.2, session, lap; `end ≥ start`; offset a multiple of 900; compact JSON; no trailing newline) | 21 of 21 files conform (18 synthetic, 3 real) |

Gaps I noticed while writing the decoder. None of them blocked decoding:
- The Golden fixtures key table does not list the top-level keys (`fixture_version`, `byte_length`, …). They are clear from the example.
- §6 does not say whether a reader must enforce the §2 column order or the latitude/longitude pairing. My decoder enforced both; Go `Decode` enforces neither. Both accept every fixture.

### Real-file sizes (by manufacturer; local only, ADR-004 item 7 rev. 3)

| Manufacturer | Sport | Duration | Samples | Present streams | `.bin` bytes | `.br` bytes | `.br` per 4 h |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `development` | cycling | 0.5 h | 2,456 | time_offset, latitude, longitude, altitude, distance, speed, heart_rate | 58,464 | 12,018 | — (< 1 h) |
| `development` | running | 0.5 h | 2,377 | the above + cadence, power | 65,606 | 12,619 | — (< 1 h) |
| `wahoo_fitness` | cycling | 2.0 h | 6,967 | time_offset, latitude, longitude, altitude, distance, speed, temperature | 161,787 | 36,482 | 75,404 |

The largest extrapolation is 75 KB per 4 h, against the 500,000-byte budget and the 250,000-byte quality trigger in ADR-006. The strict round-trip tolerance passes on all three files, so no value was clamped.

### Mutation probes (scratch copy of `src/server`, `go test ./internal/datafile ./cmd/...`)

32 mutants, 31 killed. Two did not compile in their first form and were rewritten so they did.

| Area | Mutants | Killed by |
| --- | --- | --- |
| Quantisation | `Round` changed to `Floor`, `RoundToEven` or `Trunc`; speed factor 1000 changed to 100 | corpus round trip, golden bytes, `TestQuantisation` |
| Clamping | `hi+1`, which makes the sentinel reachable; lower clamp removed; temperature minimum −127 | `TestClamping`, `TestRoundTripRandom` |
| Delta | `prev` reset at a missing sample; writer sentinel `MaxInt32−1`; first value not stored absolute | round trip, golden bytes |
| Alignment and length | `align2`; no padding between columns; trailing padding written; a single W-8 pass | round trip |
| Presence and pairing | all-NaN stream written; pairing loop removed; nil-side rule removed | `TestOrderAndPresence`, `TestPairing` |
| Metadata | `indexRange` using `>=`; `-0` kept; totals rounded to 2 decimals; offset of an unknown time zone written | golden fixtures, `TestMetadata`, `TestTotalFormat` |
| `Decode` §6 | trailing bytes allowed; alignment, overlap, descriptor, UTF-8, missing-key, *N* = 0 and lap-range checks each removed; raw sentinel ignored; time-increasing check removed | `TestDecodeErrors`, round trip |
| **Survived** | `Decode`'s **session** index-range check disabled | — (QA-03-r1-3) |

## Findings

### Minor

- **ID:** QA-03-r1-1
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/internal/datafile/decode.go:68-80`
- **Relates to:** Format §6 ("rejects the file, with an error and never a crash"), the `Decode` doc comment, `FuzzDecode`
- **Evidence:**
  - Input: a 1-sample file with only `time_offset`, whose descriptor `byte_offset` is `9223372036854775804` (a multiple of 4), padded so the metadata length is unchanged (660 bytes in total).
  - Result: `Decode` panics with `runtime error: slice bounds out of range [:-9223372036854775808]`.
  - Cause: `end = off + n*int64(spec.size)` overflows int64 and becomes negative. The `end > len(b)` bounds check then passes, and `b[off:end]` panics.
  - Reproduced in a scratch probe test.
  - The 60 s fuzz run did not find it.
- **Consequence:**
  - Today only tests call `Decode`, so nothing in production is exposed.
  - The documented contract is still wrong, and the fuzz test gives false confidence.
  - Any later use on stored or uploaded bytes (F-16 tests, a `regenerate` self-check) would inherit the crash.
  - A JavaScript reader cannot hit this case, because a `Number` larger than the file length is rejected anyway.
  - Suggested fix: check `off > len(b)` before adding, or compare against `len(b) - off`. Add this input as a `FuzzDecode` seed.

- **ID:** QA-03-r1-2
- **Severity / kind:** Minor / risk
- **Route:** backend (the architect may want W-1 to list the check)
- **Where:** `src/server/internal/datafile/encode.go:326-338`, together with W-1
- **Relates to:** W-1 ("`Encode` checks the invariants it relies on and returns an error"), Format §5.3 (`end ≥ start`), §6 rule 7
- **Evidence:** `Encode` of `offsetsOnly(5)` with `Laps = [{Start: 3, End: 1}]` returns no error. `Decode` of that output fails with `datafile: lap 0 index range [3, 2) outside [0, 5]`.
- **Consequence:**
  - F-02 E-2 maps an end before its start to the start, so `ingest.Parse` cannot produce this case today.
  - F-16 (hiding) and F-30 (regenerate) will pass modified `workout.Workout` values to `Encode`. A lap with `End < Start`, which can happen if trimming or hiding ever touches laps, would then be published silently in a file that the strict reader rejects (F-04 rule 7 is optional, so the browser may accept it with a negative range).
  - Suggested fix: `Encode` returns `datafile: lap N ends before it starts` (or clamps as E-2 does), and W-1 lists the check.

- **ID:** QA-03-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/datafile/datafile_test.go:564-569`; code at `decode.go:99-103`
- **Relates to:** Format §6 rule 7; design Tests table, row "Decode errors" ("index range outside [0, *N*]")
- **Evidence:** I disabled the session loop in `Decode`'s rule 7 check. Every test in `./internal/datafile` and `./cmd/...` still passes. Only lap ranges are tested.
- **Consequence:** A regression in the session check would go unnoticed. Sessions select the per-sport ranges for the cadence label (§2), so a wrong session range matters as much as a wrong lap range.

## Previous findings

None. This is round 1.

## Questions

1. **Architect: size of the golden `.bin` files in git (ADR-005 Consequences).**
   - `testdata/datafile/` is 2.4 MB. `ride-12h.bin` alone is 1.18 MB, and every normaliser or format change rewrites most of it.
   - The ADR accepts this cost "for a reviewable, cross-language contract".
   - Does F-04 (or F-07) actually need a committed 12-hour `.bin`? Or would the 4-hour file plus `ride-12h.json` do, with a Go step generating the 12-hour file before the TypeScript tests?
   - What would settle it: either a sentence in ADR-005 naming which F-04/F-07 tests need the committed 12-hour golden file, or dropping that file from the golden set.
2. **Architect: ignore rule for data files (ADR-004 item 7 rev. 3).**
   - All real-derived files that F-03 produces go under `/build/`, which is ignored. I verified this with `git check-ignore` on every file in `build/corpus/real/`, `git add --dry-run -A`, and a fixed-string search for 59 values derived from the real data files (start times, digests, position statistics, sizes) over all non-ignored files: 0 hits.
   - However, `.gitignore` has no rule for `*.bin`/`*.bin.br` outside `build/`. For example, `parse -datafile ./x.bin real.fit` run from the repository root would produce an addable file that contains coordinates.
   - Should the ignore block get `*.bin`, `*.bin.br` with `!/src/server/testdata/datafile/synthetic/*.bin`? A yes or no settles it.
3. **Architect / F-09: brotli memory against the ADR-006 trigger.**
   - `BenchmarkBrotli12h` measures 52 MB/op. The cost trigger is 64 MB/op, so we are at 81 % of it.
   - Nothing is wrong now. F-09 and F-15 should decide whether compressions run one at a time on the VPS (publish together with on-deploy `regenerate`).
   - What would settle it: a line in the F-09 or F-15 design.

## User conditions

- **F-03 AC 6 / OQ-8:** the cross-language check completes in F-04 QA (F-04 AC 2 reads `testdata/datafile/synthetic/`). F-03 QA verified that the description, `Decode` and the fixtures agree (see "Independent decoder").
- **From F-02 (OQ-6):** AC 2 still needs real files from a third FIT writer. The present files come from `development` and `wahoo_fitness`.
- **From F-01:** a real GitHub Actions run (with the Go cache on), Node 24 and the `@v7` tags. None can be checked here: there is no remote, and Node 22 is installed locally.
- OQ-4 and OQ-5 are unchanged and not affected by F-03.

## Coverage

| AC | Code | Test(s) | Status |
| --- | --- | --- | --- |
| 1 Header `WEXD`, version, *N* for every corpus file | `encode.go:367-372` | `TestHeader`; `TestRoundTripCorpus` (18 synthetic in CI and 3 real here); `make corpus` (21 ok) | traced, passing |
| 2 Every column offset divisible by 4 | `encode.go:349-363`, `align4` | `TestAlignment` (N = 1, 2, 3, 5; uint8-only; no trailing padding); `TestRoundTripCorpus`; doc decoder on all fixtures | traced, passing; align2 and no-padding mutants killed |
| 3 Round trip within resolution, sentinels where missing | `quantise.go`, `encode.go:381-472`, `decode.go:170-196` | `TestRoundTripCorpus` (tolerance scale/2 + 1e-9, NaN in exactly the same places), `TestRoundTripRandom` (50 workouts), `TestDelta`, `TestQuantisation`, `TestClamping` | traced, passing; rounding, delta and sentinel mutants killed |
| 4 Indoor: no latitude/longitude columns or descriptors | `encode.go:414-432` | `TestRoundTripCorpus` on `indoor-no-gps`, `indoor-no-distance-no-gps` and `pool-swim` (also asserts these files exist); `TestPairing`; those three fixture JSONs have 0 position entries | traced, passing |
| 5 4-hour ride under 500 KB as `.br`; sizes recorded | `precompress.go` | `TestSizeBudget4h` (asserts 14,400 samples and all 10 streams, so it cannot pass vacuously): 106,951 bytes. `MEASUREMENTS.md` has synthetic sizes and benchmarks; the real sizes are in this report | traced, passing |
| 6 Format description exists and matches the writer | F-03 design §1–§9, `decode.go`, `testdata/datafile/` | `TestDataFileCorpus` (byte for byte, new/stale, offsets digest equals the report's), `TestWorkedExample`, `TestFixtureWorkedExample`, `TestDecodeErrors`, the independent doc-only decoder | met for F-03 (OQ-8); the TypeScript check is in F-04 |

Requirements and rules checked and found correct:
- **NFR Performance:** 4 h at 106,951 bytes `.br`. The bound "by construction" holds: 27 bytes per sample, `ride-4h.bin` 396,156 bytes.
- **AP-4:** absent streams are not written.
- **SH-4 / D-7:** the metadata holds no coordinates. Checked by `TestMetadataHasNoCoordinates`, the closed key set on 21 files, and `Encode` errors that carry no values.
- **Encoder rules:** W-1 to W-10 (with W-1 completeness as in QA-03-r1-2). W-9 determinism is covered by `TestDeterminism` and the byte-for-byte golden files.
- **ADR-005 items 1–10.**
- **ADR-006:**
  - items 1–4: pinned v1.2.6, sole importer, quality 9 with window 22, `.br` decompresses to the `.bin` (tested in `TestRoundTripCorpus` and `TestParseDatafile`);
  - item 5: benchmark recorded.
- **ADR-004 rev. 3 item 7:** real outputs only under `build/`, no real rows in `MEASUREMENTS.md`, no golden files on the real side.
- **Dependency rules (overview §5 v1.4):** `datafile` imports only `workout` and the standard library and does no I/O; `corpustest` is not in the binary.
- **CLI (D-12):** `-datafile` builds everything in memory before writing. The tests cover: exit 1 with nothing on stdout for a missing folder, exit 2 without a FIT argument, and stdout equal to the golden report.
- **`corpus.sh`:** size suffix on `ok`/`DIFF`/`NEW`; counts unchanged.
- **`ride-4h.fit`:** reproduced by `TestSyntheticCorpusCurrent`, and its corpus README row is present.

Not run:
- `make corpus-update`, because it rewrites golden files. `TestDataFileCorpus` without `-update` passes, so the committed files are current.
- The `-update` stale-removal path.
- A TypeScript decoder (F-04).
- A GitHub Actions run (no remote).
- Fuzzing beyond 60 s.
