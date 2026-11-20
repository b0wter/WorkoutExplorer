# QA F-54 Translated interface: German, Spanish, French - round 1

- **Date:** 2026-11-20
- **Verdict:** not ready - QA-54-r1-1: the upload page no longer shows the English sentences UP-10 quotes for `rate_limited`, `busy` and `too_large`, and the test that should catch it does not check the sentence.
- **Inputs:** `features/F-54-translated-interface.md` (19 ACs), `architect/F-54-translated-interface.md` v0.1 (LG-1 to LG-11), ADR-059, ADR-011 rev. 16 (budget 62,000, user decision 20 Nov), `src/server/testdata/api/error-codes.json`; spec UP-2, UP-3, UP-8, UP-10, UP-11, DS-6, DS-1, INF-1, AP-24. Commits `4cbe231` (backend, on main), `c91b574`, `1baa58d`; tree reviewed at `38025e2` (branch `f-54-frontend`, clean).

## Build and test results
- `make ci` at `38025e2`: **exit 0**. Go `go test -count=1 ./...` all ok. Vitest **86 files, 2,226 tests passed**. svelte-check 0 errors, 0 warnings. Size: activity entry **60,281 / 62,000** gzip (F-54 +2,482 over its 1,000 ceiling, accepted by the user's budget decision in ADR-011 decision 6; not a finding). Upload 5,337 / 8,000, static entry 824 / 2,000. Language files 531 to **4,047** (static-de) / 5,120 gzip. Upload-entry check ("no storage API but the wx.lang key") PASS. `caddy-check` against Caddy v2.11.9 ok, `docker-check` ok, update and sign harnesses 50/50 and 12/12.
- Mutants, run on a copy of the tree in the scratchpad (vitest on the test folders named):

| # | Mutant | Result |
| --- | --- | --- |
| M1 | French key removed (`satellite`) | killed (`fr > has a non-empty text for every English key`) |
| M2 | German F-58 example gains `{r}` | killed (placeholder test) |
| M3 | Literal `<span>Imprint</span>` in `PageFooter.svelte` | killed (LG-8 Svelte scan) |
| M4 | `slot.innerHTML = t('uploadLanguage')` | killed (LG-7 scan) |
| M4b | Same through a variable (`const label = t(…); slot.innerHTML = label`) | **survived** (QA-54-r1-4) |
| M5 | Head script: storage throws, browser languages skipped | killed (`storage that throws skips only the stored step`, plus the hash tests) |
| M6 | File after the hold was lifted applied anyway | killed |
| M7 | No-hold limit 500 → 5,000 ms | killed |
| M8 | API code `stale` without a text | killed |
| M9 | Upload drop-down also writes `wx.units` | killed |
| M10 | Language file fetched with `?lang=` | killed (4 loader tests) |
| M11 | Head script never releases the hold | killed |
| M12 | French count text without `one` | killed |
| G1 | Go: new code constant `gone` not in the list | killed (`TestErrorCodeList`) |
| G2 | Go: `msgMapBusy` wording changed | killed |

## Findings

### Major

- **ID:** QA-54-r1-1
- **Severity / kind:** Major / defect
- **Route:** frontend (architect to note the rule in LG-6 if the fix is a per-page text)
- **Where:** `src/web/src/lib/errors/texts.ts:9`, `:19`, `:23`; `src/web/src/lib/upload/rules.ts:68`; `src/web/src/lib/upload/rules.test.ts:56-63`
- **Relates to:** UP-10 (quoted sentences), feature assumption "Texts the spec quotes exactly stay as quoted in English", AC "Upload errors and messages (UP-2, UP-3, UP-8, UP-10, UP-11) appear in the chosen language on the upload page", LG-6
- **Evidence:** the browser now picks its own text by `error` code. Codes the server sends with more than one sentence got one context-free sentence, so the upload page in **English** shows other words than UP-10 quotes. Probe (`outcomeOf` in vitest on a copy of the tree):
  - 429 `rate_limited`: expected "Too many uploads. Please try again later.", received "Too many requests. Please try again later."
  - 503 `busy`: expected "Too many uploads right now. Please try again in a minute.", received "The server is busy. Please try again in a minute."
  - 413 `too_large`: expected "This file is larger than 20 MB.", received "The file is too large."

  The existing table in `rules.test.ts` still lists the UP-10 sentences as its third column, but the assertion compares against `errors[error]` and never reads that column. The test passes for the wrong reason. Neither P-14 (which covers only the five general codes) nor LG-6 decides this.
- **Consequence:** a sharer who hits the hourly limit or the busy limit gets a vaguer message than the spec requires, in every language. The de/es/fr versions translate the vague sentence too. The iOS Shortcut is not affected (it shows the server's message).

### Minor

- **ID:** QA-54-r1-2
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/about.html:10` (`<title>About - Workout Explorer</title>`)
- **Relates to:** AC "every interface text of the pages … in that language" (About page, app part)
- **Evidence:** the LG-8 scan removes `<title>` before checking, and the static entry translates only `data-t` keys. My workouts sets its title from a table (`src/mine/index.ts:39`), but About does not. Found by reading the code, not run in a browser.
- **Consequence:** in German, Spanish or French the About tab and its history entry still read "About".

- **ID:** QA-54-r1-3
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/upload/page.ts:34-40` (`show()` sets `#notice`, `#progress` and `#message` once); `src/web/src/upload/main.ts:9-10` (a switch re-applies only the `data-t` markup)
- **Relates to:** AC "A choice applies at once"
- **Evidence:** a message already on screen when the language changes is not redrawn. This covers the paused notice (UP-8, shown before any pick), the last error and the progress line. The same holds for the viewing page's load-error line. Found by reading the code, not probed.
- **Consequence:** after a switch, the page mixes two languages until the next state change. The paused notice keeps the old language until the page is reloaded.

- **ID:** QA-54-r1-4
- **Severity / kind:** Minor / risk
- **Route:** frontend
- **Where:** `src/web/src/i18n/i18n.dom.test.ts:162-165` (LG-7 guard), `:131-158` (LG-8 scan)
- **Relates to:** LG-7, LG-8, OQ-228 (F-62 must not ship English-only text)
- **Evidence:** the innerHTML guard matches one source line at a time, so the variable form survives (M4b). The LG-8 scan covers the HTML templates and Svelte markup only. Text that TypeScript builds into the DOM (`textContent = 'Literal'`, `new Option('Literal')`, `setAttribute('aria-label', 'Literal')`) is not scanned. LG-8 names only templates and Svelte, so this is within the design. A grep today finds only `doc.title = 'Workout Explorer'` and the `?measure` recorder line, both acceptable.
- **Consequence:** at present, English-only text added by TypeScript DOM code (the plain-TS upload, static, My workouts, pins and efforts code) would pass CI. F-62 is the next feature that could do this.

- **ID:** QA-54-r1-5
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect
- **Where:** `src/web/src/i18n/groups.ts:30-31` (`edit.html`, `my-workouts.html`: `hold: false`), `src/web/src/main.ts:4`
- **Relates to:** LG-2 ("If the language is not `en` and the page names a file for it, the head script sets a hold")
- **Evidence:** the edit, published and My workouts pages name language files but do not hold. Their first pass runs within 500 ms counted from when the bundle starts. The language file is requested only after `main.js` has loaded, not from the head. Because these shells are empty until the chunk renders, no English is painted first, so the outcome holds. The deviation is recorded only in code comments, not in the design.
- **Consequence:** on a slow link these pages are more likely to stay English than the viewing page. A later reader of LG-2 would expect a hold on them.

## Previous findings
None (round 1).

## Questions
1. QA-54-r1-1: should the upload page keep the UP-10 sentences (upload-specific texts for `rate_limited`, `busy` and `too_large`, as for `storage_full`), or should the spec's quoted wording change? Suggested: keep UP-10. The upload page maps these three codes to its own `lib/upload` texts, and the rules test asserts the sentence column.

## Coverage

| AC | Code | Test / check | State |
| --- | --- | --- | --- |
| First visit `de-AT,en` → de, `it,fr` → fr, `it` → en | head script in every entry | `page/theme-script.test.ts` "first browser language…" | traced, passes |
| Drop-down on viewing, edit and upload pages, applies at once, remembered, wins | `page/start.ts:175-178`, `upload/lang.ts` | loader and switch tests (`lib/i18n/i18n.dom.test.ts`), upload M9 | traced. Cursor and range kept across a switch: hand check. Messages already shown are not redrawn (r1-3) |
| No cookie, link, header or parameter | `lib/i18n/load.ts` (no query), prefs | M9, M10 killed. Grep: no `document.cookie` or `lang=`. Request not logged: LG-9 Go test, `caddy-check` ok | traced |
| Storage blocked | head script `try` | M5 killed | traced |
| Every text in each language | tables, `data-t` | LG-8 tests, M1, M3 killed | traced with gaps r1-2, r1-4. User's on-screen review (OQ-225) pending |
| Per-feature list (F-58, F-55, F-57, F-56, F-53) | tables | keys present in de/es/fr (completeness test). I spot-read hiding sliders, pins reasons, My workouts and summary dialog in all three languages: correct register (du / tú / vous), placeholders kept | traced |
| F-58 example adds no value (AP-24) | `summary-info/index.ts:93-94` (owner-only radius line, unchanged gate) | placeholder test, M2 killed | traced. A translation that adds a word like "hidden" without a placeholder cannot be caught by a test: covered by the user's review |
| Singular and plural | `lib/i18n/index.ts` `format` (`Intl.PluralRules`) | CLDR test, M12 killed | traced |
| Upload messages in chosen language | `lib/errors/texts.ts`, `lib/upload/texts.ts` | `rules.test.ts` | **fails UP-10 in English (r1-1)** |
| Completeness test | `i18n/i18n.dom.test.ts` | M1, M2, M3, M8, M12 killed | traced |
| Number and date formats; units unchanged | `lib/format` | `lib/format/lang.test.ts` | traced |
| Phone portrait, longest language | — | — | hand check |
| No JavaScript: English | served markup English | by construction | not run in a browser |
| No visible switch; 500 ms; LCP | head script, `startLang`, `plain.css` / `page.css` hold rule (present in every built page's CSS, checked) | M6, M7, M11 killed | traced. Throttled German first visit and LCP < 2.5 s on the reference phone: hand check |
| Only the chosen file, ≤ 5 KB, entry budget | `scripts/lang-files.mjs`, build metas | size check PASS (max 4,047). Entry 60,281 / 62,000 | traced |
| Failed file → English | `fetchGroup` catch | "a failed file leaves the page English" | traced |
| Old page after update (INF-9) | hashed `assets/lang-*.json`, ADR-053 additive copy | not tested by me | design only |
| Not translated | owner text, `Card`, `<meta>` untouched | read | traced by reading |
| About "In your browser" lists the language | `about.html` | `about/about.test.ts` | traced |

Checked and found correct: only `wx.lang` is written by the upload page. No table text reaches `innerHTML` in the current code (every sink read: SVG strings carry numbers and constant duration names only). `applyMarkup` places only kept elements and text nodes. The API code contract is complete: the Go test parses constants, refusals, fixed bodies and ingest kinds, and no `writeError` uses a literal code. The F-58 privacy gate does not depend on language.

Hand checks (user or main session): network view per page (one language's files, no query, no cookie); throttled first visit in German with the file held back (English at 500 ms, no later switch); LCP in German on the reference phone; phone portrait in the longest language; each language reviewed on screen (OQ-225); cursor and range kept across a switch.
