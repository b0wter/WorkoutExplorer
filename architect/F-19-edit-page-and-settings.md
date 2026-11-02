# F-19 Edit page: draft view and settings panel: architecture

- **Version:** 0.3 (30 Oct 2026: no first-open marker, `/published/` served by the same page, no visibility control; built with F-20 v0.2)
- **Status:** accepted (29 Oct 2026). Normal lane plus critical checks (edit token). OQ-81 and OQ-82 resolved
- **History:** [history/F-19-edit-page-and-settings.md](history/F-19-edit-page-and-settings.md)
- **Date:** 29 Oct 2026
- **Feature:** [`features/F-19-edit-page-and-settings.md`](../features/F-19-edit-page-and-settings.md)
- **Requirements:** UP-3, UP-4, UP-7, AC-1, AC-5, SH-2, SH-4, SH-8, AP-17, AP-18, DS-4; TD Publish and view flow (Publish 4, 6), TD Security and privacy (IDs and tokens)
- **Builds on:** [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md) items 7, 9 and rev. 2, [ADR-025](ADR-025-owner-api-access-and-not-found-response.md), [F-12](F-12-owner-api.md) (endpoints, settings object, O-3 to O-6, hand-off to F-19), [F-13](F-13-activity-page-layout.md) D-6, D-7, D-9, D-14, D-15, D-17 to D-20, [ADR-026](ADR-026-activity-page-template-adopted-not-hydrated.md) item 4, [F-16](F-16-start-and-end-hiding.md) H-8 and its hand-off, [ADR-011](ADR-011-activity-page-bundle-allocation.md), [ADR-012](ADR-012-component-smoke-tests-in-jsdom.md); new: [ADR-030](ADR-030-to-be-hidden-ranges-computed-at-upload.md)
- **Owner:** both: backend step B-1 first (ADR-030), then frontend

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| E-1 | **Page and entry.** `src/web/edit.html` is a static HTML file. Caddy serves it for every `/edit/*` and `/published/*` with `no-store` (ADR-020 item 7, F-20 C-1, F-22). In development, the Vite dev server serves it for both. It has the same head as the template (`referrer no-referrer`, `robots noindex, nofollow, noarchive`, viewport, color scheme), the icon sprite, and no workout data. There is no inline script or style (ADR-026 item 5), and no data file preload. It loads the **same entry module as `index.html`**. The entry tells the two pages apart by a marker in `edit.html` and `import()`s the owner chunk there. Everything owner-only is in that chunk: `components/page` (F-13 D-9), the `lib/page` builder, the panel and the owner client. The owner chunk imports `lib/page/view.ts` and `lib/format/dates.ts` by path, not through their `index.ts` (an exception to the TD's index-only rule, owner chunk only), so the builder stays out of the activity entry | ADR-011 item 2, F-13 hand-off: the activity CI line grows only by the switch and the call site. One entry means no shared chunk split (ADR-024 reasoning) |
| E-2 | **The token in the browser** (critical). ADR-020 and ADR-025 decide the transport. On top of that: **one module (`lib/owner`) reads `location` and holds the token**. It takes `public_id` from the path (`/edit/{id}` or `/published/{id}`) and the token from the fragment, up to the first `&` (ADR-020 item 7). If either does not match `^[0-9A-Za-z]{22}$`, the page shows the generic page and sends no request. All other code gets the owner client, never the token. The token is never written to the DOM (no text, attribute or input value; the one exception is F-20 C-5's edit link on the published page), to `document.title`, to Web Storage, IndexedDB, cookies, the Cache API or `history.state`, or to the console or an error message. It never appears in any URL the page builds, and it is never passed to the map, the charts or `lib/prefs`. Every owner request is same-origin to `/api/workouts/{public_id}…`, with `Authorization: Bearer`, no query string, `cache: 'no-store'`, `credentials: 'omit'`, `referrerPolicy: 'no-referrer'` and `redirect: 'error'`. The page never copies or shows `location.href`. Copy link uses the view path (F-13 D-14) | Confining the token to one module makes "never leaks" checkable by a source scan. `redirect: 'error'` means a misconfigured proxy redirect can never forward the header. The fragment never reaches a server or a `Referer` (ADR-020 item 7) |
| E-3 | **No first-open marker** (ADR-020 rev. 2). Nothing rewrites the address; the token stays in the fragment, so a reload works. The edit page never shows the edit link; only F-20's published page does | AC-1, AC-6: the edit link is shown only on the published page |
| E-4 | **Not found and errors.** A `404` from any owner call, and a malformed path or token, all replace the page with the generic page's content: the same heading, sentence and About link as `generic.html`, in the page's theme. The page's HTTP status stays `200`, because it is static. A `500` or a network failure shows the server's `message` (or UP-10's server-error sentence when there is none), never the generic page | ADR-025 consequences, F-12 hand-off. The owner of a damaged link sees what a stranger sees (AC 6). A server fault is not "not found" |
| E-5 | **Load and render.** `GET` settings and `GET draft.bin` run in parallel. The `lib/page` builder makes the view model from the decoded file plus the settings, never with `Route.Hidden` (F-16 H-8). The D-9 root mounts with `report = false`. Then the viewing page's glue (`src/page/start.ts`) runs on that root with the already-decoded data, so the map, charts, range panel, laps and splits, units, splitter and pin toggle are the same code (F-13 D-19, AP-17, AP-18). How the glue takes its data instead of the preload is the engineer's choice; there is no second copy of the glue. If `draft.bin` fails to decode, the panel still works and the page area shows F-13 D-13's load-error text | One glue, one set of components. The owner can still change settings (and, with F-20, delete) when the data file is unreadable |
| E-6 | **Saving.** There is one Save action for the panel (OQ-82). It sends one `PATCH` with only the fields that differ from the last stored settings, and only one save runs at a time. The response is the single source of truth: it replaces the client's settings, and the header (title, description, expiry) re-renders from it, never from unsaved input. The panel shows "saved", or the error `message` (`400`, `500`). Text the enhancers rewrote (expiry, F-13 D-7) must stay correct after that re-render; the engineer chooses how | F-12 O-3 makes partial updates safe across two tabs. On a published workout, every `PATCH` rebuilds public files (O-11), so it should be one request per deliberate save, not one per keystroke |
| E-7 | **Fields.** *Title* and *description*: the panel never sends text the server refuses. It applies the limits in code points, or `maxlength` (F-12 O-4). It never sends a Unicode control character other than a description line feed; the engineer decides how a pasted tab is handled, but the rest of the text is never lost (UP-4). An empty title shows the default as its placeholder (UP-4). *Hiding* and *download*: shown as stored. New uploads store them off (F-09), so the panel has no defaults of its own (AC 4). *Visibility*: shown as text at most, never a control (SH-2; publishing is F-20 and cannot be undone, F-12 O-6). *Lifetime*: whole days from upload (F-12 O-5). It offers only days `d` with 1 ≤ `d` ≤ 30 and `created_at + d·86,400 s` after now. "Now" is taken once at page open; an option that goes stale while the page stays open is refused by the server with `400`, which the panel shows (QA-19-r1-2, accepted by the user, 29 Oct 2026). Next to it, the panel shows the resulting expiry as a date and time in the browser's time zone, formatted by `lib/format` from local getters, with no `Intl` (F-13 D-6; decision log OQ-62) | Every server rule has one client counterpart, so the `400` sentences never show in normal use (F-12 "invalid_settings") |
| E-8 | **Truncated-file notice** (UP-3). When `data_end_s` is not `null`, the panel shows "The file was incomplete; data after hh:mm:ss is missing.", with the elapsed time since the start (review assumption 11) and hours not capped at 24. It is owner-only: it is in the owner chunk and never in the template | UP-3, AC 5 |
| E-9 | **Hidden-section marks** (SH-4, ADR-030). The map marks the sample ranges in `hide_ends_ranges` while the hiding toggle in the panel is on, saved or not, so the owner sees the effect before saving. A full-route range marks the whole track, with no extra warning (feature scope). `[]` marks nothing, so an indoor workout shows no effect and no error (AC 8). The marks go through `RouteMap` into the lazy `map-view` as their own source `wx-hidden` with layers `wx-hidden-bridge` and `wx-hidden-line`, drawn above the route and below the range; the route source is unchanged. They reuse `lib/route`'s range geometry (F-17); markers and the cursor are unchanged. The engineer proposes the look, and the user reviews it once visible (OQ-63 rule) | The browser has no hiding rule (F-16 hand-off). Marks are display only; the public build alone decides what is public |
| E-10 | **Layout.** The D-9 root renders F-13 D-17's wrapper and map-case class, so two columns from 1024 px, the sticky desktop map, the splitter and the phone pin toggle work unchanged (AC 7). The panel sits at the top of the content column, above the header, always open (OQ-81). The header's Copy link (F-13 D-14) is hidden while the workout is a draft, because `/w/{public_id}` shows the generic page until publish; F-20's Share replaces it (user, 29 Oct 2026) | AP-17, AP-18: "the edit page uses the same layout" |

**Module boundary** (adds to F-13's table):

| Module | May import | Never |
| --- | --- | --- |
| `lib/owner` (token, owner client, settings types) | nothing of ours | DOM beyond `location`, `history`, `fetch` |
| `components/edit` (panel, notices) | `lib/owner` (types), `lib/format` | the token |
| `src/edit` (edit glue, lazy) | `lib/owner`, `lib/page`, `components/page`, `components/edit`, `src/page` | — |
| `src/page`, `main.ts` | as before, plus one `import()` of `src/edit` | static imports of anything above |

## B-1 Backend step (senior-backend-engineer, first)

ADR-030: the upload computes the to-be-hidden ranges with the publish rule and stores them (schema v2), and `GET` and `PATCH` return `hide_ends_ranges`. `regenerate` (F-30) recomputes them. The ranges are never logged (F-12 O-13).

## Budget

| What | Limit |
| --- | --- |
| Activity entry (CI line, ADR-002) | **F-19 share 400 bytes gzip**, from the reserve: the page switch, the `import()` call site (the helper is already paid, ADR-011 rev. 4), the glue's data seam and the marks prop in `RouteMap`. Above 400: stop and report. Measured 374 at QA r1 (ADR-011 rev. 10) |
| `edit.html` entry | Same counted set as the activity entry, plus at most the marker. QA reads both lines |
| Owner chunk (lazy, not counted) | Reported in each QA round. Above 15,000 gzip: stop and report (it delays the edit page by its size) |

## Verification (critical lane: QA at effort high)

| What | Automated (`make ci`, ADR-012) | Manual (operator) |
| --- | --- | --- |
| Token (AC 1, E-2, E-3) | jsdom, on an `/edit/{id}#{token}` location. Every request has the header and a path under `/api/workouts/{id}`, with no query. After load, save, error, not-found and indoor states, the token is not in `outerHTML` (attributes included), `document.title`, storage, `history.state` or console output. A source scan shows that only `lib/owner` reads the fragment or builds `Authorization` | Network log on desktop and phone: the token appears only in the request header. Tile requests carry no `Referer` |
| Not found (AC 6, E-4) | A malformed token sends no request. A `404` on any call shows the generic content. A `500` does not | A modified token on the deployed page |
| Fields, saving (AC 2 to 4, E-6, E-7) | Lifetime options against a fixed now and `created_at`. A pasted tab and other control characters are never sent. Only changed fields are sent | Title change, then reload |
| Notice (AC 5) | Synthetic truncated corpus file, with and without truncation | — |
| Layout, marks (AC 7, 8, E-9) | The root carries the map-case class only with a route. `[]` and a full range render without error | Desktop two columns, phone pin, indoor one column, marks on a hiding-on ride |
| B-1 | On the synthetic corpus and F-16's generated tracks, the stored ranges equal the samples whose positions the public file lacks, exactly | — |
| Mutants | Each must fail a test: token in the query, header dropped, generic page on `500`, `redirect` followed, a storage write of the token, marks computed in TypeScript | — |

## Hand-offs

| To | What |
| --- | --- |
| F-20 | Mount into the owner chunk, using the owner client. Publish updates the panel from the response, as E-6 does. The SH-10 notice applies when hiding is turned on after publishing (F-16 hand-off) |
| F-22, F-31 | Serve `build/web/edit.html` for `/edit/*`, with `no-store`, `Referrer-Policy: no-referrer`, the CSP and `X-Robots-Tag` as for `/w/` pages (ADR-020 "Would choose differently if"). Never `log_credentials` |
| F-30 | Recompute ADR-030's ranges on every rebuild |
| F-12 design | The settings table gains `hide_ends_ranges` (read-only), pointing to ADR-030 |
| Requirements | F-19 Owner becomes `both` (backend first). TD Data model gets the ADR-030 columns. TD Repository layout and Dependency rules get `edit.html`, `lib/owner`, `components/edit` and `src/edit` (module boundary above) |

## Open questions (resolved 29 Oct 2026, decision log of `features/README.md`)

| ID | Question | Decision | Blocks |
| --- | --- | --- | --- |
| OQ-81 | Where does the edit panel go in the layout? | **Resolved: accepted.** At the top of the content column, above the header and always open: above everything in one column, at the top of the right column in two. The owner sees the settings first after upload, and the rest of the page stays as viewers will see it | E-10, the panel's CSS |
| OQ-82 | One Save button, or save each change automatically? | **Resolved: accepted.** One Save button with a clear "saved" or error state (E-6). Each save of a published workout rebuilds its public files, and one deliberate save is easier to understand. Unsaved changes are lost on leaving the page, with no warning | E-6 |
