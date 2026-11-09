# F-42 Multisport sections: architecture

- **Version:** 0.1
- **Status:** accepted (9 Nov 2026). Normal lane. OQ-138 to OQ-143 decided as suggested (decision log); OQ-144, OQ-145 resolved (10 Nov 2026)
- **Feature:** [`features/F-42-multisport-sections.md`](../features/F-42-multisport-sections.md)
- **Requirements:** AP-12, AP-3, AP-4, AP-5, AP-7, AP-8, AP-15, SH-4, NFR Performance
- **Builds on:** [F-03](F-03-workout-data-file-writer.md) §5.3, §5.4; [F-06](F-06-route-map-linked-to-cursor.md) (lap markers); [F-18](F-18-laps-and-splits-table.md) L-2, L-3, L-7; [F-26](F-26-fullscreen-map.md); ADR-009, ADR-011 (rev. 14: this feature lazy), ADR-026, ADR-027. TD glossary "Section"
- **Owner:** both. B-1 backend first (MS-1 to MS-5 in both languages, samples, template), then the frontend (MS-6 to MS-9)

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| MS-1 | **Which workouts:** `meta.sessions` holds two or more distinct `sport` values, compared as written (`transition` counts). Nothing else is read; `meta.sport` is not used | A truncated one-session file with a differently hinted tail is `multisport` (F-08 D-17) but out of scope. One rule both languages evaluate from the metadata alone |
| MS-2 | **Section model.** Sessions ordered by `start`, file order on ties; one section each. Section *k* starts at its session's `start_index`, section 1 at sample 0, and runs to the next section's start: a gap belongs to the section before, samples before the first session to the first, samples after the last session to the last. Name: the sport name as shown elsewhere (`lib/format` / Go `sportName`); transition sections are "T1", "T2", … in order. A section with no samples (start ≥ *N*, or the same start as the next) keeps its separator row and T number but gets no line, name on the chart, marker or tap | Feature scope; one boundary rule (start indices only) for lines, rows, ranges and markers. F-08's legs (summary, record leg) are not reused: their tail rule differs |
| MS-3 | **Laps to sections.** A lap belongs to the last section whose start ≤ the lap's `start_index`. Lap rows keep `meta.laps` order and numbers; before each lap row come the separator rows of every section after the previous lap's section up to its own, after the last lap the remaining ones. No laps ("No laps recorded."): no separator rows. Splits unchanged | Lap order and numbering stay, so F-18 L-2's DOM-order mapping holds. Laps are in time order for every known writer (file order, F-02); an out-of-order lap stays under the separator above it, accepted |
| MS-4 | **One model, two languages, one contract.** Go: inside `internal/page` (its only server consumer; imports `datafile` only; `workout` unchanged). TS: new pure `lib/sections`, used by `lib/page`. Separator rows are part of the page view model with the name and the column span as strings (the template prints, never computes, ADR-026 item 3). Identity is pinned by the page samples: `triathlon` plus samples for a section without laps, a lap crossing a section start, a truncated tail, and a two-ride file (no sections) | The no-JS page and the edit page must agree row for row; the sample contract already enforces that for the laps table |
| MS-5 | **No data file change.** `meta.sessions` already carries `sport`, `start`, `start_index` (F-03 §5.3); format version, summary and goldens unchanged | Confirmed against F-03 §5.1, §5.3 |
| MS-6 | **Separator rows in the table.** One header cell spanning all columns, marked by a class. The static enhancers count only lap rows (units check, F-18 L-2); the name has no unit and is never rewritten. **Tap:** range `a` = the section start, `b` = the next section's start (the shared boundary sample, as adjacent laps, F-18 L-2) or *N* − 1 for the last; selectable when `b > a`. Separator rows use the lap rows' tap, mark and L-7 code: the static enhancer maps every row to a range by DOM order, separator rows are unselectable until MS-9's chunk supplies their ranges | OQ-140; one tap path, no second L-7 |
| MS-7 | **Charts.** A line at each section start after the first, in every chart, at the axis's carried x (`at[start]`, time and distance axis), so a selected section's shading ends on its lines. Names only in the top chart, inside the section from its left edge; a name wider than its section is left out, the line stays. Placed on layout (width, axis switch, chunk arrival), never on a cursor or selection flush; no pointer events. `components/charts` holds no section rule: it offers one generic decoration seam fed from outside | AP-5, AC 1; the 60 fps path and the gesture code stay untouched (ADR-008, ADR-009) |
| MS-8 | **Map.** A leg marker at each section start after the first, only when that sample has a position (the lap-marker rule, never a substitute point), labelled with the name; it replaces a lap marker at the same sample; kept in fullscreen like lap markers. Built in the lazy map chunk from `lib/sections`; `lib/route` still imports only `lib/datafile`, the starts reach the marker list as input. Privacy: no new public data; a start inside a hidden end has no position in the public file, so no marker (SH-4). The edit page reads the draft, so it shows markers inside the to-be-hidden marks, as for laps | OQ-141, AC 5, AC 6; follows AC 38 (see OQ-144) |
| MS-9 | **Lazy in the browser (ADR-011 rev. 14).** Section lines, names and separator ranges live in a chunk `src/sections/`, reached by one `import()` from `src/page` on the viewing and the edit page, only when MS-1 holds, after the charts' first paint. Static: MS-1's check, the call site, the two seams (MS-6, MS-7), separator CSS. Before it arrives, without JavaScript or when it fails to load: separator rows (server markup) without taps, no lines; nothing else changes | ADR-011 item 1: only what every viewer needs is static; sections serve few workouts and the reserve is below 1,000 |

## Cost

| What | Decision |
| --- | --- |
| Static | Ceiling **300 bytes gzip** (ADR-011 rev. 14). The engineer measures the entry before and after and records the figure in ADR-011 item 4. Above 300: stop and report |
| Lazy | `src/sections/` chunk with `lib/sections` and its CSS; map-chunk markers; `components/page` separator rows in the owner chunk. Not counted |
| Data file, HTML, LCP | Data file unchanged. HTML gains one row per section (multisport only). Nothing new before the summary |

## Open questions

None. OQ-144 and OQ-145 accepted by the user on 10 Nov 2026 (decision log, `features/README.md`).
