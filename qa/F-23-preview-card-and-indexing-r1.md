# QA F-23 Preview card, Open Graph and indexing - round 1

- **Date:** 2026-10-31
- **Verdict:** ready with conditions. No Blocker or Major. The image cannot show a hidden route part: it is drawn only from the decoded public data file, and hidden positions are not in that file (an equivalent mutant confirms this). The conditions are the manual checks no automated test covers: the operator's OQ-91 look and N review, the AC 3 visual comparison, and AC 8 and AC 9 on a deployed instance.
- **Inputs:** `features/F-23-preview-card-and-indexing.md`; `architect/F-23-preview-card-and-indexing.md` v0.1 (PC-1 to PC-12); ADR-033, ADR-026 (31 Oct note), ADR-028 item 5; F-16 H-6, H-7; F-22 H-6, H-8, H-11; product spec SH-4, SH-5, SH-10, SH-11; TD Configuration row `WORKOUT_EXPLORER_BASE_URL`; decision log 31 Oct (OQ-91, OQ-92). Commits 490ad2a, a1f78fe, bfbfbfe (working tree clean at bfbfbfe). Critical lane, effort high.

## Build and test results

| Command | Result |
| --- | --- |
| `make ci` | exit 0. `go vet -tags caddycheck` clean. svelte-check: 0 errors, 0 warnings. Go: all packages ok. vitest: 51 files, 1,331 tests passed. Size: edit and index 56,366 / 60,000 B gzip, upload 2,272 / 8,000 B (unchanged, as expected). `caddy-check` with pinned 2.11.7: ok, 2.29 s. (Local node 22 vs `>=24`: an EBADENGINE warning that was already there.) |
| `go test -race -count=1 ./...` | all 15 packages ok, no race reported (preview 267 s, publish 437 s under `-race`) |
| `go test -count=1 -v -run TestCorpus ./internal/preview` | pass. Largest PNG is ride-12h at 77,417 B (hiding off) and 77,412 B (on); render takes 79 to 93 ms |
| `make corpus` | 55 ok, 0 diff, 0 fail, 0 new, 0 stale |
| `go test -bench . -benchtime 5x ./internal/publish` | publish ride-12h 267 ms, title save ride-12h 145 ms. That is far below ADR-028's 1 s trigger, although a title save now also renders the PNG |
| Binary size (`go build ./cmd/workout-explorer`, a1f78fe vs bfbfbfe) | 22,399,033 B → 23,588,300 B (+1.19 MB) |
| Worst-case PNG probe (scratch copy): 43,200 random points, a 12 h random walk, 21,600 isolated single points, a 100 × "W" title | 36 to 43 KB with default compression. `BestCompression` saves 2 to 5 % |
| Mutants (scratch copies; `preview`, `page`, `publish`, `cmd` packages; Caddy check for `edit.html`) | see the table below: 7 of 7 design mutants killed, plus 3 extra mutants (2 killed, 1 equivalent). One extra survivor (M11) is finding r1-2 |

### Mutants

| # | Mutant | Killed by |
| --- | --- | --- |
| M1 | Preview rendered from the parsed workout before the transform | `TestHidingCorpus`, `TestHidingFullyHidden`, `TestHidingOnAfterPublish` (the stored PNG must equal `Render` of the public data file) |
| M2 | Route drawn from the public file, but the box fitted to the full route | same three tests |
| M3 | `Max heart rate` accepted as average | `TestCard` |
| M4 | Distance `0` shown | `TestCard` |
| M5 | `og:image` not linked by path (`/i?w=…&p=preview…`) | `TestCard` and 15 publish and cmd tests (sweep, kill, end to end) |
| M6 | Base URL with trailing slash accepted | `TestBaseURL`. `TestServeWithoutBaseURL` hangs instead of failing (r1-3) |
| M7 | `og:image` added to the built `edit.html` (Caddy check against a copy of `build/web`) | `TestCaddy/routes` |
| M8 (extra) | `write` keeps old `.png` files | `TestHidingOnAfterPublish`, `TestTitleOnly`, `TestPublishEndToEnd` |
| M9 (extra) | Sweep treats every `preview.*` as referenced | `TestKill`, `TestSweepAfterStoppedRemoveFirst`, `TestReferenced` |
| M10 (extra) | `Points` reads the raw `Ints` of invalid positions | Survived, but **equivalent**: the decoder sets `Ints` to 0 for missing samples, so hidden coordinates are not in the file (I-3 by construction) |
| M11 (extra) | Description drawn into the image next to the name | **Survived** (r1-2) |

