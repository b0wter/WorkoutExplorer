# F-29 About page and attribution list

- **Phase:** 1 MVP
- **Owner:** both
- **Requirements:** INF-1, DS-4 (theme, decision log OQ-68), NFR Privacy (privacy policy, cookie-less, no tracking); Technical design: Hosting and operations (About page)
- **Depends on:** F-13

## Goal
One static page that is the privacy policy and the complete, automatically generated attribution list.

## Scope
- Frontend: About page template, built in the release build and served by Go at `/about` with the operator part, version and Pushover block filled at start (F-29 design AB-1, AB-3, AB-4), linked from the footer of every page (upload, workout, edit, generic, About).
- Light and dark theme from `prefers-color-scheme`, CSS only, the same theme as the other pages (DS-4, F-14).
- The page has two parts (design AB-2): an **app part** in the template, true for every deployment of this code, and an **operator part** from the operator file (`WORKOUT_EXPLORER_OPERATOR_FILE`, design AB-8) supplied per deployment: who runs the site and contact, hosting, what the TLS proxy logs, backups, legal basis and complaints. The repository ships only `deploy/operator.example.txt` with prompts.
- App part content: what is stored (FIT file, workout data, settings, reports), what the viewer's own browser keeps (the units choice, DS-1; the x axis, AP-5; the map/content split, AP-17; the theme choice, DS-4: in browser storage, no cookie, never sent to the server, stays until the viewer clears it), and a concrete list of what is removed from every FIT file on upload (UP-9): device and sensor serial numbers and ANT+ IDs, wherever the file records them; the user profile (name, gender, age, height, weight, heart-rate settings); saved locations; segment leaderboard entries, the segment leaders' activity IDs and segment account keys; raw ANT radio messages. Also how it is used, who else sees what (the map tile host sees viewers' IP addresses), how long it is kept (7 days by default, at most 30; no backups; and this sentence, exactly: "Workouts are not archived: after an app update, a damaged file may show less than before or be removed."), no cookies or tracking.
- Backend/CI: attribution list generated in every build from the npm packages the bundler ships and the Go modules linked into the binary, plus a committed sources file for map data and other sources: OpenStreetMap via OpenFreeMap; the FIT Protocol licence (OQ-5); GeoNames when place names ship (design AB-5).
- Operator part missing (setting unset or file absent): one clearly marked placeholder block naming what is missing, filled in by the operator before launch (F-32).

## Out of scope
- Data-processing record (operator document, F-32 checklist).

## Acceptance criteria
- [ ] Every page type has a footer link to the About page.
- [ ] The page lists the four kinds of stored data, their use, the units choice, x axis, map/content split and theme choice kept only in the viewer's browser, the tile host's access to IP addresses, the 7/30-day lifetime, the absence of backups, and the sentence "Workouts are not archived: after an app update, a damaged file may show less than before or be removed." word for word.
- [ ] The page lists every kind of data removed on upload, as in UP-9: serial numbers, ANT+ IDs, the user profile (name, gender, age, height, weight, heart-rate settings), saved locations, segment leaderboard entries, the segment leaders' activity IDs and segment account keys, raw ANT radio messages.
- [ ] Adding a new npm or Go dependency makes it appear on the next build's attribution list without manual edits; CI fails if a dependency's licence cannot be determined.
- [ ] OpenStreetMap/OpenFreeMap attribution and licence and the FIT Protocol licence are listed.
- [ ] The template states no fact about one deployment (operator, hosting, proxy logs, backups, legal basis); `deploy/operator.example.txt` has a prompt for each of these, and the page shows the operator file's content as escaped text, or the placeholder block when it is missing.
- [ ] The page sets no cookies and loads no third-party resources.
- [ ] The page follows the OS light or dark theme, also with JavaScript off.
