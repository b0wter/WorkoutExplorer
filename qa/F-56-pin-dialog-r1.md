# QA F-56 Pin dialog - round 1

- **Date:** 2026-11-23
- **Verdict:** ready with conditions - no Blocker or Major open; conditions are the manual browser checks below. Four Minor findings (two confirmed defects by probe: focus lands on `body` after a pin delete when added laps exist; a dialog open across a Save response deletes or edits the wrong pin).
- **Inputs:** `features/F-56-map-pins.md` (AC 1, 2, 4, 8; decisions of 23 Nov 2026), `features/README.md` decision log "Decided on 23 Nov 2026, pin dialog", product spec PIN-1, PIN-8, `architect/UI-pin-dialog.md`. Diff `git diff 484341f bdc2b66 -- src/` (merge bdc2b66 on main, clean tree).

## Build and test results

- `make ci` (foreground, exit 0): svelte-check clean; `go test` all packages ok; vitest **92 files, 2,380 tests passed**; vite build ok; size checks all PASS; docker-check ok; update_test 50 ok, sign_test 12 ok.
- Budgets (checker output): entry `edit.html`/`index.html` **60,465 / 62,000** gzip; owner chunk `start.svelte` **19,701** gzip (19,412 before, +289; ceiling 20,000, 299 left). `src/dialog` stays its own chunk (`dialog-*.js` 735 gzip), not inlined into the owner chunk.
- **Entry +4 bytes traced:** built `src/web` at 484341f and bdc2b66 in the scratchpad. The two `main-*.js` are both 144,503 raw and **byte-identical after normalising the 8-character content hashes** of the lazy chunk file names in the preload map (`m.f=[...]`, 34 names). The +4 gzip is hash entropy only (changed `dialog`, `pins`, `start.svelte` and their dependants' hashes). Nothing new in the viewing entry.
- Probes (scratchpad copy of bdc2b66 with two extra vitest cases, not in `src/`): see QA-56d-r1-1 and -2.

## Findings

### Minor

- **ID:** QA-56d-r1-1
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/edit/pins.ts:211-216` (`focusRow`); also `edit.css:642-656`
- **Relates to:** UI-pin-dialog "Openers" (focus never lands on `body`), PIN-8
- **Evidence:** the added-laps table (`addedtable.ts`) lives in the same pins slot and reuses `.pin-item` / `.pin-row` (as `div.pin-row.added-row`, plus a header row). `focusRow` takes `slot.querySelectorAll('.pin-row')`, so after deleting the last pin, or all pins, it picks an added-laps `div` that cannot take focus. Probe: pins a, b plus one added lap; × on b, then × on a: `document.activeElement` is `BODY` both times (expected: row a, then the Pins heading). Same path for the dialog's Delete. The new CSS `.pin-list > .pin-item > .pin-row { min-height: 44px }` (coarse) also hits the added-laps rows and their non-interactive header row (overrides `.added-row`'s 32 px).
- **Consequence:** keyboard and screen-reader focus is lost after a delete whenever added laps exist; on touch the added-laps header grows to 44 px. Fix: scope to the pins list (`[data-pin] > .pin-row` or `.pin-item[data-pin]`), in both the query and the CSS.

- **ID:** QA-56d-r1-2
- **Severity / kind:** Minor / defect (race)
- **Route:** frontend
- **Where:** `src/web/src/edit/pins.ts:251-262` (`close`, stale `k`), `:298-304` (`saved` replaces `list`)
- **Relates to:** PIN-8, AC 1 ("nothing changes until OK"), AC 36
- **Evidence:** `open` captures index `k` and object `p`; `saved()` replaces `list` with fresh objects. Save's request does not block taps on pin rows, so: Save, tap a row before the response, response arrives, then Delete or OK. Probe: pins a, b; open a; Save response without a (trimmed); Delete in the dialog → **b is deleted** (`list.splice(k)` on the new list). OK in the same situation assigns to the detached object: the edit is silently lost.
- **Consequence:** rare, but the wrong pin disappears (unsaved until the next Save, which then publishes the loss). Fix: in `close`, resolve by identity (`list.indexOf(p)`, bail when -1), or close the dialog without applying in `saved()`.

- **ID:** QA-56d-r1-3
- **Severity / kind:** Minor / defect (engineer's own note, confirmed by reading)
- **Route:** frontend
- **Where:** `src/web/src/edit/pins.ts:220-227`
- **Relates to:** PIN-1, PIN-8
- **Evidence:** `sess` is set before `await import('../dialog')`; a rejected import leaves `sess` non-null, the pin selected, and an unhandled rejection. Every later Add pin, row or marker tap returns at `if (sess) return`. Realistic after a deploy that removes the old hashed chunk while an edit page is open. Other lazy imports on the page `console.warn` and recover.
- **Consequence:** pins cannot be added or edited until reload, with no message; reloading drops other unsaved edits. Fix: `try/catch` around the import that clears `sess` and deselects.

- **ID:** QA-56d-r1-4
- **Severity / kind:** Minor / risk (documentation)
- **Route:** architect
- **Where:** `src/web/src/pins/index.ts:64-65, 297`; `architect/UI-pin-dialog.md`
- **Relates to:** UI-pin-dialog "Openers" ("optional hook in the existing `env.edit` seam"), ADR-013 seam
- **Evidence:** the engineer added `PinsView.onrow` (viewing chunk, +~30 bytes gzip to `pins-*.js`) instead of a hook in `env.edit`. Behaviour on the viewing page is unchanged (`rowFn` undefined there; `tap` toggle and the F-18 section deselect still gated by `!env.edit`), so it is acceptable, but the doc still says otherwise. The doc also still reads "Status: proposal" with open question 1 (moving) unresolved, although the decision log settled it (no move).
- **Consequence:** the doc contradicts the code. Update the doc (small lane).

## Previous findings

None for this change (first round of the pin dialog; F-56 map-pins r1-r3 are separate reports).

## Engineer's notes checked

1. `view.onrow` instead of an env field: acceptable, see QA-56d-r1-4.
2. Safari marker focus falls back to the row: Safari does not focus a clicked button, so `from` is `body` and focus goes to the list row. In the fullscreen map the page body is `visibility: hidden`, so that row cannot take focus and focus stays on `body`. Not reproducible in jsdom; manual check below. Not raised as a finding (mouse-on-Safari-in-fullscreen only; keyboard users do focus the marker).
3. Failed dynamic import: confirmed, QA-56d-r1-3.
4. jsdom stubs for `showModal`/`close`: patched on the prototype in `beforeEach` without restore, as in the howto, summary-info and report tests (per-file isolation keeps it contained). They do not model Esc → `cancel` → `close`, the Android back gesture, the top layer or real backdrop hit-testing; those need a browser (below).

## Shared dialog (`src/dialog/index.ts`)

- `actions` is optional; without it the Close button is built exactly as before. How-to and summary-info pass no `actions`: unchanged. Their dom tests pass (`howto.dom.test.ts` "closes with its button and a click on the backdrop, not inside" still passes because `down` defaults to the dialog and resets after every click).
- Backdrop: closes only when `pointerdown` and `click` both target the dialog element. In browsers every pointer click is preceded by `pointerdown`, so a plain backdrop tap still closes the info dialogs; a drag out of a field no longer does (tested for the pin dialog).
- Escape: the pin dialog stops `keydown` propagation, so the map's document-level Esc handler (`map-view.ts:496`) does not leave fullscreen (tested). The info dialogs do not need it (not opened in fullscreen).

## Viewing page

`src/pins/index.ts` changes are the `onrow` hook, a re-export of `pinName` and a comment. On the viewing page `rowFn` is undefined, the note under a selected row, the tap toggle and the section deselect are unchanged. Entry identical modulo hashes. Viewing pins tests pass.

## Questions

None that block. For the architect: is focus on `body` acceptable for Safari mouse users in the fullscreen map (note 2)? Answer "yes, accept" or "focus the map container instead".

## Manual browser checks (conditions)

1. Chrome and Safari, desktop: Add pin from the right-click menu and from "+", OK/Cancel/Esc/backdrop; dialog anchored over the right column at >= 1024 px with the map visible and the marker outlined; focus after close (the "+", the map, the row, the marker).
2. Fullscreen map: the dialog shows over it (`html.wx-fs .pin-dlg { visibility: visible }`); Esc closes only the dialog; marker tap in fullscreen on Safari (focus, note 2).
3. Android Chrome: the back gesture closes the dialog without a pin and without leaving fullscreen or the page.
4. iPhone: dialog top-aligned, fields stay above the keyboard; no zoom on focus (16 px fields); buttons and rows at least 44 px; short landscape with the keyboard scrolls inside the dialog.
5. Text selection dragged out of Title onto the backdrop does not close; a plain backdrop tap does (pin dialog and How to use / "How these numbers are computed").
6. Both themes: dialog, fields, × buttons; added-laps table header height on touch (QA-56d-r1-1).

## Coverage

| Criterion | Code | Test |
| --- | --- | --- |
| AC 1: Add pin opens the dialog; created only on OK; Cancel/Esc/back/backdrop leave none | `pins.ts` `placePin`, `open`, `close` | "PN-12: … Add pin opens the dialog on Title, OK makes the pin, Cancel makes none"; "dimmed reasons … " (nothing until OK). Esc/back simulated as `cancel`+`close` only; real back gesture manual |
| AC 1: row or marker opens the same dialog, filled in | `view.onselect` → `open(k)` | "edit via a row …", "edit via a map marker …" |
| AC 1: list read-only (icon, title, distance or dim reason, ×); delete by × or Delete | `view.onrow` (note removed, × added) | "Delete in the dialog and the row × …"; read-only note asserted |
| AC 2: section over a range, no moving of ends | `spot`, removed `toSection`/Move | "Add pin over a range makes a section once OK …" |
| AC 4: dimmed pins still editable and deletable | dim reason as place line | Not tested through the dialog (only row texts); Minor gap, not raised separately |
| AC 8 / PIN-8: edits and deletes reach the public page after Save | `changed` → `patch` | Patches asserted in the row/delete tests; server side unchanged from F-56 r3 |
| PIN-1 dialog in both layouts | `openDialog(..., slot, ...)`, `edit.css` | Layout and anchoring manual only |
| Decision 23 Nov: no Move | Move, `movePin`, `donePin`, `pinRefused` removed in en, de, es, fr | Move tests removed; i18n key parity checked by the existing tests |

Not run: real browsers (jsdom only), Lighthouse, 60 fps scrubbing (no change on the hot path: the cursor subscription now only refreshes "+").

Probe files: `/tmp/claude-1000/-home-b0wter-syncthing-work-workout-explorer/1973ef5e-b065-4f5d-9058-f861d81899dd/scratchpad/pd-bdc2b66/src/web/src/edit/edit.dom.test.ts` (cases named `PROBE`).
