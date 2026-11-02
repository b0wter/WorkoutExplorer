# F-03 Workout data file writer

- **Phase:** 0 Prototype
- **Owner:** backend
- **Requirements:** NFR Performance (data file under 500 KB compressed for a 4-hour ride), AP-4; Technical design: Workout data file
- **Depends on:** F-02

## Goal
Write the normalised workout as `data.bin` plus a pre-compressed `.br`, in the binary layout the browser reads without parsing, and prove it meets the size budget.

## Scope
- Layout as in the technical design: magic `WEXD`, format version (uint16), sample count (uint32); length-prefixed JSON metadata (sport, start time and time zone, summary, laps, sessions, one descriptor per stream with name, type, scale, unit, byte offset); one column per present stream.
- Little-endian; every column starts on a 4-byte boundary.
- Stream types, resolutions and delta/raw encoding per the technical design table; missing samples use the type's maximum as sentinel; absent streams are not written.
- Brotli pre-compression to `data.bin.br`.
- Laps and sessions reference the streams by sample index or time offset, so the browser can place lap markers and select lap ranges.
- A precise written description of the format (header, metadata JSON schema, descriptor fields) for the front end. Any later change bumps the version.
- Extend the diagnostic CLI subcommand (F-02) to write `data.bin` and `data.bin.br` for a FIT file; `make corpus` reports the compressed size per file.

## Out of scope
- Content-hashed file names and the `public/` folder (F-15); hiding (F-16).
- Reading the file in the browser (F-04).

## Acceptance criteria
- [ ] Every corpus file produces a `data.bin` whose header reads `WEXD`, the current version and the correct sample count.
- [ ] Every column offset is divisible by 4.
- [ ] Round-trip test: decoding a written file in a Go test reproduces every stream value within its resolution (e.g. latitude within 10⁻⁶°), with sentinels where samples are missing.
- [ ] An indoor file has no latitude/longitude columns and no descriptors for them.
- [ ] A 4-hour ride with every stream is under 500 KB as `.br`; the measured sizes for the corpus are recorded.
- [ ] The format description exists in the repository and matches the writer (checked by the F-04 decoder tests using files from this writer).
