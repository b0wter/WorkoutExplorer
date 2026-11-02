# ADR-023 Identifying data stripped by a byte-level rewriter in `ingest`, verified by a second parse

- **Status:** accepted
- **Revision:** 9 (22 Oct 2026). First version 16 Oct 2026. The "(rev. N)" tags mark where a rule was introduced; each rule is stated in its current form
- **History:** [architect/history/ADR-023-identifying-data-stripped-by-a-byte-level-rewriter.md](history/ADR-023-identifying-data-stripped-by-a-byte-level-rewriter.md) (revision list, struck and replaced text)
- **Features:** F-09 (introduces it), F-12 (serves the stored file), F-29 (lists what is removed), F-30 (regenerate re-applies it), F-32
- **Supersedes:** none. Adds to ADR-003 (rev. 4; rev. 5 for rev. 4 of this ADR), ADR-017 (rev. 2; rev. 5), ADR-018 (rev. 2; rev. 3), ADR-004 (rev. 7; rev. 8)
- **Requirements:** UP-9 ((1) blank serial numbers, ANT+ device numbers and ANT IDs, keep transmission types; rev. 3: meant literally, every ANT+ device number of every sensor in any message, raw ANT messages removed, segment device IDs and leaders' activity IDs blanked; rev. 4: segment account keys blanked; (2) remove the user profile; (3) remove saved locations; (4) remove segment leaderboard entries; besides these only repaired framing, and (rev. 4) when reading stops early at a damaged record, the stored file ends at that record; none of it shown, logged or stored), AP-10 (zones never from the user profile), UP-2, UP-3, NFR Privacy (store as little as possible), TD Publish and view flow (Publish 2), TD Security and privacy (Originals), F-09 AC 6 and AC 7; TD invariants I-2, I-5, I-8

## Context

`original.fit` is kept for the workout's lifetime so it can be regenerated (F-30) and downloaded by its owner (SH-8). UP-9 requires the stored file to be the upload with exactly these changes, for every FIT sequence of a chained file. Messages are dropped, so data sizes and CRCs change and the file must be rewritten. What F-08 accepts includes truncated files, unclosed headers (data size 0), CRC failures, chained files with skipped sequences and trailing bytes, and all of them must survive the rewrite with the same workout and the same UP-3 notice.

Options checked against `muktihari/fit` v0.28.4:

| Option | Verdict |
| --- | --- |
| Decode the whole file into `proto.FIT`, edit, re-encode with the library's `encoder` | **Rejected.** Retains every message (500,000 records at the caps), undoing ADR-017's `WithBroadcastOnly` memory bound. The encoder writes its own definitions and headers, so far more than the UP-9 fields changes. It always writes a complete, closed file, which turns a truncated upload into a complete one and loses the UP-3 notice on every regeneration. The saved-location message (global number 29) is not in the library's profile |
| `decoder.RawDecoder` for framing (header, definition, data and CRC chunks as raw bytes) | **Rejected.** Its handling of data size 0 and of truncation is undocumented, and it finds sequence boundaries on its own, so it could disagree with F-08's sequence walk about where a sequence ends |
| **Own minimal record-level rewriter in `ingest`, driven by F-08's sequence walk** | **Chosen.** Copies every byte it does not have to change; about 300 lines of framing; linear time; one output buffer |

## Decision

1. **Where and when.** A new method `(*ingest.Parser).Accept(ctx, upload) (Accepted, error)`, holding one parse slot for the whole flow (ADR-017 rev. 2):
   1. **Parse** the upload as today: limits, deadline, rejection kinds (ADR-018). A rejection ends here with the upload's own outcome and message.
   2. **Reference:** `sha256(workout.ReportJSON(w1))`, with `Integrity.CRCFailed` and `Integrity.IgnoredBytes` set to their zero values first (the rewrite repairs both). Then `w1` is dropped, so two workouts are never in memory at once. Rev. 2: "dropped" means unreachable. No variable in `Accept`'s call chain and no field of its log outcome may still point into `w1` or into the first parse's decoded recording (ADR-017 rev. 3 item 2; `TestAcceptReleasesFirstParse`). The golden report covers every field of the workout and a digest of every stream (ADR-004 item 2), so there is no second equality function to maintain.
   3. **Strip** the upload's bytes (items 3 to 6) into a new buffer.
   4. **Reparse** the stripped bytes (same limits, own decode deadline, no log line).
   5. **Verify:** the reparse is accepted and its reference equals step 2's. Otherwise the upload fails (item 8).
   6. Return `Accepted{Workout: w2, Original: stripped, Stripped: counts}`. `w2`, parsed from the stored bytes, is what `draft.bin` and the database row are made from. So by construction nothing that was removed can reach them, and regeneration from `original.fit` (F-30) gives the same draft.

   `Parse` stays as it is for the CLI, the corpus and F-30's read path.

2. **The sequence walk tells the rewriter what the parser saw.** `walk` records for every sequence its start, header size, data region and status: `complete` (decoded, possibly with a CRC failure or only the CRC missing), `truncated`, or `skipped` (another file type, still complete). The rewriter handles exactly those sequences and drops every byte after the last one (trailing bytes, R-e), so it cannot disagree with the parser about boundaries. (rev. 5) The walk gives where each sequence starts and where its header says the data ends. Where the stored records of a sequence end comes from `records` below, for every status: the decoder loops while its position is before the header's data size and reads each record in full (`decoder.go:1010-1016`), so the last record it accepts can end after that size.

   (rev. 4) It also records **`records`**: how many records (definition messages and data messages, in file order) the library's decoder accepted in that sequence. That tells the rewriter where the parser stopped *inside* a sequence:
   - Counting:
     - A counter is registered on both decoder hooks, `WithMesgListener` and `WithMesgDefListener` (ADR-003 rev. 5).
     - The decoder calls a hook only after a record has been read and accepted in full. So the record it rejects, or cannot finish, is never counted.
     - The counter is created fresh for each decoder call. `records` is the count of the sequence's **last** decoder call (after a header-CRC retry, the retry's).
     - Data messages that item 5 removes are counted, because the decoder read them. `records` counts input records, not output records.
   - Why a count and not a copy of the decoder's checks:
     - The rule does not depend on *why* the decoder stopped. The cause can be end of data, a missing definition, an invalid base type in a definition, a developer field whose description has an invalid base type, or any rule a later library version adds.
     - A copy of `basetype.Valid()` in the framer would cover only the first of those. The developer-field case would need the decoder's field-description lookup re-implemented in the framer. That is exactly the second implementation of the decoder that this item exists to avoid.
     - The decoder exposes no record offset: its byte counter is internal and runs ahead of the record start, and it appears only inside error texts.
