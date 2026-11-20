# QA F-53 My workouts on this device - round 1

- **Date:** 2026-11-20
- **Verdict:** ready with conditions - no Blocker or Major. Conditions: the browser hand checks listed under Coverage (network view, phone, blocked storage in a real browser). Four Minor findings.
- **Inputs:** `features/F-53-my-workouts.md` (18 ACs, numbered AC 42 to AC 59 below as in the design), `architect/F-53-my-workouts.md` v0.1 (MW-1 to MW-10), ADR-058, ADR-011 (F-53 row), ADR-020, ADR-025, F-19 E-2, F-20 C-5, F-11 U-10; spec AC-7, INF-1, NFR Privacy; TD code structure and storage rule. Commits 15b92d8 (backend B-1), f8e6901 (frontend), 5c9d654, b9230d9, a325794 (docs). Working tree clean at f8e6901.

## Build and test results

- `make ci`: exit 0. go vet, `go test -count=1 ./...` ok; svelte-check clean; vitest **82 files, 2,091 tests passed**; `npm run size`: entry for index.html, edit.html and my-workouts.html **57,926 / 60,000** gzip (+64 against F-53's 60 ceiling, as booked in ADR-011), upload 3,785 / 8,000; `check-upload-entry`: "no storage API, no shared chunk; viewing page: no wx.workouts PASS"; caddy-check on caddy:2.11 = v2.11.9 (`TestCaddy` ok); docker-check ok. (Local node 22 gives an EBADENGINE warning; the image build uses node 24.)
- Built output: `wx.workouts` appears in exactly one file, the lazy `lib/mine` chunk; neither `index.html` nor `my-workouts.html` loads it statically.
- **Mutants** (copy of `src/web` in the scratchpad, vitest on `lib/mine`, `mine`, `edit.dom`, `page/hygiene`, `upload`; build mutant through `vite build` + `check-upload-entry.mjs`): **31 mutants, 27 killed, 2 equivalent, 2 survived.**
  - All seven mutants the design's verification table requires are killed:
    - link built from a stored string (M1);
    - title through `innerHTML` (M2);
    - removal on not-found without the token match (M3b);
    - viewing page importing `lib/mine` (M4);
    - list page rewriting an unreadable value (M5);
    - upload page touching storage (M6, build check);
    - draft expiry ignoring 48 h (M7).
  - Also killed: M8 to M17, M19 to M24, M27 to M30 (delete/not-found keep the entry, `target=_blank`, View on a draft, token unchecked, write-back failure ignored, save/publish/published page not writing, date unchecked, Clear without confirm, Forget removes all, ID regex unanchored at either end, published page adds, default title stored as text, unreadable value not replaced by the edit page, `published` always true, shell without noindex).
  - Equivalent:
    - M3 (token dropped only from `remove`'s filter; the guard still requires the match, and there is one entry per id);
    - M26 (an extra upsert before the not-found check; the later write supersedes it).
  - Survived: M18, M25 (QA-53-r1-3).
- Probes: a `__proto__` key in a stored entry stays inert (no link, `published` false, `Object.prototype` untouched); id with a non-ASCII digit and token with a trailing newline give no Edit link; 10,000 entries plus a 500 KB title: `upsert` 22 ms (jsdom), list renders (no error). Focus after Forget, Clear and Cancel falls to `<body>` (see r1-4).

## Findings

### Minor

- **ID:** QA-53-r1-1
- **Severity / kind:** Minor / defect (privacy wording)
- **Route:** frontend
- **Where:** `src/web/about.html:99`; `src/web/src/lib/mine/index.ts` (`upsert`, `prune`)
- **Relates to:** INF-1, AC 58, MW-4
- **Evidence:** About (the privacy policy) says "a workout's entry also goes when the workout expires or is deleted here". The code drops expired entries only when My workouts is shown (`prune`) or when that workout's edit page answers not found. Edit and published pages never prune. A sharer who never opens My workouts keeps every expired workout's ID and edit token in storage indefinitely.
- **Consequence:** The policy promises more than the code does. The tokens are useless once the workout is gone, so the harm is small, but the policy is wrong. Cheapest fix: drop expired entries in `upsert` too, using the same rule as `prune`; or reword: "leaves the list once it has expired, the next time My workouts is shown".

- **ID:** QA-53-r1-2
- **Severity / kind:** Minor / risk (documents out of step)
- **Route:** architect
- **Where:** `architect/F-19-edit-page-and-settings.md:18` (E-2), `:53`, `:59`; `architect/F-20-publish-share-delete-controls.md:34` (C-5), `:65`
- **Relates to:** ADR-058 items 1, 2, 4; MW-5; F-19 E-2; F-20 C-5
- **Evidence:** F-19 E-2 still says the token never goes to Web Storage, and that the only DOM exception is F-20 C-5's edit link. F-20 C-5 and its verification row say "never an attribute (`href` …), … a link element … storage". F-53 now stores the token in `localStorage` and puts it in `<a href="/edit/{id}#{token}">` on `/my-workouts`. ADR-058 says it "adds one exit to F-19 E-2", but the live F-19 and F-20 texts were not amended. Neither ADR-058 nor MW-5 says why a link is acceptable here when C-5 rejected one for the same secret. The href shows the full URL with the token in the status bar on hover, and in "Copy link" and drag-and-drop. The published page's "Go there" is a button for exactly this reason.
- **Consequence:** A later reviewer applying F-19 or F-20's rule as written will flag F-53 or "fix" it. The trade-off (link versus a navigating button) is not recorded. I accept the link if the architect records why. Anyone at this browser can already read storage, so hover exposure adds little.

- **ID:** QA-53-r1-3
- **Severity / kind:** Minor / test gap
- **Route:** frontend
- **Where:** `src/web/src/edit/start.svelte.ts:389` (`if (r.ok) record(r.value);` in the `pageshow` handler); `src/web/src/lib/mine/index.ts` `upsert`
- **Relates to:** AC 46 ("every successful save updates…"), AC 44, MW-3, ADR-058 "Token rotation"
- **Evidence:** Two mutants survive the whole suite:
  - **M18:** deleting that `pageshow` line. The F-20 C-2 back-forward tests already dispatch `pageshow` with `persisted`, but none checks the list entry.
  - **M25:** `upsert` keeping the stored `token` over the address's (`token: old?.token ?? entry.token`). No test upserts over an entry whose token differs.
- **Consequence:**
  - **M18:** after a bfcache restore of a draft that was published in another tab, the entry may be left showing Draft, and nothing catches it.
  - **M25:** if this regresses, a wrong stored token (hand-edited now, rotation later) is never repaired by opening the valid link. Its Edit link then always gets 404, and P-12 keeps it, so it stays broken until it expires.
  - One assertion each closes both: one in the existing "published meanwhile" test, one in "one entry per workout" with `token: OTHER` stored.

- **ID:** QA-53-r1-4
- **Severity / kind:** Minor / preference (accessibility is not a goal, spec NFR Accessibility)
- **Route:** frontend
- **Where:** `src/web/src/mine/index.ts` `render()` (`main.replaceChildren()`)
- **Relates to:** AC 50, MW-10
- **Evidence:** Probe: after Forget, Clear list or Cancel, `document.activeElement` is `BODY`; the confirmation's "Remove all" is not focused. Forget buttons are all named "Forget", with no workout in the name.
- **Consequence:** A keyboard user loses their place after every action. List it for the small lane, or ignore it under the spec's accessibility stance.

## Previous findings

None (round 1).

## Questions

1. **Architect (r1-2):** On `/my-workouts`, should Edit be a plain link (current) or a button that navigates, as on the published page? Suggestion: keep the link, and record in ADR-058 why C-5's "never an attribute" does not apply here. Settled by: one line in ADR-058 plus amended F-19 E-2 and F-20 C-5 texts.
2. **Main session, bundle:** +64 against the 60 ceiling. I tried no variant. The only lever I see is chunk layout (merge `lib/mine` into the `src/mine` chunk to drop one preload name). That changes the owner chunk's dependencies, and is not cheap for 4 bytes. Suggestion: accept from the reserve (reserve 2,074).

## Coverage

| AC | Code | Test | Result |
| --- | --- | --- | --- |
| 42 entry after upload, no cookie, nothing sent | `edit/start.svelte.ts` `record(got.value)`; upload page unchanged | `edit.dom` "first read adds one entry … nothing sent", `document.cookie` empty, request URLs and bodies checked; `check-upload-entry` (build mutant M6 killed) | traced; network view: hand check |
| 43 link, columns, order, default title, same tab, empty text | `upload.html`, `mine/index.ts` | `mine/page.dom` "the list", "empty"; `upload/page.dom` | traced |
| 44 one entry per workout | `lib/mine` `upsert` by id | `mine.test` upsert; `edit.dom` "one entry per workout" | traced |
| 45 relative expiry, draft 48 h | `effectiveExpiry`, `relative` | `mine.test`, `page.dom` (M7 killed) | traced |
| 46 publish, save, delete | `record` in apply/publish, `remove` on delete | `edit.dom` publish, save, delete both pages (M9, M15, M16, M17 killed) | traced; pageshow path untested (r1-3) |
| 47 prune on show, no request | `prune`; page sends nothing | `page.dom` afterEach `fetch` never called; "drops expired" | traced |
| 48 not found with token match only (P-12) | `remove`, `gone()` | `mine.test`, `edit.dom` P-12, save 404 (M3b, M10 killed; M3 equivalent under one-entry-per-id) | traced |
| 49 add on any valid edit link | `record` at first read | `edit.dom` first read, with another entry present | traced |
| 50 Forget, Clear asks once | `forget`, `clear`, confirm render | `page.dom` (M20, M21 killed) | traced |
| 51 note on list and published page | `TEXT.note`, `PublishedView.svelte` | `page.dom`, `edit.dom` | traced |
| 52 storage blocked, full, unreadable | guarded `read`/`write`; write-back decides availability | `mine.test`, `page.dom` (four cases), `edit.dom` three cases incl. publish (M5, M14 killed) | traced; Safari private and a browser "block site data" setting: hand check |
| 53 hostile title as text | `textContent` only | `page.dom` hostile, boundary source scan (M2 killed) | traced |
| 54 foreign or script links never built | links only from `ID`-checked parts | `mine.test`, `page.dom` (M1, M12, M13, M22, M23 killed) | traced |
| 55 phone portrait, tap targets | `mine.css` `.mine-box` overflow, 44 px | none (CSS) | hand check |
| 56 noscript | shell `<noscript>` | `page.dom` shell test | traced |
| 57 viewing page never reaches the list | `main.ts` `import()` only | `boundary.test` static graph; build check on index.html files (M4 killed) | traced |
| 58 About paragraph | `about.html` | `about.test` | traced (wording, r1-1) |
| 59 server unchanged | diff: `shell.go` list, Caddyfile route, Caddy test | `TestCaddy` /my-workouts 200 no-store with security headers and no Set-Cookie; `.html` and trailing slash 404 | traced |

**Also checked, correct:**
- The token leaves `lib/owner` only through `token()`, which only `src/edit`'s `list()` calls. It is not in any request URL or body, the console, a cookie or history state. `expectNoTokenOutsideTheFragment` still runs and exempts only the `token` field of `wx.workouts`.
- Stored links are never trusted: no stored `url`/`edit`/`view` field is read. Links are rebuilt from `^[0-9A-Za-z]{22}$` id and token as root-relative paths.
- `v:2`, invalid and truncated JSON, `null` and a wrong-shape `workouts` are unreadable on the list page, which never rewrites them; the edit page replaces them (ADR-058 item 4).
- A Caddy route with global CSP, `Referrer-Policy: no-referrer` and `X-Robots-Tag`; the shell has noindex, no-referrer meta, no Open Graph tags, and the same theme script bytes, so the CSP hash is unchanged.
- The upload entry contains no storage API (F-11 U-10 holds).
- TD and ADR-011 updated.

**Not run:** a real browser (network panel, phone viewport, Safari private mode, a browser storage block setting, JS off); Go `-race` (server change is a string list and a Caddy route only).
