# QA F-07 Prototype page and scrubbing gate - round 1

- **Date:** 2026-10-15
- **Verdict:** **not ready.**
  - QA-07-r1-1 (Major): the scrub recorder does not enforce ADR-016 decision 3, which says a finger must be down in every frame of a touch run. If the finger lands up to about 2.5 s into the window, the run still counts. The idle frames before it raise the pass share. In a probe, a scrub that fails on its own (98.74 % of frames ≤ 20 ms) is reported as `Counts: yes` and `PASS: yes`. Fix this **before** the operator runs the phone gate, or the gate records cannot be trusted.
  - Apart from that, the build is sound. CI is green, the bundle numbers reproduce, protoserve matches B-1 to B-5, and the pixel-ratio fix is correct and well tested. The gate itself (AC 1, 2, 3, the manual part of 7) and AC 8 still wait on the operator and the user, as expected.
- **Inputs:**
  - Feature: `features/F-07-prototype-page-and-scrubbing-gate.md`.
  - Design and decisions: F-07 design v1.1 (including "Engineer choices, confirmed"), ADR-016, ADR-011 rev. 7, ADR-013 rev. 4, ADR-014 rev. 2 (CSP table), overview v1.10, F-06 design v1.6 (bundle procedure (b)).
  - Code: `src/server/tools/protoserve/`, `src/server/scripts/prototype-data.sh`, `Makefile`, `README.md`, `src/web/index.html`, `src/web/src/main.ts`, `src/web/src/prototype/`, `src/web/src/measure/`, the `map-style`, `map-view` and `maplibre-fake` diffs, `src/web/bench/MEASUREMENTS.md`.
  - State: the uncommitted working tree on `f037688`, untracked files included.

## Build and test results

| Check | Result |
| --- | --- |
| `make ci` | **Exit 0.** gofmt clean. svelte-check: 259 files, 0 errors, 0 warnings. `go test ./...`: 9 of 9 packages ok, `tools/protoserve` included. Vitest: **30 files, 712 passed** (matches the engineer). Size line: `entry index.html: 45553 / 60000 bytes gzip (40823 brotli) PASS`. |
| `go test -race -count=1 ./tools/protoserve`, `go vet` | ok, clean. |
| `make prototype-data` | `ok     ride-12h  bin 1176640 br 266150` / `ok     ride-4h  bin 396156 br 106951` / `budget ride-4h br 106951 / 500000 PASS` / `prototype data: 2 files in build/prototype/data`. Both `cmp` against `testdata/datafile/synthetic/*.bin` are silent. |
| `make prototype-serve PROTO_ADDR=:18090` + curl (B-5 step 5) | `br` → `200 266150`; no `br` → `200 1176640`; `GET /` with `br`: 200, `Content-Encoding: br`, `Content-Type: text/html; charset=utf-8`, `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, `Vary: Accept-Encoding`, `X-Content-Type-Options: nosniff`; `/.vite/manifest.json` → 404; `POST /` → 405. Extra checks: `--path-as-is /../index.html` and `/data/../index.html` → 404; `/data/ride-12h.bin.br` → 404; the query string is ignored; `curl --compressed` decodes to bytes identical to `ride-12h.bin`. The log has one B-3 line per request. The start-up block is in the B-3 order. |
| Stop | SIGINT to the make process group: `protoserve: stopped`, then `make: *** [Makefile:113: prototype-serve] Error 1` (as the engineer reported; `go run` exits 1). Port 18090 is free afterwards. |
| `-tls` (built binary, `127.0.0.1:18092`) | `curl -k` → `200 2` (HTTP/2). The SHA-256 line is printed. Stopped cleanly. Unknown flag → exit 2; `-h` → usage, exit 0. A listen error prints one `protoserve: …` line. |
| B-5 step 7 | Dependencies: `andybalholm/brotli(/matchfinder)` and `internal/precompress` only. `go list -deps ./cmd/... \| grep -c protoserve` → `0`. |
| Bundle, rebuilt in the scratchpad from `f037688` with the step 1 temporary imports | **B0 = 42,524** (engineer 42,525), **B0′ = 45,329** (engineer 45,328), **M0 = 283,994**. After F-07: **E = 45,553**, so **E − B0′ = 224 ≤ 300** and E ≤ 46,000. **M1 = 284,135**, so M1 − M0 = **141 ≤ 300**. The recorder chunk is **2,438** gzip ≤ 4,000 (5,131 raw, 2,150 brotli). The ADR-011 rev. 7 ledger adds up: 45,553 + 13,000 + 1,447 = 60,000. |
| Step 5 and step 6 | `grep -c '(resolution: ' index-*.js` → `0`; `grep -l` finds `map-view-*.js`. `build/web/index.html` has no `modulepreload`. The manifest lists `src/measure/recorder.ts` only under `dynamicImports` of `index.html`. "Scrub record" appears only in `recorder-*.js`. |
| MapLibre | 6.14.0 installed; `Map#setPixelRatio` sets `_overridePixelRatio` and calls `resize()`, as the code comment says. |
| Commit hygiene | `git status --porcelain --untracked-files=all` lists only source and docs. Nothing is under `build/`, and there is no `.bin`, `.br` or report `.json`. `git check-ignore` confirms that `build/prototype/data/*` and `corpus/real/*` are ignored. The three real FIT files in `corpus/real/` are ignored and untracked. No tracked `.bin`, `.br` or `.fit` exists outside `synthetic/`. The parse report contains `first`/`last` latitude and longitude, and it stays under the ignored `build/`. |
| Working tree | QA changed nothing apart from this report and its INDEX row. All probes ran in the scratchpad. |

