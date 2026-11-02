# F-07 Prototype page and scrubbing gate: architecture

- **Version:** 1.4
- **Status:** accepted. OQ-10 and OQ-22 to OQ-29 are answered by the user (15 Oct 2026, decision log of `features/README.md`); OQ-23, OQ-25 and OQ-28's laps and splits columns are provisional (`INDEX.md`, Open questions)
- **History:** [architect/history/F-07-prototype-page-and-scrubbing-gate.md](history/F-07-prototype-page-and-scrubbing-gate.md) (revision list, the questions as asked and answered, the requirements hand-off)
- **Date:** 15 Oct 2026
- **Feature:** [`features/F-07-prototype-page-and-scrubbing-gate.md`](../features/F-07-prototype-page-and-scrubbing-gate.md)
- **Depends on:** TD (system-wide picture), [F-02 design](F-02-fit-parsing-and-normalisation.md) v1.6 (`parse`), [F-03 design](F-03-workout-data-file-writer.md) v1.3 (`parse -datafile`, `.gitignore`), [F-04 design](F-04-workout-data-file-reader.md) v1.1 (`loadDataFile`, OQ-10), [F-05 design](F-05-stacked-charts-with-shared-cursor.md) v1.4 (`ChartStack`, `onready`, the 60 fps pass rule), [F-06 design](F-06-route-map-linked-to-cursor.md) v1.6 (wiring, `pixelRatio` row, bundle procedure (b)), [ADR-002](ADR-002-bundle-size-budget-measurement.md), [ADR-004](ADR-004-parser-corpus-and-golden-reports.md) rev. 5, [ADR-006](ADR-006-brotli-pre-compression.md) rev. 2, [ADR-007](ADR-007-decoded-workout-data-in-the-browser.md) rev. 1, [ADR-011](ADR-011-activity-page-bundle-allocation.md) rev. 7, [ADR-012](ADR-012-component-smoke-tests-in-jsdom.md) rev. 3, [ADR-013](ADR-013-maplibre-behind-a-lazy-adapter.md) rev. 4, [ADR-014](ADR-014-map-tiles-and-third-party-privacy.md) rev. 2, [ADR-015](ADR-015-map-interaction-hover-touch-nearest-sample.md) rev. 2, [ADR-016](ADR-016-scrubbing-smoothness-measurement-and-gate.md)
- **Requirements:** Roadmap gate 0 ("Smooth cursor scrubbing on a 12-hour workout on the operator's phone"); NFR Performance (60 fps on a 12-hour workout; data file under 500 KB for a 4-hour ride); TD Budget (app bundle 60 KB); AP-3, AP-6; spec open question "Metric definitions"; QA-06-r4-1
- **Owner:** both. **Backend first** (sections B-1 to B-5, `senior-backend-engineer`), then frontend (sections F-1 to F-8, `senior-frontend-engineer`). The two parts share one contract: the URL layout in D-4.
- **Supersedes:** the F-01 placeholder page (`App.svelte`, `lib/health.ts`), which F-01 AC 4 needed and which is now retired (D-1)

## Decisions at a glance

| # | Decision | Where |
| --- | --- | --- |
| D-1 | **The prototype page becomes the app entry.** `index.html` → `main.ts` → `src/prototype/Prototype.svelte`. The F-01 placeholder (`App.svelte`, `lib/health.ts` and its test) is deleted. From now on the CI size line measures the real activity page set (charts, cursor, data file reader, map glue). F-13 replaces the page layout later; the entry stays | Frontend F-1 |
| D-2 | **Which file the page shows:** `?data=<name>` loads `data/<name>.bin`, a URL relative to the page. The default is `ride-12h`. Names match `^[a-z0-9][a-z0-9-]{0,63}$`. There is no upload, API or database | F-2 |
| D-3 | **Load failure** shows the OQ-10 proposal (F-04): "The workout data could not be loaded.", plus " Reload the page." for `unsupported-version`. The page shows no charts and no map. This is the default until the user answers OQ-10 | F-3 |
| D-4 | **Serving:** a dev-only Go tool, `src/server/tools/protoserve`, serves `build/web/` at `/` and `build/prototype/data/` at `/data/`. At start-up it compresses every text-like file in memory with `internal/precompress` (brotli, quality 9, ADR-006). It sends `br` when the request advertises it. It serves plain HTTP by default, or HTTPS with an in-memory self-signed certificate (`-tls`) when a browser does not advertise `br` over plain HTTP. It is not Caddy and is never deployed; production serving is F-31 | B-3 |
| D-5 | **Data files:** `make prototype-data` runs `parse -datafile` (F-03) on the **synthetic** `ride-12h` and `ride-4h`, writing to `build/prototype/data/` (git-ignored, ADR-004 rev. 5). Real files are added only on request (`REAL=…`) and land in the same ignored folder. **The gate file is synthetic `ride-12h`**: it is committed and reproducible, and it is the worst case (43,200 samples, every stream, 24 laps, repeated loops). Real files are information only (OQ-29) | B-1, B-2 |
| D-6 | **Measurement:** the in-page **scrub record** of ADR-016, a lazy module `src/measure/recorder.ts` loaded only with `?measure`. It splits each cursor flush into a charts part and a map part, so a failing run points at its bottleneck | F-4, ADR-016 |
| D-7 | **Gate procedure and pass rule:** ADR-016. There are three counted runs each of a chart touch scrub (AC 2) and a route touch scrub (AC 3) on the phone, and each must pass. The operator's "smooth" judgement is recorded with every run | Manual checks |
| D-8 | **QA-06-r4-1, pixel ratio follows the device pixel ratio:** all of it lives in the **lazy** `map-view.ts`. A `matchMedia('(resolution: <dpr>dppx)')` listener is re-armed on every change and calls `map.setPixelRatio(mapPixelRatio(devicePixelRatio))` only when the capped value changes. `mapPixelRatio` is a pure function in `map-style.ts`. **Static cost: 0 bytes**; F-06's static share is untouched | F-5; ADR-013 rev. 4 |
| D-9 | (v1.1) **Bundle:** F-07's static share is the page wiring net of the retired placeholder: **E − B0′**, where B0′ is F-06's measurement (b) plus `loadDataFile`. It measured **225**, with a ceiling of **300** for QA rounds. The share comes from the ADR-011 reserve, not from F-06's 2,100. The ledger closes at 60,000 with the reserve at 1,447 (ADR-011 rev. 7). The absolute 45,000 trigger is retired; the review trigger is now the reserve. The recorder is lazy (≤ 4,000 gzip, not counted). The map-view chunk grows by ≤ 300. (v1.0 said 1,500 net of B0 and wrongly assumed B0 contained the data file reader) | Bundle impact; ADR-011 rev. 7 |
| D-10 | **OQ-13 (user, 15 Oct 2026): the cursor stays** when the mouse leaves a chart or the route, or when the finger lifts. F-07 confirms it on the prototype page (PD3, PP4); nothing changes in code | Manual checks |
| D-11 | **Android is out of scope until F-32.** It has not been checked yet. The gate is the operator's phone (iPhone 13 mini, Orion, iOS 27). Every record says "Android not checked" | Manual checks |
| D-12 | **Metric definitions:** OQ-22 to OQ-28 go to the user with a recommendation each. The answers (or an explicit deferral) are handed to the requirements engineer for the spec (AC 8) | Open questions; Hand-off |

## Boundary with neighbouring features

