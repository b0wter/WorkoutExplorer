# F-28 Report form: architecture

- **Version:** 0.1
- **Status:** accepted (3 Nov 2026). Normal lane. OQ-103, OQ-104 open, not blocking (suggestions built unless the user objects)
- **Date:** 3 Nov 2026
- **Feature:** [`features/F-28-report-form.md`](../features/F-28-report-form.md); "AC n" is the n-th acceptance criterion there
- **Requirements:** INF-2, DS-3, NFR Privacy, NFR Compatibility; TD Front end (Implementation notes: footer, Report link); decision log 1 Nov 2026 (report limit fixed at 60 per hour), 3 Nov 2026 (no auto-hide, OQ-101: 11th report answered `204`)
- **Builds on:** [F-27](F-27-reports-api-and-operator-delete.md) R-5, the Responses table and its F-28 hand-off (this design replaces that hand-off's counter sentence, RF-4); [F-13](F-13-activity-page-layout.md) D-8; [F-19](F-19-edit-page-and-settings.md) (text rule of the description field); ADR-011 items 2 and 4, ADR-025 item 5, ADR-026 items 4 and 5, ADR-032 items 2 and 3
- **Owner:** frontend only. Template and `PageFooter.svelte` change; no Go code, view model or API change

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| RF-1 | **Where.** Only on the viewing page `/w/{public_id}` (the template's footer). Not on the edit page (`ActivityPage` keeps `report = false`), not on the owner's published page `/published/…`, not on the generic page (nothing to report, no script). The footer control becomes `<button type="button" data-report hidden>Report</button>` in `index.html` and `PageFooter.svelte` alike: it opens a dialog, so it is a button, not a link | INF-2, decision 21. A button is what assistive technology and the keyboard expect for "opens a dialog" |
| RF-2 | **Static part: one enhancer in `src/page`, run with the copy-link enhancer before any `await` in `startPage`**, so Report works when the data file fails or is slow (a broken or abusive workout is exactly what gets reported). It removes `hidden` and, on click, `import()`s the report chunk and opens the form. A failed import shows one fixed line beside the button (RF-8) and nothing else. **Static share: ceiling 300 bytes gzip** from the ADR-011 reserve, measured as the CI entry delta at F-28 start (about 56,900); above it, stop and report | ADR-011 item 2: no cost before use |
| RF-3 | **Lazy chunk `src/report/`, plain TypeScript and its own CSS, no Svelte.** It builds a native `<dialog>` once on first open, appends it to `body`, opens it with `showModal()` and reuses it for later opens. **Lazy ceiling: 3,000 bytes gzip** (JS plus CSS), reported in each QA round | Svelte runtime functions used by a lazy chunk whose module already sits in the entry are bundled into the entry (see the note at `src/web/src/edit/start.svelte.ts:7`); plain DOM keeps the static delta to RF-2's call site. `showModal()` gives focus move, Esc and Android back as close, inert background and focus return on close, with no code of ours |
| RF-4 | **Form shape and validation (mirror of F-27 R-5).** A fieldset "Reason" with four radio buttons, none preselected, `required`: Spam → `spam`, Illegal content → `illegal_content`, Someone else's personal data → `personal_data`, Other → `other`. A `<textarea>` "Message (optional)" with `maxlength="1000"`, line breaks allowed, and a counter in the **same unit as `maxlength`** (UTF-16 units), tied to the field by `aria-describedby`, not live. Before sending, control characters other than line feed become spaces, the F-19 description rule (one shared function, not a copy). The server trims and counts code points, so the browser's limit is never looser than the server's: a valid form never gets `400`. A note under the field: no contact details, reports get no reply. No name, email or other contact field (AC 4) | AC 2, AC 3. **Replaces** F-27's hand-off "counter counts code points": a counter in code points next to a `maxlength` in UTF-16 units stops before 1,000 on emoji. **Cost:** text of emoji stops a little before 1,000 visible characters |
| RF-5 | **Request.** `fetch('/api/reports', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body })` with `body` exactly `{"public_id", "reason", "message"}` (`message` may be `""`). `public_id` is the last segment of the page's own `ViewPath` as the template writes it (the copy link's `href`), never `location`; when it is missing or not 22 base62 characters, the Report button stays hidden. One request at a time: while sending, the send button is disabled and reads "Sending…" | F-27 R-5; `connect-src 'self'` already allows it (ADR-032) |
| RF-6 | **Privacy.** The request carries the body above and what the browser adds by itself, nothing else: no query string, no page URL or fragment, no units, theme or other preference, no time, screen or language field. `Referer` is already suppressed by the page's policy. Nothing about a report is kept in the browser beyond the open page: no `localStorage`, `sessionStorage` or cookie, no "already reported" flag; the text is in memory only. The message is never written to the console | NFR Privacy, I-7 on the client side |
| RF-7 | **Answers:** the table below. Every error keeps the reason and the text in the form (AC 5) and shows its sentence in an element with `role="alert"` inside the dialog | AC 2, AC 5; F-27 Responses |
| RF-8 | **Texts** (English, DS-3; wording is OQ-104): heading "Report this workout"; note "Please do not include contact details. Reports get no reply."; buttons "Send report", "Cancel", "Close"; confirmation "Thank you. The report has been sent."; fallback "The report could not be sent. Please try again later."; chunk failure "The report form could not be loaded. Reload the page and try again." Closing with Cancel, Esc or back keeps the entered text for the next open on the same page | One set of fixed sentences, no server text invented |
| RF-9 | **No JavaScript.** The button carries `hidden` in the markup, so viewers without JavaScript see no Report control (OQ-103). No `<noscript>` fallback and no HTML form post: the API accepts only `application/json` (F-27 R-5, its CSRF guard) | A dead link is worse than none; a form post would need a second, CSRF-exposed API path |
| RF-10 | **Phone and desktop (AC 1).** The dialog is centred with a max width on desktop and nearly full width below 1024 px, scrolls inside itself when the keyboard takes the space, and uses at least 16 px text in the textarea (iOS zooms the page on focus below that). Styles only from the chunk's CSS file, no inline style (ADR-032) | AC 1 without a layout change to the page |

**Answers (RF-7)**

| Answer | The viewer sees |
| --- | --- |
| `204` | The form is replaced by the confirmation and a Close button (`role="status"`); the form is cleared, so a later open starts empty. An 11th report on a workout is also `204` (OQ-101): same confirmation, no extra state |
| `400`, `404`, `429`, `500` with a JSON body that has a `message` string | That `message`, as is (`404`: "This workout has expired or is not available.", ADR-025 item 5; `429`: "Too many reports. Please try again later."). No `Retry-After` countdown |
| Any other status, or a body that is not that JSON (for example Caddy's `502` while Go is down) | The fallback sentence (RF-8) |
| Network failure (`fetch` rejects) | The fallback sentence |

## Verification

| AC | Automated (`make ci`, jsdom) | Manual (operator, through Caddy) |
| --- | --- | --- |
| 1 | Button unhidden by the enhancer, also when the data file fails; none on the edit page | Phone and desktop: opens, keyboard does not hide the send button, Esc and back close |
| 2 | No request without a reason; reason only gives `204` and the confirmation; body has exactly the three keys | One real report, row seen with R-7's query |
| 3, 4 | `maxlength` 1000, counter, control characters replaced; no contact field | — |
| 5 | Each row of RF-7's table, text and reason kept on every error | `429` by 61 reports in an hour; Go stopped: fallback sentence |
| RF-2, RF-3 | CI entry delta ≤ 300; chunk not loaded before the first click | No CSP violation in the console |

## Open questions

| ID | Question | Suggestion |
| --- | --- | --- |
| OQ-103 | INF-2 says every workout page has a Report link, but a report needs JavaScript (the API takes JSON only). Without JavaScript, show nothing, or a line "Reporting needs JavaScript"? | **Show nothing** (RF-9). The page without JavaScript has no charts or map either. Blocks nothing |
| OQ-104 | Wording of the dialog texts (RF-8) | As in RF-8; the user reviews them on screen. Blocks nothing |
