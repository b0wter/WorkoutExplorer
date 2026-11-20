# F-53 My workouts on this device: architecture

- **Version:** 0.1
- **Status:** accepted (20 Nov 2026). Critical lane (edit links stored in the browser and rendered back): QA at effort high. Two open points (P-12, P-13), neither blocks the build
- **Feature:** [`features/F-53-my-workouts.md`](../features/F-53-my-workouts.md); decisions OQ-218 to OQ-222, OQ-278, OQ-279
- **Requirements:** AC-7, UP-1, UP-5, UP-7, UP-11, SH-2, AC-1, AC-4, AC-6, INF-1, INF-5, NFR Privacy
- **Builds on:** [F-19](F-19-edit-page-and-settings.md) E-1, E-2 (owner page, `lib/owner`), [F-20](F-20-publish-share-delete-controls.md) (published page), [F-11](F-11-upload-page.md) U-9, U-10, [F-29](F-29-about-page-and-attributions.md), [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md), [ADR-024](ADR-024-upload-page-plain-typescript-entry.md), [ADR-025](ADR-025-owner-api-access-and-not-found-response.md), [ADR-011](ADR-011-activity-page-bundle-allocation.md), [ADR-032](ADR-032-response-headers-in-caddy.md), [ADR-043](ADR-043-page-shells-filled-at-start.md). New: [ADR-058](ADR-058-my-workouts-list-in-browser-storage.md)
- **Owner:** **both** (was frontend). Backend step B-1 is three lines of serving (MW-1); everything else is frontend. No endpoint, table, column, setting or log line (AC 59)

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| MW-1 | **Page `/my-workouts` on the activity entry.** A static shell `src/web/my-workouts.html` loads `main.ts` like `edit.html` (`src/web/edit.html:15`), with its own marker element; `main.ts` gets one more branch that reaches the lazy `src/mine/` by one `import()` (as `src/web/src/main.ts:7`). Shell: same head theme script bytes (CSP hash unchanged), `noindex`, `no-referrer`, no Open Graph card, Impressum marker, a `<noscript>` line saying the page needs JavaScript (AC 56). **B-1 (backend):** the shell joins `internal/shell`'s list (`src/server/internal/shell/shell.go:31`); Caddy serves `/my-workouts` from `{data}/site/` with `Cache-Control: no-store` (it renders edit links, like the owner pages), `/my-workouts.html` stays a 404; one row in the Caddy test. Rejected: a plain-TypeScript entry (ADR-024 pattern) | Sport names, dates, the default title (`lib/page`'s `defaultTitle`), the theme and F-54's language mechanism already live in the activity entry. A plain entry would duplicate them or pull them into a chunk shared with the activity entry, ADR-024 item 3's cost. Cost: the page downloads the activity entry, normally cached from the edit page the sharer came from. ADR-024 stands: the upload page gains only a link (MW-8) |
| MW-2 | **Stored entry** (ADR-058): `id`, `token` (22 base62 each); `title` = the stored title when not blank, else `suggested_title`, `""` when the default title applies; `sport` = the FIT sport code the edit page's header shows (trimmed sport after a trim); `date` = the header's local start date `YYYY-MM-DD` (workout zone, UTC when unknown); `created`, `expires` = the settings' `created_at`, `expires_at` (RFC 3339 UTC); `published` = `visibility` is `unlisted`. Nothing else | Store as little as possible. The default title is built at display, so F-54 translates it; the owner's title and the suggestion are never translated (F-54 scope) |
| MW-3 | **Who writes when.** The **edit page** upserts by `id` after every successful settings read, save and publish (AC 44, 46, 49); sport and date come from the decoded draft and keep their stored values when it did not load. The **published page** updates title, status and expiry of an existing entry, never adds one (it decodes no draft); the edit page already wrote it at publish. A successful delete removes the entry (AC 46). Any owner request that ends in the not-found page (`showGeneric`, `src/web/src/edit/start.svelte.ts:134`) removes the entry **only if its token equals the address's** (ADR-058 item 5, P-12). Each write reads storage afresh and writes at once, never from a copy held across an `await`. The **upload page writes nothing** (F-11 U-10 holds): a successful upload always navigates to the edit page (U-9), whose first read adds the entry (P-13) | One writer with complete values; no storage code in the upload entry |
| MW-4 | **The list page** reads, drops entries past their effective expiry, and writes the result back; then renders. It sends no request (AC 47). Forget writes the list without that entry at once; Clear list removes the key after one confirmation (OQ-279). Effective expiry, computed at display, never stored: `expires` when published; for a draft the earlier of `expires` and `created` + 48 h (UP-7, AC 45). The device clock decides; no grace period. Order: newest `created` first | One rule in `lib/mine`; server time for order and expiry, device time only for "now" |
| MW-5 | **Hostile values** (AC 53, 54). An unreadable value (ADR-058 item 3) shows the not-available line and is never rewritten by this page. Per entry: not an object → dropped; `created` or `expires` unparseable → counts as expired; `id` failing the check → no Edit and no View; `token` failing → no Edit; `title`, `sport`, `date` of the wrong type → absent. Links are built only from checked parts as root-relative paths; plain `<a href>`, same tab, no `target`, no `window.open` (AC 43). Every stored string reaches the DOM only as text (`textContent` or Svelte text interpolation), never `innerHTML`, `{@html}` or an attribute other than the built `href` | Own-origin edit and viewing links by construction; nothing stored is interpreted |
| MW-6 | **Storage failures** (AC 52). Every access is guarded as in `lib/prefs` (`src/web/src/lib/prefs/index.ts:54`): a throwing getter, `getItem` or `setItem` (blocked, quota) is contained. Owner pages carry on without a message. The list page counts the list as available only when the value is readable and its write-back (MW-4) succeeds, which also catches storage that reads but refuses writes ("full", Safari's zero quota) | A failed list write must never break upload, edit or publish |
| MW-7 | **Module boundary.** `lib/mine`: pure (parse and check, upsert, remove, prune, effective expiry, link paths), storage passed in; imports nothing of ours. Imported only by `src/edit` and `src/mine`; never by `src/page`, `components/`, `lib/prefs`, `lib/owner` or the upload page. `lib/owner` stays the only reader of the fragment; it exposes the token to `src/edit` through one read-only field used only for the list write. The viewing page (`/w/`) never loads `src/edit` or `src/mine`, so it cannot reach the key (AC 57: checked on the import graph and on the built viewing-page chunks) | TD rule "only `lib/prefs` touches storage" becomes "`lib/prefs`, and `lib/mine` for `wx.workouts` only" |
| MW-8 | **Other pages.** Upload page: one static link to `/my-workouts` in `upload.html`, no script change (AC 43). Published page: the note under its links, in the owner chunk (AC 51). About page: the "In your browser" paragraph of `src/web/about.html` names the list and its edit links (AC 58) | No storage or code in the upload entry |
| MW-9 | **Texts and F-54.** Every interface text of `src/mine` sits in one place; statuses and sports are stored as codes, so only `title` is stored text. Relative expiry uses `Intl.RelativeTimeFormat` (language `en` now; F-54 passes the chosen one), which handles plurals in all four languages; our code only picks the unit (minutes, hours, days). Sport names and dates through `lib/format` as on the edit page | F-54 swaps one text table and one language argument; nothing stored needs translating |
| MW-10 | **Layout** (AC 55). The table scrolls inside its own box on a phone; Edit, View and Forget get touch-sized targets | No horizontal page scroll |

## Interaction with lifetime, delete and tokens

| Event | Entry |
| --- | --- |
| Lifetime changed or published here | Updated by the save or publish answer (MW-3) |
| Expired (server) | Dropped by the list page at the same stored `expires` (MW-4) |
| Deleted here | Removed (MW-3) |
| Deleted by the operator (INF-3), or changed in another browser | Stale until it expires or its edit page is opened here (OQ-221). A draft published elsewhere is dropped here 48 h after upload; the other browser holds it. Accepted |
| Opened with a wrong token for a listed ID | Not-found page; the listed entry stays (P-12) |
| Token rotation | None exists (ADR-020). If one is added, the upsert by `id` writes the new token; see ADR-058 "Would choose differently" |

## Cost

| What | Limit |
| --- | --- |
| Activity entry (static) | The `main.ts` branch only: ceiling **60 bytes gzip** from the ADR-011 reserve (2,142 after F-56). The engineer records the measured figure in ADR-011 item 4; above: stop and report |
| Lazy | `src/mine/` with `lib/mine` about 3,000 bytes gzip; owner chunk growth about 600. Above twice that: report |
| Upload entry | No code change |

## Verification (critical lane: QA at effort high)

| What | Check |
| --- | --- |
| AC 42, 52, 59 | Network view during upload, edit, publish and list: no request carries an entry field; no cookie; server diff touches only the shell list, Caddyfile and Caddy test |
| AC 53, 54 | Hand-written storage values: hostile title, `javascript:` and foreign links in every field, wrong types, wrong `v`, invalid JSON, truncated value |
| AC 48, P-12 | Not-found with the stored token removes; with another token for the same ID keeps |
| AC 52 | Storage blocked (browser setting), quota zero, getter throwing; upload, edit and publish still work |
| Mutants | Each must fail a check: link built from a stored string; title through `innerHTML`; removal on not-found without the token match; viewing page importing `lib/mine`; list page rewriting an unreadable value; upload page touching storage; draft expiry ignoring the 48 h |

## Open points

| # | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| P-12 | AC 48 says not-found "removes the entry for that workout". The 404 is the same for a wrong token (ADR-025), so a mistyped or someone else's link for a listed ID would erase a working edit link | Remove only when the stored token equals the address's (MW-3); built so unless the user objects | Nothing |
| P-13 | AC 42 "after a successful upload … the browser holds one entry": the edit page writes it after the upload navigates there, not the upload page. If that navigation never completes, no entry (the address bar still holds the link) | Accept; QA reads AC 42 after the edit page has loaded | Nothing |

## Hand-offs

| To | What |
| --- | --- |
| Main session | Owner becomes both (B-1 in MW-1, before the frontend step). P-12, P-13 to the user |
| Requirements | TD: Code structure (`my-workouts.html`, `src/mine/`, `lib/mine`), the storage rule (MW-7), the `lib/owner` token exit; F-11 U-10 unchanged |
| F-54 | MW-9: one text table, the `Intl.RelativeTimeFormat` language, sport names and dates through `lib/format` |
