# ADR-004 Parser test corpus and golden reports

- **Status:** accepted
- **Revision:** 11 (22 Oct 2026). First version 10 Oct 2026. The "(rev. N)" tags mark where a rule was introduced; each rule is stated in its current form
- **History:** [architect/history/ADR-004-parser-corpus-and-golden-reports.md](history/ADR-004-parser-corpus-and-golden-reports.md)
- **Features (rev. 3):** also F-04, F-32 (fixtures derived from real files stay local)
- **Features:** F-02 (introduces it), F-03 (adds data-file sizes), F-08 (adds broken files), F-30, F-32
- **Supersedes:** none
- **Requirements:** NFR Reliability; TD FIT parsing ("Keep a corpus of real FIT files ... run every parser change against it"); TD Hosting and operations (Local development: `make corpus`)

## Context

Every parser change must be checked against real and synthetic FIT files, and a change in output must be visible as a diff (F-02 AC 3). A 12-hour file has about 43,000 samples per stream; storing full stream dumps as expected output would make golden files of several MB each and diffs nobody can read. The corpus must also run in CI so a regression cannot be merged unnoticed.

## Decision

1. **Location.** `src/server/testdata/corpus/` (Go ignores `testdata` directories in builds):

   ```
   src/server/testdata/corpus/
     README.md          manifest: one row per file (see F-02 design, Corpus)
     MEASUREMENTS.md    parse time and memory for the 12-hour file (F-02 AC 12), later F-03 sizes
     real/              real device files, <brand>-<model>-<what>.fit        LOCAL ONLY, git-ignored (rev. 2)
     synthetic/         generated files, <what>.fit                          committed
     expected/real/     <name>.json golden report per real file              LOCAL ONLY, git-ignored (rev. 2)
     expected/synthetic/<name>.json golden report per synthetic file         committed
   ```

   `real/` and `expected/real/` stay in the repository as empty folders, each holding only `.gitkeep`.

2. **Golden output is a report, not a dump.** `workout.Report` turns a normalised workout into a fixed JSON structure: all metadata in full (device, start time, time zone, sport, sessions, laps, timer events, derived values, summary) and, per stream, statistics plus a digest of every value (F-02 design, Golden report). Any change to any sample changes the digest; the statistics tell a human what moved. Values are rounded to fixed decimals before formatting, so the report is byte-stable.
3. **One encoder, two drivers.** `workout.ReportJSON` is the only function producing report bytes. The `parse` subcommand prints it; the Go test `TestCorpus` compares it. Neither has its own formatting.
4. **Two entry points.**
   - `go test ./...` (so `make test` and CI) runs `TestCorpus`: every corpus file must parse without error and its report must equal the golden file byte for byte. On mismatch it fails naming the file and telling the reader to run `make corpus`.
   - `make corpus` (human-facing) builds the binary, runs `parse` over every file, writes the outputs to `build/corpus/`, and shows `diff -u` against `expected/`. `make corpus-update` rewrites the golden files (via `go test ./internal/ingest -run TestCorpus -update`); the diff of `expected/` is then reviewed in the commit like code.
   - **Names (rev. 1).** Only lower-case `*.fit` files count as corpus files. Hidden files are skipped. Any other entry in `real/` or `synthetic/` fails both drivers (`UNKNOWN` in `make corpus`, a test failure in `TestCorpus`). So a device file such as `GARMIN01.FIT` cannot sit in the corpus untested.
