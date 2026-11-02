# QA F-03 Workout data file writer - round 2

- **Date:** 2026-10-11
- **Verdict:** ready with conditions.
  - QA-03-r1-1, r1-2 and r1-3 are fixed. All three r1 questions and both r1 doc gaps are settled in design v1.2 and ADR-004 rev. 4.
  - No new findings. No regressions: the golden files are unchanged, and every size is identical to r1.
  - AC 6 is met on the OQ-8 basis. The doc-only decoder, updated to the v1.2 §6 rules, again reproduces all 18 fixtures. It also agrees with Go `Decode` on 19 crafted files.
  - What remains are conditions only: the cross-language check in F-04 QA (OQ-8), and the user's F-02 and F-01 items.
- **Inputs:**
  - Architecture: `architect/F-03-workout-data-file-writer.md` v1.2 (change log, §6 rules 5–7, W-1, Pre-compression, `.gitignore`, `ride-12h` golden), ADR-004 rev. 4, ADR-005, ADR-006 rev. 1, `architect/INDEX.md`.
  - Previous round: `qa/F-03-workout-data-file-writer-r1.md`.
  - Code reviewed: the working tree on top of `f8a6c24`, compared file by file against my r1 snapshot of `src/server` (`diff -r`). Changes are in:
    - `internal/datafile/{datafile,decode,encode}.go`;
    - `datafile_test.go`, `helpers_test.go`;
    - `internal/precompress/precompress{,_test}.go`;
    - `.gitignore`.
  - `testdata/` is unchanged since r1 (`diff -rq`).
  - Real files: still three, local and git-ignored (`development` ×2, `wahoo_fitness`).

