# QA F-15 Publish pipeline and static page - round 1

- **Date:** 2026-10-25
- **Verdict:** not ready - one Major: a remove-first build (hiding on, later download off) that stops after `index.html` is removed leaves the older, less hidden data file in a live folder, the startup sweep keeps it, and Caddy serves it by its known URL (QA-15-r1-1, SH-10). Latent today because the public transform is the identity, but it breaks the guarantee ADR-028 and P-7/P-8 were built for, and F-16 plugs straight into it.
- **Inputs:** `features/F-15-publish-pipeline-and-static-page.md` (AC 10 as reworded 25 Oct), `architect/F-15-publish-pipeline-and-static-page.md` v1.0, ADR-028, ADR-022, ADR-025, ADR-026, ADR-027; spec SH-1, SH-2, SH-10, UP-4, UP-7, AC-5, DS-4, NFR Privacy; commit `0829e8e` on master (clean tree); engineer's deviation list from the build report.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0, 27.7 s. Go: all 14 packages ok. Vitest: 40 files, 1,076 tests passed. Entry `index.html` 50,220 / 60,000 B gzip PASS; `upload.html` 2,270 / 8,000 B PASS; no shared chunk PASS |
| `go vet ./...` | clean |
| `go test -race -count=1 ./...` | all 14 packages ok (publish 31 s, lifecycle 4 s, cmd 14 s) |
| Local Caddy 2.10.2 with `deploy/Caddyfile` (my script `qacaddy.sh`, ports 18287/18288, scratch data dir) | AC 1, 5, 6 pass; details under Coverage |
| SIGKILL loop: real binary, `ride-12h`, 40 kills at random points in title / hiding on / hiding off / private changes, restart after each | 40 / 40 states were a complete page (hash of the data file matches its name, `.br` present) or no `index.html`; 0 broken |
| Go vs JavaScript differential: 510,296 values (random, binary ties `k/2^j`, decimal ties as typed, realistic distances and speeds) through `formatFixed` (0, 1, 2 decimals), `formatPace`, `formatElapsed`; JS run with Node 22 on a copy of `lib/format/index.ts` | 0 mismatches |
| Publish of every corpus file the parser accepts (synthetic and the local real files, read only) | 52 files: 44 published, 8 refused by `Accept` as expected, 0 failures, no public file contains `original.fit` |
| Concurrency probe: 12 goroutines × 25 random publish / private / unlisted / title / hiding / delete on 4 workouts, `-race`, 3 runs | no race; afterwards a folder exists exactly for each unlisted row, each with 3 entries; no ERROR line |
| Mutants (scratch copy): `LivePublicIDs` ignores expiry; `Delete` skips the public folder; recheck ignores expiry; `fail()` keeps the path | all 4 killed |

## Findings

### Major

- **ID:** QA-15-r1-1
- **Severity / kind:** Major / risk (latent defect: correct output today only because the transform is the identity)
- **Route:** architect (P-8 and ADR-028 item 5 sweep rule), then backend
- **Where:** `src/server/internal/publish/publish.go:426` (Sweep keeps every non-dot entry of a kept folder), `publish.go:155-159` (remove-first), `deploy/Caddyfile:47-48` (`/w/{id}/{name}` is served whether or not `index.html` exists)
- **Relates to:** SH-10, ADR-028 context 3 and item 4, P-7, P-8, F-21 hand-off ("the SH-10 bound for every failure path"), AC 8
- **Evidence:** probe test in a scratch copy of the module: publish, then a hides-more change (`hide_ends` on, transform standing in for F-16 drops a stream). The build is stopped at step `remove folder`, once by an injected `EACCES` (the owner gets a 500) and once by a panic standing in for a kill. Then `Sweep` runs with the row's `public_id` in `keep` (what startup does for a live unlisted row). Result in both cases: `index.html` absent, old `data.7dcbb7dc349d0092.bin` (and `.br`) still on disk, `sweep removed 0`. Caddy serves `/w/{id}/data.{hash}.bin` straight from the folder (checked in the Caddy run: data files are served by the `@file` route independently of the page). `TestKill` accepts this state ("no `index.html`") and nothing cleans it afterwards.
- **Consequence:** once F-16 (hiding) or F-24 (export, download off) make the transform non-identity, a crash or a file-system error between the two remove steps keeps the pre-hiding data public at a URL every earlier viewer already has (browser cache, a copied link), until the owner happens to save again. Neither the startup sweep nor F-21's per-minute sweep (same `Sweep`) fixes it, so SH-10's 60 s does not hold on this path. A cheap rule fixes it: the sweep removes a kept folder that has no `index.html` (it shows the generic page anyway), or removes every file `index.html` does not reference.

