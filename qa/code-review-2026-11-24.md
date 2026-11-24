# Whole-codebase code review - 2026-11-24

- **Date:** 2026-11-24
- **Reviewed:** main at fb3f6ab (working tree clean at start)
- **Scope:** Go server (`src/server/`), Svelte/TS front end (`src/web/`), deploy scripts (`deploy/`)
- **Reviewer:** senior-qa-engineer (whole-codebase review requested by the user)
- **Status:** complete (coverage limits in Coverage below)
- **Verdict:** not ready for a hostile internet without fixes - 0 Blocker, 2 Major (CR-01, CR-02, both CONFIRMED by probe), 0 Minor. Both are denial-of-service paths open to anyone who can upload a file; CR-02 also delays expiry past SH-10. No privacy leak, token leak, injection or data-loss defect found in the areas covered.

## Plan

- [x] P1 Build and tests: `make ci` (or go test -race, go vet, vitest), record exact results
- [x] P2 Read spec / technical design invariants (UP-9, AC-, SH-, INF-) and architect/INDEX.md
- [x] P3 HTTP trust boundary: `internal/httpapi` (upload, owner, report), routing in `cmd/workout-explorer/wiring.go`, `main.go`
- [x] P4 Token / owner auth: `internal/store`, `internal/lifecycle/owner.go`, token hashing, header-only tokens
- [x] P5 Ingest: `internal/ingest` (FIT parse, limits, strip, devfields, listener, frame, sequence) - DoS via crafted FIT, memory bounds
- [x] P6 Privacy pipeline: `internal/workout/hiding.go`, `trim.go`, `internal/publish` (pins, publish), `internal/preview` (map, preview), `privatedir` - nothing public before hiding, delete keys, devfields
- [x] P7 Lifecycle: `internal/lifecycle` (lifecycle, regenerate, pins, previewmap), expiry, delete, crash consistency, atomic writes
- [x] P8 Data file: `internal/datafile` encode/decode, front-end `lib/datafile` decode (bounds, alignment)
- [x] P9 Reports / notify: `internal/reports`, `internal/notify`, `store/reports.go`, `httpapi/report.go` - SSRF, rate limits, stored data
- [x] P10 Page rendering: `internal/page`, `internal/assets`, `internal/about`, CSP, headers, generic page, OG data
- [x] P11 CLI commands: `cmd/workout-explorer` (delete, parse, regenerate, settings, notifystate)
- [x] P12 Front end: `src/web/src` (edit, owner, pins, map, charts, upload, mine) - XSS, token handling, storage, listeners
- [x] P13 Deploy: `deploy/` (deploy.sh, update.sh, sign-release.sh, updater, podman ctl, systemd, Caddyfile, Dockerfile) - shell injection, permissions
- [x] P14 Tests that pass vacuously (spot checks)

## Progress

