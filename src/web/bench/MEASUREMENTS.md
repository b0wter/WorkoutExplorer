# Decode timing (F-04 AC 5)

What is timed: `decodeDataFile(buffer)` on the in-memory `ArrayBuffer` of the synthetic golden file `ride-12h.bin` (12 hours at 1 Hz, 43,200 samples, 10 streams, 1,176,640 bytes). Not the network, not brotli. Procedure and page: `architect/F-04-workout-data-file-reader.md` v1.1, "Decode timing"; `bench/decode.html`.

**Trigger:** cold median above 50 ms on the phone (ADR-007 rev. 1). Then decoding moves into a Web Worker, in a later change. Warm values are for information only. There is no target in AC 5; Vitest does not assert timings.

## Procedure

1. `npm run dev` in `src/web`, then open `/bench/decode.html`. On a phone, prefer Android Chrome remote debugging with port forwarding of the Vite port (nothing exposed on the network); otherwise `npm run dev -- --host` on the home network for the duration of the measurement only.
2. Press **Reset** to clear stored cold values.
3. 5 × (reload the page, press **Run**). Each page load allows exactly one cold run; after it the button reads "Reload the page for the next cold run".
4. Record the median of the 5 cold values and their min–max (both shown by the page), and the last load's warm mean and max.
5. The phone run counts only if the page shows `ok: true`, N 43200 and 10 streams (it is also the browser check, QA r1 Q2).

## Results

| Date | Commit | Device | OS | Browser and version | File | Samples | Cold median ms (5 loads) | Cold min–max ms | Warm mean ms | Warm max ms |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-11 | a00d10e + F-04 working tree (uncommitted) | Desktop, Intel Core i7-4770T @ 2.50 GHz (reference) | Fedora Linux 44 Server, x86_64 | Node.js 22.22.2 (V8), not a browser: this machine has none. 5 separate Node processes stand in for 5 page loads, each running the `bench/decode.ts` procedure (1 cold, 20 warm) on the bundled `lib/datafile` | ride-12h.bin | 43,200 | 20.2 | 19.8–22.5 | 3.81 | 8.5 |
| 2026-10-12 (reported) | not reported | iPhone 13 mini (operator) | iOS | Orion (iOS WebKit) | ride-12h.bin | 43,200 | 31 | not reported | not reported | not reported |

Phone row: ok: true, 10 streams; 31 ms < 50 ms (ADR-007 rev. 1), so no Worker; also the bench page's browser check (OQ-11 closed).

# F-05 scrubbing

What is measured: scrubbing the synthetic golden file `ride-12h.bin` (43,200 samples, all six charts) on `bench/charts.html`. Procedure and pass rule: `architect/F-05-stacked-charts-with-shared-cursor.md` v1.0, "Performance (NFR: 60 fps on a 12-hour workout)" and "Dev page `bench/charts.html`". The phone run is F-07. Vitest asserts no timings.

**Pass:** at least 99 % of frame intervals ≤ 20 ms, **and** flush p95 ≤ 2 ms **and** flush max ≤ 8 ms. Flush = time from the listener subscribed before `ChartStack` to the one subscribed after it (charts and readout only).

## Procedure

1. `npm run dev` in `src/web`, then open `/bench/charts.html` on a desktop browser. Window width at least 1,200 CSS px.
2. Pick **ride-12h** (the default) and wait until **Auto-scrub 10 s** is enabled (every chart has its data).
3. Close DevTools, keep the mouse off the page, press **Auto-scrub 10 s**. Wait 10 s and copy the result block into a row below.
4. Confirmation (no row, note it under the table): one manual 10 s hover scrub recorded in the DevTools Performance panel. No long task (> 50 ms) and no dropped frames in the Frames track during the scrub.
5. Information only: repeat step 3 with DevTools CPU throttling 4× (Throttling column "4×"). It does not decide the pass.

## Results

