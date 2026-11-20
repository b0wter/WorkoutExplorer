# QA F-56 Map pins - round 2

- **Date:** 2026-11-20
- **Verdict:** ready. The Major QA-56-r1-1 is fixed: a new test catches M5. Minors r1-3 to r1-6 are fixed, and each mutant that survived round 1 is now caught. r1-2 is accepted at +257. r1-7 stays a hand check, as decided.
- **Inputs:** `qa/F-56-map-pins-r1.md`; the diff `08dec44..98deb62` (commits `27cdb94` backend tests, `98deb62` frontend fixes and ADR-011); the decisions passed on by the main session: r1-1 the shortcut's placement stays, now with its test; r1-2 +257 accepted from the reserve; r1-7 stays a hand check. Clean working tree at `98deb62`.

## Build and test results

- `make ci` at HEAD: **exit 0**.
  - Go: every package ok (`publish` 67 s).
  - Vitest: 79 files, **2,028 tests passed** (+1).
  - svelte-check is clean.
  - Size: `entry index.html: 57862 / 60000 bytes gzip PASS`, and the same for `edit.html`. That is +4 against the 57,858 recorded. The diff adds no static code: the `pins/index.ts` line is in the lazy `pins-*.js` (2,968 gzip). The +4 is within the ±4 that ADR-011 already records for chunk names in the preload map.
  - caddy-check, docker-check, the update harness and sign 12/12 ok.
- **Mutants**, run one at a time on scratch copies of HEAD:

| Mutant | Change | Result | Killed by |
| --- | --- | --- | --- |
| M5 | `!sameRecord(b2, after)` removed from `samePublicPins` | killed | `TestPinsEditWithHidingChangeRebuildsRemoveFirst` |
| L4 | `sameAnchor` compares the start only | killed | `TestPinsSectionEndMovedIntoHiddenPart` |
| M13 | `ei <= si` changed to `ei < si` in `Merged` | killed | `TestPinsMergedSection` |
| F7 | the section end in `toSection` taken raw (`b`) | killed | edit.dom "a section from the range … (AC 34)" |
| F7b | the section end snapped but not checked | killed | the same test |
| r1-3 | `maxPatchBytes = 128 << 10` | killed | `TestPatchPins` |

## Findings

None at Blocker, Major or Minor level from this diff.

Checked and found correct:

- **F8 removal (`edit/pins.ts:176`, the `dimReason({from: sa, to: sb})` line):** the mutant is equivalent.
  - `place()` runs `dimReason({from: s})` on each end and refuses the end when it gets any reason.
  - `dimReason` (`lib/pins/index.ts:120`) takes a section's ends as a set and judges each end on its own, with `some`/`every` over the ends. Nothing in it depends on both ends together.
  - So once both `place` calls pass, the section check cannot fail, and deleting the line is safe.
- **Min-height release (`pins/index.ts:282`):** the reserve in `page/start.ts:134` is `count * 32 + 40`. In `pins.css`, rows and the heading are a fixed 32 px and the list has an 8 px bottom margin. The legend state lives in memory only (OQ-239), so at load every row shows and the list is exactly as tall as the reserve. Releasing the reserve shifts no layout at load.
  - The release runs only on the viewing page after a successful load. The edit page keeps its reserve. When the load fails, the slot is hidden as before.
  - The r1 hand-check item "hiding a type leaves empty space" no longer applies: the list now shrinks when a type is hidden, which is the intended behaviour.
- **r1-1 test:** it covers a radius change and a trim change, each in the same Save as a pin retitle. It asserts remove-first, a new data file, and that neither title appears in `public/`. M5 fails it.
- **r1-4 test:** `setTexts` now stores a pin with marker title and note and confirms through `GET` that it was stored. The delete and expiry byte scans (`dataLeft(..., texts...)`) therefore search for the pin text. The pre-delete check at `expiry_test.go:248` still proves that the scan finds bytes while the row exists.

## Previous findings

| ID | Status |
| --- | --- |
| QA-56-r1-1 (Major) | **Fixed.** Test added, M5 killed. Placement kept as decided (Question 2 of r1 answered: keep it). |
| QA-56-r1-2 | **Fixed.** ADR-011 records +257 against the `9ec6db3` baseline, 7 over the ceiling and accepted from the reserve. The reserve of 2,142 is unchanged. |
| QA-56-r1-3 | **Fixed.** The body is built from `😀` escapes, and a 128 KiB cap now fails. |
| QA-56-r1-4 | **Fixed.** The delete and expiry byte scans search for the pin title and note. |
| QA-56-r1-5 | **Fixed.** L4 and M13 are killed. |
| QA-56-r1-6 | **Fixed.** The editor section test kills F7. F8 is equivalent (see above) and its line is removed. |
| QA-56-r1-7 | **Open by decision:** a hand check (60 fps with 50 pins on `ride-12h`). |

## Questions

None.

## Coverage

- Round 1's coverage table stands. This round changes it as follows:
  - AC 34 now has an editor test.
  - AC 36 and AC 40 have the combined-change rebuild test.
  - AC 44 has a database byte scan for pin text.
  - AC 35 has the `sameAnchor` end test.
- Not run: browser and device checks, and the scrub gate. The hand checks listed in round 1 remain for the user, apart from the empty-space item.
