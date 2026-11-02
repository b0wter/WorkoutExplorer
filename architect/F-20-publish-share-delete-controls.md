# F-20 Publish, Share and delete controls: architecture

- **Version:** 0.2 (published page, draft line, private state removed, QA-20-r1 folded in)
- **Status:** accepted (30 Oct 2026). Critical lane: delete is data loss, publish makes the workout public, the published page shows the edit link. OQ-86 resolved
- **History:** [history/F-20-publish-share-delete-controls.md](history/F-20-publish-share-delete-controls.md)
- **Date:** 30 Oct 2026
- **Feature:** [`features/F-20-publish-share-delete-controls.md`](../features/F-20-publish-share-delete-controls.md)
- **Requirements:** UP-4, UP-7 (draft line), SH-2, SH-6, SH-8, SH-10, AC-1, AC-4, AC-5, AC-6, UP-9; TD Publish and view flow (Publish 4 to 6)
- **Builds on:** [F-19](F-19-edit-page-and-settings.md) E-1, E-2, E-4 to E-7, E-10; [F-12](F-12-owner-api.md) endpoints, O-6, O-7, O-9, O-10; [F-15](F-15-publish-pipeline-and-static-page.md) P-1, P-7; [F-13](F-13-activity-page-layout.md) D-14; [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md) rev. 2; [ADR-020](ADR-020-workout-identifiers-edit-token-and-links.md) rev. 2; [ADR-025](ADR-025-owner-api-access-and-not-found-response.md); [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md); [ADR-011](ADR-011-activity-page-bundle-allocation.md) rev. 11. Decision log, 30 Oct 2026 (both blocks)
- **Owner:** both. **Backend first** (B-1: remove the built private state, Caddy route), then frontend (everything else). The frontend does not wait on any new endpoint; the order only means the hand check runs against a server without `private`
- **New ADR:** none

## Flow