### Mutation probes (scratchpad copies)

- **Frontend: 27 mutants. 21 killed, 4 survived, 2 excluded.**
  - Excluded: M2, which is the proposed fix for r1-1. Also `createRecorder` moved after the state assignments without an `await`: that is equivalent, because Svelte mounts in a later microtask. The variant with an `await` is killed.
  - Killed: the stats bounds (≤ 20, ≥ 25, < 0.75, the max rule, nearest rank), the `mapInView` check, `startedOn`, the capture flag, listener removal, the window length, the `mapPixelRatio` cap and its finite check, `next !== applied`, re-arm, teardown, the page-query regex (length, case, empty value), the `measure` presence rule, the reload text, and `createRecorder` after an `await` behind the mount.
  - Survived:
    - charts and map buffers swapped;
    - `tM` never set;
    - `tM = tA`;
    - `flushed` never reset.

  See QA-07-r1-2.
- **protoserve: 23 mutants, 18 killed.**
  - Killed: `*` accepted, `q < 0`, an unparsable q accepted, case sensitivity of the coding and of `q`, `br` sent without `ae`, `Vary` missing or always set, no dot-skip, no `.br` skip, missing `nosniff` or `Referrer-Policy`, a body on HEAD, a 6-day validity, `IsCA`, no duplicate check, HTML not compressed, `path.Clean` on the request path.
  - Survived:
    - `Cache-Control` removed from 404/405;
    - `Content-Type` removed from 404/405;
    - an 8-bit serial;
    - no `KeyUsage`;
    - a different CN.

  The last three are not required by B-4. See QA-07-r1-4.

## Findings

### Major

