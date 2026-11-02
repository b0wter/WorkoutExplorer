# F-24 Viewer download of the published data

- **Phase:** 3 Later (deferred on 2 Nov 2026; design kept as written)
- **Owner:** backend
- **Requirements:** SH-8 (Should), NFR Security (originals never public); Technical design: Publish and view flow (Publish 5), Security and privacy (Originals)
- **Depends on:** F-16

## Goal
When the sharer allows it, viewers download a FIT file built only from the published, privacy-applied data.

## Scope
- `allow_download` off by default.
- When on: `export.{hash}.fit` written from the public streams (hidden positions removed) at publish and on every relevant change; a Download button appears on the public page (button slot in the F-13 template).
- Content kept minimal (assumed: no device serial number or sensor list; see README). The stored original has no serial numbers or other identifying data either (UP-9); the export is still built from the public streams, not copied from the original.
- Turning download off deletes the export file and removes the button.
- The original is never offered to viewers.

## Out of scope
- Owner's original download (F-12, F-20). Trim applied to the export (phase 2).

## Acceptance criteria
- [ ] New workout: no Download button on the public page and no export file on disk.
- [ ] Download on: the button downloads a FIT file that passes the F-08 validator and opens in a third-party FIT tool.
- [ ] With hiding on, the exported file has no positions in the hidden sections.
- [ ] The exported file is not byte-identical to the original and contains no serial number.
- [ ] Turning download off removes the button and the old export URL stops being served within 60 s.
