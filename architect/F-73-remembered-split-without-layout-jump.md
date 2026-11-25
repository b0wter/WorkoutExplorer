# F-73 Remembered map split without a layout jump: architecture

- **Version:** 0.1
- **Status:** accepted (25 Nov 2026); normal lane; two assumptions listed under Open points
- **Feature:** [`features/F-73-remembered-split-without-layout-jump.md`](../features/F-73-remembered-split-without-layout-jump.md)
- **Requirements:** AP-17; DS-4, DS-6, INF-1; NFR Performance (LCP), NFR Privacy
- **Builds on:** [F-13](F-13-activity-page-layout.md) D-10, D-17; [F-17](F-17-range-selection-and-axis-toggle.md) (splitter, `lib/prefs`); [F-22](F-22-caching-and-security-headers.md) H-2, H-4; [F-39](F-39-how-to-use.md) HU-2 and [F-54](F-54-translated-interface.md) LG-2, LG-9 (head script, hash rotation); [ADR-032](ADR-032-response-headers-in-caddy.md) item 3; [ADR-011](ADR-011-activity-page-bundle-allocation.md); [ADR-043](ADR-043-page-shells-filled-at-start.md); [ADR-053](ADR-053-web-assets-served-from-the-data-directory.md). No new ADR
- **Owner:** both, one commit: frontend (head script, `restoreSplit`), backend part = the hash in `deploy/Caddyfile` and in `caddy_test.go`'s policy copy

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| RS-1 | **The shared head script applies the stored split to `<html>`.** It picks the key exactly as `splitter.ts`'s `layoutOf` does: `wx.split.landscape` when `LANDSCAPE_QUERY` matches, else `wx.split.wide`, and sets `--map-w` with `document.documentElement.style.setProperty`. It does not test `TWO_COLUMN_QUERY` or look for a map: `page.css` reads `--map-w` only inside the two two-column media queries and only under `.has-map`, so phone portrait, a workout without a visible route, and the upload, About, generic and My workouts pages ignore it | `.layout` does not exist yet while the head script runs, and the edit page's layout is mounted later by the owner chunk. A custom property on the root is inherited by any `.layout` the moment it exists, so the viewing and edit pages get the split at their first paint without waiting for anything. Mirroring `layoutOf` keeps one rule. One script text for every page (one hash) |
| RS-2 | **The same validation as the bundle, and never an error.** A stored text is applied only if it matches `lib/prefs`' pattern (`^\d{1,3}(\.\d{1,2})?$`) and lies in 30 to 70 inclusive (`MIN_SHARE`, `MAX_SHARE`). It is applied as the stored text plus `%`, which the pattern keeps a safe CSS token. Anything else (20, 80, `abc`, empty, missing), a storage getter that throws, or a missing or throwing `matchMedia` sets nothing. The split part sits inside the script's existing `try`, so a failure there never stops theme, hint or language | AC 3. `60` and the bundle's `60.00%` lay out the same |
| RS-3 | **One owner after load: `restoreSplit` takes over.** When it finds `.layout`, it applies the stored split to `.layout` as today and then removes `--map-w` from `<html>`, in the same synchronous call. Dragging, double-click reset, the landscape listener and storage stay as they are, all on `.layout` | Without the handover a reset, or a rotation into a layout with nothing stored, would remove `.layout`'s value and expose the inherited head-script value: the reset would not reset. After the handover the bundle's model is unchanged |
| RS-4 | **CSSOM only, read only.** No `style` attribute in markup, no `setAttribute('style')` (F-13 D-10, F-22 H-4); CSSOM writes are not governed by `style-src`. The script only reads storage; `lib/prefs` stays the only writer. Nothing new is stored, sent or logged; About text unchanged (INF-1) | CSP; TD storage rule |
| RS-5 | **Script placement.** The script stays after `<meta name="viewport">` in every page | `matchMedia` before the viewport meta evaluates a phone against the 980 px layout viewport, so phone landscape would read the wide key |
| RS-6 | **Hash rotation by ADR-032 item 3, no new rule.** The new script's `sha256-` goes first after `'self'` in `script-src`, in `deploy/Caddyfile` and in `caddy_test.go`'s copy, in the same commit as the script (F-54 LG-9). The current F-54 hash and the two older ones stay; dropping old hashes follows the existing Caddyfile rule (no published page carries the text: rebuilt or expired), not F-73. Effects: **pages published before the release** keep the old script and load their own older bundle (ADR-053), so they keep today's one shift until rebuilt or expired; **edit and other shells** are filled at start (ADR-043) and always pair the new script with the new bundle. No case pairs the old script with the new bundle, and an absent root value makes the handover a no-op anyway. A deploy that briefly serves new shells under the old policy blocks the head script for that window, as at F-39 and F-54; no new ordering mechanism | Old pages must stay CSP-clean (ADR-032 item 3); the policy grows by one hash |
| RS-7 | **Cost.** Entry bundle (ADR-002 count): only the handover line, **ceiling 30 bytes gzip**, booked in ADR-011 when measured; no new module. The head script grows by about 150 bytes raw in each HTML page, which ADR-002 does not count. LCP: one more `localStorage` read and one `matchMedia` before the body, synchronous, no network, no forced layout; LCP must not change beyond run-to-run noise. With JavaScript off nothing runs: CSS defaults (40 % landscape, 1:1 desktop), page unchanged | AC 6 |

## What the tests must prove

| Claim | Proof |
| --- | --- |
| Same validation as the bundle (RS-2) | The head script, run against stand-ins (storage, `matchMedia`, root style), sets `--map-w` for exactly the stored texts that `lib/prefs` plus `restoreSplit` accept, to the same number, for both layouts, over one shared input list (at least 20, 29.99, 30, 60, 70, 70.01, 80, `abc`, `50%`, `1e2`, empty, missing) |
| One rule, no drift (RS-1) | The script contains `LANDSCAPE_QUERY` and both `PREF_KEYS` split keys verbatim; it picks the landscape key only when the query matches |
| Silent failure (RS-2) | Throwing storage and missing `matchMedia`: nothing set, theme, hint and language still decided |
| Handover (RS-3) | jsdom: with `--map-w` on the root, after `restoreSplit` the value is on `.layout` and gone from the root; then a reset, and a landscape change to a layout with nothing stored, leave no `--map-w` anywhere |
| Placement and identity (RS-5) | The existing check that all six pages carry the same script with no template action or comment, plus: the script follows the viewport meta in each |
| Hash (RS-6) | New hash first in both policy copies (existing check); the previous hash still in both; every built page's inline script has its hash in the served CSP (existing Caddy check) |
| AC 1 to 6 in a browser | QA through Caddy: performance trace with a stored 60 % on a published page and the edit page, cached and uncached (split CLS 0); phone landscape; the AC 3 values and blocked site data; phone portrait and a workout without route; no CSP violation on viewing, edit, upload, About, generic; JavaScript off |

Mutants that must fail a check: the head script accepting 80; the handover removed; the script moved above the viewport meta; the previous hash dropped.

## Hand-offs

| To | What |
| --- | --- |
| Requirements | TD Load order step 1: the head script also applies the stored split (F-73 RS-1). F-17's Waiting on item for the phone-landscape shift closes with AC 2 |

## Open points (assumed; the user answers by number in the report)

1. AC 6 "entry bundle unchanged" read as: no new entry code beyond RS-3's handover line, ceiling 30 bytes gzip.
2. Pages published before the release keep today's one shift until rebuilt or expired (RS-6); no forced rebuild for F-73.