| Not in F-07 | F-07 provides instead | Owner |
| --- | --- | --- |
| Page layout, header, summary strip, laps table, map height, server-rendered HTML, data file preload in the HTML (TD Load order step 2) | A minimal page: map above charts, no header. The entry and `main.ts` that F-13 builds on | F-13 |
| LCP measurement (NFR: under 2.5 s on 4G) | Load timings in the scrub recorder's Load block (information only) | F-13, F-32 |
| Production web server, Caddy config, bundle pre-compression in deploy, cache headers | `protoserve`, which mimics the transport only (brotli, `no-store`) and is never deployed | F-31, F-22 |
| CSP header | Not sent by `protoserve`. `Referrer-Policy: no-referrer` is sent on every response (ADR-014 rev. 2: the worker asset needs it) | F-22 |
| Dark map style, units | Nothing new | F-14 |
| Android check | "Android not checked" in every record | F-32 |
| Whether the scrub recorder stays in the product | It stays behind `?measure` as a lazy chunk until F-13 decides. The recommendation is to keep it for F-25 and F-32 | F-13 |

## Module boundary and layout

```
src/server/
├── scripts/prototype-data.sh        NEW (backend). make prototype-data driver
└── tools/protoserve/                NEW (backend). Dev-only static server; never in the binary
    ├── main.go
    └── main_test.go
src/web/
├── index.html                       CHANGED (frontend). + <meta name="referrer" content="no-referrer">; no inline style (v1.2: the body rule is in Prototype.svelte, bundle CSS)
├── src/main.ts                      CHANGED. Mounts Prototype instead of App
├── src/App.svelte                   DELETED (F-01 placeholder)
├── src/lib/health.ts, health.test.ts  DELETED (only App used them)
├── src/prototype/                   NEW. STATIC, in the counted entry
│   ├── Prototype.svelte             the page: query, load, error text, cursor store, RouteMap + ChartStack, import('../measure/recorder')
│   ├── page-query.ts                pure: parsePageQuery, dataFileUrl
│   ├── load-error.ts                pure: loadErrorText
│   ├── page-query.test.ts, load-error.test.ts      Node
│   └── Prototype.dom.test.ts        jsdom (ADR-012)
├── src/measure/                     NEW. LAZY, reached only through import('../measure/recorder') with ?measure
│   ├── recorder.ts                  DOM: Record button, frame monitor, flush listeners, Load and result blocks
│   ├── stats.ts                     pure: quantile, scrubStats, formatScrub, constants
│   ├── stats.test.ts                Node
│   └── recorder.dom.test.ts         jsdom
├── src/components/map/map-view.ts   CHANGED (lazy). Pixel ratio follows devicePixelRatio (D-8)
├── src/components/map/map-style.ts  CHANGED (lazy). + mapPixelRatio()
└── bench/MEASUREMENTS.md            CHANGED. New section "F-07 prototype and gate"
Makefile, README.md                  CHANGED (backend). prototype-data, prototype-serve
```

**Dependency rules** (in TD Code structure, Dependency rules):

| Module | May import |
| --- | --- |
| `tools/protoserve` | Standard library and `workout-explorer/internal/precompress` only. Never imported by `cmd/` |
| `src/prototype/Prototype.svelte` | `svelte`, `lib/datafile`, `lib/cursor`, `lib/route` (only `routePresence`), `components/charts/ChartStack.svelte`, `components/map/RouteMap.svelte`, `./page-query`, `./load-error`, and `../measure/recorder` **only** through `import()` |
| `src/prototype/*.ts` | `lib/datafile` (types) |
| `src/measure/recorder.ts` | `./stats`, `lib/cursor` and `lib/datafile` (types only) |
| `src/measure/stats.ts` | nothing |

- Nothing imports `src/measure/` statically. QA checks that the build manifest lists `src/measure/recorder.ts` only as a dynamic import of `index.html`.
- `src/prototype/` follows the `components/` rules: no `style=` or `style:` attributes and no `on…` markup handlers. Listeners use `addEventListener`.

---

## Backend (senior-backend-engineer)

Do exactly what this section says. When something here does not match what you find in the repository (a file, a function, a number), **stop and report** instead of choosing. Do not process real FIT files: the operator does that if they want (OQ-29).

### B-1 `src/server/scripts/prototype-data.sh` (new)

Usage: `prototype-data.sh <binary> <corpus dir> <out dir> [<real.fit> ...]`. POSIX `sh`, `set -eu`, `LC_ALL=C`, in the style of `scripts/corpus.sh` (header comment naming this design).

