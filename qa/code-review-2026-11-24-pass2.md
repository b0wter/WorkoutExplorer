# Code review 2026-11-24 - pass 2 (whole codebase, areas not covered by pass 1)

## Status

COMPLETE (2026-11-24, main at d2afbe9). Verdict: ready - 0 Blocker, 0 Major, 2 Minor (CR2-01, CR2-02).

## Plan

- [x] A. Read pass 1 (r1-r3) to know covered areas
- [x] B. Go data file encoder (data.bin v8) vs TS decoder parity
- [x] C. Preview image (PNG) rendering
- [x] D. page/format.go vs front-end formatters parity
- [x] E. Front end: charts (uPlot, decimation, cursor/range, F-51, F-67 leg zoom, F-68 dynamics chunk)
- [x] F. Front end: map (MapLibre, hiding, pins F-56, location F-62, satellite F-61, GPX F-57)
- [x] G. Edit page flows (trim, hiding radius F-55, pins dialog, added laps F-65, remove auto-laps F-66, publish, delete, preview map)
- [x] H. Splits F-46/F-64/F-69, summary-explained F-58
- [x] I. i18n F-54, my-workouts F-53, upload page
- [x] J. XSS/HTML injection, token handling (URLs, storage, referrers)
- [x] K. Listener/memory leaks, async load races, budgets
- [x] L. Dockerfile, systemd units, polkit rules, Caddyfile parity with docs
- [x] M. Build and tests (make ci or equivalent)

## Progress

