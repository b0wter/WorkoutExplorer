# F-18 Laps and splits table

- **Phase:** 1 MVP
- **Owner:** both (backend first: lap values computed on the server)
- **Requirements:** AP-8, AP-7 (row selects range), DS-1 (1 km or 1 mi splits); Technical design: Front end (Load order step 1)
- **Depends on:** F-17

## Goal
A table comparing the device's laps and automatic 1 km or 1 mi splits; tapping a row selects that section.

## Scope
- Laps from the device (pre-rendered in the HTML by the template from F-13 and adopted by the client without re-rendering, ADR-026).
- Automatic splits of 1 km (metric) or 1 mi (imperial), computed in the browser from the distance stream; the last partial split shown with its actual length. Split k ends at the first sample at or beyond k km (or k mi); the stream never decreases (F-08 R-14), so each boundary is crossed once, but it can stay flat and have samples without distance.
- Switch between laps and splits. Every page starts on laps; the choice is not remembered (OQ-76).
- A workout with distance but no device laps shows the laps section with "No laps recorded." and the switch offers splits (OQ-77). Without laps and without distance there is no laps section.
- Columns (provisional; review once visible): number, time, distance, average speed or pace, gain, average heart rate, power and cadence, limited to metrics the workout has; missing metrics hidden, never zero. Laps use the device's lap values where recorded, otherwise values computed as for a range (F-17), on the server so they show without JavaScript. A lap that starts in a transition gets no computed speed, cadence or power; it shows "–", as in the summary (OQ-78). Splits use the same columns, computed.
- Tapping or clicking a row selects that range via the F-17 selection store (charts shaded, map highlighted, panel filled). The panel shows range values, which may differ from a device lap's row values; no note (OQ-79).
- From 1024 px the range panel appears in the page above the laps, so the table moves down when a row is tapped; this jump is accepted for chart drags (user, 27 Oct 2026). When a row tap makes the panel appear, the tapped row stays in place on screen, in all layouts (OQ-75).
- On a phone in portrait the panel's place is above the laps; after a row tap while reading the laps it may be above the screen, where the viewer scrolls up to read it (accepted, OQ-80; AP-7).

## Out of scope
- Swim lengths (AP-13, phase 2), multisport legs (AP-12, phase 2).

## Acceptance criteria
- [ ] A 10.4 km run in metric shows splits 1–10 of 1 km and an eleventh of 0.4 km; switching to imperial shows 6 full miles and one partial.
- [ ] A file with 5 device laps shows 5 lap rows with the device's lap values.
- [ ] A file with a single device lap shows one lap row and the splits still work.
- [ ] A file without distance shows the laps table but no splits option.
- [ ] A file with distance but no device laps shows "No laps recorded." in the laps section, and the switch shows its splits.
- [ ] A lap that starts in a transition shows "–" for speed, cadence and power; its other columns are filled.
- [ ] Tapping a lap row selects exactly that lap's time range in charts, map and panel.
- [ ] When a row tap makes the range panel appear, the tapped row does not move on screen, on desktop, on a phone in landscape and on a phone in portrait.
- [ ] Every page load starts on laps, even after the viewer switched to splits on an earlier load.
- [ ] The laps table is visible with JavaScript disabled on a published page.