| Step | Behaviour |
| --- | --- |
| 1 | Fewer than 3 arguments: print `usage: $0 <binary> <corpus dir> <out dir> [<real.fit> ...]` to stderr, exit 2 |
| 2 | `mkdir -p "$out"` |
| 3 | For `name` in `ride-12h` then `ride-4h`: `rm -f "$out/$name.bin" "$out/$name.bin.br" "$out/$name.json"`, then run `"$bin" parse -datafile "$out/$name.bin" "$corpus/synthetic/$name.fit" > "$out/$name.json"`. If it exits non-zero, print `FAIL   $name` and exit 1 (parse's own `parse: …` line already went to stderr) |
| 4 | After each success, print `ok     <name>  bin <bytes> br <bytes>`. The sizes come from `wc -c < file \| tr -d ' '` of `<name>.bin` and `<name>.bin.br`, written as plain integers, with five spaces after `ok` and two before `bin`, as in `corpus.sh` |
| 5 | After `ride-4h`, print `budget ride-4h br <bytes> / 500000 PASS` if the `.br` is smaller than 500000, otherwise the same line ending in `FAIL`, then exit 1 |
| 6 | For each real argument `f`, in order: if it is not a regular file, print `FAIL   <f>: not a file` and exit 1. If its name does not end in `.fit` (lower case), print `FAIL   <f>: not a .fit file` and exit 1. Let `base=$(basename "$f" .fit)`. If `base` is empty, starts with `-` or contains anything but `a-z`, `0-9` and `-`, print `FAIL   <f>: name must be lower-case letters, digits and hyphens` and exit 1. Otherwise `name=real-$base` and proceed as in steps 3 and 4 with `"$f"` as the input |
| 7 | Last line: `prototype data: <k> files in <out>`, where *k* is the number of `.bin` files written in this run. Exit 0 |

Real names and sizes appear only in the terminal. The outputs contain coordinates and stay under `build/` (ADR-004 rev. 3).

### B-2 `Makefile` and `README.md`

Add a section after the parser corpus section:

```make
# ---- prototype (F-07) -------------------------------------------------------

PROTO_DATA  := $(BUILD_DIR)/prototype/data
PROTO_ADDR  ?= :18080
PROTO_FLAGS ?=
REAL        ?=

# Data files for the prototype page: synthetic ride-12h and ride-4h, plus REAL=<fit files> on request.
prototype-data: build-server
	sh $(SERVER_DIR)/scripts/prototype-data.sh $(BINARY) $(CORPUS_DIR) $(PROTO_DATA) $(REAL)

# Serve build/web and the prototype data with brotli (dev only). PROTO_FLAGS=-tls for HTTPS.
prototype-serve: build-web
	cd $(SERVER_DIR) && go run ./tools/protoserve -addr '$(PROTO_ADDR)' -web '$(CURDIR)/$(BUILD_DIR)/web' -data '$(CURDIR)/$(PROTO_DATA)' $(PROTO_FLAGS)
```

- Add `prototype-data prototype-serve` to `.PHONY`.
- `PROTO_ADDR`, `PROTO_FLAGS` and `REAL` are Make variables passed as command-line flags. No program reads them from the environment, so TD Conventions (Configuration) still holds (`DEV_ADDR` stays the only dev environment variable).
- `README.md`, "Make targets", add two rows:
  - `make prototype-data`: "Data files for the prototype page in `build/prototype/data/` (synthetic `ride-12h`, `ride-4h`; `REAL=<file.fit …>` adds local real files; never commit the output)".
  - `make prototype-serve`: "Serve the built page and the prototype data with brotli on `$(PROTO_ADDR)`, default `:18080`; `PROTO_FLAGS=-tls` for HTTPS (development only)".
- Also add `tools/protoserve/` and `scripts/prototype-data.sh` to the README's layout tree.

### B-3 `src/server/tools/protoserve` (new, `package main`)

Header comment: what it is, "development only, never deployed; production serving is F-31", and this design's path.

**Command line.** `protoserve -web <dir> -data <dir> [-addr :18080] [-tls]`

| Case | Result |
| --- | --- |
| `-web` or `-data` missing or empty, `-addr` empty, or any positional argument | Usage on stderr, exit 2 |
| `-h` | Usage on stderr, exit 0 |
| Load error, certificate error or listen error | One line `protoserve: <error>` on stderr, exit 1 |
| SIGINT or SIGTERM | `Shutdown` with a 5 s grace, print `protoserve: stopped`, exit 0 |

**Functions** (the tests call them by these names and signatures):

```go
func main()                                                     // os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
func run(args []string, stdout, stderr io.Writer) int           // flags, load, print, listen, serve; returns before listening on any error
type file struct {
	raw         []byte
	br          []byte // nil when the type is not compressed
	contentType string
}
func load(webDir, dataDir string) (map[string]file, error)      // keys are URL paths
func contentType(name string) (ctype string, compress bool)     // by lower-cased extension
func acceptsBrotli(header string) bool                          // Accept-Encoding
func newHandler(files map[string]file, log io.Writer) http.Handler
func selfSigned(now time.Time, ips []net.IP) (tls.Certificate, error)
func localIPs() []net.IP                                        // addresses of up, non-loopback interfaces
```

**`load`.**
- Walk `webDir` with `filepath.WalkDir`. Skip every file or directory whose name starts with `.` (this skips `.vite/`), every file whose lower-cased name ends in `.br`, and every entry that is not a regular file.
- The key is `"/" + filepath.ToSlash(rel)`.
- Walk `dataDir` the same way, with the key `"/data/" + filepath.ToSlash(rel)`.
- Errors:
  - `webDir/index.html` is missing: `index.html not found in <webDir>`.
  - `dataDir` is missing or not a directory: `data directory <dataDir>: …`.
  - The same key comes from both folders: `duplicate path <key>`.
- For each file: read it, set `contentType`, and if `compress` is true, set `br = precompress.Brotli(raw)`.
- The snapshot is taken once. Restart `protoserve` after `make build-web` or `make prototype-data`.

**`contentType`** (extension compared in lower case):

| Extension | Content-Type | Compressed |
| --- | --- | --- |
| `.html` | `text/html; charset=utf-8` | yes |
| `.js`, `.mjs` | `text/javascript; charset=utf-8` | yes |
| `.css` | `text/css; charset=utf-8` | yes |
| `.json` | `application/json` | yes |
| `.svg` | `image/svg+xml` | yes |
| `.txt` | `text/plain; charset=utf-8` | yes |
| `.bin` | `application/octet-stream` | yes |
| `.png` | `image/png` | no |
| `.woff2` | `font/woff2` | no |
| anything else | `application/octet-stream` | no |

**`acceptsBrotli`.**
- Split the header at `,`. For each element, trim spaces and split at the first `;`.
- The coding (trimmed, lower-cased) must equal `br`; `*` does not count.
- If a parameter `q=<v>` is present (name case-insensitive, spaces trimmed), parse `v` as a float. A parse error or `v <= 0` means not accepted; otherwise accepted.
- `br` without a `q` is accepted. No `br` element means false.

**`newHandler`** (used directly as `http.Server.Handler`; no `ServeMux`, so no path cleaning and no redirects):

| Request | Response |
| --- | --- |
| Method other than `GET` or `HEAD` | 405, `Allow: GET, HEAD`, `Content-Type: text/plain; charset=utf-8`, `Cache-Control: no-store`, body `method not allowed\n` |
| `r.URL.Path` == `/` | Treated as `/index.html`. The query string is ignored everywhere |
| Path not exactly a key (including `/assets/`, `//index.html`, `/../x`, `/.vite/manifest.json`, `*.br`) | 404, `Content-Type: text/plain; charset=utf-8`, `Cache-Control: no-store`, body `not found\n` |
| Key found | 200 with headers `Content-Type` (from the table), `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, and for compressed types `Vary: Accept-Encoding`. If the type is compressed and `acceptsBrotli(Accept-Encoding)`, the body is `br` with `Content-Encoding: br`; otherwise the body is `raw` without `Content-Encoding`. `Content-Length` is the body length. `HEAD` sends the same headers and no body. Conditional and `Range` headers are ignored (always 200, full body) |

**Request log.** One line per request to `log`, written under a mutex (requests are concurrent):

```
<METHOD> <path> <status> <encoding> <bytes> ae=<br|no-br>
```

- `<path>` is `r.URL.Path` as received, without the query.
- `<encoding>` is `br` or `identity` for 200, and `-` for 404 and 405.
- `<bytes>` is the number of body bytes written (0 for `HEAD`).
- `ae` is `acceptsBrotli` of the request.
- Example: `GET /data/ride-12h.bin 200 br 266150 ae=br`.
- These lines are the record of transfer sizes (Manual checks).
- Paths may name a local real file, so never paste such lines into the repository.

**Start-up output** (stdout, in this order):

```
protoserve: development only, never deploy (F-07)
<key>  raw <bytes>  br <bytes or ->              one line per file, sorted by key
protoserve: <k> files
protoserve: TLS self-signed certificate, SHA-256 <64 hex digits>     only with -tls
url  <http|https>://<ip>:<port>/?data=ride-12h&measure              one line per IPv4 address from localIPs()
protoserve: listening on <ln.Addr()>
```

**`-tls`.**
- `selfSigned` creates an ECDSA P-256 key and a certificate with these fields:
  - serial: a random 128-bit number;
  - subject CN: `protoserve (development only)`;
  - validity: `now − 1 h` to `now + 7 × 24 h`;
  - `KeyUsageDigitalSignature`, `ExtKeyUsageServerAuth`, `BasicConstraintsValid: true`, `IsCA: false`;
  - DNS name `localhost`; IP addresses `127.0.0.1`, `::1` and `ips`.
- The certificate is self-signed and kept in memory only; no file is written.
- Serve it with `tls.Config{Certificates: …, MinVersion: tls.VersionTLS12}` and `srv.ServeTLS(ln, "", "")`, which enables HTTP/2 like production.
- A new certificate is created at every start, so browsers warn again after a restart. Accept it once per start.

**Server.** `net.Listen("tcp", addr)` first, then `http.Server{Handler: newHandler(...), ReadHeaderTimeout: 10 * time.Second}`.

### B-4 Tests (`tools/protoserve/main_test.go`)

These run in `go test ./...`, so in CI. Temp folders come from `t.TempDir()`. Brotli bodies are checked by **byte equality with `precompress.Brotli(raw)`**, which is deterministic, so the test never imports the brotli library (TD Code structure, Dependency rules).

| Test | Must cover |
| --- | --- |
| `TestAcceptsBrotli` | `""` false; `gzip, deflate, br` true; `br` true; `BR` true; `gzip;q=1.0, br;q=0` false; `br;q=0.000` false; `br;q=0.5` true; `br; q=1` true; `br;Q=0` false; `brotli` false; `*` false; `gzip, deflate, br, zstd` true; `br;q=abc` false |
| `TestContentType` | Every row of the table, plus `.JS` (upper case) as `.js`, and `.xyz` as `application/octet-stream`, not compressed |
| `TestLoad` | Web: `index.html`, `assets/app-abc.js`, `assets/style.css`, `assets/img.png`, `.vite/manifest.json`, `index.html.br`. Data: `ride.bin`, `ride.bin.br`. The keys are exactly `/index.html`, `/assets/app-abc.js`, `/assets/style.css`, `/assets/img.png`, `/data/ride.bin`. `br` equals `precompress.Brotli(raw)` for the compressed ones and is nil for the PNG |
| `TestLoadErrors` | No `index.html`: the error contains `index.html`. Missing data folder: error. Web folder with `data/ride.bin` and data folder with `ride.bin`: the error contains `duplicate path /data/ride.bin` |
| `TestHandler` (`httptest.NewRecorder`, `handler.ServeHTTP`) | (1) `GET /` with `Accept-Encoding: gzip, deflate, br`: 200, `Content-Encoding: br`, body = `Brotli(index raw)`, `Content-Type: text/html; charset=utf-8`, `Content-Length` = body length, `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, `Vary: Accept-Encoding`. (2) `GET /index.html` without `Accept-Encoding`: body = raw, no `Content-Encoding`, `Vary` present. (3) `GET /?data=ride-12h&measure`: same as (1) (query ignored). (4) `GET /data/ride.bin` with `br`: `application/octet-stream`, brotli body. (5) `GET /assets/img.png` with `br`: raw body, no `Content-Encoding`, no `Vary`. (6) `HEAD /` with `br`: the same headers as (1), empty body. (7) 404 with body `not found\n` for `/missing.js`, `/data/missing.bin`, `/assets/`, `/assets`, `/.vite/manifest.json`, `/index.html.br`, `/data/ride.bin.br`, `//index.html`, `/../index.html`. (8) `POST /`: 405, `Allow: GET, HEAD`. (9) On a fresh handler, the log after requests (1), `GET /missing.js` with `br`, `POST /` without `Accept-Encoding` and (6), in this order, is exactly `GET / 200 br <n> ae=br\nGET /missing.js 404 - 10 ae=br\nPOST / 405 - 19 ae=no-br\nHEAD / 200 br 0 ae=br\n` |
| `TestSelfSigned` | Parses with `x509.ParseCertificate`; contains DNS `localhost` and IPs `127.0.0.1` and `::1` plus a given IP; `ExtKeyUsageServerAuth`; `NotAfter − NotBefore` = 7 days + 1 h; not a CA. A TLS round trip: an `httptest.NewUnstartedServer(newHandler(...))` with `TLS` set to this certificate, and a client whose `RootCAs` pool holds only this certificate, gets 200 for `GET /` |
| `TestRunArgs` | `run(nil)` → 2 with usage on stderr; only `-web x` → 2; `-addr ''` → 2; a positional argument → 2; a non-existent `-web` with a valid `-data` → 1 and stderr starting with `protoserve: ` |

### B-5 Verification (run these and paste the outputs into your report)

1. `make ci` exits 0.
2. `make prototype-data`. The expected lines are below; the sizes are those recorded on 11 Oct 2026, quality 9, in `testdata/corpus/MEASUREMENTS.md`:
   ```
   ok     ride-12h  bin 1176640 br 266150
   ok     ride-4h  bin 396156 br 106951
   budget ride-4h br 106951 / 500000 PASS
   prototype data: 2 files in build/prototype/data
   ```
3. `cmp build/prototype/data/ride-12h.bin src/server/testdata/datafile/synthetic/ride-12h.bin && cmp build/prototype/data/ride-4h.bin src/server/testdata/datafile/synthetic/ride-4h.bin` prints nothing and exits 0. If the `cmp`s pass but the `br` sizes differ from step 2, report the new sizes (no stop). If a `cmp` fails, stop and report.
4. `git status --porcelain` shows only your source changes, and nothing under `build/`, no `.bin`, `.br` or report `.json` file.
5. `make prototype-serve` in one terminal. It builds whatever page `src/web` holds at that moment; the placeholder is fine. In a second terminal:

   | Command | Expected |
   | --- | --- |
   | `curl -sS -o /dev/null -w '%{http_code} %{size_download}\n' -H 'Accept-Encoding: br' http://127.0.0.1:18080/data/ride-12h.bin` | `200 266150` (the `br` size from step 2) |
   | `curl -sS -o /dev/null -w '%{http_code} %{size_download}\n' http://127.0.0.1:18080/data/ride-12h.bin` | `200 1176640` |
   | `curl -sS -D - -o /dev/null -H 'Accept-Encoding: br' http://127.0.0.1:18080/` | `HTTP/1.1 200 OK` with `Content-Encoding: br`, `Content-Type: text/html; charset=utf-8`, `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, `Vary: Accept-Encoding` |
   | `curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18080/.vite/manifest.json` | `404` |
   | `curl -sS -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:18080/` | `405` |

   The `protoserve` terminal shows one log line per request in the B-3 format. Stop it with Ctrl-C (`protoserve: stopped`).
6. `make prototype-serve PROTO_FLAGS=-tls`, then `curl -sS -k -o /dev/null -w '%{http_code} %{http_version}\n' -H 'Accept-Encoding: br' https://127.0.0.1:18080/` → `200 2`.
7. `cd src/server && go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./tools/protoserve` lists only `workout-explorer/internal/precompress` and paths starting with `github.com/andybalholm/brotli`. `go list -deps ./cmd/... | grep -c protoserve` prints `0`.

If port 8090 is taken, run with another free port and say so. Do not change the default. (v1.1) On the operator's machine, 8090 and 8091 are taken; the operator procedure uses `PROTO_ADDR=:18090`.

---

## Frontend (senior-frontend-engineer)

### F-1 Entry and files

| File | Change |
| --- | --- |
| `index.html` | Add `<meta name="referrer" content="no-referrer" />` (ADR-014; the page loads the map). The title stays "Workout Explorer". (v1.2, QA-07-r1-5) **No inline `<style>` and no `style=` attribute**: ADR-014's CSP has `style-src 'self'`, which blocks both once F-22 sends it. A hash exception is not used either, because it goes stale with every edit of the rule. The body rule lives in `Prototype.svelte` (F-2) |
| `src/main.ts` | `mount(Prototype, { target })` instead of `App` |
| `src/App.svelte`, `src/lib/health.ts`, `src/lib/health.test.ts` | Delete. Grep first. If anything else imports `lib/health`, stop and report |
| `vite.config.ts` | Unchanged. The `/api` proxy stays for F-09 onwards |

Before any change, measure the baseline B0 (Bundle impact, step 1).

### F-2 `src/prototype/page-query.ts` and the page

```ts
export const DEFAULT_DATA_NAME = 'ride-12h';
export type PageQuery =
  | { readonly ok: true; readonly name: string; readonly measure: boolean }
  | { readonly ok: false; readonly measure: boolean };
/** URLSearchParams(search). `data` absent → DEFAULT_DATA_NAME; present → must match
 *  /^[a-z0-9][a-z0-9-]{0,63}$/ (an empty value fails). measure = params.has('measure'). */
export function parsePageQuery(search: string): PageQuery;
/** 'data/' + name + '.bin': relative to the page, so it also works under a sub-path. */
export function dataFileUrl(name: string): string;
```

**`Prototype.svelte`.** Its state is `let data = $state.raw<WorkoutData | null>(null)`, `cursor` (the same, `$state.raw`), `errorText = $state('')` and `chartsReady = $state(false)`. `$state.raw` is used so that Svelte never proxies `WorkoutData`. In `onMount`:

1. `q = parsePageQuery(location.search)`. If `!q.ok`: `console.warn('prototype: invalid data name')`, `errorText = loadErrorText('bad-name')`, stop. `fetch` is never called.
2. `performance.mark('wx-page:load-start')`.
3. Start both in parallel: `loadDataFile(() => fetch(dataFileUrl(q.name)))` and, only if `q.measure`, `import('../measure/recorder')`. A rejected recorder import is reported with `console.warn` and the page continues without it.
4. `performance.mark('wx-page:data-ready')`. On failure: `console.warn(result.error.message)`, `errorText = loadErrorText(result.error.code)`, stop.
5. `const c = createCursorStore(data.sampleCount, (f) => requestAnimationFrame(() => f()))`.
6. If the recorder module is there: `recorder = mod.createRecorder({ cursor: c, data, name: q.name, hasRoute: routePresence(data) === 'route' })`. This happens **before** step 7, so that its first listener is subscribed before the charts' listeners (ADR-016).
7. `cursor = c; data = result.data`. This mounts the components.

Template (wiring as in F-06 "Wiring"):

```svelte
{#if errorText}
  <p class="load-error">{errorText}</p>
{:else if data && cursor}
  <RouteMap {data} {cursor} load={chartsReady} onmapready={onMapReady} />
  <ChartStack {data} {cursor} onready={onChartsReady} />
{/if}
```

- `onChartsReady`: `performance.mark('wx-page:charts-ready')`, then `chartsReady = true`, then `recorder?.chartsReady()`.
- `onMapReady`: `performance.mark('wx-page:map-ready')`, then `recorder?.mapReady()`.
- CSS in the component's `<style>`: `.load-error { margin: 16px; color: #555; font-size: 14px; }`, and (v1.2, QA-07-r1-5, moved from `index.html`) `:global(body) { margin: 0; font: 15px/1.4 system-ui, sans-serif; color: #111; background: #fff; }`. Svelte extracts both into the entry CSS file, which `style-src 'self'` allows. The rule costs a few dozen bytes inside F-07's ceiling of 300 (225 measured before it); above 300, stop and report. Nothing else; layout is F-13's, and F-13 takes the body rule over.
- The URL is built from the query (`dataFileUrl`), not taken from the HTML as F-04 intends for production. This is acceptable for the prototype; F-13 puts the URL in the page.

### F-3 `src/prototype/load-error.ts` (OQ-10 default)

```ts
export const LOAD_ERROR_TEXT = 'The workout data could not be loaded.';
export const RELOAD_TEXT = 'Reload the page.';
/** 'unsupported-version' → LOAD_ERROR_TEXT + ' ' + RELOAD_TEXT; every other code and 'bad-name' → LOAD_ERROR_TEXT. */
export function loadErrorText(code: DataFileErrorCode | 'bad-name'): string;
```

No technical detail is shown on the page. The error code and message go to the console only (F-04: the message never holds a value).

### F-4 Scrub recorder (`src/measure/`, lazy; ADR-016)

```ts
export interface RecorderOptions {
  readonly cursor: CursorStore;
  readonly data: WorkoutData;
  readonly name: string;
  readonly hasRoute: boolean;
}
export interface Recorder {
  /** Charts are ready: subscribe listener M; if !hasRoute also Z, then enable Record and print the Load block. */
  chartsReady(): void;
  /** Map is ready (marker subscribed): subscribe Z, enable Record, print the Load block. */
  mapReady(): void;
}
export function createRecorder(options: RecorderOptions): Recorder;   // subscribes listener A at once
```

**DOM.** `createRecorder` appends two elements to `document.body`:
- a `<button>` "Record", disabled until ready, styled with `style.cssText` as `position:fixed;right:8px;bottom:8px;z-index:10;font-size:16px;padding:8px 12px`;
- a `<pre class="wx-measure">` (`white-space:pre-wrap;word-break:break-word;margin:8px;font-size:12px`) holding "Measurement: waiting for the charts…".

It uses plain DOM, no Svelte.

**Flush timing.** Listeners run in subscription order: **A** (`createRecorder`, before `ChartStack`), the charts and the readout, **M** (`chartsReady`), the map marker (subscribed during map start-up), and **Z** (`mapReady`; or in `chartsReady` when there is no route).
- During a recording: A stores `tA = performance.now()` and sets `flushed = true`. M stores `tM`. Z writes `total = now − tA`, `charts = tM − tA` and `map = now − tM` into preallocated `Float64Array(MAX_SAMPLES)` buffers.
- No listener allocates (F-05 D-10).

**Recording.**
1. **Arm.** Pressing Record disables the button. Passive capture listeners on `window` for `touchstart`, `touchend` and `touchcancel` set `fingers = e.touches.length`. A `touchstart` also records `startedOn = target.closest('.route-map') ? 'route' : 'chart'` and `input = 'touch'`.
2. **Countdown.** The button text shows `3`, `2`, `1`, one second each, through `setTimeout`.
3. **Window start.** The map-in-view check reads `.route-map`'s `getBoundingClientRect()` (`bottom > 0 && top < innerHeight`), or "no map" when `!hasRoute`. This happens once, before the window. The button text becomes `●`.
4. **Window.** A `requestAnimationFrame` loop for `RECORD_MS` = 10,000 ms from its first timestamp, per frame:
   - store the interval;
   - (v1.2, QA-07-r1-1) if `fingers === 0`, count a no-finger frame. This applies to **every** frame of the window, whether or not a touch has been seen yet: a finger that lands after the window starts leaves no-finger frames before it, and the run does not count (ADR-016 decision 3). Mouse runs count such frames too, and `scrubStats` ignores them there (`touch` false). (v1.0 had `&& input === 'touch'`, which skipped the frames before a late first touch; QA-07-r1-1);
   - if `flushed`, count a changed frame and reset `flushed`.

   **No DOM write and no layout read happens during the window.**
5. **End.** Remove the touch listeners. Compute `scrubStats`, append `formatScrub(...)` to the `<pre>`, and restore the button to "Record". `input` is `'mouse'` when no `touchstart` was seen since arming; `startedOn` is then `–`.

**Load block** (printed once, at `mapReady`, or at `chartsReady` when there is no route):

```
Load · <name> · N <N> · streams <k> · laps <l>
Data file: transfer <transferSize> B · encoded <encodedBodySize> B · decoded <decodedBodySize> B · request to response end <ms> ms   (0 = not reported by the browser)
Body read + decode: <wx-page:data-ready − responseEnd> ms
Since navigation start: data ready <ms> · charts ready <ms> · map ready <ms or –>
dpr <devicePixelRatio> · window <innerWidth> CSS px · <navigator.userAgent>
```

The data file's entry is the Resource Timing entry whose URL path ends in `/data/<name>.bin`.

**`stats.ts`** (pure):

```ts
export const RECORD_MS = 10_000, COUNTDOWN_S = 3, MAX_SAMPLES = 8192;
export const FRAME_OK_MS = 20, PASS_SHARE = 0.99, FLUSH_P95_MS = 2, FLUSH_MAX_MS = 8;
export const CAPPED_MEDIAN_MS = 25, MIN_CHANGED_SHARE = 0.75;
/** Nearest rank on a sorted array: sorted[ceil(q·n) − 1]; NaN when empty. */
export function quantile(sorted: Float64Array, q: number): number;
export interface ScrubInput {
  intervals: Float64Array; frames: number;
  total: Float64Array; charts: Float64Array; map: Float64Array; flushes: number;
  changedFrames: number; noFingerFrames: number;
  touch: boolean; mapInView: boolean | null;   // null: no map
}
export interface ScrubStats { /* every number of the block below, plus */ counts: boolean; reasons: string[]; pass: boolean }
export function scrubStats(input: ScrubInput): ScrubStats;
export function formatScrub(stats: ScrubStats, context: { name: string; n: number; startedOn: string; input: string }): string;
```

Validity and pass (ADR-016):

| Rule | Reason text |
| --- | --- |
| `frames === 0` | `no frames` |
| median interval ≥ 25 ms | `30 Hz cap (median ≥ 25 ms; Low Power Mode on?)` |
| `touch` and `noFingerFrames > 0` | (v1.2) `no finger down in <z> frames` (v1.0: `a finger was lifted`, which misnamed a late finger) |
| `changedFrames / frames < 0.75` | `cursor still in more than 25 % of frames` |
| `mapInView === false` | `map not in view` |

- `counts` is true when no rule applies.
- `pass` = share of intervals ≤ 20 ms ≥ 0.99, **and** total flush p95 ≤ 2, **and** total flush max ≤ 8. All bounds are inclusive.
- With no flushes, the flush values are `NaN`, so `pass` is false. The `cursor still` rule already keeps such a run from counting.

Result block (exact lines; numbers with 2 decimals, shares with 2):

```
Scrub record · <name> · N <N> · input <touch|mouse> · started on <chart|route|–>
Frames <k> · interval ms median / p95 / max <a> / <b> / <c> · ≤ 20 ms <x> %
Flush ms (<f>) total p95 / max <..> / <..> · charts p95 / max <..> / <..> · map p95 / max <..> / <..>
Cursor changed in <y> % of frames · frames without a finger <z or –> · map in view at start <yes|no|no map>
Counts: <yes | NO: reason; reason>
PASS: <yes|NO> (≥ 99 % ≤ 20 ms, flush p95 ≤ 2 ms, max ≤ 8 ms)
dpr <d> · window <w> CSS px · <userAgent>
```

### F-5 Pixel ratio follows the device pixel ratio (D-8, QA-06-r4-1; lazy only)

- `map-style.ts`: `export function mapPixelRatio(dpr: number): number` returns `Math.min(dpr, MAX_MAP_PIXEL_RATIO)` for a finite `dpr > 0`, and `1` otherwise.
- `map-view.ts`:
  - the `pixelRatio` option becomes `mapPixelRatio(window.devicePixelRatio)`;
  - right after `new Map()` succeeds, inside the block covered by the partial-mount cleanup, arm the watcher:

```ts
let applied = mapPixelRatio(window.devicePixelRatio);
let query: MediaQueryList | null = null;
const onResolution = (): void => {
  arm();
  const next = mapPixelRatio(window.devicePixelRatio);
  if (next !== applied) { applied = next; map.setPixelRatio(next); }
};
const arm = (): void => {
  query?.removeEventListener('change', onResolution);
  query = typeof matchMedia === 'function' ? matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`) : null;
  query?.addEventListener('change', onResolution);
};
arm();
```

  - `teardown()` removes the `change` listener and sets `query = null`, after removing the other listeners and before `map.remove()`.
- **Verify in the installed `maplibre-gl` (6.14.x)** that `Map#setPixelRatio(number)` exists and that it resizes the canvas. Name the version you checked in a code comment. If it is missing, stop and report.
- Not on the hot path. Nothing in `RouteMap.svelte` or `touch-quiet.ts` changes.

