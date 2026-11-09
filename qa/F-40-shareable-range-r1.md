# QA F-40 Shareable range - round 1

- **Date:** 2026-11-10
- **Verdict:** ready. No Blocker or Major finding. There are two Minors: the × focus call can scroll the page, and AC 6 (hidden sections) has no test of its own.
- **Inputs:** `features/F-40-shareable-range.md`, `architect/F-40-shareable-range.md` (SR-1 to SR-8), ADR-020 rev. 3 item 11, ADR-011 rev. 14, ADR-038 item 3, `qa/F-39-how-to-use-r1.md`. Commit reviewed: `658f524` (HEAD on main, clean tree).

## Build and test results

- `make ci` (lint, test, build, size, caddy-check, docker-check): exit 0.
- Go: every package `ok`. Vitest: 65 files, 1,663 tests passed.
- Size: `entry index.html: 58855 / 60000 bytes gzip PASS`. The entry grew by 510 bytes (58,345 before). The F-40 share is 700. ADR-011 records the same figure, and the reserve of 1,145 is correct.

## Findings

### Minor

- **ID:** QA-40-r1-1
- **Severity / kind:** Minor / risk (not reproduced in a browser)
- **Route:** frontend
- **Where:** `src/web/src/components/charts/ChartStack.svelte:179` (`stack.focus()`)
- **Relates to:** QA-39-r1-2 fix, F-39 "no jump when the hint goes"
- **Evidence:** `focus()` is called without `{ preventScroll: true }`. The chart stack is usually taller than a phone viewport, and the × sits at its bottom. Browsers scroll a focused element into view when it is not fully visible, so tapping × can move the page towards the top of the stack. jsdom does not scroll, so the test cannot catch this. A second effect of `tabindex="-1"`: a click or tap anywhere in the charts now moves focus to the stack div.
- **Consequence:** The page may jump when the hint is dismissed, and keyboard users may see a focus ring around the whole stack. Fix: `stack.focus({ preventScroll: true })`. Then check on a phone (hand check 3).

- **ID:** QA-40-r1-2
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/page/page.dom.test.ts` (range link block)
- **Relates to:** AC 6 (hidden sections), AC 1 (map highlight)
- **Evidence:** The page tests check the store through the ChartStack stub only. No test opens a link range inside a hidden start or end section, and none checks that the map receives the range. By reading the code, both work: the map and charts share the one store, which is set before they mount, exactly as a manual selection sets it (SR-2). The lib test also covers offsets that do not start at 0.
- **Consequence:** This is low risk now. A later change that gives the map its own selection path would not be caught.

## Engineer's deviations

| # | Judgement |
| --- | --- |
| 1 | Accepted. The button spans 3 grid tracks, wraps, and is 32 px tall. The feature allows wrapping on a narrow panel. The wording is long and `nowrap` is set, so on a 360 px phone the button may overflow its 3 tracks. Not checkable in jsdom; see hand check 2. |
| 2 | Accepted. The exemptions in the source scans remove exact strings only. If the code changes, the `not.toMatch` check fails, so the tests fail closed. |
| 3 | Accepted. Showing the fallback field only when `u === url()` and a range exists means it never shows a stale link (SR-7). Any store change removes the field (tested). |
| 4 | Accepted. The panel's normal mount point is the first chart paint (F-17 R-9). This keeps the LCP unchanged. |
| 5 | Accepted with QA-40-r1-1. |

## Checks against the requested points

- **Hostile fragments:** one anchored regex, `^#range=(\d{1,7})-(\d{1,7})$`. There is no decoding, and `+` is applied to the digit groups only. The tests cover 27 hostile or invalid inputs, including a percent-escape, a trailing newline, `&`, a doubled fragment, upper case, `1e3`, `0x1`, a token-like fragment, a reversed range, a zero-length range, a range entirely outside the data, a range inside a pause, a one-sample range and an empty workout.
- **Edit and published pages:** `hash = edit ? '' : win.location.hash`, and `replaceState` is guarded by `if (hash)`. As a result, the edit seam never reads or writes the fragment. The published page does not call `startPage`. Tested: the edit seam selects nothing, keeps its fragment after a selection, and shows no button. The E-2 source scan still forbids every other `.hash` and `replaceState`.
- **replaceState:** it passes `win.history.state` and `pathname + search`. Tested: the query and `{fs:1}` are kept and `history.length` does not change. Fullscreen's own `pushState(null, '')` is untouched.
- **Hint:** the link's range is filtered out by comparing it with `(la, lb)` until the viewer's first change. Tested: no hint is marked done and nothing is written to storage on opening. Clear drops the fragment but does not mark the hint done. A real change marks it done.
- **Stale copy:** the URL is built from `selection.start()`/`end()` at click time, which is the latest written value, not the delivered one. The fallback check compares against the current range again when the promise settles.

## Previous findings (F-39 r1)

- **QA-39-r1-1:** fixed. `hygiene.test.ts` now asserts the `[data-hint='done'] body > .page .charts { --hint: 0px; }` rule and the edit page's `#wx-edit` structure.
- **QA-39-r1-2:** fixed. Focus moves to the chart stack, and the test asserts it. QA-40-r1-1 records the scroll risk of this fix.
- **QA-39-r1-3:** fixed. The pin check and the button-count check are separate, and each has its own message.
- **Sharing line:** present in `howto/index.ts`. The touch and mouse wording match the F-39 feature table exactly, and both are asserted in `howto.dom.test.ts` (AC 10).

## Questions

None.

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 Round trip | `rangeCopy`, `parseRangeLink` | Page test: copy, then reopen gives [123, 2345]. Lib round-trip test. Map highlight not asserted (QA-40-r1-2) |
| 2 Button, "Link copied" 2 s, fallback | `RangePanel` `oncopy`, `copier`, `copyField` | Page tests (fake timers, field selected and read-only) |
| 3 Fragment only | No server change in the diff. Browsers do not send fragments | By reading. Access log: hand check 6 |
| 4 Time positions only | `rangeFragment` uses `data.time` only | Lib test |
| 5 Edit and published pages untouched | `edit ?` guard | Page test (edit seam), E-2 source scan |
| 6 Cut to the data, hidden sections | `bound` clamps | Lib and page cut tests. Hidden section: by reading only (QA-40-r1-2) |
| 7 Invalid values ignored | Regex, `from >= to`, `a < b` | Lib tests (27 cases), page tests (6 cases) |
| 8 JavaScript off, LCP | Read after decode. No markup change | By reading. Hand check 7 |
| 9 Address bar | Subscriber plus `replaceState` | Page tests (change, Clear, no fragment, history length) |
| 10 F-39 line | `howto/index.ts` | `howto.dom.test.ts` |

Not run: a real browser. The hand checks for the user:

1. Select a range, use "Copy link to this selection", and open the link in another browser. The map highlight and the panel values should match.
2. On a 360 px phone, the button sits beside Clear or wraps to its own line, with no horizontal overflow.
3. Dismiss the hint with × on a phone: the page should not jump. With the keyboard, check the focus ring on the stack.
4. Over plain HTTP (no Clipboard API): the field appears in the panel and disappears when the range changes.
5. Open a link with a range, change the range, then press Back: you should leave the page. Also enter and leave fullscreen on a link with a range.
6. The Caddy access log shows no `range=`.
7. With JavaScript off, the page looks as before.