| Date | Commit | Device | OS | Browser and version | File | Charts | Plot width | DPR | Throttling | Frames | Interval median/p95/max ms | Share ≤ 20 ms | Flush median/p95/max ms | Pass |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-11 | 699a1d1 + F-05 working tree (uncommitted) | Desktop, Intel Core i7-4770T @ 2.50 GHz | Fedora Linux 44 Server, x86_64 | **Not a browser:** Node.js 22.22.2 with jsdom 26.1.0 (one-off scratch run, not in the repo), canvas stubbed, frames from 16 ms timers. Checks the page's logic (auto-scrub, frame monitor, flush timing, report) only. **Not AC 7 evidence** | ride-12h.bin | 6 | 1136 | 1 | none | 603 | 16.51 / 17.63 / 44.69 | 99.83 % | 1.05 / 1.53 / 2.48 | n/a (not a browser) |
| 2026-10-12 | ff21c02 | Operator's desktop (OQ-12) | Linux x86_64 | Firefox 156.0 (`Mozilla/5.0 (X11; Linux x86_64; rv:156.0) Gecko/20100101 Firefox/156.0`) | ride-12h.bin (N 43,200) | 6 | 2,217 (window 2,313) | 1.5 | not stated | 599 | 17.06 / 17.08 / 33.12 | 99.83 % | 0.00 / 1.00 / 2.00 (599 flushes) | **PASS** |
| pending | | Operator's desktop (OQ-12), information only | | | ride-12h.bin | 6 | | | 4× | | | | | – |

### Manual checks (QA r3 list)

- **Desktop, Firefox 156 on Linux:** D2, D3, D4, D5, D8 (works down to about 300 px wide), D9, D10, D11, D12, D13 and D14 pass. D1, D6 and D7 passed earlier.
- **Phone, iPhone 13 mini with Orion (iOS WebKit):** P1, P5, P7, P8, P9, P10, P11, P12 and P13 pass. P2 (information only): the cursor now keeps following when the drag drifts onto the next chart; the operator confirmed it keeps following across charts during one drag. P3 passes: touch-and-hold sets the cursor, and a vertical swipe scrolls the page and restores it.
- **Android:** not checked.

# F-06 map

What is measured, on `bench/map.html` with the synthetic golden file `ride-12h.bin` (43,200 samples, 24 laps, six charts, the map above the charts): scrubbing with the map visible (MD10, MP6), and the start-up window from charts `onready` (MD11, MP1). Procedure, pass rule and trigger: `architect/F-06-route-map-linked-to-cursor.md` v1.4, "Dev page `bench/map.html`" and "Manual checks (operator)"; ADR-013 rev. 2. Vitest asserts no timings.

- **Scrubbing pass (desktop):** F-05's rule, now including the map marker: at least 99 % of frame intervals ≤ 20 ms, **and** flush p95 ≤ 2 ms **and** flush max ≤ 8 ms. Flush = time from the listener subscribed before `ChartStack` and the map to the one subscribed in `onmapready` (charts, readout and marker). Phone: information only (the gate is F-07).
- **Start-up trigger (v1.4, ADR-013 rev. 2):** check MP1 fails, **or** the longest frame interval on the phone, measured after the v1.4 changes with Low Power Mode off and no 30 Hz cap, exceeds **300 ms** (the user accepted OQ-21 on 14 Oct 2026). Then the remedies of ADR-013 rev. 2 apply, in their order.

## Procedure

1. `npm run dev` in `src/web`, then open `/bench/map.html`. Desktop: Firefox, window at least 1,200 CSS px. Phone: `npm run dev -- --host` on the home network for the duration of the run only (F-04 "Phone access"). Phone: **Low Power Mode off** (the battery icon is not yellow).
2. **Start-up (MD11, MP1):** reload the page with **ride-12h** and Tiles **normal**. The page measures for 5 s after the charts are ready, then 2 s more with nothing happening (frame-rate baseline; keep still), and prints the "Start-up window" block; the first show after a page load is marked COLD. Record one row per reload, five reloads, and copy the block (with its step table) below the table. If the block says **"30 Hz cap: the run does not count"**, repeat the reload; if the cap remains with Low Power Mode off, repeat once in Safari and record both. On the phone, swipe vertically on a chart again and again from the moment the charts appear until the map has tiles (MP1).
3. **Scrubbing (MD10, MP6):** wait until **Auto-scrub 10 s** is enabled (after `onmapready`). Close DevTools, keep the mouse off the page, keep the map in view (the result says "Map visible"), press the button and copy the result block into a row.
4. Optional: repeat 2 with Tiles **blocked** (the Tiles column says so).