| Area | Status | Notes |
| --- | --- | --- |
| P1 | done | make lint test build size: rc 0 (23 Go packages ok, vitest 95 files / 2502 tests, svelte-check clean, entry 61,364/62,000). go test -race on httpapi, lifecycle, notify, store, reports, privatedir, page, about, assets: all ok. Race on cmd/workout-explorer: ok (487 s). caddy-check/docker-check/update-check not run (docker pull = external). |
| P2 | done | Technical design invariants I-1..I-8, dependency rules read |
| P3 | done | wiring.go, main.go, all httpapi handlers read. Shutdown: Shutdown grace 10 s < upload deadline 180 s, then the DB closes under running handlers; consistency is restored by Recover at the next start (not a defect). |
| P4 | done | lookup: base62 check, SHA-256, constant-time compare, dummy hash for absent rows; token only in Authorization header; bearer() single header; query string refused. Front end lib/owner: redirect 'error', credentials omit, no-referrer. OK |
| P5 | done | ingest.go, log.go, limits.go, strip.go, strip_table.go, accept.go, device.go, devfields.go, apple_name.go read; CR-01 found and probed (Normalise outside the time limit, quadratic in overlapping laps). Strip/framing not re-audited (fuzzed and swept in F-09 rounds). Panic value logged under ADR-018 (accepted). KindInternal deletes on regenerate: user decision (F-09 v1.9 line 716), not a finding. |
| P6 | done | publish.go, pins.go, hiding.go, HidesMore allow-list, remove-first path, sweep, Caddy file routes read. Probe: PATCH of laps/pins/title on a published workout reaches the page through the RG-5 recheck (scratch probe_patch_published_test.go) - no defect. Pins public only when positioned after trim+hiding; town from public file. |
| P7 | done | Upload (reserve, staged create, insert, cleanup), Delete (row first, public, private), Recover, Expire, SweepPublic, Regenerate (re-read, Accept, Replace, UpdateDerived, hook, recheck), privatedir (O_EXCL, fsync, rename, Replace never creates the folder), assets Copy/Clean read. Crash consistency relies on startup Recover and is sound. CR-02 found. Regenerate's storage drift is documented (F-30 RG-13). |
| P8 | done | front-end decode.ts: bounds, alignment, ordering, sentinel checks all present. Go side not re-read (golden tests). |
| P9 | done | httpapi report/reportdelete, reports.go, store/reports.go, notify.go read; no defect found (operator-set URL, html-escaped message, key hashed, single-use via UPDATE ... RETURNING) |
| P10 | done | index.html template actions (html/template contextual escaping), LoadTemplate probe, shell fill (html.EscapeString), about/impressum handlers, Caddyfile headers (CSP, Referrer-Policy no-referrer, X-Robots-Tag, nosniff, frame-ancestors none, private immutable for hashed files) read. page/format.go parity with lib/page not re-checked (contract tests). |
| P11 | done | delete.go (title printed with %q), regenerate.go, settings.go (baseURL strict), notifystate.go read; no defect |
| P12 | partly done | Spot checks: HTML sinks (innerHTML only with constants, numbers, escaped server HTML; owner text via textContent), lib/owner token handling, lib/mine, lib/datafile decode, network calls (only same origin + tile hosts), console logging (no tokens), listener add/remove counts. Not reviewed in depth: charts/map hot paths, edit page flows, splits/legs/efforts, zip reader, i18n. |
| P13 | done | Caddyfile, compose.yaml, Podman units and ctl, deploy.sh, update.sh (minisign, manifest, unpack checks, curl -K for secrets), sign-release.sh, workout-explorer-run read; no defect found. Dockerfile, systemd units and polkit rule only skimmed via Makefile checks. |
| P14 | done (spot checks) | TestNoRequestMetadataStored (needles in files, DB and logs), TestParseLogs (allow-listed keys, position scan with a >=1000-needle guard), F-65 end-to-end test read: all assert behaviour. Gap: the ADR-017 all-caps benchmark only has non-overlapping laps (see CR-01). |

## Findings

