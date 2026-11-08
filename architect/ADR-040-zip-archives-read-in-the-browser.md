# ADR-040 Zip archives read in the browser by our own central-directory reader

- **Status:** accepted (8 Nov 2026, with the F-35 design)
- **Date:** 8 Nov 2026
- **Features:** F-35
- **Supersedes:** none. Works within [ADR-024](ADR-024-upload-page-plain-typescript-entry.md) and [ADR-021](ADR-021-upload-request-handling-and-rate-limit.md) item 1
- **Requirements:** UP-11, NFR Security, NFR Compatibility; decision log 7 Nov 2026 (zip upload)

## Context

UP-11 lets the upload page take the one FIT file out of a zip archive. The user decided (decision log, 7 Nov 2026): browser only, the server never receives an archive, detection by content, native `DecompressionStream('deflate-raw')`, no new dependency, both 20,000,000-byte limits, Zip64 and split archives refused. That leaves the reader itself: what it trusts, how it recognises an archive, and what it sends. A hostile archive reaches only the sharer's own tab, but a bad choice here can send zip bytes or archive data to the server, freeze the tab, or refuse ordinary archives.

## Decision

1. **A small reader of our own in `src/lib/zip/`** (F-35 Z-1), on the native `DecompressionStream`. No library: the feature needs a fraction of one, and a dependency would add more code than it saves.
2. **An archive is a file whose first four bytes are a zip signature** (`PK\3\4`, `PK\5\6`, `PK\7\8`). Nothing else is searched, so a FIT file (first byte 12 or 14) can never be read as an archive. Archives with a prefix, and later parts of split archives, are not recognised and go to the server as an ordinary file, which refuses them as not FIT (OQ-108).
3. **The central directory is the only source of entry facts**; the local header must agree on signature, method and name, and is used only to find the data. Local sizes, extra fields and data descriptors are never read. Offsets are bounds-checked; the structure must fill the file exactly (F-35 Z-4, Z-5).
4. **Limits apply to bytes produced, not bytes declared.** Input is inflated in small slices; output is counted and the stream cancelled before it passes 20,000,000 bytes. The result must then match the declared length and CRC-32. Only the one root FIT entry is ever inflated.
5. **Only FIT bytes leave the browser**: sent under the fixed name `workout.fit`, and never when they start with a zip signature.

## Consequences

- A FIT upload is unchanged in bytes, name and code path; the only new cost is a four-byte read.
- Archives that some tools write in non-standard ways are refused (damaged) rather than guessed at: an end record not at the very end, gaps or padding around the central directory, local and central names that differ.
- The reader is about 2 KB gzip in the upload entry; the activity page is not touched (ADR-024 item 3).
- No overlap check between entries is needed, because amplification would need more than one inflated entry.

## Would choose differently if

- A common device export writes Zip64 end records for small archives: read the 64-bit fields (still bounded by the 20 MB archive). That is a product decision, as UP-11 refuses Zip64.
- Prefixed or split archives turn up in practice: search the tail for an end record, accepting that a crafted FIT file could then be refused as a damaged archive.
- More formats (gzip, phase 3 or later) are wanted: `lib/zip` becomes `lib/archive`, detection stays by leading bytes.
