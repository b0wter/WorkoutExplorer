# ADR-045 Place data: a vendored GeoNames snapshot, embedded in the binary

- **Status:** accepted
- **Date:** 11 Nov 2026
- **Features:** F-45 (suggested title); F-29 (attribution), F-30 (regenerate uses the binary's table)
- **Supersedes:** none. Narrows the TD tech-stack row "Place names (UP-5)": the "grid or k-d tree" becomes a linear scan until measured otherwise (F-45 ST-2)
- **Requirements:** UP-5, INF-1; NFR Privacy (no external services), NFR Performance

## Context

UP-5 needs the nearest town of `cities15000` to one point, on the server, with no outbound call (TD Principles). The data must reach the binary somehow. The repository is public (AGPL-3.0), builds run in CI, in the Dockerfile for self-hosters, and on the operator's machine.

GeoNames publishes `cities15000.zip` at one fixed URL and rewrites it as the database changes; there is no versioned or dated copy. A build-time download therefore gives a different table on different days and cannot be checked against a pinned hash.

Rejected:
- **Download at build time from GeoNames:** not pinnable (above); every build, CI and self-hosted, depends on GeoNames being up.
- **Our own pinned snapshot as a release asset, fetched with a SHA-256 check:** keeps the repository free of the data, but adds an upload step, a network dependency to `make ci` and a second place where build inputs live.
- **A file on the server, next to the binary:** a second deploy artefact and a missing-file state for every self-hoster, for no gain.
- **Committing the full upstream file (about 7 MB with alternate names):** most of it is never read.

## Decision

1. **Vendored.** One UTF-8 TSV, `name`, `latitude`, `longitude`, copied verbatim from upstream columns 2, 5 and 6, in upstream row order, committed in `src/server/internal/places/`, with a source note: download date, URL, licence, and the exact command that produced it, so anyone can reproduce the projection from that day's upstream file. Ceiling 1.5 MB (about 1 MB expected).
2. **Embedded** with `go:embed` into the one binary; no runtime file, no setting. `make ci` checks the table (F-45 ST-1).
3. **Refreshed by hand**, as a normal reviewed commit, when someone wants newer towns; no schedule. A refresh can change the town of existing workouts at the next `regenerate`: accepted, workouts are short-lived.
4. **Licence.** The table keeps its own licence, CC BY 4.0, stated in the source note; it is not AGPL. Attribution on the About page through the F-29 sources file, marked as modified (columns reduced).

## Consequences

- Builds are offline for this data and reproducible; a diff shows exactly which towns changed.
- About 1 MB of third-party data in the repository, and about the same again in git history per refresh. This is the main cost, and the reason refreshes stay rare.
- The binary grows by the table's size; the index costs at most 4 MB of heap per process (F-45 ST-9).

## Reversibility

Cheap for builds: switching to a pinned release asset changes only where the file comes from. Not cheap for history: committed snapshots stay in git history unless it is rewritten.

## Would choose differently if

- The table must be refreshed often (for example monthly): then a pinned release asset, so the history does not grow with each refresh.
- A finer or larger dataset is wanted (`cities1000`, about 10 MB): then a release asset, and a grid index.
