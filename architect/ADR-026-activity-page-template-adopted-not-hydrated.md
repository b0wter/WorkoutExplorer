# ADR-026 Activity page template: Go `html/template` authored as the activity entry, server markup adopted not hydrated, strings formatted before the template, no inline CSS

- **Status:** accepted (24 Oct 2026); rev. 1 (24 Oct 2026): item 4 names bundle-only controls, item 5 the layouts ([history](history/ADR-026-activity-page-template-adopted-not-hydrated.md))
- **Date:** 24 Oct 2026
- **Features:** F-13 (introduces it); F-15 (executes the template), F-14, F-17, F-18, F-19, F-23, F-24, F-28 (build on the markup and the view model); F-22, F-31 (serving)
- **Supersedes:** none. Refines TD Front end, Load order step 1 ("HTML with inline critical CSS") and Publish and view flow, View step 1; settles the front-end half of TD Open technical questions, "Template location and how Go and Vite share it"
- **Depends on:** [ADR-005](ADR-005-workout-data-file-format-v1.md) (metadata), [ADR-011](ADR-011-activity-page-bundle-allocation.md), [ADR-014](ADR-014-map-tiles-and-third-party-privacy.md) (CSP `style-src 'self'`, `script-src 'self'`)
- **Requirements:** AP-1, AP-2, AP-8, SH-9, NFR Performance (LCP, summary before charts), DS-1; review decision 30 (`features/README.md`)

## Context

The TD puts the summary and laps into a static HTML page that Go writes at publish, and has the bundle take over in the browser. Four facts constrain how:

1. Go has no JavaScript runtime, so it cannot run Svelte's server renderer. Svelte 5's `hydrate()` needs Svelte's own markup, including its internal comment markers. If they don't match, Svelte throws the markup away and mounts again, which is the re-render and shift F-13 forbids.
2. The page text includes title and description, which are user input (UP-4). A template engine that escapes by context is the cheapest way to get AC 5 right.
3. ADR-014's CSP has `style-src 'self'` and `script-src 'self'`. An inline `<style>` or `<script>` needs a hash in the header. The hash changes with every CSS edit, and kept workouts (ADR-005 item 9) keep older pages, so the header would have to list every live hash.
4. The same numbers are formatted in Go (static page, preview card) and in TypeScript (edit page, units, range panel, splits). Review decision 30 accepts that and asks for a test that keeps them equal.

## Decision

1. **Template format and ownership.** The template is a Go `html/template` file. Its source is `src/web/index.html`, the activity entry of the Vite build. Vite inserts the content-hashed script and stylesheet tags. `build/web/index.html` is the deployable template that the server executes, and it is never served raw. The frontend writes the template; the backend executes it (F-15). The template uses only field access, `if`, `range` and `with`. It has no template functions and no `template.HTML` values, and it is executed with `missingkey=error`. Every printed value is a string.
2. **The page view model is the template's only input,** with every string already formatted. Its contract is a set of committed sample view models, produced by the TypeScript builder (`lib/page`) from the synthetic golden data files (ADR-005 item 10) plus fixed settings. The Go builder (F-15) must reproduce them exactly. A logic-free template then gives the same page from protoserve (a decoded JSON map) and from F-15 (a struct).
3. **Renderers select and format; they never compute.** Every number on the page comes from the data file's metadata as written (rounded values, F-03 §5.2), on both sides. A value the page needs but the metadata lacks is added to the data file by the server (for example AP-8's computed lap values), not derived by a renderer.
4. **Adoption, not hydration.** The client never re-renders or replaces server markup. Svelte mounts with `mount()` only into empty, size-reserved slots (map, charts, range panel). Behaviour on server markup is attached by small framework-free enhancers through hook attributes. On the edit page, Svelte components render the same markup from the same view model, and the same enhancers attach to it. A control that is useless without JavaScript (the map pin toggle, F-13 D-19) is never in the template: the client adds it inside a slot it owns, as an overlay that changes no box until the viewer uses it.
5. **No inline CSS or script.** The page has one render-blocking stylesheet, the entry CSS from our origin, plus the module script. *Exception (3 Nov 2026, DS-4): one byte-constant theme script in the head, allowed by its hash (ADR-032 item 3).* Sizes are reserved with classes, never with `style` attributes. This replaces the TD's "inline critical CSS". The page's layouts (one column, two columns, F-13 D-17) are chosen by media queries and a template class, never by script, so the first paint is already final.

## Consequences

- (31 Oct 2026, F-23 PC-6, proposed) Exception to item 2: the `Card` group (Open Graph values) is built in Go only; the samples carry a fixed placeholder for it and the Go sample comparison skips it, since no TypeScript renderer shows it.
- No Svelte internals in Go. A Svelte upgrade cannot break the static page.
- AC 5 rests on the standard library's contextual escaping. Title and description are never trusted HTML on either side (no `template.HTML`, no `{@html}`).
- **Cost:** the static modules (header, summary, laps) are written twice, as template markup and as Svelte components for the edit page. Both are loops over the view model with no logic, and each is tested against the samples. No test compares the HTML of the two across languages. If they drift, only the owner's edit page looks different, and F-19 QA would see it.
- **Cost:** formatting rules exist in Go (metric only) and TypeScript. The samples hold them equal (decision 30).
- **Cost:** one extra request for the CSS on a first visit. It goes over the same HTTP/2 connection, starts in parallel with the bundle, and is cached for a year. LCP stays a text block from the HTML (canvas is never an LCP candidate), so the budget of 2.5 s keeps a wide margin (F-13 verification).
- The lap computation (decision 3) moves to the server's normalisation. A lap row and the range panel for the same lap may differ slightly, as the summary and a whole-workout range may (OQ-24).
- Dev convenience: `vite dev` shows the raw template. Sample pages come from protoserve (F-13 design, B-1).

## Reversibility

Cheap. Every page is regenerated on deploy (TD Versioning), so the template, the view model and the adoption model can change in one release. No one-way door.

## Would choose differently if

- A Go-side renderer for Svelte becomes available without a JavaScript runtime, or Svelte documents its hydration markup as stable. Then hydrating the static modules would remove the second rendering. Revisit in a superseding ADR.
- Lighthouse shows the stylesheet request costing LCP on the reference profile (more than about 300 ms of the budget). Then inline the critical rules with a hash, and solve the hash list for kept workouts in F-22.
- The edit page's static modules drift from the template in practice (F-19 QA findings). Then add a golden HTML comparison of the two renderers.