### F-6 Tests

| File | Must cover |
| --- | --- |
| `prototype/page-query.test.ts` | `''` → ride-12h, measure false. `?data=ride-4h` → ride-4h. `?data=real-wahoo-x&measure` → measure true. `?measure=0` → measure true. `?data=` → not ok. Also not ok: `?data=Ride`, `?data=-a`, `?data=a.b`, `?data=../x`, `?data=a%2Fb`, and a 65-character name; a 64-character name is ok. `dataFileUrl('ride-12h')` = `data/ride-12h.bin` |
| `prototype/load-error.test.ts` | Every `DataFileErrorCode` and `bad-name` → the exact text; only `unsupported-version` adds `Reload the page.` |
| `prototype/Prototype.dom.test.ts` (jsdom; `vi.mock` of `ChartStack.svelte` and `RouteMap.svelte` with test-only stub components that record props and call log, ADR-012 rev. 2; `vi.mock('../measure/recorder')`; `fetch` stubbed with the golden `ride-4h.bin` from `test-support/golden.mjs`) | (1) Default query: `fetch` called once with `data/ride-12h.bin`. (2) `fetch` → 404: exact error text, neither stub mounted. (3) A valid file whose version field (bytes 4–5, F-03 §1) is patched to 2: the text ends in `Reload the page.` (4) `?data=BAD`: error text, `fetch` never called. (5) OK: both stubs receive the same `cursor` with `sampleCount` = N, and RouteMap's `load` is false until the ChartStack stub calls `onready`, then true. (6) `?measure`: `createRecorder` called once, with `hasRoute` true, **before** the ChartStack stub is created (shared call log). Then `chartsReady` after `onready` and `mapReady` after `onmapready`. (7) Without `measure`: the recorder module is never imported. (8) Marks `wx-page:load-start`, `wx-page:data-ready`, `wx-page:charts-ready` exist |
| `measure/stats.test.ts` | `quantile` nearest rank (n = 1, 4, 100; q 0.5, 0.95); empty → `NaN`. Pass boundaries: share exactly 0.99 passes and 0.9899 fails; flush p95 2.00 passes and 2.01 fails; max 8.00 passes and 8.01 fails. Validity: median 25 → does not count; 24.99 → counts; `touch` with 1 no-finger frame → does not count, reason `no finger down in 1 frames` (v1.2); `touch` false ignores no-finger frames; changed 0.75 counts and 0.7499 does not; `mapInView` false → does not count; null → counts. Several reasons joined with `; `. `formatScrub` gives the exact lines for a fixed input |
| `measure/recorder.dom.test.ts` (jsdom, fake timers, stubbed rAF) | `createRecorder` subscribes exactly one listener at once, `chartsReady` one more, `mapReady` one more (with `hasRoute` false, `chartsReady` subscribes two). Record is disabled until ready. A full run (Record, 3 s countdown, 10 s of stubbed frames with flushes, touches as plain `Event`s with `touches`) appends a block starting with `Scrub record`. **A `MutationObserver` on `document.body` sees no mutation between the window's first and last frame.** Every `addEventListener` on `window` is matched by a `removeEventListener` with the same capture flag at the end of the run. (v1.2, QA-07-r1-1) **Late finger:** no touch during the countdown, then `touchstart` on a chart about 2.3 s into the window, then a flush in every frame, all intervals ≤ 20 ms → `frames without a finger` > 0, `Counts: NO` with `no finger down in <z> frames`. **Finger down during the countdown** and held to the end → `frames without a finger 0` and that reason absent. **Mouse run** (no touch events) → `frames without a finger –`, and the reason absent |
| `components/map/map-style.test.ts` (extend) | `mapPixelRatio`: 1 → 1, 1.5 → 1.5, 2 → 2, 3 → 2, 0 → 1, `NaN` → 1 |
| `components/map/map-view.dom.test.ts` (extend; add `setPixelRatio` to the MapLibre fake; stub `window.matchMedia` with a fake `MediaQueryList` that records its query and listeners; set `devicePixelRatio` through `Object.defineProperty`) | dpr 1.5 at mount → option `pixelRatio` 1.5 and one listener on `(resolution: 1.5dppx)`. Change to 2.25 and fire → `setPixelRatio(2)` once, the old listener removed, a new one on `(resolution: 2.25dppx)`. Then 3 → no call (still 2), but re-armed. Then 1 → `setPixelRatio(1)`. `destroy` → no listener left on any fake list, and a later `change` calls nothing. `matchMedia` undefined → mount succeeds. Partial mount (the existing `Marker.addTo` throw case) → no listener left |