- **ID:** QA-07-r1-1
- **Severity / kind:** Major / defect
- **Route:** architect (F-4 step 4 prescribes the faulty condition), then frontend
- **Where:** `src/web/src/measure/recorder.ts:158`, `if (fingers === 0 && input === 'touch') noFingerFrames++;`, which follows F-07 design F-4 step 4 word for word.
- **Relates to:** ADR-016 decision 3 ("in a touch run, a finger was down in every frame"); F-07 AC 2 and AC 3 (the gate)
- **Evidence:**
  - `input` becomes `'touch'` only at the first `touchstart`. Frames before it are therefore never counted as no-finger frames, even though the run is then reported as a touch run.
  - Probe `scratchpad/mut/src/web/src/measure/probe-late-finger.dom.test.ts`, run against the real `recorder.ts`: no finger during the countdown, finger down on a chart at 2.3 s into the window, then a flush every frame, with 6 of the 475 scrubbing frames at 33 ms (98.74 % ≤ 20 ms on their own, a fail). The recorder printed:
    - `Frames 619 · … · ≤ 20 ms 99.03 %`
    - `Cursor changed in 76.74 % of frames · frames without a finger 0 · map in view at start yes`
    - `Counts: yes`
    - `PASS: yes`
  - The only remaining guard is the 75 % changed-frames rule. It lets up to a quarter of the window be idle, cheap frames.
- **Consequence:**
  - A late finger, which is easy to do when the procedure says "within 3 s", turns a failing chart or route scrub into a counted pass that also reports "frames without a finger 0".
  - Gate 0 is the reason F-07 exists, and phase 1 starts on its verdict.
- **Fix that would settle it:** count every frame with `fingers === 0` and keep `touch: input === 'touch'` at the end. `scrubStats` already ignores no-finger frames for mouse runs.
  - QA mutant M2 makes exactly this change, and the 29 existing tests in `src/measure` still pass with it.
  - Add a recorder test where the finger lands after the window starts; it must report `a finger was lifted`.
  - Do this before the phone runs.

### Minor

- **ID:** QA-07-r1-2
- **Severity / kind:** Minor / test gap (risk)
- **Route:** frontend
- **Where:** `src/web/src/measure/recorder.dom.test.ts:217-225`; `recorder.ts:118-133`, `:160-161`
- **Relates to:** ADR-016 decision 1 (the charts/map split) and decision 3 (the 75 % changed rule); F-07 AC 6
- **Evidence:** four mutants survive the whole measure suite:
  - charts and map buffers swapped;
  - listener M a no-op;
  - `tM = tA` in listener A;
  - `flushed` never reset, so every frame after the first flush counts as changed.

  The full-run test checks only the `Flush ms (625) total p95 / max` prefix and uses a flush in every frame.
- **Consequence:**
  - A regression could send a failing run's operator to the wrong remedy in the "If the gate fails" table (AC 6).
  - It could also disable the cursor-still validity rule silently.
- **Wanted:** stub `performance.now` so that the charts and marker stand-ins advance it by known amounts, and assert the charts and map values. Add a run with flushes in only some frames and assert the changed percentage.

- **ID:** QA-07-r1-3
- **Severity / kind:** Minor / defect (recording template)
- **Route:** frontend
- **Where:** `src/web/bench/MEASUREMENTS.md:112` and `:140`
- **Relates to:** F-07 design v1.1, "What to record" (Bundle columns: B0, **B0′**, E, **E − B0′**, …) and D-9
- **Evidence:**
  - The section header cites design **v1.0**, whose operator procedure uses port 8090; v1.1 uses 18090.
  - The Bundle table has `B0 | E | E − B0` and no B0′ column.
- **Consequence:** the operator records the wrong share, 3,028 against a 300 ceiling instead of 224/225, and the template contradicts the ledger in ADR-011 rev. 7.

- **ID:** QA-07-r1-4
- **Severity / kind:** Minor / defect (log) and test gap
- **Route:** backend
- **Where:** `src/server/tools/protoserve/main.go:298-305` (`plain`), `:289`; `main_test.go:236-249`
- **Relates to:** F-07 design B-3 (request log: "`<bytes>` … 0 for `HEAD`"; 404/405 headers), B-4 (7), (8)
- **Evidence:**
  - Live log line: `HEAD /nope 404 - 10 ae=br`. On HEAD, `net/http` reports the 10 discarded bytes as written, so the log says 10 where B-3 says 0.
  - Mutants that drop `Cache-Control` or `Content-Type` from 404/405 responses survive, because the tests check only status and body (404) or status and `Allow` (405).
