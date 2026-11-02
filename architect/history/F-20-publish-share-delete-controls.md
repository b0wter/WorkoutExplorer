# F-20 Publish, Share and delete controls: history

History of [architect/F-20-publish-share-delete-controls.md](../F-20-publish-share-delete-controls.md). Read it only when you need the history.

## Version 0.2 (30 Oct 2026): published page, draft line, private state removed, QA-20-r1 folded in

Product changes: decision log of 30 Oct 2026 (edit link after upload; private state removed). Superseded text of v0.1, in short:

- **Owner:** "frontend only … No backend step". Now both, backend first (B-1 removes the built private state).
- **C-2:** "Save, Publish and Delete share one busy state". The built code also locks the original download (QA-20-r1-4); v0.2 records it.
- **C-3:** after a successful publish "the page stays (no navigation or reload), the panel shows the unlisted ↔ private control (E-7), and Share and the header Copy link appear". Now publish navigates to the published page; no private control.
- **C-4:** Share "shown only while the workout is unlisted (OQ-83)"; "After a successful publish the Share block opens by itself." Now: shown whenever not a draft.
- **C-5 First-open notice:** shown only when `firstOpen` (F-19 E-3, `&new` marker, ADR-020 rev. 1) at the top of the panel; `lib/owner` exposed the edit link only on a first open; the notice stayed for the visit (OQ-84). Replaced by the published page (AC-6). Its token rules (one text node, never an attribute or input, Clipboard API only, select-by-hand fallback) carry over to v0.2 C-5.
- **Budget:** owner chunk "F-20 is expected to add about 2,000 to 3,000" (measured 1,649 at r1).
- **Open questions:** OQ-83 (Share hidden while private: superseded, no private state), OQ-84 (no close button on the notice: superseded, no notice), OQ-85 (save first, then publish: kept as decided, now in C-3).
- **QA-20-r1-1** (no notice after a failed first settings request): obsolete, the notice and the marker are gone.
- **C-2 (v0.2):** both owner pages reloaded on `pageshow` with `persisted`. Replaced: reloads on app resume on iPhone (Orion/WebKit) could drop unsaved changes.
