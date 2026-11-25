# QA F-73 Remembered map split without a layout jump - round 1

- **Date:** 2026-11-25
- **Verdict:** ready with conditions. No Blocker or Major finding. All six acceptance criteria are browser behaviour, so they stay open until the hand checks below have been done.
- **Inputs:** `features/F-73-remembered-split-without-layout-jump.md`, `architect/F-73-remembered-split-without-layout-jump.md` (RS-1 to RS-7), ADR-032 item 3, ADR-011, F-13 D-10, F-22 H-4; branch `f-73-split-head-script` at 5dbf511, diff against `main` (12 files).

## Build and test results

- `make ci` (lint, test, build, size, caddy-check, docker-check, update-check) ran in the foreground: **exit 0**. Vitest: 95 files and 2,508 tests passed. Every Go package `ok`. `go test -tags caddycheck -run '^TestCaddy'` ok (4.3 s).
- Size: entry index, edit and my-workouts are 61,383 / 62,000 gzip, which matches the ADR-011 row (+6 bytes, under the RS-7 ceiling of 30). Upload 5,624 / 8,000; about and generic 824 / 2,000.
- CSP hash compared against the built output with a Python sha256 over each `<script>` body. All six built shells (`build/web/{index,edit,upload,about,generic,my-workouts}.html`) contain exactly one inline script, and its hash is `sha256-QnlpnggjNgj6/bJESxN8vqyqRgf1z6PDFhppNHpQ6Jg=`. That is the first hash after `'self'` in `deploy/Caddyfile` and in `caddy_test.go`. In every built shell the script comes after the viewport meta. impressum and report-delete have no inline script. The page that html/template renders (published page) and the served `edit.html` are hashed through Caddy by `TestCaddy` (`caddy_test.go:321-332`), which passed.
- Mutants. Each one was applied, run with `vitest run src/page/theme-script.test.ts src/page/splitter.dom.test.ts`, then reverted with `git checkout -- .`. The tree is clean afterwards.

| Mutant | Result |
| --- | --- |
| Head script accepts 80 (`!(v>80)`) | killed (4 tests: the 30-70 validation test, the bundle-parity test, plus the hash and identity tests) |
| Head script accepts 70.01 (`!(v>70.01)`) | killed (same 4) |
| Handover line removed from `restoreSplit` | killed (`hands over to .layout`) |
| Script moved above the viewport meta (upload.html) | killed (`follows the viewport meta in every page`) |
| Previous (F-54) hash `O55B…` dropped from both policy copies | killed (`its sha256 is in the CSP…`) |
| Extra: middle hash `oU85…` dropped from both copies | **survives** (QA-73-r1-1) |

## Findings

### Minor

- **ID:** QA-73-r1-1
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/page/theme-script.test.ts:59-60`
- **Relates to:** RS-6, ADR-032 item 3, AC 5 ("the old hash still listed")
- **Evidence:** The test pins `z5Sj…` and `O55B…` but not `oU85…`. Removing `oU85…` from both the Caddyfile and `caddy_test.go` passes every check, because the Caddy check compares the two copies with each other. RS-6 says "the current F-54 hash and the two older ones stay".
- **Consequence:** If the hash is dropped too early, any published page that still carries that script loses its head script under the CSP. The listed mutant (the F-54 hash) is killed, so F-73's own requirement is met. Fix: also assert `oU85…`, or assert the whole older-hash list.

- **ID:** QA-73-r1-2
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/page/splitter.dom.test.ts:175-188`
- **Relates to:** RS-3 proof row ("then a reset, and a landscape change to a layout with nothing stored, leave no `--map-w` anywhere")
- **Evidence:** The handover test covers the reset but not the landscape change to a layout with nothing stored. The handover-removed mutant is still killed through the reset path.
- **Consequence:** None today. If the reset path changes later, the rotation half of the claim would be untested.

## Previous findings

None (first round).

## Questions

Design open points 1 and 2 (entry growth read as at most 30 bytes, measured +6; published pages keep one shift until rebuilt or expired). The code follows both assumptions, so neither needs QA input. The user confirms or overrides them by number in the architect's report.

## Coverage

- **RS-1:** traced. The script uses `LANDSCAPE_QUERY` and both `PREF_KEYS` verbatim, only one `matchMedia` query is asked, and the key choice is tested. `page.css` reads `--map-w` only under `.has-map .layout` / `.has-map .map-slot` inside the two two-column media queries (lines 733-797), so other pages, portrait and route-less workouts ignore the root value.
- **RS-2:** traced. Same pattern and inclusive 30-70 range. The parity test runs over the design's list plus `45.5` in both layouts. Throwing storage and a missing `matchMedia` set nothing, and theme, hint and language still apply.
- **RS-3:** traced. `splitter.ts:46-47`, test above (QA-73-r1-2). On the edit page `startPage` is called after the owner chunk mounts `.layout`, and the root value covers the gap until then.
- **RS-4:** the script only uses `getItem` and `style.setProperty` (no `style` attribute, no writes, no network beyond the existing hold fetch).
- **RS-5, RS-6, RS-7:** traced. See build results and mutants above.
- **AC 1 to 6:** these need a browser and were not run (no browser here). Hand checks for the user, all through Caddy or protoserve:
  1. Desktop: set the stored wide split to 60 % (drag, or `localStorage['wx.split.wide']='60'`). Hard-reload and cache-reload a published page and the edit page. The map is at 60 % in the first frame, and in a DevTools Performance trace there is no layout shift from the split (CLS 0).
  2. Phone in landscape with a stored landscape split: the same as check 1.
  3. Set `wx.split.wide` to `20`, `80` and `abc`, then block site data (Safari private, or block cookies and site data): each gives the default split and nothing in the console.
  4. Phone portrait, and a workout without a visible route: one column, the stored split has no effect.
  5. Viewing, edit, upload, About and generic pages: no CSP violation in the console.
  6. JavaScript off: default split, page unchanged. LCP within run-to-run noise compared with main.
- **Not run:** a browser of any kind, fuzzing (not needed in this lane).