- **Consequence:** the log line is the record of transfer sizes, so a HEAD 404 misstates its bytes. This is low impact, since browsers do not send HEAD here. The header contract in the B-3 table is untested for error responses.

- **ID:** QA-07-r1-5
- **Severity / kind:** Minor / risk (design contradiction)
- **Route:** architect
- **Where:** `src/web/index.html:8` (inline `<style>`), prescribed by F-07 design F-1
- **Relates to:** ADR-014 rev. 2 CSP table: `style-src 'self'`; F-22
- **Evidence:** the built `index.html` ships `<style>body{…}</style>`. Under the CSP that ADR-014 fixes, the browser blocks an inline `<style>` element. CSSOM writes such as `style.cssText` in the recorder and ChartStack are still allowed. protoserve sends no CSP, so nothing breaks today.
- **Consequence:** when F-22 adds the header, the body margin and font are dropped. It may also lead F-22 to an unplanned `'unsafe-inline'` or hash exception.
- **Wanted:** the architect decides now:
  - move the rule into the entry CSS (cost to measure against the 300 ceiling);
  - or record a `style-src` hash exception in ADR-014.

### Assessed and accepted (no finding)

Engineer-reported deviations and choices:
- **Port :18090:** design v1.1 adopted it.
- **`precompress.Brotli` returns an error, passed up as `<key>: <err>`:** necessary, because B-3 assumed no error. The message holds only the URL key, and `run` prints `protoserve: …` and exits 1 as B-3 asks for load errors.
- **`make: *** Error 1` on Ctrl-C:** reproduced. It is cosmetic, because protoserve itself prints `stopped` and returns 0. A one-line note in the README or the operator procedure would stop it being read as a failure (preference).
- **Unknown flag → 2:** consistent with the B-3 usage cases.
- **IPv4 only in the URL list:** this is what B-3 says. On this machine the list has 17 lines, mostly Docker bridges (preference: filter `docker*`/`br-*`; no change needed).
- **Hidden entries skipped:** this is what B-3 says.
- **Report `.json` next to `.bin`:** this is what B-1 step 3 says.
  - protoserve therefore also serves `/data/*.json`, and the JSON contains first and last coordinates.
  - For a real file this exposes nothing beyond the `.bin` it sits next to. It is on the home network only and in development only, so it is acceptable.

Other checks:
- `prototype-data.sh` matches B-1 step for step. I probed it with a stub binary: the usage line exits 2; a missing file, `.FIT`, an upper-case name and a failing parse each print the exact `FAIL` line and exit 1; the outputs land only in `<out>`.
  - One cosmetic point: two real files with the same basename overwrite each other but are both counted.
  - QA processed no real FIT file.
- `acceptsBrotli` follows B-3 literally, so it also accepts `br;q=NaN`, which I checked live. Browsers never send that, so there is no finding.

## Previous findings

This is the first F-07 round. Carried in from F-06:

| Finding | Status |
| --- | --- |
| QA-06-r4-1 / F-06 r3-1: the map's pixel ratio is frozen at mount (feature AC 7) | **Fixed in code, verified by test.** `mapPixelRatio` and the re-armed `(resolution: <dpr>dppx)` listener are in the lazy `map-view` only (0 bytes static, +141 lazy). Teardown removes the listener before `map.remove()`, and a partial mount leaves no listener. All five targeted mutants are killed: the cap, the finite check, `next !== applied`, re-arm, teardown. The real-browser part (Firefox zoom, monitor move) is the operator's PD6 and has not been run. |

## Questions

