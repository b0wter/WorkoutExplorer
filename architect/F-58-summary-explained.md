# F-58 How the summary is computed: architecture

- **Version:** 0.1
- **Status:** accepted (19 Nov 2026). Critical lane (privacy): QA at effort high for the hiding check only. P-11 decided 19 Nov 2026 as suggested
- **Feature:** [`features/F-58-summary-explained.md`](../features/F-58-summary-explained.md); decisions OQ-251 to OQ-260
- **Requirements:** AP-24; explains AP-2, AP-4, AP-10, AP-12, AP-13, AP-16, UP-6, SH-4, SH-9; reuses AP-22; DS-1, DS-4, NFR Performance, NFR Privacy
- **Builds on:** [F-39](F-39-how-to-use.md) HU-6, HU-7 (dialog, lazy chunk), [F-13](F-13-activity-page-layout.md) D-2, D-9, D-10, [F-16](F-16-start-and-end-hiding.md) H-1, H-5, H-6, [F-47](F-47-trim.md) TR-3, TR-7, TR-9, [F-15](F-15-publish-pipeline-and-static-page.md) P-4, ADR-011, ADR-026. No data file format change, no migration, no setting
- **Owner:** frontend, plus one small backend step (B-1, SE-2). The feature file says frontend; it becomes **both**

## Decisions

