# F-11 Upload page: history

History of [architect/F-11-upload-page.md](../F-11-upload-page.md). Read it only when you need the history.

## Version 1.1 (30 Oct 2026, F-20 design v0.2): first-open marker removed (ADR-020 rev. 2)

- U-4: `lib/upload` held "the edit-link check and the marker".
- U-9 read: "The page appends the first-open marker `&new` (ADR-020 rev. 1) and calls `location.assign`, so Back returns to the upload page. `view_url` is not used: the edit page's Share button shows it (F-20)". Why: "AC-1 and F-20 need to know about the first open, and browser storage is ruled out (AC 8)."
- Hand-off F-19, F-20: "The edit page receives `/edit/{public_id}#{token}&new` once after upload and must remove the marker before it shows anything (ADR-020 rev. 1). The edit link that F-20 shows is the address without the marker".
- Verification AC 1: the jsdom check expected "the absolute link plus `&new`"; the manual pass expected `/edit/…#…&new`.