### F-7 `bench/MEASUREMENTS.md`

Add the section "F-07 prototype and gate" with the empty tables of "What to record" below. Its header names this design and ADR-016 as the procedure. The operator fills them in.

### F-8 What to report

The bundle numbers of the next section; the recorder chunk's raw, gzip and brotli sizes; the `(resolution: ` grep; the test count.

### Engineer choices, confirmed (v1.1)

Where the design was silent, the engineer chose the following. All are confirmed as the design:

| Choice | Verdict |
| --- | --- |
| The recorder appends the line `dpr · window · UA` as the 7th line, after `formatScrub`'s six | Confirmed. The block reads exactly as in F-4 |
| `NaN` prints as `–`; the Load block uses 2 decimals | Confirmed |
| "Request to response end" = `responseEnd − requestStart`. With no Resource Timing entry, the Load block says so and prints `–` | Confirmed |
| `startedOn` comes from the last `touchstart` since arming, so a touch that starts during the countdown counts | Confirmed. That is the intended use: the finger goes down during the countdown |
| The Load block replaces "waiting for the charts…"; result blocks are appended below it | Confirmed |
| On a workout with a route whose map fails to load, Record stays disabled (and no Load block is printed) | Confirmed, intended. A gate run needs the map marker (AC 2, ADR-016 decision 3), and RouteMap's failure text already says what happened |
| A failed recorder chunk logs `prototype: scrub recorder not loaded` | Confirmed |
| The recorder type comes via `typeof import('../measure/recorder')`, with no static import | Confirmed. A type position emits no import; QA's manifest check (Bundle impact step 6) guards it |
| Test-only stub components live in `src/prototype/testing/` | Confirmed. This matches the `testing/` convention (TD Code structure, Dependency rules: never imported by app code) |
| `mapPixelRatio` maps negative values and ±Infinity to 1 | Confirmed (the rule "finite and > 0, else 1") |

