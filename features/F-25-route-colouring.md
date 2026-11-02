# F-25 Route colouring by metric

- **Phase:** 1 MVP
- **Owner:** frontend
- **Requirements:** AP-9 (Should), NFR Performance (60 fps); Technical design: Tech stack (Map)
- **Depends on:** F-06, F-14

## Goal
Viewers colour the route by speed, heart rate, power or gradient to see where effort and terrain change.

## Scope
- Selector offering only metrics the workout has; gradient only when altitude and distance exist.
- Gradient computed in the browser from altitude and distance. Where distance does not change between samples (a standstill or a held device dropout, F-08 R-14), there is no gradient: drawn as a missing value, never as an extreme one.
- Colour scale with a legend in the viewer's units. For the pace sports (DS-2), the speed option is labelled pace and its legend shows pace.
- Default: plain single-colour route (assumption, see README); the choice is not stored.
- Loaded with the map; no cost before the map loads.

## Out of scope
- Zones-based colouring (AP-10, phase 2).

## Acceptance criteria
- [ ] A ride with heart rate but no power offers speed, heart rate and gradient, not power.
- [ ] An indoor file, or a route hidden entirely (F-16), shows no selector (no map).
- [ ] Colouring by speed shows faster sections in a different colour than slow ones on a corpus ride; legend values match the units drop-down.
- [ ] On a run, the speed option is labelled pace and the legend shows pace (min/km or min/mi), as in the summary (DS-2).
- [ ] Scrubbing on the 12-hour file with colouring on still runs at 60 fps on the operator's phone.
- [ ] Hidden start and end sections stay hidden when colouring is on.
- [ ] Dashed segments across GPS dropouts and time gaps stay dashed and uncoloured when colouring is on, since they have no recorded data (AP-3).