5. **Synthetic files are generated, deterministic and committed.** The generator `src/server/tools/corpusgen` (package `main`, dev-only, not part of the binary) writes every synthetic file from fixed parameters and fixed seeds; `make corpus-gen` re-runs it, and re-running must produce identical bytes. CI tests the committed files. **Rev. 1:** CI also checks that they are current. `TestSyntheticCorpusCurrent` (in `tools/corpusgen`, so it runs in `go test ./...`) generates into a temporary directory and requires the result to match `synthetic/` byte for byte, with no file missing and none extra. It costs a few seconds per run (QA measured it), which is cheap next to a corpus whose manifest, generator and bytes drift apart silently.
6. **Reference platform.** Golden files are produced on linux/amd64 (the CI runner). Go may fuse multiply-add on arm64, so a last-digit difference on another architecture is possible; rounding makes it unlikely. If it happens, CI's result counts. Real-file reports are produced on the machine that holds the files.
7. **Real files are local only (rev. 2, user decision 10 Oct 2026: "I don't want them in the repo").**
   - **Scope.** The decision covers real device files. It does not cover synthetic files: those hold no personal data, and CI needs them (`TestCorpus`, `TestSyntheticCorpusCurrent`). Ignoring them would leave CI with no corpus at all.
   - **Golden reports of real files are ignored too.** They are derived personal data: start time and time zone, totals, and the latitude and longitude min/max, which locate the route.
   - **`.gitignore`.** A catch-all for FIT files anywhere, with one exception for the synthetic folder, plus the real golden reports:
     ```
     # FIT files are never committed, except generated synthetic ones (ADR-004 rev. 2, rev. 3)
     *.[fF][iI][tT]
     !/src/server/testdata/corpus/synthetic/*.fit
     /src/server/testdata/corpus/real/*
     !/src/server/testdata/corpus/real/.gitkeep
     /src/server/testdata/corpus/expected/real/*
     !/src/server/testdata/corpus/expected/real/.gitkeep
     ```
   - **Rev. 3 (F-02 QA r3-1).** One bracket pattern catches every spelling of the extension (`ride.fit`, `Ride.Fit`, `GARMIN01.FIT`); it replaces the two lines `*.fit` and `*.FIT`. Only lower-case `synthetic/*.fit` is re-included, on purpose: synthetic files must be lower case anyway (item 4), so a wrongly named file there stays ignored and fails `UNKNOWN` locally.
   - **The drivers use one rule for both kinds of file.** Every lower-case `*.fit` file needs a golden report, and every golden report needs its `*.fit` file. A fresh checkout and CI have neither real files nor real reports, so the real side is simply empty and passes. On a machine that has real files, a missing report is `NEW` and fails until `make corpus-update` is run there. Both drivers also print how many real files they checked (`real: N files (local only)`), so it is visible whether a run covered any.
   - **Everything derived from a real file stays local too (rev. 3).** This covers every later feature:
     - F-03: `data.bin` / `data.bin.br` written from real files, and their measured sizes. Sizes for real files are not recorded in committed files such as `MEASUREMENTS.md`; a size table there lists synthetic files only. Real-file sizes go into the QA report, by manufacturer only.
     - F-04: round-trip and decoder fixtures.
     - F-08: truncated, bad-CRC and chained files.
     - F-32: the end-to-end sweep's inputs and outputs.
     - Any later fixture, snapshot or golden file.

     **Rev. 4 (F-03 QA r1 Q2, 11 Oct 2026).** `parse -datafile` writes data files wherever it is pointed, so `.gitignore` also contains `*.bin` and `*.br`, with one re-include, `!/src/server/testdata/datafile/synthetic/*.bin` (the F-03 golden data files, which come from synthetic files only).

     **Rev. 5 (F-03 QA r2 note, 11 Oct 2026; applied in F-04).** For the same reason as rev. 3, the two patterns become `*.[bB][iI][nN]` and `*.[bB][rR]`, so `X.BIN` or `data.Bin.BR` written by hand is ignored too. The re-include stays lower case only: golden files are generated with lower-case names, and a wrongly named file there stays ignored. Exact block in the F-03 design, section Golden fixtures.

     **Committed fixtures are built from synthetic files only.** Outputs from real files go under `build/` (already ignored) or the ignored `real/` folders, never anywhere else in the tree.
   - **Coverage.** CI covers synthetic files only. Real files are covered on the machines that hold them: the operator's machine, and QA when it runs there. The symmetric-bug risk in ADR-003 is therefore mitigated only by local runs.