The dev server runs with `validateStyle` on (`import.meta.env.DEV`); the production build has it off (v1.4), so dev-server start-up numbers are an upper bound for that step.

## Scrubbing with the map

| Date | Commit | Device | OS | Browser and version | File | Charts | Map visible | Plot width | DPR | Throttling | Frames | Interval median/p95/max ms | Share ≤ 20 ms | Flush median/p95/max ms | Pass |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-14 | 80c79fa | Operator's desktop (OQ-19), MD10 | Linux x86_64 | Firefox 156 | ride-12h.bin | 6 | yes | 2,217 (window 2,313) | 1.5 | none | 598 | 17.06 / 17.08 / 33.14 | 99.67 % | 0.00 / 1.00 / 1.00 (596 flushes) | **PASS** |
| 2026-10-14 | 80c79fa + v1.4 `map-view`/`map-style` changes (uncommitted, dev server) | Operator's phone (OQ-19), iPhone 13 mini, MP6, information only | iOS 27, Low Power Mode off | Orion (iOS WebKit) | ride-12h.bin | 6 | yes | 279 (window 375) | 3 | none | 598 | 17.00 / 18.00 / 45.00 | 99.16 % | 0.00 / 1.00 / 1.00 (594 flushes) | PASS (information; the gate is F-07) |

## Map start-up

Columns from design v1.4. "Time to `onmapready`" is counted from charts `onready` (the page also prints both times since navigation start). "Baseline" is the median frame interval of the 2 idle seconds after the window (≥ 25 ms: 30 Hz cap, the run does not count). Rows before v1.4 have no baseline and no step column (the page did not measure them).

| Date | Commit | Device | OS | Browser | File | Tiles | Low Power Mode | Baseline median interval ms | Time to `onmapready` ms | Longest frame interval ms | Intervals > 50 ms | Step at the longest interval |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's desktop (OQ-19), MD11, reload 1 | Linux x86_64 | Firefox 156 | ride-12h.bin | normal | n/a | – | 271 | 129.6 | 2 of 289 | – |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's desktop, MD11, reload 2 | Linux x86_64 | Firefox 156 | ride-12h.bin | normal | n/a | – | 281 | 133.6 | 2 of 286 | – |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's desktop, MD11, reload 3 | Linux x86_64 | Firefox 156 | ride-12h.bin | normal | n/a | – | 268 | 133.8 | 2 of 289 | – |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's desktop, MD11, reload 4 | Linux x86_64 | Firefox 156 | ride-12h.bin | normal | n/a | – | 256 | 133.6 | 2 of 288 | – |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's desktop, MD11, reload 5 | Linux x86_64 | Firefox 156 | ride-12h.bin | normal | n/a | – | 390 | 216.9 | 2 of 279 | – |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's phone (OQ-19), iPhone 13 mini, dpr 3, MP1, reload 1. **Superseded** by the re-run below | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | unknown, likely on (about 37 ms average) | – | 362 | 379.0 | 8 of 124 | – |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's phone, MP1, reload 2. **Superseded** | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | unknown, likely on (about 37 ms average) | – | 226 | 269.0 | 7 of 135 | – |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's phone, MP1, reload 3. **Superseded** | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | unknown, likely on (about 37 ms average) | – | 209 | 264.0 | 4 of 137 | – |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's phone, MP1, reload 4. **Superseded** | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | unknown, likely on (about 37 ms average) | – | 212 | 276.0 | 4 of 135 | – |
| 2026-10-14 | 80c79fa (before v1.4) | Operator's phone, MP1, reload 5. **Superseded** | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | unknown, likely on (about 37 ms average) | – | 252 | 297.0 | 6 of 132 | – |
| 2026-10-14 | 80c79fa + v1.4 `map-view`/`map-style` changes (uncommitted, dev server, `validateStyle` on); page without baseline and step table yet | Operator's phone, iPhone 13 mini, dpr 3, MP1 re-run, reload 1 (likely a cold cache) | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | **off** | – (not yet measured; 290 intervals in 5 s, so not capped) | 683 | 93.0 | 4 of 290 | – |
| 2026-10-14 | as above | Operator's phone, MP1 re-run, reload 2 | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | **off** | – (298 intervals in 5 s) | 335 | 62.0 | 1 of 298 | – |
| 2026-10-14 | as above | Operator's phone, MP1 re-run, reload 3 | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | **off** | – (298 intervals in 5 s) | 105 | 48.0 | 0 of 298 | – |
| 2026-10-14 | as above | Operator's phone, MP1 re-run, reload 4 | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | **off** | – (294 intervals in 5 s) | 88 | 69.0 | 3 of 294 | – |
| 2026-10-14 | as above | Operator's phone, MP1 re-run, reload 5 | iOS 27 | Orion (iOS WebKit) | ride-12h.bin | normal | **off** | – (300 intervals in 5 s) | 69 | 37.0 | 0 of 300 | – |

