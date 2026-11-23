# F-66 Remove auto-laps

- **Status:** planned (user, 23 Nov 2026); listed last in the build order; open questions decided (user, 23 Nov 2026)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first: stored setting, laps rebuilt into the published data and the server-rendered laps table; frontend: the action on the edit page, laps mirrored over `draft.bin`)
- **Lane:** normal (merged lap values come from the public streams, as F-65; no coordinates stored)
- **Requirements:** AP-8; AP-7, AP-12, AP-13, AP-19, AP-20, AP-23, UP-4, UP-6, UP-7, AC-4, AC-5, SH-4, DS-6; NFR Performance, NFR Reliability
- **Depends on:** F-46 (trigger column, splits only without laps), F-47 (trim), F-48 (stroke), F-51 (comparison chart), F-54 (four languages), F-64 (per-leg tables), F-65 (added laps, ADR-060)

## Goal
A sharer whose device wrote a lap every kilometre (Apple Watch exports, device "development / Watch7,5", write one every 1,000 m with no trigger) removes all those automatic laps in one go, so the laps view shows only the laps that mean something, or the splits.

## What exists
- Every device lap stays a lap, whatever ended it; automatic laps are never filtered out (AP-8, F-46). The parser maps FIT `lap_trigger` to `button`, `time`, `distance`, `position` (all four position values merged), `stop` (session end), `equipment`, or "" (missing or unknown); added laps have `added` (F-65).
- A workout with distance but no device laps shows only the splits (AP-8). Device lap boundaries cannot be removed (F-65 OQ-302).
- Added lap boundaries are stored as instants and merged into the data file's laps before trim and hiding; the edit page mirrors the merge over `draft.bin` (ADR-060).

## Scope
- **Auto-lap:** a device lap whose trigger is `distance`, `time`, `position` or missing/unknown (OQ-313). `button`, `stop`, `equipment` and `added` laps are never removed.
- **Remove auto-laps** (edit page, before and after publishing): removes the end boundary of every auto-lap, so it merges with the next lap. Merged laps get values computed as for a range (AP-7) and the trigger of their last part. A lap boundary that is also a leg boundary (AP-12) stays (OQ-314). The end of the workout is not a boundary and stays.
- **Reversible** (OQ-306): one stored on/off setting; while on, the action reads "Restore auto-laps" and turning it off brings the device laps back. No confirmation (OQ-309).
- **Shown only when it applies** (OQ-308): only when the workout has at least one auto-lap boundary that can be removed, or the setting is on. Placement: one line under the laps table on the edit page, with the number of laps it removes (OQ-311).
- **All removed** (OQ-307): when no device boundary is left and there are no added laps, the workout counts as having no device laps: splits only, no laps view (AP-8). Without distance there is no laps and splits section (AP-8).
- **Added laps** (F-65) stay; they split the merged laps. "Add lap" is allowed on a removed boundary. On restore, an added boundary that falls on a restored device boundary is dropped on the next Save (OQ-312).
- **Shown like any lap change** (F-65): laps table (server-rendered, also without JavaScript), map lap markers, lap row taps, comparison chart (AP-23), per-leg tables (F-64). The edit page shows the change at once; the viewing page after Save, without re-uploading. Splits, lengths, summary, range links (AP-19, by time), preview image and GPX download (no laps) are unchanged (OQ-310).
- **Trim and hiding:** removal applies to the whole recording before trim and hiding, as added laps (ADR-060); a merged lap the trim cuts keeps its remaining part (UP-6). Inside a hidden start or end, merged laps show time and distance as today, no map marker.
- **Pool swims:** a merged lap's Stroke is the common stroke of its parts, otherwise "Mixed" (AP-13).
- **Privacy:** stores one setting, no coordinates; deleted with the workout (UP-7, AC-4). Merged lap values come from the public streams, so nothing new is revealed (ADR-060 item 3).
- **Texts:** "Remove auto-laps", "Restore auto-laps" and the count in English, German, Spanish and French (DS-6).
- **Bundle:** the action in the owner chunk; viewers load nothing new.

## Out of scope
- Removing single device laps or choosing trigger types. Removing `button`, `stop`, `equipment` or added laps. Changing splits. Any change to the original upload (SH-8 owner download stays the full file).