| # | Decision | Why |
| --- | --- | --- |
| SE-1 | **Viewing page: closed list of inputs.** The dialog on `/w/{id}` reads only: the public metadata's `sport`, `summary`, the sessions' sports, `lengths`, `zones`; the page's trim string (SE-2); the units store. It never reads stream descriptors or samples (positions included), `lib/route` or map state, the `Route.*` markup or hints, the title or suggested title (UP-5 follows hiding), or anything derived from positions. The viewing glue hands the chunk the metadata **without `streams`**, so the chunk cannot reach them; how is the engineer's choice | F-16 H-1, H-6: everything on the list is bit-identical with hiding off, on, at any radius, and for a fully hidden route. The text is then identical by construction (AC 39), not by care in wording. The only hiding-dependent public facts are positions, stream descriptors (fully hidden) and the title |
| SE-2 | **Trim fact travels in the markup.** One new view-model string `Trim`: `""`, `start`, `end` or `both`, from the row's `trim_start_at`, `trim_end_at` being non-null (TR-3 stores `NULL` for a side resolved to the first or last sample, so non-null means trimmed). Printed as a data attribute on the ⓘ button, never as visible text. Set by F-15's Go builder (B-1) and by `lib/page` on the edit page from the saved settings. One new trimmed sample (F-13 D-3) | The data file says nothing about trim (UP-6 "as if recorded that way"); a format bump plus every golden for a two-state fact only this dialog shows is not worth it (same choice as F-16 H-5). Independent of hiding, so H-6 holds. Trim changes are already remove-first (TR-7): no P-7 change. Would move it into the data file if a second consumer needed it |
| SE-3 | **Topics and their conditions** (OQ-254), each a general line plus this workout's example: totals, moving and elapsed time, ranges and laps versus totals: always. Averages: when the summary has one; the transition sentence only when a session's sport is `transition`. Trim: when `Trim` is set ("totals recomputed, calories dropped"). Pool: when the summary has a pool length or `lengths`. Zones: when the page starts the zones section (the test `src/page` already uses). **Hiding: on every workout, both pages**, like SH-9; on the viewing page the general line only, no example | Any condition on the hiding line ("has GPS", "has a map") would itself be a hiding signal for a fully hidden route, so it is the one deliberate exception to "only what the workout has" |
| SE-4 | **Edit page extras through the edit seam only.** From the **saved** settings (as the strip, TR-9): trim ends; start and end radius (0 shown as not hidden); with a trim, which strip totals were recomputed and which were dropped (keys of `trimmed.summary` against the draft summary). Sport and summary come from the edit glue's existing strip source (`src/web/src/edit/start.svelte.ts:152`); sessions, lengths and zones from `draft.bin`, as the rest of the edit page (OQ-178). The viewing entry has no path that passes owner facts. The published page (`/published/`, same edit seam) counts as the edit page. **F-55:** the radius text has one source; today `hide_ends` gives 300 m at both ends or 0, and F-55 swaps in its two values there | OQ-256. The owner may see everything; the seam keeps owner facts structurally off the viewing page |
| SE-5 | **The ⓘ button is template markup** inside `.summary` (and the edit page's summary component, F-13 D-9), `hidden` until enhanced, absolutely placed in the strip's top-right corner, which CSS reserves with and without JavaScript. Revealed once the data file is decoded (the example needs it); on a load failure there is none. `title` and accessible name "How these numbers are computed"; no accent, no stored state | OQ-251, OQ-257. No shift (F-13 D-10); markup costs no bundle bytes; `hidden` is fine here because the button takes no flow space |
| SE-6 | **One dialog shell, two lazy chunks** (OQ-252). F-39's shell (modal `<dialog>`, title, lines, Close, backdrop click, CSS) moves into one lazy module both dialogs import. The summary dialog is its own lazy chunk, reached by one `import()` from `src/page`; failed load gives the one line next to the button, as How to use. Content is rebuilt on every open (units and saved settings may have changed). `showModal`: no history entry. At 1024 px and wider (F-13 D-17 breakpoint) the dialog is placed over the strip when opened, kept inside the viewport, and does not follow scrolling; narrower, the centred dialog (OQ-258). The How to use dialog gets one line pointing to the ⓘ | One look, no duplicate code; neither dialog's text costs entry bytes |
| SE-7 | **Select and format, never compute** (ADR-026): values from the summary through `lib/format` and the units store (DS-1); themes through the existing tokens (DS-4). All texts sit in one place in the chunk, so F-54 translates them once (OQ-260) | One formatting home; no rule duplicated from Go |

## Cost

| What | Limit |
| --- | --- |
| Entry (static) | Enhancer, `import()` call site, corner CSS: ceiling **200 bytes gzip** from the ADR-011 reserve (2,824). The engineer records the measured figure in ADR-011 item 4; above 200: stop and report |
| Lazy chunk | About 3,000 bytes gzip JS and CSS, shared shell not counted (OQ-259); above: report |
| HTML, LCP | Button markup and one attribute; LCP unchanged (nothing new on the first-paint path) |

## Verification (critical lane: QA at effort high for the hiding check)

Synthetic files only (ADR-004).

| What | Check |
| --- | --- |
| AC 39 (hiding) | Over the synthetic samples (ride, triathlon, pool, indoor, zones, trimmed): publish through Go with hiding off, on, and a fully hidden route; the dialog's text on each viewing page is byte-identical. In jsdom, the same metadata with and without position descriptors gives identical text. Mutants that must fail a check: a topic or example conditioned on position streams, `routePresence`, the `Route.*` markup or the title; owner facts rendered without the edit seam |
| AC 40 (owner) | Edit and published page show radii (0 as not hidden), trim ends and the recomputed and dropped totals; no owner text appears on the viewing page |
| SE-2 | The Go builder reproduces the trimmed sample; `Trim` is equal with hiding on and off |
| AC 37, 38, 41, 42 | Topic matrix over the samples; entry ceiling; both themes and both unit systems |

## Hand-offs

| To | What |
| --- | --- |
| Main session, requirements | Owner becomes both: frontend writes `lib/page`'s `Trim`, the template attribute and the sample; backend B-1 makes F-15's Go builder reproduce it, in the same commit so `make ci` stays green |
| F-55 | Swap the radius source (SE-4). Re-run the AC 39 check with one end hidden and several radii, which only exist after F-55 |
| F-54 | Translate the chunk's texts (SE-7) |

## Open questions

| ID | Question | Suggestion | Blocks |
| --- | --- | --- | --- |
| P-11 | **Decided (user, 19 Nov 2026): accepted.** OQ-256 "which totals were recomputed": read as UP-6's recomputation after a trim (recomputed and dropped totals listed; untrimmed, none). Per-metric "device or computed" without a trim would need a new server flag, since the browser cannot tell (F-08 R-10 combines over legs) | Accept the trim reading (SE-4) | Wording of SE-4's owner example only |
