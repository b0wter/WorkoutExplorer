# ADR-056 Hiding steps: the boundary for every radius step, from the cut's own pass, stored for the edit page

- **Status:** accepted (19 Nov 2026, with the F-55 design)
- **Date:** 19 Nov 2026
- **Features:** F-55 (introduces it); F-09 (upload computes it), F-12 (settings object), F-16 (the rule), F-19 (shading), F-30 (regenerate), F-45 (owner suggestion), F-47 (per read with a trim), F-56 (pin check uses the same function)
- **Supersedes:** [ADR-030](ADR-030-to-be-hidden-ranges-computed-at-upload.md) items 2 and 3 (four range columns, `hide_ends_ranges`). ADR-030 items 1 and 4 (computed where the exact input exists; display only) carry over unchanged
- **Requirements:** SH-4, UP-4, UP-5; F-55 OQ-249 (shading follows the sliders before saving); invariant I-3

## Context

F-55 gives each end its own radius, 0 to 2,000 m in 50 m steps, and the edit page must shade the hidden sections for any slider position before saving. ADR-030 stored the ranges for one fixed radius; its "Would choose differently if" proposed computing them on `PATCH` once the radius became a setting, which cannot shade unsaved values. The exact rule still needs positions before quantisation and the gap-filling marks (F-16 H-3), neither of which is in `draft.bin`, so a rule run in the browser would be a second definition, off by up to three samples. A parse per edit-page load stays rejected for ADR-030's reasons (parse slot, no rate limit on owner endpoints).

## Decision

1. **One pass per end, every step.** A pure function in `workout` returns, for each radius 0, 50, …, 2,000 m (41 entries per end), the first (start) or last (end) sample whose position stays public, or `-1` when none does. Entry 0 is the window's first or last sample (radius 0 cuts nothing). The public cut takes its boundaries from this result for the record's radii; one Go function combines two entries (fully hidden when either is `-1` or start > end). No other code measures.
2. **Stored on the row** as one nullable column `hiding_steps` (encoding the engineer's choice), written by the upload and by `regenerate` as a derived column. `NULL`: no positions, or not computed yet (rows migrated to schema v9 until the deploy's `regenerate`). A `hiding_steps` of another length is read as not computed (QA-55-r1-2).
3. **Returned read-only** in the settings object as `hiding_steps`, `{"start": […], "end": […]}` in `draft.bin` indices, or `null`. With a trim stored, computed per read on the trimmed workout (ADR-047 item 3) and shifted into `draft.bin` indices; `-1` is not shifted.
4. **Display only.** The public build never reads the column; it runs item 1 on every build from `original.fit` (ADR-028 item 2). The browser looks up two entries and applies the one comparison of item 1, as a display mirror; it never measures. The owner's suggested title uses item 1's combination at the stored radii. Never logged, never outside the owner API.

## Consequences

- The shading matches the public cut sample for sample at every step, and the browser holds no measure.
- Cost: 82 integers per workout instead of 4, no coordinates, derived from data already in `draft.bin`, deleted with the row (AC-4).
- Cost: during a deploy that migrates to schema v9, existing workouts show no shading and no suggested town with a radius above 0 until `regenerate` reaches them.
- A later change to the rule leaves stale steps until `regenerate`, as ADR-030 noted; every deploy runs it.

## Would choose differently if

- Radii become free values rather than steps. Then return the monotone distance profile per end (each recorded fix that is farther from the end point than every fix before it, with its distance), from the same pass.
- Owner reads parse anyway for most workouts (for example if trims become the norm). Then compute the steps per read and drop the column.
