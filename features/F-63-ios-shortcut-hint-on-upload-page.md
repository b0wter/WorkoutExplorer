# F-63 iOS Shortcut hint on the upload page

- **Status:** planned (user idea, 22 Nov 2026, "in the future"); no build-order position; draft, open questions OQ-291 to OQ-295
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend first: fill at start; frontend: hint markup, texts, styling)
- **Lane:** small to normal (no new data, no new request; reuses the F-34 setting and the F-38 fill mechanism). The architect may skip a design if the fill is a copy of IM-7.
- **Requirements:** INF-5 (extended); UP-1, UP-8, DS-6, INF-1; NFR Privacy, NFR Performance
- **Depends on:** F-34 (setting, About section, Shortcut documentation), F-11 (upload page), F-38 (pages filled by `serve` at start, ADR-043), F-54 (four languages)

## Goal
A sharer on an iPhone or iPad who opens the upload page learns that the hoster offers a Shortcut for uploading straight from the share sheet, and can add it from there, without first finding the About page.

## Scope
- **Setting:** the existing `WORKOUT_EXPLORER_IOS_SHORTCUT_URL` (F-34, INF-5) also turns the hint on; no new variable (OQ-293). Same validation as today: an absolute `https://` link with a host and no user info, otherwise `serve` exits 2 naming the setting (OQ-292).
- **Unset:** the upload page is byte for byte as today: no hint, no empty element, no extra text in the page.
- **Set:** the upload page shows one short hint with the link, e.g. "On an iPhone or iPad? Upload from the share sheet with this Shortcut." (wording reviewed by the user on screen). Who sees it: OQ-291. Placement and states: OQ-294. Link target and About wording: OQ-295.
- **How the link reaches the page:** the upload page is already filled by `serve` at start and written to `{data}/site/` (F-38 IM-7, ADR-043), like the Impressum footer link. The hint is filled the same way, HTML-escaped, so it needs no script, no extra request and no CSP change; a changed setting takes effect at the next restart of `app`. The architect or engineer decides the details (marker, escaping, tests).
- **Texts** in English, German, Spanish and French (DS-6), through the existing upload-page translation keys. The link URL itself is not translated.
- **Privacy:** the page makes no request to Apple or any other third party; only a tap on the link leaves the site. The link carries no referrer (the page's `no-referrer` meta; `rel="noopener noreferrer"` if it opens a new tab). Nothing is stored or logged.
- **Documentation for hosters** (`deploy/RUNBOOK.md` settings row, `deploy/example.env` comment): the setting now shows the link on the About page **and** on the upload page; a short note that an iCloud Shortcut link (`https://www.icloud.com/shortcuts/…`) is the usual value.
- **Technical design:** the configuration row of `WORKOUT_EXPLORER_IOS_SHORTCUT_URL` names both places (engineering).

## Out of scope
- The Shortcut itself: building, hosting, signing or updating it stays the hoster's job (F-34 documentation). The project ships no Shortcut.
- Android or other platforms; a share target or PWA (discarded 14 Nov 2026).
- Detecting whether the Shortcut is already installed; remembering that the hint was dismissed (no new browser storage).
- Any API change, including `GET /api/status`.

## Constraints
- **Budgets (F-11 U-2, NFR Performance):** the upload entry stays under its 8,000-byte gzip ceiling; LCP of the upload page is unchanged. If OQ-291 is answered "iOS only", the detection is a few bytes in the existing entry, not a new chunk.
- **CSP (F-11 U-3, F-22):** no inline `<script>`, `<style>` or `style=`; the hint is plain HTML in the filled page.
- **Reliability:** a missing or malformed hint marker in the page source makes `serve` exit 1 before listening, as for the other filled markers (IM-7, P-10).

## Assumptions
- The hint is a plain link to the hoster's Shortcut; on iOS Safari it opens the Shortcuts app's "Add Shortcut" screen. Elsewhere it opens Apple's iCloud web page; that is acceptable.
- The About page's "Upload from your iPhone" section (F-34) stays as it is, apart from OQ-295.
- `make ci` covers the fill (set, unset, escaping) with tests; the look is checked by the user.

## Acceptance criteria
- [ ] Setting unset: the upload page served at `/` is identical to today's (test on the filled bytes).
- [ ] Setting an https link: the upload page shows the hint with that link, HTML-escaped (a value containing `"`, `<` or `&` cannot break out of the attribute or text; test).
- [ ] Setting `http://…`, a relative path or other text: `serve` exits 2 naming the setting (unchanged F-34 rule, still tested).
- [ ] The hint is shown to the audience decided in OQ-291, in the place and page states decided in OQ-294.
- [ ] The hint text exists in English, German, Spanish and French and follows the language drop-down (DS-6).
- [ ] Network log: loading the upload page with the setting on makes no request to any host other than the site's own; only tapping the link leaves the site, without a referrer.
- [ ] Nothing new is written to browser storage or cookies; nothing new is logged.
- [ ] The upload entry stays under 8,000 bytes gzip (CI line); the page passes the existing CSP with no change.
- [ ] `deploy/RUNBOOK.md` and `deploy/example.env` say the setting shows the link on the About page and the upload page; the technical design's configuration row says the same.
- [ ] About page text follows OQ-295.
- [ ] Checked by the user on an iPhone, in light and dark theme, portrait and landscape: the hint is readable, does not push the drop zone off the first screen, and tapping it opens the Shortcut.

## Open questions
- **OQ-291: Who sees the hint?** Suggested: everyone (no device detection). The page is filled at start and needs no script; detecting iPadOS is unreliable (Safari on iPad reports itself as a Mac); the wording ("On an iPhone or iPad? …") tells others it is not for them. Alternative: shown only on iOS and iPadOS by a small script check, at the cost of a script-dependent hint and an unreliable iPad case.
- **OQ-292: Stricter URL check?** Suggested: no, keep F-34's rule (any absolute `https://` link). A hoster may host the Shortcut elsewhere or behind a redirect; one rule for the About page and the upload page. Alternative: accept only `https://www.icloud.com/shortcuts/…` (catches typos, but blocks other hosting and changes the F-34 rule for existing hosters).
- **OQ-293: Same setting for both pages?** Suggested: yes, `WORKOUT_EXPLORER_IOS_SHORTCUT_URL` turns on the About section and the upload hint together. Alternative: a second on/off setting for the upload hint (more configuration for one line of text).
- **OQ-294: Where and when is it shown?** Suggested: one short line below the drop zone (above "My workouts"), in the start state and after an error; hidden while an upload runs and while uploads are paused (UP-8), since the Shortcut would be refused then too. Without JavaScript it is simply shown. Alternative: always visible, or in the footer next to About.
- **OQ-295: Link straight to the Shortcut, and an About sentence?** Suggested: the hint links straight to the Shortcut and adds "More" to the About section (`/about`, F-34); the About section gets one sentence that the link opens Apple's iCloud site, which then sees the visitor's IP address, and that the site sends nothing to Apple before the tap. Alternative: the hint links only to the About section (one more tap, but the explanation comes first).
