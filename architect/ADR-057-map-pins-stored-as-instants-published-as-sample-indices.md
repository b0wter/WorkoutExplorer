# ADR-057 Map pins: stored as sample instants in one row column, published as sample indices in a content-hashed file

- **Status:** accepted (20 Nov 2026, with the F-56 design); rev. 1 (20 Nov 2026, F-56 v0.2): items 3 and 5 for pins a radius hides; old text in [history](history/ADR-057-map-pins-stored-as-instants-published-as-sample-indices.md)
- **Date:** 20 Nov 2026
- **Features:** F-56 (introduces it); F-12 (settings object), F-15 (public build, P-7), F-30 (regenerate reads, never writes), F-21 (deleted with the row)
- **Supersedes:** the TD's sketch of a `pins` table (`id`, `workout_id`, `type`, `lat`, `lon`, `start_s`, `end_s`, `title`, `note`, `created_at`, `updated_at`), "exported to `pins.json` on every change"
- **Depends on:** [ADR-019](ADR-019-sqlite-store-schema-and-migrations.md), [ADR-028](ADR-028-public-files-rebuilt-whole-page-as-commit-point.md), [ADR-047](ADR-047-trim-cut-of-the-normalised-workout.md) item 2, [ADR-056](ADR-056-hiding-steps-for-every-radius.md)
- **Requirements:** PIN-1, PIN-3, PIN-4, PIN-7, PIN-8, SH-4, SH-10, UP-6, UP-7, AC-4; invariant I-3

## Context

A pin must never reveal a hidden or trimmed place (PIN-7), at each end's own radius, and must stay right when the owner later raises a radius, changes the trim, or a deploy re-parses the original (`regenerate`). Pins are saved as a set with the edit page's Save (OQ-274), at most 50. The viewing page needs them on the map, the charts and the list, and must load nothing when there are none.

## Decision

1. **Storage: one nullable column `pins` on `workouts`** (schema v10, forward-only; the migration is the one-way door). It holds the whole set, written only by Go after validation, in a canonical order (start instant, then point before section, then end instant, then the order sent), so equal sets are equal rows. `NULL` = no pins; an empty set is stored as `NULL`. Per pin: type, title, note, start instant, end instant or none. No coordinates, no per-pin ID, no timestamps. The encoding is the engineer's choice.
2. **Anchor: instants of samples**, Unix seconds, as the trim (ADR-047 item 2): an index moves when a parser change alters gap filling, an offset moves when R-1's start time changes, an instant stays at the same moment. Every use resolves an instant to the **nearest sample of the normalised workout, ties to the earlier**; the server stores the resolved sample's instant.
3. **The public rule is the public build's own output.** A point pin is public exactly when its sample lies in the trim window and has a position in the workout after trim and hiding (`publicTransform`). A section is public exactly when both ends are; since hiding and trim only cut the ends, the whole section then lies inside the visible route. A section that touches a hidden or trimmed part is **never clipped**. What happens to a pin that is not public depends on the cause: outside the trim window it is **deleted** from the row (the trim is a cut of the recording, ADR-047); kept in the trim window but hidden by a radius it **stays stored and unpublished**, and becomes public again when a lower radius uncovers it. New or moved anchors must be public. One module computes both predicates; the owner `PATCH`, the trim removal and the build all call it.
4. **Public artefact: `pins.{hash}.json`** in the workout's folder, written by the whole build (ADR-028 item 2) only when at least one pin is public. It holds type, title, note and **sample indices into the data file of the same build**, never a coordinate or an instant. `index.html` links it through the view model (`Pins.URL`), which is also the viewing page's only "has pins" signal. Not in the data file (a format bump, and a pin edit would rewrite and recompress the streams); not in the page (owner text in the first-paint HTML, a second list renderer in Go).
5. **Every pin change that alters the public projection is remove-first; one that does not causes no rebuild.** `pins` stays off F-15 P-7's allow-list. An edit can delete a note's text or move a pin away from a place; telling those from additions would need a per-pin diff for a save that costs about a second of the generic page. Comparing the projection (the pins file content) is cheap and keeps a change to unpublished pins from showing even as a page swap.

## Consequences

- The row may hold pins no public file shows; their existence, number and text reach only the owner API and edit page.
- No public file holds a pin coordinate that is not already a public position of the data file it references; positions, distances and times on the viewing page come from that data file.
- A pin can never outlive its row (AC-4, `secure_delete`), and its texts sit in one column the F-21 database scan can check.
- Cost: an owner `PATCH` that sends pins, or changes a radius or the trim of a workout with pins, parses the original once under the O-7 lock (as a trim `PATCH` does, F-47 TR-4).
- Cost: two tabs that both edit pins: the later Save wins for the whole set (as for the title).
- Cost: viewers of a published workout see the generic page for about a second during each pin save.
- After a parser change, a pin may resolve to a neighbouring sample; if that sample is not public, the pin silently leaves the public file until the owner edits it. Accepted: workouts are short-lived (UP-7).

## Would choose differently if

- Pins become editable one at a time through the API, or shared across workouts: then a child table with `ON DELETE CASCADE` (ADR-019 rev. 4).
- Pin saves become frequent enough that the remove-first second is noticed: put "pins only added" (the new set contains the old one, byte for byte) on the allow-list.
- Privacy zones or another mid-route cut arrive: "both ends public" is then no longer enough; a section must also contain no cut sample.