```
upload ──▶ /edit/{id}#{token}   draft: settings, preview, draft line, Publish; no links
              │ Publish (save pending changes first; publish only if that save succeeded)
              ▼ 200
          /published/{id}#{token}   viewing link + Copy, edit link + Copy + warning,
              │                     Back to edit, Delete
              │ Back to edit                   reload: shown again; on a draft: replace → edit page
              ▼
          /edit/{id}#{token}   published: settings, Share, Copy link, download, Delete; no Publish
```

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| C-1 | **Code and serving.** All F-20 UI is in the owner chunk (`components/edit`, `src/edit`). `edit.html` is the page for both `/edit/*` and `/published/*`. **Caddy** serves `/published/*` in the same handle as `/edit/*` (`deploy/Caddyfile`, the `@owner` route for `/edit/*` and `/published/*`): rewrite to `edit.html`, `no-store`, `Referrer-Policy: no-referrer`; F-22 adds the same headers to both. **Dev:** the Vite `wx-edit-route` plugin maps `/published/*` to `edit.html` as it does `/edit/*`. The entry is unchanged (its `#wx-edit` marker loads the owner chunk). `lib/owner` reads the page kind from the path (`/edit/{id}` or `/published/{id}`; anything else is the generic page, F-19 E-2) and `src/edit` mounts either F-19's edit page or the published view. The published view fetches only the settings: no `draft.bin`, no `components/page`, map or charts. `lib/owner`'s client gains publish (`POST …/publish`), delete (`DELETE …`), the original (`GET …/original.fit`), and two navigations, to the published page and to the edit page, through F-19's single request function and `location` | The activity entry does not grow (ADR-011 item 2). One page file and one token module. The entry is already in the cache from the edit page. A separate plain-TypeScript entry (ADR-024 style) would need its own Delete, confirm and copy code |
| C-2 | **One owner action at a time.** Save, Publish, Delete and Download original share one busy state; while one runs, the others are disabled. A slow download therefore blocks writes until it ends or hits the server's download deadline (QA-20-r1-4: accepted, so a delete never runs during a download). Every settings object a save returns goes through E-6. Neither owner page reloads on `pageshow` with `persisted` (mobile WebKit fires it on app resume; a reload would drop unsaved changes): the published page does nothing, the edit page showing a draft re-fetches the settings and, if published meanwhile, shows the published state and ends a kept busy state without refilling the form, so Back never shows a stale draft | The server serialises writes (O-7). The client must never apply a late response after a delete, or show a draft that was published meanwhile |
| C-3 | **Publish** (UP-4, SH-2). Offered only on a draft, with **no confirmation** (user, 30 Oct 2026). With unsaved changes, it first sends them as one `PATCH` (E-6's diff) and publishes only after `200`; on any error it stops and shows it (OQ-85). Then `POST …/publish`. **`200`**: `lib/owner` navigates with `location.assign` to the published page; the busy state stays until the page unloads, and nothing re-renders. **`500`**: the message; Publish stays offered, and repeating it is safe (P-1; the accepted edge case leaves the row published with no page). **`404`**: the generic page (E-4) | E-9 shows the hiding marks before saving, so an owner expects them published. `assign` keeps the edit page in the history, so Back returns to it (and C-2 reloads it as published) |
| C-4 | **Share on the edit page** (SH-6). Shown whenever the workout is not a draft. It shows the viewing link as text, `location.origin` + `view_url` (ADR-020 item 8), never `location.href` or a path built in the browser, with a copy button. Copy and its fallback are F-13 D-14's code: a read-only, pre-selected field when the Clipboard API fails (safe, no token). The header Copy link uses the same condition (E-10) | `view_url` is the one home of the link, built on the server by `lifecycle` |
| C-5 | **Published page** (AC-1, AC-6, critical). It loads the settings only. A **draft** answer makes `lib/owner` call `location.replace` to the edit page (Back does not bounce). `404` and a malformed path or token: the generic page; `500`: the message (E-4). Otherwise it shows a heading, the viewing link with Copy (as C-4), the edit link with Copy and the save warning, "Back to edit", and Delete (C-7, C-8). No other workout data; the document title is the fixed "Workout Explorer". **Edit link:** `location.origin + '/edit/' + public_id + '#' + token`, exposed by `lib/owner` only on the published page (otherwise `null`). It is the **only** place the token is in the DOM: one text node, in an element with `translate="no"` (user, 30 Oct 2026), which also selects as one unit. Never an attribute (`href`, `title`, `aria-*`, `data-*`, `value`), an input, a link element, the title or a log line. Copy writes the in-memory string with the Clipboard API; when that fails, the page tells the owner to select the link and copy it by hand (no input fallback, unlike D-14). **Back to edit** is a button; `lib/owner` navigates with `location.assign`. The wording the user reviewed (warning, copy failure) stays. **Note (31 Oct 2026, user):** each box has a "Go there": a plain link to `view_url` in the viewing box; in the edit box it replaces "Back to edit" and stays the same button | AC-6. Keeping the token out of attributes and inputs keeps "one exception" checkable by scanning the DOM. `translate="no"` keeps browser page translation from handling the link |
| C-6 | **Download original** (SH-8, UP-9), edit page only, every state, whatever `allow_download` says (O-9). `fetch` through the owner client, body as a Blob, object URL (`blob:`, no token) on a temporary `<a download>`, clicked, removed, revoked later. File name from `Content-Disposition` (OQ-59), else `workout.fit`. Bytes neither read nor changed. Busy state as C-2. `404`: generic page; `500` or network failure: the message | ADR-025: the token header rules out a plain link. Up to 20 MB in memory for a moment is accepted |
| C-7 | **Delete** (AC-4), on the edit page **and** the published page (user, 30 Oct 2026), one component, the reviewed wording. Two steps in the page: Delete shows a confirmation (permanent, both links stop working) with Confirm and Cancel; never `window.confirm`. Cancel sends nothing. Confirm sends `DELETE`. `204`: the deleted screen (C-8). `404`: the generic page (deleted elsewhere, expired, or a lost earlier response). `500`: the message; the workout may still exist (O-10) and the owner may retry | Some in-app browsers suppress native dialogs, and a suppressed `confirm` counts as Cancel |
| C-8 | **Deleted screen**, on both pages: it replaces the body as E-4's generic page does: title "Workout Explorer", one fixed sentence, a link to `/`, About. No workout data in the DOM or the title (no title text, numbers, map, `public_id`, links to the workout). The address keeps the dead token; a reload shows the generic page | One teardown path (E-4) |
| C-9 | **SH-10 notice:** a fixed sentence always in the edit panel, draft and published, next to Share and Delete. No condition | A condition only adds states that can be wrong |
| C-10 | **Draft line** (UP-7): on a draft, the panel shows "Unpublished drafts are deleted 48 hours after upload." Fixed text, no computed time; not shown once published | The deadline itself is F-21's. A fixed sentence cannot drift from it |
| C-11 | **No private state** (SH-2). No visibility control (F-19 E-7), and no private branch in Share, Copy link or the published page. The client never sends `visibility` | Publishing cannot be undone. Since B-1 the server answers such a field with `400` |

**Module boundary** (changes to F-19's table):

| Module | Change |
| --- | --- |
| `lib/owner` | Gains publish, delete, the original, the page kind, the edit link (published page only) and the two navigations. Still the only module that reads `location`, the fragment or builds `Authorization`, and the only one that navigates to a URL with the token |
| `components/edit` | Gains the published view. Receives the edit link only as that view's input; never the token alone; never stores or logs it |

## B-1 Backend step (senior-backend-engineer, first)

Removal of the built private state, as decided in [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md) rev. 2, [F-12](F-12-owner-api.md) O-6, [F-15](F-15-publish-pipeline-and-static-page.md) P-7, P-8 and [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md) items 3 to 5: a new migration narrows `visibility` to `draft` and `unlisted` and drops `private` rows; `visibility` is read-only in the settings object; the allow-list keeps only draft → unlisted; nothing branches on private any more. The hides-more rule stays. Plus the Caddy route of C-1. Nothing is stored for F-21 (hand-off below).

## Budget

| What | Limit |
| --- | --- |
| Activity entry (CI line, ADR-002) | **F-20 share: 0 planned, ceiling 100 bytes gzip**, against 56,360 at F-20 start. Measured: ADR-011 ledger. v0.2 adds nothing static: the page kind is read in the lazy chunk. Above 100: stop and report |
| `edit.html` entry | The same counted set as the activity entry |
| Owner chunk (lazy, not counted) | **15,000 gzip.** 7,629 at QA r1. Reported in each QA round; above the limit, stop and report |

## Verification (critical lane: QA at effort high)

| What | Automated (`make ci`, ADR-012) | Manual (operator) |
| --- | --- | --- |
| Token (C-1, C-4 to C-6) | On the edit page, through load, save, publish, share-copy, download, delete and deleted states: the token is nowhere in the DOM. On the published page: exactly one text node holds it, inside the `translate="no"` element. Never in an attribute, an input's or textarea's value property, the title, storage, `history.state`, a request URL or the console. The download anchor's `href` is `blob:` on both kinds of open. Share and viewing-link copy are exactly `origin + /w/{public_id}`. F-19's source scan passes | Network log on desktop and phone: the token only in request headers. Page translation offered on the published page leaves the edit link untouched |
| Publish (C-2, C-3) | `PATCH` before `POST`; no `POST` after a failed `PATCH`; `200` navigates to `/published/{id}#{token}` with `assign`; `500` stays with Publish offered; the busy state covers all four actions | Publish a hiding-on ride, open `/w/…` in another browser |
| Published page (C-5) | Draft answer: `replace` to the edit page; `404` and a bad token: generic page; Back to edit: `assign` to the edit link | Through Caddy: reload shows it again; Back returns to an edit page showing the published state |
| Delete (C-7, C-8) | On both pages: Cancel sends nothing; the deleted screen has no title, description, numbers or `public_id`; `404` generic, `500` not | Both links show the generic page through Caddy (AC-4) |
| Draft line, no private (C-10, C-11) | The line on a draft only; no visibility control; no request carries `visibility` | — |
| B-1 | Migration on a database with draft, unlisted and private rows: private rows gone, others unchanged, `CHECK` refuses `private`; `PATCH` with `visibility` is `400`; recovery leaves no folder of a dropped row | — |
| Mutants | Each must fail a test: the token in the Share or viewing-link copy; the edit link on the edit page; the edit link in an attribute or input value; `translate="no"` dropped; publish without the pending `PATCH`; publish after a failed `PATCH`; navigation before the publish `200`; `assign` instead of `replace` on a draft; delete without confirmation; the title left on the deleted screen; download outside the busy state; the token in the download URL; a `private` row surviving the migration | — |

## Hand-offs

| To | What |
| --- | --- |
| F-21 | **Nothing to store now.** A draft's deadline is `min(created_at + 48 h, expires_at)` while `visibility = 'draft'`; all three are stored, and `created_at` never changes. F-21's design decides the sweep query and whether ADR-025's lookup treats an overdue draft as not found, as it does an expired row (suggested: yes, so the rule holds to the second like expiry) |
| F-22, F-31 | `/published/*` gets every header of `/edit/*` (CSP with C-6's `blob:` download and the Clipboard API, `X-Robots-Tag`); never `log_credentials`. Include both pages in F-22's browser check |
| F-30 | No private case remains (F-09 hand-off note) |
| Requirements (TD) | Once B-1 is built: drop the Open technical questions row "Removing the built private state" and the Data model note on `private`. Publish and view flow, first sentence ("The draft and the published page are the same page") now collides with the glossary's "Published page": reword to "draft and published workout" |

## Open questions (product; the user answers by number)

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| OQ-86 | AC-6 lets the owner open the published page again later, but nothing links to it: the owner must edit the address. Add a way from the edit page? | **Yes:** one "Show links" button in the Share block (published only) that opens the published page through `lib/owner` (a button, never an `href` with the token). Otherwise AC-6 holds only by typing the path | **Resolved 30 Oct 2026: accepted** (decision log). One button in C-4 |
