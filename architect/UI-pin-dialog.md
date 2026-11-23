# UI: the pin dialog (edit page)

- **Status:** accepted, built (23 Nov 2026, bdc2b66). Current state only.
- **Scope:** adding and editing pins on the edit page through one modal dialog; the read-only pins list. PIN-1, PIN-3 to PIN-5, PIN-8, AP-15, DS-4, DS-6; [F-56](../features/F-56-map-pins.md) PN-12, [F-65](../features/F-65-owner-added-laps.md) AL-9; user decision 23 Nov 2026 (dialog replaces "scroll to the form, focus Title").
- **Basis:** code only (`src/web/src/edit/pins.ts`, `plus.ts`, `mapmenu.ts`, `addedtable.ts`, `edit.css`, `src/pins/index.ts`, `pins.css`, `src/dialog/`). Nothing was rendered.

## Rules

1. **One dialog for add and edit**, built once on first use with the info dialogs' shell (`openDialog` in `src/dialog`, class `dlg pin-dlg`): native modal `<dialog>`, `--bg`/`--fg`/1 px `--line`, radius 8 px, 50 % backdrop, page scroll locked without shift. No new pattern, no animation.
2. **Nothing changes until OK.** Cancel, Esc, the Android back gesture and a tap on the backdrop all close without a change. A new pin exists only after OK. Changes still reach the public page only with the page's Save (OQ-274).
3. **Opening for edit selects the pin, closing deselects it.** Opening calls the view's existing select (marker, chart mark and row highlighted, cursor moved, a section's range selected, PIN-5). Any close calls `view.select(null)`; a section's range stays selected. So the next tap on any pin opens the dialog again; the view's tap toggle never fires on the edit page.
4. **The list is read-only:** icon, title, distance (or dim reason), ×. No fields, no note under the row.

## Openers

| Opener | Opens | Focus returns to |
| --- | --- | --- |
| "Add pin" in the "+" list, the right-click menu, the one-column box | new pin at the spot the item showed ("at 12.4 km") | the "+" (two columns), the "+ Pin" button (one column), the map (right-click) |
| tap/click on a list row (`.pin-row`) | that pin, filled in | that pin's row |
| tap/click on a map marker (`.pin-mk`), also in fullscreen | that pin, filled in | that pin's marker |

OK and Delete re-render rows and markers, so the opener is gone: focus the same pin's new row or marker (`[data-pin]`), after a delete the row now at that index, else the Pins heading, else the "+" (the rule `addedtable.ts` already uses). Always `preventScroll: true`; focus never lands on `body`.

The viewing page is untouched: the editor opens the dialog from the view's existing `onselect`, not from new code in `src/pins`. The only change there is the optional row hook `view.onrow` on `PinsView` (`src/web/src/pins/index.ts`), called per rendered row, through which the editor appends the × (rows are rebuilt on units and language changes, so the editor cannot add it once).

## Layout

```
two columns (>= 1024 px)                        phone (320-390 px)
+-------------- map ------------+ +-- right --+  +--------------------------+ 8 px
| (dimmed, selected marker      | | +-------+ |  | Edit pin                 |
|  outlined, stays visible)     | | |dialog | |  | 3.1-4.2 km               |
|                               | | |480 px | |  | Type   [Danger       v]  |
+-------------------------------+ | +-------+ |  | Title  [______________]  |
                                  +-----------+  | Note   [______________]  |
                                                 |        [______________]  |
                                                 | [Delete pin]  [OK][Cancel]|
                                                 +--------------------------+
```

- **>= 1024 px:** 480 px wide, anchored with `openDialog`'s `anchor` to the pins slot: right edges aligned, top at the slot's top, clamped into the viewport. It sits over the right column; the map stays visible, dimmed by the backdrop, with the selected marker outlined.
- **Below 1024 px (phones, tablets, short landscape):** full width minus 16 px as the shell, `max-width: 480px`, **top-aligned** (`margin: 8px auto auto`) so the fields stay above the on-screen keyboard. It covers the map; the place line says where the pin is. Taller than the viewport (short landscape with keyboard): the shell's own scrolling.
- **Fullscreen map:** the dialog must show over it: `html.wx-fs .pin-dlg { visibility: visible }` (the page body is hidden there).

