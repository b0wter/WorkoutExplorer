# Browser UI suite

Playwright tests that drive the real pages (upload, viewing, edit) in real browsers. They are **not** part of `make ci`
(a full run takes a long time); QA and engineers run them with `make ui-test`. Unit and DOM tests (Vitest) stay next to their
modules in `src/web/src`; tests that span whole pages live here.

## Run

```sh
make ui-test                                  # everything, all projects
make ui-test PROJECT=iPhone                   # one project
make ui-test UI_ARGS='-g F-73'                # Playwright arguments (grep a title, a file name, --retries=0 ...)
make ui-test PROJECT=chromium UI_ARGS=f74     # one file
```

What it does (`run.sh`): builds the web bundle and the server into `build/ui-test/` (never `build/web` or `build/data`, so a dev
server keeps running), generates the suite's own synthetic FIT files (`corpusgen -e2e`), starts its own Go server and Caddy on two
free ports (never 18080/18081), uploads and publishes the shared fixtures once, and runs Playwright in the Docker image
`mcr.microsoft.com/playwright:v1.49.1-noble` (browsers included, `--network host`). Needs Docker, Go, Node, and the pinned Caddy
(`make caddy` runs as a dependency). Logs: `build/ui-test/server.log`, `caddy.log`; results, traces and screenshots of failures:
`build/ui-test/results/`; the HTML report: `build/ui-test/html` (`npx playwright show-report build/ui-test/html`).
`UI_SKIP_BUILD=1 sh src/web/e2e/run.sh "$PWD/build/tools/caddy-2.11/caddy" ...` reuses the last build while you write a test.

`@playwright/test` is pinned to 1.49.1 in `package.json` (its own package: `src/web`'s install, type check and bundle budget never
see it) and must equal the Docker image's version.

## Projects

| Project | Browser | Emulation |
| --- | --- | --- |
| `chromium`, `webkit`, `firefox` | desktop | 1440 x 900 |
| `iPhone` | WebKit | iPhone 13: 390 px, touch, DPR 3 |
| `Android` | Chromium | Pixel 7 |
| `iPad` | WebKit | iPad Mini (the small-tablet case where the header controls wrapped) |
| `firefox-phone` | Firefox | 390 x 844 viewport only (Firefox has no touch or mobile emulation) |

A layout-specific test restricts itself: `desktopOnly(test, why)` (helpers.mjs) skips the phone and tablet projects,
`test.skip(isMobile, why)` skips touch projects. Headless Firefox has no WebGL, so the map never starts there; `open()` ignores that error.
`open()` also aborts every non-local request (tile host, fonts) for the whole context and lists them in `errors.blocked`: tests never reach an outside host.

## Files

| File | Covers |
| --- | --- |
| `f73-split.pw.mjs` | F-73 remembered map split, first frame, invalid values, blocked storage, JS off |
| `f74-spike.pw.mjs` | F-74 spike removed, kept glitch clips the axis, AP-24 line in four languages |
| `f71-compare.pw.mjs` | F-71 laps and splits comparison chart |
| `f72-yards.pw.mjs` | F-72 yards for swims, live unit switch |
| `laps.pw.mjs` | F-18, F-46, F-69, F-48, F-64 laps and splits table: switch, presets, triggers, strokes, leg tables, row click |
| `edit.pw.mjs` | F-19, F-45, F-21 edit page of a draft: fields save, suggested title, lifetime, expiry on the view page, token only in the fragment |
| `publish.pw.mjs` | F-20, F-12 publish, published page, Share and Copy link, viewing page, edit link after publishing, Delete and the gone page |
| `hiding.pw.mjs` | F-16, F-55, F-47 start/end hiding radii and trim: the published data file holds no position inside a radius or trimmed part, totals, fully hidden route |
| `pins-laps.pw.mjs` | F-56, F-65, F-66 edit page: add, edit, delete pins and the pin dialog, add and remove lap boundaries (table and comparison chart follow), Remove/Restore auto-laps |
| `prefs.pw.mjs` | F-14, F-54, F-41 units (pace, speed, /100 m), language and theme switch live with no data or API request, tint cards in both themes, choices in browser storage only |
| `legs.pw.mjs` | F-67 leg switch clears the range, charts stay aligned in every leg |
| `dialogs.pw.mjs` | dialog shell: fixed header, scrolling body, scroll lock, close |
| `layout.pw.mjs` | header wrap, readout labels, summary cards in en/de/es/fr, several widths |
| `help-share.pw.mjs` | F-39, F-58, F-40 how-to bar (shows, x dismisses, stays dismissed), "How these numbers are computed" dialog, range link opens the range, Copy link to this selection round trip |
| `info-pages.pw.mjs` | F-29, F-38, F-28, F-37 About attributions, no Impressum (bare 404), generic expired page, Report form (validation, keeps text, submit answer), report-delete page with an invalid key |
| `mine.pw.mjs` | F-53 My workouts: empty state, a UI upload is listed, survives a reload, Forget empties it |
| `upload.pw.mjs` | upload page: reserved drop zone, paused, fail-open, refusal, one real upload |
| `helpers.mjs`, `fixtures.mjs`, `setup.mjs` | helpers, fixture list, global setup |

