# F-14 Units, pace or speed, and theme

- **Phase:** 1 MVP
- **Owner:** frontend
- **Requirements:** DS-1, DS-2, DS-3, DS-4 (Should); decision log 24 Oct 2026 (OQ-66 to OQ-68), 29 Oct 2026 (units remembered), 3 Nov 2026 (theme drop-down); Technical design: Front end (Implementation notes)
- **Depends on:** F-13

## Goal
Viewers see every number in their units and the sport's natural pace or speed, and the page follows the system light or dark theme unless the viewer picks one.

## Scope
- Default units from the browser language (mapping assumed in README); a units drop-down switches metric/imperial. A choice made in the drop-down is remembered in `localStorage` on the viewer's device (no cookie, never sent to the server) and applies to every workout and edit page later opened in that browser, over the browser-language default. Unavailable, blocked or unreadable storage falls back silently to the default.
- Units apply to summary, chart axes and readouts, range panel, laps and splits, and the route colour legend (F-25): distance (km/mi), elevation (m/ft), speed (km/h, mph), pace (min/km, min/mi), temperature (°C/°F).
- Sport-specific display (DS-2): pace for running, walking and hiking; per 100 m (or 100 yd) for swimming; per 500 m for rowing; speed for cycling and every other sport.
- For the pace sports, pace also in the chart and the cursor readout (OQ-66): axis flipped so faster is up; stops (speed at or near 0) are gaps in the line with "–" in the cursor readout; no fixed slowest pace: the axis range comes from the workout's own data, and slow outliers are clipped to the axis edge. The engineer proposes the stop threshold; the user reviews it on screen.
- Distance in km or mi for every sport, swims included (OQ-67).
- Server-rendered summary and laps re-rendered client-side when the default or chosen units differ from the pre-rendered ones (see README on pre-render units).
- Switching units or theme never refetches data.
- Light and dark theme, including charts and, where available, the map style. On the activity and edit pages a theme drop-down in the header's control row, next to the units drop-down and Copy link, styled like the units drop-down, offers System (default, follows `prefers-color-scheme`), Light and Dark. A choice is remembered in `localStorage` like the units choice (no cookie, never sent to the server, silent fallback to System) and applied before first paint, so the page never flashes in the other theme.
- The upload page gets the same theme, CSS only, following the system setting, no drop-down (OQ-68).
- English only (DS-3).

## Out of scope
- German interface (Later). Units on the preview image and Open Graph text (F-23).
- Swim distance in yards (phase 2, with pool lengths, AP-13, AP-16).
- Theme of the About page (F-29).

## Acceptance criteria
- [ ] A browser set to `en-US` opens a ride in miles, feet, mph and °F; `de-DE` and `en-GB` open it in kilometres, metres, km/h and °C (per assumed mapping).
- [ ] Switching the drop-down updates summary, charts, readout, range panel and table at once, without a network request.
- [ ] Without a stored choice, the page opens in the browser-language default, and nothing is written to browser storage until the viewer uses the drop-down.
- [ ] After switching to imperial in a `de-DE` browser, a reload and another workout's page both open in imperial (stored value wins over the browser-language default); the same in reverse for metric in an `en-US` browser. The edit page (F-19) uses the same stored value.
- [ ] With browser storage unavailable or throwing (e.g. blocked site data, private mode that refuses writes) or holding an unknown value, the page opens in the browser-language default with no message or console error, and the drop-down still switches units for that page.
- [ ] A run shows pace (min/km or min/mi); a ride shows speed; a pool or open-water swim shows pace per 100 m (or per 100 yd in imperial); a rowing session shows pace per 500 m; a sport with no special rule (e.g. inline skating) shows speed.
- [ ] For a run, the speed chart is a pace chart with faster up, and the cursor readout shows pace; a stop is a gap in the line and the cursor readout shows "–" there; the axis range comes from the workout's own data, and a slow outlier is clipped to the axis edge and does not flatten the line.
- [ ] A swim's distance shows in kilometres or miles, never in yards.
- [ ] With the drop-down on System, switching the OS to dark mode switches the page, charts and map style (where available) without a reload.
- [ ] The activity and edit pages show the theme drop-down in the header's control row next to the units drop-down and Copy link, with System, Light and Dark; without a stored choice it shows System.
- [ ] Choosing Light or Dark switches the page, charts and map style (where available) at once, without a network request, and the OS setting no longer changes it; choosing System follows the OS again.
- [ ] After choosing Dark in a browser set to light, a reload and another workout's page open in dark with no light frame shown first (checked with a performance trace or slowed-down recording); the same for Light in a browser set to dark. The edit page uses the same stored value.
- [ ] With browser storage unavailable, throwing or holding an unknown value, the page opens in System with no message or console error, and the drop-down still switches the theme for that page.
- [ ] The upload page follows the OS light or dark theme, also with JavaScript off.
- [ ] After switching units and theme, no cookie exists, `localStorage` holds exactly one entry for the units choice and one for the theme choice and nothing else, and no request sent by the page contains either choice.
