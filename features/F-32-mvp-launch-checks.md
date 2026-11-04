# F-32 MVP launch checks

- **Phase:** 1 MVP
- **Owner:** both
- **Requirements:** NFR Performance (all three), NFR Compatibility, NFR Privacy (GDPR, data-processing record), NFR Reliability, SH-5 (messenger check), SH-10; Roadmap gate 1
- **Depends on:** F-08 to F-33, except F-24 (phase 3); F-34 is phase 2

## Goal
Verify the whole MVP end to end on the live server before the operator and friends start using it for real workouts.

## Scope
- End-to-end run of every corpus file: upload, edit, publish, view, delete.
- Performance on the live site: LCP, scrubbing on the 12-hour file with map loaded, data size.
- Browser matrix from the compatibility requirement, including the Android Chrome part of F-11 AC 1 (upload by file picker).
- Messenger preview check (SH-5).
- SH-10 timings on the live server.
- Privacy checks: no cookies, CSP, originals unreachable, no coordinates in logs, nothing left after delete and expiry.
- Operator writes the GDPR data-processing record (a document, not software).

## Out of scope
- Fixes found here are done in the owning feature.

## Acceptance criteria
- [ ] Every corpus file (including indoor, GPS dropout, long pause, multi-session, truncated, 12-hour) uploads, publishes and views without a broken page; non-activity and garbage files are refused with their messages.
- [ ] LCP under 2.5 s on a mid-range phone over 4G (reference phone and conditions as in the spec's Performance requirements) for a 4-hour ride.
- [ ] 60 fps scrubbing on the 12-hour file on the operator's phone on the live site.
- [ ] Android check: the page loads and scrubs smoothly on an Android phone (not checked before F-32).
- [ ] Upload on Android Chrome (F-11 AC 1, moved here): picking a valid `.fit` file with the file picker opens the new draft's edit link.
- [ ] Workout page, upload, edit and About pages work in the last two versions of Chrome, Safari, Firefox and Edge, iOS Safari and Android Chrome.
- [ ] Preview cards render in WhatsApp, Signal, Slack, Telegram and X; draft links show no route or numbers.
- [ ] Delete, expiry and hiding-on each stop old content within 60 s on the live server.
- [ ] The data-processing record exists.
- [ ] The About page's operator placeholder is gone: the operator file answers every prompt of `deploy/operator.example.txt`, including what the TLS proxy logs.
- [ ] Gate 1 starts: the operator and friends use the site for real workouts for a few weeks.