3. **Framing inside a sequence.** A record header with bit 7 set is a compressed-timestamp data message for local type `(h >> 5) & 3`. Otherwise bit 6 marks a definition, with bit 5 meaning developer fields, for local type `h & 0x0F`. A definition is: reserved, architecture (rev. 7: **0 little-endian, every other value big-endian**, as the decoder reads it, `decoder.go:550-554`), global message number (2 bytes, in that byte order), field count, 3 bytes per field (number, size, base type), and, with the developer bit, a developer field count and 3 bytes per developer field. A data message's length is 1 plus the sum of its definition's field and developer field sizes; a field of size 0 adds nothing. Definitions are local to their sequence.

3a. **(rev. 7) The strip reads every byte exactly as the pinned decoder does.**
   - Where the decoder accepts something unusual, the strip mirrors it. (rev. 8) Reading a byte differently from the decoder is a defect in every case. `errRecordCount` means exactly that and is always `internal`. The strip may refuse a file that `Parse` accepts only through a deliberate refusal of item 8a. Reading a byte differently can store an identifier, as the rev. 7 input stored a `user_profile`.
   - The normative mapping is the **framing audit table** in the F-09 design (section Strip, "Framing audit"). It has one row per decision the walk, the framer and the fill make from upload bytes: header size, signature, versions, data size, both CRCs, sequence boundaries, record kind, local types, developer flag, missing definitions, reserved byte, architecture, global number, field and developer field definitions, invalid base types, size-0 fields, duplicate fields, the field-description lookup, data length, redefinition, unknown messages and fields, read type, padding, and the decoder state carried across records. Each row cites the decoder's `file:line` and gives a verdict.
   - Every row is the same rule as the decoder's (the one divergence the rev. 7 audit found, the architecture byte, is fixed in item 3). One rule is the same only through an identity: the file CRC of a 14-byte header. The decoder starts it after the header, and the strip computes it over the header and its correct CRC, which leaves the CRC state at 0.
   - A change to any of these decisions edits its row in the same change. A library upgrade re-checks every row's cited lines (ADR-003 item 1).

3b. **(rev. 7) Keeping the two from drifting.**
   - **One place.** Every rule that reads a record byte lives in one pure iterator, `ingest/frame.go`, with one function per audit row. It uses the library's own constants and `proto.LocalMesgNum`, so the expression is the decoder's own wherever the library exports one. The rewriter (output, removal, blanking, CRCs) consumes the iterator and reads no record byte itself. (rev. 8) So does the U2 classifier of item 8a. It is the only other production caller, and it runs only after a reference mismatch.
   - **One check.** A test-only cross-check decodes each span with the library, with a recorder on both hooks (the record stream does not depend on the checksum option), and compares it with the iterator record by record: definition or data, header byte, local type, global number, byte order, field and developer field lists (size 0 left out, as the decoder does), data length, and for every blanked field its offset, size, read type and byte order. It also checks that the decoder's count equals `records`.
   - **Where it runs.** On every corpus file (synthetic in CI, real locally), every committed fuzz seed, every `FuzzAccept` input and its stored result, and the fill matrix. It also runs in a deterministic sweep that changes every byte of a small hand-built probe to every value. A divergence then fails in CI even when it changes nothing visible in the stored file, which is what makes the sweep and the fuzzer sensitive to the whole class.
   - **Why not only a shared helper.** The decoder cannot call our code, so a helper alone would remain a second copy. The cross-check is what ties the copy to the decoder.
   - **Why not drop our framing.** The decoder exposes no record offsets (item 2), and its `RawDecoder` was rejected in Context.
