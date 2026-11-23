# F-65 Owner-added laps

- **Status:** planned (user, 22 Nov 2026, "for the future"); no build-order position; open questions decided (user, 22 Nov 2026)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first: stored lap markers, owner API checks, laps rebuilt into the published data and the server-rendered laps table; frontend: the right-click and "+" menus and the one-column box, markers on the edit page)
- **Lane:** normal (hidden and trimmed parts checked as for pins; no coordinates stored)
- **Requirements:** AP-8, PIN-1; AP-7, AP-12, AP-13, AP-19, AP-23, UP-4, UP-6, UP-7, AC-4, AC-5, SH-4, SH-10, DS-6, INF-1; NFR Performance, NFR Reliability
- **Depends on:** F-18 (laps table), F-46 (trigger column, splits only without laps), F-47 (trim), F-48 (lengths, stroke), F-51 (comparison chart), F-54 (four languages), F-55 (radius per end), F-56 (Add pin, edit page Save); F-64 if built before it

## Goal
A sharer who forgot to press the lap button (or wants intervals the watch did not mark) adds lap boundaries after uploading, and viewers see the laps as if the device had recorded them.

## What exists
- Laps come only from the device: every device lap stays a lap (AP-8, F-46), with a Trigger column. A workout with distance but no device laps shows only the splits (no laps view).
- Laps already include samples in a hidden start or end (SH-4): only positions are removed there, and a lap marker on the map is drawn only where its first sample has a position (F-06). After a trim the laps follow the remaining part (UP-6, F-47).
- On the edit page, "Add pin" places a pin at the cursor or over a selected range; it is docked bottom-left on the map in the two-column layout and below the map otherwise; disabled without a selection or in hidden or trimmed parts (PIN-1, PIN-7, F-56). Changes reach the public page on Save (PIN-8).

## Scope
- **"+" button** (changed 23 Nov 2026): on the edit page, before and after publishing, in the two-column layout only, "Add pin" becomes a "+" button on the map in the same place; its items have no box around them and the menu opens with a short animation (none under reduced motion). Changed again 23 Nov 2026 (UI review): the "+" is a control in the map's bottom-left corner beside the pin legend (never overlapping), shown only on touch (coarse-pointer) devices; with a mouse it is hidden and a right-click on the map opens a themed menu with the same items, also in fullscreen. In the one-column layout there is no "+": a full-width muted box below the map has two rows, "[+ Lap] reason" and "[+ Pin] reason"; with nothing selected it shows "Select a point or range first". In every menu and the box, an enabled item's reason slot shows where it acts ("at 12.4 km", "12.4–13.1 km" for a range, elapsed time without distance). Add controls and the lap row × are at least 44 px on touch devices. Disabled without a cursor point or range, as "Add pin" today. A click or tap opens the menu with "Add lap" and "Add pin"; choosing an item closes the menu. "Add pin" keeps F-56's rules and reasons unchanged; it is not in the menu without a visible route (OQ-275). Without a visible route (indoor, pool, fully hidden) there is no map and no two-column map area: a plain "Add lap" sits below the no-location hint (AP-3) (OQ-305, changed 23 Nov 2026). Esc, a click outside or tapping "+" again closes the menu without change. Keyboard: Enter or Space opens it, Tab moves between the items, Esc closes it and returns focus to "+".
- **Add lap:** places a lap boundary (lap marker) at the cursor; with a range selected, at both ends of the range, so the range becomes a lap; an end already on a boundary adds nothing (OQ-304). The boundary splits the lap it falls in (OQ-301). Disabled, with a one-line reason as for pins, when the cursor lies in a part the trim removes, on an existing lap boundary ("There is a lap boundary here already."), or when the workout has 100 markers. Allowed inside a hidden start or end (see Privacy).
- **Merged laps** (OQ-301): every device lap stays a lap; a marker splits it into two laps. Laps are numbered in time order, so a new marker renumbers the later laps. A lap that is not split keeps its device values; both parts of a split lap get values computed as for a range (AP-7). The first part's trigger is "Added" (OQ-303); the second part keeps the device lap's trigger (the trigger names what ended the lap).
- **Without device laps:** the first marker gives the workout two laps, so the laps view appears and the page opens on it (AP-8 rule); deleting the last marker returns to splits only.
- **Editing** (OQ-302): only added markers can be deleted, by an × on their row in the edit page's laps table; device lap boundaries stay fixed; moving a marker is delete and add again. Changes are made on the edit page and reach the public page on Save, without re-uploading (as PIN-8); the edit page shows them at once.
- **Shown like device laps:** laps table (server-rendered, so also without JavaScript), the map's lap markers (only where the boundary sample has a position), lap row taps (AP-7), the comparison chart (AP-23), per-leg tables (F-64, a lap belongs to the leg of its first sample). Splits, lengths, the summary, range links (AP-19, by time, so renumbering does not break them) and the preview image are unchanged.
- **Pool swims:** in a lap-swimming session a marker snaps to the nearest length end; each part's Stroke cell keeps the device lap's stroke (AP-13).
- **Trim (UP-6):** markers in a part the trim removes are deleted on Save, as pins (OQ-237); adding one there is refused.
- **Privacy:** a marker is stored as a sample instant only, never a coordinate. Inside a hidden start or end it is allowed: like a device lap there, it shows time and distance values but no map marker, because those samples have no position. Nothing new is shown that a range selection does not already show. Stored with the workout for its lifetime and deleted with it (UP-7, AC-4); the About page lists lap markers among the stored data (INF-1).
- **Texts:** "+", "Add lap", the trigger word, reasons and delete control in English, German, Spanish and French (DS-6).
- **Bundle:** the "+" menu and marker editing in the owner chunk; viewers load nothing new.