## Findings

No Blocker, no Major.

### Minor

- **ID:** QA-23-r1-1
- **Severity / kind:** Minor / risk (documentation out of step with the code)
- **Route:** architect
- **Where:** `architect/ADR-033-preview-image-rendered-with-x-image.md:16` and `:23`; `architect/F-23-preview-card-and-indexing.md:9` and `:51`; `src/server/internal/preview/preview.go:107`
- **Relates to:** ADR-033 items 1 and Consequences; F-29 hand-off
- **Evidence:** ADR-033 item 1 says "best compression", but the code uses `png.DefaultCompression`. The probe shows the deviation is harmless: the worst case is 77 KB against the 300 KB limit, and best compression saves 2 to 5 %. ADR-033 and the design say the binary grows by "about 300 KB"; it grew by 1.19 MB. `go mod why` shows `golang.org/x/text/encoding/charmap` linked through `x/image/font/sfnt`, and `x/sys/cpu` through `x/image/vector` (bumped 0.48 → 0.49). Neither appears in ADR-033's Consequences, and the F-29 hand-off names only `golang.org/x/image`. ADR-033 is still `proposed` and the design `draft`. The ADR-026 one-line note (PC-6) is already present (`ADR-026…md:29`), so that open item is done.
- **Consequence:** A later reader would "fix" the compression level or the size claim against the ADR. F-29's attribution list could also miss `golang.org/x/text` (BSD-3, same licence family, so this is not a licence problem). Fix in the small lane: one-line notes in ADR-033 (default compression, measured +1.2 MB, x/text and x/sys transitive) and add x/text to the F-29 hand-off.

- **ID:** QA-23-r1-2
- **Severity / kind:** Minor / test gap
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/publish_test.go:166-168`
- **Relates to:** PC-2 ("determinism keeps a lifetime or description save from writing a new image"), Consequences of the design
- **Evidence:** Mutant M11 draws `vm.Header.Description` into the image. It survived every test, because only a lifetime change is checked to keep the preview name.
- **Consequence:** If the description slipped into the image, every description save would write a new PNG. That is not a privacy leak, because the description is public on the page, but a stated design property would be unguarded. Fix: add a description PATCH next to the lifetime check and assert the same preview name.

- **ID:** QA-23-r1-3
- **Severity / kind:** Minor / risk (test robustness)
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/settings_test.go:56-70` (same idiom at `:97`, `:192`, `:277`)
- **Relates to:** PC-8
- **Evidence:** With mutant M6 applied, `TestServeWithoutBaseURL` does not fail. `serve` starts on `127.0.0.1:0` and runs until the test binary's timeout: confirmed with `-timeout 20s` (`panic: test timed out … TestServeWithoutBaseURL`). In the full mutant run the `cmd` package took 600 s.
- **Consequence:** A regression in base URL validation costs a 10-minute CI hang instead of a clear failure (`TestBaseURL` still names the cause). Fix: run `serve` in these tests with a short deadline or a cancellable context, and fail on timeout.

## Previous findings

None (first round).

## Questions

1. **AC 3 visual check (operator).** The synthetic routes are open arcs. Because PC-1 fits the hidden-ends route to its own box, the hiding-on card looks almost the same as the hiding-off card: I rendered `run-30min` and `ride-30min` both ways, and the arc is only slightly shorter and re-centred. The privacy property holds: no start or end marker, no scale, and the box never reaches into the hidden ends. The visual part of AC 3 still needs the operator's comparison on a loop-shaped route, where the open ends are obvious. A local real file is fine for that, since it is not committed. **Settles it:** the operator ticks AC 3 after looking at one loop route with hiding on and off.
2. **OQ-91 look and N.** The three review cards exist (`WX_CARDS=<dir> go test -run TestSampleCards ./internal/preview`). The engineer chose the name at 48 px bold, cut to 40 code points and then to the width. The decision log says the user sets N and the look after reviewing them. **Settles it:** a dated decision-log line that accepts the look and N = 40, or the changed values.

## Engineer's deviations (judged)

