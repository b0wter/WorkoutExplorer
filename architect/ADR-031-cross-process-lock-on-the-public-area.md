# ADR-031 Cross-process lock on the public area

- **Status:** accepted (31 Oct 2026), with the F-21 design v0.2
- **Date:** 31 Oct 2026
- **Features:** F-21 (introduces it); F-30, F-27 (other processes); F-15 (the code it changes)
- **Supersedes:** none. Settles QA-15-r2-1. Narrows the F-12 hand-off "the O-7 lock is per process" for `public/`
- **Depends on:** [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md) items 3 to 5
- **Requirements:** SH-10, AC-4, NFR Privacy; TD invariant I-3

## Context

1. ADR-028 item 5's sweep removes every entry of a kept folder that `index.html` does not reference, and a kept folder without `index.html`.
2. A build writes its new data file and `.br` before `index.html` (ADR-028 item 3), with brotli in between. A first publish or a remove-first build has a folder with no `index.html` for that whole time.
3. `serve` sweeps every 30 s (F-21). `regenerate` (F-30) builds while `serve` is up (TD Deploys), and `delete` (F-27) removes folders. The O-7 mutex is per process.
4. QA-15-r2-1 showed the result: a sweep in one process during a build in another deletes the new data file, and the build then commits a page that links a missing file.

## Decision

1. **One exclusive advisory lock**, `flock(2)` with `LOCK_EX`, blocking, on `{data}/public.lock`. `publish` creates the file when it opens the area (mode `0600`). It is never under `public/`.
2. **`publish` takes it for every operation that changes `public/`**, in every process: a build's write phase (from creating the folder to removing unreferenced entries), every folder removal (owner delete, expiry, remove-first, the post-build re-read) and the sweep. Reading the original, parsing and encoding run outside it; compression of a new file may run inside it. Callers never take it themselves.
3. **Lock order** in `serve`: O-7 first, then the public lock. Nothing waits for O-7 while holding the public lock.
4. **`flock`, not `fcntl` locks**: `flock` belongs to the open file, so two openings in one process exclude each other (testable in one test process), and the kernel releases it when the holder dies (no stale lock). Pure Go `syscall`, so the static `CGO_ENABLED=0` binary is unaffected.
5. **Rejected: a grace period** (the sweep leaves unlinked files younger than N seconds). It does not cover context 2's folder without `index.html`, and it makes what the sweep removes depend on file times.

## Consequences

- A sweep never sees a half-written build from any process, so F-30's "every page keeps working" holds against `serve`'s sweep.
- Another process's write phase delays `serve`'s sweep and owner writes by milliseconds, up to about a second with compression of a large file (F-21 X-6).
- A holder that hangs (stopped, not dead) blocks every change to `public/`. Not guarded; an operator matter.
- **Not covered:** a build from a record that changed meanwhile (F-30 must re-check under the lock, F-21 hand-off), and `private/` (F-30).
- Local file systems only; `flock` is unreliable on network file systems.

## Reversibility

Cheap. No data format; the lock file can be dropped with the code.

## Would choose differently if

- Public files move to an object store (TD scaling path): no `flock`; the commit point changes too (ADR-028), so a new ADR.
- More than one host writes the data directory.
