# ADR-017 Parse isolation and limits: in-process `ingest.Parser` with a decode deadline, counted budgets and parse slots

- **Status:** accepted
- **Revision:** 8 (4 Nov 2026; rev. 8: QA-30-r1-6, a deploy's `regenerate` is not added to the cap, see rev. 4 item 3). First version 15 Oct 2026. Labels "rev. N" and "rev. N item M" below are the stable IDs that code and other documents cite; each marks a decision made in that revision, stated here in its current form.
- **History:** [architect/history/ADR-017-parse-isolation-and-limits.md](history/ADR-017-parse-isolation-and-limits.md) (revision list, findings, the revision sections as written)
- **Features:** F-08 (introduces it), F-09 (calls it per upload), F-30 (regenerate), F-31 (machine sizing), F-32
- **Supersedes:** the F-02 entry point `ingest.Parse(ctx, io.Reader)` (F-02 design, Public surface; TD Code structure, Bounded contexts and modules)
- **Requirements:** NFR Security ("parsing runs with a time limit and a memory limit. A malformed or hostile file can fail only its own upload, never the service"), TD Security and privacy (Uploads: "parse with a timeout and a memory cap"), NFR Reliability, UP-2, UP-3

## Context

Every upload and every regeneration runs the FIT decoder (`muktihari/fit` v0.28.4, ADR-003) on bytes from the internet. A crafted file must not stop the process, exhaust its memory, or hold the parser so long that other uploads cannot run. Go has no per-goroutine memory cap: a parse that allocates without bound kills the whole process (`fatal error: out of memory` cannot be recovered).

What a hostile file can do, checked in the v0.28.4 source (`decoder/decoder.go`, `decoder/readbuffer.go`) with our options (`WithBroadcastOnly`, `WithNoComponentExpansion`):

| Path | Bounded by the library? | Growth |
| --- | --- | --- |
| Main loop | Yes. Every message reads at least one byte; the loop ends at `DataSize` or EOF. It cannot loop forever | Linear in bytes |
| Reads | Yes. At most 765 bytes per read, fixed buffer | — |
| Retained messages | None with `WithBroadcastOnly` | — |
| (rev. 5) Definition copies for the definition listener (our record counter, ADR-003 rev. 5; `decoder.go:613-621`, `sliceutil.Clone`) | Yes. One copy per definition, at most 765 bytes, nothing retained: the counter keeps only an int | Allocation linear in bytes (a definition is at least 6 bytes); CPU linear |
| Component expansion (recursive) | Off (ADR-003 rev. 1) | — |
| `fieldDescriptions` | **No.** One retained object per `field_description` message, cleared per sequence | Memory linear in those messages |
| Developer-field lookup | **No.** Each developer field scans every field description | CPU: dev fields × field descriptions (super-linear) |
| Our listener (`Recording`) | **No.** One value per `record`, `lap`, `session`, timer `event` (and `sport` message, rev. 1) | A 1-byte message (compressed-timestamp header, no fields) adds 96 to 200 bytes: 20 MB of them is 2 to 4 GB |
| `workout.Normalise` | Linear in samples; gap filling inserts up to 3 samples per record | Memory up to 4 × records |

Every unbounded path either passes through our listener, which sees every message as it is decoded, or is CPU in the decoder's message loop, which checks the context once per message.

F-02 measured the 12-hour file (`synthetic/ride-12h.fit`, 43,200 records, 1.3 MB) on the development machine (Intel i7-4770T, go1.26.8): about 71 ms, 27.6 MB allocated, 23 to 28 MB max RSS for the whole `parse` command (`testdata/corpus/MEASUREMENTS.md`).

## Decision

1. **The parse runs in-process, in the caller's goroutine, through one `ingest.Parser` per process.**
   ```go
   func NewParser(lim Limits, log *slog.Logger) *Parser // log nil: no log lines (CLI)
   func (p *Parser) Parse(ctx context.Context, data []byte) (workout.Workout, error)
   ```
   `cmd/workout-explorer` creates the parser once and passes it to whatever needs it (F-09, F-30). The input is the whole file as bytes. The upload is at most 20 MB and F-09 keeps it anyway to store `original.fit`. Bytes allow a second pass over a sequence (CRC retry, F-08 design), let the parser check the size before any work, and make `Parse` a pure function of its input for fuzzing. Uploads go through `(*Parser).Accept` (rev. 2). `(*Parser).Limits() Limits` returns the limits in effect after defaults, as a read-only copy (rev. 7 item 2).

2. **Time limit: a decode deadline.** `Parse` derives `context.WithTimeoutCause(ctx, lim.Timeout, errTimeLimit)` after it has a parse slot (item 5) and passes it to every decoder call. The decoder checks it before each message, and `ingest` checks it before each FIT sequence. (rev. 1) `workout.SampleCount` and `workout.Normalise` stay outside the deadline and take no context (`workout` stays pure, TD Code structure, Dependency rules). They run only after the budgets (item 3) have passed, and they must run in O((n + k) log(n + k)), with n = records or samples and k = sessions + laps + timer events + sport hints. That rules out any loop over sessions, laps, timer events or hints per sample. Since every input is bounded by a budget, the bound gives a bounded time (measured at the caps, item 7). So `Parse` returns within `Timeout` plus the normalisation time at the caps.

   **Why not put normalisation under the deadline.** It would need a context in `workout` and checks in its hot loops, for a cost that the budgets and the complexity bound already cap. The deadline exists for the decoder, whose CPU per byte is the part a hostile file controls (developer-field lookups). Normalisation's cost depends only on counts the budgets fix.

3. **Memory limit: counted budgets, enforced before the memory is used.** The listener counts every message by kind as it arrives. When a count goes over its budget, it cancels the decode with a cause (`context.WithCancelCause`) and ignores everything after. `Parse` then returns a `limit` rejection (ADR-018). Before `Normalise`, `ingest` calls `workout.SampleCount(rec)` (exact sample count after gap filling, without building the samples) and rejects above `MaxSamples`. The input size is checked first of all.

4. **Defaults** (`ingest.DefaultLimits`), from the F-02 measurements with headroom:

   | Limit | Default | Basis |
   | --- | --- | --- |
   | `MaxBytes` | 20 MiB (20,971,520) | At least F-09's 20 MB upload cap. 16 × the 12-hour file |
   | `Timeout` | 10 s | 12-hour file: about 71 ms (140 ×). At the record cap: about 0.9 s estimated (11 ×), and still at least 4 × on a VPS core two to three times slower |
   | `MaxRecords` | 500,000 `record` messages | 11.6 × the 12-hour file. 138 h at 1 Hz, enough for a 1,200 km brevet recorded every second (OQ-31) |
   | `MaxSamples` | 500,000 after gap filling | The same 138 h for smart-recording files, whose 2 to 4 s gaps are filled |
   | `MaxLaps` | 10,000 | 12-hour file: 24. Auto-lap every km on a 1,000 km ride: 1,000 |
   | `MaxSessions` | 100 | Triathlon: 5 |
   | `MaxTimerEvents` | 100,000 | Auto-pause in town, about one stop and start per minute: about 1,500 in 12 h |
   | `MaxFieldDescriptions` | 256 | Real files: a few to a few dozen (Connect IQ fields, power meters) |
   | `MaxSequences` | 64 | Chained files: 2 to 5 sequences |
   | `MaxSportMessages` (rev. 1) | 1,000 `sport` messages, limit name `sport_messages` (ADR-018 rev. 1) | Real files: one per activity; a multisport file plausibly one per leg; 1,000 is 200 × a triathlon. Counted on arrival like the others |
   | `MaxConcurrent` | 2 parse slots per `Parser` | Memory: 2 × the per-parse peak (item 7) |

   Every limit is a field of `Limits`, so tests can pass small values. Production uses `DefaultLimits` unchanged, except `Timeout`, which production takes from the server setting `WORKOUT_EXPLORER_PARSE_TIMEOUT` (rev. 7); `DefaultLimits.Timeout` stays 10 s and is that setting's default. A different value of any other limit needs a revision of this ADR.

5. **Parse slots.** At most `MaxConcurrent` parses run at once per `Parser`. A buffered channel serves as the semaphore. Waiting for a slot respects the caller's `ctx`; if `ctx` ends first, the result is `canceled` (ADR-018). Waiting time does not count towards `Timeout`. With two slots, one hostile file cannot hold up a friend's upload, and the worst-case memory stays at two parses.

6. **Panics are contained.** `Parse` recovers a panic from the decoder, the listener or `Normalise`. The panic becomes an `internal` rejection, and the log line holds the panic value and the stack as function names with `file:line` only (`runtime.Callers`, no argument words, ADR-018). No other goroutine is started, so there is nothing that can panic outside the `recover`.

7. **Measured, with a stop rule.** (rev. 1) `BenchmarkParseAtCap` and the RSS run use one in-memory file with every budget at its cap: 500,000 records at 1 Hz with every stream, 100 sessions alternating `cycling` and `running`, 10,000 laps with a sport, 100,000 timer events, 1,000 `sport` messages, 256 field descriptions, within `MaxBytes` (written to `build/` by `corpusgen -cap-file`, never committed). Decode and normalise are reported separately. The engineer records ns/op, B/op and the max RSS of `parse` on the same file in `MEASUREMENTS.md`. **Budget: max RSS at the caps ≤ 400 MB, and decode + normalise ≤ 2 s on the development machine.** `Accept` has its own rule (rev. 2 item 4). Every stop rule is measured on the development machine with `DefaultLimits`, never with the time-limit setting (rev. 7 item 3), and with neither `GOGC` nor `GOMEMLIMIT` set (rev. 3 items 3 and 5). If a rule is exceeded, the engineer stops and reports. The caps are then lowered in a revision of this ADR; the code does not change.

### `Accept` (rev. 2)

Uploads go through `(*Parser).Accept`: parse, strip identifying data (UP-9), and reparse the stripped bytes to prove the workout is unchanged (ADR-023 item 1).

1. **One slot for the whole `Accept`.** The slot is taken once, before the first parse, and released after the reparse. Slot waiting still respects the caller's `ctx`.
2. **Deadline per decode.** Each of the two decodes gets its own `Timeout` (the configured T, rev. 7); the strip in between is linear in the input bytes and checks `ctx` between sequences. `Accept` returns within 2 × (T + normalisation at the caps) plus the strip. Slot waiting is not counted.
3. **Memory.** Before the reparse, the first workout is reduced to a 32-byte reference (ADR-023 item 1.2) and dropped, which means unreachable (rev. 3 item 2). The peak is one parse plus the upload bytes and the stripped copy. The upload is at most `MaxBytes`; the stripped copy at most `MaxBytes` plus ADR-023 item 7's growth (rev. 6 item 3).
4. **Measured.** `BenchmarkAcceptAtCap` uses the all-caps shape (item 7). The RSS run is `parse -original` on the all-caps file. Stop rule: max RSS ≤ 400 MB, each decode + normalise ≤ 2 s, the whole `Accept` ≤ 4.5 s on the development machine. If any is exceeded, the engineer stops and reports.

### The first parse is unreachable; server memory model (rev. 3, rev. 4)

1. **No GC setting in code:** no `debug.SetGCPercent` and no `debug.SetMemoryLimit`. A lower `GOGC` would hide retention, cost CPU and apply to the whole process. Memory problems are fixed by removing retention, not by lowering caps the user accepted (OQ-31) or raising the RSS rule.
2. **"Dropped" means unreachable.** Once the reference digest exists, nothing reachable from `Accept`'s frames or from its `outcome` may point into the first workout or into the first listener's `Recording`:
   - `run` computes the reference itself and clears its own `w` before the strip. `strip` receives the 32-byte reference, never a workout.
   - `outcome` holds no `*listener`. When `parse` returns, on every path, it copies the values that the log line and `errorOf` need into the outcome: file type, device, Garmin product name and the count of record messages. The listener and its `Recording` then die with `parse`'s frame. The reparse uses its own `outcome`, which is dropped after the check.
3. **Proof.**
   - `TestAcceptReleasesFirstParse` (CI, deterministic): the heap at the reparse is at most the stripped copy plus 1 MiB above the baseline (F-09 design, section Tests).
   - The RSS rule of rev. 2 item 4: on each of 3 runs, with neither `GOGC` nor `GOMEMLIMIT` set.
4. **Server memory model.** This replaces the memory sentence in ADR-021's Consequences. A request's memory depends on its stage:

   | Stage of one upload request | Holds | Bound at the caps |
   | --- | --- | --- |
   | Reading the body, or waiting for a parse slot | The upload U | ≤ 20 MB |
   | Inside `Accept` (at most `MaxConcurrent` = 2 at once) | U, the stripped copy, one parse's working set | **A** = max RSS of `parse -original` on the all-caps twin, ≤ 400 MB |
   | After `Accept`, lifecycle steps 2 and 3 (encode) | U, the stripped copy, the reparsed workout, the data file and the encoder's working set | **B**, measured with the server run (F-09 design, section Tests) |
   | Lifecycle steps 4 to 6 (files, row) | U, the stripped copy, the data file | < B |

   (rev. 4) Process memory that clients can grow is added on top: the upload limiter's key table, whose per-key log is a fixed array of whole seconds (ADR-021 rev. 3 item 8), bounded at **L ≤ 40 MB** at its 100,000-key cap and proven by `TestLimiterMemory`.

   Worst case of `serve`: **M(N) ≤ 2·A + (N − 2)·B + L for N ≥ 2**, and A + L for N = 1. The default N = 4 gives 2·A + 2·B + L. The server run measures the first part and never fills the rate table, so F-31 adds L to the measured M(4). A synthetic estimate, replaced by the measured row:
   - A is about 270 MB (230 MB for a plain `parse` plus the upload and the stripped copy).
   - B is about 150 to 250 MB. The workout is close to `Normalise`'s 98 MB, the encoder is the 12-hour file's 8.8 MB per run scaled to 500,000 samples, and the upload and stripped copy add about 35 MB.
   - So M(4) is about 0.85 to 1.05 GB, plus L.

   N = 16 allows up to 2·A + 14·B + L in the worst case. In practice, requests waiting for a slot hold only U. Every later in-memory table that clients can grow (for example F-27's report limiter) states its own bound and adds it to M(N) the same way.
5. **Runtime variables are F-31's.** `GOMEMLIMIT` is read by the Go runtime, not by our code. If F-31 sets it, the systemd unit sets it, next to `MemoryMax` and below it, sized from the measured M(N) (rev. 8: not plus `regenerate`). Every stop rule in this ADR is measured without it.

(rev. 4 item 3) **Machine (OQ-51, decided by the user, 16 Oct 2026):** the VPS has at least 2 GB RAM, and N stays at its default of 4. F-31 sets systemd `MemoryMax` at or above the measured M(4) + L. (rev. 8) During a deploy, `regenerate` (F-30) runs next to `serve` in the same cgroup, one workout at a time (about 365 MB max RSS on the all-caps file, QA-30-r1-6). It is **not** added to the cap: going over needs a deploy to coincide with 4 uploads at the caps, and the result is an OOM kill inside the cgroup that is recovered (ADR-022, F-30 RG-7, RG-11 retry). The gate of 2 (Would choose differently, rev. 3) is no remedy for this: at N = 4 it does not lower M(4). `GOMEMLIMIT`, if used, goes below `MemoryMax`.

### Definition listener (rev. 5)

The record counter (ADR-003 rev. 5) adds the definition-copy row of the table in Context; no limit changes. A file made only of definitions is not counted by any budget, because definitions are not messages. Its cost is bounded by `MaxBytes` and checked by the decode deadline.

**Measured.** `BenchmarkAcceptDefinitions` (F-09 design, section Tests) runs `Accept` on two in-memory 20,000,000-byte uploads: one made of minimal definitions (1 field each) and one of definitions with 255 fields each, both behind one `file_id` and one `record`. Stop rule: at most 4.5 s per `Accept` on the development machine (rev. 2 item 4).

### Limits on the reparse (rev. 6)

1. `MaxBytes` is checked on the input of `Parse` and of `Accept`'s first parse. The reparse in `Accept` does not check it. Its input is our own output, bounded by ADR-023 item 7 (2 bytes per sequence whose file CRC was missing, plus under 8.4 MB only for a crafted overlap). Every other budget and the decode deadline apply to the reparse unchanged, and none of them can be exceeded by the strip: removing messages only lowers the counts.
2. A reparse that hits the decode deadline is reported as the upload's `limit`/`time` (ADR-018 rev. 4), not as `internal`. A deadline depends on load, so a second decode can exceed it when the first did not.
3. Memory (rev. 2 item 3): the stripped copy is at most `MaxBytes` plus ADR-023 item 7's growth. A and the stop rule are unchanged; the all-caps file has no overlap.
4. F-30 reads stored originals, which are our own output too. Its design gives `Accept` on a stored original the same allowance: `MaxBytes` plus ADR-023 item 7's growth.

### The time limit is a server setting (rev. 7)

The user decided on 22 Oct 2026 that the decode deadline (`Timeout`, item 4) is configurable, because the server may be an old, slow machine.

1. **The setting.** It follows ADR-021 item 4 rule for rule.

   | Property | Value |
   | --- | --- |
   | Name | `WORKOUT_EXPLORER_PARSE_TIMEOUT` |
   | Format | Whole seconds: ASCII digits without sign, unit, spaces or leading zero (`^[1-9][0-9]*$`, at most 2 digits). `10s`, `1m` and `10.0` are invalid |
   | Unset or empty | `ingest.DefaultLimits.Timeout` (10 s). That is the only source of the default; `cmd` does not repeat the number |
   | Range | 5 to 60 s inclusive |
   | Invalid | The subcommand prints `<subcommand>: WORKOUT_EXPLORER_PARSE_TIMEOUT must be a whole number of seconds from 5 to 60` to stderr and exits 2, before it opens the database, the data directory or the input file |
   | Logged | `serve`'s `server started` line gains `parse_timeout_s` (integer) |

   Why whole seconds and not a Go duration (`time.ParseDuration`): one strict format for every setting (TD Conventions, Configuration). Sub-second values have no use here, and `ParseDuration` accepts forms like `1m-50s`, `+30s` and `10000ms`. The name keeps the user's suggestion. A unit mistake (`30s`) cannot pass silently, because it fails with a message that names the unit.

2. **Where it is read and passed in.**
   - `cmd/workout-explorer/settings.go` reads it through the injected `getenv`, once at start, next to the in-flight cap. No other package reads it. `ingest` never reads the environment.
   - The value reaches the parser as `ingest.Limits{Timeout: t}`. Every other field stays zero, which means its default. One helper in `cmd` builds the parser from it, and every subcommand that constructs a `Parser` uses that helper:
     - `serve` (F-09).
     - `parse`, the diagnostic. The operator needs it to measure the production machine (item 6) and to reproduce a time limit seen in the log.
     - `regenerate` (F-30), item 5.
   - `ingest` has `(*Parser).Limits() Limits`, the limits in effect after defaults. It is a read-only copy, so a test can prove the wiring.
   - `ingest` itself checks nothing about the range. Tests still pass any positive `Timeout`, and zero still means the default.

3. **Only `Timeout`.** Every budget, `MaxBytes` and `MaxConcurrent` stay fixed. A different value of any of them still needs a revision of this ADR. The stop rules (item 7, rev. 2 item 4, rev. 5) are measured on the development machine with `DefaultLimits` and never with the setting.

4. **Why 5 to 60.**
   - **Upper bound 60 s.** One client address may start 60 uploads per hour (ADR-021 item 7). A crafted file that runs each of its two decodes almost to the deadline holds a parse slot for up to 2·T. So one address can occupy 60 × 2·T = 120·T slot-seconds per hour, out of 2 slots × 3,600 s = 7,200. Its share is **T / 60 s**: a sixth at the default and all of it at 60 s. Above 60 s, one address within its rate limit could keep both slots busy all the time, and friends' uploads would wait behind it or be refused as `busy`.
   - **Lower bound 5 s**, half the default. The setting exists to give a slow machine more time. A lower value has no use case and could refuse files at the caps under load even on the development machine.

5. **Consequences of a configured T.**

   | Concern | Effect |
   | --- | --- |
   | Peak memory | **Unchanged.** A, B, L and M(N) (rev. 3, rev. 4) are bounded by the budgets and by the in-flight cap N, not by time. A longer T makes a request hold the same memory for longer. Only requests whose file runs into the deadline are affected, and N still bounds how many exist at once. `MemoryMax` sizing (rev. 4 item 3) does not change |
   | `Accept` duration | At most 2 × (T + normalisation at the caps) + the strip (rev. 2 item 2). Slot waiting is still not counted |
   | Concurrency | Still 2 parse slots and N in-flight uploads. A file that hits the deadline holds its slot for up to 2·T instead of 20 s. Item 5's promise holds unchanged: one hostile file holds one slot and cannot hold up a friend's upload in the other. The share one address can take is item 4 above |
   | HTTP write deadline | Read deadline + 6·T: 180 s at the default; 480 s at 60 s (ADR-021 rev. 4). Without this, a friend's upload queued behind two crafted ones at T = 60 s could finish after a fixed 180 s. Its draft would be stored, but the response lost |
   | `busy` (503) and `Retry-After: 30` | **Unchanged.** `busy` means N uploads are in flight. Normal uploads finish in seconds, so 30 s stays a good hint. Only crafted files hold a slot near 2·T, and F-11 shows the message and does not retry on its own. The text is UP-10's and fixed |
   | Rate limit (429) | Unchanged; its `Retry-After` comes from the window |
   | `regenerate` (F-30) | **The same setting, read the same way.** It runs on the same machine over the same files. With a shorter limit than `serve`, a workout that was accepted at upload would hit `limit`/`time` on every run, and be kept and retried each time (OQ-53 follow-up 1). A time limit never deletes a workout, so changing T between runs cannot delete anything. F-30 adds no per-workout deadline of its own |
   | Message and log | **Unchanged.** A time limit stays kind `limit`, limit `time`, with "This file is too large or too complex to read." (ADR-018). The log line is still `fit rejected` at WARN with `limit` `time`, and its existing `duration_ms` shows how long the decode ran. No key is added to ADR-018's allow-list. The configured value appears once, in `server started`. `TestParseLogs` and the rejection goldens do not change |

6. **Choosing T on the production machine (F-31).**
   - Copy the synthetic all-caps twin (`corpusgen -cap-file`, built on the development machine, no real data) to the server.
   - Run `WORKOUT_EXPLORER_PARSE_TIMEOUT=60 workout-explorer parse <twin>` three times with the service idle. Take the median wall time W.
   - If 4·W ≤ 10 s, keep the default and leave the variable unset. Otherwise set T to 4·W rounded up to a whole second.
   - If 4·W > 60 s, the machine is too slow for the caps. Stop and report. The caps are lowered in a revision of this ADR, or the machine is replaced. T never goes above 60 s.
   - The factor 4 is item 4's basis ("at least 4 ×"): two parses and the encoders share the cores under load.
   - The trigger "decode + normalise above half of T on the production VPS" (Would choose differently if) reads against the configured T. The procedure keeps W ≤ T/4, so the trigger fires only if the procedure was skipped.

## Why not a subprocess

A child process (`workout-explorer parse-worker` with `RLIMIT_DATA` and a kill timer) gives hard limits even for library bugs we cannot see. It costs:

- A protocol for the normalised workout between processes. JSON cannot carry `NaN` (missing samples), so the protocol would need its own encoding: a second serialisation of the core model to keep in sync.
- A second entry point in the binary, process start per upload, and behaviour that depends on Linux rlimits interacting with the Go runtime's address-space use (and on `os.Executable()` while a deploy replaces the binary).
- Tests that need the built binary.

It buys protection against exactly one class of failure: unbounded growth that our listener cannot see, or an unrecoverable runtime fatal error. The table above shows that in v0.28.4, with our options, no such growth path exists: the retained state is the field descriptions, which the listener counts. There is no recursion and no goroutine. `FuzzParse` (F-08 design) and the corpus guard this; ADR-003 already pins the version.

## Consequences

- One parser per process. `serve` and `regenerate` (F-30) are separate processes, so during a deploy up to three parses can run on the machine (regenerate uses one slot at a time). The cap covers `serve` only; the deploy-time overlap is accepted (rev. 4 item 3, rev. 8). Both read the same `WORKOUT_EXPLORER_PARSE_TIMEOUT`, so F-31 gives them one environment (rev. 7 item 5).
- `ingest.Parse(ctx, io.Reader)` is gone. Callers (CLI `parse`, corpus tests, data-file tests, later F-09 and F-30) read the file into memory first, the CLI through `io.LimitReader(f, MaxBytes+1)`.
- A file over a budget is rejected, never cut to fit: over 138 h at 1 Hz, the sharer gets the `limit` message (OQ-31).
- Library upgrades must re-check the table in Context (retained state, recursion, goroutines, super-linear paths). The check is part of the upgrade hand-off (ADR-003 item 1).
- (rev. 7) One more environment variable, and an HTTP write deadline that depends on it. A slow machine's operator can raise T to 60 s, at which point one client address can saturate both parse slots with crafted files (rev. 7 item 4). The bound makes that the worst case, not a mistake past it.
- Reversible. The `Parser` API is the boundary: moving the work into a subprocess later changes `ingest` only.

## Would choose differently if

- Fuzzing or production shows an unrecoverable crash (stack overflow, runtime fatal) or memory growth the listener cannot count: move the decode into a subprocess behind the same `Parser` API.
- A library upgrade adds retained state, recursion with expansion off, or goroutines: same, unless the listener can count the new path.
- The measured peak at the caps exceeds the budget in item 7 and lowering the caps would reject real workouts the user wants (OQ-31): then reduce peak memory in `workout` (for example, sort record indices instead of copying records) before considering a subprocess.
- (rev. 1) The all-caps measurement exceeds the stop rule with the complexity bound met: lower `MaxSessions` or `MaxLaps` in a revision first. F-31 measures decode + normalise above half of T on the production VPS: give `Normalise` a cancellation check per stage, not per sample.
- (rev. 3) `parse -original` is above 400 MB with `TestAcceptReleasesFirstParse` green: profile the heap at the reparse first, next reduce the peak inside `workout` (point above), and only then lower caps, in a revision.
- (rev. 3) The measured M(4) is above 1 GB and F-31's machine cannot hold it, or the operator needs N above 4 on a small machine: put lifecycle steps 2 and 3 behind a gate of 2, so only two reparsed workouts are being encoded at once. That gives M(N) ≤ 2·A + 2·B + (N − 4)·(U + stripped copy + data file) + L.
- (rev. 8) `docker inspect` shows an OOM kill of `app` during a deploy, or friends really upload files near the caps: `regenerate` takes one of `serve`'s parse slots across processes (a `flock` per slot in the data directory, as ADR-031), so at most two parses run at once in the cgroup. Not raising the cap: on 2 GB it would leave under 600 MB for the host.
- (rev. 4) F-27's report limiter, or any later in-memory table that clients can grow, has no bound of its own: each such table states its bound and adds it to M(N).
- (rev. 5) The definition flood exceeds 4.5 s per `Accept`: add a definition budget to `Limits` (a revision of item 4), counted in the counter's `OnMesgDef`, which cancels the decode like the other budgets.
- (rev. 6) A later change to the strip can grow its output by more than ADR-023 item 7 says: the reparse gets an explicit byte bound again, `MaxBytes` plus the new growth.
- (rev. 7) `fit rejected` lines with `limit` `time` arrive at close to 60 per hour while friends get `busy`: lower T, or move rate limiting into Caddy (ADR-021, "Would choose differently if").
- (rev. 7) The operator needs more than 60 s for legitimate files: lower the caps in a revision. A higher bound would let one client address deny parsing to everyone.
- (rev. 7) A second limit has to vary per machine (for example `MaxConcurrent` on a single-core machine): add it as its own setting, by the same rules, in a revision. Do not widen this one into a "limits profile".
