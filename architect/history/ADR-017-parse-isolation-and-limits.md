# ADR-017 Parse isolation and limits: history

History of [architect/ADR-017-parse-isolation-and-limits.md](../ADR-017-parse-isolation-and-limits.md). The live ADR states every decision once, in its current form, and keeps the revision labels (rev. 1 to rev. 7, with their item numbers) that code and other documents cite. This file holds, verbatim, the revision list of the header and the Revision 1 to 7 sections as they stood on 22 Oct 2026, including their findings and the parts later revisions replaced. Read it only when you need the history.

## Header revision list (as of revision 7)

- **Date:** 15 Oct 2026 (revision 1, 15 Oct 2026, F-08 QA round 1: budget `MaxSportMessages`, a complexity bound for normalisation, and the stop rule measured with every budget at its cap; see Revision 1)
- **Revision 2:** 16 Oct 2026, F-09: `Accept` (parse, strip, reparse) holds one slot; each decode has its own deadline; the first workout is reduced to a digest before the reparse; the stop rule is measured for `Accept` too. See Revision 2
- **Revision 3:** 16 Oct 2026, F-09 round 1 stop (RSS over 400 MB): no limit changes; "dropped" made precise (nothing reachable points into the first parse) and proven by a test; no GC setting in code; the server memory model for N uploads in flight. See Revision 3
- **Revision 4:** 16 Oct 2026, F-09 v1.4 (QA-09-r1-4, OQ-51 decided): the server memory model gains the rate table's bound L; machine and `MemoryMax` per the user's OQ-51 decision. See Revision 4
- **Revision 5:** 21 Oct 2026, F-09 v1.5 (QA-09-r2-1): the decoder gets a definition listener (a record counter, ADR-003 rev. 5). It adds one short-lived copy per definition and no retained state. No limit changes; measured on a definition flood. See Revision 5
- **Revision 6:** 21 Oct 2026, F-09 v1.6 (QA-09-r3-1): `MaxBytes` bounds the client's input, not the reparse of the strip's output; a reparse time limit is the upload's. No limit values change. See Revision 6
- **Revision 7:** 22 Oct 2026, the user's decision, F-09 v1.11: `Timeout` becomes the server setting `WORKOUT_EXPLORER_PARSE_TIMEOUT` (whole seconds, default 10, 5 to 60, invalid → exit 2), read by `serve`, `parse` and `regenerate`. Every other limit stays fixed. Peak memory unchanged; the upload write deadline scales with it (ADR-021 rev. 4). See Revision 7

## Revision 8 (4 Nov 2026, QA-30-r1-6): replaced wording

- Header: "**Revision:** 7 (22 Oct 2026)."
- Rev. 4 item 3, end: "During a deploy, a `regenerate` process (F-30), with its own parser and two slots, adds up to 2·A if it runs next to `serve` (item 5). F-31 either adds 2·A to the cap or never runs the two at once." F-31 did neither (cap 1,200m from `serve` alone); QA measured `regenerate` at 365 MB, giving about 1,338 MB worst case. Rev. 8 accepts the overlap instead.
- Item 5: "sized from the measured M(N) plus a `regenerate` process (F-30) during deploys."
- Consequences, first bullet: "so during a deploy up to four parses can run on the machine. F-31 sizes the VPS and its `MemoryMax` with that in mind, using the model of rev. 3 item 4."

## Item 4, closing paragraph (rev. 7 wording)

