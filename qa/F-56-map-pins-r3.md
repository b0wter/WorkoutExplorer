# QA F-56 Map pins - round 3

- **Date:** 2026-11-21
- **Verdict:** ready with conditions. No path creates or moves a pin into a hidden or trimmed part: probes show the button state follows the cursor, range, radius and trim, and the click checks again before it places. Two conditions remain. (1) AC 33 still says "Cancel or Esc in the form leaves without a pin". The build has neither, and PN-12 and the commit dropped them, so the user must decide (r3-1). (2) The round-1 hand checks still apply.
- **Inputs:** commit `01d0647` only (follow-up round, critical lane: hidden start and end refusal). `features/F-56-map-pins.md` AC 33 to 36 (changed 21 Nov 2026), OQ-272 as changed. `architect/F-56-map-pins.md` PN-12 (and PN-10 for the hot path). Spec PIN-1, PIN-4, PIN-7. `qa/F-56-map-pins-r2.md`. Clean working tree at `01d0647`.

## Build and test results

- `make ci` at HEAD: **exit 0**.
  - Go: every package ok. `TestNoRequestMetadataStored` passed and the flake did not recur.
  - Vitest: 90 files, **2,287 tests passed**.
  - svelte-check is clean.
  - Size: `entry edit.html: 60341 / 62000 bytes gzip PASS`, and the same for `index.html`. The lazy `pins-*.js` is 3,054 gzip.
  - The working tree is still clean after the run. `build/` was not touched beyond what `make ci` itself does.
- **Mutants:** run on a scratch copy of `src/web` against `edit.dom.test.ts`, `src/pins` and `src/lib/pins` (169 tests). The "probes" column shows the result against the five probe tests described below.

| Mutant | Change (`src/web/src/edit/pins.ts` unless noted) | Repo tests | Probes |
| --- | --- | --- | --- |
| A1 | `add.disabled = false` | killed | - |
| A2 | `place` ignores the shade | killed | killed |
| A3 | `spot` checks only the range start for `< 0` | survived, **equivalent** (a refused end gives -1, so the `s[0] >= s[1]` check catches it) | - |
| A4 | `spot` allows `from >= to` (a range that snaps to one sample) | survived | - |
| A5 | `cursor.subscribe(enable)` removed | killed | - |
| A6 | `refresh` no longer calls `controls()` when no dim reason changes | **survived** | killed (P1, P2, P4, P5) |
| A7+A8 | Move ignores the shade, both in its disabled state and in its click handler | **survived** | killed (P3) |
| A9 | `spot` ignores the selection | killed | - |
| A10 | `toSection` checks only `sa < 0` | survived, **equivalent** (`sb = -1` makes `sa >= sb` true) | - |
| A12 | the selection subscription no longer calls `controls()` | survived | survived. Safe: in the harness the cursor also moves, and the click calls `spot()` again, so a stale enabled state cannot place a pin |
| A13 | `start.svelte.ts` onhide: `pinEditor?.refresh()` removed | **survived** | killed (P1, P4) |
| A14 | `start.svelte.ts` ontrim: the refresh removed | **survived** | killed (P2, P5) |
| A15 | `toSection` refusal without `enable()` | killed | - |

- **Probes** (appended to a scratch copy of the F-56 describe block in `edit.dom.test.ts`). They pass on HEAD unless marked:
  - P1: with the cursor at sample 100, raising the start radius to 2000 m disables Add pin, and a click adds nothing. Lowering it to 300 m enables the button again.
  - P2: with the cursor at sample 2000, moving the trim start to 2500 disables the button, and a click adds nothing.
  - P3: a point pin is selected and the cursor is in the hidden start. Move is disabled, the click moves nothing, and Save sends no `pins`.
  - P4: with a range of 100 to 3000, raising the start radius hides the range start, and the button is disabled.
  - P5: with a range of 1000 to 3000, moving the trim end to 2000 (inside the range) disables the button.
  - P6 and P7 **fail on HEAD** and show the r3-1 gap. Esc in the form of a pin just added leaves the pin in place, and the form has only Move, Delete pin and Done.
  - P8: Add pin while a pin is selected adds a second pin at the same place (r3-4).

## Findings

No Blocker or Major finding.

### Minor

- **ID:** QA-56-r3-1
- **Severity / kind:** Minor / defect (AC text and build disagree)
- **Route:** main session, then requirements (user decision) or frontend
- **Where:** `features/F-56-map-pins.md:33`; `src/web/src/edit/pins.ts:85-88` (form: Move, Delete pin, Done)
- **Relates to:** AC 33, OQ-272 (changed 21 Nov)
- **Evidence:**
  - AC 33 says "Cancel or Esc in the form leaves without a pin".
  - The code has no Cancel button and no Escape handler (`grep Escape src/web/src/edit src/web/src/pins` finds nothing outside tests).
  - PN-12 no longer mentions Cancel or Esc, and the commit message says they were removed.
  - Probes P6 and P7 fail.
- **Consequence:** one AC clause has no code and no test. An owner who taps Add pin by mistake must use "Delete pin". There is no privacy effect.

