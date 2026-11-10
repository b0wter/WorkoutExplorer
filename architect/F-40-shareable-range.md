# F-40 Shareable range: architecture

- **Version:** 0.1
- **Status:** accepted (9 Nov 2026). Normal lane. No open questions
- **Feature:** [`features/F-40-shareable-range.md`](../features/F-40-shareable-range.md)
- **Requirements:** AP-19, AP-7, AP-8, SH-6, AP-22, NFR Performance, NFR Privacy
- **Builds on:** [F-17](F-17-range-selection-and-axis-toggle.md) R-1 (selection store), [F-39](F-39-how-to-use.md) HU-4, HU-7, ADR-011, ADR-020 (rev. 3 item 11 adds the range link), ADR-038 (fullscreen history entry)
- **Owner:** frontend only. No server, template or Caddy change

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| SR-1 | **Format (ADR-020 item 11):** `/w/{public_id}#range={from}-{to}`, time offsets (TD glossary) in whole seconds, `from` rounded down from the first selected sample's offset, `to` rounded up from the last's. Accepted only when the **whole** fragment matches `range=` + 1 to 7 digits + `-` + 1 to 7 digits; anything else (other keys, `&`, signs, decimals, spaces, percent-escapes, a second `range`) is ignored as a whole. No `decodeURIComponent`, no number parsing outside the match | Bounded, allocation-light parse of hostile input; no exception path. 7 digits allow offsets up to about 115 days |
| SR-2 | **To samples:** the first sample with offset ≥ `from` to the last with offset ≤ `to`, both cut to the data. Ignored, with no message, when `from ≥ to`, or fewer than two samples fall inside (the selection store's `a < b`). Offsets come from the decoded public data file only; never distance or coordinates | AC 4, AC 6, AC 7. Offsets are whole seconds, so a round trip is exact on our data; hidden sections need nothing special (the store is set as by hand) |
| SR-3 | **One pure module `lib/rangelink`** holds the format and the parse (offset array in, sample pair or nothing out; sample pair in, fragment out). Imports nothing; imported only by `src/page`. Never touches `location`, the DOM or storage | Security-relevant parsing tested in Node (TD front-end rules) |
| SR-4 | **Viewing page only, read once.** `src/page` reads the fragment once, after the data file is decoded and before the charts mount, and sets the selection store, only on the path without the edit seam: the edit and published pages never read or write it, and their token fragment is untouched. No `hashchange` listener: a fragment edited in the address bar applies on reload. No scroll or zoom | AC 5; one read, one place |
| SR-5 | **Address bar.** Selecting never writes the address. At the viewer's first range change or Clear on the page (not the link's own range arriving in the store), any fragment is removed with `history.replaceState`, keeping `history.state` (ADR-038's fullscreen entry) and the query (`?measure`). No history entry. An ignored, invalid fragment stays until then | AC 8 |
| SR-6 | **F-39 hint.** The range applied from the link does not mark the hint done (F-39 HU-4) | It is not the viewer's selection |
| SR-7 | **"Copy link to this selection" in `RangePanel`, next to Clear.** (User 10 Nov 2026, replacing the `.copy` look: Clear and this button share their own row under the values, drawn as links; the fallback field sits below the row.) The panel only shows the button and calls a copy action the page passes in; without it (edit page) there is no button. The page's action builds the link from the Copy link anchor's view path (`viewingUrl`, never `location`) plus SR-1's fragment for the current range, and runs **the same copy routine as Copy link** (Clipboard API, "Link copied" for 2 s, read-only selected field as fallback): one routine in `src/page`, used by both buttons. The fallback field sits in the panel and never shows a link for another range than the current one (updated or removed on change). Copy link itself stays plain | SH-6 behaviour without a second copy; keeps `components/range` free of clipboard and URL rules |
| SR-8 | **F-39 dialog line** "Share a range" goes into the `src/howto/` chunk | 0 static bytes |

## Cost

| What | Decision |
| --- | --- |
| Static | `lib/rangelink`, the read, apply and `replaceState` glue, the panel button. Share **700 bytes gzip** from the ADR-011 reserve; measured and recorded in ADR-011 item 4 like F-39. Above 700: stop and report |
| Lazy | One line in the F-39 chunk |
| LCP | Unchanged: the fragment is read after the decode, nothing new before the summary |