**Reading (14 Oct 2026).** The first phone run (124–137 intervals in 5 s, about 37 ms average) looks capped at 30 Hz, probably by Low Power Mode, so its longest intervals of 264–379 ms are not comparable; they fired the v1.0 trigger and led to design v1.4. The re-run with Low Power Mode off and the v1.4 `map-view`/`map-style` changes in the tree (dev server, so `validateStyle` still on) has a longest interval of 37–93 ms, under the 300 ms trigger (and under the v1.0 assumption of 100 ms). MP1 passes on this re-run (operator, 14 Oct 2026); the earlier 264–379 ms rows are superseded. No further start-up run was needed to close F-06, so no row has the baseline or step columns yet.

Step tables: none recorded. The page prints them since v1.4 (frame-rate baseline, "Steps", "Frame intervals > 50 ms"); paste the blocks here with any future start-up row.

### Manual checks (MD1–MD12, MP1–MP10)

- **Desktop, Firefox 156 on Linux (80c79fa):** MD1–MD12 pass. MD9: the attribution passes, and (14 Oct 2026) the network part passes: the only tile host contacted is `tiles.openfreemap.org`. MD10: 99.67 % of frames ≤ 20 ms (row above). MD11: recorded above.
- **Phone, iPhone 13 mini with Orion (iOS WebKit) (80c79fa):** MP1–MP10 pass (subjectively, including MP1 a and b); see the v1.4 re-checks below.
- **v1.4 re-checks (14 Oct 2026):** MD5 passes (the dashed bridge over the GPS dropout). MP7 passes: GPS dropouts are drawn as a dashed bridge on the phone. MP1 passes on the re-run with Low Power Mode off (longest frame 37–93 ms; the 264–379 ms rows are superseded). MP6 phone scrub: 99.16 % of frames ≤ 20 ms (information).
- **Android:** not checked.

MP5 (design v1.2) also covers: a double tap and two quick scrubs on the route must not zoom the map, and the + button where it lies over the route zooms without moving the cursor.

# F-07 prototype and gate

What is measured: the prototype page (the production build, served with brotli by `make prototype-serve`) with the synthetic data files `ride-12h` (43,200 samples, every stream, 24 laps; the gate file) and `ride-4h`. Procedure, pass rule and validity rules: `architect/F-07-prototype-page-and-scrubbing-gate.md` v1.4 ("Operator procedure", "Manual checks (operator)", "What to record") and `architect/ADR-016-scrubbing-smoothness-measurement-and-gate.md` (the in-page scrub record, `?measure`). Vitest asserts no timings.