### Minor

- **ID:** QA-15-r1-2
- **Severity / kind:** Minor / defect
- **Route:** backend
- **Where:** `src/server/internal/lifecycle/lifecycle.go:174`
- **Relates to:** ADR-022 item 6, F-12 AC 5, NFR Privacy (no data kept past deletion)
- **Evidence:** probe through `newApp`: a file where `public/{public_id}` belongs makes `Public.Remove` fail. `DELETE` answers 500, the row is gone, and `private/{id}/original.fit` is still on disk. A retry answers 404, so the owner cannot finish the delete. Before F-15 the private folder did not depend on any other removal.
- **Consequence:** the original FIT file (with coordinates) stays on disk until the next restart, which can be weeks. Fix: still call `Files.Remove` after a public failure and join the errors. The same applies to a future caller (F-21, F-27) that passes an empty or wrong `publicID`: `Remove` refuses it with `errInvalidID` after the row is already gone.

- **ID:** QA-15-r1-3
- **Severity / kind:** Minor / risk
- **Route:** architect (F-22)
- **Where:** `deploy/Caddyfile:47-55` (`precompressed br`)
- **Relates to:** P-13, F-22, ADR-006
- **Evidence:** a `GET` of the data file with `Accept-Encoding: br` and no `Range` header gets `206 Partial Content`, `Content-Range: bytes 0-12174/12175` (Caddy 2.10.2). Without `br` it gets `200`. The client checks `response.ok`, so loading works. Browser cache and preload matching of a 206 that answers a request without `Range` were not checked.
- **Consequence:** possible double download or no reuse from the HTTP cache in some browsers, which counts against F-22's caching work. This is recorded only in the engineer's report, not in the design. F-22 should confirm it in Chrome, Firefox and Safari, or work around it.

- **ID:** QA-15-r1-4
- **Severity / kind:** Minor / risk
- **Route:** architect (F-30 hand-off), then backend
- **Where:** `src/server/internal/publish/publish.go:319,331` (fixed temp name `.{name}.tmp`, `O_TRUNC`)
- **Relates to:** P-6, ADR-028 item 5 (other processes: F-27, F-30)
- **Evidence:** reading the code. In one process the O-7 lock serialises builds, so nothing goes wrong today. Two processes building the same workout (F-30 running while the server is up) would open the same temp file, truncate it under each other and rename possibly mixed bytes over `index.html`.
- **Consequence:** a broken page if F-30 runs alongside the server. Either use `os.CreateTemp` in the folder (dot prefix kept) or have F-30 state that it runs only with the server stopped.

- **ID:** QA-15-r1-5
- **Severity / kind:** Minor / preference (documentation)
- **Route:** architect
- **Where:** `architect/F-15-publish-pipeline-and-static-page.md`
- **Relates to:** CLAUDE.md ("lists every deviation"), P-3, P-8, P-14, P-15
- **Evidence:** the build commit changes only the status lines of the design and the ADR. None of the engineer's deviations is in a design document: the port shape (`Remove` + `Sweep(keep)` with `Repository.LivePublicIDs`), `Delete(ctx, id, publicID)` (F-21 and F-27 must pass the right `public_id`, see r1-2), bytes freed by the post-hook re-read not released until restart (`owner.go:306`), deadline `2 × (30 s + T)`, the Caddy 206.
- **Consequence:** F-21, F-27 and F-30 will build on undocumented contracts. My judgement of each deviation is in the Decisions section below; only the documentation is missing.

- **ID:** QA-15-r1-6
- **Severity / kind:** Minor / preference
- **Route:** backend
- **Where:** `src/server/internal/publish/publish.go:302,437,455` (`size, _ := usage(p)`)
- **Relates to:** P-9
- **Evidence:** reading the code: a measuring error is dropped and counts as 0 freed bytes.
- **Consequence:** the storage total can only drift upwards (uploads close early), never towards a leak. Log it or count it.

## Decisions (engineer's deviations)