4. **What is blanked** (the field's bytes are overwritten in place, so the length never changes). The table lives in one file, `ingest/strip_table.go`, with the library's `mesgnum` and `fieldnum` constants:

   | Message | Fields blanked | UP-9 |
   | --- | --- | --- |
   | `file_id` (0) | `serial_number` (3) | (1) |
   | `device_info` (23) | `serial_number` (3), `ant_device_number` (21) | (1) |
   | `training_file` (72), `schedule` (28) | `serial_number` (3 and 2) | (1) "every serial number" |
   | `hrm_profile` (4) | `hrm_ant_id` (1) | (1) |
   | `sdm_profile` (5) | `sdm_ant_id` (1) | (1) |
   | `bike_profile` (6) | `bike_spd_ant_id` (4), `bike_cad_ant_id` (5), `bike_spdcad_ant_id` (6), `bike_power_ant_id` (7) | (1) |
   | (rev. 3) `ant_channel_id` (82) | `device_number` (2) | (1) "every ANT+ device number of every sensor" |
   | (rev. 3) `segment_id` (148) | `device_id` (5), the device that created the segment | (1) |
   | (rev. 4, OQ-52) `segment_id` (148) | `user_profile_primary_key` (4), the segment creator's account | (4) |
   | (rev. 3) `segment_file` (151) | `leader_activity_id` (9) and `leader_activity_id_string` (10): the leaders' activity IDs, as number and as text | (1), (4): the same IDs that the removed message 149 carries |
   | (rev. 4, OQ-52) `segment_file` (151) | `user_profile_primary_key` (4) and `leader_group_primary_key` (8): the accounts of the segment's creator and of the leaders' groups | (4) |
   | (rev. 3) `tank_update` (319), `tank_summary` (323) | `sensor` (0): profile type `ant_channel_id` (uint32z), which packs the tank pod's ANT device number | (1) |

   The table has 20 fields in 12 messages. `TestStripTable` compares `blankTable` with it exactly.

   `leader_activity_id_string` and the tank `sensor` fields were not in the user's list of 16 Oct 2026. They carry the same data as fields on it: the leaders' activity IDs as text, and an ANT device number inside a packed channel ID. So they follow the literal reading. The user has confirmed both (rev. 4; feature file, Decisions).

   Kept: ANT transmission-type fields (`device_info.ant_transmission_type`, `ant_channel_id.transmission_type`, `*_ant_id_trans_type` in the profiles; OQ-46, decided by the user); `ant_channel_id.device_type` and every `device_index` (a sensor kind and a slot, not an identifier); user-chosen names (`sport.name`, `bike_profile.name`, `device_info.product_name`, `segment_id.name`, …); developer data and its descriptions. No row names a message of item 5.

4a. **(rev. 3) Identifier rule.** `TestStripTable` walks every field of every message of the pinned profile, outside the messages of item 5. A field is **identifier-like** if its name, or the name of one of its subfields, matches one of these rules, or its profile type is `ant_channel_id`:

   | Rule line | Catches today |
   | --- | --- |
   | contains `serial` | `serial_number` (0, 23, 72, 28) |
   | ends with `device_number` | `ant_device_number` (23), `device_number` (82) |
   | contains `activity_id` | `leader_activity_id`, `leader_activity_id_string` (151) |
   | contains `primary_key` | `user_profile_primary_key` (148, 151), `leader_group_primary_key` (151), all blanked (rev. 4) |
   | ends with `_id`, `_ids`, `uuid` or `identifier`, or is `id` | the profile ANT IDs `*_ant_id` (4, 5, 6), `device_id` (148), `leader_activity_id` (151), and the kept list below |
   | profile type `ant_channel_id` | `sensor` (319, 323) |

   Every identifier-like field must be in exactly one of two tables in `strip_table.go`: the blank table (item 4) or `keptIdentifiers` (message, field, reason). Both directions are checked: an identifier-like field in neither table (or in both) fails, and an entry of either table that is not identifier-like, or not in the profile, fails. So every rule line is needed by some entry, and removing one fails the test. A profile upgrade that adds an identifier therefore fails until someone decides it. `keptIdentifiers` for the pinned profile, exactly:

   | Message | Field | Reason it is kept |
   | --- | --- | --- |
   | `bike_profile` (6) | `id` (14) | the bike profile's slot number on the device |
   | `dive_alarm` (262), `dive_apnea_alarm` (393) | `id` (6) | the alarm's number |
   | `field_description` (206) | `fit_base_type_id` (2), `fit_base_unit_id` (13) | type and unit codes |
   | `developer_data_id` (207) | `developer_id` (0), `application_id` (1), `manufacturer_id` (2) | developer data, kept by UP-9 |
   | `weather_alert` (129) | `report_id` (0) | the weather service's report |
   | `camera_event` (161) | `camera_file_uuid` (2) | a video file on the camera |
   | `segment_id` (148) | `uuid` (1) | the segment (a place), not a device or person |
   | `segment_lap` (142) | `uuid` (65) | as above |
   | `segment_file` (151) | `file_uuid` (1) | as above |
   | `hsa_event` (315) | `event_id` (0) | an event code |
   | `exd_data_field_configuration` (201), `exd_data_concept_configuration` (202) | `field_id` (2) | a data screen field |
   | `device_aux_battery_info` (375) | `battery_identifier` (3) | which battery of the device |

   That is 17 fields in 14 messages. `TestStripTable` compares `keptIdentifiers` with it exactly.

   **OQ-52 (decided by the user, 16 Oct 2026: blank).** Rev. 3 kept three account keys here as "pending": `segment_id.user_profile_primary_key`, `segment_file.user_profile_primary_key` and `segment_file.leader_group_primary_key`. They identify a person's or a group's account on the segment service. Rev. 4 moves them to item 4's table, and they are no longer in this list.

   The test also keeps its rev. 1 checks: the blank table names no field ending in `trans_type` or `transmission_type` (OQ-46) and no message of item 5; every blank-table entry exists in the profile with the number listed here.

   **Invalid value (rev. 6).** The fill follows the type the pinned decoder reads the field as.
   - **What the decoder does** (`decoder.go:699-740`, `proto/value_unmarshal.go`). For a field the profile knows, it takes the base type and the array flag from the profile (`factory.CreateField`) and ignores the declared base type. The declared base type is only checked for validity when the definition is read (`decoder.go:572`); an invalid one ends the sequence before the strip sees it. With n = the profile type's size and s = the declared size:
     - s < n: the s bytes are padded to n with **zero high-order bytes** (little-endian: copied to the front; big-endian: copied to the back), then read as one value.
     - s ≥ n, profile array: every whole element is read; the s mod n bytes after the last one are not read.
     - s > n, not an array: only the first n bytes are read.
     - string: the bytes up to the first 0x00, for each element of an array.
     - (rev. 7) s = 0: the decoder drops the field from the definition and reads no byte for it (`decoder.go:576-579`). The data message has no such field, so it reads as **absent**, whatever the type.
     - For a field the profile does not know, the declared base type is used, with array = s > size and s a multiple of size (`decoder.go:710-720`).
   - **Read type T** = the profile's base type and array flag for a known field, from the factory the parser's decoder uses (`factory.StandardFactory()`; `sequence.go` passes no `WithFactory`), else the declared base type. Every blank-table field is in the pinned profile (`TestStripTable`), so the fallback is never reached today.
   - **Fill**, with n = size(T) and s the declared size, in the definition's byte order:

     | Case | Bytes written | The pinned decoder reads |
     | --- | --- | --- |
     | T is `string` | s bytes 0x00 | empty: absent |
     | s ≥ n | T's invalid value for each whole element (`enum`, `uint8`, `byte` 0xFF; `sint8` 0x7F; `sint16` 0x7FFF; `uint16` 0xFFFF; `sint32` 0x7FFFFFFF; `uint32` 0xFFFFFFFF; `float32`, `float64`, `uint64` all 0xFF; `sint64` 0x7FFF…FF; z types 0), then 0x00 for the s mod n bytes after the last whole element | every element it reads is invalid: absent. The trailing bytes are not read |
     | s < n, T a z type | s bytes 0x00 | 0 after zero padding: absent |
     | (rev. 7) s = 0 | nothing | the field is dropped: absent |
     | 0 < s < n, T not a z type (**short field**) | s bytes 0x00 | **0, a valid value.** No s-byte pattern reads as invalid: the padding is zero, and the invalid value of every non-z type has a non-zero high-order byte |

   - **The short field is filled, not refused.** It holds no byte of the upload, so I-8 holds, and it has a faithful stored form, so item 8a gives no reason to refuse it. Rewriting the definition so the field reads as unknown would change a definition message, which items 3 and 6 keep unchanged (OQ-48 accepted frame changes only). With the pinned profile a short field can occur only in the five `uint32` fields of 148 and 151 (148/4, 148/5, 151/4, 151/8, 151/9), declared with 1 to 3 bytes (a size-0 field is absent, not short). Zeros are the fill because they are what the decoder pads with: the field reads as if it had no bytes.
   - The fill must follow the read type, not the declared one: with the declared type, a `uint32z` field declared `uint8` would get 0xFF and read as 255.
   - `TestBlankFill` (F-09 design) checks every blank-table field against every valid declared base type, a range of declared sizes and the architecture bytes 0, 1, 0x30 and 0xFF, reading the result with the library's decoder; every case also runs the cross-check of item 3b.

5. **What is removed:** every data message with one of these global numbers, whole, with every field and every developer field it carries. The list is the second table in `ingest/strip_table.go`:

   | Global number | Message | UP-9 |
   | --- | --- | --- |
   | 3 | `user_profile` (`mesgnum.UserProfile`): name, gender, age, height, weight, heart-rate settings, `global_id` and every other field | (2) |
   | 29 | saved location or waypoint; not in the public FIT profile, so our own named constant `mesgNumLocation` | (3) |
   | 149 | `segment_leaderboard_entry` | (4) |
   | 80, 81 | (rev. 3) `ant_rx`, `ant_tx` (`mesgnum.AntRx`, `mesgnum.AntTx`): raw ANT radio payloads, which include the sensors' channel IDs and device numbers in bytes no field table can reach | (1) |

   Their definition messages stay: they carry no data. The parser reads none of the five (`listener.go` switches on `file_id`, `record`, `lap`, `session`, `event`, `field_description`, `sport`, `activity` only), so removing them changes no report and no golden. The rewrite does not compensate a removed message's `timestamp` for later compressed-timestamp records; if that would change the workout, step 1.5 catches it (item 8).
6. **Rewriting the sequence frame:**
   - **Complete** sequence: the header keeps its size, protocol and profile version. The data size becomes the new length of the records. A 14-byte header gets a freshly computed header CRC. The file CRC is computed over header and records and written little-endian, using the library's `kit/hash/crc16` (`Sum16()`; its `Sum` is big-endian). An upload whose CRC was wrong or missing is therefore stored repaired. (rev. 5) **The first `records` records are written**, as for a truncated sequence. The framing may read past the header's data size, up to the end of the upload, because the decoder's last record may cross it (item 2). If one of those records cannot be framed, the upload fails closed. Then the framing must end where the decoder's loop ended, else it fails closed (item 8). Let E be the end of the last written record. (a) The last record starts before the header's data end. (b) E reaches that end, or the record at E runs past the end of the upload (the decoder stopped at the end of the bytes and the walk counted the sequence complete with its CRC missing, F-08 R-b). The data size written is the length of the records written. A header data size that ended inside a record is therefore repaired like a wrong CRC (OQ-48), and the stored sequence holds exactly the records the parser read (item 6a). Every byte of the upload's records is unchanged; only the frame and the items 4 and 5 changes differ. (rev. 8) A complete sequence with `records` = 0 is never written. Its closed form would have data size 0, which FIT reads as "unclosed", so the reparse would see a truncated sequence. It is refused before the strip (item 8a, U1).
   - **Truncated** sequence: (rev. 4) **the first `records` records are written, and nothing after them**: not the record the decoder rejected or could not finish, and no byte after it (item 6a). If the framing cannot read one of those first `records` records, the walk and the rewriter disagree, and the upload fails closed (item 8). `records` = 0 writes the header alone. The sequence is written in the **unclosed form** of F-08 R-c (data size 0, header CRC 0, no file CRC), so the reparse sees it as truncated again, with the same `DataEnd` and the same UP-3 notice.
   - **Skipped** sequences are stripped like the others and kept. A settings file chained behind the activity loses its `user_profile` too. Its `file_id` stays, and the skip status comes from `file_id.type` (`listener.go:157-166`), so the reparse still counts it as skipped.

   These frame changes are the only differences from the upload besides items 4 and 5 (OQ-48, accepted by the user; rev. 4: the cut of item 6a, QA-09-r2-1, decided by the user).

6a. **(rev. 4) The stored file holds only records the parser read.** The decoder can stop inside a sequence. It stops at the end of the bytes (a cut-off upload) or at a record it rejects, and in both cases F-08 R-a marks the sequence truncated. The stored sequence then ends at the same record boundary, whatever the bytes after it are. Consequences that follow from it:
   - The stored original never holds data the sharer was not shown: the rewriter never frames past a record that the decoder rejected (QA-09-r2-1).
   - Every stored sequence reads to its end with the pinned decoder. A closed sequence decodes without error. An unclosed one, read the way F-08 R-c reads it, decodes every record and stops only at the missing file CRC. The owner's download (SH-8) is therefore a clean file.
   - `Accept` on a stored original returns the same bytes (F-30 regeneration is idempotent). The walk on the stored file accepts every record in it, so `records` covers all of them, and blanking and removal change nothing a second time.
   - The workout, `data_end_s` and the UP-3 notice are unchanged. The first parse never read the dropped bytes, the reparse reads exactly the kept records, and step 1.5 proves it on every upload.
7. **Bounds.** Linear in the upload's bytes; the output is at most the input plus 2 bytes per sequence (an added CRC). `Accept` checks `ctx` between sequences. Strip errors and messages hold byte offsets and message numbers, never a field value.

   (rev. 5) **Overlap and the new bound.**
   - After a complete sequence whose last record crosses its data size, the walk looks for the next sequence 2 bytes after the declared end (F-08). In a crafted file the next header can sit inside that last record. Then two sequences overlap in the upload.
   - Each sequence is written from its own bytes, so the overlapping bytes are stored twice, once in each role the parser read them in. The reparse then reads what the first parse read.
   - The output is at most the input plus, per sequence, 2 bytes and the part of its last record that lies after its declared end. That part is shorter than one data message: at most 1 + 255 · 255 + 255 · 255 = 130,051 bytes. With 64 sequences (ADR-017) the output exceeds the input by less than 8.4 MB, and only for a crafted file. For a file without an overlap, the growth is at most 2 bytes per sequence, as before.
   - UP-9 applies to messages as the parser reads them. In a crafted overlap, bytes that one sequence reads as a blanked field and the other as a kept field are blanked only in the first role. A device never writes such a file. The uploader would have to build it deliberately, and could equally put the same bytes into a developer string.
   - The reparse therefore applies every ADR-017 limit except `MaxBytes` (ADR-017 rev. 6). `MaxBytes` bounds what a client sends, and this item bounds what the strip makes of it.
8. **Failure is closed.** A framing disagreement, a reparse that is rejected, or a reference mismatch makes `Accept` return `*ingest.Error{Kind: internal}` with a cause starting `strip:`. (rev. 4, rev. 5) Every framing disagreement has the one sentinel `errRecordCount`: fewer than `records` records can be framed, in either status, or a complete sequence's framing fails check (a) or (b) of item 6. Its cause names the sequence's byte offset and what disagreed (the two counts, or where the records end). Exception to "a rejected reparse": a reparse rejected for the time limit returns the upload's `limit`/`time` error, because a time limit depends on load and does not say the file is unreadable (ADR-018 rev. 4). The upload is refused with "This file could not be read." (ADR-018), and one ERROR log line is written. The unstripped upload is never stored, never written to `tmp/`, and never logged. (rev. 8) Exception: the deliberate refusals of item 8a are `unreadable` at WARN. Every other failure in this item is `internal`, which always means a defect on our side.

8a. **(rev. 8) Deliberate refusals (the user's acceptance rule, 21 Oct 2026).**
   - **The rule.**
     - (rev. 9, the user, 22 Oct 2026) **No file is promised acceptance**, from a real device or not: nobody can know in advance what errors a device writes. Upload stores what it can store faithfully and refuses the rest under this item.
     - (rev. 9) **Corpus strictness is a regression check.** Every synthetic corpus file and every local real file must still pass `Accept` in the tests (ADR-004 rev. 10 and 11, F-09 D-29). The built code accepts them, so a refusal of one is a change in our behaviour that must be explained, like any failing test. It is not a product promise about devices.
     - On malformed or crafted input, `Accept` may refuse a file that `Parse` accepts, but only for a cause listed below. The kind is `unreadable` and the message "This file could not be read." (ADR-018 rev. 5). It is never `internal`.
     - Privacy does not bend. Nothing identifying is ever stored (I-8). A case the strip is unsure of is refused, never stored on a guess.
     - A refusal is the right answer only when the file has **no faithful stored form**: no way to write it so that the reparse gives the same workout and the same UP-3 notice. Where a faithful form exists, the strip writes it, as for a short field (item 4) or an overlap (item 7).
   - **The closed list.** A new cause needs a new row here first, with a precise detector and a deterministic test. It is never added to the code alone.

     | # | Cause | Detected | Sentinel | Why no faithful stored form exists |
     | --- | --- | --- | --- | --- |
     | U1 | **Empty complete sequence**: a span with status complete and `records` = 0. The decoder's first record runs past the end of the upload, and the walk counts the sequence complete with only its CRC missing (F-08 R-b). That needs a later sequence: as the only sequence it leaves the file without a record, and `Parse` rejects it as `no_data` | `strip` checks every span before it writes any byte, then refuses (pure predicate `emptyComplete`) | `errEmptySequence` | Written closed, the header would hold data size 0, which FIT reads as unclosed: the reparse would see a truncated sequence and an extra phantom record (the two CRC bytes). Written unclosed, it would be truncated. Leaving it out changes `Integrity.Sequences`. Each of these changes the workout's report |
     | U2 | **Removed timestamp before a compressed-timestamp record** (OQ-54, closed by the user: refused). A data message that item 5 removes has a field 253 of size above 0. A later data message in the same sequence, which is kept, has a compressed-timestamp header. Without the removed message, that record's time moves | Only after a reference mismatch (item 1.5). The pure classifier `removedTimestampDependency(data, spans)` walks the first `records` records of each span with the framing iterator. If it returns true, the refusal is U2; otherwise it stays `internal` | `errRemovedTimestamp` | Keeping the timestamp would mean keeping a removed message (OQ-54 (b), rejected), or rewriting later records |
     - (QA-09-r4-1, 23 Oct 2026) U2 now also covers a removed data message that itself has a compressed-timestamp header: that header moves the decoder's clock too (`decoder.go:641-644`). Still `unreadable`; the set of refused files is unchanged.

   - **What does not change.**
     - The set of files `Accept` refuses changes only by U1, which `Accept` refused before rev. 8 too, as `internal`. U2 only renames an existing refusal.
     - The strip still reads every byte as the decoder does (item 3a). Neither `Parse` nor `walk`, `Integrity` or the reference gets a matching rule: `Parse` keeps accepting both cases (F-08, UP-3), and only the stored form is in question.
   - **Why U1 is detected in `strip` and not in `walk` or `Accept`.**
     - `walk` belongs to `Parse`. Marking the span truncated there would change F-08's `Integrity` for such files, against the user's "Parse need not change".
     - `Accept` alone would leave every test that calls `strip` directly (the sweep, the hand-built cases) without the rule.
     - `strip` is the one production loop (F-09 v1.3). Putting the check there gives both callers the same rule with no second call site.
   - **Why U2 is classified after the mismatch and not detected before the strip.** A detector before the strip would also refuse files where the removed timestamp changes nothing, for example when a normal timestamp follows first. Those are accepted today, and a real device could write one. Classifying only an actual mismatch keeps the refused set exactly as built. The cost is that a strip defect in a file that also has this pattern would be reported as `unreadable`, not `internal`. Nothing is stored either way.
9. **Nothing removed is kept anywhere:**
   - The upload buffer lives only in the request and is never written.
   - The parser never reads the blanked fields or the removed messages: `Device` has no serial number, and the listener ignores those messages.
   - The database and `draft.bin` come from the reparse of the stripped file.
   - Log lines carry only ADR-018's keys, and the upload line carries counts only (`blanked_fields`, `removed_messages`).

   TD invariant I-8 states this. `TestStripIdentifyingData` and the end-to-end test in F-09 enforce it.

## Consequences

- Every accepted upload costs two parses and two reference digests instead of one parse, inside one slot. At the caps on the development machine, `Accept` took 2.3 s against a 4.5 s budget, about four times a plain parse. The two `ReportJSON` calls behind the digests take about as long as the two decodes (ADR-017 rev. 3; synthetic all-caps file).
- Zone messages (`zones_target`, `time_in_zone`) are kept as uploaded, heart-rate and power settings included (F-09 OQ-50, decided 16 Oct 2026, user). Neither message is in item 4 or item 5.
- The stored original differs from the upload in the UP-9 fields and messages and also in its frame: CRCs repaired, trailing bytes and an incomplete last record dropped (OQ-48, accepted), and (rev. 4) everything from a record the decoder rejected onwards (item 6a), and (rev. 5) a header data size that ended inside a record, repaired to cover that record. The workout it parses to is the same; the check proves it on every upload.
- The stored original has no `user_profile`, so no later feature can read heart-rate settings, weight or similar from it, and F-30 regeneration cannot bring them back. AP-10 zones (phase 2) come from the file's zone messages or from the sharer.
- `ingest` now reads FIT framing outside the library, for this one purpose. The library still does all decoding, and only `ingest` touches FIT bytes (I-2 unchanged).
- A library or FIT profile upgrade re-checks the tables: `TestStripTable` applies the identifier rule of item 4a to every field of the pinned profile outside the messages of item 5 and fails on any identifier-like field that is neither blanked nor in `keptIdentifiers`. It also fails if the blank table names a transmission-type field (OQ-46) or a message of item 5 (ADR-003 rev. 4).
- (rev. 3) `ant_rx` and `ant_tx` carry a `timestamp`. A file that has them between records written with compressed timestamps could reparse to a different workout once they are gone; step 1.5 then refuses the upload as `unreadable`, U2 of item 8a (item 8). No synthetic corpus file other than `identifying-data.fit` has these messages; the first "Would choose differently if" point covers it if real uploads are refused for it.
  - (rev. 5) The decoder takes a compressed header's time from the last `timestamp` it read in any message (`decoder.go:641-644`, `742-745`), so the case covers any removed message whose definition has field 253, message 29 included. (rev. 8) **OQ-54, closed by the user (21 Oct 2026): refused.** It is deliberate refusal U2 (item 8a), `unreadable`. `TestAcceptRemovedTimestamp` (F-09) pins it.
- (rev. 8) **Test properties under item 8a.** `TestAcceptDataSizeSweep` checks the strict form, "`Parse` accepts ⇒ `Accept` accepts", because a miscounted data size is damage a real device can produce. `FuzzAccept` and `TestFramingSweep` check the rule of item 8a: accepted with a faithful stored file, or refused as U1 or U2, never `internal`. Load-dependent outcomes (`canceled`, a time limit) are outside both. The cases found in QA round 3 are closed: a data size that ends inside a record (item 6), a last record cut by the end of the upload in a sequence the walk counts complete (item 6 (b)), overlapping sequences (item 7), and a stored file longer than `MaxBytes` (item 7, ADR-017 rev. 6).
- (rev. 6) **Blanked fields with a mismatched definition.** The fill is "absent" as the pinned decoder reads it. Two weaker outcomes are accepted, and neither stores a byte of the upload:
  - A short field (item 4) reads as 0 with the pinned decoder. `checkStored` accepts 0 exactly there: a non-z, non-string blank-table field that (rev. 7) **the data message's own definition, the one active for its local type,** declared shorter than its profile type (F-09 design, `FuzzAccept`).
  - A reader that decodes by the declared base type, as the FIT protocol describes, can show the fill of a mismatched definition as a value (for example 0 for a `uint32z` field declared `uint8`). For a definition that declares the profile's own type at a multiple of its size, every reader sees the field as absent, as before rev. 6.
  - The rev. 6 fuzz input is accepted under item 8a. A stored original written under rev. 5 with such a definition is rewritten the next time F-30 runs `Accept` on it.
- (rev. 7) **Byte-reading divergences.** Fuzz rounds 3, 5 and 6 each found a different place where the strip read upload bytes differently from the decoder: a data size inside a record, the declared versus the profile base type, and the architecture byte. Items 3a and 3b turn this class from something the fuzzer has to stumble on into a deterministic CI check. A library upgrade that changes how the decoder reads any framing byte fails the cross-check on the corpus or in the sweep before it reaches a stored file. The cost is about one extra decode per checked file in tests, and none in production. A stored original written before rev. 7 can only differ for a crafted file with an architecture byte other than 0 or 1, and F-30 rewrites it the next time it runs `Accept`.
- F-30 runs `Accept` on each stored original and rewrites `original.fit` when the result differs, so a later extension of the table also reaches existing workouts.
- (rev. 4) **Library upgrades and the decoded prefix.** The cut (item 6a) ties every stored original to the records the pinned decoder accepted.
  - A later library that **accepts more** (for example, what v0.28.4 rejects) cannot make a stored workout grow: the bytes it would now read are no longer stored. This closes the regeneration hazard QA found in QA-09-r2-1.
  - A later library that **rejects more** would make `Accept` on a stored original stop earlier. (A widened strip table may still shorten an original by removing messages; that is not this case.) Regeneration would then shorten the workout and cut its original further, and those bytes cannot be restored (one-way). (rev. 5, OQ-53 decided by the user, 21 Oct 2026) **The app does not store workouts long term, and this one-way door is accepted.** F-30 keeps the shorter regenerated workout, original included, and a deploy is never stopped to protect stored workouts. (rev. 8, OQ-53 follow-up 1, user) A stored original that the new version refuses is deleted like an expired workout (F-30), but only when the file itself is rejected (`not_fit`, `unsupported_type`, `no_data`, `unreadable`, `limit` other than `time`, `internal`). On `canceled`, a time limit or a server-side error the workout is kept for the next run (F-09 v1.9, F-30 hand-off). F-30 may count such changes for the deploy report.
  - The first guard is earlier, at the upgrade itself. `make corpus` must show no unexplained diff (ADR-003 item 1). The synthetic `ride-30min-bad-basetype.fit` pins where the decoder stops on a rejected definition (ADR-003 rev. 5), and (rev. 5) `ride-30min-size-short.fit` pins that it reads a record that crosses the data size. That guard protects new uploads and the behaviour the corpus shows; it is not a gate for stored workouts.
- (rev. 4) Every decoder call carries a definition listener, so the library copies each definition's field lists once. That is garbage, not retained state, and linear in the bytes (ADR-017 rev. 5).
- **Reversible.** The rewriter is internal to `ingest`; the stored files stay valid FIT for any reader.

## Would choose differently if

- Uploads are refused in practice because of a reference mismatch (U2, OQ-54, closed as "refuse"): reopen OQ-54 with the user, to remove such messages together with their timestamp's effect instead of failing. Its option (b) keeps a timestamp-only shell instead of rewriting later records, because rewriting a compressed header needs a new definition, which would change data messages and not only the frame. Such refusals are not a broken requirement (item 8a, rev. 9).
- (rev. 9) Real uploads are refused in practice (`unreadable` included) often enough to matter to the users: look for a faithful stored form, add the file's pattern to the synthetic corpus, and never relax the corpus strictness to cover it. That is an improvement, not a bug fix: no file is promised acceptance (item 8a).
- The user decides to also remove user-chosen names or developer fields (UP-9's revisit when extra sensor streams are planned): that is a new table row, or for developer fields a new removal rule over `developer_data_id`, `field_description` and the developer bytes of data messages, still in this rewriter.
- `Accept` at the caps exceeds `Timeout` on the production machine (F-31): stream the report into the hash instead of building it as bytes, or digest the workout's streams directly.
- (rev. 4) The library gains an API that reports the byte offset of the record where decoding stopped: use that offset instead of the record count (item 2). The cut rule of item 6a stays the same.
- (rev. 4) Real uploads are refused because of `errRecordCount`: the framing and the decoder disagree about record lengths. Find the difference and fix the framing. Never fall back to "framed as far as they go".
- (rev. 7) A fuzz run, after the cross-check is in place, finds another input in which the strip and the decoder read a byte differently. Then the cross-check or the sweep has a gap. Extend them until they fail on the unfixed code, then fix the code (F-09 v1.8, convergence rule). If that happens twice, the own-framing approach is too fragile: reconsider a decoder-driven rewrite, for example a fork or upstream patch of the library that reports each record's offset and length.
