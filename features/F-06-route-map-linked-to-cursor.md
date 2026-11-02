# F-06 Route map linked to the cursor

- **Phase:** 0 Prototype
- **Owner:** frontend
- **Requirements:** AP-3, AP-6 (map side), NFR Performance; Technical design: Front end (Load order step 4), Tech stack (Map, Map tiles)
- **Depends on:** F-05

## Goal
Show the route on an interactive map that loads last and stays in sync with the chart cursor in both directions.

## Scope
- MapLibre GL JS loaded with a dynamic `import()` after the charts render; OpenFreeMap tiles.
- Route connecting every sample that has a position, as one GeoJSON source with two features: recorded segments (solid) and bridges (dashed). A bridge is the straight, dashed segment across a GPS dropout (samples without position) or a time gap (5 s or more between consecutive positioned samples). Pan and zoom; initial view fits the route.
- Start and finish markers and lap markers (from lap sample indices in the metadata).
- Cursor marker as a single feature updated on each cursor-store change.
- Hovering or touch-dragging the route sets the cursor store to the nearest sample (self-crossing routes: on a tie within 3 px, the earliest sample wins).
- No position stream: MapLibre is not loaded; the hint "No location data in this workout." takes the map's place.
- A route hidden entirely (F-16): MapLibre is not loaded; the hint "The entire route has been hidden by the uploader." takes the map's place (AP-3).

## Out of scope
- Range highlight (F-17), route colouring (F-25), fullscreen (F-26), dark map style (F-14).

## Acceptance criteria
- [ ] Network trace: the MapLibre script is requested only after the charts have rendered.
- [ ] Scrubbing a chart moves the map marker; hovering the route moves the chart cursor and readout.
- [ ] Start, finish and lap markers appear at the correct positions for a corpus file with several laps.
- [ ] A ride with a several-minute GPS dropout, and one with a time gap of 5 s or more, each show one connected line with a straight, dashed segment across the dropout or gap.
- [ ] An indoor file shows "No location data in this workout." and the network trace contains no MapLibre or tile requests.
- [ ] When the cursor is on a sample without a position (dropout or hidden section), the marker is hidden rather than placed at a wrong point.
- [ ] Charts stay usable while tiles are slow or blocked (tested with tile requests throttled or failing).