- Done: A (pass 1 read: it covered httpapi, token auth, ingest, hiding/publish, lifecycle, reports/notify, page templates/CSP, CLI, deploy scripts, TS decode bounds); M (`make lint test build size` at d2afbe9: rc 0, 2m53s; 23 Go packages ok; vitest 95 files / 2,502 tests; svelte-check clean; entry index/edit/my-workouts 61,364 / 62,000 gzip PASS; all language files PASS)
- Done: B (encode.go, quantise.go, position.go, decode.go head vs decode.ts/meta.ts/format.ts/prefix.ts: §2 table, sentinels, alignment, delta reader, rule 5-7 checks match; golden.test.ts decodes every Go fixture. No defect.) C (preview.go, map.go: route only from public file, basemap re-encoded, OpenFreeMap style only, attribution matches; no defect) D (format.go vs lib/format: probe of formatFixed 0/1/2, formatElapsed, formatPace on 200,000 values incl. .5/.05/.005 ties: 0 differences; sportName/jsTrim differ only on non-ASCII, sports are FIT enum names)
- Read, no defect: E charts (decimate.ts, axis.ts, MetricChart.svelte, ChartStack.svelte, specs.ts, dynamics chunk, cursor store); F map (map-view.ts whole, location.ts, location-state.ts, satellite style, basemap.ts, basemap-job.ts), pins viewer (src/pins, lib/pins); G edit glue (edit/start.svelte.ts, EditPanel.svelte, fields.ts); H splits (lib/splits, splits/index.ts, lib/range), summary-info, dialog; I i18n core/loader, upload page, lib/zip, lib/mine, mine page; GPX (lib/gpx, gpx/index.ts)
- Also read, no defect: edit/pins.ts (pin dialog), lib/addedlaps + edit/laps.ts (golden parity tests exist), page/start.ts, page/enhance.ts, report/form.ts, report-delete/page.ts, lib/rangelink, impressum/page.ts. HTML sinks swept (grep innerHTML/insertAdjacentHTML/{@html}): only constants, numbers, slot indices and the server's escaped Impressum fragment.
- L deploy files read: Dockerfile (+ .dockerignore allow-list), systemd units (app, caddy, regenerate, slice, upgrade path, notify drop-in), polkit rule, workout-explorer-run, compose.yaml, Podman app/caddy units, Caddyfile; ports, users, groups, sandbox, CSP/Permissions-Policy vs F-22 H-2/ADR-014/TD: consistent. No defect.
- Also read, no defect: RouteMap.svelte, map-props.svelte.ts, ActivityPage/LapsTable (lapsKey re-key), page/laps.ts, lib/sections, sections chunk (F-67 picker), lib/route (route, index), lib/prefs keys, measure (no exits)
- K done: listener add/remove pairs in MetricChart, map-view teardown, location stop, touch-quiet, mapmenu, plus; edit-page relaps keeps old splits/laps-enhancer subscriptions (documented shortcut in edit/start.svelte.ts, detached DOM only, no wrong behaviour found: LapsTable is re-keyed so the old section's click listener goes with it). Budgets: entry 61,364 / 62,000 (ADR-011 rev. 16 budget), language files and lazy entries PASS. Inline head-script SHA-256 in all built shells = first CSP hash (checked).
- Also done: F-51 compare (lib/compare, splits/chart.ts), CursorReadout, RangePanel, uplot-options, PublishedView, DeleteControl, lib/owner; i18n placeholder-parity test exists and is real; LC-10 source-scan test is real; GPX test asserts no <time>.
- Go preview rendering cost probe done: CR2-01.
- Mirrored TS/Go rules re-checked: text cleaning (cleanText vs ApplyPatch, same Cc range; TS cuts before Go trims, display only), trim/hiding/pin shading mirrors are display-only (server decides), page.Build route/GPX/charts class vs TS presence and presentCharts, CardItems. No defect.
- All areas done.


## Findings

### CR2-01 - Minor - CONFIRMED - Preview card rendering takes seconds on a crafted route and runs inside every locked public build
- **Where:** `src/server/internal/preview/preview.go:256-278` (`stroke`: one 16-gon plus one segment quad per kept point; points are skipped only when closer than `minStepPx` = 1 px to the last kept one), called from `publish.go:428-449` (`card`) in every public build: publish, PATCH of a published workout, preview-map PUT commit (all under O-7, ADR-061) and `regenerate`.
- **Defect:** rasterisation cost grows with the summed on-screen length of all route segments, with no cap on point count or drawn length. F-23 (Consequences) budgets "one PNG encode (tens of ms)"; ADR-017's stop rules bound parse and Accept but nothing bounds the build's card.
- **Evidence (probe):** scratch `review2/mod/internal/preview/probe_cost_test.go`, a synthetic workout encoded and decoded through `datafile` (not through ingest; ingest has no position-jump filter, so a FIT file with these positions reaches the build unchanged), zigzag positions inside a ~5 km box so every sample lands on a new pixel. Baseline (100 samples): `Render` 45 ms, `RenderOnMap` 205 ms. 43,000 samples: 0.24 s and 0.59 s. 500,000 samples (the MaxSamples limit): 2.50 s and 5.84 s (two runs: 2.43/5.88 s). Real routes (metres between samples) are far cheaper because of the 1 px skip.
- **Failure scenario:** an uploader with a crafted 500,000-sample file and a stored basemap makes each PATCH of the published workout hold O-7 for ~6 s plus parse and encode. ADR-061 accepts "K writes queued on O-7" as a residual but assumed a build of parse + tens of ms. This multiplies the delay to other owners' saves and publishes per queued write. It also adds 2.4 to 8 s per such workout to every `regenerate` on deploy. No privacy or data-loss effect.
- **Route:** backend (simplify the projected polyline before stroking, e.g. drop points within the stroke width or cap points per pixel column; add a worst-case card to the ADR-017 all-caps benchmark).

### CR2-02 - Minor - CONFIRMED - RUNBOOK's header list for proxy operators omits Permissions-Policy
- **Where:** `deploy/RUNBOOK.md:149` vs `deploy/Caddyfile:53` (`(security)` snippet).
- **Defect:** the runbook tells hosters which response headers the app sets itself (CSP, Referrer-Policy, X-Content-Type-Options, X-Robots-Tag, HSTS) and why a proxy must not add its own; since F-62 (LC-11) Caddy also sends `Permissions-Policy: geolocation=(self)`, which the list leaves out. No other deploy doc lists the headers.
- **Failure scenario:** a hoster whose proxy adds a site-wide hardening header such as `Permissions-Policy: geolocation=()` (a common nginx/Caddy snippet) reads the list, sees no conflict, and the viewing page's "Show my location" (AP-25) fails for every viewer with the denied message. The general "do not add headers" rule covers it, but the explicit list suggests the others are free.
- **Route:** backend (one line in RUNBOOK.md: add `Permissions-Policy` to the list).

## Coverage

- **Run:** `make lint test build size` at d2afbe9 (rc 0, 2 m 53 s): gofmt and `go vet -tags caddycheck` clean, 23 Go packages ok, svelte-check clean, vitest 95 files / 2,502 tests, entry 61,364 / 62,000 gzip, every language file and lazy entry PASS, upload-entry storage check PASS. Probes: formatter parity (Go `page/format.go` copied vs the JS rules, 200,000 values, 0 differences); inline head-script SHA-256 of every built shell vs the CSP; preview render cost (CR2-01).
- **Not run:** `make caddy-check`, `docker-check`, `update-check` (external image pulls), fuzzing, `go test -race` (no concurrency code in this pass's scope; pass 1 and r3 ran it), browser runs on real devices, `make corpus` on real files (never read).
- **Not covered in depth:** zones (F-49) and efforts (F-50) chunks beyond the HTML-sink sweep, lib/colouring, the chart and route gesture reducers (pure, read only for wiring), MapLibre internals.

## Notes for continuing

- Review complete. Unverified leads, not reported (no reachable path found):
  - `datafile.Encode` does not enforce TS rule 7 for `moving_time` / `elevation_gain` (never missing), and Go `Decode` does not check it either, so publish's re-decode would not catch a NaN there; Normalise and trim never produce one today (W-1 risk only).
  - Edit page: after a delete, `showDeleted` replaces `body` without destroying the MapLibre instance or the chart stores (detached; no requests seen in code); harmless, page is done.
  - Title: TS `patchOf` cuts to 100 code points before Go trims (UX only).
- Probe module copy: review2/mod (src/server without testdata); probe file `internal/preview/probe_cost_test.go`; formatter probe in review2/fmt.
- Scratch dir: /tmp/claude-1000/-home-b0wter-syncthing-work-workout-explorer/1973ef5e-b065-4f5d-9058-f861d81899dd/scratchpad/review2/
