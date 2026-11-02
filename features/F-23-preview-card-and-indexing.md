# F-23 Preview card, Open Graph and indexing

- **Phase:** 1 MVP
- **Owner:** both (backend first)
- **Requirements:** SH-5, SH-11, NFR Privacy (unlisted pages carry `noindex`); Technical design: Tech stack (Preview images), Security and privacy (Indexing and preview cards)
- **Depends on:** F-16

## Goal
A link posted in a messenger shows a rich card with the route and headline values, while leaked links stay out of search results.

## Scope
- `preview.{hash}.png`, 1200 × 630, under 300 KB, rendered in Go: route (after hiding) on a plain background plus a sport icon (run, ride, swim, walk/hike, generic) in front of the workout's name (cut at 40 characters plus "…"); below it distance and duration (moving time; elapsed time when the workout has no GPS); elevation gain only when altitude was recorded and the gain is above zero; pace for runs, walks and hikes (min/km), speed for rides (km/h), pace per 100 m for swims; average heart rate when present; distance, pace or speed only when present (SH-5, decision log 31 Oct 2026, OQ-91).
- Workouts without positions, or fully hidden (F-16): card with name and values only, no hint text (assumption).
- Open Graph and Twitter tags early in `<head>`: `og:title`, `og:description` with the same values, absolute `og:image` URL, `og:image:width` 1200, `og:image:height` 630, `twitter:card` `summary_large_image`.
- Regenerated with the data file when hiding or other stream-affecting settings change; old image deleted.
- No `robots.txt` (`404`), so nothing disallows `/w/`. The `X-Robots-Tag` header is F-22's, on every response (decision log 31 Oct 2026, OQ-89).
- Draft and expired links: generic page without route or numbers (see README on "generic card").
- Edit page (`/edit/{id}`) and published page (`/published/{id}`): fixed, workout-independent OG/Twitter tags, title "Edit link: do not share" plus one short sentence; no image (F-23 design PC-9; SH-11, decision log 31 Oct 2026).
- Base URL for absolute links as a server setting.

## Out of scope
- Units in the preview: metric assumed (see README). Map tiles behind the route (fallback, not chosen).

## Acceptance criteria
- [ ] A published ride's preview PNG is 1200 × 630, under 300 KB, and shows the route shape, the ride icon and name, distance, moving time, speed in km/h and average heart rate, plus elevation gain when altitude was recorded and the gain is above zero.
- [ ] An indoor workout without heart rate shows the name and elapsed time, and no distance, pace, speed, elevation or heart-rate value or zero.
- [ ] A run shows the run icon and pace in min/km; a pool swim shows the swim icon and pace per 100 m; a walk or hike shows the walk/hike icon and pace in min/km; any other sport shows the generic icon.
- [ ] A name longer than 40 characters is cut to 40 plus "…".
- [ ] With start and end hiding on, the preview route visibly lacks the hidden ends (compared with hiding off).
- [ ] The HTML contains every listed OG/Twitter tag with an absolute image URL.
- [ ] `/robots.txt` answers `404` (no rule for `/w/`).
- [ ] A draft and an expired link each produce no image and no numbers when fetched by a messenger crawler.
- [ ] `/edit/{id}` and `/published/{id}` return the same OG/Twitter tags for any two workouts and for a draft; title "Edit link: do not share"; no workout name, value, route image or ID in any tag (SH-11).
- [ ] Manual check: pasting an edit link in WhatsApp, Signal and Slack shows the warning card.
- [ ] Manual check: the card renders in WhatsApp, Signal, Slack, Telegram and X.
