# QA F-39 How to use - round 1

- **Date:** 2026-11-10
- **Verdict:** ready. No Blocker or Major. Each acceptance criterion has code and a unit or DOM test. The parts a DOM test cannot show (phone layout, pointer wording, native Esc and back gesture, network log, layout shift) are listed as user checks.
- **Inputs:** `features/F-39-how-to-use.md`, `architect/F-39-how-to-use.md` (HU-1 to HU-8), ADR-011 rev. 14, ADR-032 item 3, product spec AP-22, AP-7, DS-1 and NFR Compatibility. Reviewed commit `dedb4b2` (`git show HEAD`).

## Build and test results

- `make ci`: exit 0. Vitest 64 files, 1,618 tests passed. `svelte-check --fail-on-warnings` clean. `go vet` (including `-tags caddycheck`) clean. Go packages ok. `docker-check: ok`.
- Bundle size (`check-bundle-size.mjs`): `entry index.html: 58345 / 60000 bytes gzip PASS`, and the same for `edit.html`. That is +599 over the 57,746 baseline, inside the 600 share (ADR-011 rev. 14). The lazy chunk is `howto-*.js` (780 B gzip) plus `howto-*.css` (290 B gzip). The entry refers to the chunk only by name, through the dynamic import.
- Head-script hash recomputed with `openssl dgst -sha256 -binary | base64`: `oU85smkdewIErbjHrk9QZ1FiICdjcwU0koHX7ul9Dpw=`. It matches `deploy/Caddyfile` and `caddy_test.go`. The old hash stays second in the list, as ADR-032 item 3 requires.

## Findings

### Minor

- **ID:** QA-39-r1-1
- **Severity / kind:** Minor / risk (test gap)
- **Route:** frontend
- **Where:** `src/web/src/page/page.css` rule `[data-hint='done'] body > .page .charts { --hint: 0px; }`
- **Relates to:** AC 6, HU-2
- **Evidence:** No test reads this rule. `grep data-hint` over the tests finds only the head-script test and the `dataset` checks in `page.dom.test.ts`. If the rule were deleted, every test would still pass. Later loads would then keep an empty 24 px row, which the spec forbids. Nothing tests the edit-page exclusion either. By reading, it works: the edit page's `.page` sits inside `#wx-edit`, not directly under `body`.
- **Consequence:** A future CSS refactor could silently bring the reserved row back, or apply it to the edit page. Fix: add a `hygiene.test.ts` assertion on the selector and its value.

- **ID:** QA-39-r1-2
- **Severity / kind:** Minor / defect (accessibility)
- **Route:** frontend
- **Where:** `src/web/src/components/charts/ChartStack.svelte`, `.hint-x`
- **Relates to:** HU-5, NFR Accessibility
- **Evidence:** When the × is pressed, its `{#if hint === 'once'}` branch removes the button. Focus falls back to `body`. Not run in a browser; this follows from the DOM removal.
- **Consequence:** A keyboard or screen-reader user who dismisses the hint loses their place on the page. This is small, but it is cheap to fix, for example by moving focus to the chart stack.

- **ID:** QA-39-r1-3
- **Severity / kind:** Minor / preference
- **Route:** frontend
- **Where:** `src/server/tools/protoserve/samples_test.go:135-137`
- **Relates to:** engineer deviation 6
- **Evidence:** The check now also fails when the button count is not 2 or when the How to use markup is missing. It still reports `"pin toggle in the server markup"`.
- **Consequence:** A failure would point the reader at the wrong cause.

## Engineer's deviations

| # | Judgement |
| --- | --- |
| 1 | The button turns plain at once instead of after a reload. Accepted: the feature says "until opened once, then plain"; the AC only tests the reload. The flag is written only after `showModal`, and nothing is written when the chunk fails to load (tested). |
| 2 | One paragraph per section, as "**Name.** text". Accepted: it meets "one line per section", and the texts match the feature table exactly (asserted in `howto.dom.test.ts`). |
| 3 | `.select-hint:has(button)`. Accepted. NFR Compatibility asks for the last two versions of Chrome, Safari, Firefox and Edge; all of them have supported `:has` since late 2023. The codebase already uses it in `edit.css` and `map-view.css`. |
| 4 | Shared CSS rules, and no accent border when emphasised. Accepted. The feature asks for "the accent background", which is there. Both looks share the same 120 × 32 box, so the switch causes no shift. |
| 5 | A load failure reuses `.report-fail` and the shared `loadFailed` helper. Accepted, and tested: one line only, no flag written. |
| 6 | The `samples_test.go` edit is correct in substance. See QA-39-r1-3 for the message. |
| 7 | The F-40 note: `start.ts` subscribes to the selection store, so a range applied from a link would count as the first selection. This is already recorded as HU-4 and in a code comment. It is F-40's job and does not affect F-39. |

## Previous findings

None (first round).

## Questions

None.

## Coverage

| AC | Code | Test |
| --- | --- | --- |
| 1 Button on `/w/` only, phone and desktop | `index.html` `.ctl`; no button in `edit.html` or the other pages | `hygiene.test.ts` (template buttons), `samples_test.go`, `edit.dom.test.ts` (no button). The 360 px phone layout is a **manual** check |
| 2 Emphasised until opened | `enhanceHowto`, `.fresh`, `prefs.howtoOpened` | `enhance.dom.test.ts`, `page.dom.test.ts` (fresh, then plain with the stored flag) |
| 3 Texts and wording, the same for every workout | `howto/index.ts` `LINES`, `(pointer: coarse)` in `howto.css` | `howto.dom.test.ts` checks every string. Which wording a device shows is a **manual** check |
| 4 Closing, no history entry | native `showModal`, close button, click on the backdrop | `howto.dom.test.ts` (button, backdrop, click inside does not close). Esc, the back gesture and history are **manual** checks (jsdom has no `showModal`) |
| 5 Lazy chunk, budget | `import('../howto')` on click | `enhance.dom.test.ts` (no load before the click); CI size check 58,345 / 60,000 |
| 6 Hint until the first range or ×, no jump | head script, `data-hint`, `start.ts` `hintDone`, `HintMode` | `theme-script.test.ts`, `page.dom.test.ts`, `ChartStack.dom.test.ts`. The CSS rule is untested (QA-39-r1-1). The absence of a jump is a **manual** check |
| 7 Storage blocked | `lib/prefs` guard; the head script's `try` | `page.dom.test.ts` (throwing storage), `prefs.test.ts` |
| 8 No cookie, no request carries the flags | storage only through `lib/prefs` | by reading: no cookie or fetch code was added. The network log is a **manual** check |
| 9 Edit page unchanged | `hint = edit ? 'plain'` | `edit.dom.test.ts` |

Not run: a real browser. No `-race` run, since the diff changes no Go code apart from tests. No double-click race probe on the first chunk load: a second `showModal` on an open modal dialog does nothing in current browsers.
