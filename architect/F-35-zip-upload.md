# F-35 Zip upload: architecture

- **Version:** 0.1
- **Status:** draft (8 Nov 2026; normal lane; OQ-108 to OQ-111 open, none blocks the build)
- **Feature:** [`features/F-35-zip-upload.md`](../features/F-35-zip-upload.md)
- **Requirements:** UP-11, UP-1, UP-2, UP-10, NFR Security, NFR Compatibility
- **Builds on:** [F-11 design](F-11-upload-page.md) (U-4 to U-10), [ADR-024](ADR-024-upload-page-plain-typescript-entry.md), [ADR-002](ADR-002-bundle-size-budget-measurement.md), [ADR-021](ADR-021-upload-request-handling-and-rate-limit.md) item 1; new: [ADR-040](ADR-040-zip-archives-read-in-the-browser.md)
- **Owner:** frontend. **Server unchanged** (confirmed): it reads only the bytes of the one `file` part and ignores its file name and type (`src/server/internal/httpapi/upload.go:150-170`).

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| Z-1 | **Module boundary.** New pure module `src/lib/zip/` (with `index.ts`): detection, end record and central directory, root FIT selection, FIT entry checks, inflating under a limit, CRC-32. It imports nothing, takes bytes and a byte limit, returns a result value (FIT bytes, or one refusal reason from a closed set), never throws to its caller and holds no text. Only `src/upload/` imports it. `lib/upload` keeps the 20,000,000 constant and gets the archive sentences (one home for client texts, F-11 U-8) and the new busy state. `src/upload/` does the I/O: reads from the `File`, builds the `File` to send, drives the states | Zip format is a generic subdomain with its own reason to change. Node 24 (`.nvmrc`) has `DecompressionStream('deflate-raw')`, so all of it runs in Vitest without a DOM |
| Z-2 | **Static, not lazy.** `lib/zip` is in the upload entry. F-35 may add up to 2,500 bytes gzip (entry today about 2,275 of 8,000, ADR-024 item 4); above that the engineer reports | A lazy chunk saves about 2 KB per visit but escapes the counted set (ADR-002 item 1) and adds a fetch at drop time that can fail with no message of its own |
| Z-3 | **Detection by the first four bytes only** ([ADR-040](ADR-040-zip-archives-read-in-the-browser.md) item 2): `PK\3\4`, `PK\5\6` (empty archive) or `PK\7\8` (split marker, refused). Anything else, a file under four bytes included, takes the FIT path unchanged: the picked `File` object is sent as it is. Only those four bytes are read before the decision | A FIT file starts with 12 or 14, never `P`. No search for an end record in other files, so a FIT file can never be taken for an archive |
| Z-4 | **Structure.** The archive (at most 20,000,000 bytes, checked from `File.size` first) is read into memory once. End record: searched backwards over the last 22 + 65,535 bytes; the first candidate whose comment length reaches exactly the end of the file wins. Then strictly: central directory inside the file and ending exactly where the end record starts; exactly the declared number of central headers, each with its signature and inside the directory, together filling it exactly | Every offset is bounds-checked before use; a truncated or padded archive fails here |
| Z-5 | **What is trusted** (ADR-040 item 3). The central directory decides name, flags, method, sizes, CRC and local header offset. The local header must have its signature and agree with the central entry on method and name bytes; only its name and extra lengths are used, to find the data. Local sizes, CRC, extra fields and the data descriptor are never read. The FIT entry's data must end at or before the central directory. Overlaps between entries are not checked | Data descriptors (Finder) leave the local sizes zero, so the central sizes are the only ones that always exist. Only one entry is ever inflated, and its output is capped (Z-8), so overlapping entries cannot amplify |
| Z-6 | **Refused as a whole**, before entries are counted: Zip64 (a Zip64 end locator right before the end record, any all-ones count, size or offset in the end record, or `0xFFFFFFFF` in any central entry's sizes or offset) and split archives (a disk number other than 0, entries on this disk ≠ total, or the `PK\7\8` start). A local Zip64 extra alone does not count, since local fields are not read | UP-11. Checking every central entry is free: each is visited for its name anyway |
| Z-7 | **Names** are bytes: never decoded, shown, logged or sent. Root: no `/` and no `\` byte (a backslash is a folder separator in some Windows tools). FIT: last four bytes `.fit`, ASCII case-insensitive. No other special cases (directories, empty names, NUL, invalid UTF-8 just follow the rule). Entries are counted, not names: two headers named `ride.fit` mean "more than one". Order: Z-4 and Z-6, then the count (0 or ≥ 2), then the FIT entry (encryption bit 0 or 6, method other than 0 or 8), then its declared size against the limit, then inflating | One byte test per rule; the encoding question (CP437 or UTF-8) never comes up |
| Z-8 | **Limit on produced bytes** (ADR-040 item 4). Deflate: `DecompressionStream('deflate-raw')` reading the compressed range in small slices pulled one at a time, so one slice's output stays within a few MB (deflate expands at most about 1,032:1). Output is counted per chunk; once it would pass the limit the stream is cancelled and the archive refused. The chunks are kept as they are, and CRC-32 is computed as they arrive. At the end, length and CRC must equal the central entry's. Stored: compressed size must equal uncompressed size; the bytes are a view of the archive. A declared size over the limit is refused before inflating; a declared size is never what stops a bomb. Exactly 20,000,000 is accepted | Peak memory is the archive plus at most 20 MB plus one slice's output, and the main thread yields between slices, so the tab stays responsive. Any stream error (truncated or invalid data) means damaged; trailing bytes after the deflate end are not relied on, since length and CRC decide |
| Z-9 | **Browser support** is checked once a file is detected as an archive, before it is read: `DecompressionStream` exists and constructing it with `'deflate-raw'` does not throw. Otherwise every archive gets the "cannot open zip archives" message, stored ones included. The FIT path never touches it | One predictable rule, testable by removing the global |
| Z-10 | **What is sent** (ADR-040 item 5): the extracted bytes as a new `File` with the fixed name `workout.fit`, through the existing request (F-11 U-7). Extracted bytes that start with a zip signature are refused as damaged (no nesting) | The name is the only archive data that would otherwise leave the browser, can be 65,535 bytes long, and the server leaves 16 KiB for multipart framing (`upload.go:28`). The signature check keeps "no request carries zip bytes" true in one place |
| Z-11 | **Page flow.** The page is busy (picker and drop zone off) from the moment a file is taken, the four-byte read included, until the upload ends, through one new state with its progress text (engineer's wording). The F-11 generation guard covers extraction: a back-forward-cache restore drops a running extraction's result. A read error of the `File` shows UP-10's "could not be read". After any refusal the page is ready with the message. Nothing is stored (F-11 U-10) | Same pattern as the upload request, so no second busy rule |
| Z-12 | **Picker:** still no `accept` attribute (F-11 U-6). It already accepts `.zip` and `.fit`, as any file | An extension list can grey out `.fit` in phone pickers (U-6). OQ-110 |

**Refusals.** Texts are the feature file's; `lib/zip` returns the reason, `lib/upload` holds the sentence.

| Reason | When | Message (feature file) |
| --- | --- | --- |
| archive over 20,000,000 bytes | `File.size`, before any read (unchanged F-11 check) | "larger than 20 MB" |
| no support | Z-9 | "This browser cannot open zip archives…" |
| no FIT | no root FIT entry, empty archive included | "contains no FIT file" |
| several | two or more root FIT entries | "more than one FIT file…" |
| too large | declared or produced size over 20,000,000 | "The FIT file in this archive is larger than 20 MB." |
| damaged | every other failure in Z-4 to Z-8 and Z-10 | "could not be opened…" |
| read error | the browser cannot read the picked file | UP-10 "could not be read" |

## Verification: what must be proven

Archives in committed tests are built by a test helper from synthetic FIT files (Node's `zlib`, dev only), emulating each tool's layout: data descriptor with and without signature, `__MACOSX/` entries, extra fields, stored entries. Archives made by Finder or Explorer are used for the manual AC 45 run only and never committed: they carry machine metadata (extended attributes, owner IDs).

| Claim | Proven by |
| --- | --- |
| Detection never mistakes a FIT file, and a FIT file is sent byte-identical as the original `File` | Node: every synthetic FIT file and short files take the FIT path; jsdom: the sent part is the picked `File` |
| Every structural failure is a refusal, never an exception | Node: truncation at every region (data, directory, end record), comment of 65,535 bytes, a fake end record inside the comment, wrong counts, offsets out of range, bad signatures; plus a mutation run (a few thousand random byte changes of a valid archive) where only result values come out |
| Zip64 and split are refused whatever the entries | Node: each trigger in Z-6 on its own, with an otherwise valid single root FIT |
| Selection follows Z-7 | Node: AC 48 to AC 50 cases, `sub\ride.fit`, directory entries, duplicate root names, an empty archive |
| Only the FIT entry is inflated, and only once | Node: overlapping entries pointing at one local header (with one and with two root FIT names); a spy shows one `DecompressionStream` per archive |
| The output limit holds whatever is declared | Node: exactly 20,000,000 accepted; 20,000,001 refused; a 1 GB bomb declaring a small size refused with output read ≤ limit plus one slice and the stream cancelled; a declared size over the limit refused without inflating |
| Lying sizes and data are damaged | Node: declared length too small and too large, wrong CRC, truncated deflate data, stored entry with unequal sizes, local header disagreeing in name or method |
| Many entries stay fast | Node: 65,535 entries with one root FIT, accepted in well under a second |
| No zip bytes and no archive name ever reach the server | jsdom: the part is named `workout.fit` and holds the FIT bytes; a root `.fit` that is a zip is refused; no request on any refusal |
| Fallback and page states | jsdom: AC 54 with the global removed; AC 55 busy during the four-byte read, extraction and upload, usable after each refusal |
| Budget | `make ci`: upload entry ≤ 8,000, F-35's growth ≤ 2,500, no shared chunk, no new dependency, storage check unchanged |

Manual: AC 45 (tools, iOS picker), AC 58 (wording on screen).

## Open questions

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| OQ-108 | A split archive's later part, or an archive with a prefix (self-extracting), does not start with a zip signature, so it is sent and the server answers "This file is not a FIT file." (Z-3). AC 51's split case then holds only for a part that starts with a signature | Accept; requirements words AC 51 as "a split archive whose part starts with a zip signature" | Wording of AC 51 only |
| OQ-109 | Some macOS tools (`ditto` without `--sequesterRsrc`) put `._ride.fit` next to `ride.fit` in the root; by UP-11 that is "more than one FIT file" | Ignore root names starting with `._` (one byte test in Z-7). Built as UP-11 says until decided | Nothing |
| OQ-110 | Feature scope "the picker accepts `.fit` and `.zip`": no filter (Z-12) or an extension filter? | No filter, as U-6 | Nothing |
| OQ-111 | A root `.fit` entry holding a zip: UP-11 says a non-FIT entry is sent, but also that the server never receives an archive | Refuse as damaged, nothing sent (Z-10) | Nothing |
