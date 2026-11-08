# F-11 Upload page: architecture

- **Version:** 1.1 (30 Oct 2026: no first-open marker, ADR-020 rev. 2; built with F-20 v0.2)
- **Status:** accepted (OQ-56 to OQ-58 decided 23 Oct 2026, see the decision log in `features/README.md`)
- **History:** [history/F-11-upload-page.md](history/F-11-upload-page.md)
- **Date:** 23 Oct 2026
- **Feature:** [`features/F-11-upload-page.md`](../features/F-11-upload-page.md)
- **Requirements:** UP-1, UP-2 (messages), UP-8 (notice), UP-10, AC-1, DS-3, INF-1 (footer link), NFR Compatibility, NFR Privacy
- **Builds on:** [F-09 design](F-09-upload-api-and-storage.md) (`POST /api/workouts`, response table), [F-10 design](F-10-storage-limit-and-status.md) (`GET /api/status`, S-10, S-11), [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md) rev. 2, [ADR-021](ADR-021-upload-request-handling-and-rate-limit.md) items 1, 5, 9, [ADR-002](ADR-002-bundle-size-budget-measurement.md), [ADR-011](ADR-011-activity-page-bundle-allocation.md) item 3, [ADR-012](ADR-012-component-smoke-tests-in-jsdom.md); new: [ADR-024](ADR-024-upload-page-plain-typescript-entry.md)
- **Owner:** frontend

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| U-1 | **Entry and serving.** A second Vite HTML entry, `src/web/upload.html`; `index.html` stays the activity page. In production, Caddy serves it at `/` by rewrite, not redirect (hand-off). In development it is `/upload.html` on the Vite dev server, which already proxies `/api` | ADR-024 item 1. A rewrite keeps the address the bare site name, which is what people share |
| U-2 | **Plain TypeScript, no Svelte.** The two entries share no chunk, and the activity entry's CI line does not grow. Ceiling 8,000 bytes gzip for the upload entry (ADR-002 counting) | ADR-024 |
| U-3 | **What needs no script is in the HTML:** `lang="en"`, `<meta name="referrer" content="no-referrer">`, the page text, and the footer with the About link. No inline `<script>`, `<style>` or `style=` attribute (F-07 rule; the CSP has `style-src 'self'`). Scripts change state through classes or `hidden` | The footer works without JavaScript. The page is ready for F-22's CSP as it stands |
| U-4 | **Module boundary.** `src/lib/upload/` is pure: the size rule, reading the status answer, turning a response into an outcome, the edit-link check, page states, and the client-held sentences. It has no DOM and imports nothing. `src/upload/` is the DOM glue: elements, `fetch` for the status, the upload request, navigation. It imports only `lib/upload` (and, from F-35, `lib/zip`: [F-35 Z-1](F-35-zip-upload.md)). Nothing else imports either. Neither imports `svelte`, `uplot`, `maplibre-gl` or another `lib/` module | TD front-end rules: decisions pure and tested in Node, glue smoke-tested in jsdom |
| U-5 | **Status check.** `GET /api/status` runs on load, and again when the page comes back from the back-forward cache (`pageshow` with `persisted`), which also resets the page to its start state. Until the answer arrives, no drop zone is shown. With `uploads_open: false`, UP-8's notice replaces the drop zone. If the answer is an error, malformed, or late (a short bounded wait, value chosen by the engineer), the page **fails open** and shows the drop zone, with no message. No polling | AC 4: the notice comes before any pick. The status is only advisory: the server gates every upload at step 0 (F-10 S-9), and a refusal shows the same sentence. A broken status endpoint must not block uploads that would succeed |
| U-6 | **Picking.** One file per upload. If several files are dropped, only the first is used. The file input has **no `accept` filter**. In the browser, the only check is `size > 20,000,000` (ADR-021 item 1), which shows UP-10's 20 MB sentence and sends no request. An empty file is sent, and the server refuses it. A file dropped anywhere on the page never makes the browser open the file. After any refusal, the same file can be picked again | AC 2, AC 6. `.fit` has no registered type on iOS or Android, and an extension filter can grey out every file in the phone pickers. The server is the validator (I-2) |
| U-7 | **Upload request.** `XMLHttpRequest` with `FormData`, one field `file`, sent to the relative URL `/api/workouts`. The Content-Type is not set by hand. While bytes are sent, progress comes from `xhr.upload` progress events. Once the body has been sent, the page shows an indeterminate "processing" state until the response arrives. No client timeout and no automatic retry. One upload at a time: the drop zone is inactive while a request is in flight | `fetch` has no upload progress in Safari or Firefox. Parse time cannot be seen from the client, so it is shown as a state, not a percentage. The server bounds every request (read 120 s, write 120 s + 6·T, ADR-021 item 5). A shorter client timeout could drop the links of a draft that was stored (AC-1). A retry after a lost response would only make a second draft |
| U-8 | **Messages come from the server.** For every non-201 response whose body is JSON with a non-empty string `message`, that text is shown as it is (ADR-021 item 9). Codes never choose a text. The one code that changes behaviour is `storage_full`, which switches the page to the paused state (U-5). The client holds only four sentences, copied word for word from the spec: UP-10's "larger than 20 MB" (U-6), "could not be read" and "server error", and UP-8's notice (U-5). One module holds them (table below). F-35 adds UP-11's archive sentences to the same module (F-35 Z-1) | One home per message: ADR-018 and F-09 own the texts. A proxy error page or a lost connection has no `message` to show |
| U-9 | **Success.** The page reads only `edit_url`. Resolved against `location.origin`, it must be same-origin, path `/edit/` plus 22 base62 characters, and fragment 22 base62 characters (ADR-020 items 2, 3, 7). Anything else counts as a server error. The page calls `location.assign` with it unchanged, so Back returns to the upload page. `view_url` is not used: the published page and Share show it (F-20) | The shape check means a broken or injected response can never send the token elsewhere. No marker: the edit link is shown only on the published page (AC-1, AC-6) |
| U-10 | **Privacy (AC 8).** No cookies, no `localStorage`, `sessionStorage`, IndexedDB, Cache API or service worker. Nothing from the response is logged to the console or kept after navigation. The page only contacts its own origin | AC 8, NFR Privacy, I-6. CSP `connect-src 'self'` covers the page (hand-off F-22) |
| U-11 | **About link** points to `/about`. That path stays a 404 until F-29 serves the About page there | INF-1. This fixes the path now, so the footers of all pages agree. F-29 may change it, together with every footer |

