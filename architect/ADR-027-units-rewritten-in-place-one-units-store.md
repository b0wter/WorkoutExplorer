# ADR-027 Units on the activity page: metric server markup rewritten in place from the data file's metadata; one units store

- **Status:** accepted (24 Oct 2026), with the F-14 design v1.0
- **Date:** 24 Oct 2026
- **Features:** F-14 (introduces it); F-15 (markup stays metric), F-17, F-18, F-19, F-25 (use the store and the formatters)
- **Supersedes:** none. Applies ADR-026 item 4 (adoption, enhancers through hooks) to units
- **Depends on:** [ADR-026](ADR-026-activity-page-template-adopted-not-hydrated.md), [ADR-008](ADR-008-shared-cursor-store.md) (store shape), [ADR-005](ADR-005-workout-data-file-format-v1.md) (metadata as written)
- **Requirements:** DS-1, DS-2; decision log questions 7 and 8

## Context

The summary and laps are server markup, written once at publish and served as one static file to every viewer, so they are metric (question 7). The viewer's units are known only in the browser (language default, then the drop-down). ADR-026 forbids re-rendering server markup and inline script, so the swap cannot happen before the first paint and must not replace elements. Three sources could feed the swap: the data file's metadata, raw numbers added to the markup, or the metric text itself.

## Decision

1. **Source: the decoded data file's metadata** (`sport`, `summary`, `laps`), the values the server built the markup from (as written, ADR-026 item 3). The bundle already loads it for the charts.
2. **One builder:** the swap runs `lib/page`'s summary and laps builders with the chosen unit system, the same code the committed samples hold equal to Go in metric.
3. **Text only, through hooks.** The enhancer writes strings into existing nodes, matched by the view model's `Key` strings and by position. It never adds, removes or replaces an element. A mismatch is a defect: the markup stays as it is.
4. **One units store per page** (`lib/units`), memory only, created by the page glue and passed to every consumer. No feature keeps its own unit state or reads the browser language.

## Consequences

- No second contract between Go and TypeScript, no number printed for machines in the HTML, and no change to the view model, the samples or F-15.
- **Cost:** an imperial viewer sees metric until the data file is decoded, not only until the bundle runs. The file is preloaded and loads in parallel with the bundle, and question 7 accepts a switch after first paint.
- **Cost:** if the data file fails to load, the page stays metric. Nothing else on the page then shows units, so the page is still consistent.
- The two builders join the counted entry (they were lazy, edit page only); their bytes are in F-14's share.

## Rejected

- **Raw values in data attributes.** Swaps before the data arrives, but adds a second machine-readable contract that Go and TypeScript must print byte-equal, and HTML on every page.
- **Converting the metric text.** Rounds twice, and makes a renderer compute (ADR-026 item 3).
- **Choosing units on the server** (`Accept-Language`). The page is one pre-compressed static file per workout; varying it breaks caching and adds request data the server does not otherwise use.

## Reversibility

Cheap: browser code only; pages are regenerated on deploy. No one-way door.

## Would choose differently if

- The Lighthouse run in `en-US` (F-13 verification) shows the swap delaying LCP or causing a shift. Then add raw values for the summary only, to swap as soon as the bundle runs.