### CR-01 - Major - CONFIRMED - Normalise is quadratic in overlapping laps; a crafted upload holds a parse slot for 40+ s outside the parse time limit
- **Where:** `src/server/internal/workout/laps.go:26-31` (`completeLaps` -> `completeTotals` -> `rangeValues`/`rangeMean`, linear in each lap's sample range); time limit only around the decode walk in `src/server/internal/ingest/ingest.go:132-184`.
- **Defect:** every lap's AP-8 totals are computed by scanning the lap's samples, so the cost is O(laps x lap length). ADR-017 item 2 requires Normalise to be O((n + k) log(n + k)) and rules out "any loop over ... laps ... per sample"; the ADR-017 all-caps benchmark uses 10,000 *non-overlapping* laps (`tools/corpusgen/f08.go:565`), so it never sees this.
- **Evidence (probe):** scratch copy of the module, `tools/corpusgen/probe_overlap_test.go` writes a 15.2 MB FIT (under the 20 MB cap): 500,000 records at 1 Hz, one session, 10,000 bare laps each from the first to the last record. `ingest.NewParser(Limits{}).Parse` = 24.5 s, `Accept` = 43.1 s, both succeed (parse time limit 10 s). ADR-017 stop rule: decode + normalise <= 2 s, Accept <= 4.5 s.
- **Failure scenario:** anyone can upload such a file (60 per hour per IPv4 address or IPv6 /64). Each upload holds one of the process's 2 parse slots for ~43 s and then is *accepted* (the HTTP write deadline is 180 s), so it is stored. Every later parse of that workout (publish build, owner GET/PATCH with a trim or pins, preview frame, regenerate on every deploy) takes ~24 s, most under the O-7 service lock. Two or three such uploads in a row stall all uploads and owner edits/publishes of every user; regenerate on deploy is slowed by 24 s per such workout.
- **Same pattern, owner-triggered:** `workout/addedlaps.go:30-38` (`lapSpans` marks every covered sample per lap) and `workout/autolaps.go:68` (`completeTotals` per merged run) on such a file, once added lap boundaries or remove_auto_laps are set. Sessions are capped at 100, so `trim.go:193` stays bounded.
- **Route:** backend (lap totals from prefix sums, or cap the summed lap span; add overlapping laps to the ADR-017 all-caps benchmark and stop rule).

### CR-02 - Major - CONFIRMED - Any uploader can hold the global owner lock (O-7) with unthrottled owner GETs and delay expiry, deletes and hiding changes of every workout
- **Where:** `src/server/internal/lifecycle/owner.go:562-573` (`View`: a trimmed workout's owner view takes `s.mu` and parses the original), called by every `GET /api/workouts/{id}` (`src/server/internal/httpapi/owner.go:338-349`); `Expire` takes the same `s.mu` per due row and for the sweep (`src/server/internal/lifecycle/lifecycle.go:322-366`); `DeleteByOwner`, `Update`, `Publish`, `PutPreviewMap` take it too. The owner endpoints have no rate limit or per-token concurrency cap.
- **Defect:** a read-only request serialises on the process-wide write lock for the duration of a full parse + encode, and nothing bounds how many such requests one token holder can queue. Go's mutex hands off FIFO under contention, so every lock user waits behind all queued GETs.
- **Evidence (probe):** scratch `cmd/workout-explorer/probe_lock_test.go` through the real wiring: upload synthetic `ride-12h.fit`, PATCH a trim, then fire k concurrent owner GETs and time one `svc.Expire` pass (no due rows, so only its sweep step takes the lock): k=0 -> 1 ms, k=8 -> 1.27 s, k=32 -> 4.43 s (one trimmed GET = 0.19 s). The wait grows as k x parse time; with the CR-01 file (24 s per parse) two queued GETs already exceed SH-10's 60 s.
- **Failure scenario:** anyone uploads a file (no account needed; the edit token comes back in the response), sets a trim, and keeps a few hundred GETs (or, with CR-01's file, two or three) in flight. Every expiry step then waits behind the queue, so expired workouts stay public past SH-10's 60 s and drafts past 48 h (privacy, UP-7/SH-10); other owners' DELETE, PATCH (raising a hiding radius) and publish block for the same time and their 30 s/150 s request deadlines expire.
- **Design context:** F-47 TR-10 puts the owner-view parse under O-7 on purpose (one owner-side build at a time bounds memory; "trimmed workouts are rare"), and F-12 O-2 says "no rate limit". Neither weighs that the same lock gates expiry (SH-10's 60 s) and deletes, nor a hostile token holder; that consequence is undocumented.
- **Route:** architect, then backend (do not parse under `s.mu` for a read: compute the owner view outside the lock or cache it per row version; consider a per-token or per-IP limit on owner endpoints; give expiry priority or its own lock scope).

## Coverage

- **Run:** `make lint test build size` (rc 0); `go test -race` on httpapi, lifecycle, notify, store, reports, privatedir, page, about, assets and cmd/workout-explorer (all ok). Probes in a scratch copy of the module (synthetic testdata only).
- **Not run:** `make caddy-check`, `docker-check`, `update-check` (Docker pulls from external registries), fuzzing, `make corpus` on real files (never read).
- **Not covered in depth:** front-end hot paths and newer UI features (F-64 to F-69), Go datafile encoder, preview rendering, page/format parity, Dockerfile and systemd/polkit files.

## Notes for continuing

- Probe module copy (no testdata) at scratchpad/review/mod; probe files `tools/corpusgen/probe_overlap_test.go` and `internal/ingest/probe_overlap_test.go` (run with PROBE_OUT / PROBE_IN env).
- Read so far: wiring.go, main.go, httpapi (all non-test), lifecycle (lifecycle, owner, identity, record, storage, previewmap, regenerate, laps), publish (publish, pins, template), workout (hiding, workout, addedlaps, laps head), store (store, reports), privatedir, assets, reports, notify, preview/map head, cmd regenerate, deploy scripts.
- Review finished. Leads closed: shutdown ordering (Recover restores consistency), sessions (bounded by MaxSessions 100), front-end decode bounds (checked), delete/settings CLI (fine), shell escaping (fine). Second probe files: `cmd/workout-explorer/probe_patch_published_test.go` (no defect), `cmd/workout-explorer/probe_lock_test.go` (CR-02); they need scratchpad/review/web with the template sources copied from src/web.
- If continued: depth review of the front end (P12 rest), the Go datafile encoder, preview rendering with hostile titles, a fuzz run of FuzzAccept, and make caddy-check/docker-check/update-check (need Docker and a network pull).

- Started 2026-11-24. Scratch probes go in `/tmp/claude-1000/-home-b0wter-syncthing-work-workout-explorer/1973ef5e-b065-4f5d-9058-f861d81899dd/scratchpad/review/` (may be cleared).
- Never read files under corpus/real or expected/real.
