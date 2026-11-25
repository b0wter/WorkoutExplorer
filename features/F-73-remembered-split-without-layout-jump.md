# F-73 Remembered map split without a layout jump

- **Status:** planned (25 Nov 2026); no open product question
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (frontend: head script; backend: the CSP hash in `deploy/Caddyfile` and its check; frontend first)
- **Lane:** normal (changes the inline script the CSP allows; short architect note revising F-13 D-10's exception and F-22 H-4 / OQ-88)
- **Requirements:** AP-17; DS-4, DS-6, INF-1; NFR Performance (LCP), NFR Privacy (no tracking cookies)
- **Depends on:** F-13, F-17 (splitter, stored split), F-22 (CSP), F-54 (today's head script)

## Goal
A viewer who dragged the map/content splitter opens the next workout or edit page already at their split, without the columns jumping after the first paint.

## What exists
- The split is stored per layout in browser storage (`wx.split.wide`, `wx.split.landscape`, `lib/prefs`) and applied by the page bundle as `--map-w`; module scripts run after the first paint, so a stored split shifts the layout once (F-13 D-10 exception, accepted by the user 29 Oct 2026; F-17 Waiting on). OQ-88 deferred the fix (F-22 H-4).
- Every page already has one byte-constant inline head script (theme, hint flag, language) allowed by its `sha256-` hash in the CSP (ADR-032 item 3, ADR-059); `caddy_test.go` checks the built pages' script against the policy.

## Scope
- **Before the first paint:** the head script reads the stored split for the layout the window is in (wide or short landscape, the same media queries as the splitter) and applies it, so the first paint already has the viewer's split. The bundle's own application stays (it is then a no-op); dragging, double-click reset and storage are unchanged (AP-17).
- **Same validation as the bundle:** a value outside 30 % to 70 %, unreadable, or blocked storage gives the default split, without a message.
- **Pages:** the viewing page and the edit page (AP-17: same layout). Other pages keep the same script text (one hash), where it finds no splitter and does nothing visible.
- **No inline style:** the value is set through the CSSOM (allowed by `style-src 'self'`), never a `style` attribute (F-13 D-10, F-22 H-4).
- **CSP:** the new script's hash is added; the previous hash stays until no served page carries it (ADR-032 item 3).
- **Read only:** the head script still only reads storage (TD: `lib/prefs` stays the only writer). Nothing new is stored or sent; About text unchanged (INF-1 already lists the split).

## Out of scope
- Rotating a phone or resizing the window after load (the bundle handles it, as today); remembering the pinned map (AP-18 stays per load).

## Acceptance criteria
- [ ] With a stored wide split of 60 %, a cached and an uncached load of a published page and of the edit page on desktop show the map at 60 % in the first paint; no layout shift from the split in a performance trace (CLS contribution 0).
- [ ] Phone in landscape with a stored landscape split: the same, closes the F-17 Waiting on item.
- [ ] Stored values 20, 80, "abc", and blocked storage (Safari private, site data blocked) give the default split, no error in the console.
- [ ] Phone portrait and a workout without a visible route: one column, the stored split has no effect.
- [ ] No CSP violation on any page (viewing, edit, upload, About, generic); the Caddy check passes with the new hash; the old hash still listed.
- [ ] JavaScript off: default split, page unchanged; LCP and entry bundle unchanged.
