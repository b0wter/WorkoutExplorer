# F-54 Translated interface: German, Spanish, French

- **Status:** decided (OQ-223 to OQ-228 on 14 Nov 2026, OQ-280, OQ-281 on 19 Nov 2026, language-file trace on 20 Nov 2026); acceptance criteria written (19 and 20 Nov 2026)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first, for server-made pages and messages)
- **Lane:** normal
- **Requirements:** DS-3 (widened 14 Nov 2026), DS-6 (new), DS-1 (browser storage like the units), INF-1, INF-9 (published pages keep their files after an update), NFR Performance (bundle budget, LCP)
- **Depends on:** all features with visible text; built last (build order of 19 Nov 2026), so it also translates the texts of F-58, F-55, F-57, F-56 and F-53, which ship in English. Features built after it (F-61, F-62) ship their own texts in all four languages (OQ-228)

## Goal
Viewers and sharers use the app in English, German, Spanish or French.

## Scope
- Every text of the interface in four languages: upload page, viewing, edit and published pages, How to use and How these numbers are computed dialogs (AP-22, AP-24), My workouts (AC-7), About page (app part), Report form, error and "expired or not available" pages, upload and server messages (UP-2, UP-3, UP-10, UP-11), labels, units and number and date formats.
- The language comes from the browser's preferred languages, English when none of the four matches; a drop-down next to units and theme switches it on the viewing, edit and upload pages (OQ-280); the choice is remembered in this browser like the units (DS-1); no cookie, link or request parameter carries it. The About page's "In your browser" paragraph (INF-1) lists it.
- Only the chosen language is loaded (lazy), so the entry bundle and LCP budgets hold (OQ-226).
- Not translated: what people wrote (title, description, pin titles and notes, report message), the operator file and the Impressum, operator-only texts (Pushover notifications, command output, logs), the preview cards (SH-5, SH-11) and the suggested title (OQ-224), pages without JavaScript (OQ-223).

## Out of scope
- Other languages; right-to-left scripts; translations done by visitors; machine translation at run time; language chosen by the server (no `Accept-Language` handling, no language in links).

## Assumptions
- A preferred language matches by its primary subtag (`de-AT` is German); the first match in the browser's order wins.
- The drop-down names each language in itself (English, Deutsch, Español, Français).
- The units default still comes from the browser language, not from the chosen language (OQ-227).
- Texts the spec quotes exactly stay as quoted in English; the other languages say the same.
- Report reasons are translated on the form; the stored reason and the operator's notification stay English. The iOS Shortcut shows the server's English messages.
- Switching language redraws the page without reloading data; cursor, selected range and settings are kept.
- The GPX download (F-57) is not translated: file name `route-YYYY-MM-DD.gpx` and file contents stay as they are.
- Pins store their type, not its name, so a pin with an empty title shows its type's name in each viewer's language (F-56, OQ-277).
- Fetching only the chosen language's file tells the server which language was chosen (accepted 20 Nov 2026); like every request, it is not logged (TD I-7).

## Acceptance criteria
Language choice
- [ ] A first visit with browser languages `de-AT, en` shows German; `it, fr` French; `it` English.
- [ ] The language drop-down is on the viewing, edit and upload pages, next to units and theme, naming each language in itself; the published, My workouts, About, Report and error pages follow the remembered choice without one (OQ-280). A choice applies at once, without reloading data (cursor, range and settings kept), and to every page later opened in this browser, and wins over the browser languages.
- [ ] No cookie, link, header or request parameter carries the choice; the only trace on the server is the fetch of the chosen language's file, which is not logged (TD I-7).
- [ ] With browser storage unavailable, blocked or unreadable, the browser's languages decide, without a message, and switching still works for that page.

Coverage
- [ ] In each language, every interface text of the pages and dialogs in Scope is in that language; only the exceptions listed in Scope stay as they are. The user reviews each language on screen (OQ-225).
- [ ] This includes, by feature: the summary dialog (F-58, AP-24) with its trigger tooltip, topics and "This workout" example on the viewing and edit pages; the Start and End hiding sliders' labels ("Start: hide 500 m", "Start: not hidden", in metres, feet or miles, F-55) and the hiding texts of the edit page; the "Download route (GPX)" button, the edit-page line under the sliders and the About sentence (F-57, SH-13); pins (F-56): "Add pin", the form (type, title, note, Section, Cancel), the six type names, the legend, the list, the dimmed reasons ("Removed on Save", "Hidden by the start radius" and the end radius) and the line with the number removed; My workouts (F-53, AC-7): headings, columns, statuses, relative expiry, Forget, Clear list and its question, the edit-link note, the empty-list and not-available lines.
- [ ] The F-58 example says the same in every language: no translation adds a value, and on the viewing page none states whether, which end or how far the start or end is hidden (AP-24).
- [ ] Texts with a count or a relative time ("in 3 days", the number of pins removed, "1 pin" and "2 pins") use the correct singular and plural in each language.
- [ ] Upload errors and messages (UP-2, UP-3, UP-8, UP-10, UP-11) appear in the chosen language on the upload page.
- [ ] A test fails when any interface text is missing in one of the four languages, so later features (F-61, F-62) cannot ship a text in English only (OQ-228).

Formats and layout
- [ ] Numbers and dates follow the language (decimal comma in German, Spanish and French; date order and month names); units do not change with the language (OQ-227).
- [ ] On a phone in portrait, the longest language causes no horizontal page scroll and no clipped button, label, slider label, dialog line, pin form or My workouts column.

Loading and fallbacks
- [ ] Without JavaScript, every page is English and works as before (OQ-223).
- [ ] A published page in a language other than English shows no visible switch from English: the language file is requested at the top of the page and the text shows once it arrives, falling back to English after at most 500 ms; the LCP budget still holds (OQ-281).
- [ ] Only the chosen language's file is loaded; each is at most about 5 KB gzip and outside the entry bundle; the entry bundle stays within its budget, and LCP stays under 2.5 s on the reference phone in German (OQ-226). Texts of code that loads lazily (pins, GPX, the summary dialog) may load with that code, within the same limits.
- [ ] A language file that fails to load leaves the page in English, without an error.
- [ ] A page published under the previous release still loads its texts after an update, until it is rebuilt (INF-9).

Not translated
- [ ] Titles, descriptions, pin titles and notes, report messages, the operator file and the Impressum, the preview cards (SH-5, SH-11, SH-12), the suggested title and the GPX file stay exactly as written or made; the stored report reason and the operator's notification stay English.
- [ ] The About page's "In your browser" paragraph lists the language choice (INF-1).

## Decisions (user, 14 Nov 2026)
- OQ-223 to OQ-228 accepted as suggested: static pages English without JavaScript; preview card and suggested title stay English; the engineer drafts the texts, reviewed on screen, one file per language; each language file lazy, about 5 KB gzip at most, outside the entry bundle; number and date formats follow the language, units stay separate; from F-54 on every feature ships its texts in all four languages.
- The language choice is remembered in browser storage like the units (DS-1) and listed in the About page's "In your browser" paragraph (INF-1).

## Decisions (user, 19 Nov 2026)
- **OQ-280:** the drop-down is on the viewing, edit and upload pages; the other pages follow the remembered choice.
- **OQ-281:** no visible switch on published pages: text shown once the language file arrives, English after at most 500 ms.

## Decisions (user, 20 Nov 2026)
- Fetching only the chosen language's file tells the server which language was chosen; accepted (request not logged, TD I-7). DS-6 reworded accordingly.
