# F-58 How the summary is computed

- **Status:** draft (OQ-251 to OQ-260 open)
- **Category:** Planned (backlog scheme of 14 Nov 2026; no phase, no priority)
- **Owner:** frontend
- **Lane:** normal (critical only if OQ-253 lets the viewing page state hiding details)
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
- The content is general text with this workout's metrics filled in. It states nothing on the viewing page about this workout's hiding beyond what SH-9 already says (OQ-253).
- Closes like the how-to dialog (×, Esc, tap outside); opening it adds no history entry (as F-39 AC 4).
- Code and text load only when it is opened; entry bundle and LCP budgets unchanged (OQ-259).

## Out of scope
- Formulas or per-sample detail; editing values; a glossary of every chart.

## Acceptance criteria
- [ ] The summary strip has the trigger on the viewing page, and on the edit page if OQ-256 says so; without JavaScript there is none (OQ-257).
- [ ] Opening it shows the dialog in the how-to dialog's style, over the summary strip on wide screens and as the same full dialog on a phone; it closes with ×, Esc and a tap outside, and adds no history entry.
- [ ] Only topics for metrics and features this workout has are listed: no pool line on a ride, no transition line without multisport, no trim line on an untrimmed workout (OQ-254).
- [ ] On the viewing page, no text reveals whether, which end, or how much of the route is hidden, beyond the SH-9 notice (OQ-253).
- [ ] The dialog's code and text are not in the entry bundle; the entry bundle stays within its budget and LCP is unchanged.
- [ ] Both themes (DS-4); the units setting is used in any value shown (DS-1).

## Open questions
- [ ] **OQ-251 Trigger.** Suggested: a small "ⓘ" button at the summary strip's top-right corner, with the label "How these numbers are computed" as its tooltip only (no room for text); no accent colour, no "first visit" emphasis.
- [ ] **OQ-252 Shared component.** Suggested: one dialog component for both, the how-to dialog and this one; one line added to the how-to dialog pointing to the ⓘ.
- [ ] **OQ-253 General or this workout's state.** Suggested: general text, filtered to the topics this workout has (OQ-254); on the viewing page never "this workout's start was hidden (500 m)", since that tells a viewer how far the home is from the visible start; trimmed or not may be stated (it reveals nothing about places).
- [ ] **OQ-254 Which topics.** Suggested: only those this workout has (multisport, pool, zones, trim on the edit page); totals, moving and elapsed time, and range/laps always.
- [ ] **OQ-255 Device total or computed, per metric.** Suggested: not per metric now ("where the device recorded no total, the server computed it"); per-metric marks need the data file to say which, a later idea.
- [ ] **OQ-256 Edit page.** Suggested: yes, the same trigger, plus this workout's actual state for the owner (trim set, start and end radii, which totals are recomputed); the owner may see it.
- [ ] **OQ-257 Without JavaScript.** Suggested: no trigger, as the how-to button (AP-22).
- [ ] **OQ-258 Position.** Suggested: anchored over the summary strip on wide screens (two columns), the normal centred dialog on phones.
- [ ] **OQ-259 Budget.** Suggested: lazy chunk shared with the how-to dialog's text, at most about 3 KB gzip extra; entry bundle gains only the button (under 200 bytes).
- [ ] **OQ-260 Build order and translation.** Suggested: build F-58 right after F-53 and before F-54, so its texts are translated with everything else; otherwise F-54 must cover them anyway.
