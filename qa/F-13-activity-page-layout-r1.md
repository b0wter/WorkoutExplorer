# QA F-13 Activity page layout and template - round 1

- **Date:** 2026-10-24
- **Verdict:** ready with conditions. No Blocker or Major finding. The one condition is AC 7 (Lighthouse LCP, paint order, layout shift): it has not been run and waits for the user's Chromium machine (OQ-64), so it is listed as open and not as a finding.
- **Inputs:** `features/F-13-activity-page-layout.md`; `architect/F-13-activity-page-layout.md` (v1.0, D-1 to D-16, B-1); ADR-026, with ADR-011 (budget), ADR-012 (jsdom tests) and ADR-014 (CSP, referrer); decision log of 24 Oct 2026 in `features/README.md` (OQ-61 to OQ-64, no inline CSS, no hydration); spec AP-1, AP-2, AP-3, UP-7, SH-6, SH-9, INF-1, INF-2. Code at `6cf4f9b` (commits `dcaaf64`, `15d89f0`, `6cf4f9b`). The working tree differs only in `features/README.md`, which is not code. Spot checks against the running protoserve on port 8095.

## Build and test results

- `make ci`: exit 0. `go vet ./...` clean. `svelte-check`: 279 files, 0 errors, 0 warnings. `go test ./...`: all packages ok. `vitest run`: 36 files, 959 tests passed. Bundle: `entry index.html: 46487 / 60000 bytes gzip PASS`. F-13's share is 46,487 − 45,582 = 905 bytes, within its 5,000 (ADR-011). Upload entry 2,272 / 8,000 PASS.
- `go test -count=1 -race ./tools/protoserve/`: ok, 1.3 s.
- protoserve: `/` and `/index.html` return 404. `/w/ride-12h` is 7,035 bytes gzip, against the 15,000 limit (AC 1). `/w/ride-30min` sends `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff` and `Cache-Control: no-store`.
- `build/web/index.html`: no `<style>` and no `style=`. Vite added only the module script and the stylesheet link.
- Noise, not caused by F-13: npm's engine warning (Node 22 against `>=24`), `npm audit` reporting 1 high-severity issue (no F-13 commit touches `package.json` or the lock file), and jsdom's "navigation to another Document" message from the test that clicks an absolute link.

## Findings

No Blocker, no Major.

### Minor

- **ID:** QA-13-r1-1
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/page/start.ts`, `showLoadError` (it picks the first of `#map-slot`, `#charts-slot`, `#range-slot`)
- **Relates to:** AC 9, D-13 ("The map and chart area show 'The workout data could not be loaded.'")
- **Evidence:** I read the code and `page.dom.test.ts` "404: the exact text …". When the workout has a route, the text goes into the map slot only. `#charts-slot` keeps its reserved height and stays empty. On a phone, `ride-30min` / `hostile-text` (`charts-6 cells-8`) reserve 4 × 20 − 2 + 892 = 970 px.
- **Consequence:** If a data file fails to load, a viewer sees the grey map box with the error text and then about 1,000 px of blank page before the laps table. Nothing is wrong with the data, but the page looks broken and D-13's wording is only half followed. Possible fixes: put the text in the chart area as well, or drop the reserved height once the load has failed, since nothing can shift any more at that point.

- **ID:** QA-13-r1-2
- **Severity / kind:** Minor / risk
- **Route:** frontend
- **Where:** `src/web/src/components/page/SummaryStrip.svelte` (`{item.Value}{#if item.Unit}&nbsp;<span class="u">`) compared with `src/web/index.html` (`{{.Value}}{{if .Unit}} <span class="u">`)
- **Relates to:** ADR-026 Consequences (the template is the reference, and the two renderers must not drift), D-9
- **Evidence:** I read both files. The Svelte component puts U+00A0 between value and unit; the template puts a normal space.
- **Consequence:** The edit page (F-19) produces different markup from the viewing page. In narrow cells the template may wrap the unit onto its own line where the component does not. This is small now, but it is exactly the drift ADR-026 says only F-19 QA would catch. Use the same separator in both renderers.

- **ID:** QA-13-r1-3
- **Severity / kind:** Minor / preference
- **Route:** backend
- **Where:** `src/server/tools/protoserve/samples.go`, `gzipSize` (`zw, _ :=`, `_, _ = zw.Write`, `_ = zw.Close()`)
- **Relates to:** AC 1 (the size figure in the listing)
- **Evidence:** I read the code. The errors cannot occur with a constant level and a `bytes.Buffer`, but they are discarded silently, and this function produces the AC 1 figure.
- **Consequence:** None today. Return the error or add a comment saying why it cannot happen, so the next reader does not have to work it out.

