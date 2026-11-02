# ADR-003 FIT decoding library: `muktihari/fit` behind the ingest anti-corruption layer

- **Status:** accepted
- **Revision:** 6 (21 Oct 2026). First version 10 Oct 2026. The "rev. N" labels are the stable IDs code and other documents cite; each marks a decision made in that revision, stated here in its current form
- **History:** [architect/history/ADR-003-fit-decoding-library.md](history/ADR-003-fit-decoding-library.md) (revision list, findings, the revision sections as written)
- **Features:** F-02 (introduces it), F-08 (extends it), F-09 (strip, ADR-023), F-30 (regenerate uses it)
- **Supersedes:** none
- **Requirements:** TD Tech stack (FIT decoding), TD FIT parsing, TD Principles ("One parser", "Lean"), NFR Reliability, TD invariant I-2

## Context

The server must decode FIT activity files from any brand (DS-2) into one normalised workout. The TD names `muktihari/fit` and asks to confirm it against the corpus; the feature file forbids Garmin's FIT SDK as a fallback. Lean principles: no external services, few dependencies, one static binary (`CGO_ENABLED=0`).

Facts checked on 10 Oct 2026:

| Item | `github.com/muktihari/fit` | `github.com/tormoder/fit` (the other mature Go library) |
| --- | --- | --- |
| Licence | BSD-3-Clause (LICENSE file, © 2023 Hikmatulloh Hari Mukti). Garmin's `Profile.xlsx` and `testdata/from_official_sdk/*` in the repo are under the FIT SDK licence and used only for code generation and tests; they are not compiled into importers | MIT |
| Status | Active: v0.28.4 published 11 Sep 2026; v0.27.1 to v0.28.4 released Jan to Sep 2026; FIT profile 21.217 | Its README (Sep 2024) says it is no longer actively maintained and points to `muktihari/fit` as successor; profile 21.115 |
| Go version | `go 1.26.0`, same as ours (`src/server/go.mod`) | — |
| Imports of the packages we use (`decoder`, `profile/mesgdef`, `profile/typedef`, `proto`) | Standard library plus packages of the same module only. The module's `go.mod` lists tooling modules (`carto`, `xlsxreader`, `misspell`, `go-cmp`, `x/text`) used by its code generator and tests; they are not compiled into our binary | none |
| Relevant API | `decoder.New(r, opts...)`, `DecodeWithContext(ctx)`, `WithMesgListener`, `WithBroadcastOnly`, `WithIgnoreChecksum`, `Next()` for chained files; typed `mesgdef.New<Mesg>(*proto.Message)` constructors with raw values and the profile's invalid sentinels; component expansion (e.g. `speed` → `enhanced_speed`) on by default and can be switched off with `WithNoComponentExpansion()` (we switch it off, rev. 1); unknown messages tolerated | Comparable, older profile |

v0.28.4 behaviour the decisions below rely on (`decoder/decoder.go`):
- (rev. 2) Messages reach the listener as soon as each one is complete, and the file CRC is checked only after the last message. So a truncated file or one with a wrong file CRC has already delivered all its intact messages when the error comes. A wrong header CRC is reported before any message. Errors are sticky: after one, `Next()` and `Decode` return the same error. A header with data size 0 is refused as `ErrNotFITFile`, although unclosed device files have exactly that.
- (rev. 5) The decoder rejects some records that a framer can still read: a definition whose base type fails `basetype.BaseType.Valid()` (`decoder.go:572-575`), or a data message whose developer field points to a field description with an invalid `fit_base_type_id` (`decoder.go:891-893`). F-08 R-a then treats the sequence as truncated, and the stored file ends at the rejected record (ADR-023 rev. 4 item 6a).
- (rev. 6) Records are decoded `for d.cur < DataSize`, and each record is read in full (`decoder.go:1010-1016`). A record that starts before the header's data size and ends after it is accepted and counted, and the file CRC is then read after it. F-08 counts such a sequence complete, and the strip stores exactly the records the decoder read and repairs the data size (ADR-023 rev. 5 item 6).

## Decision

