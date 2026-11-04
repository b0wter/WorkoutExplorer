# F-34 iOS Shortcut upload link

- **Phase:** 2
- **Owner:** both (backend first)
- **Requirements:** INF-5 (Could), INF-1, UP-1, UP-10, AC-1; extends F-29
- **Depends on:** F-29, F-31

## Goal
A hoster can offer iPhone users a Shortcut that uploads a FIT file from the share sheet and opens its edit page.

## Scope
- Backend: an optional setting with an https link to the hoster's Shortcut. Unset: nothing changes. A value that is not an https link: `serve` exits 2 naming the setting, like the other settings. When set, `serve` fills an "Upload from your iPhone" section into the About page at start, the same way as the Pushover block (F-29 design).
- Frontend: the section in the About page template, shown only when filled.
- Documentation (runbook or `README.md`, for every hoster): how to build the Shortcut against the existing upload API: receive files from the share sheet; `POST /api/workouts` with multipart field `file`; on an error, show the `message` from the JSON (429 carries `Retry-After`); on success, open base URL + `edit_url` in Safari. The edit link is a secret: the Shortcut opens it and does not log, show, save or share it otherwise.
- No API change.

## Out of scope
- A Shortcut shipped by the project; Android; a share target or PWA built into the app (Later).

## Acceptance criteria
- [ ] Setting unset: the About page has no "Upload from your iPhone" section.
- [ ] Setting an https link: the section shows that link, escaped, in light and dark theme.
- [ ] Setting `http://…`, a relative path or other text: `serve` exits 2 naming the setting.
- [ ] The documentation describes the Shortcut steps above, including the error message, 429, opening `edit_url` and the warning about the edit link.
- [ ] A Shortcut built from the documentation uploads a FIT file from the iOS share sheet and opens its edit page (user check on an iPhone).