| Deviation | Judgement |
| --- | --- |
| PNG default compression instead of best (ADR-033 item 1) | Accept. Worst case 77 KB; document it (r1-1) |
| Name 48 px bold, cut to 40 code points, then cut until it fits the width | Accept. Tested in `TestName` (wide W, trailing space trimmed before "…"); look pending OQ-91 (Question 2) |
| OQ-92 handling (drop glyphs missing in Go Bold, collapse white space, default title if nothing remains) | Accept. Matches the decision log. `og:title` keeps the full title, HTML-escaped (`TestPublishEndToEnd`) |
| Route broken at missing positions, 200 m minimum span | Accept. Breaking at gaps never draws across a missing stretch, which is safer than the page's dashed bridge for later privacy zones. Degenerate routes render inside the box (`TestShapes`) |
| `page.Build` twice per build | Accept (preference). Costs a few ms; the title save stays at 145 ms for ride-12h. The first build's `Card.Image` is never used |
| Stricter base URL validation (host regex, port 1 to 65535, no leading zero) | Accept. Within PC-8's `http(s)://host[:port]`. The error message names the variable, not the value |
| Binary +1.2 MB instead of ~300 KB | Accept; document it (r1-1) |
| `x/text` indirect, `x/sys` bump | Accept. Both are pulled in by `x/image` (`go mod why`), BSD-3; add them to the F-29 hand-off (r1-1) |

## Coverage

| AC / requirement | Code | Test | Status |
| --- | --- | --- | --- |
| AC 1 (1200 × 630, < 300 KB, route, name, moving time, distance, avg HR) | `preview.Render`, `page.CardItems` | `TestCorpus` (every accepted synthetic file, hiding off and on, deterministic), `TestCard`, `complete()` in publish tests, Caddy `preview` subtest (`image/png`, no `Content-Encoding`, bytes equal to disk) | traced; the look awaits OQ-91 |
| AC 2 (indoor without HR: name, elapsed time, no distance or HR, no zero) | `CardItems` (elapsed first without a route; distance only if not all zeros; HR only "Avg") | `TestCard` "indoor without heart rate", "distance 0", "max heart rate only"; `TestNoRoute` (0 route pixels); card viewed | traced |
| AC 3 (hidden ends absent) | PC-1: `Render` receives only the decoded public `datafile.File` | `read()` in `hiding_test.go`: the stored PNG equals `Render` of the public file, for the whole corpus; `TestHiddenEnds` (every point is a public position, the box is smaller); mutants M1, M2 killed, M10 equivalent | traced; visual part open (Question 1) |
| AC 4 (every OG/Twitter tag, absolute image URL) | `index.html:7-12`, `Card` | `TestPublishEndToEnd` (all six tags before the preload, exact values, escaped title), Caddy `preview` subtest (URL under the rig's base URL) | traced |
| AC 5 (`/robots.txt` 404) | no file; Caddy catch-all | `TestCaddy/routes` | traced |
| AC 6 (draft and expired: no image, no numbers) | no folder for a draft; expiry removes the folder | `TestCaddy/routes` (draft → generic page, no `og:` or `preview` in it); `TestCaddy/SH-10` (expired → old page and old preview gone) | traced |
| AC 7 (SH-11 fixed card) | `edit.html:7-9` (static) | `TestCaddy/routes`: `/edit` and `/published` for two workouts and a draft return the same bytes as `edit.html`; exact tags; no `og:image`, `og:url`, `{{` or 22-character ID; mutant M7 killed | traced |
| AC 8, AC 9 (messengers) | — | manual, operator, deployed instance | not run (needs deployment) |
| SH-10 (preview gone within 60 s on hiding on, delete, expiry) | remove-first build, `removeFolder`, expiry | `TestCaddy/SH-10` `gone` checks for the preview in all three cases | traced |
| Sweep keeps the linked preview, removes an unlinked one (ADR-028 item 5, PC-7) | `linked` (the next character `"` is not a name character) | `TestSweepAfterStoppedRemoveFirst` (`preview.0000…png` removed, linked one kept), `TestKill`; mutants M5, M9 killed | traced |
| PC-8 base URL required, exit 2 before the data directory | `settings.baseURL`, `newApp` guard | `TestBaseURL` (5 valid, 21 invalid), `TestServeWithoutBaseURL`; mutant M6 killed (r1-3) | traced |
| PC-12 no external fetch, no CSP change | fonts embedded (`gofont`), no network in `preview` | read | verified by reading |

Not run: messenger checks (AC 8, 9), the operator's visual checks (Question 1, 2), and a browser check (F-23 adds no script or CSS, so there is nothing for it to test). Probes and mutants ran only on scratch copies; `src/` is unchanged.
