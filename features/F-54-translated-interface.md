# F-54 Translated interface: German, Spanish, French

- **Status:** draft
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first, for server-made pages and messages)
- **Lane:** normal
- **Requirements:** DS-3 (widened 14 Nov 2026), DS-6 (new), DS-1 (browser storage like the units), INF-1, NFR Performance (bundle budget, LCP)
- **Depends on:** all features with visible text; built after F-53, so later features (F-55 to F-57) must ship their texts in all four languages

## Goal
Viewers and sharers use the app in English, German, Spanish or French.

## Scope
- Every text of the interface in four languages: upload page, viewing, edit and published pages, How to use, About page (app part), Report form, error and "expired or not available" pages, upload and server messages (UP-10), labels, units and number and date formats.
- The language comes from the browser's preferred languages, English when none of the four matches; a drop-down next to units and theme switches it; the choice is remembered in this browser like the units (DS-1), never sent to the server.
- Only the chosen language is loaded (lazy), so the entry bundle and LCP budgets hold (OQ-226).
- Not translated: what people wrote (title, description, report message), the operator file and the Impressum, operator-only texts (Pushover notifications, command output, logs).

## Out of scope
- Other languages; right-to-left scripts; translations done by visitors; machine translation at run time.

## Acceptance criteria
After the open questions are decided.

## Open questions
- [ ] **OQ-223 Pages without JavaScript.** Published pages are static files built once in English. Suggested: the static markup stays English and the script switches it to the viewer's language; without JavaScript the page is English.
- [ ] **OQ-224 Preview card and suggested title.** Both are made once on the server for every viewer (SH-5, UP-5). Suggested: they stay English; a page shows the suggested title in the viewer's language only where the browser builds it (placeholder on the edit page).
- [ ] **OQ-225 Who translates.** Suggested: the engineer drafts the German, Spanish and French texts; the user (or friends who speak them) reviews them on screen, as with OQ-58 and OQ-63; texts live in one file per language.
- [ ] **OQ-226 Budget.** Suggested: language files load lazily, at most about 5 KB gzip each, and are not counted in the 60,000-byte entry bundle; the language drop-down itself fits the entry reserve.
- [ ] **OQ-227 Number, date and unit formats.** Suggested: follow the chosen language (decimal comma in German, Spanish and French); units stay a separate choice (DS-1).
- [ ] **OQ-228 Translations in later features.** Suggested: from F-54 on, every feature's acceptance criteria include its texts in all four languages.