**Privacy note for this report.** It names manufacturers and pass/fail results only. It contains no coordinates, times, dates, places or file names from real files.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` (exit code captured directly) | **exit 0**. `svelte-check` 0 errors and 0 warnings. `go test ./...`: 8 of 8 `ok`. Vitest 23/23. Bundle `10613 / 60000 bytes gzip PASS` |
| `go test -race -count=1 ./...` | 8 of 8 `ok` |
| `go vet ./...`, `gofmt -l .`, `go mod tidy -diff` | clean |
| `make corpus` | exit 0, `real: 3 files (local only)`, `corpus: 21 ok, 0 diff, 0 fail, 0 new, 0 stale, 0 unknown`. All 21 `bin`/`br` sizes are identical to r1 (ride-4h: `bin 396156 br 106951`) |
| `TestSizeBudget4h -v` | PASS. `data.bin 396156 bytes, data.bin.br 106951 bytes (budget 500000)` |
| `TestDecodeErrors` / `TestEncodeErrors` | PASS. 35 and 9 subtests (were 22 and 5) |
| `go test -race -count=30 -run TestBrotliOneAtATime ./internal/precompress` | `ok`: 30 runs, not flaky |
| `go test ./internal/datafile -run '^$' -fuzz FuzzDecode -fuzztime 60s` | PASS, no crash, no `testdata/fuzz` written. The r1-1 input is now a seed (`datafile_test.go`, `FuzzDecode`) |
| `git status --porcelain` before and after the round | identical |

### Doc-only decoder against v1.2 (AC 6, OQ-8)

I extended my r1 Python decoder from the **v1.2 text of §6** only:
- rule 5: §2 order, and `latitude`/`longitude` both present or both absent;
- rule 6: `byte_offset ≤ len` and `N × size ≤ len − byte_offset`;
- rule 7: lat/lon missing at the same indices, and `end ≥ start` for sessions and laps.

| Input | Result |
| --- | --- |
| 18 golden `synthetic/*.bin` | **18 of 18 fixtures reproduced exactly** |
| 3 real outputs in `build/corpus/real/` | all decode and pass every check |
| 19 crafted files built from `gps-dropout.bin` (metadata rewritten in place, lap totals emptied for slack). Cases: streams out of order (altitude/distance; longitude before latitude); latitude without longitude and the reverse; half a position at one index; lap and session `end < start`; session `end_index` > *N*; session range reversed; `byte_offset` > file length, = 4,294,967,292, = 2⁶³−4, and = 2⁶³−4 with *N* = 2³²−1; *N* + 1 in the header; unknown name; repeated name; non-increasing time. Two accept cases: `scale` written `1e-06` → `0.000001`, and the unmodified file with slack | **19 of 19: the same accept/reject verdict from the doc decoder and Go `Decode`**. No panic. Go's error messages name the rule the doc decoder hit |

Both r1 doc gaps are closed:
- The Golden fixtures section now lists `fixture_version`, `format_version`, `byte_length`, `sample_count` and `metadata_length`.
- §6 rule 5 now states column order and pair presence for every reader; rule 7 states per-index pairing for Go.

§2 and §5.5 both give `1e-06`, which is what the writer emits.

### Mutation probes (fresh scratch copy; `go test ./internal/datafile ./internal/precompress ./cmd/...`)

44 mutants: the 32 from r1 plus 12 for the r1 fixes and the v1.2 rules. 42 were killed, and 2 survived as **equivalent mutants**.

| New mutant | Killed by |
| --- | --- |
| r1-3: session range loop disabled; lap range loop disabled; range check weakened to `from < 0` | `TestDecodeErrors/session_*`, `lap_*` |
| v1.2: order check removed; lat/lon presence check removed; per-index pairing removed; `end < start` removed in `Decode` | `streams_out_of_order`, `latitude_without_longitude`, `half_a_position`, `*_end_before_start` |
| r1-2: `Encode` `end < start` removed; `start < 0` removed; session spans unchecked | `TestEncodeErrors/*` |
| Mutex removed from `precompress.Brotli` | `TestBrotliOneAtATime` |
| r1-1: `off > size` guard removed (alone) | **survived**: equivalent, see below |
| r1-1: overflowing `off + n*size > size` put back (alone) | **survived**: equivalent, see below |
| r1-1: **both** of the above | killed. `TestDecodeErrors/byte_offset_near_2^63_and_large_N` panics with `slice bounds out of range` |

Why the two survivors are equivalent:
- The two guards in `decode.go:81-89` each make the other redundant.
- `off ≤ len(b)` alone means `off + N·size` cannot overflow, since N·size < 2³⁴.
- `N > (len − off)/size` alone also rejects `off > len`, because the right-hand side is negative.
- The test catches the regression that matters (removing both), and the code follows §6 rule 6 literally.

All 32 r1 mutants are still killed.

## Findings

None.

## Previous findings

| ID | r1 summary | Status | Evidence |
| --- | --- | --- | --- |
| QA-03-r1-1 | `Decode` panics on an int64 overflow of `byte_offset + N × size` | **fixed** | See "r1-1 detail" below |
| QA-03-r1-2 | `Encode` writes a file that its own `Decode` rejects (`End < Start`) | **fixed** | See "r1-2 detail" below |
| QA-03-r1-3 | Session index-range check untested | **fixed** | 4 new session subtests. The session-loop mutant that survived in r1 is now killed |

**r1-1 detail.**
- `decode.go:81-89` checks `off < 0 \|\| off > len` first, then `N > (len − off)/size`.
- Tests: `byte_offset 4294967292`, `near 2^63 and large N`, `near 2^63 on a later column`. The crafted r1 input is a `FuzzDecode` seed.
- My r1 crafted input and the 2⁶³ cases in the differential test all return errors.

**r1-2 detail.**
- `encode.go` `checkSpan` returns `datafile: lap 1: end before start` and `start before 0` for sessions and laps. The message carries the index only, no values.
- Four `TestEncodeErrors` cases.
- `Decode` rule 7 rejects `end < start` too.
- W-1 v1.2 records "error, not clamp" with its reason.

| r1 question or gap | Status |
| --- | --- |
| Q1: committed `ride-12h.bin` | Settled in the Golden fixtures section (v1.2): kept for the F-04 prefix-sum and AC 5 timing tests, with a drop trigger (`testdata/datafile/` > 5 MB or `.git` > 100 MB). Currently 2.4 MB |
| Q2: data files outside `build/` | **Settled.** See ".gitignore check" below |
| Q3: brotli memory | Settled. A package-level mutex allows one compression per process (`precompress.go`). `TestBrotliOneAtATime` covers it, and the mutation probe confirms it. The design tells F-09 not to compress drafts without a reason |
| Gap 1: fixture top-level keys | closed |
| Gap 2: reader order and pairing rules | closed (§6 rules 5 and 7) |

**`.gitignore` check for Q2.** ADR-004 rev. 4: `*.bin`, `*.br`, `!/src/server/testdata/datafile/synthetic/*.bin`.

`git check-ignore --no-index`:

| Result | Paths |
| --- | --- |
| ignored | `x.bin`, `x.bin.br`, `src/server/x.bin`, `…/datafile/synthetic/a.bin.br`, `…/datafile/synthetic/sub/a.bin`, `…/datafile/real/a.bin`, `…/expected/synthetic/a.bin`, `src/web/public/x.br` |
| not ignored | `…/datafile/synthetic/a.bin`, `…/datafile/synthetic/a.json` |

- All 9 files in `build/corpus/real/` are ignored.
- `git add --dry-run -A` adds exactly the 18 golden `.bin` and 18 fixture `.json` files, and nothing under any `real/` folder.
- Leak search: 52 identifying values from the real data files and reports (start times and dates, stream digests, position statistics, long sample strings and numbers) were searched as fixed strings over every non-ignored file, including `qa/`. **0 hits.**

## Questions

None that block. Two notes for later features, with no action needed in F-03:
- **`.gitignore` case.** `*.bin` is case-sensitive, so a hand-typed `-datafile X.BIN` would not be ignored. The FIT pattern was made case-insensitive in ADR-004 rev. 3 because devices write `.FIT`. Here the name is always chosen by a person, so I do not count it as a finding. If the architect wants symmetry, `*.[bB][iI][nN]` settles it.
- **Mutex and cancellation.** `precompress.Brotli` takes no `context`, so a caller waiting on the mutex cannot give up. At about 120 ms for the worst case (12 h) this does not matter now. F-15 and F-30 should keep compression out of any request path that has a deadline. ADR-006 does not mention the mutex yet (it is recorded in the F-03 design v1.2). Worth one line the next time ADR-006 is revised.

## User conditions

- **F-03 AC 6 / OQ-8:** the TypeScript cross-language check completes in F-04 QA (F-04 AC 2 reads `testdata/datafile/synthetic/`). In v1.2, §6 rules 5 and 6 (order, pair presence, overflow-safe bounds) apply to the F-04 reader as well.
- **From F-02 (OQ-6):** AC 2 still needs real files from a third FIT writer. Present: `development` and `wahoo_fitness`.
- **From F-01:** a real GitHub Actions run (with the Go cache on), Node 24 and the `@v7` tags. None can be checked here: there is no remote, and Node 22 is installed locally (`npm ci` warns EBADENGINE).
- OQ-4 and OQ-5 are unchanged.

## Coverage

| AC | Status in r2 |
| --- | --- |
| 1 Header for every corpus file | traced, passing: `TestHeader`, `TestRoundTripCorpus` (18 synthetic + 3 real here), `make corpus` 21 ok |
| 2 Offsets divisible by 4 | traced, passing: `TestAlignment`, `TestRoundTripCorpus`; alignment mutants killed |
| 3 Round trip within resolution, sentinels | traced, passing: `TestRoundTripCorpus`, `TestRoundTripRandom`, `TestDelta`, `TestQuantisation`, `TestClamping`; mutants killed |
| 4 Indoor: no positions | traced, passing: `TestRoundTripCorpus` on the three files, `TestPairing`, fixture JSONs. The new rule 5 pair check also passes on every fixture |
| 5 4 h under 500 KB as `.br`; sizes recorded | traced, passing: `TestSizeBudget4h` (106,951 bytes), `MEASUREMENTS.md` (unchanged and still accurate). Real sizes are in r1 and unchanged in r2 |
| 6 Format description matches the writer | met for F-03 (OQ-8): v1.2 format section, `Decode`, golden fixtures, doc-only decoder (18/18 fixtures, 19/19 differential verdicts). TypeScript check in F-04 |

Also checked:
- W-1 v1.2: errors carry the index and no values (I-5 / W-10).
- The dependency rule is unchanged: `precompress` adds only `sync`. `git diff` shows no change to `go.mod` since r1.
- `CGO_ENABLED=0` build in `make ci`.

Not run:
- `make corpus-update`, because it rewrites golden files. `TestDataFileCorpus` passes, so they are current.
- A TypeScript decoder (F-04).
- A GitHub Actions run (no remote).
- Fuzzing beyond 60 s.
