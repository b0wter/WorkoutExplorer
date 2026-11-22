# QA F-63 iOS Shortcut hint on the upload page - round 1

- **Date:** 2026-11-22
- **Verdict:** ready with conditions - no Blocker or Major; conditions: the user's iPhone hand check (last AC) and a browser network-log look with the setting on.
- **Inputs:** `features/F-63-ios-shortcut-hint-on-upload-page.md` (OQ-291 to OQ-295), `architect/F-38-impressum-page.md` and `architect/F-11-upload-page.md` (F-63 notes), ADR-043, Technical Design configuration row; branch `worktree-agent-a43782a83b8a5f66a`, commit 3ff0b10 against `main` d81b145.

## Build and test results
- `make ci` in the worktree: exit 0. Go: all packages `ok` (`internal/shell` 0.022 s). Vitest: 90 files, 2,298 tests passed. Size: `entry upload.html: 5495 / 8000 bytes gzip (4816 brotli) PASS`; upload language files de/es/fr 1,021-1,045 / 5,120 PASS; upload entry storage check PASS.
- Built `build/web/upload.html` keeps `<!--wx:shortcut-->`, `<!--wx:shortcut-url-->`, `<!--/wx:shortcut-->` (Vite does not strip them).
- Probe: `git show main:src/web/upload.html` equals the new source with the block cut out (Python, `True`), so unset gives today's bytes.
- Mutant (copy of `src` in the scratchpad): `shell.Write(..., impressum, "")` in `wiring.go` instead of `s.iosShortcutURL` - `go test ./cmd/workout-explorer/` still `ok`.

## Findings

### Minor
- **ID:** QA-63-r1-1
- **Severity / kind:** Minor / risk
- **Route:** backend
- **Where:** `src/server/cmd/workout-explorer/wiring.go:92`; `settings_test.go:190`
- **Relates to:** AC 2 (setting an https link shows the hint)
- **Evidence:** the mutant above survives. `TestShortcutHint` covers the fill on the real source; nothing checks that `serve` passes the setting through. The existing About test already runs `newApp` with `iosShortcutURL` set over the real `upload.html`, so it proves a broken block fails the start but not that the hint appears.
- **Consequence:** a later refactor that drops the argument silently removes the hint. A full serve-through-Caddy test is not needed; one assertion that `{data}/site/upload.html` contains the escaped link after that existing `newApp` call is enough.

## Previous findings
None (round 1).

## Questions
None.

## Coverage
- AC 1 unset byte-identical: `TestShortcutHint` (cut of current source) plus the probe against `main`'s bytes; Caddy rig `/` equals `site/upload.html`.
- AC 2 escaping: `TestShortcutHint` with `"><script>` and `&` in attribute and text; `html.EscapeString` covers `<>&'"`.
- AC 3 exit 2: unchanged F-34 validation, `settings_test.go:120` still runs.
- Reliability (exit 1 on broken marker): missing, reversed and doubled blocks fail `shell.Write` naming `upload.html`; `newApp` returns the error before listening.
- AC 4 placement and states: hint follows `#message`, before "My workouts" (DOM test); CSS hides it in `opening`, `uploading`, `processing`, `paused` (test reads the CSS); shown in `checking` (the no-JS initial state) and `ready` (after an error). Checked against every `show()` state in `page.ts`.
- AC 5 links: direct Shortcut link, same tab, "More" to `/about#shortcut`; About `h2` has `id="shortcut"` (tests).
- AC 6 languages: de/es/fr keys added; `i18n.dom.test.ts` enforces key completeness and matching `{link}`/`{more}` placeholders; `applyMarkup` handles the nested `data-p` links as for `mineLink`.
- AC 7 network/referrer: by reading only - plain `<a>`, no new script, fetch or asset; page has `<meta name="referrer" content="no-referrer">` and Caddy sends `Referrer-Policy no-referrer`. Network log not run in a browser.
- AC 8 storage/logging: no storage, cookie or log code touched (upload storage check PASS).
- AC 9 budget and CSP: 5,495 / 8,000; no inline script, style or `style=` added (U-3 test).
- AC 10 docs: RUNBOOK row and section, `example.env`, Technical Design row updated.
- AC 11 About sentence: English in `about.html` (test), de/es/fr in the language files.
- AC 12 iPhone look: not checkable here; user hand check. Grey link (`--muted`, underlined) has about 5:1 contrast in both plain themes.
- Engineer deviations accepted: notes in F-38/F-11 instead of an F-34 doc (one home per fact holds); `CutBlock` moved to `shell` (shared, `about` imports `shell` already); `id="shortcut"`; grey link colour. No end-to-end serve test: not needed beyond QA-63-r1-1.
