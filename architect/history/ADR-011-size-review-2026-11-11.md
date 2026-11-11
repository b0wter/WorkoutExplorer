# ADR-011 size review, 11 Nov 2026

Analysis only, no code changed. Build of `48573fb`, Vite 8.3.4 (rolldown, oxc minifier), Svelte 5.57.2, uPlot 1.6.32. Entry `index.html` = `edit.html` (same chunk): **59,485 / 60,000 gzip-9** (JS 55,148 + CSS 4,337), reserve 515. Method: sourcemap attribution of the minified entry chunk; "removal Δ" = gzip(chunk) − gzip(chunk without those bytes), the closest estimate of a saving. Config experiments ran with a scratch Vite config (ADR-002 measurement, same numbers as CI for the base).

## 1. Breakdown of the entry (gzip bytes)

| Part | Removal Δ | Share of 59,485 |
| --- | --- | --- |
| uPlot (JS) | 22,617 | 38 % |
| Svelte runtime (`node_modules/svelte`) | 13,715 | 23 % |
| `page/` glue (start, enhance, laps, splitter, strip, map-props, load-error) | 4,607 | 8 % |
| CSS (`page.css` 3,232 standalone; uPlot CSS ≈ 400; component styles ≈ 700) | 4,337 | 7 % |
| `lib/datafile` (decode 1,440, meta 910, format 246, load 155) | 2,863 | 5 % |
| `components/charts` (MetricChart 1,611, ChartStack 894, CursorReadout 246) | 2,745 | 5 % |
| `lib/charts` (gesture 857, specs 616, decimate 327, axis 179) | 1,975 | 3 % |
| `components/map` (touch-quiet ≈ 300 + Vite preload helper ≈ 600, RouteMap 520) | 1,453 | 2 % |
| `lib/range` 754, `lib/page` 742, `lib/format` 648, RangePanel 561 | 2,705 | 5 % |
| `lib/prefs` 334, `lib/cursor` 282, `lib/selection` 204, `lib/rangelink` 162, `lib/units` 143, `lib/route` 60, `main.ts` 272, chunk map 280 | 1,737 | 3 % |

Compiled Svelte component code (5 components + `map-props.svelte.ts`): 3,509 removal Δ.

## 2. Already lazy (not counted, ADR-002 item 1)

| Chunk | JS + CSS gzip | Loaded when |
| --- | --- | --- |
| MapLibre (`map-style`) | 283,057 + 10,458 | map start (F-06) |
| `map-view` (F-06, F-25, F-26) | 6,802 + 764 | map start |
| Owner UI `edit/start.svelte` (F-19, F-20) | 7,954 + 1,002 | edit page only |
| `basemap` (F-19 preview) | 620 + 103 | edit page |
| Scrub recorder (ADR-016) | 2,434 | `?measure` only |
| Report form (F-28) | 1,206 + 531 | Report click |
| How to use (F-39) | 825 + 381 | button click |
| Sections (F-42) + shared `lib/sections` | 371 + 263 + 825 | multisport only (lib/sections also map, owner) |

## 3. Candidates

"Real" = fewer bytes downloaded per view; "budget" = bytes move out of the counted entry but still load on most views.