## Constraints
- **Budgets:** entry bundle and LCP unchanged within noise; data file within its budget; 60 fps scrubbing on `ride-12h`.
- **Reliability:** workouts with only auto-laps, one lap, laps ending in a gap, truncated files, multisport legs and transitions, pool swims and trimmed workouts must not break the laps table or the chart.

## Acceptance criteria
- [ ] An Apple Watch-style file (a lap every 1,000 m, no trigger) shows "Remove auto-laps" with its count on the edit page; choosing it leaves splits only (no laps view) on the edit page at once and on the viewing page after Save, JavaScript on and off.
- [ ] A file with `distance`, `time` or `position` laps mixed with `button` laps keeps only the `button` laps' end boundaries; merged laps show computed values and the trigger of their last part.
- [ ] "Restore auto-laps" brings back the device laps with their device values.
- [ ] The action is absent on a workout without auto-lap boundaries (only `button`, `stop`, `equipment` or no laps).
- [ ] Added laps (F-65) stay and split merged laps; "Add lap" works on a removed boundary; after a restore, an added boundary on a restored device boundary is gone after Save.
- [ ] Multisport: no merge across a leg boundary; per-leg tables (F-64) and the comparison chart follow the merged laps.
- [ ] Trim and hiding: merged laps follow the trim; no position is published for samples in a hidden start or end.
- [ ] Pool swim: merged laps show the common stroke or "Mixed".
- [ ] The server refuses the setting from a viewing link and accepts it from the edit link (AC-5).
- [ ] Test: after delete or expiry nothing of the setting remains (UP-7, AC-4).
- [ ] New texts in all four languages (DS-6); works in both themes, phone and desktop, touch and mouse.
- [ ] Viewers load no new code; entry bundle and LCP unchanged within noise; every corpus file renders.

## Decisions (user, 23 Nov 2026)
- OQ-306 to OQ-314: decided as suggested (see Open questions); Scope and acceptance criteria already follow them.

## Open questions
- [x] **OQ-306 Reversible or one-off.** Decided (user, 23 Nov 2026) as suggested: a reversible on/off setting stored as one value with the workout (smaller than a list of removed boundaries like `added_lap_boundaries`, and undoable). Alternative: one-off removal storing the removed boundaries. Affects storage, Scope "Reversible", AC 3.
- [x] **OQ-307 Splits when all laps are removed.** Decided (user, 23 Nov 2026) as suggested: fall back to splits only, as a workout without device laps (AP-8). Alternative: keep one lap over the whole workout. Affects AC 1.
- [x] **OQ-308 Show only when it applies.** Decided (user, 23 Nov 2026) as suggested: yes, only with at least one removable auto-lap boundary (or while on). Alternative: always shown, disabled with a reason. Affects AC 4.
- [x] **OQ-309 Confirm dialog.** Decided (user, 23 Nov 2026) as suggested: none, because it is reversible and the viewing page changes only on Save. Affects the UI.
- [x] **OQ-310 Viewing page and GPX.** Decided (user, 23 Nov 2026) as suggested: viewing page shows the merged laps after Save, like added laps; GPX has no laps and the preview image no lap markers, so both are unchanged. Affects Scope "Shown like any lap change".
- [x] **OQ-311 Placement.** Decided (user, 23 Nov 2026) as suggested: one line under the laps table on the edit page, "Remove auto-laps (12)". Alternative: an item in the "+" menu (F-65). Affects the UI.
- [x] **OQ-312 Added boundary on a restored device boundary.** Decided (user, 23 Nov 2026) as suggested: the device boundary wins; the added one is dropped on the next Save (ADR-060 already skips it in the build). Affects AC 5.
- [x] **OQ-313 Which triggers count as auto.** Decided (user, 23 Nov 2026) as suggested: `distance`, `time`, `position` and missing/unknown. `position` merges all four FIT position values (incl. marked position and waypoint) and is kept whole, no parser change. Alternative: leave missing/unknown out (would not solve the Apple Watch case). Affects Scope "Auto-lap", AC 1, AC 2.
- [x] **OQ-314 Multisport legs.** Decided (user, 23 Nov 2026) as suggested: a lap boundary at a leg boundary always stays, so no lap spans two legs (F-64). Affects AC 6.
