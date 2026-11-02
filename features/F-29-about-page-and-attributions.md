# F-29 About page and attribution list

- **Phase:** 1 MVP
- **Owner:** both
- **Requirements:** INF-1, DS-4 (theme, decision log OQ-68), NFR Privacy (privacy policy, cookie-less, no tracking); Technical design: Hosting and operations (About page)
- **Depends on:** F-13

## Goal
One static page that is the privacy policy and the complete, automatically generated attribution list.

## Scope
- Frontend: static About page built at deploy, linked from the footer of every page (upload, workout, edit, generic, About).
- Light and dark theme from `prefers-color-scheme`, CSS only, the same theme as the other pages (DS-4, F-14).
- Content: what is stored (FIT file, workout data, settings, reports), what the viewer's own browser keeps (the units choice, DS-1; the x axis, AP-5; the map/content split, AP-17: in browser storage, no cookie, never sent to the server, stays until the viewer clears it), and a concrete list of what is removed from every FIT file on upload (UP-9): device and sensor serial numbers and ANT+ IDs, wherever the file records them; the user profile (name, gender, age, height, weight, heart-rate settings); saved locations; segment leaderboard entries, the segment leaders' activity IDs and segment account keys; raw ANT radio messages. Also how it is used, who else sees what (the map tile host sees viewers' IP addresses), how long it is kept (7 days by default, at most 30; no backups; and this sentence, exactly: "Workouts are not archived: after an app update, a damaged file may show less than before or be removed."), no cookies or tracking.
- Backend/CI: attribution list generated in CI from Go modules (e.g. `go-licenses`) and npm dependencies (e.g. `rollup-plugin-license`), plus data sources: OpenStreetMap via OpenFreeMap; GeoNames when place names ship (see README).
- Operator name and contact address for GDPR, as clearly marked placeholders (e.g. `[OPERATOR NAME]`, `[CONTACT EMAIL]`) that the operator fills in before launch (F-32).

## Out of scope
- Data-processing record (operator document, F-32 checklist).

## Acceptance criteria
- [ ] Every page type has a footer link to the About page.
- [ ] The page lists the four kinds of stored data, their use, the units choice, x axis and map/content split kept only in the viewer's browser, the tile host's access to IP addresses, the 7/30-day lifetime, the absence of backups, and the sentence "Workouts are not archived: after an app update, a damaged file may show less than before or be removed." word for word.
- [ ] The page lists every kind of data removed on upload, as in UP-9: serial numbers, ANT+ IDs, the user profile (name, gender, age, height, weight, heart-rate settings), saved locations, segment leaderboard entries, the segment leaders' activity IDs and segment account keys, raw ANT radio messages.
- [ ] Adding a new npm or Go dependency makes it appear on the next build's attribution list without manual edits; CI fails if a dependency's licence cannot be determined.
- [ ] OpenStreetMap/OpenFreeMap attribution and licence are listed.
- [ ] The page sets no cookies and loads no third-party resources.
- [ ] The page follows the OS light or dark theme, also with JavaScript off.