---

## Bundle impact (ADR-002, ADR-011 rev. 7)

| Step | Who | What | Limit |
| --- | --- | --- | --- |
| 1 | Frontend, before any F-07 change | Build F-06's measurement (b): temporary imports of `ChartStack`, `createCursorStore` and `RouteMap` in `main.ts`, exactly as in the F-06 design "Bundle impact". Record **B0** = the `entry index.html` gzip total and **M0** = the gzip size of the `map-view-*.js` line in the dynamic list. (v1.1) Then add a temporary import of `loadDataFile` from `lib/datafile`, used so it is kept, and record **B0′**. (b) alone does not contain the data file reader: none of its `data file: …` strings appear in that build. Revert. Measured on 15 Oct 2026: B0 42,525, B0′ 45,328 (reader +2,803; ADR-011 had 2,793) | — |
| 2 | Frontend, after F-07 | `make size` → **E** (the CI line) | (v1.1) **E − B0′ ≤ 300** (F-07's share: page wiring minus the retired placeholder; measured 225) and **E ≤ 46,000** (60,000 − the 13,000 of the F-13, F-14, F-17 and F-18 shares − the 1,000 reserve floor, ADR-011 rev. 7 decision 5). Above either: stop and report. (v1.0 had E − B0 ≤ 1,500 and E ≤ 45,000, both based on the wrong baseline) |
| 3 | Frontend | Recorder chunk (dynamic list, `recorder-*.js`) | ≤ 4,000 gzip; reported, not counted |
| 4 | Frontend | `map-view-*.js` gzip after F-07 = **M1** | **M1 − M0 ≤ 300** (the D-8 watcher) |
| 5 | Frontend | (v1.1) `grep -c '(resolution: ' build/web/assets/index-*.js` prints `0`; `grep -l '(resolution: ' build/web/assets/map-view-*.js` finds the file. (A plain `dppx` grep cannot work: uPlot in the entry contains `(min-resolution: …dppx)`) | D-8 adds nothing static |
| 6 | QA | `build/web/index.html` has no `modulepreload` of `map-view` or `recorder`; `.vite/manifest.json` lists `src/measure/recorder.ts` only under `dynamicImports` of `index.html` | — |

Measured (v1.1): E = 45,553 / 60,000; E − B0′ = 225. Close-out (QA r2, after the body CSS moved into the bundle): **E = 45,582**, E − B0′ = 254. The CI line moves from `10613 / 60000` to E; that is intended (D-1). The ledger after F-07 is in ADR-011 rev. 7: the planned shares of 13,000 leave a reserve of 1,418.

## Operator procedure

The dev machine has no browser, so the operator runs these. QA reviews the record. This follows the OQ-19 precedent.

1. `make prototype-data`, then (v1.1) `make prototype-serve PROTO_ADDR=:18090`, on the dev machine. 8090 and 8091 are taken there; any free port works, and 18090 is used below. Keep the `protoserve` terminal visible; its log lines are the transfer-size record.
2. Desktop: open `http://localhost:18090/?data=ride-12h&measure` in Firefox 156 on Linux, window ≥ 1,200 CSS px wide.
3. Phone: iPhone 13 mini, Orion, iOS 27, **Low Power Mode off** (the battery icon is not yellow), portrait, home Wi-Fi. Open `http://<your-lan-ip>:18090/?data=ride-12h&measure`.
   - If the phone cannot connect, the firewall probably blocks the port. Either run `sudo firewall-cmd --add-port=18090/tcp` (runtime only, gone after a reboot), or stop Vite and run `make prototype-serve PROTO_ADDR=:5173` (a port the phone already reaches).
4. **Brotli check:**
   - Look at the log line of `GET /data/ride-12h.bin` from each device. `ae=br` means the browser advertised brotli: done.
   - (v1.3) Measured on the first runs, 15 Oct 2026: Firefox 156 and Orion both sent `ae=no-br` over plain HTTP, and `ride-12h.bin` went out uncompressed. **`-tls` is therefore effectively required for realistic transfer sizes**; plain HTTP is fine only for the scrub runs.
   - `ae=no-br` means the browser does not advertise brotli over plain HTTP. Restart with `make prototype-serve PROTO_ADDR=:18090 PROTO_FLAGS=-tls`, open the same address with `https://` and accept the certificate warning once.
   - If Orion offers no way past the warning, record that. Run PP1 and PP6 in Safari over HTTPS, and run the gate (PP2, PP3) over plain HTTP. The transport does not affect scrubbing.
   - Never paste log lines for a real file into the repository.

## Manual checks (operator)

Desktop: Firefox 156 on Linux. Phone: as in procedure step 3. **Android: not checked (out of scope until F-32, D-11).**

| # | Device | Check | Pass / record | Covers |
| --- | --- | --- | --- | --- |
| PD1 | Desktop | Reload `?data=ride-12h&measure` with the Network panel open | The entry JS, CSS and `data/ride-12h.bin` arrive with `Content-Encoding: br`. MapLibre files are requested only after the charts appear. Copy the Load block and the `protoserve` log lines of this load | Scope (brotli), AC 1 |
| PD2 | Desktop | Look at the page; scrub a chart; hover the route | All six charts, the map with start, finish and laps 2–24. The marker follows the chart cursor; hovering the route moves the chart cursor and readout | AC 1 |
| PD3 | Desktop | Hover a chart, then move the mouse sideways off the chart and out of the window. Hover the route, then move off the route and off the map | The cursor line, readout and map marker stay where they were (OQ-13, cursor stays) | D-10 |
| PD4 | Desktop | Record, then hover-scrub the top chart left and right until the result appears. 3 runs. (v1.3) Keep the mouse moving the whole time, including at the turns. A pause leaves the cursor still, and more than 25 % still frames voids the run (first runs: 41 %) | 3 counted runs; record the rows (information; the gate is the phone) | ADR-016 |
| PD5 | Desktop | As PD4, hovering along the route | As PD4 | AC 3 (desktop) |
| PD6 | Desktop | Map in view: Ctrl + to 150 %, then 200 %; Ctrl − to 67 %; Ctrl 0. After each step, wait 1 s, then reload at the same zoom and compare | The route, labels and markers are as sharp before the reload as after it, at every step. If a monitor with another scale factor is at hand, move the window there and back; otherwise record "monitor move not checked" | AC 7 (QA-06-r4-1) |
| PD7 | Desktop | `?data=missing`, then `?data=BAD` | "The workout data could not be loaded." and nothing else; no MapLibre request | D-3 |
| PD8 | Desktop | `?data=ride-12h` without `measure` | No Record button; no `recorder-*.js` request | D-6 |
| PP1 | Phone | Reload `?data=ride-12h&measure` 5 times | Every time: all charts and the map with tiles. Copy the 5 Load blocks. Information: if the median "Body read + decode" exceeds 50 ms, re-run `bench/decode.html` for the ADR-007 trigger | AC 1 |
| PP2 | Phone | **Gate, chart scrub.** Wait until the map shows tiles. Scroll so that the lower part of the map and the top chart are both on screen. Tap Record. Within 3 s, put a finger on the top chart and drag left and right continuously, without lifting, until the result appears (about 13 s). (v1.3) The finger must be down and moving **before the button shows ●**. Every frame of the window without a finger voids the run (first runs: 247 and 51 such frames). Drag back and forth within the chart's width, so the finger never slides off the screen edge. 3 counted runs; repeat a run that does not count | Each counted run: PASS yes. The marker moved continuously and it felt smooth (operator's yes or no per run). Copy every block, including runs that do not count | AC 2 (gate) |
| PP3 | Phone | **Route scrub.** As PP2, but start the touch on the route (within 20 px) and drag along it | As PP2 (`started on route`) | AC 3 |
| PP4 | Phone | Lift the finger after a chart scrub and after a route scrub | The cursor stays (OQ-13) | D-10 |
| PP5 | Phone | Reload; as soon as the charts appear, swipe vertically on a chart again and again until the map has tiles | Scrolling never feels stuck (F-06 MP1 on the production build) | D-4 of F-06 |
| PP6 | Phone | `?data=ride-4h&measure` | Loads with charts and map. The `protoserve` line for `/data/ride-4h.bin` shows `br` and a byte count under 500,000 | AC 4 |
| PP7 | Phone | `?data=ride-12h` without `measure` | Works, with no Record button | D-6 |
| PP8 | Phone, optional | Real files, if OQ-29 provides them. (v1.4) First, put the file into `src/server/testdata/corpus/real/` under the name ADR-004 item 1 prescribes: lower case, `<brand>-<model>-<what>.fit` (for example `garmin-edge530-ride-long.fit`). No date, place or person in the name: the name flows into the ignored golden report and into the `real-<file>` URL path in the `protoserve` log. Then run `make corpus-update` locally, once, so the file has its local golden report; without it `TestCorpus` reports the file as `NEW` and the local `make ci` fails (ADR-004 item 7). Only then run `make ci`. Then `make prototype-data REAL=src/server/testdata/corpus/real/<file>.fit`, then `?data=real-<file>&measure`; PP1 and PP2 | Information only. Record in the QA report, not in `MEASUREMENTS.md`: manufacturer, sport, duration to 0.5 h, samples, `.br` bytes, the result lines. No file names beyond the manufacturer, no dates, no routes (ADR-004 rev. 3, F-03) | OQ-29 |

