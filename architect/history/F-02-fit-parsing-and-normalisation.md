# F-02 FIT parsing and normalisation: history

History of [architect/F-02-fit-parsing-and-normalisation.md](../F-02-fit-parsing-and-normalisation.md), moved out of the live document on 22 Oct 2026 (restructuring, no decision changed). Read it only when you need the history.

## Change log

| Version | Change |
| --- | --- |
| 1.11 | F-08 design 1.4 (QA-08-r3-2). R-6a changes: the device's distance stream is kept as recorded and then made non-decreasing by F-08's **R-14** (running maximum over valid values, right after R-3), because a real writer's distance falls at a leg boundary and drops to near 0 inside an open-water swim. Marked in place; the F-08 design is authoritative. OQ-6 resolved: a third FIT writer (`garmin`) is available locally; AC 2 is ticked when QA reruns `make corpus` on the operator's machine. No other rule changes |
| 1.10 | F-08 design 1.3 (user decision OQ-38). F-08 adds rule **R-13 transition legs** between R-3 and R-4: at transition samples and in transition session and lap totals only position, altitude, heart rate, distance and temperature survive; the rest (speed, cadence, power) become missing samples or absent totals, which every later rule reads as missing. R-7 gains one clause in each branch: the whole transition counts as moving (F-08 R-13.M, decided by the user, answer 2). Speed branch: a transition sample with `Δt < 5` adds `Δt` without the threshold. Timer branch: moving time is the length of the union of the timer-running intervals and the transition intervals, so transition time while the timer was stopped is added and nothing is counted twice. Both are marked in place; the F-08 design is authoritative for them. No other rule changes |
| 1.9 | F-08 design 1.2 (QA-08-r2-3, user answers OQ-35 and OQ-36). The boundary paragraph on the `multisport` exception is marked as history; F-08 D-17 (confirmed) is the sport rule. R-1 and D-3 are restated as F-08 D-19: one time axis from the workout start across every session and FIT sequence, never rebased; the multi-leg summary elapsed time is the wall time on that axis (F-08 D-18). No rule of this document changes. |
| 1.8 | F-08 design 1.1 (F-08 QA round 1) replaces three more parts, marked where they appear; the F-08 design is authoritative for them. (1) R-5's membership sentence becomes F-08's **sport timeline**: sessions first, then sport hints (`sport` message, `lap.sport`) for samples no session covers, then the old session fallback; R-7's threshold uses the same timeline. (2) R-9's sport bullet becomes F-08's "Workout sports `S` and R-9 sport": `multisport` only when the sports differ. (3) E-4 holds per first FIT sequence (F-08 D-15). The `Recording` gains `Sports`, `SequenceStarts` and `RawLap.Sport`, and F-08's R-12 (distance across sequences) runs before R-2. Rules R-1 to R-4, R-6 to R-8 and E-1 to E-3, E-5 to E-8 are unchanged. |
| 1.7 | F-08 design 1.0 replaces three parts of this document. They are marked where they appear, and the F-08 design is authoritative for them. (1) The entry point `ingest.Parse(ctx, io.Reader)` becomes `(*ingest.Parser).Parse(ctx, []byte)` (ADR-017). (2) R-10 Summary is replaced by F-08's revised R-10 and the new R-11 (record totals), and R-8 also yields elevation loss. (3) The golden report becomes version 2 (`integrity`, new `derived` keys). The boundary rows assigned to F-08 are designed there. Rules R-1 to R-9 and E-1 to E-8 are unchanged. |
| 1.6 | QA round 3: `.gitignore` covers every case of `.fit`; everything derived from real files stays local (both in ADR-004 rev. 3). OQ-6: a second FIT writer is available (`wahoo_fitness`); one more is needed. |
| 1.5 | AC numbers follow the reworded feature file (OQ-7 closed by requirements). The old AC 1 is now AC 1 (synthetic, CI) and AC 2 (real, operator's machine); every later AC moves down one place (old 2→3, …, old 11→12). The Required files table now marks every `synthetic/` file as required ("real preferred" removed: real files stay local). **Change-log entries 1.0 to 1.4 keep the old numbers.** |
| 1.4 | User decision 10 Oct 2026: real FIT files are not committed (ADR-004 rev. 2). Real files and their golden reports are git-ignored; the corpus README has a local-only procedure and no per-file rows for real files; AC 1 is verified on the operator's machine (feature-file wording goes to requirements); OQ-3 closed, OQ-6 opened (brands still missing). |
| 1.3 | QA round 1 ([qa/F-02-fit-parsing-and-normalisation-r1.md](../../qa/F-02-fit-parsing-and-normalisation-r1.md)). Q1: D-10 gives the reason `parse` ships in the binary; the feature file wording goes to requirements. Q2: new edge rule E-8, a FIT `date_time` below `0x10000000` (device system time) counts as missing; new corpus file `system-time-record.fit`. Q3: the boundary section says how F-08 reuses R-9. r1-1: any corpus entry that is not lower-case `*.fit` fails both drivers (ADR-004 item 4). r1-2, r1-3: thresholds and :45 time zones pinned by tests. r1-5: `TestSyntheticCorpusCurrent` (ADR-004 item 5). |
| 1.2 | Implementation follow-up. The rolling-hills assertions are replaced by the noise-free and noisy checks in R-8 tests: the zig-zag filter picks up about 0.35 m of smoothed noise per bump, by design, so "60 ± 4 m" with noise was a wrong target. E-7 now names `device` explicitly. |
| 1.1 | Engineer stop during implementation. **D-11:** component expansion is turned off, because in v0.28.4 an expanded `altitude`/`speed` overwrites a recorded `enhanced_*` value, which breaks AC 4 at file level. `ingest` falls back to the legacy fields itself (section Decoding; ADR-003 rev. 1). Engineer assumptions settled in section Edge rules (E-1 to E-7). `offsets_digest` uses integers. New synthetic file `rolling-hills.fit` makes the elevation filter visible in the corpus. `enhanced-and-legacy.fit` gets three segments. |
| 1.0 | First version. |

## Superseded text, as it stood in v1.11

**Boundary with later features, closing paragraph:** *(v1.9: the rest of this paragraph is history. The workout sport rule is F-08's "Workout sports `S` and R-9 sport" (F-08 D-17, confirmed by the user, OQ-36): `multisport` only when the sessions and the record leg have two or more distinct sports; two sessions of one sport give that sport. It lives in `workout`, as below, and is the only sport rule.)* One deliberate exception: the workout-level **sport** is set to `multisport` when there are 2+ sessions (decision 12), because F-03 needs a workout sport and the rule is a property of the normalised workout, not of validation. F-08 keeps the totals rule. **For F-08 (v1.3, QA r1 Q3):** F-08's design reuses R-9 (`workout` sport rule) and must not rebuild it. Chained files get `multisport` only once F-08 decodes the later sequences into the same `Recording`; F-02 decodes the first sequence only.

**Public surface, `ingest` entry point and its paragraph:**

```go
// package ingest
func Parse(ctx context.Context, r io.Reader) (workout.Workout, error)
```

`Parse` = decode into a `Recording` (listener) + `workout.Normalise`. It does not wrap `r` in a `bufio.Reader` (the decoder buffers). *(v1.7: replaced by `ingest.NewParser(Limits, *slog.Logger)` and `(*Parser).Parse(ctx, []byte)`, ADR-017 and the F-08 design. `Normalise`, `Report` and `ReportJSON` keep their signatures; the report is version 2.)*

**E-6 status cell:** **Corrected** (was 3 decimals)

**R-5, membership sentence:** A sample or lap belongs to the session with `Start ≤ offset ≤ End`; outside every session: the last session that started before it, else the first session; no sessions: no conversion. *(v1.8: the membership sentence is replaced by the F-08 design's sport timeline, which adds sport hints for samples no session covers and also serves R-7. Sessions still win where they cover a sample.)*

**R-9, sport bullet:** - Sport: one session → its sport; 2+ sessions → `multisport` (decision 12); no session → `""`. Pool swims need nothing special (decision 33): no position stream, other streams as present. *(v1.8: replaced by the F-08 design, "Workout sports `S` and R-9 sport": the distinct sports of the sessions and of the record leg; one → it, two or more → `multisport`. v1.9: confirmed by the user, OQ-36.)*

**R-10 Summary (the F-02 rule, kept for history):** **R-10 Summary.** *(Replaced by the F-08 design, section Summary, v1.7 of this document. The text below is the F-02 rule, kept for history.)* One session: `Summary.Totals` = that session's totals (AP-2: device values as recorded), with fallbacks only where the device recorded nothing: `ElapsedTime ← Derived.ElapsedTime`, `Distance ← Derived.Distance`, `Ascent ← Derived.ElevationGain`. `Summary.MovingTime ← Derived.MovingTime` always (TD rule, D-5). No other value is computed (decision 5: the summary shows what the device recorded). Zero or 2+ sessions: only the four derived values (F-08 adds the multi-session rule).

## Closed open questions

| ID | Question | Blocks | Owner |
| --- | --- | --- | --- |
| ~~OQ-3~~ | Closed 10 Oct 2026 by user decision: real files are not committed (ADR-004 rev. 2). | — | — |
| ~~OQ-6~~ | Resolved 15 Oct 2026 (v1.11): a third FIT writer is available locally, `garmin` (an Edge Explore ride, `garmin-edge-explore-ride.fit`, which parses and has a local golden), next to the third-party app on the Apple Watch and `wahoo_fitness` (ELEMNT ROAM). AC 2 is ticked when QA reruns `make corpus` on the operator's machine and lists the three manufacturers. | — | — |
