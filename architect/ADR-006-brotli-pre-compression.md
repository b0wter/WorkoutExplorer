# ADR-006 Brotli pre-compression with `andybalholm/brotli`

- **Status:** accepted
- **Date:** 10 Oct 2026; revision 2 of 11 Oct 2026
- **History:** [architect/history/ADR-006-brotli-pre-compression.md](history/ADR-006-brotli-pre-compression.md)
- **Features:** F-03 (introduces it), F-15 (writes `data.{hash}.bin.br` under `public/`), F-22 and F-31 (Caddy serves it), F-30 (regenerate)
- **Supersedes:** none
- **Requirements:** NFR Performance (data file under 500 KB compressed for a 4-hour ride); TD Workout data file ("Caddy serves a pre-compressed `data.bin.br` written at publish time"), TD Tech stack (Caddy: brotli and zstd), TD Conventions (Binary: `CGO_ENABLED=0`) and TD Code structure (Dependency rules: standard library first)

## Context

The TD requires a pre-compressed `data.bin.br` next to every data file. Go's standard library has gzip and zlib but no brotli. Caddy's `encode` directive compresses on the fly with gzip and zstd only, so a brotli file has to be written ahead of time and served with `file_server { precompressed br }` (F-31). The binary is static and built without cgo, which rules out the reference C library through cgo.

Facts checked on 10 Oct 2026 (pkg.go.dev, GitHub tags, `go.mod` at the tag):

| Item | `github.com/andybalholm/brotli` |
| --- | --- |
| What | Pure-Go port of Google's brotli encoder and decoder (translated from the C reference), plus its own `matchfinder` package |
| Licence | MIT |
| Status | Active: v1.2.6 tagged 30 Sep 2026; v1.2.3 to v1.2.6 released Aug to Sep 2026 |
| Dependencies | `go.mod` requires only `github.com/xyproto/randomstring`, used by its tests; nothing is compiled into importers except the module's own packages |
| API | `NewWriterOptions(w, WriterOptions{Quality: 0–11, LGWin: 10–24})`, `NewReader(r)` |

## Decision

1. **Use `github.com/andybalholm/brotli`, pinned at `v1.2.6`** (exact version in `go.mod`, `go.sum` committed). An upgrade is a deliberate change that must pass the round-trip tests.
2. **Only `internal/precompress` imports it** in production code. That package exposes one function, `Brotli(src []byte) ([]byte, error)`, and knows nothing about workouts. Tests may use the library's reader to decompress.
3. **Parameters: quality 9 (rev. 1), window `LGWin` 22 (4 MiB).** See section Why quality 9 for the measurements. A 4 MiB window covers the whole of a 12-hour data file (about 1.2 MB), and browsers decode it without trouble (they accept windows up to 16 MiB).
4. **The `.br` file is a transport encoding, not a format.** It must decompress to exactly the bytes of `data.bin`. Browsers decompress it transparently through `Content-Encoding: br`, so the F-04 decoder never sees brotli. Every `.br` file that is written has a `.bin` file with the same bytes next to it, for clients that do not send `Accept-Encoding: br`.
5. **Measured, not assumed.** `BenchmarkBrotli12h` records the compression time of the 12-hour data file in `testdata/corpus/MEASUREMENTS.md`.
6. **One compression at a time per process (rev. 2; decided in F-03 design v1.2).** `precompress.Brotli` holds a package-level mutex for the whole compression. About 52 MB is allocated per 12-hour compression at quality 9, so concurrent publishes, uploads and regeneration could otherwise add up on a small VPS. Callers (F-09, F-15, F-30) need no locking of their own. `Brotli` takes no `context`, so a caller waiting on the mutex cannot give up; at about 120 ms worst case this is acceptable, but F-15 and F-30 keep compression out of any request path that has a deadline. Covered by `TestBrotliOneAtATime`.

## Consequences

- One new third-party module, with a justification in the F-03 design as TD Code structure (Dependency rules) requires. The dependency check for QA becomes `workout-explorer/...`, `github.com/muktihari/fit/...` and `github.com/andybalholm/brotli/...`.
- (rev. 1) Quality 9 compresses a 12-hour file in about 0.13 s. F-09 decides whether `draft.bin` also gets a `.br`. F-03 provides the function either way.
- The compressed bytes depend on the library version. Tests therefore compare decompressed bytes, never `.br` bytes. No golden `.br` file is committed.

## Would choose differently if

- (rev. 1) **Size:** `ride-4h.fit` as `.br` reaches 250,000 bytes (half the budget), or a real 4-hour file in a QA report does. Then try quality 10. **Cost:** `BenchmarkBrotli12h` exceeds 1 s/op or 64 MB/op. The **reference machine** is the one named in the latest benchmark row of `MEASUREMENTS.md` (currently an i7-4770T). If the machine changes, re-measure all three qualities on it. Changing the quality is a constant and needs no new ADR.
- Real 4-hour files land under 250 KB even with gzip. Then the standard library's gzip plus Caddy's on-the-fly zstd would remove the dependency. That would need a new ADR, because the TD names brotli.
- The library becomes unmaintained or shows a correctness bug. Then use another pure-Go brotli encoder behind the same `precompress.Brotli` function. Callers do not change.
- (rev. 2) Waiting on the mutex shows up in a path with a deadline, or compressions queue during regeneration. Then add a `context` to `Brotli` or a small bounded worker, inside `precompress`.

## Why quality 9 (rev. 1)

Measured during F-03 implementation (i7-4770T, `andybalholm/brotli` v1.2.6, window 22):

| Quality | `ride-4h` `.br` | Time | `ride-12h` `.br` | Time |
| --- | --- | --- | --- | --- |
| 11 | 93,668 | 1.10 s | 212,617 | 3.43 s (154 MB allocated) |
| 10 | 93,279 | 0.55 s | 220,248 | 1.32 s |
| 9 | 106,951 | 0.06 s | 266,150 | 0.13 s |

**Decision: quality 9.** It is about 25 times faster than quality 11. Quality 10 and 11 use the expensive high-quality path; quality 9 does not, so it also avoids the 154 MB per compression. Memory on a small VPS during publish and on-deploy regeneration matters more than the bytes saved. The cost is about 13 KB more per 4-hour view (+14 %), and the result is still under a quarter of the 500 KB budget. On 4G that is well under 0.1 s, and the data file is not on the LCP path. The window stays at 22.