## What to record (`bench/MEASUREMENTS.md`, section "F-07 prototype and gate")

Synthetic files only.

| Table | Columns |
| --- | --- |
| Transfer sizes (one cold load per device, from the `protoserve` log) | Date, Commit, Device, Scheme (http/https), Resource (`index.html`, entry JS, entry CSS, `data/ride-12h.bin`, `data/ride-4h.bin`, map-view JS, map-view CSS, worker, recorder JS), Encoding, Bytes |
| Bundle | Date, Commit, B0, B0′, E, E − B0′, recorder gzip, M0, M1 |
| Load | Date, Commit, Device, Browser, File, Data transfer B, Request to response end ms, Body read + decode ms, Data ready / Charts ready / Map ready ms since navigation start |
| Scrub records | Date, Commit, Device, OS, Browser, Low Power Mode, File, Input, Started on, Frames, Interval median/p95/max ms, Share ≤ 20 ms, Flush total p95/max, Charts p95/max, Map p95/max, Changed %, No-finger frames, Map in view, Counts, PASS, Smooth (operator) |
| Gate 0 verdict | Date, Commit, PASS or FAIL, the rows it rests on |
| Checks | PD1–PD8, PP1–PP8 pass/fail lines; "Android: not checked" |

## If the gate fails (AC 6)

