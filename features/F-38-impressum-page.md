# F-38 Impressum page

- **Phase:** 2
- **Owner:** both (backend first)
- **Lane:** normal
- **Requirements:** INF-7 (new, Should within phase 2, assumed), INF-1 (footer), DS-4 (theme), DS-3, NFR Privacy (`noindex`, no third party, no cookies); extends F-29 (operator-file pattern, design AB-8), F-13 and F-15 (footer), F-30 (`regenerate`), F-22 (headers), F-23 (indexing)
- **Depends on:** F-29, F-31, F-37 (its confirmation page gets the footer link)

## Goal
A hoster can publish an Impressum that readers find from every page, while scrapers that read only HTML and search engines don't get the hoster's personal data.

## Scope
- **Setting and file (backend).** One optional setting naming a plain-text file on the server, next to the operator file (F-29 AB-8 pattern): same three-rule format (`## ` heading, `- ` list, paragraphs), every character escaped, no links or markup; at most 16 KiB, valid UTF-8, no control characters except tab and line ends, at least one non-blank line (OQ-122). Read once at start. Set but missing, unreadable, too large or invalid: `serve` exits 2 naming the setting (OQ-122). Unset: no page, no text address, no footer link. A change takes effect at the next start. The hoster's file never enters the repository or the release; the repository may ship an example with prompts only (architect).
- **Page `/impressum`.** Heading "Impressum / Legal notice". The HTML holds no Impressum text; the page's own script fetches the text from a separate address on the same site and shows it as escaped text in the operator-file format. Without JavaScript: only a short note, e.g. "JavaScript is needed to show the Impressum." Load failure: "The Impressum could not be loaded. Reload the page." (OQ-124). Light and dark theme from the system setting, no drop-down, like the About page (DS-4). Rest of the page chrome in English (DS-3); the text is the hoster's, in any language.
- **Kept from search engines.** The page and the text address carry `<meta name="robots" content="noindex, …">` (page) and `X-Robots-Tag: noindex` (both; already on every response, F-22). A `robots.txt` disallows `/impressum` and the text address; it never disallows `/w/` or anything else (OQ-121). The text address is not linked from any HTML except through the page's script.
- **Footer link.** "Impressum" in the footer of every page when set: upload, viewing (published workout), edit (draft and published), published page (AC-6), About, the delete confirmation page (INF-6, F-37), the generic "expired or not available" page and other error pages. Pages built by the server at start follow the setting after a restart; static published pages follow it after the next `regenerate` (OQ-120); the runbook says so.
- **Privacy and third parties.** No external service, no cookie, no consent needed, no new log of viewers; the text and page may be cached only as `/about` is (architect). Nothing new is stored per workout.
- **Docs.** Runbook: how to set the file, restart, run `regenerate`; that `/impressum` is only hard to crawl, not secret.
- Spec traceability in the technical design (architect): Configuration (new setting row), Hosting (Impressum page), Indexing (`robots.txt` now exists; "There is no `robots.txt`" changes), Repository layout (example file), footer line under Front end.

## Out of scope
- Whether an Impressum is legally required, and what it must contain: the hoster's call.
- Protection against scrapers that run JavaScript, and against people copying the text. Hard to crawl, not secret.
- A scrambling font or images of text (rejected 9 Nov 2026).
- Editing the Impressum from the site; clickable e-mail or web links (OQ-122); a German interface.
- Mentioning the Impressum on the About page beyond its footer link (INF-1 unchanged).

## Acceptance criteria
- [ ] Setting unset: `/impressum` and the text address give the site's not-found answer; no page type has an "Impressum" footer link.
- [ ] Setting a valid file: `/impressum` shows the heading "Impressum / Legal notice" and the file's text with headings, lists and paragraphs as in the operator file; `<b>`, `<script>` and `{{` show as literal text.
- [ ] The HTML response of `/impressum` contains no part of the file's text (checked with a distinctive marker string in the file); only the text address returns it.
- [ ] With JavaScript off, `/impressum` shows the heading and the note that JavaScript is needed, and no Impressum text.
- [ ] With the text address failing (e.g. blocked), the page shows "The Impressum could not be loaded. Reload the page."
- [ ] The text on the page can be selected and copied as plain text.
- [ ] `/impressum` and the text address both carry `X-Robots-Tag: noindex` (with `nofollow, noarchive` as every response); the page also has the robots meta tag.
- [ ] `robots.txt` disallows `/impressum` and the text address and nothing else; `/w/` stays allowed, so preview crawlers still fetch workout pages (F-23).
- [ ] With the setting set, every page type listed in Scope shows the "Impressum" footer link; after unsetting, restarting and running `regenerate`, none does.
- [ ] Setting a missing file, a directory, an unreadable file, a file over 16 KiB, invalid UTF-8, a file with control characters, or an empty or blank-only file: `serve` exits 2 naming the setting and does not start listening.
- [ ] The page sets no cookie and loads nothing from another site (CSP unchanged); it follows the system light or dark theme, also with JavaScript off.
- [ ] No file in the repository or the release contains a real Impressum; only the example with prompts.

## Open questions

Suggestions are assumed until the user decides:

- **OQ-120 Footer link on static published pages.** Published workout pages are static files built at publish; they cannot follow the setting on their own. *Suggested:* they take the link (or lose it) on the next `regenerate`, which the runbook tells the hoster to run after a change; the deploy already runs it (F-30). Alternative: always include the link and accept a not-found answer while unset (rejected: the user asked for no link when unset).
- **OQ-121 `robots.txt`.** *Suggested:* one static file, always served, disallowing only `/impressum` and the text address (naming them reveals nothing when unset). Alternative: served only when set (needs the server to build it). Either way `/w/` stays allowed, as decided 10 Oct 2026 (question 1).
- **OQ-122 File rules.** *Suggested:* the operator file's format and limits (16 KiB, UTF-8, three rules, escaped, no clickable links); a setting that names a missing file exits 2 instead of showing a placeholder, since a placeholder Impressum would be wrong on a live site.
- **OQ-123 Priority.** *Suggested:* Should within phase 2 (for a hoster in Germany it may be legally needed).
- **OQ-124 Load failure.** *Suggested:* "The Impressum could not be loaded. Reload the page."; the user reviews all wording on screen.
