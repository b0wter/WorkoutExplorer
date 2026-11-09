# F-38 Impressum page

- **Phase:** 2
- **Owner:** both (backend first)
- **Lane:** normal
- **Requirements:** INF-7 (Should within phase 2), INF-1 (footer), DS-4 (theme), DS-3, NFR Privacy (`noindex`, no third party, no cookies); extends F-29 (operator-file pattern, design AB-8), F-13 and F-15 (footer), F-30 (`regenerate`), F-22 (headers), F-23 (indexing)
- **Depends on:** F-29, F-31, F-37 (its confirmation page gets the footer link)

## Goal
A hoster can publish an Impressum that readers find from every page, while scrapers that read only HTML and search engines don't get the hoster's personal data.

## Scope
- **Setting and file (backend).** One optional setting naming a plain-text file on the server, next to the operator file (F-29 AB-8 pattern): same three-rule format (`## ` heading, `- ` list, paragraphs), every character escaped, no links or markup; at most 16 KiB, valid UTF-8, no control characters except tab and line ends, at least one non-blank line (OQ-122). Read once at start. Set but missing, unreadable, too large or invalid: `serve` exits 2 naming the setting (OQ-122). Unset: no page, no text address, no footer link. A change takes effect at the next start. The hoster's file never enters the repository or the release; the repository may ship an example with prompts only (architect).
- **Page `/impressum`.** Heading "Impressum / Legal notice" and a button "Show Impressum". The HTML holds no Impressum text, and nothing is fetched on page load: only a click on the button makes the page's own script fetch the text from a separate address on the same site; the button is then replaced by the text, shown as escaped text in the operator-file format. Without JavaScript: no button, only a short note, e.g. "JavaScript is needed to show the Impressum." Load failure: "The Impressum could not be loaded. Reload the page." (OQ-124), with the button shown again to retry. Light and dark theme from the system setting, no drop-down, like the About page (DS-4). Rest of the page chrome in English (DS-3); the text is the hoster's, in any language.
- **Text address hardening.** The text address answers only requests from the site's own `/impressum` page (a same-origin check, e.g. `Sec-Fetch-Site: same-origin` or a matching `Origin`/`Referer`); any other request gets the site's not-found answer. It allows at most 10 requests per 10 minutes per IP address; beyond that it refuses until the window has passed, and the page shows the load-failure message. No new per-reader data is kept beyond the in-memory counter, which the server forgets after the window. This only raises the effort for scrapers: a scraper can fake the headers, run JavaScript or rotate addresses.
- **Kept from search engines.** The page and the text address carry `<meta name="robots" content="noindex, …">` (page) and `X-Robots-Tag: noindex` (both; already on every response, F-22). A `robots.txt` disallows `/impressum` and the text address; it never disallows `/w/` or anything else (OQ-121). The text address is not linked from any HTML except through the page's script.
- **Footer link.** "Impressum" in the footer of every page when set: upload, viewing (published workout), edit (draft and published), published page (AC-6), About, the delete confirmation page (INF-6, F-37), the generic "expired or not available" page and other error pages. Pages built by the server at start follow the setting after a restart; static published pages follow it after the next `regenerate` (OQ-120); the runbook says so.
- **Privacy and third parties.** No external service, no cookie, no consent needed, no new log of viewers; the text and page may be cached only as `/about` is (architect). Nothing new is stored per workout.
- **Docs.** Runbook: how to set the file, restart, run `regenerate`; that `/impressum` is only hard to crawl, not secret.
- Spec traceability in the technical design (architect): Configuration (new setting row), Hosting (Impressum page), Indexing (`robots.txt` now exists; "There is no `robots.txt`" changes), Repository layout (example file), footer line under Front end.

## Out of scope
- Whether an Impressum is legally required, and what it must contain: the hoster's call.
- Protection against scrapers that run JavaScript, fake request headers or rotate IP addresses, and against people copying the text. Hard to crawl, not secret.
- A scrambling font or images of text (rejected 9 Nov 2026).
- Editing the Impressum from the site; clickable e-mail or web links (OQ-122); a German interface.
- Mentioning the Impressum on the About page beyond its footer link (INF-1 unchanged).

## Acceptance criteria
- [ ] Setting unset: `/impressum` and the text address give the site's not-found answer; no page type has an "Impressum" footer link.
- [ ] Setting a valid file: `/impressum` shows the heading "Impressum / Legal notice" and the file's text with headings, lists and paragraphs as in the operator file; `<b>`, `<script>` and `{{` show as literal text.
- [ ] The HTML response of `/impressum` contains no part of the file's text (checked with a distinctive marker string in the file); only the text address returns it.
- [ ] Opening `/impressum` makes no request to the text address; a click on "Show Impressum" fetches it once and replaces the button with the text.
- [ ] A request to the text address without a same-origin signal (e.g. `curl` with no `Sec-Fetch-Site`, `Origin` or `Referer`, or with another site's) gets the not-found answer and no text; the page's own fetch succeeds.
- [ ] The 11th request to the text address from one IP address within 10 minutes is refused and returns no text; the page then shows the load-failure message; another IP address is unaffected; after the window, requests succeed again.
- [ ] With JavaScript off, `/impressum` shows the heading and the note that JavaScript is needed, and no Impressum text.
- [ ] With the text address failing (e.g. blocked), the page shows "The Impressum could not be loaded. Reload the page." and the button again.
- [ ] The text on the page can be selected and copied as plain text.
- [ ] `/impressum` and the text address both carry `X-Robots-Tag: noindex` (with `nofollow, noarchive` as every response); the page also has the robots meta tag.
- [ ] `robots.txt` disallows `/impressum` and the text address and nothing else; `/w/` stays allowed, so preview crawlers still fetch workout pages (F-23).
- [ ] With the setting set, every page type listed in Scope shows the "Impressum" footer link; after unsetting, restarting and running `regenerate`, none does.
- [ ] Setting a missing file, a directory, an unreadable file, a file over 16 KiB, invalid UTF-8, a file with control characters, or an empty or blank-only file: `serve` exits 2 naming the setting and does not start listening.
- [ ] The page sets no cookie and loads nothing from another site (CSP unchanged); it follows the system light or dark theme, also with JavaScript off.
- [ ] No file in the repository or the release contains a real Impressum; only the example with prompts.

## Decisions

Decided on 9 Nov 2026 (user, as suggested): OQ-120 static published pages take or lose the footer link on the next `regenerate`; OQ-121 one static `robots.txt`, always served, disallowing only `/impressum` and the text address; OQ-122 operator-file format and limits, a set but invalid or missing file exits 2; OQ-123 Should within phase 2; OQ-124 load-failure wording as in Scope. Added the same day: text loads only after "Show Impressum", same-origin check and per-IP rate limit on the text address.