Stop F-07 and report to the architect with the blocks. The engineers do not improvise remedies; the architect revises this design. Read the failing blocks in this order:

| Symptom | Likely place | First remedy (architect decides) |
| --- | --- | --- |
| Flush charts p95 or max over budget | F-05 hot path (six cursor lines, readout text) | Profile the same scrub on desktop Firefox with 4× CPU throttling. Then tighten F-05 D-10 (fewer readout writes, `contain` on the chart areas) |
| Flush map p95 or max over budget | `Marker.setLngLat` | ADR-013 "Would choose differently if": our own absolutely positioned element from `map.project` |
| Flush within budget, but long frame intervals while touching | Style, layout, paint or composite; input handling (non-passive `touchmove`) | Compare chart scrub and route scrub, and runs with tiles blocked (F-06 bench). Then decide on layer promotion (`will-change`) or the input path |
| None of the above fixes it | Renderer choice | Evaluate the TD fallbacks: custom canvas renderer (ADR-009), Leaflet (ADR-013). Either is a new ADR |

Phase 1 does not start until a later run passes (feature AC 2).

## Acceptance criteria to verification

| AC | Verified by |
| --- | --- |
| 1 12-hour file loads on the phone with all charts and the map | PP1, PD2; `Prototype.dom.test.ts` (5) |
| 2 Chart touch scrub ≥ 10 s, marker continuous, 60 fps, judged smooth (**gate**) | PP2 under ADR-016; recorded rows and the Gate 0 verdict |
| 3 Route hover or drag at the same smoothness | PP3 (phone, touch), PD5 (desktop, hover) |
| 4 4-hour ride `data.bin.br` under 500 KB | `make prototype-data` budget line (synthetic `ride-4h`, expected 106,951); `TestSizeBudget4h` in CI; PP6 transfer line |
| 5 Bundle under 60 KB compressed | `make size` in CI (measured 45,582 / 60,000 at close-out, ADR-011 rev. 7); Bundle impact steps 2–6 |
| 6 Bottleneck identified, fixed or fallback evaluated if the gate fails | "If the gate fails"; the flush split in every record |
| 7 Map sharp after desktop zoom or monitor change, without reload | F-5 tests; PD6 |
| 8 Metric definitions decided (or deferral) and handed to requirements | Open questions OQ-22 to OQ-28; Hand-off |

## Open questions

None for F-07. The answers to OQ-10 and OQ-22 to OQ-29 are in the decision log of `features/README.md`; the provisional ones (OQ-23, OQ-25, OQ-28 columns) are tracked in `INDEX.md`. The questions as asked are in history.