## Out of scope
- Laps added by viewers. Naming or typing laps (e.g. "warm-up"). Moving or deleting device laps (OQ-302). Lap markers in the preview image or the GPX download (SH-13). Undoing a trim restoring deleted markers.

## Constraints
- **Budgets:** entry bundle and LCP unchanged within noise; 60 fps scrubbing on `ride-12h` with 100 markers; data file within its budget.
- **Reliability:** markers on workouts with one sample per lap, laps that end in a gap, truncated files, multisport legs and transitions must not break the laps table or the chart.

## Acceptance criteria
- [ ] On the edit page in the two-column layout, "Add pin" is replaced by "+" on the map in the same place; "+" is disabled without a cursor point or range; it opens a menu with "Add lap" and "Add pin", items without a box, with a short open animation (none under reduced motion); Esc, a click outside or "+" again closes it; Enter/Space, Tab and Esc work as in Scope.
- [ ] The "+" is shown only on a coarse pointer, in the map's bottom-left corner without covering the pin legend; with a mouse a right-click on the map opens the same items in a themed menu, also in fullscreen.
- [ ] In the one-column layout there is no "+": a full-width box below the map has the rows "[+ Lap] reason" and "[+ Pin] reason", showing "Select a point or range first" with nothing selected; without a visible route only "Add lap" sits below the no-location hint.
- [ ] An enabled item shows where it acts ("at 12.4 km"); add controls and the lap row × are at least 44 px on touch devices.
- [ ] "Add pin" from the menu behaves exactly as F-56's button (all F-56 acceptance criteria still pass).
- [ ] "Add lap" at a cursor inside a device lap splits it: the laps view shows one more lap, later laps renumbered, both parts with computed values, the first part's trigger "Added", the second the device trigger; the other laps keep their device values.
- [ ] With a range selected, "Add lap" adds boundaries at both ends, so the range becomes one lap; an end already on a boundary adds nothing.
- [ ] "Add lap" is disabled with its reason on an existing boundary, in a trimmed part and at 100 markers; the server refuses the same cases when sent directly to the owner API.
- [ ] A marker inside a hidden start or end is accepted; the lap appears in the table, no map marker is drawn there, and the public data file and page hold no position for it.
- [ ] A workout without device laps shows only splits until the first marker; after Save the viewing page opens on the laps view with two laps; deleting the last marker returns it to splits only.
- [ ] Changes show on the edit page at once and on the viewing page after Save, with JavaScript on and off, without re-uploading.
- [ ] An added marker is removed by the × on its row in the edit page's laps table; device lap rows have no ×, and the server refuses removing a device boundary.
- [ ] Markers in a part the trim removes are deleted on Save; the comparison chart, lap row taps, map lap markers and (if built) F-64 per-leg tables include the added laps.
- [ ] Pool swim: a marker snaps to the nearest length end; both parts show the device lap's stroke.
- [ ] Test: after delete or expiry nothing of the markers remains (UP-7, AC-4).
- [ ] New texts in all four languages (DS-6); works in both themes, phone and desktop, portrait and landscape, touch and mouse.
- [ ] Viewers load no new code; entry bundle and LCP unchanged within noise; 60 fps scrubbing on `ride-12h` with 100 markers; every corpus file renders.

## Decisions (user, 22 Nov 2026)
- OQ-301 to OQ-305: as suggested (see Open questions).
- 23 Nov 2026: "+" only in the two-column layout, on the map, boxless items, short open animation; one column shows a full-width box below the map with two rows, "[+ Lap] reason" and "[+ Pin] reason" (see Scope).
- 23 Nov 2026 (UI review, [architect/UI-edit-page-add-controls.md](../architect/UI-edit-page-add-controls.md)): "+" on touch only, right-click menu with a mouse (also in fullscreen), where-it-acts reasons, "Select a point or range first" in one column, 44 px touch targets (see Scope).

## Open questions
- [x] **OQ-301 Merge or replace.** Decided (user, 22 Nov 2026) as suggested: merge: device laps all stay, a marker splits the lap it falls in. Alternative: once the owner adds a marker, the owner's laps replace the device laps entirely. Affects AP-8, Scope "Merged laps", AC 3.
- [x] **OQ-302 What can be changed.** Decided (user, 22 Nov 2026) as suggested: added markers can be deleted (an × on their row in the edit page's laps table); moving = delete and add again; device lap boundaries stay fixed. Alternative: device boundaries can also be removed (merging two laps), which stores a list of removed boundaries. Affects AC 9 and the stored data.
- [x] **OQ-303 Trigger word for added laps.** Decided (user, 22 Nov 2026) as suggested: "Added". Alternative: "Manual" (close to today's "Button (manual)" and easily confused) or none (empty cell). Affects AP-8's trigger list and AC 3.
- [x] **OQ-304 Add lap with a range selected.** Decided (user, 22 Nov 2026) as suggested: boundaries at both ends of the range, so the range becomes a lap (an end already on a boundary adds nothing). Alternative: "Add lap" only at the cursor, disabled with a range. Affects AC 4.
- [x] **OQ-305 Placement without a visible route.** Indoor, pool or fully hidden workouts have no map and no "Add pin" (OQ-275). Decided (user, 22 Nov 2026) as suggested: "+" sits below the no-location hint (AP-3), where it sits below the map otherwise, with only "Add lap" in its menu. Changed 23 Nov 2026 (one column has no "+"): a plain "Add lap" sits below the no-location hint. Alternative: a plain "Add lap" button there, no menu. Affects AC 1.