## Previous findings

None. This is round 1.

## Questions

None that block. AC 7 is open by decision (OQ-64). It is settled by the Lighthouse run in the design's Budgets table (mobile and desktop presets, `/w/ride-12h`, CLS 0, LCP < 2.5 s, summary painted before the charts in the filmstrip), recorded in `bench/MEASUREMENTS.md`.

## Coverage

| AC | Code | Test / check | Status |
| --- | --- | --- | --- |
| 1 Size, header, summary and notice without JS; "Expires around" in the workout's zone (UTC when it has none); "Expires in …" the same in UTC−5 and UTC+9 | `index.html`, `lib/page/build.ts` (`expiryEstimate`), `lib/format` (`formatTimeLeft`), `page/enhance.ts` | Go `TestSamplePages` (content and gzip ≤ 15,000 for every sample); `ActivityPage.dom.test` AC 1; `page.test` expiry estimate; `enhance.dom.test` (NY, Tokyo, UTC). protoserve: ride-12h 7,035 B gzip; `indoor-unknown-zone` shows "05:00 UTC" and a UTC estimate | traced. `formatTimeLeft` works on milliseconds only, so the time-zone loop cannot fail; it is correct by construction. The no-JS view was checked by the user on an iPhone, not by me |
| 2 UTC+2 ride shows 07:30 to a UTC−5 viewer | `formatLocalStart` (UTC getters on the shifted instant) | `page.test` AC 2 (asserts that the process really runs at offset 300) | traced |
| 3 No power on a run; no HR or "0 bpm" without HR | `summaryItems`, `lapsGroup` | `page.test`, `ActivityPage.dom.test`, Go test. protoserve: `run-30min` has no power cell or column; `ride-no-hr` has no HR cell or column | traced |
| 4 Notice identical with and without hiding | literal in the template and in `SummaryStrip` (D-15) | Go test and dom test over all samples | traced |
| 5 HTML and `<script>` shown as literal text | `html/template` contextual escaping; no `{@html}` | Go test (escaped forms present, raw forms absent); `hygiene.test`. protoserve `hostile-text`: title, `<title>` and description all escaped, line breaks kept | traced |
| 6 Copy link copies the viewing URL, never a token | `enhanceCopyLink`, `viewingUrl` (origin + root-relative href, query and fragment cut; `//` and absolute URLs refused) | `enhance.dom.test` (on `/edit/abc#SECRET-TOKEN`, a missing or refusing Clipboard API shows the field) | traced; the field layout on the phone was checked by the user after `6cf4f9b` |
| 7 LCP < 2.5 s, summary before charts, no shift | reserved slots in `page.css` (D-10); `hygiene.test` checks that `--area` equals `chartAreaHeight` and the readout row counts | not run | **open** (OQ-64) |
| 8 Indoor: hint instead of map; charts and summary render | `Route.NoLocation`, `ChartsClass` | `page.test`, `ActivityPage.dom.test`, `page.dom.test`, Go test; protoserve `pool-swim`, `indoor-unknown-zone` | traced |
| 9 Load failure text; "Reload the page." for a version mismatch; no technical detail | `start.ts` `showLoadError`, `load-error.ts` | `page.dom.test` (404, version patched to 2, indoor) | traced; see r1-1 |
| 10 Footer: About and Report | template footer, `PageFooter.svelte` | Go test, `ActivityPage.dom.test` | traced. The Report link is a `#report` placeholder until F-28 |

Decision log, 24 Oct 2026:

- **Maximums** (OQ-61) are in the same cell as the average for speed or pace, HR, power, cadence and temperature. When only a maximum exists, it is shown with the label "Max …" (triathlon: "Max speed").
- **Expiry rule** (OQ-62) is as described under AC 1.
- **No inline CSS or script:** the template, the built template and `hygiene.test` all comply. protoserve sends no CSP, so the user's phone check did not exercise CSP. The chart islands write styles only through CSSOM (`style.cssText`), which `style-src 'self'` allows.
- **No hydration:** only `mount()` into empty slots; `page.dom.test` "adoption, not hydration" shows that the server-rendered nodes and their text stay unchanged.
- **Page texts** were accepted by the user on the phone.

Also checked and found correct:

- module order;
- the preload `crossorigin` matching `fetch`'s default credentials, so the file downloads once;
- the data file URL is read from the page and never built;
- `?data=` is removed;
- the viewing entry does not import `components/page`;
- samples are regenerated and compared in CI;
- the head has `noindex`/`noarchive` and the referrer meta;
- `missingkey=error` is tested.

Not run: AC 7 (Lighthouse). Real-device behaviour of the clipboard and fallback field on HTTPS.