1. **Architect (QA-07-r1-1):** should F-4 step 4 read "if `fingers === 0`, count a no-finger frame"? Any frame without a finger then disqualifies a touch run, as ADR-016 decision 3 says. Saying yes settles it. If you instead want a late start tolerated, ADR-016 decision 3 needs a written exception, together with a rule that keeps the idle frames out of the pass share.
2. **Architect (QA-07-r1-5):** should the body rule go into the bundle CSS, or does ADR-014 get a `style-src` hash? Either answer settles it.
3. **User (already open, not a finding):**
   - OQ-27 (reference phone and conditions) must be answered before a Gate 0 verdict is final (ADR-016 status).
   - OQ-22 to OQ-28 are needed for AC 8.
   - OQ-10 decides the wording that F-07 ships as the default.

## Coverage

| AC | Code | Automated test | Status |
| --- | --- | --- | --- |
| 1 12-hour file loads on the phone with all charts and the map | `Prototype.svelte` (default `ride-12h`, `loadDataFile`, map above charts); `page-query.ts` | `Prototype.dom.test.ts` (1), (5) (with the golden `ride-4h`; reader and charts on `ride-12h` are covered by F-04 and F-05) | **Operator:** PP1, PD1, PD2 |
| 2 Chart touch scrub ≥ 10 s, 60 fps, judged smooth (**gate**) | `measure/recorder.ts`, `stats.ts` | `stats.test.ts`, `recorder.dom.test.ts` (validity hole QA-07-r1-1; split untested QA-07-r1-2) | **Operator:** PP2 ×3, after r1-1 is fixed |
| 3 Route scrub at the same smoothness | as AC 2 (`startedOn` route) | as AC 2 | **Operator:** PP3 ×3 (phone), PD5 (desktop) |
| 4 `ride-4h` `.br` < 500 KB | `prototype-data.sh` budget line | `TestSizeBudget4h` (CI); run: **106,951 PASS** | Verified. PP6 adds the transfer line |
| 5 Bundle < 60 KB | — | `make size` in CI: **45,553 / 60,000**; steps 1–6 reproduced | Verified |
| 6 Bottleneck identified if the gate fails | design "If the gate fails"; the charts/map split in every record | split values not asserted (QA-07-r1-2) | Applies only if the gate fails |
| 7 Map sharp after desktop zoom or monitor move | `map-style.ts` `mapPixelRatio`, `map-view.ts` watcher | `map-style.test.ts`, `map-view.dom.test.ts` (4 new tests) | Code verified. **Operator:** PD6 |
| 8 Metric definitions handed to requirements | — (no code) | — | **User:** OQ-22 to OQ-28 |

Other checks:
- **Design conformance:**
  - B-1 to B-5 match apart from QA-07-r1-4. No silent deviation by the backend engineer was found beyond those listed above.
  - F-1 to F-7 and the ten confirmed engineer choices match.
  - The dependency rules hold: `measure/` is reached only through `import()`, `stats.ts` imports nothing, `prototype/*.ts` imports only `lib/datafile` types, and Prototype has no `style=` or `on…` attributes.
  - Stale comment: `Prototype.svelte:7` cites ADR-011 rev. 6; it should be rev. 7. Trivial; fix it with r1-3.
- **Manual checks and template:**
  - PD1 to PD8 and PP1 to PP8 are complete and executable, and the procedure uses :18090 and the firewall fallback.
  - The template has every table of "What to record" except the B0′ column (QA-07-r1-3).
  - Every record must say "Android: not checked" (D-11); the template does.
- **Pending items:**
  - `src/web/src/App.svelte`, `src/web/src/lib/health.ts` and `health.test.ts` **are still present**. Only they import each other (`App.svelte` → `lib/health`, `health.test.ts` → `health`). Nothing in the app imports them, and they are not in the build manifest.
  - `health.test.ts` is still counted in the 712 tests.
  - `README.md` line 30 still says `main.ts, App.svelte`, and line 28 still calls `index.html` the "dev placeholder page". Both are part of the pending clean-up.
- **Not run:**
  - every operator check (PD1 to PD8, PP1 to PP8), including the phone gate and the desktop pixel-ratio check;
  - a real browser (none on this machine);
  - real FIT files (OQ-29).
