# UI review: edit page, add lap / add pin controls (history)

History of [architect/UI-edit-page-add-controls.md](../UI-edit-page-add-controls.md): the proposal of 23 Nov 2026 as it stood, moved here on 23 Nov 2026 after it was built and its open questions were decided (decision log of `features/README.md`, "edit page add controls"). Superseded afterwards: the "+" moved from the bottom-left stack to the top-right stack (commit 8cdd102). Read it only when you need the history.

---

- **Status:** proposal (23 Nov 2026), senior-ui-designer. Current state only.
- **Scope:** the "+" on the map (two columns), the add box under the map (one column), the right-click menu, and how they sit with the map controls, pin form, pins list, laps table and range panel. AP-8, PIN-1, PIN-7, AP-22, DS-4; [F-65](../../features/F-65-owner-added-laps.md) AL-9, [F-56](../../features/F-56-map-pins.md) PN-12; decision log 21–23 Nov 2026.
- **Basis:** code only (`src/web/src/edit/plus.ts`, `pins.ts`, `mapmenu.ts`, `laps.ts`, `edit.css`, `page/page.css`, `components/map/map-view.css`, `pins/pins.css`, MapLibre's CSS). Nothing was rendered; every "looks" judgement below is from reading CSS and needs a check in a browser. Findings marked **(bug?)** are most likely visible defects.

## Top problems (most visible first)

1. **Right-click menu has no surface (bug?).** `.map-menu` and its `.edit-button` items get no background (only `.wx-addpin .edit-button` is made white). The menu is transparent outlined buttons in `--fg` text, and reasons in `--muted` text, straight on the map tiles. Hard to read on both themes and on Satellite.
2. **"+" covers the pin legend (bug?).** `.wx-addpin` sits at `left: 10px; bottom: 10px` of the map slot. The pin legend is a MapLibre bottom-left control, which MapLibre places at the same 10 px / 10 px. On a workout with pins the "+" (painted later) covers the legend's first type buttons. In the short-landscape layout the "+" is also lifted by `100lvh - 100svh` while the legend is not, so they mis-align differently.
3. **Right-click menu is invisible in fullscreen (bug?).** It is appended to `#map-slot`, outside MapLibre's pseudo-fullscreen box; in fullscreen `body` is `visibility: hidden` and only that box is visible. The right-click is still taken (`preventDefault`), so the user gets neither our menu nor the browser's.
4. **The result appears far from the action (two columns).** After "Add pin" the pin form opens in the pins slot, which in two columns sits under the charts (`order: 1`), usually below the fold. Nothing scrolls or moves focus there. "Add lap" adds a row to the laps table, further down still.
5. **On desktop the target drifts.** The cursor follows hover on the route (ADR-015 item 4). Moving the mouse from the chosen spot to the bottom-left "+" can cross the route and move the cursor; the item then acts on a different spot. Nothing on the item says where it will act.
6. **One column, idle state is dead space.** With no cursor both buttons are disabled and both reason lines are empty (user decision 22 Nov: no hint). The result is a grey box about 90 px tall with two greyed buttons and nothing to explain them. Without a route the hidden "+ Pin" row still keeps its height (`visibility: hidden`), so the box has a permanently blank row.
7. **Touch targets are too small on phones.** Box buttons are about 32 px (`.edit-button` 6 px padding + 14 px text), the map "+" is 29 px, and the lap row × (`.lap-x-btn`, `padding: 0 8px`) is about 20 px, sitting inside a row that is itself a tap target (range selection).
8. **The explanation is out of reach.** The only text on how to add a lap or pin is in the How to use dialog, and the edit page has no How to use bar.

## Rules

1. **One model, two entry points.** Select a spot (cursor or range) first, then add. Two columns: "+" on the map, plus the right-click menu for a mouse. One column: the add box under the map. All of them have the same items in the same order, Add lap then Add pin, with the same states and reasons (`PlusItem`).
2. **Map overlays have two looks only.** *Buttons* look like MapLibre's buttons in both themes: `#fff` background, black text, 29 px tall (44 px on a coarse pointer), radius 4 px, `box-shadow: 0 0 0 2px rgba(0,0,0,.1)`. *Panels with text* (pin legend, colour box, reasons, the right-click menu) follow the theme: `--bg` background, `--fg` text, radius 4 px, the same shadow. Nothing transparent on the tiles.
3. **One owner per map corner.** Top-left: colour control. Top-right: zoom, fullscreen, pin toggle, satellite (attribution beside it in two columns). Bottom-left: "+" and pin legend, in MapLibre's bottom-left stack, so they can never overlap and share the Safari lift. Bottom-right: attribution (one column).
4. **Never shift layout.** Menus are overlays. Reserved lines keep their height, but only for content that can appear in this visit.
5. **A disabled item stays readable.** `aria-disabled`, opacity 0.6, the reason on the same line or right under it. Reasons are themed text, not buttons.
6. **Sizes.** Buttons are 32 px tall with a fine pointer and at least 44 px with `(pointer: coarse)`. Gaps are 4, 6 or 8 px, nothing else in this area.
7. **Show the result where the eye is.** After an add, the new thing is visible without scrolling, or focus moves to it.
8. **Motion.** 150 ms ease-out rise and fade, none under `prefers-reduced-motion`. This is already built; keep it.

## Changes, two columns (desktop and short landscape)

```
 map (left column)                                   right column
 ┌───────────────────────────────────────────┐ ┆
 │ [colour ▾]                       [+][-]    │ ┆
 │                                  [⛶]      │ ┆
 │        ~~~route~~~                [📍]     │ ┆
 │                                            │ ┆
 │  ┌───────────────┐  (open)                 │ ┆
 │  │ Add lap       │  ← white buttons        │ ┆
 │  │ Add pin       │    reason chip beside   │ ┆
 │  └───────────────┘    or under (themed)    │ ┆
 │  [ + ] [●][●][●]   ← "+" first, legend     │ ┆
 └───────────────────────────────────────────┘ ┆
```

- **Q-fix "+" placement (small lane):** add the "+" group as a control in MapLibre's bottom-left stack, through the existing `map.control()` seam, placed **before** the pin legend: "+" in the corner, the legend 10 px to its right, bottom edges aligned. Hide it in fullscreen (`html.wx-fs .wx-addpin { display: none }`), as now. Move the Safari lift from `.wx-addpin` to `.route-map .maplibregl-ctrl-bottom-left` in the two-column media query, so the legend gets it too.
- **Right-click menu (small lane):** one themed panel: `--bg` background, `--fg` text, 1 px `--line` border, radius 6 px, the rule 2 shadow, 4 px padding, `min-width: 160px`. Items are full-width rows, not outlined buttons: 32 px tall, 0 12 px padding, left-aligned text, no border; hover and focus get the `--range-shade` background and keyboard focus a 2 px `--accent` outline (inset). A disabled row is at opacity 0.6, with its reason as a 12 px `--muted` line under the label, inside the same row. This is the platform's context-menu look, which fits `role="menu"`. Keep the clamp inside the map box.
- **Right-click in fullscreen:** see question 2. If kept, the menu must be appended inside MapLibre's container (the pseudo-fullscreen box) so it is visible.
- **After "Add pin":** select the new pin as now, then `scrollIntoView({ block: 'nearest' })` on the pin form and focus its Title field. Under reduced motion, no smooth scrolling. After "Add lap": nothing moves. The map's lap marker is the feedback, and the status line stays empty.
- **Empty status line:** `.pin-tools.on-map` reserves 20 px for the status line under the charts even when it is empty. Keep the reserve only while the status shows text that can change during the visit. Simplest: the status line belongs to the pin form's row, so the 20 px goes.
- **Coarse pointer** (short-landscape phones): the "+" and its items `min-height: 44px; min-width: 44px`.

## Changes, one column (phone 320–390 px)

```
 ┌ map 320 px (or pinned 35lvh) ───────────┐
 └─────────────────────────────────────────┘
 ┌ box: --card, 1px --line, r 6, p 8 ──────┐
 │ [ + Lap ]  at 12.4 km              (Q1) │  44 px rows on touch
 │ [ + Pin ]  Already a pin here           │  reason or target, 13 px, ellipsis
 └─────────────────────────────────────────┘
 pin form (when a pin is selected)
 Pins ─ list
 charts …
 range panel (sticky bottom)
```

- Keep the approved two-row grid (`max-content | 1fr`). At 320 px the reason column is about 200 px wide, which fits every English reason at 13 px. German needs a check.
- Rows `min-height: 44px` with `(pointer: coarse)`, 32 px otherwise. Box padding 8 px and a 6 px row gap stay. The buttons keep `.edit-button`.
- Reserve the Add pin row only when the workout has positions (`data.position` exists); otherwise drop the row (`display: none`), so the box has no blank row without a route.
- Lap row ×: a 32 px square hit area (44 px on coarse) via padding. Its glyph stays 14 px. It stops propagation as now.
- The pin form follows directly under the box, as now; it is already visible after a tap.

## Open questions (decided 23 Nov 2026, see the decision log; question 2 changed: the menu also works in fullscreen)

1. **Show the target in the reason slot when an item is enabled** (for example "at 12.4 km", "3.1–4.2 km" for a range, in the readout's units), in the box, the "+" items and the right-click menu. This fixes problem 5 and fills the reserved line. Suggestion: yes.
2. **Right-click menu in fullscreen.** Fullscreen has no "+", no pin form and no laps table, so an add there gives no visible result beyond a marker. Suggestion: no menu in fullscreen (the browser's menu stays), matching the "+".
3. **Hint when nothing is selected.** Problem 6: the reserved line could say "Select a point or range first" while both buttons are disabled. It no longer causes a layout jump, which was the reason for the 22 Nov decision. Suggestion: yes, one column only. Two columns keep a disabled "+" with that text as its tooltip.
4. **How to use bar on the edit page.** The edit instructions live only in the viewing page's dialog. Suggestion: show the same bar on the edit page, above the "How these numbers are computed" bar, as on the viewing page.
5. **Decision log wording.** The 23 Nov entry says one column shows Add lap and Add pin "side by side", but the built and approved layout is two rows. Suggestion: requirements engineer rewords it to "two rows, button and reason".

## Broader observations (ranked)

1. **The settings panel sits above the page header** (F-19 E-10). On a phone the owner scrolls past the whole form (title to delete) before seeing the title, map or laps, and Save is far from the map, pins and laps edits that it also saves. Worth a question: header first, then the panel, or a sticky Save / status bar at the bottom in one column.
2. **Two filled primary buttons side by side** on a draft (Save and Publish both `.edit-save`). Suggest Publish filled and Save outlined (`.edit-button`), or the reverse; one primary per row.
3. **Button heights vary** in this area: 29 px (map), 32 px (`.copy`, `.gpx`, selects), about 32–34 px (`.edit-button`, `.edit-save`), 40 px (`.howto` bars). Align to rule 6: 32 px, and 44 px on a coarse pointer; the 40 px bars stay as bars.
4. **Pin form above its list.** In one column the form of the selected pin sits between the add box and the "Pins" heading, away from the selected row. Consider moving the form under the selected row, as the note opens there on the viewing page.
5. **Two 40 px bars plus gaps (96 px) before the summary** on a phone's viewing page. Fine on desktop; on 320 px consider one row with two half-width bars ("How to use" | "How numbers are computed"), only if the texts fit in all four languages.
6. **Theme mix on the map.** Buttons are light while panels follow the theme. This is consistent with rule 2 but should be checked on Satellite in dark mode, where a dark legend chip on dark imagery may lack an edge. If so, add a 1 px `--line` border to themed map panels.