- **Pass (ADR-016 decision 2):** at least 99 % of frame intervals ≤ 20 ms, **and** total flush p95 ≤ 2 ms, **and** total flush max ≤ 8 ms. Flush = from listener A (before the charts) to listener Z (after the map marker), split into charts (A to M) and map (M to Z).
- **A run counts only if** (ADR-016 decision 3): median frame interval below 25 ms; in a touch run a finger was down in every frame of the window, including the frames before the first touch (a late finger does not count, design v1.2); the cursor changed in at least 75 % of frames; the map was in view at the start. A run that does not count is repeated, and its block is still copied below.
- **Gate 0 (ADR-016 decision 4):** on the reference phone, three counted chart touch scrubs (PP2) and three counted route touch scrubs (PP3) each pass, and the operator judges every one smooth. Conditions: ADR-016 decision 5 (OQ-27, closed: accepted by the user).
- **Brotli needs `-tls`** (design v1.4, procedure step 4): over plain HTTP neither Firefox 156 nor Orion sent `Accept-Encoding: br`, so the data file arrived uncompressed (1,176,940 B transferred on the phone). All counted runs below were taken with `make prototype-serve PROTO_ADDR=:18090 PROTO_FLAGS=-tls`.
- **Synthetic files only.** Real files (PP8, OQ-29) are recorded in the QA report, never here; never paste `protoserve` log lines for a real file.
- **Android: not checked** (out of scope until F-32, D-11).

All rows: 15 Oct 2026, commit `9ccd744`, production build. The operator reported the values below as numbers; the raw blocks and `protoserve` log lines were not pasted, so cells the report did not give say "not reported".

## Transfer sizes

One cold load per device. Source: the Resource Timing values in the Load block (encoded body size), not the `protoserve` log lines; the other resources were not reported.

| Date | Commit | Device | Scheme (http/https) | Resource | Encoding | Bytes |
| --- | --- | --- | --- | --- | --- | --- |
| 2026-10-15 | 9ccd744 | Operator's phone (Orion) | https | `data/ride-12h.bin` | br | 266,312 encoded (266,612 transferred) |
| 2026-10-15 | 9ccd744 | Operator's phone (Orion), earlier load | http | `data/ride-12h.bin` | identity (no `br` advertised) | 1,176,940 transferred |
| 2026-10-15 | 9ccd744 | Operator's desktop (Firefox 156) | https | `data/ride-12h.bin` | br | 266,150 encoded (266,450 transferred) |
| 2026-10-15 | 9ccd744 | both | – | `index.html`, entry JS, entry CSS, `data/ride-4h.bin`, map-view JS, map-view CSS, worker, recorder JS | – | not reported |

`make prototype-data` on the dev machine: `ride-4h` br 106,951 B (< 500,000, AC 4) and `ride-12h` br 266,150 B.

## Bundle

