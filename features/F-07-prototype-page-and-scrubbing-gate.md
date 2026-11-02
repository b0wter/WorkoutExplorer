# F-07 Prototype page and scrubbing gate

- **Phase:** 0 Prototype
- **Owner:** both
- **Requirements:** Roadmap gate 0 (smooth cursor scrubbing on a 12-hour workout on the operator's phone), NFR Performance (60 fps, data size); Open question: Metric definitions
- **Depends on:** F-03, F-05, F-06

## Goal
Put parser, data file, charts and map together on a page the operator can open on their phone, and pass the phase 0 gate before MVP work starts.

## Scope
- Backend: generate `data.bin` / `data.bin.br` for the 12-hour file and a 4-hour ride with the diagnostic CLI subcommand (F-02).
- Frontend: a prototype page that loads a given data file and shows charts and map with the synced cursor; built bundle served with brotli so the phone sees realistic transfer sizes.
- Hosted where the operator's phone can reach it (local network or a temporary host); no upload, no database.
- Measurement on the reference phone (iPhone 13 mini, iOS 27, Orion, Low Power Mode off, portrait, production build, synthetic `ride-12h` with the map in view): the in-page scrub record of a scrubbing session on charts and on the route counts as the performance trace. Desktop reference: Firefox on Linux. Android is checked in F-32. Real long files are information only and never committed.
- Record the results (fps, data sizes, decode time, bundle size) in the repository.
- Fix a known F-06 limitation (QA-06-r4-1, user's decision to defer it to F-07): the map's pixel ratio is frozen at mount, so after desktop browser zoom or a move to another monitor the map stays blurry until reload; phones are unaffected. Update the pixel ratio when the device pixel ratio changes (e.g. `map.setPixelRatio`; details in the F-06 design).
- Settle the "Metric definitions" open question with the user while the prototype is in hand, so F-13, F-17, F-18 and F-23 have definitions to build against.

## Out of scope
- Everything in phase 1: upload, storage, publishing, edit page, units, page layout.

## Acceptance criteria
- [ ] The 12-hour file (about 43,000 samples per stream) loads on the operator's phone and shows all charts and the map.
- [ ] Touch-dragging along a chart for at least 10 s moves the map marker continuously; the trace shows 60 fps during the scrub, and the operator judges it smooth. **Gate: phase 1 starts only when this passes.**
- [ ] Hovering/dragging on the route moves the chart cursor at the same smoothness.
- [ ] The 4-hour ride's `data.bin.br` is under 500 KB.
- [ ] The app bundle stays under 60 KB compressed (CI check from F-01).
- [ ] If the gate fails, the bottleneck is identified and either fixed or a fallback from the technical design (custom canvas renderer, Leaflet) is evaluated before moving on.
- [ ] On desktop, after browser zoom or moving the window to a monitor with a different pixel ratio, the map is sharp again without a reload (QA-06-r4-1).
- [ ] Metric-definition decisions (or an explicit decision to defer them) are handed to the requirements engineer for the spec.