- **ID:** QA-56-r3-2
- **Severity / kind:** Minor / defect
- **Route:** frontend
- **Where:** `src/web/src/edit/pins.ts:93`, `:128`
- **Relates to:** PN-12 ("disabled with a hint"), AC 35 ("refused with a one-line message")
- **Evidence:** The only hint is the button's `title`. It says "Select a point on the map or a chart first." in every disabled case, including when a point or range is selected but lies in a hidden or trimmed part. `title` tooltips do not show on touch devices. For point pins, AC 35's one-line message now exists only as a disabled button. The `pinRefused` line still shows for a section's new range.
- **Consequence:** on a phone, the owner sees a greyed button with no reason. On desktop, the reason given is wrong in exactly the privacy case. The fix fits in the existing `msg` line, which is already in the row, so the layout stays still.

- **ID:** QA-56-r3-3
- **Severity / kind:** Minor / test gap (critical lane)
- **Route:** frontend
- **Where:** `src/web/src/edit/edit.dom.test.ts` (the F-56 describe block)
- **Relates to:** AC 33, AC 35 ("moving a pin … into a hidden or trimmed part is still refused"), PIN-7
- **Evidence:**
  - Mutants A6, A13 and A14 survive the repo tests: the button state does not follow a radius or trim change.
  - A7+A8 survives: Move ignores the shade.
  - The test "Move puts a point pin at the cursor; disabled without a usable cursor" never asserts a disabled state.
  - There is no test with the cursor or range in the trimmed part.
  - A4 (a range that snaps to one sample) is untested.
  - Probes P1 to P5 cover all of these except A4, and could be adopted almost as they are.
- **Consequence:** the client guard against moving a pin into a hidden part is correct today but unguarded by tests. The server's moved-or-new check (`internal/lifecycle/pins.go:201`, tested in r1 and r2) is the backstop, which is why this is not Major.

- **ID:** QA-56-r3-4
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/web/src/edit/pins.ts:203-211`
- **Relates to:** PN-10, PN-12
- **Evidence:** Selecting a pin sets the cursor to it (a point) or selects its range (a section), so Add pin is enabled. A click then adds an exact duplicate (P8).
- **Consequence:** a likely slip that leaves overlapping twin pins. Disabling Add pin while a pin is selected, or placing it only when the spot differs, would avoid it.

- **ID:** QA-56-r3-5
- **Severity / kind:** Minor / documentation
- **Route:** main session
- **Where:** `features/README.md:816`
- **Relates to:** "one home per fact"
- **Evidence:** The 21 Nov decision entry says "Not yet updated: `architect/F-56-map-pins.md` PN-12". PN-12 was updated in the same commit.
- **Consequence:** a stale to-do in the decision log.

## Previous findings

Round 2 closed with no open findings. r1-7 (60 fps with 50 pins on ride-12h) stays a hand check. Since this diff, the edit page runs `enable()` on each cursor flush (rAF-coalesced; two or three `snap`/`dimReason` calls). This is O(1) on a positioned sample and O(gap) on an indoor stretch, and it does not touch the viewing page that PN-10's 60 fps rule is about.

## Questions

1. **AC 33 "Cancel or Esc in the form leaves without a pin" (r3-1).** Suggestion: strike the clause, since "Delete pin" in the form does the same. This closes r3-1 without a code round. The alternative is that Esc, and a Cancel button, remove a pin that has just been added and not yet edited.

## Coverage

- **AC 33:**
  - Disabled without a cursor: repo test and A1.
  - Disabled with the cursor in the hidden start: repo test and A2.
  - Disabled with the cursor or range in a trimmed part, and after a radius change: probes only (P1, P2, P4, P5). Code is correct.
  - The pin lands on the snapped visible sample and the form opens: repo test.
  - Cancel or Esc: **not implemented** (r3-1).
  - A tap only moves the cursor: the document click listener and the Escape listener were removed. `grep` finds no leftover.
- **AC 34:**
  - Add pin over a range makes a section: repo test and A9.
  - A new range moves its ends, and a hidden end is refused with the message and leaves the pin unchanged: repo test and A15.
  - "Any part of the range": the code checks the two ends only. This is equivalent because the public part is one contiguous window `[max(a, from), min(b, to)]` (`dimReason`, `Shade`). A range cannot cross a hidden zone and keep both ends public. P5 confirms this with a trim end inside the range.
- **AC 35:**
  - Client refusal: as above, with the r3-2 wording caveat.
  - Move into a hidden part: disabled, and the click checks again (P3; repo gap r3-3).
  - Radius raised after placing: the pin dims and is kept (OQ-236, unchanged).
  - The server check is unchanged by this diff and was not re-run beyond `make ci`.
- **Layout:**
  - Read from the code: on cursor, range, radius or trim changes, `disabled` and `title` change and nothing is added or removed.
  - `add.hidden` toggles only on `MAX_PINS` and the visible-route mirror, both unchanged from r2.
  - The hint span and the Cancel button that appeared on arming are gone.
  - The `msg` span sits in the tools row, which is below the map in one column. Its text changes only on a section refusal or after Save, both unchanged from r2.
  - Not checked in a browser.
- **i18n:**
  - `addPinHint` is in en, de, es and fr, and is set again on a language change (`label()`).
  - `placePin`, `cancelPin` and `pinSection` are gone from the code, the tests and all three language files.
  - The how-to and About texts have no "tap to place" wording. The `pinsTouch` line is about viewing and still holds.
- **Docs:**
  - PIN-1, the F-56 Scope and AC 33 and 34, the decision log and PN-12 agree with the build, except for r3-1 and r3-5.
- **Not run:**
  - Browser and phone hand checks (placing at the cursor on the map and charts, the disabled look in both themes, no map resize on the reference phone).
  - `make build` (as instructed).
