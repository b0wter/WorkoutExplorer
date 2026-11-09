# QA F-34 iOS Shortcut upload link - round 1

- **Date:** 2026-11-09
- **Verdict:** ready - no Blocker, Major or Minor finding; AC 5 (real iPhone) is a user check
- **Inputs:** `features/F-34-ios-shortcut-upload-link.md`; spec INF-5, INF-1, UP-1, UP-10, UP-11; `architect/F-29-about-page-and-attributions.md` (settings table, marker contract); commit 8501338 (working tree clean)

## Build and test results
- `make ci`: exit 0. Go: all packages `ok` (incl. `cmd/workout-explorer`, `internal/about`, `internal/httpapi`); vitest 61 files, 1,578 tests passed; docker-check ok.
- `build/web/about.html` (built output) still carries `<!--wx:shortcut-->`, two `<!--wx:shortcut-url-->`, `<!--/wx:shortcut-->`: the build does not strip the markers.
- Probe (scratchpad, same predicate as `iosShortcutURL` + `html.EscapeString`): `HTTPS://x.org/`, `https://x.org/a b`, a trailing space, `https://x.org/"onx=1`, `https://x.org#<!--wx:version-->` are accepted and every one is escaped (`"` -> `&#34;`, `<` -> `&lt;`); a newline is refused. No attribute breakout, no marker re-expansion.

## Findings
None.

## Previous findings
None (first round).

## Questions
- **QA-34-r1-q1 (user check, docs):** RUNBOOK 4b step 5 says a non-JSON response (site down) gives an empty alert, so add a fallback text. In Shortcuts, **Get Dictionary Value** on a non-JSON body may instead stop the Shortcut with a system error before the If is reached. Settles it: on the iPhone test, upload once with the site stopped (or a wrong base URL). If the Shortcut stops with an error, change step 5 (small lane, docs only); if the If branch runs, leave it.

## Coverage
| AC | Code | Test |
| --- | --- | --- |
| 1 unset: no section | `about.go` `fill`/`cutBlock` (keep = `Shortcut != ""`) | `TestFillShortcut` (off), `TestFillNoRescan`, `TestAboutRoute` ("SC" absent) |
| 2 https link shown, escaped | `about.go` replacer with `html.EscapeString`, one pass; `about.html` href and text | `TestFillShortcut` (exact output with `"><script>` and a marker in the link), `TestAboutRoute` (`&amp;` in href), `about.test.ts` (block, heading, two URL markers inside it). Themes: no new CSS; the link uses the existing `main a { color: var(--accent) }` and `overflow-wrap: anywhere` (checked by reading, not rendered) |
| 3 http, relative, other text: exit 2 naming the setting | `settings.go` `iosShortcutURL` | `TestIOSShortcutSetting` (12 bad values incl. http, relative, schemeless, `javascript:`, empty host, userinfo, space, NUL, leading space; plus `run serve` exits 2 with the name on stderr) |
| 4 documentation | `deploy/RUNBOOK.md` 4b, `deploy/example.env` | Checked by reading against `internal/httpapi/upload.go` and `respond.go`: field `file` (required, exactly one part, so "no other fields" is right), 201 body `edit_url` = `/edit/<id>#<token>` (`lifecycle.EditLink`), every refusal is JSON with `message`, 429 and 503 busy set `Retry-After` (503 storage full does not; the doc says only "busy", correct), no Origin/CSRF check that would block a Shortcut. Secret warning present and complete (no show, quick look, notification, clipboard, save, message, log). Zip limitation stated (UP-11) |
| 5 real iPhone | - | Not run: user check |

Also checked: the template contract test (`TestAboutTemplateContract`) and the malformed-template case ("no shortcut block") fail start as for Pushover; the Shortcut block is cut before the values are inserted; no API change; the server contacts no one new.

Not run: rendering in a browser (light/dark), anything on iOS.