## Content, top to bottom

1. **Heading** (shell `h2`, 18 px): "Add pin" (existing `addPin`) or "Edit pin" (new key `editPin`).
2. **Place line**, `.edit-small` (13 px, `--muted`): the "+" items' `where()` text ("at 12.4 km", "3.1–4.2 km", elapsed time without distance). For a dimmed pin its reason instead ("Removed on Save", "Hidden by the start radius").
3. **Fields** in one `<form method="dialog">`, the current form's fields and limits unchanged, each an `.edit-field` label, 8 px apart:
   - Type: `select` of the six types; changing it updates Title's placeholder to the type's name.
   - Title: `input`, `maxlength` 60, placeholder the type's name (empty title allowed, OQ-277).
   - Note: `textarea`, `maxlength` 500, 3 rows, `resize: vertical`.
4. **Actions** (`.dlg-actions`, 12 px above, flex, 8 px gap, wrapping): **Delete pin** (`.edit-button`, existing `deletePin`, only for an existing pin) on the left with `margin-right: auto`; on the right **OK** (`.edit-save`, `type="submit"`, new key `ok`) then **Cancel** (`.edit-button`, existing `cancel`), the edit page's order (primary first, as the delete confirmation). Delete acts at once and closes, as the list's × (no confirmation; nothing is saved before Save).

**Validation:** none beyond `maxlength`; OK is never disabled. Every check (spot, cap, twin, hidden part) already ran before "Add pin" was enabled, and the spot cannot change while the dialog is modal. Text rules (trim, no line breaks) stay where they are applied today.

## Keyboard and focus

- **On open:** new pin: Title (the keyboard opens on phones; typing is the next step). Existing pin: the first control, Type, which the native `showModal` focuses (on phones this opens no keyboard, so Delete stays in view).
- Tab stays inside (native modal). Enter in Title is OK (implicit submit); Enter in Note is a new line. Esc is Cancel. The pin dialog's Escape `keydown` calls `stopPropagation()` so the fullscreen map stays open (as `plus.ts` and `mapmenu.ts` do).
- Focus visible: the UA ring on fields, the edit buttons' focus as elsewhere on the page.

## Pins list (edit page only, `edit.css`)

- `.pin-item`: `display: flex; align-items: center; gap: 4px`; `.pin-row` takes `flex: 1`; the × is a sibling of the row button, never inside it.
- × reuses `.added-x-btn` (32 px square, 44 px on `pointer: coarse`, `--line` border, radius 4 px), name and title "Delete pin: <title or type name>". Focus after × as in `addedtable.ts`. Note: the list has no × today; this adds it.
- `@media (pointer: coarse)`: `.pin-row` at least 44 px tall.

## Sizes, themes, motion

- Touch (`pointer: coarse`): OK, Cancel, Delete pin, `select` and `input` at least 44 px tall; field text 16 px (15 px makes iOS zoom on focus). Fine pointer: as the edit panel (buttons about 32 px).
- Themes: only existing tokens (`--bg`, `--fg`, `--line`, `--muted`, `--accent`, `--on-accent`); fields follow the page's colour scheme as in the edit panel. Contrast is the edit panel's, already AA.
- Reduced motion: nothing moves; the old smooth scroll goes.

## Shell changes (`src/dialog`)

1. An optional `actions` argument: nodes that replace the Close button in `.dlg-actions`. The info dialogs are unchanged.
2. Backdrop close only when the `pointerdown` also hit the dialog element. Without this, a text selection dragged out of a field ends with a click on the backdrop and throws the entry away. This also fixes the info dialogs.

## No move (PIN-8, OQ-273 changed 23 Nov 2026)

There is no Move button and no range-moving of section pins: moving a pin is deleting it and adding it again, as for added laps (OQ-302).

## Budget

Owner chunk 19,701 / 20,000 gzip after the dialog (ADR-011 item 4). `src/dialog` stays its own shared chunk (735 gzip), not counted in `start.svelte`; if Rollup ever inlines it, F-19's rule applies (split, never raise).
