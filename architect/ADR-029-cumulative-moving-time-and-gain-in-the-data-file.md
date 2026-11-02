# ADR-029 Range moving time and gain come from cumulative streams written by the server (data file format v2)

- **Status:** accepted (26 Oct 2026, with the F-17 design v0.2)
- **Date:** 26 Oct 2026; rev. 1 of 29 Oct 2026 (F-17 QA r1: `M` anchored at 0 on the first sample, corpus-wide invariants)
- **Features:** F-17 (introduces it); F-18 (splits, computed lap values), F-19 (`draft.bin`), F-30 (regenerate)
- **Supersedes:** none. Bumps [ADR-005](ADR-005-workout-data-file-format-v1.md) to version 2 under its item 9. Refines the TD's Implementation note "cumulative distance, gain and heart-rate sums are built once"
- **Depends on:** ADR-005, ADR-007, ADR-026 item 3; F-02 R-7 and R-8; F-08 R-13.M
- **Requirements:** AP-7, AP-8; NFR Performance (data file under 500 KB compressed for 4 hours)

## Context

The range panel (AP-7) needs moving time and elevation gain for any range in constant time. Both are workout-model rules on the server:

- Moving time (F-02 R-7) is taken from timer events when the file has any, as nearly every real device file does, and otherwise from a speed threshold per sport; F-08 R-13.M adds transitions. ADR-005 item 7 keeps timer events out of the data file.
- Gain (F-02 R-8) is a smoothed altitude with a 3 m hysteresis filter.

The TD gives every normalisation rule to `workout`, and ADR-026 item 3 says that a value the page lacks is added by the server, not derived by a renderer.

Rejected:

- **The browser applies the speed rule.** It cannot reproduce the timer source, so the moving time of a whole-workout range would differ from the summary on almost every real file. R-7 and R-13.M would also exist in two languages.
- **Timer events go into the metadata.** The browser would still re-implement R-7 and R-13.M, and the public file would carry more than the page shows.

## Decision

1. **`workout` computes two per-sample cumulative series** after normalisation and before any public transform (F-16 H-1):
   - Moving time `M` (s): `M[0] = 0`; for `i ≥ 1`, `M[i]` is R-7's moving time, R-13.M included, within `[0, Offset[i]]`, so moving time before the first sample counts in the first interval. Moving time after the last sample counts in the last one. It never decreases, and `M[N−1]` equals the summary's moving time in whole seconds, rounded the same way (rev. 1, QA-17-r1-1).
   - Gain `G` (m): it never decreases, `G[0] = 0`, and it rises only inside climbs that R-8 counts, so `G[N−1]` equals R-8's total. The backend engineer chooses how a climb is spread over its samples.
   
   A series is absent when its total is absent: no moving-time source for `M`, no altitude for `G`.
2. **Data file format version 2** carries them as two int32 delta streams (1 s and 0.1 m). They have no missing samples. Their names are in F-03's normative section. They are never charted and never shown in the readout.
3. **The browser holds no moving-time or gain rule.** A range's value is `M[b] − M[a]` or `G[b] − G[a]` (F-17 R-2). The server's computed lap values use the same series.

## Consequences

- **Version bump:** the writer, the decoder and the goldens change in one commit. A deploy regenerates every workout, and kept workouts stay on their old bundle (ADR-005 item 9).
- The worst-case raw size grows from 27 to 35 bytes per sample (a 4-hour file with every stream is about 504 KB uncompressed). The budget counts the compressed file. The deltas are almost all 0 or 1, so brotli should add little; F-17 measures it on `ride-4h` and `ride-12h`.
- A whole-workout range shows the summary's moving time by construction (`M[0] = 0`, `M[N−1]` = summary). The invariants of item 1 (`M[0] = 0`, `G[0] = 0`, never decreasing, last value = summary) are checked on every accepted corpus file, synthetic and local real (`make corpus`), not only on hand-built recordings. The synthetic corpus has at least one file whose first sample is after the start with the timer running.
- A whole-workout range's gain is R-8's derived value, not the device's ascent (OQ-24, accepted).
- F-17 needs a backend step before its frontend part.

## Would choose differently if

- The measured compressed growth on `ride-4h` exceeds 5 KB. Then use a narrower per-sample encoding, in a new ADR.
- A later feature brings timer events to the browser for another reason (none is planned).

## Reversibility

Cheap: every deploy rebuilds every data file, and workouts live at most 30 days. No one-way door.