**Response handling** (U-8, U-9). Wording comes from F-09's response table and UP-10; it is not copied here.

| What the browser gets | Shown | Page afterwards |
| --- | --- | --- |
| `201`, valid `edit_url` | nothing | navigates (U-9) |
| `201` without a valid `edit_url`, or not JSON | UP-10 server error | drop zone usable |
| `503` `storage_full` | its `message` | paused notice, no drop zone |
| any other status, JSON with a string `message` | its `message` | drop zone usable (AC 3, AC 5) |
| any other status, any other body (proxy error, HTML, empty) | UP-10 server error | drop zone usable |
| network error or abort, no status | UP-10 "could not be read" | drop zone usable |

## Consequences

- On a phone, switching apps during "processing" can lose the response. The draft is then stored but unreachable, and it expires (AC-1, OQ-45). No mitigation in the MVP.
- Until F-19 exists, a successful upload lands on an address with no page behind it: Caddy has no `/edit/*` route yet, and Vite's dev server falls back to the activity entry. What F-11 verifies is that the browser goes to the new draft's edit link (OQ-56).

## Hand-offs

| To | What |
| --- | --- |
| F-19, F-20 | The edit page receives `/edit/{public_id}#{token}` after upload, exactly as the server returned it |
| F-22, F-31 | Caddy: `/` is a rewrite to `upload.html`, with `Cache-Control: no-cache` like every HTML page. For this page, a CSP of `default-src 'self'` is enough (it covers the two API calls); nothing more is needed. No request-body limit below F-09's `maxBodyBytes` on `/api/workouts`, so the browser sees Go's JSON `413` and not a proxy page. `/about` goes to F-29's page |
| F-29 | Serve the About page at `/about` (U-11), or change every footer in the same change |
| F-14 | If the theme follows the system setting, the upload page can follow with a CSS media query; no shared code |
| Requirements (TD) | Repository layout: `upload.html`, `src/upload/`, `src/lib/upload/`. Front end, Implementation notes: the upload page is plain TypeScript (ADR-024). Feature file AC 1, if OQ-56 is accepted |

## Verification

Synthetic files only (ADR-004 item 7). No browser runner in CI (ADR-012).

| AC | Automated (`make ci`) | Manual (engineer, or the operator for phones) |
| --- | --- | --- |
| 1 | Pure: edit-link check. jsdom: success leads to `location.assign` with the absolute link and no marker, and a back-forward-cache restore resets the page | Against `make dev` + `make dev-server`: drop `ride-30min.fit` on desktop (Firefox on Linux); pick it in iOS Safari and Android Chrome, with the phone reaching the dev server over the LAN (the engineer documents how). Pass: the address is the new draft's `/edit/…#…`, and the draft's folder exists under `build/data/private/` |
| 2, 6 | Pure: 20,000,000 is sent, 20,000,001 is refused. jsdom: a refused file opens no request | DevTools network tab: no request for a 25 MB file |
| 3 | jsdom with F-09's `422` bodies: the message is shown, and a new pick works | `not-fit-png.fit` and `course.fit` show the server's sentences |
| 4 | jsdom: `uploads_open: false` shows the notice and no drop zone; a failed status shows the drop zone | Server with `WORKOUT_EXPLORER_STORAGE_LIMIT_MB=1` on a data folder over 1 MB |
| 5 | jsdom with F-09's `429` and `503 busy` bodies (the Go tests already cover the server side) | none |
| 7 | jsdom: while an upload runs the picker is disabled and a second drop or pick starts nothing; after an error both work again | Drop zone greyed out during an upload (not yet checked by hand) |
| 8 | A check over the built upload chunks for cookie and storage APIs | DevTools after one upload: no cookies, storage empty |

Size: the CI line for `upload.html` stays at or below 8,000. The activity entry's line does not grow, and the two counted sets are disjoint (ADR-024).

## Open questions

OQ-56 to OQ-58 are decided (23 Oct 2026, see the decision log in `features/README.md`); the entries below are kept for the reasoning.

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| OQ-56 | AC 1 says the upload "ends on the edit page", which F-19 builds later | F-11 AC 1 is met when the browser opens the new draft's edit link. The page rendering there is F-19 AC 1, which already covers it. Requirements rewords AC 1 | Ticking AC 1 |
| OQ-57 | Is an Android phone available now for AC 1? | If not, check the Android part in F-32 with the other Android checks (F-32 already lists the upload page on Android Chrome). F-11 waits on the user for that part only | Ticking AC 1 (Android) |
| OQ-58 | The spec has no wording for the page's own texts | Heading "Workout Explorer"; intro "Upload a FIT file to get a page with map, charts and laps that you can share."; drop zone "Drop a FIT file here" with the button "Choose a file"; progress "Uploading… 42 %", then "Reading the workout…"; footer "About" | Final wording; the build can start with the suggestion |
