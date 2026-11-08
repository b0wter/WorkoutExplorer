# Workout Explorer — Technical Design

Oct 24, 2026 · @Joe

## Overview

Workout Explorer runs as one Go binary, one SQLite file and a folder of static files on a single EU server. The server parses each FIT file once, writes a compact binary data file and a static HTML page, and from then on viewers only download static files.

This document covers how the app is built: the system-wide technical picture. What it does, and the requirement IDs referenced below (UP-, AP-, SH-, PIN-, AC-, DS-, INF-), are in the [product specification](https://claude.ai/code/artifact/06c8dbcb-06e7-4e19-ade0-82cd8064c369). Each feature's design (`architect/F-NN-*.md`) and the ADRs (`architect/ADR-NNN-*.md`), listed in `architect/INDEX.md`, fill in the details; they may refine this document, but not contradict it without an ADR. Terms are defined in the Glossary at the end.

**Principles**

- **Lean:** no service runs unless it must. No separate database server, no SSR framework, no CDN until traffic asks for one.
- **Fast for viewers:** the summary numbers are in the HTML itself; data arrives as typed arrays with nothing to parse; the map loads last.
- **One parser:** FIT is decoded only on the server, so the draft preview and the published page can never disagree.
- **Privacy enforced on the server:** hidden points (start and end hiding; privacy zones in phase 2) are removed before any public file is written.
- **Store as little as possible:** no accounts, no email, no backups. Every workout is deleted after at most 30 days; a lost server loses its workouts.
- **No external services:** the server contacts no third party, with one operator-only, opt-in exception: Pushover for new-report notifications (INF-4, ADR-039), off by default; the message is the report number and reason only, and `app` gets a route out only when it is turned on.

## Architecture

```
                        view path (no Go)                       publish path
 Browser ───────────────────────────────▶ Caddy ──── /api/* ────▶ workout-explorer (Go, one binary)
   │  /w/{public_id}/index.html             │                        │   writes            reads/writes
   │  bundle (content-hashed)               │ serves public/ files   ├──────────▶ /srv/workouts/public/   (only after hiding)
   │  data.{hash}.bin(.br)                  │ directly               ├──────────▶ /srv/workouts/private/  (original, draft)
   │  preview / export                      │                        └──────────▶ SQLite (WAL): ownership, settings, reports
   └── MapLibre + tiles ──▶ OpenFreeMap (loaded last)
```

Go is the only component that writes; Caddy serves everything a viewer or a messenger needs straight from `public/`, so a busy shared link costs almost nothing.

**Invariants.** Every module keeps these; the code structure (below) is arranged so that one place enforces each.

| # | Invariant | Requirements | Enforced by |
| --- | --- | --- | --- |
| I-1 | No request in the view path reaches Go. | — | Caddy config (F-31); Go never serves `public/` |
| I-2 | FIT is decoded only on the server, by one pipeline, for draft and published page alike (Principles, "One parser"). | — | Only `ingest` decodes FIT; the front end has no FIT code |
| I-3 | Nothing is written under `public/` until hidden sections are removed; only unlisted workouts go there. | SH-2, SH-4 | Only `publish` writes under `public/` |
| I-4 | Everything stored for a workout is deleted on delete or expiry. | UP-7, AC-4, SH-10 | One delete path in `lifecycle` (row first, then folders, ADR-022 item 6), used by the API, the expiry job and the CLI |
| I-5 | Coordinates never appear in logs. | NFR Operations | Logging convention (Conventions) and review; for the parser, a key allow-list and a position scan in `TestParseLogs` (ADR-018) |
| I-6 | The viewer's browser contacts no third-party host except the tile host, and sends it no referrer and no credentials. | NFR Privacy | ADR-014; CSP and `Referrer-Policy` (Security and privacy, Headers; F-22); review |
| I-7 | No request metadata is stored or logged: the client's IP address (in any form), `X-Forwarded-For`, user agent and upload file name never reach the database, a file or a log line. Rate limiters hold only process-local hashes in memory. | NFR Privacy, INF-2 | ADR-021 item 11; `TestNoRequestMetadataStored`, `TestSchema` (F-09) |
| I-8 | Identifying data that UP-9 removes from an upload is stored, shown and logged nowhere: the upload's own bytes are never written; `original.fit` is the stripped file and holds only records the parser read; `draft.bin` and the row come from parsing the stripped file. | UP-9, NFR Privacy | ADR-023; `TestStripIdentifyingData`, `TestUploadIdentifyingData`, `TestStripRejectedRecord` (F-09) |

## Tech stack

| Layer | Choice | Why | Fallback |
| --- | --- | --- | --- |
| Server | Go, standard library `net/http` | Single static binary, millisecond FIT parsing, low memory | TypeScript on Bun with Hono |
| FIT decoding | `muktihari/fit` v0.28.4, a third-party open-source Go library (BSD licence, not under Garmin's SDK licence), behind the `ingest` anti-corruption layer (ADR-003) | Native speed, no second parser in the browser; the FIT format itself is not protected | Another open-source FIT library |
| Database | SQLite, WAL mode; pure-Go driver `modernc.org/sqlite`, one connection (ADR-019) | Metadata is tiny; no server to run | PostgreSQL if several app servers are ever needed |
| Backups | None, by design | A viewer, not a database: workouts live at most 30 days, and lost data is gone | — |
| File storage | Local disk on the server | Simplest; Caddy serves it directly | S3-compatible object storage when the disk fills |
| Web server | Caddy, behind the operator's own reverse proxy, which does TLS (decision log 1 Nov 2026, later) | Brotli and zstd, immutable caching headers, serves pages while Go is down | nginx |
| Front end | Svelte 5 + Vite + TypeScript, no SvelteKit | Compiles to small plain JavaScript; the page is one interactive view | SolidJS, or plain TypeScript |
| Charts | uPlot, as renderer only: cursor, selection and pointer handling are ours (ADR-009) | Tiny, canvas-based, smooth with tens of thousands of points | Custom canvas renderer |
| Map | MapLibre GL JS, loaded after the charts, behind one lazy adapter (ADR-013) | Vector tiles, WebGL route rendering, route colouring by metric | Leaflet with raster tiles (lighter, less smooth) |
| Map tiles | OpenFreeMap, style liberty (free, no key; ADR-014) | No account, no per-view cost | Self-hosted Protomaps file served by Caddy, if the external dependency becomes a problem |
| Place names (UP-5) | Offline lookup in GeoNames `cities15000` (about 30,000 towns, a few MB, CC BY 4.0) bundled with the binary, nearest town via a grid or k-d tree | No coordinates leave the server; no API key | Skip title suggestions |
| Preview images | Rendered in Go with `golang.org/x/image` and the embedded Go fonts (ADR-033): route from the public data file plus the name and headline values, on a plain background or, from phase 2, over a basemap the sharer's browser renders from OpenFreeMap at publish and Go re-encodes (SH-12, ADR-041, F-36) | No headless browser on the server; the server never contacts the tile host | Plain background only |
| Hosting | One small VPS in the EU (e.g. Hetzner) | Low cost, GDPR, nothing to orchestrate | Add a CDN in front when traffic grows |

## Publish and view flow

The draft and the published workout are the same workout, edited on the same page; publishing only changes who may open it.

**Publish**

1. The browser uploads the FIT file (`POST /api/workouts`, multipart, 20 MB limit). The request carries only the file; lifetime, download and every other setting are chosen afterwards on the draft (step 4), which is what "at upload" means in UP-7 and SH-8.
2. The server checks header and CRC, parses the file (ADR-017, ADR-018), and stores it as the original in the private folder with the identifying data that UP-9 lists removed and its framing repaired as UP-9 describes; the workout data is unchanged (ADR-023, I-8).
3. The server normalises streams, computes summary and laps, and writes a **draft** data file. It returns the viewing link and the edit link with its secret token (UP-1, AC-1; formats in ADR-020); the viewing link shows a generic page until the workout is published.
4. The browser opens the draft: the normal activity page plus an edit panel for title, description, trim (phase 2), start and end hiding, lifetime, file download (phase 3) and, in phase 2, pins (UP-4, UP-6, UP-7, SH-4, SH-8, AC-5). A draft shows no links; after publishing, the Share button shows the viewing link (SH-6). The edit page uses the owner API (F-12 design): `GET` and `PATCH /api/workouts/{public_id}` for the settings, `GET /api/workouts/{public_id}/draft.bin`, `GET /api/workouts/{public_id}/original.fit`, and `DELETE /api/workouts/{public_id}`.
5. On publish (`POST /api/workouts/{public_id}/publish`, same token check as the owner API, ADR-025), the server applies trim and hiding to the streams, writes the public data file, renders the preview PNG, writes `export.fit` from the public streams if downloads are allowed (SH-8, phase 3), and writes the static HTML page.
   From phase 2 (SH-12, F-36 MC-8), the edit page also renders a basemap for the frame the server gives (`GET …/preview-map`) and sends it (`PUT …/preview-map/{key}`), at most 10 s, never as part of the publish request; the next build composes the preview over it while its key matches the public route's frame, else draws the plain card (ADR-041).
6. A successful publish opens the published page, `/published/{public_id}#{token}` (token in the fragment only, as for the edit link), with both links and the save warning (AC-6). After publishing, the edit link keeps opening the draft view, which serves as the edit page for the published workout (UP-4, AC-5). Every later edit (title, pins, privacy, download setting) regenerates only the files it affects. Regenerated data files get new content-hashed names, the HTML page is rewritten to point at them, and the old files are deleted (SH-10).
7. A job running every 30 s, and once at startup, deletes every workout whose `expires_at` has passed, and every draft 48 hours after upload (UP-7). Running it every 30 s keeps SH-10's 60 s limit (F-21 X-5).

**View**

1. Caddy serves `/w/{public_id}`: a static HTML page of a few KB with Open Graph tags, the summary numbers and laps table already in the markup.
2. The page loads the app bundle (cached for a year, content-hashed name).
3. The bundle fetches the data file and builds the charts from typed arrays.
4. MapLibre and the tiles load last; the route and cursor marker attach to the existing chart state.

No request in the view path reaches the Go process (I-1).

## FIT parsing

The parser reads the messages below and builds one normalised workout: sessions, laps and events, plus streams on one time axis at the device's recording interval (usually 1 s). Rules: F-02 and F-08 designs.

| FIT message | Used for |
| --- | --- |
| `file_id` | File type check, device manufacturer and product, creation time |
| `session` | Sport and sub-sport, totals and averages as the device computed them |
| `lap` | Laps table (AP-8) |
| `record` | All streams: position, altitude, distance, speed, heart rate, cadence, power, temperature. Running dynamics are not parsed in the MVP |
| `event` | Timer start and stop, used to split moving from paused time |
| `activity` | The workout's fixed UTC offset (F-02 D-7) |
| `sport` | Sport hints for samples no session covers (F-08 D-12) |
| `length` | Pool swim lengths and stroke type |
| `device_info` | Device and sensor list (AP-14) |
| `zones_target`, `time_in_zone` | Heart-rate and power zones when the device stores them |
| Developer fields | Not parsed in the MVP; extra sensor streams come later |

**Conversions and derived values**

- Positions are stored in semicircles; degrees = semicircles × 180 / 2³¹.
- Prefer `enhanced_altitude` and `enhanced_speed` over the legacy fields when both exist.
- Moving time comes from timer events; if there are none, from speed below a threshold: 1 km/h for cycling and e-biking, 0.5 km/h for every other sport (F-02 R-7).
- Elevation gain, where the device recorded none, uses smoothing plus a 3 m hysteresis, one parameter set for barometric and GPS altitude, so altitude noise is not counted as climbing (F-02 R-8).
- Distance is taken from the file and never decreases: where the device's distance falls, the last higher value is kept (F-08 R-14); missing samples stay missing. If the file has no distance, it is computed from GPS points with the haversine formula.
- Session totals from the device are shown as recorded, also when positions are hidden for privacy (AP-2, SH-9). After a trim (UP-6), totals are recomputed from the remaining streams. Values for a selected range (AP-7) are computed in the browser from the streams; moving time and gain come from the server's cumulative streams (ADR-029).
- Suggested title (UP-5): sport, a time-of-day word from the local start time (morning, afternoon, evening, night), and the nearest town of 15,000+ people to the first point that remains visible after hiding and trim. The GeoNames licence needs an attribution line on the site.
- Irregular sampling (smart recording) is resampled to 1 s only where gaps are under 5 s; longer gaps stay gaps, so pauses draw as breaks.

**Edge cases to test**

- Indoor workouts with no GPS, and rides where GPS drops out for minutes.
- Pauses of hours (café stop, overnight bikepacking).
- Files with several sessions (triathlon, brick workouts).
- Truncated files, files with a bad CRC, and chained FIT files.
- Time zones: show the workout's local time (the offset from the file's `activity` message, F-02 D-7), not the viewer's time zone.
- Very long files: a 12-hour ultra at 1 Hz is about 43,000 records per stream.

Keep a test corpus and run every parser change against it (ADR-004). Only generated synthetic FIT files are committed to the repo. Real FIT files from different brands contain the route, times and the device serial number, so they and their expected output are git-ignored and kept only on the operator's machine, where parser changes are also checked against them.

## Workout data file

Each published workout has one binary file, `data.bin`, that the browser turns into typed arrays without parsing. Caddy serves a pre-compressed `data.bin.br` written at publish time (ADR-006). The byte-level rules are normative in the F-03 design (section "Data file format, version 2"); the decisions behind them are ADR-005.

**Layout** (little-endian, every column starts on a 4-byte boundary so typed-array views need no copy)

1. Magic `WEXD`, format version (uint16), sample count (uint32).
2. Length-prefixed JSON metadata: sport, start time and time zone, summary, laps, sessions, and one descriptor per stream (name, type, scale, unit, offset in the file).
3. One column per stream that the workout has; absent streams are simply not written.

| Stream | Stored as | Resolution | Encoding |
| --- | --- | --- | --- |
| Time offset | int32 | 1 s | Delta |
| Latitude, longitude | int32 each | 10⁻⁶ degree (about 0.1 m) | Delta |
| Altitude | int32 | 0.1 m | Delta |
| Distance | int32 | 0.1 m | Delta |
| Speed | uint16 | 1 mm/s | Raw |
| Heart rate | uint8 | 1 bpm | Raw |
| Cadence | uint8 | 1 rpm or spm | Raw |
| Power | uint16 | 1 W | Raw |
| Temperature | int8 | 1 °C | Raw |
| Moving time, cumulative (format version 2, ADR-029; never charted) | int32 | 1 s | Delta |
| Gain, cumulative (format version 2, ADR-029; never charted) | int32 | 0.1 m | Delta |

Missing samples use the type's maximum value as a sentinel (for example 255 for uint8). Delta-coded columns compress very well with brotli, because most deltas are small and repetitive.

**Size.** A 4-hour ride with every stream is about 14,400 samples × 35 bytes ≈ 504 KB before compression; with delta coding and brotli it should land well inside the 500 KB budget. Measure this on the test corpus early.

**Charts** draw a min/max-per-pixel view computed in the browser, so no downsampled copy is stored; range statistics (AP-7) always use full resolution.

Pins (phase 2) live in a separate `pins.json` next to `data.bin`, so editing a pin never rewrites the streams.

**Versioning.** Only one format version and one app bundle are ever live, except for workouts kept after a failed rebuild (below). Every deploy runs `workout-explorer regenerate`, which rebuilds every live workout (drafts included) from its `original.fit` and the settings in its database row: data file, HTML page, preview image and download file, under new content-hashed names, with the old files deleted afterwards as in SH-10. Old workouts therefore gain new fields and bug fixes too, and the browser never needs readers for old versions. The app does not store workouts long term (OQ-53, decided 21 Oct 2026): if a new version, for example of the FIT library, reads a stored original differently or not at all, the workout is lost, and that is accepted. A deploy is never stopped to protect old workouts. A workout that regenerates shorter is kept as regenerated. A workout is deleted with everything stored for it, as on expiry (AC-4, UP-7), only when the new version rejects its file, so every remaining workout is on the new version unless it was kept after a failed rebuild; the deletion is logged with the device model, never coordinates. On a server-side failure, an interrupted run or a time limit, the workout is kept unchanged for the next run (decided 21 Oct 2026): it serves its previous files, and the app bundle they use, until a later run rebuilds it. A workout that keeps failing to rebuild stays on its old files and bundle and ends at expiry as usual (UP-7). An old app bundle is deleted only once no kept workout refers to it (OQ-55, decided 22 Oct 2026; ADR-005 item 9). Each bundle reads exactly one format version. The deploy reports how many workouts were deleted and how many were kept. At the expected volume (a friends-only site, workouts kept at most 30 days) a full run should take seconds.

## Data model

SQLite holds who owns what and the settings; everything large lives as files. Streams and laps are never stored in the database. Column types, constraints and migrations (forward-only, a one-way door) are in ADR-019; the exact SQL is in the F-09 design.

| Table | Key columns | Notes |
| --- | --- | --- |
| `workouts` | `id` (internal ID, ADR-020), `public_id` (128-bit random, base62), `edit_token_hash`, `title`, `description`, `sport`, `start_time`, `time_zone_offset_s`, `visibility` (draft, unlisted; publishing cannot be undone, SH-2.), `hide_ends` (SH-4), `allow_download`, `summary_json`, `data_end_s` (UP-3, truncated files only), the to-be-hidden start and end sample ranges (SH-4; display only, for the edit page's marks; schema v2, ADR-030), `created_at`, `published_at`, `expires_at` (at most `created_at` + 30 days); phase 2: `trim_start_s`, `trim_end_s` | `summary_json` duplicates the summary from the data file, for lists and preview images |
| `privacy_zones` (phase 2) | `id`, `workout_id`, `lat`, `lon`, `radius_m` | Never published |
| `pins` (phase 2) | `id`, `workout_id`, `type`, `lat`, `lon`, `start_s`, `end_s`, `title`, `note`, `created_at`, `updated_at` | Exported to `pins.json` on every change |
| `reports` (F-27) | `id`, `workout_id`, `reason`, `message`, `created_at` | INF-2. No IP address or contact details. Deleted with the workout (`ON DELETE CASCADE`), at the latest 30 days after `created_at` (F-27 R-3). Never published |

Tables and columns not built yet are added by later migrations when their feature is built (ADR-019). There are no user or session tables: the edit token is the only credential.

**Files on disk.** The data directory is `serve -data <dir>`, `/srv/workouts` in production. Modes, staged creation and startup recovery are in ADR-022.

```
/srv/workouts/
  workouts.db (+ -wal, -shm)             # SQLite (ADR-019)
  public.lock                            # cross-process lock on public/ (ADR-031); outside public/, never served
  private/{id}/original.fit              # the upload with identifying data removed (UP-9); never served publicly
  private/{id}/draft.bin                 # served only with the edit token; no .br
  private/{id}/preview-map.{key}.png     # phase 2: browser basemap, our re-encoding; never served (F-36 MC-9, ADR-041)
  tmp/{id}/                              # staging for a new workout, renamed to private/{id}; emptied at startup
  public/{public_id}/index.html          # static page with OG tags and summary; short cache
  public/{public_id}/data.{hash}.bin(.br)
  public/{public_id}/pins.{hash}.json
  public/{public_id}/preview.{hash}.png  # 1200 × 630
  public/{public_id}/export.{hash}.fit   # only if downloads are allowed (SH-8, phase 3)
```

Under `public/`, folders have mode `0750` and files `0640`; F-31 sets the group so Caddy can read them (F-15 P-6). The generic page is not under the data directory: it is `build/web/generic.html`, which Caddy serves with `404` for `/w/{public_id}` when `public/{public_id}/index.html` does not exist (F-15 P-12, P-13).

Every file except `index.html` has a content hash in its name, so it can be cached for a year while a change still reaches viewers as soon as `index.html` is revalidated.

Deleting or expiring a workout removes its row and then its folders at once (ADR-022 item 6); with no backups, nothing remains (AC-4, UP-7).

## Front end

The numbers appear with the HTML, the charts follow, and the map comes last, so a slow map never holds up the rest of the page.

**Load order**

1. HTML with the summary strip and laps table already rendered by the server, and one stylesheet from the site; no inline CSS, and one inline script only: the theme line that sets `<html data-theme>` before first paint, allowed by its hash in the CSP (ADR-026, DS-4). The client adopts this markup and never re-renders it: there is no Svelte hydration; Svelte mounts only into empty, size-reserved slots (map, charts, range panel), and framework-free enhancers attach behaviour to the server markup (ADR-026).
2. App bundle (Svelte + uPlot) and `data.bin` fetched in parallel; the HTML preloads both.
3. Charts render; cursor, range selection and laps interaction work.
4. MapLibre loaded with a dynamic `import()`, then tiles; the route is added as one GeoJSON source with two features, recorded segments (solid) and bridges (dashed), and the cursor marker as a single feature updated on each cursor move. In the MVP the route simply connects every sample that has a position; GPS dropouts and time gaps are crossed by straight, dashed bridges (AP-3).

**Budget** (compressed transfer, first visit)

| Asset | Budget |
| --- | --- |
| HTML page | 15 KB |
| App bundle, Svelte + uPlot + app code | 60 KB |
| Workout data, 4-hour ride | 500 KB |
| MapLibre GL JS (deferred) | as shipped; 6.14: about 435 KB gzip (JS + worker + CSS). An increase of more than 10 % on a MapLibre upgrade is reported to the user. |
| Largest Contentful Paint, mid-range phone on 4G | under 2.5 s |
| Cursor scrubbing, 12-hour file | 60 frames per second |

Enforce the bundle budget in CI with a size check, so it cannot grow unnoticed. ADR-002 defines what is counted (60,000 bytes gzip per entry); ADR-011 allocates the activity page's share per feature. Scrubbing smoothness is measured as ADR-016 defines.

**Implementation notes**

- One shared cursor store (a sample index) drives charts, map marker and value readouts; no component talks to another directly. The store is plain TypeScript (`lib/cursor`, ADR-008).
- Range stats (AP-7) take constant time: distance, moving time and gain are differences of cumulative streams (moving time and gain from the server, ADR-029); averages come from prefix sums built in the browser once per workout, at full resolution (F-17).
- Units and theme switch on the client without refetching data. Unit defaults come from the browser language, and a units drop-down choice is remembered in `localStorage` on the viewer's device, never in a cookie or sent to the server, and wins over that default (DS-1); one units store per page holds it (`lib/units`, F-14). The interface is English only (DS-3).
- No OS integration in the MVP: no PWA install and no share target. Upload is by drag and drop or file picker (UP-1).
- The upload page is plain TypeScript, with no Svelte: its own HTML entry, sharing no chunk with the activity page (ADR-024). From phase 2 it takes the FIT file out of a zip archive in the browser and sends only that file; the server never receives an archive (UP-11, F-35 design, ADR-040).
- The edit page renders the preview basemap offscreen (fixed light style, no route, the server's frame), never by capturing the visible map, and only at publish or after a published workout's hiding changes (F-36 MC-7, MC-8).
- Without a position stream, the map module is not loaded at all; a small "No location data in this workout" hint takes its place (AP-3).
- The upload page asks `GET /api/status` on load and, if storage is full, shows the "uploads paused" notice instead of the drop zone (UP-8).
- Every page has a footer linking to the About page (INF-1); workout pages also have the Report link, which opens a small form posting to `POST /api/reports` (INF-2).
- The only keyboard shortcut in the MVP is Esc to leave the fullscreen map (AP-15).

## Code structure

A small system, so bounded contexts map to packages, not services. The core domain is **turning a FIT file into a faithful, private-by-construction, fast activity page**: the normalised workout, the publishing rules and the viewer.

**Bounded contexts and modules**

| Context | Kind | Owns | Exposes | Lives in | First built |
| --- | --- | --- | --- | --- | --- |
| Ingestion | Supporting | FIT decoding (ADR-003), validation, recovery, parse limits, the parser log line; anti-corruption layer around `muktihari/fit` | `NewParser(Limits, *slog.Logger) *Parser`, `(*Parser).Parse(ctx, []byte) (workout.Workout, error)`, `(*Parser).Accept(ctx, []byte) (Accepted, error)` for uploads (parse, strip identifying data, reparse; ADR-023), failures as `*ingest.Error` with `Kind` and `Message()` (ADR-017, ADR-018); one `Parser` per process; no library type crosses the boundary | `src/server/internal/ingest` | F-02, F-08 |
| Workout model | Core | Normalised workout, sessions, laps, streams, summary; every normalisation rule (time axis, gap filling, moving time, gain, distance, time zone); later hiding | `Normalise(Recording) Workout`, Go types and pure functions, no I/O | `src/server/internal/workout` | F-02 |
| Publishing | Core | The public build (data file, static page; later preview image, export file in phase 3); the hides-more rule; the only writer to `public/` (I-3), holding the public lock while it writes or deletes there (ADR-031) | The `lifecycle.ChangeHook` and `lifecycle`'s public-area port (F-15 P-3); its own ports for the original's bytes and `Parse` | `src/server/internal/publish` (F-15); page view model and metric formatting in `internal/page`, pure (F-15 P-3, P-11); data file codec in `internal/datafile` and brotli in `internal/precompress` (F-03) | F-03, F-15 |
| Lifecycle | Supporting | Workout record, internal ID, public ID, edit token, links, visibility, lifetime, expiry, storage limit, the single delete path (I-4) | `lifecycle.Service` (`Upload`, `Recover`; the owner operations read, update settings, read `draft.bin` and `original.fit`, and `Delete`, each behind the one token check of ADR-025 (F-12); later features add theirs), pure `NewDraft`, `NewIdentity`, `HashToken`, `ViewPath`, `EditLink`; ports `Parser`, `Repository`, `PrivateFiles` (F-09), and the "published workout changed" hook that F-15 implements (F-12) | `src/server/internal/lifecycle`; adapters `internal/store` (SQLite, ADR-019) and `internal/privatedir` (ADR-022) | F-09 |
| Reports | Supporting | Stored reports (INF-2), operator delete (INF-3), Pushover notifications (INF-4) | `POST /api/reports`; CLI | `src/server/internal/reports` | F-27 |
| HTTP API | Adapter | Routing, request/response mapping, token header, rate limits | `/api/*` | `src/server/internal/httpapi` | F-01 |
| Viewer | Core (browser) | Data file decoding, cursor store, charts, map, range stats, units | Svelte pages | `src/web/src/` | F-04 onwards |

`reports` is a proposed name, confirmed or adjusted by the F-27 design; the other packages exist.

**Published language between server and browser** (contracts; a change needs both engineers and a design-doc update):

| Contract | Owner | Defined in |
| --- | --- | --- |
| Data file format (`WEXD`, version) | backend | Workout data file (above); F-03 design |
| HTTP API (`/api/*`) | backend | F-01 (health), F-09, F-10, F-12, F-15, F-27, F-36 designs |
| Link formats (viewing link, edit link with the token in the fragment) and the token header | backend | ADR-020 |
| Page view model and HTML template | frontend writes template, backend fills it (review decision 30) | ADR-026; F-13 and F-15 designs; the committed sample view models in `src/server/testdata/page/` are the contract |

**Repository layout** (ADR-001). Rows marked "planned" show where later code goes.

```
/                              repo root
├── Makefile                   single build entry point for people and CI (F-01)
├── README.md                  short: layout, prerequisites, make targets, local dev (F-01)
├── .github/workflows/ci.yml   GitHub Actions: thin wrapper that runs `make -k ci` (F-01)
├── build/                     all build output, git-ignored (F-01)
│   ├── workout-explorer       Go binary
│   └── web/                   Vite output incl. .vite/manifest.json and generic.html; `serve -web` reads its index.html as the template (F-15)
├── deploy/
│   └── Caddyfile              published pages, generic page, /api proxy (F-15 P-13), all security and cache headers (F-22, ADR-032); F-31 extends it; `caddy.env` pins the Caddy version; Compose file, `example.env`, deploy script and `RUNBOOK.md` (F-31, planned); `Dockerfile`, its allow-list `Dockerfile.dockerignore` and `compose.build.yaml` for self-built images (F-31 DP-16); `operator.example.txt`, the operator file's template with prompts only; the operator's own file never enters the repository (F-29 AB-8)
└── src/
    ├── server/                Go module `workout-explorer` (F-01)
    │   ├── go.mod, go.sum
    │   ├── cmd/workout-explorer/   main: flags, settings, wiring, subcommands (serve; parse (F-02, diagnostic); regenerate, F-30; delete, F-27)
    │   ├── internal/httpapi/       HTTP adapter (F-01: health; F-09: upload)
    │   ├── internal/ingest/        FIT anti-corruption layer (F-02)
    │   ├── internal/workout/       normalised workout, pure rules (F-02)
    │   ├── scripts/corpus.sh       `make corpus` driver (F-02)
    │   ├── scripts/prototype-data.sh   `make prototype-data` driver; output only in build/prototype/data (F-07)
    │   ├── tools/corpusgen/        dev-only synthetic FIT generator, not in the binary (F-02, ADR-004)
    │   ├── tools/attributions/     dev-only attribution list generator, standard library only, run by `make build`, not in the binary (F-29 AB-5)
    │   ├── tools/protoserve/       dev-only static server with in-memory brotli; serves the sample pages (F-07, F-13 B-1); never deployed
    │   ├── internal/datafile/      data file codec, format v2, pure (F-03, ADR-005)
    │   ├── internal/precompress/   brotli pre-compression (F-03, ADR-006)
    │   ├── internal/corpustest/    corpus listing for tests only, never in the binary (F-03)
    │   ├── internal/publish/       public build, hides-more rule, the only writer under <data>/public (F-15, ADR-028)
    │   ├── internal/page/          Go page view-model builder and metric formatting, pure; mirrors lib/page (F-15)
    │   ├── internal/lifecycle/     workout record, identity, links, Upload and Recover (F-09), owner operations and token check (F-12); no SQL, no os
    │   ├── internal/store/         SQLite adapter, embedded migrations in migrations/ (F-09, ADR-019)
    │   ├── internal/privatedir/    the only writer under <data>/private and <data>/tmp (F-09, ADR-022)
    │   ├── testdata/corpus/        FIT corpus and golden reports (F-02, ADR-004)
    │   ├── testdata/page/          sample page view models written by `lib/page`, the contract for F-15's Go builder (F-13, ADR-026)
    │   └── testdata/datafile/      golden data files and fixtures from synthetic files (F-03)
    └── web/                   Svelte 5 + Vite + TypeScript app (F-01)
        ├── package.json, package-lock.json, .nvmrc
        ├── vite.config.ts, svelte.config.js, tsconfig.json
        ├── index.html              activity page template: Go `html/template`, logic-free, frontend-owned, executed by the server (F-15); also the Vite activity entry (F-01, F-07, F-13, ADR-026)
        ├── upload.html             upload page entry, plain TypeScript, no chunk shared with index.html (F-11, ADR-024)
        ├── generic.html            generic page entry: static, no script (F-15 P-12)
        ├── about.html              About page template (privacy text, attribution slot), no script; filled and served by Go at `/about` (F-29 AB-1, AB-4); the committed attribution sources file sits beside it (F-29 AB-5)
        ├── edit.html               edit page: static, no workout data, same entry module as index.html (F-19 E-1)
        ├── scripts/check-bundle-size.mjs   budget check (F-01, ADR-002)
        ├── test-support/golden.mjs     reads the golden data files for tests (F-04)
        ├── bench/                      dev-only timing and check pages and MEASUREMENTS.md, never built (F-04; charts.html F-05; map.html F-06)
        └── src/
            ├── main.ts                 starts the viewing page (F-07, F-13)
            ├── page/                   viewing-page glue: reads the page, loads the data file, mounts into empty slots, load-error text, enhancers (F-13, ADR-026)
            ├── edit/                   edit-page glue, LAZY, owner chunk only, via import() from main.ts (F-19 E-1)
            ├── measure/                scrub recorder, LAZY, only via import() with ?measure (F-07, ADR-016)
            ├── upload/                 upload page DOM glue: elements, status and upload requests, navigation; imports only lib/upload and lib/zip (F-11, F-35, ADR-024)
            ├── theme/                  plain.css: the plain pages' frame and theme, CSS only; imported by upload and generic, never by the activity entry (F-14 D-9, F-15 P-12)
            ├── generic/                generic page stylesheet and its static check (F-15 P-12)
            ├── lib/                    pure TypeScript: no Svelte, no DOM (range stats planned)
            │   ├── format/             numbers, durations, pace, dates, sport names; fixed English strings, no `Intl` (F-13)
            │   ├── page/               pure page view-model builder (F-13, ADR-026)
            │   ├── units/              units store and the browser-language default; imports only lib/format (F-14, ADR-027; planned)
            │   ├── prefs/              viewer preferences remembered on the device (units, x axis, map/content split); the only code touching browser storage; imports only the Units type from lib/format; imported only by src/page (DS-1, AP-5, AP-17)
            │   ├── datafile/           data file decoder, WorkoutData (F-04, ADR-007)
            │   ├── cursor/             cursor store, nearestIndex (F-05, ADR-008)
            │   ├── charts/             chart specs, decimation, formatting, gesture reducer (F-05)
            │   ├── colouring/          route colouring: offered metrics, per-sample values, bounds, classes, legend numbers; imports only lib/datafile and lib/format (F-25, ADR-037)
            │   ├── owner/              the only holder of the edit token; owner API client and settings types (F-19 E-2)
            │   ├── upload/             upload page logic: size rule, status answer, response outcome, edit-link check, page states, client-held sentences; imports nothing (F-11)
            │   ├── zip/                zip archive reader for the upload page: detection, central directory, root FIT entry, inflating under a limit; imports nothing, imported only by src/upload (F-35, ADR-040)
            │   └── route/              route presence, geometry, markers, nearest-sample grid, map gesture reducer (F-06, ADR-015)
            └── components/             Svelte components
                ├── charts/             ChartStack, MetricChart, CursorReadout; the only uPlot importer (F-05, ADR-009)
                ├── map/                RouteMap + touch-quiet (static); map-view, map-style (lazy); the only MapLibre importer (F-06, ADR-013)
                ├── page/               static modules (header, summary, laps, footer) rendering the template's markup for the edit page; lazy, owner chunk only (F-13, F-19)
                └── edit/               edit panel and owner notices; lazy, owner chunk only (F-19)
```

**Dependency rules**

Go (arrows mean "may import"):

```
cmd/workout-explorer ──▶ httpapi, lifecycle, publish, store, privatedir, ingest   (wiring only); datafile, precompress, workout for the parse subcommand only
httpapi ──▶ lifecycle, reports, publish (application operations); ingest for the *ingest.Error type and kinds only (F-09)
lifecycle, reports ──▶ workout, ingest, datafile; I/O through small interfaces they define
reports ──▶ lifecycle for ValidPublicID only (F-27)
publish ──▶ lifecycle, page, workout, datafile, precompress; I/O and parsing through small interfaces it defines (F-15 P-3)
page ──▶ datafile (standard library otherwise, no I/O)
store (SQLite), privatedir, filesystem adapters ──▶ implement those interfaces (and import the package that defines them)
ingest ──▶ workout, muktihari/fit
datafile ──▶ workout (standard library otherwise, no I/O)
precompress ──▶ andybalholm/brotli only
workout ──▶ standard library only, no I/O (no net/http, database/sql, os file access)
```

- Decision logic (summary, hiding, expiry due, storage-limit check) is pure and unit-tested without disk or network; I/O sits at the edges.
- `tools/protoserve` (F-07, dev-only) imports the standard library and `internal/precompress` only; nothing imports it, and it never enters the binary.
- Only `ingest` imports the FIT library (ADR-003); the one exception is the dev-only `tools/corpusgen`, which never enters the binary. Only `precompress` imports the brotli library (ADR-006). Only `publish` writes under `public/`. Only `privatedir` writes under `private/` and `tmp/` (ADR-022). Only `store` imports the SQLite driver (ADR-019). `lifecycle` imports neither `database/sql` nor `os`. `corpustest` is imported by `_test.go` files only.
- Standard library first; every third-party Go module needs a line of justification in the feature's design doc.

Front end:

- `src/lib/` is plain TypeScript with no Svelte or DOM imports, so it is tested by Vitest in Node. Components get smoke tests in jsdom (`*.dom.test.ts`, in CI, ADR-012); pixels, layout and touch stay manual.
- A `lib/` module with an `index.ts` is imported only through it (`lib/datafile`, F-04). Test helpers (`testing/`, `test-support/`) and `bench/` are never imported by app code.
- Components talk through stores, never directly to each other (Front end, Implementation notes). Components subscribe to the cursor store and write to the DOM themselves, never through `$cursor` or other Svelte reactivity (ADR-008).
- Only `components/charts` imports `uplot` (ADR-009). `lib/cursor` imports nothing; `lib/charts` imports only `lib/datafile` and `lib/format` (F-05 and F-13 designs, Module boundary).
- `lib/format` imports nothing. `lib/page` imports `lib/datafile` (types), `lib/format`, `lib/charts` (chart set only) and `lib/route` (`routePresence` only). `components/page` imports `lib/page`, `components/charts` and `components/map`. `src/page` never imports `components/page` statically (F-13 design, Module boundary).
- `lib/format` owns the unit system type, every conversion factor and every unit label; no other module holds one. `lib/units` (the units store and the default rule) imports only `lib/format`, for the type. `lib/prefs` (viewer preferences remembered on the device: units, x axis, map/content split; DS-1, AP-5, AP-17) is the only code that touches browser storage; it imports only the Units type from `lib/format` and is imported only by `src/page`. `src/page` imports `lib/units` and `lib/page`'s summary and laps builders statically. No module other than `lib/units` reads `navigator` for units or keeps a unit of its own; every consumer gets the one units store passed in (F-14 design D-1, D-3; ADR-027).
- Only `components/map/map-view.ts` imports `maplibre-gl` as a value, and only `RouteMap.svelte` reaches it, through `import('./map-view')`; the one exception is `components/map/basemap.ts` (preview basemap), reached only through `import()` from `src/edit` (F-36 MC-7, ADR-013 note); `map-style.ts` uses `import type` (ADR-013). `lib/route` imports only `lib/datafile`; static code uses only its `routePresence`. `lib/route` and `lib/charts` never import each other; map and charts meet only in the cursor store (F-06 design, Module boundary). `lib/colouring` imports only `lib/datafile` and `lib/format`, never `lib/route` or `lib/charts`; `components/map` may import `lib/colouring` and `lib/units` (F-25 design, Module boundary; ADR-037).
- Owner-only code (`lib/owner`, `components/edit`, `src/edit`) follows the F-19 design's module boundary ([`architect/F-19-edit-page-and-settings.md`](../architect/F-19-edit-page-and-settings.md)): only `lib/owner` reads the token, and `src/page` and `main.ts` reach `src/edit` only through one `import()`.
- `src/measure/` is reached only through `import()` from the viewing-page glue in `src/page/`, and only with `?measure` (F-07, F-13, ADR-016). It never adds to the counted entry.
- Runtime dependencies are limited to Svelte, uPlot, and MapLibre (dynamic import, F-06). Any other runtime dependency needs a design-doc entry with its compressed size.
- Activity page bundle (ADR-011): only what every viewer needs before the map is imported statically. Map-only code, owner-only UI and the report form load with dynamic `import()`. Each feature design states its share of the 60,000 bytes and measures it.

## Conventions

| Concern | Convention |
| --- | --- |
| Binary | One binary, `workout-explorer`, with subcommands (`serve`; `parse <file>` as a diagnostic, F-02; `delete -data <dir> <public_id>`, F-27; `regenerate -data <dir> -web <dir>`, F-30). Built with `CGO_ENABLED=0` as a static binary; a later feature that needs cgo must say so in an ADR. |
| Logging | `log/slog` with the JSON handler to stdout. Never log coordinates, edit tokens or file contents (I-5). Parse outcomes are logged by `ingest` only, one line per rejection or recovery, with an allow-list of keys: the device make and model and where decoding stopped, never coordinates (ADR-018); callers add only random IDs through `logger.With`. `workout` never logs. The upload path logs only the lines and keys of the F-09 design (section Logging): the internal ID may appear, never the public ID, the token or its hash, any request metadata (I-7), or any value the strip removes (I-8). |
| HTTP | Standard library `net/http` and `ServeMux` method patterns (`GET /api/health`). All API routes under `/api/`. |
| Versions | Go version pinned in `go.mod` (`go` and `toolchain` lines); Node pinned in `src/web/.nvmrc` and `engines`; npm with committed lockfile, installed with `npm ci`. CI reads both pins from those files, never from its own config. |
| Build stamping | The binary carries a version string set with `-ldflags -X` from `git describe --always --dirty`; `dev` when built with `go run`. |

**Configuration**

- **Flags.** The binary takes command-line flags on the subcommand, with defaults that work for local development. Exceptions: `serve -data <dir>` is required, so the binary never creates a database in an accidental working directory; `serve -web <dir>` is required, the deployed web build whose `index.html` is read once at start as the page template (missing or unparsable: exit 1 before listening; F-15 P-10). `make dev-server` passes `build/data` and `build/web`. No config file.
- **Settings (environment variables).** The binary reads exactly the ones in the table below, only in `cmd/workout-explorer/settings.go`, once at start. Each has a default, a strict format (ASCII digits without sign, unit, spaces or leading zero: `^[1-9][0-9]*$`) and a range, except the base URL, the trusted proxy and the two Pushover settings, whose formats are in their rows. An invalid value makes the subcommand print a message naming the setting to stderr and exit 2 before it touches the data directory. A new setting needs a design-doc entry and a row here.

  | Setting | Meaning | Default | Range | Read by | Decided in |
  | --- | --- | --- | --- | --- | --- |
  | `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` | Uploads processed at once (reading, parsing, stripping, storing); the next one gets UP-10's "try again in a minute" | 4 | 1–16 | `serve` | ADR-021 item 4, F-09 |
  | `WORKOUT_EXPLORER_PARSE_TIMEOUT` | The parser's time limit per decode, in whole seconds; the HTTP write deadline follows it (ADR-021 rev. 4) | `ingest.DefaultLimits.Timeout` = 10 | 5–60 | every subcommand that builds an `ingest.Parser`: `serve`, `parse`, `regenerate` | ADR-017 rev. 7, F-09 D-30 |
  | `WORKOUT_EXPLORER_STORAGE_LIMIT_MB` | Storage cap (UP-8): total size of the files under the data directory, in decimal megabytes (1,000,000 bytes); once reached, new uploads get `503` and `GET /api/status` reports `uploads_open: false` | 1,000 (1 GB) | 1–1,000,000 | `serve` | F-10 design S-1 |
  | `WORKOUT_EXPLORER_BASE_URL` | Origin for absolute links in Open Graph tags; `http(s)://host[:port]`, no path, trailing slash, user info, query or fragment | none (required) | — | `serve`, `regenerate` | F-23 PC-8 |
  | `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4` | Uploads per rolling hour per IPv4 client address, refused ones counted (NFR Security) | 60 | 1–240 (the limiter's memory L grows with it, ADR-021 rev. 6 item 7) | `serve` | decision log 1 Nov 2026; F-31 DP-1c |
  | `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV6` | Uploads per rolling hour per IPv6 /64 prefix, refused ones counted (NFR Security) | 60 | 1–240 | `serve` | decision log 1 Nov 2026; F-31 DP-1c |
  | `WORKOUT_EXPLORER_TRUSTED_PROXY` | The one proxy address, besides loopback, whose `X-Forwarded-For` is trusted; one IP address, no list or prefix. Set by the Compose file, not the operator's env file | none (loopback only) | — | `serve` | ADR-021 rev. 5, F-31 DP-1a |
  | `WORKOUT_EXPLORER_OPERATOR_FILE` | The operator file shown on the About page: who runs the site, contact, hosting, web server logs, backups, legal basis (INF-1); plain text, HTML-escaped; invalid exits 2 | none: the page shows one placeholder block naming what is missing | absolute path; at most 16 KiB, UTF-8 | `serve` | F-29 AB-8 |
  | `WORKOUT_EXPLORER_PUSHOVER_TOKEN` | Pushover application token for new-report notifications (INF-4); exactly 30 ASCII letters and digits. Set together with the user key or not at all; one without the other exits 2. Error messages and logs never show its value | none (notifications off) | — | `serve` | ADR-039 item 1, F-27 R-11 |
  | `WORKOUT_EXPLORER_PUSHOVER_USER` | Pushover user key that receives the notifications (INF-4); same format and rules as the token | none (notifications off) | — | `serve` | ADR-039 item 1, F-27 R-11 |

- **Go runtime variables** such as `GOMEMLIMIT` are not application settings. Our code neither reads nor sets them (no `debug.SetMemoryLimit` or `SetGCPercent`). Only the Compose file may set them (F-31, ADR-017 rev. 3 item 5), and every measured stop rule is taken without them.
- **Dev tooling:** exactly one environment variable, `DEV_ADDR` (F-01 D-4), set by the Makefile and read only by `vite.config.ts` for the dev proxy target; it never reaches the binary or the build output. No `.env` files.

## Hosting and operations

One small VPS runs everything. Rebuilding on a fresh server (binary, Caddy config, empty database) should take under an hour and is rehearsed once before launch; the workouts on a lost server are gone.

| Concern | Approach |
| --- | --- |
| Server | One small VPS in an EU data centre (e.g. Hetzner) with at least 2 GB RAM, Debian. The Go server and Caddy run in one Docker Compose setup, started and stopped together, kept running by Docker restart policies (no systemd). The Go container's memory is capped from the measured worst case of 4 uploads at once (ADR-017 rev. 4, F-31). Settings (Conventions, Configuration) come from one env file read by Compose, the same for `serve` and `regenerate` |
| TLS and static files | The operator's own reverse proxy does TLS and forwards to Caddy on a plain HTTP port (no ACME, no 80/443 in our setup; standalone mode may come later). Caddy trusts the forwarded client address only from that proxy (a setting), Go only from Caddy. Caddy serves `public/` directly, proxies `/api/*` to Go. Content-hashed files are `immutable` for a year, workout files `private`, assets `public`; `index.html` is `no-cache`, so browsers revalidate it on every visit (SH-10). Exact table: F-22 design H-6 |
| Backups | None, by design. Server config and deploy scripts live in the repository |
| Deploys | Pushing a version tag (`vX.Y.Z`) runs CI and, only when it is green, creates a GitHub Release with the Go binary and the front-end bundle attached; `deploy.sh`, run by the operator with a release tag (no expiry; the deployed version is the tag; ADR-035), copies them over, restarts the Go container (a few seconds of API downtime), then runs `workout-explorer regenerate` (step added by F-30) and, once it finishes, removes each old bundle that no kept workout refers to (Versioning). Database migrations are forward-only, so rolling back means deploying a fixed new binary (ADR-019). A deploy that recreates Caddy (version, service definition or settings change) causes a brief page outage, accepted (decision log 1 Nov 2026). Self-hosters can build the app image from the repository's multi-stage Dockerfile (Go server and web build) with `docker compose build`, or download a release; no image is published to a registry, CI pushes none and holds no server credentials. The repository is public (AGPL-3.0), so releases are public downloads. The operator's own deploy uses the release path (F-31 DP-2, DP-16; ADR-034, ADR-035; decision log 2 Nov 2026) |
| Logs | Structured JSON logs, kept by Docker's logging driver; what may be logged is in Conventions (Logging) |
| Monitoring | Not set up by F-31; the operator adds their own monitor later (decision log 1 Nov 2026). Suitable targets: `GET /api/health` (side-effect-free, F-31 DP-12) and the upload page `/`, never a workout page |
| Storage limit (UP-8) | A config setting for the total size of `/srv/workouts`. The server keeps a running total (recomputed at startup), refuses uploads with `503` and a clear message once the limit is reached, and reports `uploads_open` in `GET /api/status` |
| Reports (INF-2, INF-3) | `POST /api/reports`, rate-limited per IP in memory like uploads, fixed at 60 per hour (not a setting; decision log 1 Nov 2026). Reports are deleted with their workout, at the latest 30 days after filing. The operator reviews the `reports` table with `sqlite3` on the host, read-only and as the app's user (F-27 R-7), and removes a workout, draft or published, with a CLI subcommand (`workout-explorer delete -data <dir> <public_id>`) that runs the normal delete. No admin web page. Notifications (INF-4): opt-in Pushover message per stored report, number and reason only, at most 10 per hour, with egress for `app` only through a Compose override (ADR-039). Other callback targets stay a later idea |
| About page (INF-1) | Built in the release build with the generated attribution list (from the bundled npm packages, the Go modules linked into the binary and a committed sources file for map data and other sources; F-29 AB-5); app facts in the template; the operator file (F-29 AB-8), version and Pushover block filled by `serve` at start; served by Go at `/about` (F-29 AB-3, AB-4) |
| Analytics | None at first; later a cookie-less tool if needed |
| Scaling path | Put a CDN in front of `public/` (purging `index.html` on every change, delete and expiry so SH-10's 60 s still holds), then move files to object storage; the Go process stays the same |

**Local development:** `go run` for the server, `vite dev` for the front end with a proxy to the API, and a `make corpus` target that runs the parser over every test file and diffs the output.

**Build and CI:** all build logic lives in the root `Makefile`; CI (GitHub Actions) only installs toolchains and runs `make -k ci` (ADR-001). CI runs on `v*` tag pushes and manual runs only; local `make ci` is the gate before each commit. Target details are in the F-01 design (section Make targets). Deploy (F-31) consumes `build/`.

## Security and privacy

The rule that holds it together: nothing is written under `public/` until hidden sections have been removed, and only unlisted workouts are written there at all (I-3).

- **Hidden sections (SH-4; SH-3 privacy zones in phase 2):** at publish, when the sharer turns hiding on, every sample before the first recorded fix more than 300 m (straight line) from the first position loses its position, and likewise backwards from the last position; gap-filled samples are never the boundary, and only latitude and longitude are removed ([F-16](../architect/F-16-start-and-end-hiding.md) H-2, H-3). In phase 2, samples inside a zone lose their position too. The other streams are kept so charts stay continuous. The bounding box and preview image are computed after the cut; the summary keeps the device totals, and every page shows the privacy notice, whether or not anything is hidden (SH-9). That a section is hidden is not revealed, with one exception: a fully hidden route shows "The entire route has been hidden by the uploader." in place of the map ([F-16](../architect/F-16-start-and-end-hiding.md) H-5). Pins inside a zone are refused (PIN-7).
- **Lifetime (UP-7):** every workout has an `expires_at`, 7 days after upload by default and at most 30; a workout never published is deleted 48 hours after upload, or earlier if its lifetime ends first (UP-7). Everything stored for a workout (original, draft, public files, pins, reports, row) is deleted when it expires. After that, `/w/{public_id}` shows a generic "expired" page with no Open Graph data.
- **Draft workouts:** stay under `private/` and are served by the Go process only to a request carrying the edit token, through the owner API under `/api/workouts/{public_id}` (F-12 design); every other request gets the not-found response (ADR-025). There is no private state (removed 30 Oct 2026, SH-2): a published workout stays unlisted until it is deleted or expires.
- **IDs and tokens:** `public_id` and edit tokens are 128-bit random values from `crypto/rand`; edit tokens are stored as SHA-256 hashes and sent as a header (`Authorization: Bearer`), never in a URL that could leak through referrers; an owner API request with any query string gets the not-found response, even with a valid header (ADR-025). The edit link itself carries the token in the URL fragment, which browsers do not send to servers (ADR-020).
- **Uploads:** 20 MB (20,000,000 bytes) cap enforced while reading; rate limit per IPv4 address and per IPv6 /64 prefix (two settings, Configuration), with refused uploads counted too (kept in memory, not stored); at most a configured number of uploads processed at once (`WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS`, default 4), the next one refused with "try again in a minute" (UP-10); parse with a time limit (`WORKOUT_EXPLORER_PARSE_TIMEOUT`, default 10 s) and a memory limit enforced as counted budgets, in two parse slots (ADR-017); a malformed file can fail its own request but not the process. Request handling: ADR-021.
- **Originals:** `original.fit` is the upload with identifying data removed (UP-9, I-8), so it is not byte-identical to the upload. It is never served publicly; the owner downloads it through `GET /api/workouts/{public_id}/original.fit` as `workout-YYYY-MM-DD.fit` (local start date) or `workout.fit` without a start time, an ASCII name that never uses the title (SH-8). Viewer downloads, when allowed, get `export.{hash}.fit`, which is built from the public, privacy-zoned streams.
- **Indexing and preview cards (SH-5):** the unguessable URL is the real protection; these rules only keep a leaked link out of search results without breaking messenger previews.
  - There is no `robots.txt` (`404`, which crawlers read as allow-all), so nothing disallows `/w/`. Search engines must be able to fetch a page to see its `noindex`, and some preview crawlers (X's Twitterbot) obey `robots.txt`.
  - Every workout page carries `<meta name="robots" content="noindex, nofollow, noarchive">`, and Caddy adds `X-Robots-Tag: noindex, nofollow, noarchive` to every response of the site (OQ-89), including data files, preview images and downloads.
  - Open Graph and Twitter tags sit early in `<head>`: `og:title`, `og:description` with the headline values (SH-5), `og:image` as an absolute URL to the content-hashed preview, `og:image:width` 1200, `og:image:height` 630, and `twitter:card` set to `summary_large_image`. The preview PNG stays under 300 KB, since WhatsApp skips larger images; the route and the frame come from the public data file only, after hiding (F-23 PC-1, F-36 MC-2). A browser basemap is used only while its key matches the current public route's frame (ADR-041).
  - **Preview basemap upload (SH-12):** owner-only (ADR-025); 4,000,000-byte cap while reading; must decode as a PNG of exactly 1200 × 630, checked from the header before any pixel allocation; one request at a time; stored only as our re-encoding, served only inside our composite, never the received bytes (F-36 MC-5).
  - `/edit/*` and `/published/*` carry SH-11's fixed text card (`twitter:card` `summary`, no image, no workout data); the token stays in the fragment.
  - Draft and expired workouts get a generic page with no Open Graph data, so a reposted link reveals no route or numbers.
  - Acceptance check before launch and after changes to the page template: the preview card renders in WhatsApp, Signal, Slack, Telegram and X.
- **Headers and third parties:** strict Content Security Policy (scripts only from the site, tiles and fonts only from the tile host); `Referrer-Policy: no-referrer` on every response of the site, including JavaScript assets, so the map worker's tile requests carry no referrer either; every page that loads the map also carries `<meta name="referrer" content="no-referrer">` (ADR-014, F-22). The tile host is the only third party the viewer's browser contacts, without referrer or credentials (I-6): no other external fonts, scripts, images or analytics. Server side, the only third party is Pushover, and only when the operator configures it (INF-4, ADR-039). `X-Robots-Tag: noindex, nofollow, noarchive` on every response. The full header set: F-22 design H-2.
- **GDPR:** EU hosting, no backups, no accounts or email addresses, no third-party trackers; the tile host sees viewers' IP addresses, which belongs in the privacy policy, or is avoided by self-hosting tiles.

## Open technical questions

Not decided in this document yet; the feature named decides it. Open questions that block design work are tracked in `architect/INDEX.md`.

None at the moment.

## Glossary

The ubiquitous language: use these names in code, API fields and documents. Add to this table when a feature introduces a term.

| Term | Meaning | Not to be confused with |
| --- | --- | --- |
| Workout | One uploaded activity, from upload until delete or expiry. | "Activity" is the page, not the entity. |
| Original | The uploaded FIT file with identifying data removed (UP-9, ADR-023), kept privately (`original.fit`); not byte-identical to the upload. A truncated original ends at the last record the parser read: an incomplete or rejected record and everything after it are not kept (ADR-023 item 6a). | Export; the upload (never stored) |
| Identifying data / strip | The fields and messages UP-9 removes from an upload / the byte-level rewrite in `ingest.Accept` that removes them, verified by parsing the result again (ADR-023). | Hidden section (positions removed at publish, SH-4) |
| Normalised workout | The parser's output: sessions, laps, events and streams on one time axis. | The FIT messages themselves |
| Stream / sample | One measured quantity over time / one index on the shared time axis. | Record (a FIT message) |
| Draft | A workout that has not been published; visible only with the edit token (SH-2). | Draft data file (`draft.bin`), which exists for every workout |
| Unlisted | The only visibility after publishing; publishing cannot be undone (SH-2). The former private state was removed on 30 Oct 2026. | Private folder (`private/{id}/`), which holds every workout's original and draft data file |
| Data file | The binary `WEXD` file the browser reads (`data.{hash}.bin`, `draft.bin`). | Original |
| Hidden section | Start or end stretch whose positions are removed before publishing (SH-4). | Trim (phase 2, removes samples) |
| Public ID | 128-bit random ID in `/w/{public_id}` (SH-1); 22 base62 characters (ADR-020). Never logged. | Internal ID |
| Internal ID | The workout's 128-bit random key, 32 hex characters; names `private/{id}/`; the only workout identifier allowed in logs; never sent to a browser (ADR-020). | Public ID; a database row number |
| Edit token | The only credential; 22 base62 characters; stored only as its SHA-256 hash (AC-1); travels in the edit link's fragment and, to the API, only as `Authorization: Bearer` (ADR-020). | |
| Workout record | The workout's row in `workouts` (ADR-019); `lifecycle.Record` in code. | Normalised workout; data file |
| Viewing link / edit link | `/w/{public_id}` / `/edit/{public_id}#{token}`, root-relative in API responses (ADR-020). | Published page |
| Published page | `/published/{public_id}#{token}`: the owner's page with both links and the save warning, opened after a successful publish (AC-6). | Edit page; the published activity page (`/w/{public_id}`) |
| Owner API | The endpoints under `/api/workouts/{public_id}` that need the edit token: settings, `draft.bin`, `original.fit`, delete (F-12), publish (F-15), preview map (F-36). | Upload (`POST /api/workouts`, no token) |
| Preview frame / key | The map camera (centre, zoom) the server computes from the public route for the preview image, and the hash that names it; a stored basemap counts only while its key equals the current frame's (ADR-041). | Bounding box of the visible map |
| Preview basemap | The tiles-only 1200 × 630 image the sharer's browser renders for a preview frame; stored privately as `preview-map.{key}.png`, never served as received (F-36). | Preview image (`preview.{hash}.png`, the served card) |
| Not-found response | The one answer an owner API request gets for an unknown or expired workout, a missing, malformed or wrong token, or any query string: `404`, byte-identical body, never `401` or `403` (ADR-025). | Generic page (the same sentence, served for `/w/{public_id}`) |
| Upload cap | 20,000,000 bytes of file content per upload, enforced while reading (ADR-021). | Parse limit `MaxBytes` (20 MiB, ADR-017) |
| Data directory | `serve -data`: `workouts.db`, `private/`, `tmp/`, `public/` (Data model, Files on disk; ADR-022). | |
| Staging folder | `tmp/{id}/`, where a new workout's files are written before one rename makes them `private/{id}/`; emptied at startup (ADR-022). | |
| Lifetime / expiry | Days until `expires_at`; then everything is deleted (UP-7). | |
| Public build | Building one workout's public files in memory: read `original.fit`, parse, apply the public transform (hiding, later trim and zones), write the artefacts (data file and `.br`; later preview, export in phase 3), then the view model and `index.html`. Artefacts see only the transform's output. Deterministic; used by publish, every later edit and regeneration (F-15 P-4, ADR-028). | Draft data file (`draft.bin`, never public) |
| Hides-more change | A change to a workout record that differs in any field outside the F-15 P-7 allow-list (title, description, expiry, publish time, visibility to unlisted, hiding off, download on). Its build removes `index.html` and the folder first, then writes as a first publish; anything not on the list counts as hiding more. | An in-place rebuild (every difference on the allow-list) |
| Generic page | The one static page served for every `/w/{public_id}` that has no published page (draft, expired, unknown): `build/web/generic.html`, status `404`, no script, no Open Graph data, byte-identical for every ID (F-15 P-12, P-13). | Not-found response (the owner API's `404` body) |
| Regeneration | Rebuilding all files of every live workout from its original on every deploy (Workout data file, Versioning; F-30). | Publishing |
| Kept workout | A workout whose rebuild failed during regeneration for a server-side reason (`canceled`, a time limit, an error not about the file). It is unchanged and serves its previous files until a later run rebuilds it (Versioning, ADR-005 item 9). | A workout deleted because its file is rejected |
| Cursor | The shared sample index (or none) that drives charts, map marker and readouts (AP-6); held in one cursor store per workout (`lib/cursor`, ADR-008). | Selection (a range, F-17); a time or pixel position |
| Units store | The page's one unit-system choice (metric or imperial): defaulted from the browser language, switched by the drop-down; a drop-down choice is remembered in `localStorage` through `lib/prefs` and wins over the default, never in a cookie or sent to the server (DS-1). One per page (`lib/units`, F-14 D-3), passed to every consumer. | Unit labels and factors (`lib/format`); the server markup, always metric (ADR-027) |
| Recording | `ingest`'s library-free extraction of a FIT file, input to normalisation (F-02). | Normalised workout |
| Session / lap | The device's `session` / `lap` with its totals as recorded (AP-2, AP-8); a lap value the device did not record is computed from the lap's samples ([F-18](../architect/F-18-laps-and-splits-table.md) L-1, [F-03](../architect/F-03-workout-data-file-writer.md) §5.3), and is absent, never 0, when the lap has no samples for it (e.g. gain without altitude). A workout has one or more sessions. | Split (computed per 1 km in the browser, F-18) |
| Time offset | Whole seconds since the workout's start time; the shared time axis. One axis across every session and FIT sequence, never rebased; the elapsed time the cursor shows is the sample's offset (F-08 D-19). | Elapsed time (a total) |
| Gap | Two consecutive samples 5 s or more apart; never filled. Drawn as a break on charts and, if both sides have a position, as a dashed bridge on the map (F-06). Shorter gaps are filled at 1 s (F-02 R-3). In the browser: an index in `WorkoutData.gaps` (F-04). | Pause (timer stopped); missing sample; GPS dropout |
| Missing sample | A present stream has no value at an index: the sentinel in the file, `NaN` in the browser (F-03 §7, ADR-007). | Gap; absent stream |
| Segment | A maximal run of consecutive samples of one stream that are not missing and do not cross a gap; a chart line is drawn only within a segment (F-05). | Session, lap |
| Break | Where a chart line is not drawn: between two segments, caused by a missing sample or a gap; never drawn as zero (F-05). | Pause (the cause of a gap) |
| Metric stream | A present stream other than time offset and position: altitude, distance, speed, heart rate, cadence, power, temperature (`WorkoutData.metrics`, F-04). | Totals (session values) |
| Decoded workout data | `WorkoutData`: the browser's form of a data file; `Float64Array`s in the format's units plus metadata (F-04, ADR-007). | Data file (the bytes) |
| Moving time | Time with the timer running, or above a speed threshold when there are no timer events (F-02 R-7). | Timer time (device total) |
| Device totals | Session values as the device recorded them; shown as recorded (AP-2), except that running cadence is doubled (F-02 R-5) and transition sessions and laps lose their speed, cadence and power totals (F-08 R-13). Lap values the device did not record are computed (see Session / lap). | Derived values (computed from streams) |
| Corpus / golden report | Test FIT files in `src/server/testdata/corpus/` and their expected JSON reports (ADR-004). | |
| Format version | The `uint16` in the data file header. Exactly one is live, except that kept workouts stay on their previous version and bundle until rebuilt; each bundle reads exactly one version (Versioning, ADR-005). | Report version (golden report shape) |
| Descriptor | The metadata entry for one column: name, type, encoding, scale, unit, byte offset (F-03). | |
| Sentinel | The stored value that marks a missing sample: the maximum of the column's type (F-03). | Absent stream (no column at all) |
| Pre-compressed file | `data.bin.br`, a byte-identical brotli transport encoding of `data.bin` (ADR-006). | A different format |
| Golden data file / fixture | `testdata/datafile/synthetic/<name>.bin` and `.json`: the writer's output and its decoded-integer digest, the contract for F-04 (F-03). | Golden report |
| Positioned sample | A sample whose position (latitude and longitude) is present (F-06). | A sample of a present position stream whose position is missing |
| Route | The line on the map through every positioned sample in index order, drawn as recorded segments (solid) and bridges (dashed) (AP-3, F-06). | The position stream itself; a lap |
| Recorded segment | A maximal run of positioned samples with no bridge between neighbours; drawn solid (F-06). | Segment (a chart term, F-05) |
| Bridge | The straight segment between two consecutive positioned samples with a GPS dropout **or** a gap between them; drawn dashed, because nothing was recorded along it (F-06). The two causes stay separate terms; on the map both look the same. | Break (where a chart line is not drawn) |
| GPS dropout | A run of samples without position between two positioned samples; the route crosses it with a dashed bridge (AP-3). | Gap (time; on the map also a dashed bridge, on charts a break); hidden section (start or end, removed at publish) |
| Route presence | `no-location-data` (no position stream: hint, no map), `no-visible-route` (stream present, no positioned sample), `route` (F-06). | |
| Cursor marker | The map's marker for the cursor; hidden when the cursor is none or its sample has no position (F-06). | Start/finish and lap markers |
| Start / finish marker; lap marker | Map markers at the first and last positioned sample; at the first sample of lap *k* ≥ 2, only if that sample has a position, labelled *k* (F-06). | Lap (the device's lap) |
| Tile host | The one external host the viewer's browser contacts: OpenFreeMap, `tiles.openfreemap.org` (ADR-014). | Our own origin |
| Range | A stretch of the workout between two sample indices `a < b`, both inclusive, selected on a chart or by a laps row; shaded on every chart, highlighted on the map, its values in the range panel (AP-7, F-17). | Cursor (one sample); lap; hidden section |
| Selection store | The page's one store for the range (or none), next to the cursor store and following its rules; written by chart gestures, the panel's clear control and laps rows, read by charts, map and panel (`lib/selection`, F-17 R-1). | Cursor store |
| Pinned map | On a phone in portrait (one-column layout), the map kept at the top of the screen, at most 40 % of its height, until the end of the laps, after the viewer taps the map's toggle (AP-18); off at every load, never stored (F-13 D-19). | Map pins (PIN-, phase 2); the desktop map, which stays in view without a toggle (AP-17) |
| Page view model | The page template's only input: display-ready strings (page, header, summary, route presence, chart count, laps; later groups from F-23, F-24), built by `lib/page` in TypeScript and by F-15 in Go (F-13 D-2, ADR-026). | The data file's metadata (its source) |
| Sample page / sample view model | A committed view model in `src/server/testdata/page/`, built from a synthetic golden data file plus fixed settings; the contract both builders reproduce exactly. protoserve serves the filled template for it at `/w/<name>` (F-13 D-3, B-1). | A published workout's page; bench pages (`bench/`, dev server only) |
| Flush | One delivery of a changed cursor value to every listener, at most once per animation frame (ADR-008). Its duration is split into charts and map in a scrub record (ADR-016). | A frame |
| Scrub record | The in-page record of frame intervals, flush times, cursor changes and finger contact during a 10 s scrub; the "trace" for the 60 fps gate (ADR-016, F-07). | Auto-scrub (synthetic events on a bench page, F-05/F-06) |
| FIT sequence / chained file | One FIT header, its messages and CRC / a file of two or more sequences back to back. Chained files become one workout; later sequences of another file type are skipped (F-08). | Session (a FIT message; one sequence can hold several) |
| Truncated file / data end | A file whose messages break off before their end (watch died, cut-off upload, unclosed file), or whose reading stops at a **rejected record**: a record the pinned decoder refuses, such as a definition with an invalid base type (UP-9 calls it a "damaged record"). Accepted when at least one record survives. The data end is the time offset of the last intact record, shown in the UP-3 notice (`Integrity.DataEnd`, F-08). | Gap; CRC failure |
| CRC failure | A wrong or missing header or file CRC, with the messages intact. Accepted, logged, no notice (F-08, OQ-33). | Truncated file |
| Rejection / rejection kind | A file `ingest` refuses, each kind with one fixed user message (ADR-018): `not_fit`, `unsupported_type`, `no_data`, `limit`, `internal` (always a defect on our side), `canceled` (the caller gave up), and, from `Accept` only, `unreadable`: a **deliberate refusal** of an upload that `Parse` accepts but that has no faithful stored form, for a cause on ADR-023 item 8a's closed list. | A recovery (accepted with `Integrity` set) |
| Acceptance rule | Upload may refuse a file that `Parse` accepts, but only as `unreadable` ("This file could not be read.") for a cause on ADR-023 item 8a's closed list, never as `internal`. No file is promised acceptance, from a real device or not: what errors a device may write cannot be known in advance. Privacy never bends: whatever the strip is unsure of is refused (ADR-023 item 8a). | Rejection; corpus strictness |
| Corpus strictness | Every corpus file that `Parse` accepts, synthetic and local real, must pass `Accept` in the tests. A regression check (the built code accepts them all), not a product promise about devices (ADR-004 rev. 11, F-09 D-29). | Acceptance rule |
| Parse limits | The time limit and the counted budgets (bytes, records, samples, laps, sessions, timer events, field descriptions, sequences, sport messages) plus parse slots, enforced in-process by `ingest.Parser` (ADR-017). The time limit is a setting (Conventions, Configuration); every other limit is fixed. | The upload size and rate limits (ADR-021) |
| Record totals | Totals computed from the samples (`Derived`: elapsed, distance, gain, loss, averages, maxima; R-11); used in the summary wherever the device recorded none (OQ-22, F-08). | Device totals; range statistics (F-17, in the browser) |
| Leg / record leg | A part of the workout the summary combines: each session is a leg; the record leg is the part after the last surviving session (the whole workout if there is no session) in a truncated or session-less file, with values from the records (F-08 R-10 step 0). | Per-leg sections (AP-12, phase 2, a display); session (a FIT message) |
| Sport hint / sport timeline | A sport statement outside the sessions: a `sport` message (placed at the next record) or `lap.sport`. The sport timeline gives every sample its sport: its session, else the latest hint since the previous session ended, else F-02's session fallback (F-08). | Workout sport (`multisport` when the legs' sports differ) |
| Combined totals | The summary of a workout with several legs: additive totals summed, except elapsed time, which is the wall time from the workout start to the last record (F-08 D-18); maxima combined, averages from the records; average speed and cadence hidden when the sports differ (F-08 R-10). The summary elapsed time is that wall time for every workout except one session recorded in one FIT sequence, which keeps the device's value (F-08 R-10.E). | Per-leg sections (AP-12, phase 2) |
| Transition / transition sample | A transition (T1, T2) is a session with FIT sport `transition`, and nothing else is (F-08 D-23). A transition sample is one covered by such a session: rule 1 of the sport timeline (the first session in file order whose span contains it) is a transition session. A sport hint or a fallback never makes a transition sample. At transition samples, and in transition session and lap totals, only position, altitude, heart rate, distance and temperature are kept; speed (so pace), cadence, power and any later stream are missing samples or absent totals, in the workout, the report and the data file (F-08 R-13, AP-4). | Gap; GPS dropout; a pause |