8. **Outcomes, not only reports (rev. 6, F-08).**
   - A corpus file's golden file in `expected/<set>/<name>.json` is its **outcome**. For an accepted file that is the report (`workout.ReportJSON`, now version 2 with `integrity`). For a rejected file it is the **rejection report** (`ingest.RejectionJSON`, ADR-018 item 6: `report_version`, then `rejected` with `kind`, `message`, `limit`, `file_type`). These two functions are the only producers of golden bytes (extends item 3).
   - Item 4's rule "every corpus file must parse without error" becomes "every corpus file must produce its golden outcome". A file that was accepted and becomes rejected (or the reverse) shows up as a diff.
   - `parse` exits 3 for a rejection and prints the rejection report; `make corpus` compares it like a report (F-08 design, section CLI and corpus driver). Rejected files have no data file (F-03 golden data files exist for accepted synthetic files only).
   - Broken files (truncated, bad CRC, chained, non-activity, garbage) are generated by `corpusgen`, the broken ones by byte operations on generated files. So they are synthetic, committed and covered by item 5. Hostile files for the parse limits (ADR-017) are built in memory by tests and never committed: they can be megabytes, and their outcome depends on the limits the test passes.
   - **Fuzz inputs.** Failing inputs found by `FuzzParse` are committed under `src/server/internal/ingest/testdata/fuzz/FuzzParse/` (Go's format). `FuzzParse` takes its seeds from `synthetic/` only, never from `real/`, so every input it writes is a mutation of synthetic bytes and may be committed. Go writes a failing input next to the test, inside the repository; a seed from `real/` would put derived real data there (item 7).

9. **The strip over the corpus (rev. 7, F-09, ADR-023).**
   - New synthetic file `identifying-data.fit` (F-09 design, section Corpus) with the usual golden report and data file.
   - `TestCorpus` also runs `Accept` on every accepted file, synthetic in CI and real locally. `Accept` must succeed, and its stored bytes, read with the library's full decoder, must hold no valid value in any field of the blank table and no data message on the remove list (ADR-023 items 4 and 5; both read from the strip's tables, rev. 8 below). No figure is asserted, so it runs unchanged on real files.
   - No stripped `.fit` is committed as a golden. Instead, a property test pins the byte differences (F-09 design, `TestStripCorpus`).
   - `parse -original <path>` writes the stripped file wherever it is pointed. Output from a real file is real-derived and stays local (item 7; `*.fit` is already ignored outside `synthetic/`).
   - `FuzzAccept` takes its seeds from `synthetic/` only, like `FuzzParse`.
   - (rev. 8) Failing inputs found by `FuzzAccept` are committed under `src/server/internal/ingest/testdata/fuzz/FuzzAccept/` (Go's format), like `FuzzParse`'s, and `go test` runs them as seeds in CI. The rule of item 8 applies: they may be committed only because the fuzzer was seeded from `synthetic/` alone. The first three are the QA-09-r2-1 inputs.
   - (rev. 8) The decoder check of `TestCorpus` reads the blank and remove lists from the strip's tables (`blankTable`, `removeTable`), not from literal numbers. It also checks that `Accept` on the stored bytes returns the same bytes (F-30 idempotence), for local real files too. No figure is asserted.
   - (rev. 8) New synthetic file `ride-30min-bad-basetype.fit` (F-09 design v1.5, section Corpus). It pins where the decoder stops on a rejected definition (F-08 R-a), which is also where the stored original ends (ADR-023 rev. 4 item 6a), and it is the upgrade signal of ADR-003 rev. 5.
   - (rev. 9) The two QA-09-r3-1 inputs (`crasher-d0ededf3c3e842f3`, `overrun-k5`) are committed under `testdata/fuzz/FuzzAccept/` too, byte for byte with their names. QA checked that both derive from synthetic files only.
   - (rev. 9, QA-09-r3-2) **The fuzz run.** A manual `FuzzAccept` run, and any acceptance run named in a design or QA report, uses `go test ./internal/ingest -run '^$' -fuzz '^FuzzAccept$' -fuzztime <T> -fuzzminimizetime 5s`. It runs in a scratch copy of `src/server` without `testdata/corpus/real/`, so nothing derived from a real file is ever in reach, and no new input is written into `src/` unreviewed. Go's default minimization limit of 60 s per input spends most of a 5-minute run on the large synthetic seeds. The same flag applies to `FuzzParse`.
   - (rev. 9) New synthetic file `ride-30min-size-short.fit` (F-09 design v1.6, section Corpus). Its header data size ends inside the last record. It pins that the decoder reads that record in full (ADR-003 rev. 6), and that the stored original is `ride-30min.fit` again (ADR-023 rev. 5 item 6).
   - (rev. 10, reason rev. 11) **Strict over the corpus.** Since ADR-023 rev. 8 item 8a, `Accept` may refuse input as `unreadable` even when `Parse` accepts it. (rev. 11) The corpus stays strict as a **regression check**: the built code accepts every corpus file, so a refusal of one is a change in our behaviour that must be explained. It is not a promise that files from real devices are accepted; the user withdrew that requirement on 22 Oct 2026 (ADR-023 rev. 9 item 8a). So `TestCorpus` and `TestStripCorpus` fail on any refusal of a file that `Parse` accepts, `unreadable` included, synthetic in CI and real locally. The failure message gives the kind and, for a real file, no figure. Fuzz seeds and fuzz inputs under `testdata/fuzz/` are crafted input and follow F-09's `FuzzAccept` rule instead.

## Consequences

- The corpus grows with each parser feature (F-08 adds truncated, bad-CRC, chained, non-activity and garbage files) without changing the mechanism.
- Golden files are small (a few KB) and reviewable; a changed digest without changed statistics points to a small, local change.
- A report does not show *which* sample changed; to locate it, an engineer prints full streams locally (F-03's data-file output or a debugger). Accepted.
- (rev. 2) Real files and their reports never leave the operator's machine. That settles OQ-3's privacy and consent part. A regression in real-file output is caught only when someone runs `make corpus` on a machine that has the files. The operator should run it before accepting any parser change (F-03, F-08, library upgrades).
- Files that F-08 adds to the corpus (truncated, bad CRC, chained) must be derived from synthetic files if they are to be committed; truncating a real file gives a real file, which stays local.

## Would choose differently if

- The corpus grew to hundreds of MB (it should stay well under 20 MB): then move real files to a separate fetched archive.
- Reports routinely failed to explain changes: add a per-stream sample at fixed indices.
- Real files with a licence that allows publishing became available, for example files the owner explicitly releases: they could go into a committed `public-real/` folder. That needs a new user decision, because it reverses item 7.
