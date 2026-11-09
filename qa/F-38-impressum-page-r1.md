# QA F-38 Impressum page - round 1

- **Date:** 2026-11-09
- **Verdict:** ready. No Blocker or Major. Three Minors: a missing `site/` on upgrade breaks the site in the middle of a deploy (documented, not checked by the script); the 1,000-key cap has no test on the Impressum handler; the design doc is still marked "draft".
- **Inputs:** `features/F-38-impressum-page.md` (15 ACs), spec INF-7, `architect/F-38-impressum-page.md` v0.1 (IM-1 to IM-9), ADR-043, F-23 PC-11 / AC on `robots.txt`; commits 9f1c8ee (backend) and 351dbd3 (frontend), clean tree.

## Build and test results

- `make ci`: exit 0. Go: every package `ok`, including `internal/shell` and `cmd/workout-explorer`. Vitest: 63 files, 1,604 tests passed. Size: `impressum.html` entry 907 / 2,000 bytes gzip PASS, app bundle within 60 KB. `caddy-check` (`TestCaddy*` with Caddy 2.11.7) passed. `docker-check: ok`.
- `go test -race -count=1 ./internal/httpapi ./internal/shell ./internal/about ./cmd/workout-explorer`: all `ok`.
- Probe through a `go test -overlay` scratch file, so `src/` was not changed: with 1,000 IPv4 addresses tracked, a new address gets `429`, `Retry-After: 60`, a JSON body and no text. 30 requests from one IPv6 /64, then the 31st from another address in the same /64 gets `429`; a different /64 gets `200`.

## Findings

### Minor

- **ID:** QA-38-r1-1
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `deploy/deploy.sh:70-74` (step 3, `docker compose up -d`)
- **Relates to:** ADR-043 consequences, F-31 DP-10
- **Evidence:** The new Caddy mount (`compose.yaml`, `create_host_path: false`) requires `{WX_DATA}/site` to exist. `deploy.sh` does not check for it. It has already copied the binary, shells, `compose.yaml` and Caddyfile when `docker compose up -d` recreates Caddy. The deploy user cannot create the folder: it belongs to 10001 and needs sudo. The RUNBOOK documents the manual step in sections 3 and 13.2 and in troubleshooting, and `features/README.md` lists it under "Waiting on".
- **Consequence:** If the operator misses this step, Caddy cannot be recreated and the site goes down partway through the deploy. The operator does not get a clean refusal before anything is copied. A preflight check in step 1, for example `r "[ -d \"\${WX_DATA:-/srv/workouts}/site\" ]"` with `.env` taken into account, would turn the outage into an exit 1. Accept this as documented, or add the check.

