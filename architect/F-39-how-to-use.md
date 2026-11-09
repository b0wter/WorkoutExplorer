# F-39 How to use: architecture

- **Version:** 0.1
- **Status:** accepted (9 Nov 2026). Normal lane. OQ-133, OQ-134 resolved (9 Nov 2026)
- **Feature:** [`features/F-39-how-to-use.md`](../features/F-39-how-to-use.md)
- **Requirements:** AP-22, AP-7, DS-1, NFR Performance, NFR Privacy
- **Builds on:** [F-14](F-14-units-and-theme.md) (theme head script), [F-17](F-17-range-selection-and-axis-toggle.md) R-12 (hint), [F-28](F-28-report-form.md) (lazy `<dialog>`), ADR-011, ADR-026 rev. 1, ADR-032 item 3. Changes TD Load order step 1 (the head script also sets `data-hint`)
- **Owner:** frontend only (the Caddyfile hash and the Caddy check's expected string change with the script, as for DS-4)

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| HU-1 | **Two `lib/prefs` entries:** `wx.howto.opened` and `wx.hint.done`, value `"1"`, written once on the event (dialog shown; first range selection or ×), never removed by the app. Any other value, a missing entry or unavailable, blocked or throwing storage reads as "not yet" (the module's existing guard), so the button stays emphasised and the hint shows, silently. `lib/prefs` stays the only code that reads or writes storage, apart from the read-only head script | Same rules as the other preferences (DS-1); AC 7, AC 8 |
| HU-2 | **No reserved hint row on later loads: the existing theme head script also reads `wx.hint.done`** and sets `<html data-hint="done">` when it is exactly `"1"`. Page CSS then sets the charts slot's `--hint` to 0, **on the viewing page only** (the rule must not match under the edit page's `#wx-edit`). One script, still byte-identical in `index.html` and `edit.html`, inside the same `try`. It gets a new `sha256-`; the Caddyfile and the Caddy check list the old and the new hash until no published page carries the old text (ADR-032 item 3; after a regeneration or 30 days) | The slot height must be right before first paint, and only an inline script runs then. Rejected: the bundle removing the row (layout shift, AC 6); reserving the row forever (spec: no space on later loads); a second script (two hashes for good) |
| HU-3 | **The bundle takes "hint shown" from the attribute, not from storage.** At start the viewing page reads `data-hint`: present → no hint line; absent → the line with its ×. When the hint is done during the page (×, or the first range selection by drag or lap/split row), the page writes the flag and the line empties but keeps its 24 px; the page never sets `data-hint` at runtime. The edit page always shows today's hint, no × | CSS reservation and rendering read the same fact once per load, so they cannot disagree |
| HU-4 | **`components/charts` stays free of storage.** `ChartStack` gets the hint mode from the page and reports a dismissal upward; the page decides "first selection" from the selection store and writes `lib/prefs`. F-40's range applied from a link never counts as the viewer's first selection | Keeps the TD rule (`lib/prefs` imported only by `src/page`) |
| HU-5 | **The ×** sits at the line's end and never ellipsises away (the text does); hit area at least 24 × 24 px inside the 24 px row; accessible name "Dismiss hint" | Small phones; NFR Accessibility |
| HU-6 | **The button is template markup, revealed by an enhancer.** `index.html` gets the button (text and icon) right after Copy link in `.ctl`, with a fixed box like the units and theme slots, invisible and inert until the enhancer reveals it (a class, no inline style; not `hidden`, which would drop its space). Without JavaScript: an empty reserved box, as the units and theme slots today. Emphasised and plain looks have the same box, so the switch causes no shift. Enhanced before any await (as Report), so it works when the data file fails. The edit page's `PageHeader` has no button and no box | ADR-026 rev. 1: no shift from bundle-only controls. Text and icon in the HTML cost no bundle bytes |
| HU-7 | **The dialog is a lazy chunk `src/howto/`**, plain DOM, one native modal `<dialog>` built on first open and reused, CSS in the chunk, reached only through one `import()` from `src/page` (the F-28 pattern; ADR-011 item 2 gains it). Both wordings are in the dialog, chosen in CSS by `(pointer: coarse)` as the hint. Close button, Esc (native) and a tap on the backdrop close it; no history entry. A failed load shows one line next to the button, as Report. "Opened" is written when the dialog is shown | AC 4, AC 5; the chunk holds every text, so it costs nothing static |
| HU-8 | **Phones:** the control row may wrap to a second line on narrow phones (OQ-133, accepted). The box is reserved in the markup, so this is a taller header, not a shift | Spec: on phones too, never in a menu |

## Cost

| What | Decision |
| --- | --- |
| Static | Enhancer, the `import()` call site (the preload helper is already in the entry, ADR-011 item 4), hint mode and ×, two prefs entries, CSS. Share **600 bytes gzip** from the ADR-011 reserve; the engineer measures the entry before and after and records the figure in ADR-011 item 4. Above 600: stop and report |
| Lazy | `src/howto/` chunk and its CSS, not counted |
| HTML | Button markup and one head-script statement, well inside 15 KB |

## Open questions

None. OQ-133 and OQ-134 accepted by the user on 9 Nov 2026 (decision log, `features/README.md`).
