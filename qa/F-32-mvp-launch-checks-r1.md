# QA F-32 MVP launch checks - round 1

- **Date:** 2026-11-05
- **Verdict:** ready with conditions. Nothing a machine can check fails. The conditions are the user checks below: devices, browsers, messengers, live timings, the operator file (the live About page still shows `[OPERATOR DETAILS: ...]`), the data-processing record and gate 1.
- **Inputs:** `features/F-32-mvp-launch-checks.md`; spec NFR Performance, Compatibility, Privacy, Security, Reliability, SH-5, SH-10, SH-11, UP-2, UP-7, AC-4; `deploy/RUNBOOK.md`, `deploy/Caddyfile`, `architect/F-22-caching-and-security-headers.md` (H-11, H-12), ADR-016. Commit 133d04b (v0.3.0 candidate, not tagged). The working tree differed only in `features/README.md` (the main session's status edit), so the binary reports `133d04b-dirty`.

## Build and test results

- `make -k ci`: **exit 0**. Go: every package `ok`. Vitest: 58 files, 1,443 tests passed. Bundle: entry 57,399 / 60,000 B gzip PASS; upload entry PASS. `caddy-check` (pinned Caddy 2.11.7): `ok`. `docker-check: ok`.
- **Local stack** (the runbook's "Local run" form, without Docker): `build/workout-explorer serve -addr 127.0.0.1:18180 -data <scratch>/data -web build/web` with `WORKOUT_EXPLORER_BASE_URL=http://127.0.0.1:18188` and `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4=240` (so the 49 uploads stay below the limit). In front of it, the pinned `build/tools/caddy-2.11.7/caddy` with `deploy/Caddyfile`, `WX_SITE=http://127.0.0.1:18188`, `WX_API=127.0.0.1:18180`. The Compose stack was not started; `docker-check` builds the image only.
- Probe scripts (scratch only, not committed): `e2e.py` (corpus sweep), `timing.py` (SH-10), a Vitest probe that runs `src/web/src/lib/datafile/decode.ts` on every downloaded data file.

## Results per acceptance criterion

| # | Acceptance criterion | Result |
| --- | --- | --- |
| 1 | Every corpus file uploads, publishes and views; non-activity and garbage refused with their messages | **Pass, checked locally** (details below). Browser rendering of the page was not run (no browser here); it is covered by the user's browser matrix (AC 6) |
| 2 | LCP < 2.5 s, reference phone, 4G, 4-hour ride | **User check** U-2 |
| 3 | 60 fps scrubbing, 12-hour file, operator's phone, live | **User check** U-3 |
| 4 | Android: page loads and scrubs smoothly | **User check** U-4 |
| 5 | Android Chrome upload by file picker opens the edit link | **User check** U-4 |
| 6 | Browser matrix | **User check** U-5 |
| 7 | Preview cards in WhatsApp, Signal, Slack, Telegram, X; drafts show no route or numbers | **Draft part: pass, checked locally.** Messenger cards: **user check** U-6 |
| 8 | Delete, expiry, hiding on stop old content within 60 s, live | **Pass locally** (timings below); live run: **user check** U-7 |
| 9 | Data-processing record exists | **User check** U-8 |
| 10 | Operator file answers every prompt, including the proxy's logging | **User check** U-9. Live `/about` (GET, 2026-11-05) still shows `[OPERATOR DETAILS: who runs this site, ...]` |
| 11 | Gate 1 starts | **User check** U-10 |

### AC 1: corpus sweep (checked locally)

For each of the 49 files in `testdata/corpus/synthetic/`, the expected outcome came from its golden report in `expected/synthetic/`.

- **41 activity files** (including `indoor-no-gps`, `indoor-no-distance-no-gps`, `gps-dropout`, `pause-hours`, `brick-run-bike`, `triathlon`, `chained-*`, every truncated and bad-CRC variant, `ride-4h`, `ride-12h`). For each one the probe did: upload 201; draft `/w/{id}` 404 generic page; owner GET 200; `draft.bin` 200 with magic `WEXD`; `/edit/{id}` 200; PATCH title, two-line description and `hide_ends: true` 200; publish 200. Then `/w/{id}` 200 `no-cache`. Its `#wx-data` URL answered 200 `Content-Encoding: br` with `private, max-age=31536000, immutable`, and every `/assets/` file the page names answered 200. The `og:image` was a 200 PNG. The owner's `original.fit` was 200 with a `.FIT` header. Then DELETE 204, page and data URL 404 straight after, no DB row, and no file or folder named by the public or internal ID left under the data directory. **0 failures.**
- **Decoded with the front end's own reader:** all 82 data files fetched (draft and published, 41 each) went through `decodeDataFile`. All 82 returned `ok`, none in an error state.
- **8 refused files**, all with HTTP 422, each giving its golden report's message: `course`, `monitoring` and `workout-plan` get "Unsupported FIT file type."; `empty`, `not-fit-png` and `not-fit-random` get "This file is not a FIT file."; `no-records` and `ride-30min-header-only` get "This file contains no readable workout data."

### Data file size (NFR Performance, checked locally)

| File | Raw | Served (`.br` sidecar) | `brotli -q 11` | Budget |
| --- | --- | --- | --- | --- |
| `ride-4h` | 511,560 B | 109,438 B | 95,337 B | 500 KB: **pass** |
| `ride-12h` | 1,522,444 B | 268,454 B | 215,237 B | (no budget; under 500 KB anyway) |

### Privacy (checked locally)

- **Cookies:** none. 828 requests in the sweep plus a route sweep (`/`, `/about`, `/edit/`, `/published/`, `/w/`, `/api/status`, `/api/health`, owner API, 404s) carried 0 `Set-Cookie` headers.
- **Headers:** every response had exactly one value each of `Content-Security-Policy`, `Referrer-Policy`, `X-Content-Type-Options`, `Strict-Transport-Security` and `X-Robots-Tag`, equal to `deploy/Caddyfile`'s `(security)` block, in 0 of 828 cases different. The inline theme script on the workout and edit pages hashes to `sha256-z5SjeUWa...O8bY=`, the hash the CSP allows. No page loads anything from another origin. Live site (read-only GETs of `/`, `/about`, `/w/<unknown>`, `/api/health`, `/api/status`, `/edit/<x>`): the same CSP string, one value of each header, no `Set-Cookie`.
- **Originals unreachable:** each of these answered 404: `/w/{id}/original.fit`; the owner endpoint without the token; `/private/{internal id}/original.fit`; `/workouts.db`; `/public.lock`; `/tmp/`; `/public/{id}/index.html`; `/index.html`, `/about.html`, `/edit.html`, `/generic.html`, `/upload.html`; and `%2e%2e` and `..%2f` traversals below `/w/{id}/`.
- **Logs:** `app.log` (154 lines) and `caddy.log` (14 lines, start-up only: no access log) hold none of the 41 edit tokens (`grep -F -f`) and no coordinate fields. The only keys are counts, outcomes, device fields and the internal row ID. There is no public ID, no link and no client address.
- **After delete and expiry:** at the end the data directory held only an empty `private/`, an empty `public/`, an empty `tmp/`, `public.lock` and the database. The database had 0 workouts and 0 reports, and after shutdown neither it nor its WAL contains any test title or summary (`secure_delete` is on).
- **Expiry:** the binary has no clock seam (`expiryEvery` is for tests only). I forced it by moving `created_at` and `expires_at` back 8 days in the scratch database.

### SH-10 timings (checked locally)

| Trigger | Old page, data file, preview image all 404 after |
| --- | --- |
| Owner delete (with a report filed; the report row went too) | 0.0 s |
| Expiry, published (rows moved 8 days back) | 7.6 s. The 30 s expiry tick sets the worst case, about 30 s plus the pass |
| Draft past 48 h | row and files gone after 29.1 s |
| Hiding on (PATCH on a published workout) | PATCH took 0.05 s; the old data file and preview 404 0.1 s after the trigger; the page names a new data file |
| Operator `delete -y` | page 404 right after exit 0 |

Caching: the page and the generic page are `no-cache`, owner pages `no-store`, the API `no-store`, hashed data and preview files `private, max-age=31536000, immutable`. Caddy has no server-side cache (F-22 H-1), so the 60 s depends on the disk and on proxies in front. A browser that already has the file keeps it, which is the exception H-11 accepts.

### Draft preview (SH-5, part a machine can check)

For all 41 drafts, `/w/{id}` was the generic 404 page: no `og:` or `twitter:` tags, the text "This workout has expired or is not available.", and no public folder. `/edit/{id}` and `/published/{id}` carry only the fixed card "Edit link: do not share" (SH-11).

## Findings

### Minor

- **ID:** QA-32-r1-1
- **Severity / kind:** Minor / defect (documentation)
- **Route:** backend (F-31 runbook), small lane
- **Where:** `deploy/RUNBOOK.md:274`
- **Relates to:** NFR Privacy (privacy policy, INF-1), F-29
- **Evidence:** section 7 still says "This project ships its privacy page in a later feature (F-29); until then, tell visitors yourself what is stored (section 9)." F-29 is done, and `/about` is the privacy page.
- **Consequence:** a self-hoster is told to do by hand what the About page already does, and is not told that the operator file (section 3) is what makes it complete.

No Blocker or Major finding.

## Previous findings

None (first round).

## Questions

1. **Live expiry check (AC 8).** The live server has no clock override, and the runbook forbids writing with `sqlite3`, so expiry can only be checked live by waiting at least 1 day (lifetime 1 day). Is the local measurement above, together with `caddy-check`'s SH-10 expiry assertion, enough for the expiry part of AC 8? **Answer that settles it:** "local is enough", which drops U-7 step c, or "check live", which keeps it.

## User checks

Use synthetic files only (`src/server/testdata/corpus/synthetic/`), never real ones, except in U-10. Do these after deploying v0.3.0.

- **U-1 (AC 1, live smoke).** Upload `ride-4h.fit`, `gps-dropout.fit`, `indoor-no-distance-no-gps.fit` and `ride-30min-cut-60.fit` on the live site. Publish each, then open its viewing link: the map and charts appear (indoor: "No location data in this workout." and charts only). Upload `course.fit` and `not-fit-png.fit`: the page shows "Unsupported FIT file type." and "This file is not a FIT file.".
- **U-2 (AC 2, LCP).** Reference phone and settings: iPhone 13 mini, iOS 27, Orion, Low Power Mode off, portrait. On a 4G connection (Wi-Fi off), open the published `ride-4h` viewing link. On desktop Chrome, open DevTools, then Lighthouse, then Mobile with throttling set to "Slow 4G", run it on the same link, and record the LCP: it must be under 2.5 s. On the phone, confirm that the summary numbers appear before the map and charts.
- **U-3 (AC 3, 60 fps).** Publish `ride-12h.fit` live. On the reference phone, open `<viewing link>?measure` with the map in view. Do three chart touch scrubs and three route touch scrubs. Each run must pass ADR-016's rule as the in-page record shows it (at least 99 % of frame intervals 20 ms or less, flush p95 2 ms or less, max 8 ms or less).
- **U-4 (AC 4, 5, Android).** On an Android phone with Chrome, open the live upload page, pick `ride-30min.fit` with the file picker: the edit page of the new draft opens. Publish it. Open the `ride-12h` link from U-3 and scrub the charts and the route: no visible stutter, the cursor follows the finger.
- **U-5 (AC 6, browser matrix).** In the current and previous version of Chrome, Firefox, Edge and Safari (macOS), on iOS Safari and on Android Chrome, open the upload page, upload a file, then the edit page (change the title, save, publish), then the published page (both copy buttons), the viewing page (map, charts, scrub, laps, fullscreen map, theme and units) and `/about`. In DevTools, the console shows no CSP errors, and Application then Cookies lists nothing for the site.
- **U-6 (AC 7, messengers).** Paste the `ride-4h` viewing link into WhatsApp, Signal, Slack, Telegram and X: each shows the card with the route image, title and numbers. Paste a draft's `/w/` link (from a fresh upload, not published): no image and no numbers. Paste an edit link: the card says "Edit link: do not share".
- **U-7 (AC 8, SH-10 live).** (a) Delete: open a published viewing link in one tab, delete the workout from its edit link, reload the viewing tab within 60 s: "This workout has expired or is not available.". (b) Hiding on: publish `ride-30min.fit` without hiding, copy the data file URL (DevTools, Network, `data.*.bin`), turn on start and end hiding, save, then within 60 s open the copied URL in a private window: 404. (c) Expiry, unless Question 1 drops it: publish with a lifetime of 1 day, note "Expires", and 1 minute after that time open the link: the not-available page.
- **U-8 (AC 9).** Write the GDPR data-processing record (Art. 30): purpose, categories of data (location history, workout data, report texts), recipients (hosting provider, OpenFreeMap via the visitor's browser, Pushover if enabled), retention (7 days by default, at most 30; drafts 48 h; reports 30 days), no backups, technical measures. Keep it with the operator's documents, not in the repository.
- **U-9 (AC 10).** Fill `/srv/workout-explorer/operator/about.txt` from `deploy/operator.example.txt`, answering every bracketed prompt, including what the host's proxy (the outer Caddy) logs and for how long. Set `WORKOUT_EXPLORER_OPERATOR_FILE` and run `docker compose restart app`. Check that `https://workouts.plugman.de/about` no longer contains `[OPERATOR DETAILS` and that `docker compose logs app` has no "operator file missing" warning after the restart.
- **U-10 (AC 11).** After U-1 to U-9, share links with friends and use the site for real workouts for a few weeks.

## Coverage

- **Traced and checked locally:** AC 1 (complete, except browser rendering), the size budget, the privacy items of the scope (cookies, CSP and headers, originals, logs, nothing left after delete and expiry), SH-10 for delete, expiry, draft expiry and hiding on, SH-5's draft part, SH-11's fixed cards, and `make ci`.
- **Not run:** a browser (no headless browser installed): rendering, LCP and scrubbing are user checks. The Compose stack: the image is built by `docker-check` only, and live Docker networking and the proxy were not exercised. The WAL contents while the server runs (checked only after shutdown). Rate-limit and size refusals (UP-10), which are out of F-32's scope and covered by the F-09 and F-21 tests.
- Nothing was uploaded to or changed on the live site.
