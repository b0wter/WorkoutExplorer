# F-09 Upload API and workout storage

- **Phase:** 1 MVP
- **Owner:** backend
- **Requirements:** UP-1 (server), UP-2 (malformed or crafted input, 21 Oct 2026), UP-7 (default lifetime), UP-9 (identifying data removed), UP-10 (upload messages), AC-1, SH-1, SH-2 (draft state), NFR Security (128-bit IDs, hashed tokens, 20 MB, 60 uploads per hour per IP, uploads processed at once, parse time limit), NFR Privacy (store as little as possible); Technical design: Publish and view flow (Publish 1–3), Data model, Security and privacy (Uploads, Originals)
- **Depends on:** F-03, F-08

## Goal
`POST /api/workouts` turns an uploaded FIT file into a stored draft and returns a viewing link and a secret edit link.

## Scope
- SQLite (WAL) with the `workouts` table as in the technical design; migrations.
- `POST /api/workouts`, multipart; the request carries only the file. 20 MB cap (20,000,000 bytes) enforced while reading.
- Rate limit of 60 uploads per hour per IP address, kept in memory only. Refused uploads (rejected file, too large) count toward the 60. For IPv6, "per IP address" means per /64 prefix.
- Uploads processed at once are capped by a server setting (environment variable, name chosen by the architect), default 4. An upload arriving while the cap is reached is refused at once with the "try again in a minute" message.
- Parse time limit as a server setting (see AC).
- Parse with F-08; on success store `private/{id}/original.fit` and `private/{id}/draft.bin`; write files to a temp path and rename.
- Before `original.fit` is stored, identifying data is taken out of the file (UP-9): every serial-number field and every ANT+ device number or ANT ID anywhere in the file, of the recording device and every sensor, is blanked: at least `file_id` and `device_info` serial numbers, `device_info` ANT device numbers, ANT IDs in sensor profile messages, `ant_channel_id.device_number` and `segment_id.device_id` and the dive-tank `sensor` fields in `tank_update` and `tank_summary`, which hold an ANT channel ID and so a device number (QA-09-r1 Q1); blanked too, because they point to other people: `segment_file.leader_activity_id` and `leader_activity_id_string` (the leaders' activity IDs as number and text), and the segment account keys `segment_id.user_profile_primary_key`, `segment_file.user_profile_primary_key` and `segment_file.leader_group_primary_key` (OQ-52); while the ANT transmission-type fields are kept (OQ-46); the whole `user_profile` message is removed, resting and maximum heart rate included; `location` messages (saved places), segment leaderboard entries and the whole `ant_rx` and `ant_tx` messages (raw radio data that can contain sensor IDs) are removed. Besides these, only the framing may change (OQ-48): CRCs are repaired, or a truncated file is stored marked as unfinished, the FIT way (data size 0, header CRC 0, no file CRC), so it keeps the UP-3 notice and F-30 can rebuild it (OQ-49); trailing bytes that are not FIT data and an incomplete last record are dropped; if the parser stops reading a sequence early at a damaged record (e.g. a definition with an invalid base type), the stored file ends at that record, like any truncated file, and nothing after it is kept (QA-09-r2-1); the workout data and the UP-3 notice are identical. `zones_target` and `time_in_zone` messages are kept as uploaded, including their max, resting and threshold heart rate and threshold power (OQ-50). User-chosen names (sport profiles, devices) and developer fields stay as uploaded. None of the removed data is stored anywhere else (database, logs, `draft.bin`).
- Malformed or crafted input (acceptance rule, 21 Oct 2026): the upload may refuse a file that F-08 accepts, with "This file could not be read.", when the file cannot be stored with UP-9 applied and its workout data unchanged (for example an empty sequence in a chained file, or a compressed timestamp that depends on a removed message, OQ-54). UP-9 is never relaxed to accept a file.
- `public_id` and edit token: 128-bit values from `crypto/rand`, `public_id` in base62; edit token stored as SHA-256 hash only.
- Visibility values (30 Oct 2026, SH-2): only `draft` and `unlisted`; the built schema still allows `private`, to be removed (removal of built behaviour; architect hand-off).
- New workouts: visibility `draft`, `hide_ends` off, `allow_download` off, `expires_at` = upload time + 7 days. Choosing lifetime and download "at upload" (UP-7, SH-8) happens on the draft edit page that opens right after upload (F-19), not in the upload request.
- Response: the viewing link `/w/{public_id}` and the edit link carrying the token in the URL fragment (exact edit-link path defined and documented here).
- Messages for refusals that are not about the file's content (UP-10; file-content messages come from F-08):

  | Case | Message |
  | --- | --- |
  | Larger than 20,000,000 bytes | "This file is larger than 20 MB." |
  | Over 60 uploads per hour | "Too many uploads. Please try again later." |
  | Too many uploads processed at once | "Too many uploads right now. Please try again in a minute." |
  | Upload could not be read (malformed request, connection broken) | "The upload could not be read. Please try again." |
  | Server error | "Something went wrong on the server. Please try again later." |

- Nothing is written under `public/` for a draft.
- Failure at any step leaves no row and no files behind. If the connection drops after the workout is stored, the sharer never gets the links and the draft stays until it expires (accepted, OQ-45).

## Out of scope
- Storage cap (F-10), upload page (F-11), owner endpoints (F-12), publishing (F-15), expiry job (F-21).
- User-chosen names and developer fields: kept as uploaded; revisit when extra sensor streams (Later) are planned.

## Acceptance criteria
- [ ] Uploading a valid ride returns both links; the database row has visibility `draft`, `expires_at` exactly 7 days after `created_at`, and an `edit_token_hash` that is not the token itself.
- [ ] The token appears only in the edit link's fragment, never in a query string, log line or the viewing link.
- [ ] An upload of exactly 20,000,000 bytes is read in full; one of 20,000,001 bytes is refused with "This file is larger than 20 MB.", and the server stops reading early (verified by test).
- [ ] The 61st upload from one IP within an hour is refused with "Too many uploads. Please try again later."; another IP can still upload. Refused uploads (a non-FIT file, a file over 20 MB) count toward the 60. Two IPv6 addresses in the same /64 share one limit; addresses in different /64 prefixes do not.
- [ ] No request metadata is stored in the database or on disk: no IP address, no user agent, no file name.
- [ ] No identifying data is stored. A synthetic file holds: serial numbers in `file_id` and in `device_info` for the device and at least two sensors; ANT+ device numbers with their transmission types in `device_info` and an ANT ID in a profile message; an `ant_channel_id` with a device number; a `segment_id` with a device ID; a `segment_id` and a `segment_file` with user profile primary keys; a `segment_file` with a leader activity ID (number and text) and a leader group primary key; a `tank_update` and a `tank_summary` with a `sensor` value; an `ant_rx` and an `ant_tx` message; a `user_profile` message with name, gender, age, height, weight, resting and maximum heart rate; a `location` message; a segment leaderboard entry; a user-chosen sport name, a developer field, and a `zones_target` and a `time_in_zone` message with max, resting and threshold heart rate and threshold power. In the stored `original.fit` every serial number, ANT device number and ANT ID (including `ant_channel_id.device_number`, `segment_id.device_id` and the tank `sensor` fields), both leader activity ID fields and the three segment primary keys are blank (invalid value) and the transmission types are unchanged; the `user_profile`, `location`, leaderboard entry, `ant_rx` and `ant_tx` messages are gone; the sport name, developer field and both zone messages are unchanged. The stored file passes the F-08 validator, parses to the same workout data as the upload, and differs from it only in these points and its CRCs. None of the removed values appears in the database, `draft.bin` or log lines.
- [ ] Framing only (OQ-48, QA-09-r2-1): a valid file with trailing non-FIT bytes, a truncated file ending in an incomplete record (recovered by F-08), and a file whose reading stops early at a damaged record (e.g. a definition with an invalid base type, with more bytes after it), are stored without those bytes (the third ending at the damaged record, nothing after it kept) and with correct CRCs, or, for a truncated file, marked as unfinished without CRCs (data size 0, header CRC 0, no file CRC); each parses to the same workout data as its upload, and a truncated or cut file shows the same UP-3 notice as its upload.
- [ ] With the cap on uploads processed at once at its default, a fifth upload arriving while four are being processed is refused with "Too many uploads right now. Please try again in a minute."; once one finishes, the next upload is accepted. Setting the cap to another value (e.g. 2) changes the limit after a restart.
- [ ] The parse time limit is a server setting from an environment variable, default 10 s, range 5 to 60 s; an invalid value stops the server at start; a file over the limit gets the UP-2 message for a file too large or too complex to read (F-08).
- [ ] A malformed multipart request, and a body cut off mid-file, are refused with "The upload could not be read. Please try again." and leave no row or files.
- [ ] A server-side failure (e.g. disk write fails, by test) returns "Something went wrong on the server. Please try again later." and leaves no row or files.
- [ ] A file rejected by F-08 returns its plain error message and leaves no row or files.
- [ ] Opening the viewing link of a fresh draft shows no route, title or numbers (generic page, finished in F-15).
- [ ] `public_id` values from 10,000 uploads in a test are unique and 22 base62 characters or fewer.
- [ ] A crafted chained file whose second sequence is complete but holds no readable record (an empty sequence; F-08 accepts the file) is refused with "This file could not be read." and leaves no row or files.

## Decisions (16 Oct 2026, user)
- **Identifying data removed (UP-9):** every serial number and ANT+ device number or ANT ID (device and every sensor, including profile messages) is blanked; the whole `user_profile` message is removed, including resting and maximum heart rate (the app never reads it; AP-10 zones in phase 2 come from zone messages or the sharer); saved locations and segment leaderboard entries are removed. User-chosen names and developer fields are kept for now, to revisit when extra sensor streams are planned. The owner's download (F-12) is the stored file, not a byte-identical copy of the upload.
- **OQ-46:** ANT transmission-type fields are kept; with the device number blank, their 4 bits identify nobody.
- **OQ-47:** no longer applies; the whole user profile is removed.
- **OQ-48:** besides the UP-9 changes, the stored file may differ only in its framing: CRCs repaired, trailing non-FIT bytes and an incomplete last record dropped. The workout data is identical.
- **QA-09-r1 Q1:** "every ANT+ device number of every sensor" is meant literally, not only `device_info` and sensor profiles: `ant_channel_id.device_number` and `segment_id.device_id` are blanked, and `segment_file.leader_activity_id` too (another person's activity ID, like the leaderboard entries); the whole `ant_rx` and `ant_tx` messages are removed (raw radio data that can contain sensor IDs; never used).
- **OQ-52 and the literal reading (confirmed by the user):** the segment account keys (`user_profile_primary_key` in `segment_id` and `segment_file`, `leader_group_primary_key` in `segment_file`) are blanked: they point to people and the app never reads them. Also blanked: `segment_file.leader_activity_id_string` (the leaders' activity IDs as text) and the dive-tank `sensor` fields in `tank_update` and `tank_summary` (an ANT channel ID, so a device number). Full field list: `architect/ADR-023`, item 4.
- **QA-09-r2-1 (option a, cut):** if the parser stops reading a FIT sequence early because of a damaged record (e.g. a definition with an invalid base type), the stored file ends at that record, like any truncated file; nothing after it is kept. The workout and the UP-3 "file was incomplete" notice are unchanged.
- **OQ-49:** a truncated file is stored marked as unfinished, the FIT way: data size 0, header CRC 0, no file CRC. It keeps the UP-3 notice, and F-30 can rebuild it.
- **OQ-50:** `zones_target` and `time_in_zone` are stored as uploaded, including max, resting and threshold heart rate and threshold power. They identify no person or device, and AP-10 names zone messages as its source.
- **Uploads processed at once:** a server setting from an environment variable, default 4 (accepts OQ-44 as a setting).
- **OQ-40, messages:** as in the table above (UP-10).
- **OQ-41:** refused uploads count toward the 60 per hour.
- **OQ-42:** for IPv6, "per IP address" means per /64 prefix.
- **OQ-43:** "20 MB" means 20,000,000 bytes.
- **OQ-45:** if the connection drops after the workout is stored, the draft stays until it expires.
- **"At upload" (UP-7, SH-8):** met by the draft edit page that opens right after upload; the upload request carries only the file.

## Decisions (21 Oct 2026, user)
- **Acceptance rule (UP-2, UP-9):** on malformed or crafted input, the upload may refuse a file that F-08's parser would accept, with "This file could not be read.". Privacy stays strict: UP-9 is never relaxed to accept a file. Replaces "every file F-08 accepts is accepted by the upload". The earlier "files from real devices are always accepted" was removed on 22 Oct 2026: which errors a device may produce cannot be planned for.
- **OQ-54:** a file in which a later record's compressed timestamp depends on a removed message's timestamp is refused, under the acceptance rule. Closed.

## Decisions (22 Oct 2026, user)
- **Parse time limit as a setting:** see the decision log in `features/README.md` (ADR-017 rev. 7, `architect/F-09` D-30).
