# F-54 Translated interface: German, Spanish, French

- **Status:** draft; OQ-223 to OQ-228 decided 14 Nov 2026
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first, for server-made pages and messages)
- **Lane:** normal
- **Requirements:** DS-3 (widened 14 Nov 2026), DS-6 (new), DS-1 (browser storage like the units), INF-1, NFR Performance (bundle budget, LCP)
- **Depends on:** all features with visible text; built after F-53, so later features (F-57, F-55, F-56) must ship their texts in all four languages

## Goal
Viewers and sharers use the app in English, German, Spanish or French.

## Scope
- Every text of the interface in four languages: upload page, viewing, edit and published pages, How to use, About page (app part), Report form, error and "expired or not available" pages, upload and server messages (UP-10), labels, units and number and date formats.
- The language comes from the browser's preferred languages, English when none of the four matches; a drop-down next to units and theme switches it; the choice is remembered in this browser like the units (DS-1), never sent to the server. The About page's "In your browser" paragraph (INF-1) lists it.
- Only the chosen language is loaded (lazy), so the entry bundle and LCP budgets hold (OQ-226).
- Not translated: what people wrote (title, description, report message), the operator file and the Impressum, operator-only texts (Pushover notifications, command output, logs).

## Out of scope
- Other languages; right-to-left scripts; translations done by visitors; machine translation at run time.

## Acceptance criteria
After the open questions are decided.

## Decisions (user, 14 Nov 2026)
- OQ-223 to OQ-228 accepted as suggested: static pages English without JavaScript; preview card and suggested title stay English; the engineer drafts the texts, reviewed on screen, one file per language; each language file lazy, about 5 KB gzip at most, outside the entry bundle; number and date formats follow the language, units stay separate; from F-54 on every feature ships its texts in all four languages.
- The language choice is remembered in browser storage like the units (DS-1) and listed in the About page's "In your browser" paragraph (INF-1).