1. **Use `github.com/muktihari/fit`, pinned at `v0.28.4`** (exact version in `go.mod`, `go.sum` committed). Upgrades are a deliberate change that must pass `make corpus` with no diff, or with every diff explained in the hand-off. **Upgrade check**, recorded in the upgrade hand-off:
   - (rev. 2) The growth paths listed in ADR-017 (retained state, recursion with expansion off, goroutines, super-linear loops), and whether decoder error texts still contain no field values (ADR-018).
   - (rev. 4) A test enumerates every field of the pinned profile and applies the identifier rule of ADR-023 item 4a. It fails if an identifier-like field is neither in the blank table nor in `keptIdentifiers`, so an upgrade that adds such a field fails until someone decides it.
   - (rev. 5) Where the decoder stops is part of what is stored: `make corpus` must show no diff in `ride-30min-bad-basetype.fit` (a definition with an invalid base type, then more records; F-09 design, section Corpus). A diff there, or any other changed `integrity.truncated` or `data_end` in a golden, means the new version stops at a different record than v0.28.4. The check also confirms that the decoder still calls `OnMesgDef` only after a whole definition has been read and accepted, and `OnMesg` only after a whole data message.
   - (rev. 6) `make corpus` must show no diff in `ride-30min-size-short.fit` (header data size ends inside the last record). A diff means the new version stops before that record or treats it differently.
   - (rev. 6) **Stored workouts are not a gate (OQ-53, decided by the user, 21 Oct 2026).** A diff in those files or in any golden's `integrity.truncated` or `data_end` is explained in the upgrade hand-off like every other diff. It is judged by what it does to new uploads. Workouts already stored are regenerated with the new version by F-30. If they come out shorter, that is accepted; if the new version refuses one, F-30 deletes it as on expiry. Neither the upgrade nor a deploy is stopped to protect them.
2. **Only `internal/ingest` imports it** in production code (TD Code structure, Dependency rules). No `fit`, `proto`, `mesgdef`, `typedef` or `basetype` type crosses the package boundary; `ingest` hands `workout.Recording` (plain Go values) to the pure `workout` package. Exception: the dev-only generator `src/server/tools/corpusgen` uses its encoder to write synthetic corpus files (ADR-004). (rev. 4) For the UP-9 strip only, `ingest` also frames records itself (headers, definitions, data message lengths; ADR-023), because the library could strip only by decoding every message into memory and re-encoding the whole file, which changes far more than the UP-9 fields and closes truncated files. The strip uses the library's `profile/untyped/mesgnum` and `profile/untyped/fieldnum` constants for the field table and `kit/hash/crc16` for CRCs. The saved-location message (global number 29) is not in the library's profile and is our own named constant.
3. **Decode with a custom `decoder.MesgListener`, `WithBroadcastOnly()` and `WithNoComponentExpansion()`.** The listener converts each relevant message into our own compact type inside `OnMesg` and drops the rest. We do not use `filedef.Activity`: it keeps every `mesgdef` message of the file (more memory for 43,000 records), and loses the arrival order between message types.
   - (rev. 2) `ingest` finds the sequence boundaries itself from each header (the data is in memory, ADR-017) and calls `DecodeWithContext` **once per sequence** on a fresh or `Reset` decoder over the bytes from that sequence's start. `Next()`, `CheckIntegrity()` and `Discard()` are not used.
   - (rev. 5) Every decoder call in the sequence walk also gets a record counter, created fresh per call, registered with `WithMesgListener` (next to the existing listener) and with `WithMesgDefListener`. Each hook adds 1 and does nothing else. The walk stores the count of a sequence's last decoder call as `records` (ADR-023 item 2).
   - **The option set** is `WithMesgListener(l, c)`, `WithMesgDefListener(c)`, `WithBroadcastOnly()`, `WithNoComponentExpansion()`, plus `WithIgnoreChecksum()` only (a) to decode a sequence a second time after a header-CRC failure, and (b) for a header with data size 0, which `ingest` replaces with the bytes present (rev. 2; exact rules: F-08 design, section Sequence walk). Nothing else: no `WithLogWriter`, no `Next()`, no `CheckIntegrity()`.
   - (rev. 5) We do **not** copy the decoder's validity rules (`Valid()` or the field-description lookup) into `ingest`. That would be a second decoder that drifts on every upgrade.
