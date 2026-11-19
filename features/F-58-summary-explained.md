# F-58 How the summary is computed

- **Status:** decided (14 Nov 2026); first after F-60 (build order of 19 Nov 2026), before F-54
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** both (backend B-1 first: page view-model `Trim`, architect F-58 SE-2)
- **Lane:** critical (privacy): the viewing page now shows this workout's state, and a careless example could reveal how much is hidden, e.g. by comparing the visible route length with the total distance. Keep QA at effort high for that check only.
- **Requirements:** AP-24 (new); explains AP-2, AP-4, AP-12, AP-16, AP-10, UP-6, SH-4, SH-9; reuses the AP-22 dialog
- **Depends on:** F-39 (how-to dialog), F-13 (summary strip); before F-54 (OQ-260)

## Goal
A viewer who wonders why a number differs from their watch, or from a selected range, can read how the summary values come about: device totals or computed values, moving time, transitions, trim and hiding.

## Scope
- A second dialog, styled like the "How to use this page" dialog (AP-22), titled e.g. "About these numbers", opened from the summary strip (trigger: OQ-251) and positioned over it on wide screens (OQ-258).
- One short line or block per topic, only for what the workout shows (OQ-254):
  - **Totals:** the values the device recorded; computed on the server where the device recorded none (AP-2).
  - **Moving time:** counted while the speed is above a small threshold; pauses count as stopped (rule as built, F-02).
  - **Elapsed time:** from the workout start to the last record, including pauses and, in multisport, transitions and gaps.
  - **Averages and maximums:** what they include, for example transitions in multisport (AP-12): heart rate yes, speed, cadence and power no.
  - **Range panel and laps:** values computed from the samples, so they can differ from the device's totals (OQ-24, OQ-79).
  - **Trim:** after a trim the totals are recomputed for the remaining part (UP-6); calories are dropped.
  - **Hidden start and end:** hiding changes the map, not the totals (SH-4, SH-9).
  - **Pool swims:** lengths count active lengths only (AP-16, AP-13).
  - **Zones:** where to find the source line (AP-10).
- The content is the general description of each topic plus this workout's actual state as an example (OQ-253), e.g. "This workout: trimmed at the start, totals recomputed, moving time 1:42 of 1:55 elapsed".
- **Viewing page:** the example never states whether, which end, or how far the start or end is hidden, and shows no value from which that follows (no visible route length next to the total distance, no hidden time or distance). Everything else may be shown: trim, device or computed totals in general, multisport, pool, zones source. Every value in the example comes from the public data only.
- **Edit page:** the same button and dialog; the example also shows the start and end radii and which totals were recomputed (OQ-256).
- Closes like the how-to dialog (×, Esc, tap outside); opening it adds no history entry (as F-39 AC 4).
- Code and text load only when it is opened; entry bundle and LCP budgets unchanged (OQ-259).

## Out of scope
- Formulas or per-sample detail; editing values; a glossary of every chart.

## Acceptance criteria
- [ ] The summary strip has the ⓘ button on the viewing and edit pages; without JavaScript there is none (OQ-257).
- [ ] Opening it shows the dialog in the how-to dialog's style, over the summary strip on wide screens and as the same full dialog on a phone; it closes with ×, Esc and a tap outside, and adds no history entry.
- [ ] Only topics for metrics and features this workout has are listed: no pool line on a ride, no transition line without multisport, no trim line on an untrimmed workout (OQ-254).
- [ ] The dialog shows each topic's general line and, where the workout has a value for it, this workout's example; on a trimmed workout the example says so and that the totals were recomputed.
- [ ] On the viewing page, for workouts with neither, one and both ends hidden at several radii, the dialog's text is identical apart from values that do not depend on hiding: no text or value reveals whether, which end, or how much of the route is hidden, beyond the SH-9 notice (OQ-253).
- [ ] On the edit page, the example shows the start and end radii (0 shown as not hidden).
- [ ] The dialog's code and text are not in the entry bundle; the entry bundle stays within its budget and LCP is unchanged.
- [ ] Both themes (DS-4); the units setting is used in any value shown (DS-1).

## Decisions (user, 14 Nov 2026)
- **OQ-251:** a small "ⓘ" button at the summary strip's top-right corner, tooltip "How these numbers are computed"; no accent colour, no first-visit emphasis.
- **OQ-252:** one dialog component shared with the How to use dialog; the How to use dialog gets one line pointing to the ⓘ.
- **OQ-253 (changed):** general description plus this workout's actual state as an example; on the viewing page never whether or how far start or end are hidden (a radius roughly reveals how far home is from the visible start); everything else may be shown.
- **OQ-254:** only topics this workout has; totals, moving and elapsed time, and ranges and laps versus totals always.
- **OQ-255:** not per metric; one general line on device and computed totals.
- **OQ-256 (merged with OQ-253):** on the edit page the owner also sees the radii and which totals were recomputed.
- **OQ-257:** no button without JavaScript.
- **OQ-258:** anchored over the summary strip on wide screens; the normal centred dialog on phones.
- **OQ-259:** lazy, about 3 KB gzip extra at most; the entry bundle gains only the button (under 200 bytes).
- **OQ-260:** built before F-54, so its texts are translated once (after F-53 until the build order of 19 Nov 2026 put F-58 first).