Bundle impact procedure of the design (B0 = F-06 measurement (b) before F-07; B0′ = B0 plus `loadDataFile`; E = `make size`; F-07's share is E − B0′, ceiling 300, ADR-011 rev. 7; M0, M1 = `map-view-*.js` gzip before and after). B0, B0′ and M0 were measured on `f037688` (before F-07); E, the recorder and M1 on `9ccd744` (`make size`, re-checked when recording).

| Date | Commit | B0 | B0′ | E | E − B0′ | recorder gzip | M0 | M1 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-15 | 9ccd744 | 42,525 | 45,328 | 45,582 | 254 | 2,434 | 283,994 | 284,135 |

## Load

From the Load block of `?measure` (PD1, PP1).

| Date | Commit | Device | Browser | File | Data transfer B | Request to response end ms | Body read + decode ms | Data ready / Charts ready / Map ready ms since navigation start |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, iOS 27, dpr 3 | Orion, https | ride-12h | 266,612 (encoded 266,312, decoded 1,176,640) | 111 | 28 | 444 / 563 / 770 |
| 2026-10-15 | 9ccd744 | Operator's phone, earlier load | Orion, https | ride-12h | not reported | 24 | 5 | 173 / 235 / 295 |
| 2026-10-15 | 9ccd744 | Operator's desktop, Linux, dpr 1.5 | Firefox 156, https | ride-12h | 266,450 (encoded 266,150) | 4 | 7 | 91 / 134 / 448 |

Body read + decode stays far under the ADR-007 trigger of 50 ms on the phone (28 ms and 5 ms), so `bench/decode.html` did not need a re-run.

Load blocks: not pasted by the operator (values above transcribed from the report).

## Scrub records

One row per run (PD4, PD5, PP2, PP3), counted or not. Flush total: the number of flushes in parentheses where reported. "Map p95/max" for phone route run 1: the operator reported a map max of 3 ms (all of that run's flush max).

| Date | Commit | Device | OS | Browser | Low Power Mode | File | Input | Started on | Frames | Interval median/p95/max ms | Share ≤ 20 ms | Flush total p95/max | Charts p95/max | Map p95/max | Changed % | No-finger frames | Map in view | Counts | PASS | Smooth (operator) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait | iOS 27 | Orion (iOS WebKit, UA Version/27.0 Safari), https (`-tls`) | off | ride-12h | touch | chart (run a) | 600–601 | 17 / 18 / 22 | 99.67 % | 1 / 2 | not reported | – | 85.17 % | 0 | yes | yes | yes | yes |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait | iOS 27 | Orion (iOS WebKit, UA Version/27.0 Safari), https (`-tls`) | off | ride-12h | touch | route (run a) | 600–601 | 17 / 18 / 21 | 99.83 % | 1 / 2 | not reported | – | 98.00 % | 0 | yes | yes | yes | yes |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait | iOS 27 | Orion (iOS WebKit, UA Version/27.0 Safari), https (`-tls`) | off | ride-12h | touch | chart (run 1) | 600–601 | 17 / 18 / 22 | 99.83 % | (561) 1 / 1 | not reported | – | 93.50 % | 0 | yes | yes | yes | yes |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait | iOS 27 | Orion (iOS WebKit, UA Version/27.0 Safari), https (`-tls`) | off | ride-12h | touch | chart (run 2) | 600–601 | 17 / 17 / 32 | 99.67 % | (563) 1 / 1 | not reported | – | 93.83 % | 0 | yes | yes | yes | yes |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait | iOS 27 | Orion (iOS WebKit, UA Version/27.0 Safari), https (`-tls`) | off | ride-12h | touch | chart (run 3) | 600–601 | 17 / 17 / 30 | 99.67 % | (565) 1 / 1 | not reported | – | 94.01 % | 0 | yes | yes | yes | yes |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait | iOS 27 | Orion (iOS WebKit, UA Version/27.0 Safari), https (`-tls`) | off | ride-12h | touch | route (run 1) | 600–601 | 17 / 17 / 24 | 99.83 % | (598) 1 / 3 | not reported | – / 3 | 99.67 % | 0 | yes | yes | yes | yes |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait | iOS 27 | Orion (iOS WebKit, UA Version/27.0 Safari), https (`-tls`) | off | ride-12h | touch | route (run 2) | 600–601 | 17 / 18 / 25 | 99.50 % | (600) 1 / 1 | not reported | – | 100.00 % | 0 | yes | yes | yes | yes |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait | iOS 27 | Orion (iOS WebKit, UA Version/27.0 Safari), https (`-tls`) | off | ride-12h | touch | route (run 3) | 600–601 | 17 / 17 / 33 | 99.50 % | (599) 1 / 1 | not reported | – | 99.67 % | 0 | yes | yes | yes | yes |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait, earlier run | iOS 27 | Orion, http (no brotli) | off | ride-12h | touch | chart | not reported | not reported / not reported / 57 | 97.48 % | – / 15 | – / 9 | not reported | 39.93 % | 247 | yes | NO (no finger down in 247 frames; cursor still) | NO | – |
| 2026-10-15 | 9ccd744 | Operator's phone, iPhone 13 mini, dpr 3, window 375 CSS px, portrait, earlier run | iOS 27 | Orion, http (no brotli) | off | ride-12h | touch | chart | not reported | not reported / not reported / 136 | 98.31 % | not reported | not reported | not reported | not reported | 51 | yes | NO (no finger down in 51 frames) | NO | – |
| 2026-10-15 | 9ccd744 | Operator's desktop, dpr 1.5, window 2,313 CSS px | Linux x86_64 | Firefox 156, https (`-tls`) | n/a | ride-12h | mouse | – (hover on the chart, run 1) | not reported | 17.06 / 17.08 / 33.12 | 99.83 % | – / 1 | not reported | not reported | 91.99 % | – | yes | yes | yes | – |
| 2026-10-15 | 9ccd744 | Operator's desktop, dpr 1.5, window 2,313 CSS px | Linux x86_64 | Firefox 156, https (`-tls`) | n/a | ride-12h | mouse | – (hover on the route, run 2) | not reported | 17.06 / 17.08 / 17.10 | 100.00 % | – / 1 | not reported | not reported | 86.33 % | – | yes | yes | yes | – |
| 2026-10-15 | 9ccd744 | Operator's desktop, dpr 1.5, window 2,313 CSS px | Linux x86_64 | Firefox 156, https (`-tls`) | n/a | ride-12h | mouse | – (hover on the chart, run 3) | not reported | 17.06 / 17.08 / 83.36 | 99.83 % | – / 1 | not reported | not reported | 91.95 % | – | yes | yes | yes | – |
| 2026-10-15 | 9ccd744 | Operator's desktop, dpr 1.5, window 2,313 CSS px | Linux x86_64 | Firefox 156, https (`-tls`) | n/a | ride-12h | mouse | – (hover on the chart, run 4) | not reported | 17.06 / 17.08 / 17.18 | 100.00 % | – / 2 | not reported | not reported | 97.83 % | – | yes | yes | yes | – |
| 2026-10-15 | 9ccd744 | Operator's desktop, dpr 1.5, window 2,313 CSS px | Linux x86_64 | Firefox 156, https (`-tls`) | n/a | ride-12h | mouse | – (hover on the route, run 5) | not reported | 17.06 / 17.08 / 17.10 | 100.00 % | – / 1 | not reported | not reported | 83.83 % | – | yes | yes | yes | – |
| 2026-10-15 | 9ccd744 | Operator's desktop, dpr 1.5, window 2,313 CSS px | Linux x86_64 | Firefox 156, https (`-tls`) | n/a | ride-12h | mouse | – (hover on the route, run 6) | not reported | 17.06 / 17.08 / 17.08 | 100.00 % | – / 1 | not reported | not reported | 72.33 % | – | yes | NO (cursor still in more than 25 % of frames) | yes | – |
| 2026-10-15 | 9ccd744 | Operator's desktop, dpr 1.5, window 2,313 CSS px, earlier run | Linux x86_64 | Firefox 156, http (no brotli) | n/a | ride-12h | mouse | – | not reported | not reported | 100.00 % | not reported | not reported | not reported | 59.23 % | – | yes | NO (cursor still in more than 25 % of frames) | not reported | – |

Operator's judgement, phone: smooth, "pretty much instant" (every counted run). Desktop runs are information only; no judgement was asked for.

Scrub record blocks: not pasted by the operator (values above transcribed from the report).

## Gate 0 verdict

| Date | Commit | PASS or FAIL | The rows it rests on |
| --- | --- | --- | --- |
| 2026-10-15 | 9ccd744 | **PASS** | Phone (reference, ADR-016 decision 5): chart runs 1–3 and route runs 1–3, all counted, all PASS, all judged smooth (plus the earlier counted chart run a and route run a, also PASS). Desktop (information): chart runs 1, 3, 4 and route runs 2, 5 counted and PASS; route run 6 did not count (cursor still in more than 25 % of frames; no verdict, timings are information only) |

## Checks

All reported by the operator on 15 Oct 2026, commit `9ccd744` ("all seems good so far").

- PD1: pass.
- PD2: pass.
- PD3: pass (the cursor stays when the mouse leaves a chart or the route, OQ-13).
- PD4: pass (rows above).
- PD5: phone gate passed (3 of 3 chart runs + 3 of 3 route runs counted and PASS). Desktop (information only): 2 of 3 counted route runs (runs 2 and 5), both pass; run 6 did not count.
- PD6: zoom: pass (the map is sharp again after browser zoom without a reload, QA-06-r4-1); monitor move: not separately reported.
- PD7: pass.
- PD8: pass.
- PP1: pass.
- PP2: pass (gate rows above).
- PP3: pass (gate rows above).
- PP4: pass.
- PP5: pass.
- PP6: pass.
- PP7: pass.
- PP8 (optional): pass; a real long ride was loaded for information. Its results belong in the QA report, not here (design v1.4, PP8; aggregates in the F-07 QA r3 report).
- Android: not checked.
