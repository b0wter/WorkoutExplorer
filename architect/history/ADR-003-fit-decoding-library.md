# ADR-003 FIT decoding library: history

History of [architect/ADR-003-fit-decoding-library.md](../ADR-003-fit-decoding-library.md). The live ADR states every decision once, in its current form, and keeps the "rev. N" labels that code and other documents cite. This file holds, verbatim, the header's revision list and the Revision 1 to 6 sections as they stood on 22 Oct 2026, including the superseded rule of Revision 5. Read it only when you need the history.

## Header revision list (as of revision 6)

- **Date:** 10 Oct 2026 (revision 1, 10 Oct 2026: decision item 3 now turns component expansion off. Found during F-02 implementation; see Revision 1. Revision 2, 15 Oct 2026, F-08: one decoder call per FIT sequence, `WithIgnoreChecksum()` only for a CRC retry or an unclosed header, and an upgrade check of the decoder's growth paths; see Revision 2. Revision 3, 15 Oct 2026, F-08 design v1.1: the `sport` message and `lap.sport` are read (sport hints), item 5 lists `sport`. Revision 4, 16 Oct 2026, F-09: `ingest` also frames FIT records itself to strip identifying data (ADR-023), using the library's profile constants and CRC; see Revision 4. Revision 5, 21 Oct 2026, F-09 v1.5 (QA-09-r2-1): every decoder call also registers a record counter on both hooks, so the strip can cut where the decoder stopped; the upgrade check covers where the decoder stops; see Revision 5. Revision 6, 21 Oct 2026, F-09 v1.6: the decoder reads a record that crosses the header's data size, and the strip relies on it (QA-09-r3-1); OQ-53 decided by the user, so a later version that stops earlier is not a gate for stored workouts; see Revision 6)

## Decision item 3 as written (rev. 2 wording)

3. **Decode with a custom `decoder.MesgListener`, `WithBroadcastOnly()` and `WithNoComponentExpansion()`** (expansion off since revision 1; one call per FIT sequence and `WithIgnoreChecksum()` only for CRC retries and unclosed headers since revision 2). The listener converts each relevant message into our own compact type inside `OnMesg` and drops the rest. We do not use `filedef.Activity`: it keeps every `mesgdef` message of the file (more memory for 43,000 records), and loses the arrival order between message types.

## Revision 1: component expansion off

**Finding (F-02 implementation, confirmed in the library source).** With expansion on (the default), `decoder.expandComponents` in v0.28.4 (`decoder/decoder.go`, lines 813–835) looks for an existing field with the component's target number and overwrites its value. `record.altitude` and `record.speed` have components that target `enhanced_altitude` and `enhanced_speed`. So when a device writes both fields, the legacy value replaces the enhanced one before our listener sees it. In a probe, raw `altitude=3000` with `enhanced_altitude=4000` decoded as `enhanced_altitude=3000`. The TD rule "prefer `enhanced_*`" (F-02 AC 5; AC 4 before the feature file was renumbered on 10 Oct 2026) therefore cannot hold at file level with expansion on.

**Decision.** Turn expansion off. `ingest` applies "enhanced if valid, else legacy" itself, for record altitude and speed and for session and lap average and maximum speed. AC 5 stays as written and is verified at file level.

**Cost.** Fields that are only reachable as components of another field are lost. Among the fields we read, that is `record.compressed_speed_distance`, a field of very old devices. It is not supported in the MVP; distance falls back to positions (F-02 R-6b). If a real file needs it, `ingest` expands field 8 itself (12-bit speed with scale 100, 12-bit accumulated distance with scale 16), and a design entry records the change.

**Second benefit (seen in the F-02 corpus).** With expansion on, the library copied legacy `speed` into `enhanced_speed` through a float round trip that sometimes rounded down by 1 mm/s: 47 of 1,001 values in `indoor-no-gps.fit`, e.g. raw 8001 became 8000. With expansion off, `ingest` reads the raw values, so they are exact.

**Rejected.** Keeping expansion and checking AC 5 only in unit tests: the corpus would then hold a known wrong value for every device that writes both fields, and "prefer enhanced" would only be true in our tests, not in production.

**Upstream.** Overwriting a field the device actually recorded looks like a library defect. Reporting it upstream is optional. Even if upstream fixes it, we keep expansion off until a corpus run shows no loss, and turning it back on would be a new revision of this ADR.

## Revision 2: F-08 decoding of broken and chained files

**Findings (v0.28.4 source, `decoder/decoder.go`).** Messages reach the listener as soon as each one is complete, and the file CRC is checked only after the last message. So a truncated file or one with a wrong file CRC has already delivered all its intact messages when the error comes. A wrong header CRC is reported before any message. Errors are sticky: after one, `Next()` and `Decode` return the same error, so a chained file cannot be continued past a broken sequence through the decoder. A header with data size 0 is refused as `ErrNotFITFile`, although unclosed device files have exactly that.

**Decision (adds to item 3).**

- `ingest` finds the sequence boundaries itself from each header (the data is in memory, ADR-017) and calls `DecodeWithContext` **once per sequence** on a fresh or `Reset` decoder over the bytes from that sequence's start. `Next()`, `CheckIntegrity()` and `Discard()` are not used.
- The option set stays `WithMesgListener`, `WithBroadcastOnly()`, `WithNoComponentExpansion()`. `WithIgnoreChecksum()` is added only (a) to decode a sequence a second time after a header-CRC failure, and (b) for a header with data size 0, which `ingest` replaces with the bytes present. Exact rules: F-08 design, section Sequence walk.
- **Upgrade check (adds to item 1).** Besides `make corpus`, a library upgrade re-checks the growth paths listed in ADR-017 (retained state, recursion with expansion off, goroutines, super-linear loops) and whether decoder error texts still contain no field values (ADR-018). The result goes into the upgrade hand-off.

## Revision 4: record framing for the UP-9 strip (F-09)

**Finding.** UP-9 needs the stored original rewritten with a few fields blanked and three message kinds removed (`user_profile` added by ADR-023 rev. 1). The library can only do this by decoding every message into memory and re-encoding the whole file, which changes far more than the UP-9 fields and closes truncated files (ADR-023, Context).

**Decision (adds to items 2 and 3).**
- `ingest` frames records itself (headers, definitions, data message lengths) in `strip.go`, for the strip only. Decoding for the workout still goes through the decoder exactly as items 3 and Revision 2 say.
- The strip uses the library's `profile/untyped/mesgnum` and `profile/untyped/fieldnum` constants for the field table and `kit/hash/crc16` for CRCs. The saved-location message (global number 29) is not in the library's profile and is our own named constant.
- **Upgrade check (adds to item 1 and Revision 2):** a test enumerates every field of the pinned profile and applies the identifier rule of ADR-023 item 4a (amended 16 Oct 2026, ADR-023 rev. 3). It fails if an identifier-like field is neither in the blank table nor in `keptIdentifiers`. An upgrade that adds such a field therefore fails until someone decides it.

## Revision 5: a record counter on every decoder call (F-09 v1.5)

**Finding (QA-09-r2-1).** The v0.28.4 decoder rejects some records that the strip's own framing can still read. Two examples are a definition whose base type fails `basetype.BaseType.Valid()` (`decoder.go:572-575`) and a data message whose developer field points to a field description with an invalid `fit_base_type_id` (`decoder.go:891-893`). F-08 R-a then treats the sequence as truncated. The strip, however, went on framing and stored every record after the rejected one. The user decided that the stored file ends at the rejected record (ADR-023 rev. 4 item 6a).

**Decision (amends item 3 and Revision 2's option set).**
- Every decoder call in the sequence walk gets one more listener: a record counter, created fresh per call. It is registered with `WithMesgListener` (next to the existing listener) and with `WithMesgDefListener`. Each hook adds 1 and does nothing else.
- The option set is therefore `WithMesgListener(l, c)`, `WithMesgDefListener(c)`, `WithBroadcastOnly()`, `WithNoComponentExpansion()`, plus `WithIgnoreChecksum()` for the two Revision 2 cases. Nothing else is added: still no `WithLogWriter`, no `Next()`, no `CheckIntegrity()`.
- The walk stores the count of a sequence's last decoder call as `records` (ADR-023 rev. 4 item 2).
- We do **not** copy the decoder's validity rules (`Valid()` or the field-description lookup) into `ingest`. That would be a second decoder that drifts on every upgrade.

**Upgrade check (adds to item 1, Revisions 2 and 4).** Where the decoder stops is now part of what is stored, so an upgrade also checks it:
- `make corpus` must show no diff in `ride-30min-bad-basetype.fit` (a definition with an invalid base type, then more records; F-09 design, section Corpus). A diff there means the new version stops at a different record than v0.28.4. Any other changed `integrity.truncated` or `data_end` in a golden means the same thing.
- ~~If that happens, the upgrade stops until the operator decides. Workouts already stored keep the old cut (ADR-023 rev. 4, Consequences: F-30 rule).~~ (superseded by Revision 6)
- The upgrade check also confirms that the decoder still calls `OnMesgDef` only after a whole definition has been read and accepted, and `OnMesg` only after a whole data message.

## Revision 6: records that cross the data size; stored workouts after an upgrade (F-09 v1.6)

**Finding (QA-09-r3-1).** v0.28.4 decodes records `for d.cur < DataSize` and reads each record in full (`decoder.go:1010-1016`). A record that starts before the header's data size and ends after it is accepted and counted, and the file CRC is then read after it. F-08 counts such a sequence complete. The strip now stores exactly the records the decoder read and repairs the data size (ADR-023 rev. 5 item 6).

**Decision (amends Revision 5's upgrade check).**
- The upgrade check also covers this behaviour: `make corpus` must show no diff in `ride-30min-size-short.fit` (header data size ends inside the last record; F-09 design v1.6, section Corpus). A diff means the new version stops before that record or treats it differently.
- **Stored workouts are not a gate (OQ-53, decided by the user, 21 Oct 2026).** A diff in `ride-30min-bad-basetype.fit`, `ride-30min-size-short.fit` or any golden's `integrity.truncated` or `data_end` is explained in the upgrade hand-off like every other diff (item 1). It is judged by what it does to new uploads. Workouts already stored are regenerated with the new version by F-30. If they come out shorter, that is accepted; if the new version refuses one, F-30 deletes it as on expiry. Neither the upgrade nor a deploy is stopped to protect them.
- The rest of Revision 5 stands: no copy of the decoder's validity rules, and the hooks are called only after a whole record.