Every limit is a field of `Limits`, so tests can pass small values. Production uses `DefaultLimits` unchanged. A different value needs a revision of this ADR. (rev. 7: except `Timeout`, which production takes from the server setting `WORKOUT_EXPLORER_PARSE_TIMEOUT`; `DefaultLimits.Timeout` stays 10 s and is that setting's default. See Revision 7.)

## Revision 1: sport messages, normalisation cost, all-caps measurement

**Findings (F-08 QA round 1).**
- F-08 design v1.1 retains a new message kind: `sport` messages become sport hints (QA-08-r1-1). Like every retained message, they need a budget, or a file of 20 MB of `sport` messages grows the `Recording` without bound.
- QA-08-r1-5: with every budget at its cap in one file (499,999 records, 100 sessions, 10,000 laps, 100,000 timer events), a parse took 1.13 to 1.15 s: decode 0.40 s, `Normalise` 0.91 s, against 0.12 s without the sessions. The cause is a per-sample scan over the sessions (R-5 and R-7 membership). Item 7 measured the record cap only, so the real worst case was more than twice the recorded one, and item 2's "the budgets bound their cost" was true only in the weak sense (O(n · sessions)).

**Decision (amends items 2, 4 and 7).**
1. **Item 4, new budget:** `MaxSportMessages` = 1,000 `sport` messages, limit name `sport_messages` (ADR-018 rev. 1). Basis: real files write one per activity, a multisport file plausibly one per leg; 1,000 is 200 × a triathlon. Counted on arrival like the others.
2. **Item 2, complexity bound instead of a deadline.** `Normalise` and `SampleCount` stay outside the decode deadline and take no context (`workout` stays pure, overview section 5). They must run in O((n + k) log(n + k)), with n = records or samples and k = sessions + laps + timer events + sport hints. That rules out any loop over sessions, laps, timer events or hints per sample. Since every input is bounded by a budget, the bound gives a bounded time.
3. **Item 7, measured at all caps at once.** `BenchmarkParseAtCap` and the RSS run use one file with every budget at its cap: 500,000 records at 1 Hz with every stream, 100 sessions alternating `cycling` and `running`, 10,000 laps with a sport, 100,000 timer events, 1,000 `sport` messages, 256 field descriptions, within `MaxBytes`. Decode and normalise are reported separately. The stop rule is unchanged (≤ 400 MB max RSS, ≤ 2 s decode + normalise on the development machine).

**Why not put normalisation under the deadline.** It would need a context in `workout` and checks in its hot loops, for a cost that the budgets and the complexity bound already cap. The deadline exists for the decoder, whose CPU per byte is the part a hostile file controls (developer-field lookups). Normalisation's cost depends only on counts the budgets fix.

**Would choose differently if** the all-caps measurement exceeds the stop rule with the bound met (lower `MaxSessions` or `MaxLaps` in a revision first), or F-31 measures decode + normalise above half of `Timeout` on the production VPS (then give `Normalise` a cancellation check per stage, not per sample).

## Revision 2: `Accept` (F-09, ADR-023)

**Context.** Uploads go through `(*Parser).Accept`: parse, strip identifying data (UP-9), and reparse the stripped bytes to prove the workout is unchanged (ADR-023 item 1).

**Decision (amends items 2, 5 and 7).**
1. **One slot for the whole `Accept`.** The slot is taken once, before the first parse, and released after the reparse. Slot waiting still respects the caller's `ctx`.
2. **Deadline per decode.** Each of the two decodes gets its own `Timeout`; the strip in between is linear in the input bytes and checks `ctx` between sequences. `Accept` returns within 2 × (`Timeout` + normalisation at the caps) plus the strip.
3. **Memory.** Before the reparse, the first workout is reduced to a 32-byte reference (ADR-023 item 1.2) and dropped. The peak is one parse plus the upload bytes and the stripped copy (each at most `MaxBytes`).
4. **Measured.** `BenchmarkAcceptAtCap` uses the all-caps shape (Revision 1 item 3). The RSS run is `parse -original` on the all-caps file. Stop rule: max RSS ≤ 400 MB (unchanged), each decode + normalise ≤ 2 s (unchanged), the whole `Accept` ≤ 4.5 s on the development machine. If any is exceeded, the engineer stops and reports.

## Revision 3: the first parse must be unreachable; server memory model (F-09 round 1)

**Finding.** `parse -original` on the all-caps twin peaked at 413 to 467 MB max RSS (3 runs), over the 400 MB rule. A plain `parse` of the same file peaks at 230 MB. `BenchmarkAcceptAtCap`: 2.3 s per call (within 4.5 s), 886 MB allocated per call. `GOGC=50` brings the RSS to 349 MB. The limits are not the cause: the code does not meet Revision 2 item 3. While the strip and the reparse run, the first parse is still reachable in two ways:
- `run`'s named result `w` holds the first workout across `p.strip` (`internal/ingest/ingest.go:80`, `:86`). `strip` clears only its own parameter copy (`accept.go:40`), and that copy shares its slices with `w`.
- `outcome.l` points at the first parse's listener (`ingest.go:120`), and so at its whole decoded `Recording` (every record, lap, session and timer event). The deferred log line at the end of `Accept` needs it (`log.go:24`, `:67-69`).

So the reparse runs while a complete first parse is still live, and the GC's heap goal doubles that.

**Decision (amends Revision 2 items 3 and 4).**
1. **No limit, cap, slot count or stop rule changes.** No GC setting in code: no `debug.SetGCPercent` and no `debug.SetMemoryLimit`. A lower `GOGC` would hide the retention, cost CPU and apply to the whole process. A lower cap would refuse workouts the user accepted (OQ-31) to pay for a bug. A higher RSS rule has no basis.
2. **"Dropped" means unreachable.** Once the reference digest exists, nothing reachable from `Accept`'s frames or from its `outcome` may point into the first workout or into the first listener's `Recording`:
   - `run` computes the reference itself and clears its own `w` before the strip. `strip` receives the 32-byte reference, never a workout.
   - `outcome` holds no `*listener`. When `parse` returns, on every path, it copies the values that the log line and `errorOf` need into the outcome: file type, device, Garmin product name and the count of record messages. The listener and its `Recording` then die with `parse`'s frame. The reparse uses its own `outcome`, which is dropped after the check.
3. **Proof.**
   - `TestAcceptReleasesFirstParse` (CI, deterministic): the heap at the reparse is at most the stripped copy plus 1 MiB above the baseline (F-09 design, section Tests).
   - The RSS rule of Revision 2 item 4 is unchanged: on each of 3 runs, with neither `GOGC` nor `GOMEMLIMIT` set.
4. **Server memory model.** This replaces the memory sentence in ADR-021's Consequences. A request's memory depends on its stage:

   | Stage of one upload request | Holds | Bound at the caps |
   | --- | --- | --- |
   | Reading the body, or waiting for a parse slot | The upload U | ≤ 20 MB |
   | Inside `Accept` (at most `MaxConcurrent` = 2 at once) | U, the stripped copy, one parse's working set | **A** = max RSS of `parse -original` on the all-caps twin, ≤ 400 MB |
   | After `Accept`, lifecycle steps 2 and 3 (encode) | U, the stripped copy, the reparsed workout, the data file and the encoder's working set | **B**, measured with the server run (F-09 design, section Tests) |
   | Lifecycle steps 4 to 6 (files, row) | U, the stripped copy, the data file | < B |

   Worst case of `serve`: **M(N) ≤ 2·A + (N − 2)·B for N ≥ 2**, and A for N = 1. The default N = 4 gives 2·A + 2·B. This estimate uses synthetic figures and is replaced by the measured row:
   - After the fix, A is about 270 MB (230 MB plus the upload and the stripped copy).
   - B is about 150 to 250 MB. The workout is close to `Normalise`'s 98 MB, the encoder is the 12-hour file's 8.8 MB per run scaled to 500,000 samples, and the upload and stripped copy add about 35 MB.
   - So M(4) is about 0.85 to 1.05 GB.

   N = 16 allows up to 2·A + 14·B in the worst case. In practice, requests waiting for a slot hold only U.
5. **Runtime variables are F-31's.** `GOMEMLIMIT` is read by the Go runtime, not by our code. If F-31 sets it, the systemd unit sets it, next to `MemoryMax` and below it, sized from the measured M(N) plus a `regenerate` process (F-30) during deploys. Every stop rule in this ADR is measured without it.

**Would choose differently if**
- After the fix, `parse -original` is still above 400 MB with `TestAcceptReleasesFirstParse` green. Then profile the heap at the reparse first. Next, reduce the peak inside `workout` (see Would choose differently if, third point). Only then lower caps, in a revision.
- The measured M(4) is above 1 GB and F-31's machine cannot hold it, or the operator needs N above 4 on a small machine. Then put lifecycle steps 2 and 3 behind a gate of 2, so only two reparsed workouts are being encoded at once. That gives M(N) ≤ 2·A + 2·B + (N − 4)·(U + stripped copy + data file).

## Revision 4: the rate table in the memory model; OQ-51 (F-09 round 1 QA)

**Finding (QA-09-r1-4).** Revision 3 item 4 counts only request memory. The upload limiter's key table is process memory that an attacker can fill from many addresses, and the server run never fills it. At its 100,000-key cap it held about 185 MB, a quarter of the measured M(4).

**Decision (amends Revision 3 items 4 and 5).**
1. The limiter's per-key log shrinks to a fixed array of whole seconds (ADR-021 rev. 3 item 8), which bounds the full table at **L ≤ 40 MB**. `TestLimiterMemory` proves the bound.
2. Worst case of `serve`: **M(N) ≤ 2·A + (N − 2)·B + L** for N ≥ 2, and A + L for N = 1. The server run measures the first part, so F-31 adds L to the measured M(4).
3. **OQ-51, decided by the user (16 Oct 2026):** the VPS has at least 2 GB RAM, and N stays at its default of 4. F-31 sets systemd `MemoryMax` at or above the measured M(4) + L. During a deploy, a `regenerate` process (F-30), with its own parser and two slots, adds up to 2·A if it runs next to `serve` (Revision 3 item 5). F-31 either adds 2·A to the cap or never runs the two at once. `GOMEMLIMIT`, if used, goes below `MemoryMax`, as before.

**Would choose differently if** F-27's report limiter, or any later in-memory table that clients can grow, has no bound of its own. Each such table states its bound and adds it to M(N) in the same way.

## Revision 5: the record counter (F-09 v1.5, QA-09-r2-1)

**Finding.** With a `MesgDefListener` set, v0.28.4 copies a definition's field and developer-field lists for the listener (`decoder.go:613-621`, `sliceutil.Clone`). Our counter keeps only an int. The copies are garbage as soon as `OnMesgDef` returns.

**Decision (adds a row to the table in Context; no limit changes).**

| Path | Bounded by the library? | Growth |
| --- | --- | --- |
| Definition copies for the definition listener (rev. 5) | Yes. One copy per definition, at most 765 bytes, nothing retained | Allocation linear in bytes (a definition is at least 6 bytes); CPU linear |

A file made only of definitions is not counted by any budget, because definitions are not messages. Its cost is bounded by `MaxBytes` and checked by the decode deadline, as before.

**Measured.** `BenchmarkAcceptDefinitions` (F-09 design, section Tests) runs `Accept` on two in-memory 20,000,000-byte uploads: one made of minimal definitions (1 field each) and one of definitions with 255 fields each, both behind one `file_id` and one `record`. The stop rule is the one of Revision 2 item 4: at most 4.5 s per `Accept` on the development machine. `BenchmarkAcceptAtCap` and A are measured again too (unchanged rules).

**Would choose differently if** the flood exceeds the rule. Then add a definition budget to `Limits` (a revision of item 4), counted in the counter's `OnMesgDef`, which cancels the decode like the other budgets.

## Revision 6: limits on the reparse (F-09 v1.6, QA-09-r3-1)

**Finding.** The stripped file can be longer than the upload. A sequence whose file CRC was missing gains 2 bytes. Since ADR-023 rev. 5, a crafted file whose sequences overlap stores the overlapping bytes twice (ADR-023 rev. 5 item 7: less than 8.4 MB more in total). An upload that `Parse` accepts close to `MaxBytes` was therefore refused by `Accept`'s reparse as `limit`/`bytes`, reported as `internal`.

**Decision (amends Revision 2 items 2 and 3).**
1. `MaxBytes` is checked on the input of `Parse` and of `Accept`'s first parse. The reparse in `Accept` does not check it. Its input is our own output, bounded by ADR-023 item 7. Every other budget and the decode deadline apply to the reparse unchanged, and none of them can be exceeded by the strip: removing messages only lowers the counts.
2. A reparse that hits the decode deadline is reported as the upload's `limit`/`time` (ADR-018 rev. 4), not as `internal`. A deadline depends on load, so a second decode can exceed it when the first did not.
3. Memory (Revision 2 item 3): the stripped copy is at most `MaxBytes` plus ADR-023 item 7's growth. That growth is 2 bytes per sequence, plus under 8.4 MB only for a crafted overlap. A and the stop rule are unchanged; the all-caps file has no overlap.
4. F-30 reads stored originals, which are our own output too. Its design gives `Accept` on a stored original the same allowance: `MaxBytes` plus ADR-023 item 7's growth.

**Would choose differently if** a later change to the strip can grow its output by more than ADR-023 item 7 says. Then the reparse gets an explicit byte bound again, `MaxBytes` plus the new growth.

## Revision 7: the time limit is a server setting (user, 22 Oct 2026; F-09 v1.11)

**Context.** The user decided that the decode deadline (`Timeout`, item 4) becomes configurable through an environment variable, like `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` (ADR-021 item 4). The reason is that the server may be an old, slow machine. Item 4 had fixed every production limit.

**Decision (amends items 1, 4 and 5, and Revision 2 item 2).**

1. **The setting.** It follows ADR-021 item 4 rule for rule.

   | Property | Value |
   | --- | --- |
   | Name | `WORKOUT_EXPLORER_PARSE_TIMEOUT` |
   | Format | Whole seconds: ASCII digits without sign, unit, spaces or leading zero (`^[1-9][0-9]*$`, at most 2 digits). `10s`, `1m` and `10.0` are invalid |
   | Unset or empty | `ingest.DefaultLimits.Timeout` (10 s). That is the only source of the default; `cmd` does not repeat the number |
   | Range | 5 to 60 s inclusive |
   | Invalid | The subcommand prints `<subcommand>: WORKOUT_EXPLORER_PARSE_TIMEOUT must be a whole number of seconds from 5 to 60` to stderr and exits 2, before it opens the database, the data directory or the input file |
   | Logged | `serve`'s `server started` line gains `parse_timeout_s` (integer) |

   Why whole seconds and not a Go duration (`time.ParseDuration`): one strict format for every setting (overview section 6). Sub-second values have no use here, and `ParseDuration` accepts forms like `1m-50s`, `+30s` and `10000ms`. The name keeps the user's suggestion. A unit mistake (`30s`) cannot pass silently, because it fails with a message that names the unit.

2. **Where it is read and passed in.**
   - `cmd/workout-explorer/settings.go` reads it through the injected `getenv`, once at start, next to the in-flight cap. No other package reads it. `ingest` never reads the environment.
   - The value reaches the parser as `ingest.Limits{Timeout: t}`. Every other field stays zero, which means its default. One helper in `cmd` builds the parser from it, and every subcommand that constructs a `Parser` uses that helper:
     - `serve` (F-09).
     - `parse`, the diagnostic. The operator needs it to measure the production machine (item 6) and to reproduce a time limit seen in the log.
     - `regenerate` (F-30), item 5.
   - `ingest` gains `(*Parser).Limits() Limits`, the limits in effect after defaults. It is a read-only copy, so a test can prove the wiring.
   - `ingest` itself checks nothing about the range. Tests still pass any positive `Timeout`, and zero still means the default.

3. **Only `Timeout`.** Every budget, `MaxBytes` and `MaxConcurrent` stay fixed. A different value of any of them still needs a revision of this ADR. The stop rules of item 7 and Revisions 1, 2 and 5 are unchanged. They are measured on the development machine with `DefaultLimits` and never with the setting.

4. **Why 5 to 60.**
   - **Upper bound 60 s.** One client address may start 60 uploads per hour (ADR-021 item 7). A crafted file that runs each of its two decodes almost to the deadline holds a parse slot for up to 2·T. So one address can occupy 60 × 2·T = 120·T slot-seconds per hour, out of 2 slots × 3,600 s = 7,200. Its share is **T / 60 s**: a sixth at the default and all of it at 60 s. Above 60 s, one address within its rate limit could keep both slots busy all the time, and friends' uploads would wait behind it or be refused as `busy`.
   - **Lower bound 5 s**, half the default. The setting exists to give a slow machine more time. A lower value has no use case and could refuse files at the caps under load even on the development machine.

5. **Consequences of a configured T.**

   | Concern | Effect |
   | --- | --- |
   | Peak memory | **Unchanged.** A, B, L and M(N) (Revisions 3 and 4) are bounded by the budgets and by the in-flight cap N, not by time. A longer T makes a request hold the same memory for longer. Only requests whose file runs into the deadline are affected, and N still bounds how many exist at once. `MemoryMax` sizing (Revision 4 item 3) does not change |
   | `Accept` duration | At most 2 × (T + normalisation at the caps) + the strip (Revision 2 item 2, with the configured T). Slot waiting is still not counted |
   | Concurrency | Still 2 parse slots and N in-flight uploads. A file that hits the deadline holds its slot for up to 2·T instead of 20 s. Item 5's promise holds unchanged: one hostile file holds one slot and cannot hold up a friend's upload in the other. The share one address can take is item 4 above |
   | HTTP write deadline | Read deadline + 6·T: 180 s at the default, unchanged; 480 s at 60 s (ADR-021 rev. 4). Without this, a friend's upload queued behind two crafted ones at T = 60 s could finish after the fixed 180 s. Its draft would be stored, but the response lost |
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
   - Revision 1's trigger ("decode + normalise above half of `Timeout` on the production VPS") now reads against the configured T. The procedure keeps W ≤ T/4, so the trigger fires only if the procedure was skipped.

**Main cost.** One more environment variable, and an HTTP write deadline that depends on it. A slow machine's operator can raise T to 60 s, at which point one client address can saturate both parse slots with crafted files (item 4). The bound makes that the worst case, not a mistake past it.

**Would choose differently if**
- `fit rejected` lines with `limit` `time` arrive at close to 60 per hour while friends get `busy`: lower T, or move rate limiting into Caddy (ADR-021, "Would choose differently if").
- The operator needs more than 60 s for legitimate files: lower the caps in a revision. A higher bound would let one client address deny parsing to everyone.
- A second limit has to vary per machine (for example `MaxConcurrent` on a single-core machine): add it as its own setting, by the same rules, in a revision. Do not widen this one into a "limits profile".

## Consequences, first bullet (rev. 7 wording)

- One parser per process. `serve` and `regenerate` (F-30) are separate processes, so during a deploy up to four parses can run on the machine. F-31 sizes the VPS and its `MemoryMax` with that in mind, using the model of Revision 3 item 4. (rev. 7) Both read the same `WORKOUT_EXPLORER_PARSE_TIMEOUT`, so F-31 gives them one environment (Revision 7 item 5).
