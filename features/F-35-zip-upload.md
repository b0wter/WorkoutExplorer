# F-35 Zip upload

- **Phase:** 2 (first in phase 2, before F-34)
- **Owner:** frontend
- **Requirements:** UP-11 (Should within phase 2), UP-1, UP-2, UP-10, NFR Security, NFR Compatibility; extends F-11
- **Depends on:** F-11

## Goal
A sharer can drop or pick a ZIP archive that holds one FIT file (for example a device website's "export original" download) without unzipping it first; the browser takes the FIT file out and uploads it as if it had been picked.

## Scope
- The upload page opens the archive in the browser. The server never receives an archive and is unchanged: it gets one FIT file through the existing upload request (`POST /api/workouts`). The upload API, and so the iOS Shortcut (F-34), stay FIT only.
- A file is treated as a zip archive by its content (zip signature), whatever its name, so a zip renamed to `.fit` is never sent either. A FIT file is never mistaken for one (its first byte is its header size, 12 or 14).
- Works for drag and drop and the file picker; the picker has no file-type filter, so it accepts `.fit` and `.zip` (a filter can grey out `.fit` in phone pickers; decision log 9 Nov 2026). The existing rules hold: only the first of several dropped files is used; the drop zone and picker are disabled from the moment a file is taken until the upload ends, extraction included.
- Accepted only when the archive's root holds exactly one entry whose name ends in `.fit` (case-insensitive), ignoring root names that start with `._` (macOS metadata). Root means the name has no folder part (no `/`). FIT files in folders don't count (this also skips macOS `__MACOSX/` copies); every other entry is ignored, never decompressed and never sent.
- Compression methods stored (0) and deflate (8). Archives made by standard tools are accepted, including entries whose sizes are written after the data (data descriptor). Encryption and compression method are checked on the FIT entry only; other entries are not checked, whatever their method or encryption.
- Refused with a message, nothing sent: no root FIT file; two or more root FIT files; an encrypted FIT entry or one with another compression method; refused as a whole, whatever their entries: a split or multi-disk archive (its first part; later parts and self-extracting archives don't start with a zip signature, are sent and refused by the server as not a FIT file), a Zip64 archive; a damaged archive (unreadable directory, sizes that don't match, CRC-32 of the extracted file wrong).
- Size limits, both the existing upload limit of 20,000,000 bytes (UP-1, NFR Security): the archive itself, checked before it is read, with today's message; and the extracted FIT file, enforced while decompressing: decompression stops as soon as the output would exceed 20,000,000 bytes, whatever size the archive declares, and the archive is refused. Exactly 20,000,000 bytes is accepted.
- The extracted file then goes through the same client-side checks and the same upload request as a picked `.fit` file; every later message (UP-2, UP-10) is unchanged. A root `.fit` entry that is not a FIT file is sent and refused by the server as today, except one that is itself a zip archive: refused as damaged, nothing sent.
- Messages (wording as below; the user reviews it on screen as with OQ-58, decision log 7 Nov 2026):
  - Archive larger than 20,000,000 bytes: "This file is larger than 20 MB." (unchanged)
  - No root FIT file: "This archive contains no FIT file."
  - Two or more: "This archive contains more than one FIT file. Please upload one at a time."
  - Extracted file over the limit: "The FIT file in this archive is larger than 20 MB."
  - Unsupported or damaged: "This archive could not be opened. Please unzip it and upload the FIT file."
  - Browser without `DecompressionStream('deflate-raw')`: "This browser cannot open ZIP archives. Please unzip it and upload the FIT file." FIT upload is unaffected.
- While extracting, the page shows a short progress text (engineer proposes, e.g. "Opening the archive…"). The drop zone and intro mention ZIP archives: intro "…or a ZIP archive holding one…", drop zone "Drop a FIT file or ZIP archive here".
- After a refusal the drop zone is usable again.
- Nothing from the archive is stored in the browser (F-11 AC 8 holds), logged or sent besides the extracted FIT file.

## Out of scope
- Server changes of any kind; zip support in the upload API or the iOS Shortcut (F-34).
- Several FIT files from one archive; FIT files in folders; nested archives (a zip inside counts as another file and is ignored).
- Other archive formats: gzip (`.fit.gz`), tar, 7z, rar.
- Sharing a file from another app to the browser on Android or iOS (share target, PWA: Later).

## Constraints (for the architect)
- Native `DecompressionStream('deflate-raw')`; no new dependency.
- The upload page is plain TypeScript (ADR-024) with its own entry and bundle budget (60,000 bytes gzip per entry, ADR-002); zip reading stays inside it and shares no chunk with the activity page.
- Read sizes and offsets from the central directory, never trust them for the output limit: the limit is enforced on the bytes actually produced.
- Browsers: NFR Compatibility (last two versions of Chrome, Safari, Firefox, Edge; iOS Safari, Android Chrome); all of them have `deflate-raw` today, the fallback message covers older ones.
- Test archives are synthetic, built from the synthetic FIT corpus; real files and real device exports are never committed.

## Acceptance criteria
- [ ] An archive with one root FIT file, made by each of: `zip` on Linux (deflate), macOS Finder "Compress" (data descriptor, `__MACOSX/` entries), Windows Explorer "Send to compressed folder", and `zip -0` (stored), opens the new draft's edit link, by drag and drop on desktop and by the picker on iOS Safari (Android Chrome checked with the phase 2 checks).
- [ ] The server receives a FIT file byte-identical to the archived one (request inspected); no request carries zip bytes.
- [ ] A zip renamed to `workout.fit` is extracted, not sent as is; a `.fit` file is uploaded exactly as before.
- [ ] Root `.FIT` (upper case) is accepted; a root `notes.txt`, a `readme.pdf` and a `.fit` in `sub/` next to one root FIT file are ignored; so is a root `._ride.fit` next to `ride.fit`.
- [ ] A later part of a split archive and a self-extracting archive are sent and refused by the server with its "not a FIT file" message.
- [ ] No root FIT file (also: an empty archive, or FIT files only in folders) shows "This archive contains no FIT file."; two root FIT files show the "more than one" message. No request is sent.
- [ ] An archive with one plain root FIT file next to an encrypted `notes.txt` and a bzip2 `readme.pdf` is accepted.
- [ ] An encrypted FIT entry, a FIT entry with another method (e.g. bzip2), the first part of a split archive, a root `.fit` entry that is itself a zip archive, a Zip64 archive, a truncated archive and an archive whose FIT entry fails its CRC-32 each show the "could not be opened" message; no request is sent.
- [ ] An archive of 20,000,001 bytes is refused with "This file is larger than 20 MB." before it is read.
- [ ] A FIT entry that inflates to exactly 20,000,000 bytes is sent; one that inflates to 20,000,001 bytes, and a crafted entry declaring a small size that inflates to 1 GB, are refused with the extracted-size message; decompression stops after at most 20,000,000 bytes of output and the tab stays responsive.
- [ ] With `DecompressionStream` removed (test stub), a ZIP archive shows the "This browser cannot open ZIP archives" message and a `.fit` file still uploads.
- [ ] During extraction and upload the drop zone and picker are disabled; after any refusal both are usable again.
- [ ] The upload page entry stays within its bundle budget (`make ci` size check); no new dependency in `package.json`.
- [ ] The page sets no cookies and stores nothing in browser storage.
- [ ] The user has reviewed every message on screen (user check).