- **ID:** QA-38-r1-2
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/internal/httpapi/impressum_test.go`; `impressum.go:24` (`maxImpressumKeys`)
- **Relates to:** AC 6 (second sentence), IM-6
- **Evidence:** `TestImpressumAPI` and `TestCaddyImpressum` cover the 31st request, another address and the end of the window. The full-table refusal is tested only on the generic limiter (`upload_test.go`), not through `newImpressumHandler`. Changing `maxImpressumKeys` or passing `maxRateKeys` instead would fail no test. My probe shows the current behaviour is correct.
- **Consequence:** A regression in the AC 6 cap would go unnoticed. Add a test that tracks 1,000 addresses and checks that a new one gets `429` with no text.

- **ID:** QA-38-r1-3
- **Severity / kind:** Minor / docs
- **Route:** architect
- **Where:** `architect/F-38-impressum-page.md:4`, `architect/INDEX.md:52`
- **Relates to:** CLAUDE.md "live documents hold the current state"
- **Evidence:** The status is still "draft" v0.1, although the design is built and the user accepted its decisions on 9 Nov 2026.
- **Consequence:** A reader cannot tell whether IM-1 to IM-9 are settled. Mark the design accepted.

## Previous findings

None (round 1).

## Questions

None that block. The legal adequacy of a JavaScript-only, click-to-load Impressum is listed as accepted in the feature decisions.

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 unset: not-found, no link | `httpapi.go` route only when set; `shell.Write` removes `impressum.html`; `Fill` gives "" | `TestImpressumAPI` "unset" (same bytes and headers as `/api/nothing-here`); `TestImpressumFooterAndAPI` unset half; `caddy_test.go:336-337` |
| 2 headings, lists, literal `<b>` `<script>` `{{` | `about.ParseOperator` reused (IM-1) | parser tests in `about`; `TestImpressumSetting` (`&lt;b&gt;`); `page.dom.test.ts` inserts the fragment, no `<b>` element. The `{{` case relies on the existing AB-8 tests (the body is never passed through a template) |
| 3 no text in any HTML | shells and template hold only the marker | marker string over every page type, through Go (`TestImpressumFooterAndAPI`) and through Caddy (`TestCaddyImpressum`, 9 paths) |
| 4 no request on load, one per click | `page.ts` (button disabled during fetch) | `page.dom.test.ts`: no fetch on load, exact options, double click gives 1 request |
| 5 same-origin check, same refusal bytes | `impressum.go:41-48`, checked before the limiter | 8 header variants x 31 requests, byte-identical code, body and headers; Caddy: curl, cross-site, other Origin and Referer |
| 6 30/h, per /64, 1,000 cap fail closed | `limiter` own instance | 31st request, other address, after the hour (Go and Caddy); /64 and cap only by my probe (r1-2) |
| 7 noscript | `impressum.html` `<noscript>`, button `hidden` | `page.dom.test.ts` |
| 8 failure message, button again | `page.ts` catch | 429, 404 and network error, then a retry that succeeds |
| 9 selectable text | no `user-select` in the CSS | manual only (user check) |
| 10 X-Robots-Tag, meta | Caddy `security` snippet; meta in the shell | `r.check` headers; meta asserted |
| 11 robots.txt exact, `/w/` allowed | Caddyfile `respond` | exact body and Content-Type, set and unset (`caddy_test.go:332`) |
| 12 footer link on every page type, gone after unset, restart and regenerate | `shell.Fill` in shells, About, `LoadTemplate`; `lib/page/impressum.ts` for footers drawn in the browser | Go and Caddy tests over upload, generic, draft, edit, published, report-delete, About, published page, Impressum; `edit.dom.test.ts` page footer, published view, not-found view and the deleted screen (set, unset, raw marker) |
| 13 bad file gives exit 2 in serve and regenerate, nothing touched | `impressumBlock` in `readSettings` and `regenerateCmd` | `TestImpressumSetting`: relative, missing, directory, unreadable, >16 KiB, bad UTF-8, control character, empty, blank; data directory not created |
| 14 no cookie, CSP unchanged, theme | CSP not changed (`connect-src 'self'`); `plain.css` `prefers-color-scheme` in a CSS file, so it works without JS | CSP and inline-script check; the theme is checked manually (user check) |
| 15 no real Impressum in the repository | only `deploy/impressum.example.txt` with bracketed prompts | `git ls-files` checked |

Also checked and correct:

- **Logs and headers:** no log line on any outcome (test with a JSON logger). Responses carry `no-store` and `text/html; charset=utf-8`. Refusals happen before the limiter, so they cost no limiter entry.
- **Shells:** `shell.Write` writes through a temporary file and a rename, with mode 0644 and `site/` at 0750. It runs before `store.Open` and the listener. A missing marker makes it exit 1. `/impressum.html` and `/impressum/` give `404`.
- **Caddy:** only `/assets/*` still comes from `WX_WEB`; every shell route comes from `{WX_DATA}/site`. The read-only `site/` mount is in both `compose.yaml` and `compose.build.yaml`.
- **Deploy:** `deploy.sh` copies all five shells before restarting `app`.
- **Runtime state:** the storage count and startup recovery (`publish` and `privatedir` walks) only read `public/`, `private/` and `tmp/`, so `site/` is unaffected. No third-party request.

Not run: a real browser, so `Sec-Fetch-Site` as sent by actual browsers, AC 9 and AC 14 (theme with JavaScript off) are not verified. No VPS deploy.

**User checks:**
1. Create `/srv/workouts/site` before the next deploy.
2. In a desktop browser and on a phone, open `/impressum` with DevTools: no request on load, one request per click, the text can be copied, light and dark theme, and the note with JavaScript off.
3. `curl https://<site>/api/impressum` gives `404`.
