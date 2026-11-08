# ADR-033 Preview image rendered in Go with `golang.org/x/image` and the embedded Go fonts

- **Status:** accepted (31 Oct 2026), with the F-23 design v0.1
- **Date:** 31 Oct 2026
- **Features:** F-23 (introduces it); F-30 (rebuilds it), F-29 (attribution)
- **Note (9 Nov 2026, F-36, ADR-041):** a second variant draws the same card over a basemap rendered in the sharer's browser, as a paletted PNG from our own quantizer (standard library only). The tiles never reach the server from the tile host, so I-6 and "no external services" hold; the second "Would choose differently if" bullet is settled that way
- **Supersedes:** none
- **Depends on:** [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md) (deterministic artefacts), [ADR-005](ADR-005-workout-data-file-format-v1.md) (positions in the public data file)
- **Requirements:** SH-5, NFR Privacy (no external services); TD Tech stack (Preview images: rendered in Go, no headless browser)

## Context

SH-5 needs a PNG with an anti-aliased route and text (name, values). The standard library encodes PNG and fills rectangles, but has no anti-aliased path filling and no font rasteriser. The server must not call an external service or run a headless browser (TD), and the binary is pure Go (`modernc.org/sqlite`, no cgo). Workout names are user text in any script.

## Decision

1. **One new module: `golang.org/x/image`**, pinned in `go.mod`, used only in `internal/preview`: `vector` for the route stroke, `font/opentype` and `font` for text, `font/gofont` (Go Regular and Go Bold) embedded as the only fonts. PNG through the standard `image/png`, default compression (best measured within 1.5 % in size and about 3× slower), no text or time chunks.
2. **No other rendering library** (no `gg`, `freetype`, `canvas`, no cgo, no system fonts, no font file read at run time).
3. **Determinism:** the same input gives the same bytes within one binary. A Go or `x/image` upgrade that changes bytes only renames the file at the next build (F-30 rebuilds all), which is accepted.
4. **Glyph coverage is the Go fonts'** (Latin, Greek, Cyrillic, common symbols including "…"). Other characters follow F-23 OQ-92.

## Consequences

- Go project sub-repository, BSD-3, pure Go: no cgo, no new licence family; the binary grows by about 1.2 MB (measured; fonts plus `sfnt` and `opentype` code). Transitive: `golang.org/x/text` (BSD-3, indirect via `sfnt`) and a `golang.org/x/sys` version bump.
- Names in other scripts show reduced in the image; `og:title` still carries them in full.
- Stroke joins, caps and decimation are our code (a few dozen lines), not a library's.

## Reversibility

Cheap: one package behind one function; the preview is rebuilt on every deploy and lives at most 30 days.

## Would choose differently if

- Users regularly title workouts in scripts the Go fonts lack (OQ-92 feedback): embed one subset Noto font, in a revision.
- A messenger rejects the card for a reason only a richer renderer fixes (map tiles behind the route): tiles are out of scope and would break I-6; ask the user first.