| # | Candidate | Est. saving (gzip) | Real or budget | Cost | Touches |
| --- | --- | --- | --- | --- | --- |
| C1 | Drop unused uPlot statics at build (`join`, `fmtDate`, `tzDate`, `sync`, `rangeLog`, `rangeAsinh`, paths `stepped`, `bars`, `spline`) via a ~10-line `transform` plugin in `vite.config.ts`; only `uPlot.pxRatio` is used | **2,566 measured** | real | Build step tied to uPlot's dist text: needs a check that the pattern matched (upgrade guard). No runtime change | ADR-009 (consistent with "uPlot only draws"), ADR-001 build |
| C2 | Replace `uplot/dist/uPlot.min.css` by the ~6 rules used (no legend, select, cursor, title rules) | **398 measured** | real | Small; pixel check of axes/canvas | ADR-009 |
| C3 | Svelte `compilerOptions.discloseVersion: false` | **22 measured** | real | none | none |
| C4 | Post-paint chunk: RangePanel + `lib/range` + splits table and switch + `enhanceUnits` with `lib/page` + range-copy routine (+ their CSS ≈ 250 est.), `import()` started at page start in parallel with `data.bin` | **≈ 3,100** (3,083 JS removal Δ + CSS − call site ≈ 150) | budget | Panel, Splits switch and imperial rewrite wait for a ≈ 3.5 KB chunk (in practice before `data.bin` arrives); one more request. No layout shift (slots reserved, L-6, R-9). No effect on scrub. Variant "load on first use" makes it real for non-interacting viewers but delays the first range panel by one round trip | ADR-011 decision 1 (F-17, F-18 listed as static), ADR-027 (units in place, unchanged), F-18 L-7 order (taps already wait for the panel) |
| C5 | Map-ready code into the `map-view` chunk: pin toggle (F-13 D-19), fullscreen strip (F-33, `strip.ts`) + their CSS (≈ 200) | **≈ 570** (371 JS + ≈ 200 CSS) | budget (map pages) | None visible: both are created at map ready already | ADR-011 item 2 (applies its own rule), ADR-038 |
| C6 | Rewrite the 5 static Svelte components (ChartStack, MetricChart, CursorReadout, RangePanel, RouteMap) as plain TS; Svelte stays in the owner chunk only | **≈ 12,000–14,000** (runtime 13,715 + compiled 3,509 − plain-TS replacement ≈ 2,500–3,500; est.) | real | Largest effort (≈ 900 lines + DOM tests rewritten from `mount`); runtime is all-or-nothing, partial rewrites save little. Scrub: neutral (hot path already bypasses reactivity). Edit page then loads the runtime lazily (+1 chunk) | TD Front end stack (plain TS is the TD's named fallback; ADR-024 did it for the upload page), ADR-026 (adopted, not hydrated, already limits Svelte to slots), ADR-012 |
| C7 | Own canvas renderer instead of uPlot (ADR-009 "Would choose differently if", ADR-011 remedy 3) | **≈ 16,000–19,000** (uPlot 22,617 + CSS ≈ 770 − renderer ≈ 4,000–6,000; est.); ≈ 2,600 less if C1/C2 are done | real | Axis ticks and labels, gaps, DPR, resize, alignment (ADR-009 item 5) re-done and re-tested; regression risk on axes; scrub unaffected (cursor is a div) | ADR-009 (superseded), ADR-011 |
| C8 | Deep-strip a vendored uPlot (cursor, legend, select, sync, time axis code inside the closure) | ≈ 2,000–4,000 beyond C1 (not measured) | real | A fork of ≈ 6,000 lines to maintain; upgrades manual | ADR-009 |
| C9 | Splitter drag code (`addSplitter`) behind `import()`; `restoreSplit` stays static | ≈ 300 | budget | Handle works one chunk later | F-13 D-10, ADR-011 |
| C10 | `build.modulePreload: false` | **123 measured** | real | Lazy chunks' imports load as a waterfall: map start one round trip later (map-view → map-style) | ADR-011 rev. 4 (helper), F-06 |
| C11 | `build.target: 'esnext'` | **39 measured** (CSS 38, JS 1) | real | Drops older-browser syntax lowering; no gain worth it | TD browsers |

Rejected (no or negative gain, measured on the unminified entry, oxc baseline 54,956):

| Option | Result |
| --- | --- |
| terser, default / passes 3 + unsafe | +179 / +8 |
| terser, mangle props `^_` | +1 |
| esbuild minify | +3,398 |
| Merge the two dark-theme blocks (`light-dark()`) | −59 at most; charts read custom properties, which `light-dark()` would leave unresolved |
| Lazy copy routines (Copy link) | ≈ 270; Safari may drop the clipboard user activation across an async `import()` |
| Lazy F-39 hint, theme/units selects, expiry | ≤ 200 each; hint box is in reserved first-paint space (layout shift), selects appear later |
| Edit-only branches in the viewing entry (`EditSeam`, `hidden`, `main.ts` switch) | < 100 total; no duplication between entries (index and edit load the same chunk) |
| `lib/datafile/meta.ts` slimming | 910 total; it is the graceful-reject validation of the data file, keep |

## 4. Budget facts (decision is the user's)

| Fact | Value |
| --- | --- |
| TD budget | "App bundle, Svelte + uPlot + app code: 60 KB" compressed, first visit; ADR-002: gzip-9 sum per entry |
| Entry today, gzip-9 / brotli (q11) / zstd (level 3) | 59,485 / 53,185 / 61,911 |
| What Caddy serves for `/assets/*` | on-the-fly `encode zstd gzip` (zstd first); no `precompressed br` there. Chrome/Firefox likely get ≈ 61.9 KB zstd, Safari gzip. Pre-compressing the bundle with brotli at build (Node `zlib`, no dependency) would cut transfer to ≈ 53.2 KB for all; ADR-002 "Would choose differently if" ties a brotli measurement to F-31 |
| Critical path | JS is a deferred module script; only `main.css` (4,337) blocks render. LCP candidates (h1, summary) are server HTML; chart canvases are not LCP candidates |
| LCP measured | never: F-32 U-2 and F-13 AC 7 (OQ-64) are open. ADR-011 "Would choose differently if" ties a budget raise to that measurement |
| Cost of 10 KB at Lighthouse Slow 4G (1.6 Mbit/s) | ≈ 50 ms, first visit only (assets immutable, cached a year) |
| Competing download | `data.bin` up to 500 KB (4-hour ride) loads in parallel and dominates time to charts |
