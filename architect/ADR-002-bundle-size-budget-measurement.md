# ADR-002 How the app bundle budget is measured and enforced

- **Status:** accepted
- **Date:** 10 Oct 2026; note of 29 Oct 2026 (F-19): HTML entries recovered from the built HTML (item 1)
- **Features:** F-01 (introduces the check); every front-end feature is bound by it, notably F-05, F-06, F-13, F-32
- **Supersedes:** none
- **Requirements:** NFR Performance (page budgets); TD Front end, Budget: "App bundle, Svelte + uPlot + app code: 60 KB" compressed transfer, first visit, "enforce the bundle budget in CI with a size check"

## Context

The TD fixes a 60 KB compressed budget but not what is counted, which compression, or whether KB means 1,000 or 1,024 bytes. Without a precise definition the check is either gamed or argued about. MapLibre has its own budget and is loaded with a dynamic `import()` (TD Front end, Load order), so it must not count against the 60 KB. Caddy can serve brotli (pre-compressed) and gzip; which one a client gets depends on the client and the server config (F-31).

## Decision

1. **What counts:** for each HTML entry of the Vite build, the entry JavaScript chunk plus every chunk it imports **statically** (transitively), plus their CSS. Chunks reached only through dynamic `import()` (MapLibre, later lazy parts) are excluded and listed separately for information. The set is read from Vite's build manifest (`build.manifest: true`, `build/web/.vite/manifest.json`), not guessed from file names. (29 Oct 2026, F-19) Vite 8 drops an HTML entry from the manifest when two HTML files load the same module (`index.html` and `edit.html`, F-19 E-1); the module becomes a plain shared chunk. The check therefore also reads each built top-level HTML file, takes its module script's chunk from the manifest and counts it as that page's entry (`withHtmlEntries` in `src/web/scripts/check-bundle-size.mjs`). A module script missing from the manifest, or no `index.html` entry, fails the check (item 5). After a Vite upgrade, check that `index.html` and `edit.html` still print their lines.
2. **Compression:** each file is compressed with **gzip at level 9** using Node's built-in `zlib`; the budget applies to the sum. Gzip is the floor every browser accepts and does not depend on Caddy config, so passing with gzip guarantees passing with brotli.
3. **Limit:** **60,000 bytes per entry** (decimal KB, the stricter reading). The limit is a single constant in the check script.
4. **Output on every run:** one line per counted file (raw, gzip, brotli sizes) and a total line `entry <name>: <gzip> / 60000 bytes gzip (<brotli> brotli) PASS|FAIL`. Brotli is printed for information only.
5. **Failure:** the script exits non-zero when any entry is over the limit, or when the manifest is missing or has no entry (so a broken build cannot pass silently).
6. **No third-party tool:** a small Node script (`src/web/scripts/check-bundle-size.mjs`) using only `node:fs`, `node:zlib` and `node:path`. Its manifest-walking logic is a pure function covered by a Vitest test with a fixture manifest.
7. **Raising the limit** requires a new ADR that supersedes this one. It is not a config change.

## Consequences

- MapLibre and anything else behind `import()` cannot leak into the critical bundle unnoticed: a static import moves it into the counted set and fails the build.
- Gzip overstates what brotli clients download by roughly 10 to 20 %, so the effective budget is a little tighter than the TD's. Accepted: the margin protects LCP (under 2.5 s) and keeps the rule simple.
- When later features add entries (e.g. the upload page and the activity page as separate entries), each entry gets its own 60,000-byte budget; shared chunks count in every entry that imports them.

## Would choose differently if

- F-31 makes brotli guaranteed for every response and the gzip margin starts blocking real features; then measure brotli at the quality Caddy's pre-compressed files use, in a superseding ADR.