| Deviation | Judgement |
| --- | --- |
| Port is `Remove` + `Sweep(keep)` with `Repository.LivePublicIDs` | Sound: the keep set comes from one query and the sweep stays dumb. The sweep's rule for kept folders is wrong, see r1-1 |
| `Delete(ctx, id, publicID)` | Fine. It needs r1-2 so that a bad or failing public step does not skip the private folder |
| Bytes of a folder removed by the post-hook re-read are not released until restart | Accept: they are overcounted (it fails towards refusing uploads), and only the cross-process path (F-27, F-30) triggers it. Record it (r1-5) |
| Startup sweep runs even if earlier recovery steps failed | Correct, it fails towards less public content. `newApp` still refuses to start on any recovery error, as before |
| Write deadline `2 × (30 s + parse limit)` (80 s at defaults) | Acceptable. The build runs under `WithoutCancel`, so a missed deadline loses only the response, never half a build |
| `LoadTemplate` dry-runs the template with a filled view model | Good: a field mismatch fails at start (tested) |
| Fixed temp names `.{name}.tmp` | Fine in one process; cross-process risk, see r1-4 |
| Shared `theme/plain.css` carries the page frame | Fine. It is CSS only, imported by two non-activity entries, and the size check shows no shared chunk |
| Caddyfile defaults (`127.0.0.1:8088`, admin off, `/` → `upload.html`) | Fine; paths have no defaults, as the design says |
| Caddy answers `.br` with 206 without `Range` | Confirmed; see r1-3 |
| No separate bundle preload tag | Accept: the module script is in `<head>` (F-13 D-11 lists it as the bundle's load), and the data file has its preload |
| TD hand-offs not applied | Main session routes them; not judged here |

## Previous findings

No earlier F-15 round. Findings routed to F-15 from earlier features:

- QA-12-r1-3 (hook and delete error text would carry `public_id`): **fixed.** `fail()` reduces path errors (mutant killed). `TestErrorsCarryNoPublicPath` and `TestPublishFailureLogs` cover it, and the Caddy run logged 0 lines with a `public_id` or `public/`.
- F-10 AC 4 hand-off (publishing while uploads are paused is counted): **fixed**, `TestPublishWhileFull`.

## Questions

1. **User / main session:** may QA-15-r1-1 be deferred as an entry condition of F-16 instead of being fixed in F-15? Suggestion: no, fix it now. The sweep rule is a few lines, and it is cheaper while ADR-028 is fresh. Settled by: a decision recorded in `features/README.md`, or an F-15 r2 with the fix.

## Coverage

| AC | Code | Test / check | Result |
| --- | --- | --- | --- |
| 1 | `lifecycle.Publish`, `publish.WorkoutChanged`, Caddyfile `@view` | `TestPublishEndToEnd`, `TestPublish`; Caddy: draft 404 generic, published 200 full page | pass |
| 2 | `page.Build`, template | `TestPublish`, `TestPublishEndToEnd` (summary values, lap cells, robots meta) | pass |
| 3 | `write` skips existing names | `TestTitleOnly` (no `.bin` step, mtime unchanged), e2e; Caddy: title PATCH kept the names | pass |
| 4 | content-hashed names, old entries removed after commit | `TestNewDataFile` (transform seam) | pass |
| 5 | `WorkoutChanged` non-unlisted → remove | `TestPrivateAndBack`, e2e; Caddy: private → 404 generic, data URL 404, unlisted → same names, and the same bytes once the title was restored | pass |
| 6 | one static `generic.html`, Caddy rewrite | Caddy: draft, private, deleted, unknown, trailing slash and lower-case ID give the same 911-byte body; headers identical apart from `Date` | pass |
| 7 | `theme/plain.css` | `generic.test.ts` static checks | pass (static only; the manual browser check with JavaScript off was **not run**, no browser here) |
| 8 | `place`, `write`, `removeFolder` order, sweep of dot files | `TestKill` (every step, 5 scenarios); my SIGKILL loop 40 / 40 | pass for the page; the leftover after a remove-first stop is r1-1 |
| 9 | `Delete` | `TestOwnerDelete` (public folder in the data holders); Caddy: deleted → generic, `public/` empty | pass; failure path r1-2 |
| 10 | build reads only from the parse | `noOriginalBytes` after publish, title and transform change; corpus run of 44 files | pass |

Spec: SH-1 (22-character base62 IDs checked before any path is built), SH-2 (404 generic for draft and private), UP-4 (publish → unlisted, `published_at` kept on republish), AC-5 (title and lifetime reach the page), DS-4 (static), NFR Privacy (no folder for a draft in any test or probe; the concurrency probe keeps folders only for unlisted rows): traced. SH-10: switching to private and deleting take effect within the request. The failure paths depend on F-21's sweep plus r1-1.

Caddy extra probes, all 404 and never file contents: dot files (existing and missing), `/index.html` (the template), `/assets/../index.html` raw and encoded, `/w/{id}/..%2f..%2fprivate`, raw `../../private/`, `/w/../workouts.db`, `/generic.html`, `/upload.html`, `/w/{id}/original.fit`, a draft's `/w/{id}/index.html`. A publish with the token in the query string gets the owner not-found response.

Not run: browser checks (AC 7 manual, the 206 behaviour), power-loss durability (`fsync` order checked by reading only), the `make corpus` golden diff (unchanged parser), F-21 and F-30 paths (not built).