`BACKLOG.md` lists the features that still have no UI test, in small pieces.

## Add a test

1. Name the file `<feature>.pw.mjs` (only that suffix is picked up) and put the feature ID in the test titles.
2. Data: synthetic FIT files only. Read-only tests use `workout('<name>')` from `helpers.mjs` (uploaded and published once in
   `setup.mjs`; names in `fixtures.mjs`). Tests that change a workout (edit, trim, publish, delete) call `upload(request, name)`
   for their own copy. A new file shape goes into `src/server/tools/corpusgen/e2e.go` (not into the parser corpus) and into
   `FIXTURES`.
3. `open(page, url, { storage })` presets `localStorage` (`wx.lang`, `wx.units`, `wx.theme`, `wx.split.*`) before any script runs and
   waits for the charts. Use `hover`, `scan` (aim a cursor at a sample), `drag` (mouse), `texts`.
4. Assert on DOM text, geometry or canvas pixels, not on screenshots. No `page.waitForFunction` with a string (the page's CSP blocks
   it); no fixed sleeps beyond a frame, use `expect(...)`/`expect.poll`.
5. Run it on every project that applies: `make ui-test UI_ARGS=<file>`. Mark flaky tests with `test.fixme` and a BACKLOG row, never
   leave them red.

The phone projects use touch: a tap does not scrub like a swipe, and a chart pixel is several seconds wide, so single-sample
checks use the mouse projects.

## Unattended run with a report

`report.sh` runs the whole suite on a clean snapshot of `main` (a temporary detached worktree, removed afterwards; the repository's
`build/` and the dev ports are untouched) and writes a dated report outside the repository.

```sh
sh src/web/e2e/report.sh            # main
sh src/web/e2e/report.sh HEAD       # another ref
UI_REPORT_DIR=/some/dir UI_REPORT_KEEP=7 sh src/web/e2e/report.sh
```

Default directory: `${XDG_STATE_HOME:-~/.local/state}/workout-explorer/ui-reports/YYYY-MM-DD_HHMM/` holding `SUMMARY.md` (commit,
start, end, duration, passed/failed/flaky/skipped per project, failures with project and first error line), `html/` (the Playwright
report), `artifacts/` (traces, screenshots of failures), `run.log`, `report.json`. The newest 14 reports are kept. Exit code: 0 all
passed, 1 failures, 2 setup error (Docker or git missing, build failed). A lock directory keeps two runs from overlapping.

systemd user timer (`~/.config/systemd/user/wx-ui-report.service` and `.timer`):

```ini
# wx-ui-report.service
[Unit]
Description=Workout Explorer UI test report

[Service]
Type=oneshot
WorkingDirectory=/path/to/workout_explorer
Environment=PATH=/usr/local/bin:/usr/bin:/bin:%h/go/bin
ExecStart=/bin/sh /path/to/workout_explorer/src/web/e2e/report.sh
TimeoutStartSec=3h

# wx-ui-report.timer
[Unit]
Description=Workout Explorer UI test report, mornings

[Timer]
OnCalendar=*-*-* 07:30
Persistent=true

[Install]
WantedBy=timers.target
```

`systemctl --user daemon-reload && systemctl --user enable --now wx-ui-report.timer`; the user's Docker access must work without a
login shell (user in the `docker` group, or rootless Docker with `DOCKER_HOST` set in the unit). Crontab line:

```
30 7 * * * cd /path/to/workout_explorer && PATH=/usr/local/bin:/usr/bin:/bin sh src/web/e2e/report.sh >/dev/null 2>&1
```

Notes: cron and systemd have a minimal `PATH`; it needs `git docker make node go npm curl`. The run uses the repository only through
`git worktree add`, so merges in the main tree while it runs change nothing. Plan about an hour of CPU; the Docker image is pulled on
first use.