4. **Read raw fields and check them against the profile's invalid sentinels ourselves**; do our own conversions (semicircles, scale and offset, enhanced preference) in `ingest`, so the rules in F-02 are visible and tested in our code, not inherited from library helpers.
5. **Fallback, if the corpus finds a file this library cannot decode and upstream cannot fix it in time:** write our own minimal decoder for the messages we read (`file_id`, `activity`, `session`, `lap`, `record`, `event`, and (rev. 3) `sport`) behind the same `ingest` interface. Not `tormoder/fit` (unmaintained), never Garmin's FIT SDK.

### Component expansion off (rev. 1)

**Why.** With expansion on (the default), `decoder.expandComponents` in v0.28.4 (`decoder/decoder.go`, lines 813–835) looks for an existing field with the component's target number and overwrites its value. `record.altitude` and `record.speed` have components that target `enhanced_altitude` and `enhanced_speed`. So when a device writes both fields, the legacy value replaces the enhanced one before our listener sees it. In a probe, raw `altitude=3000` with `enhanced_altitude=4000` decoded as `enhanced_altitude=3000`. The TD rule "prefer `enhanced_*`" (F-02 AC 5) therefore cannot hold at file level with expansion on. `ingest` applies "enhanced if valid, else legacy" itself, for record altitude and speed and for session and lap average and maximum speed. AC 5 is verified at file level.

**Cost.** Fields that are only reachable as components of another field are lost. Among the fields we read, that is `record.compressed_speed_distance`, a field of very old devices. It is not supported in the MVP; distance falls back to positions (F-02 R-6b). If a real file needs it, `ingest` expands field 8 itself (12-bit speed with scale 100, 12-bit accumulated distance with scale 16), and a design entry records the change.

**Second benefit (seen in the F-02 corpus).** With expansion on, the library copied legacy `speed` into `enhanced_speed` through a float round trip that sometimes rounded down by 1 mm/s: 47 of 1,001 values in `indoor-no-gps.fit`, e.g. raw 8001 became 8000. With expansion off, `ingest` reads the raw values, so they are exact.

**Rejected.** Keeping expansion and checking AC 5 only in unit tests: the corpus would then hold a known wrong value for every device that writes both fields, and "prefer enhanced" would only be true in our tests, not in production.

**Upstream.** Overwriting a field the device actually recorded looks like a library defect. Reporting it upstream is optional. Even if upstream fixes it, we keep expansion off until a corpus run shows no loss, and turning it back on would be a new revision of this ADR.

## Consequences

- One third-party Go module in the binary. Its notice goes into the generated attribution list (F-29, TD About page).
- `go.sum` now exists, so the CI cache switch planned in F-01 (section CI, Step 2) applies: `cache: true`, `cache-dependency-path: src/server/go.sum`.
- Library upgrades can silently change output; the corpus golden reports (ADR-004) are the guard.
- Synthetic files are encoded by the same library that decodes them, so a symmetric bug would pass; the real files from three FIT writers (F-02 AC 2) are the guard against that. Since ADR-004 rev. 2 that guard only works locally, on the operator's machine.
- Swapping the library later touches `internal/ingest` only (about the listener and field mapping); `workout`, F-03 and everything downstream are unaffected. This is a reversible decision.

## Would choose differently if

- The corpus shows decode failures on real files from common devices that upstream does not fix: switch to the own minimal decoder (item 5).
- The project became unmaintained and a FIT profile change broke new devices: same.

## Open

- **OQ-5 (user, non-blocking for implementation, must be settled before launch).** The library's README says "The FIT Protocol and FIT file format are proprietary to Garmin and are licensed under the FIT Protocol License" and "Use of this library may require compliance with the FIT Protocol License". The TD states "the FIT format itself is not protected". This is a legal question about the product, not an architecture one; the architecture does not change either way (we do not use or ship SDK files).
