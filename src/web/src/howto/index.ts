// The How to use dialog (F-39 HU-7; AP-22). LAZY: reached only through enhanceHowto's import().
// The frame is the shared shell (src/dialog, F-58 SE-6). Both wordings are in the markup; CSS picks
// one by the primary pointer, as the range hint does.

import { line, openDialog } from '../dialog';

/** [section, touch, mouse]; the same lines for every workout (OQ-126). */
const LINES: [string, string, string][] = [
  [
    'Values at a point',
    'Touch a chart and slide sideways to see the values at that point; on workouts with GPS, you can also drag along the route on the map.',
    'Point at a chart to see the values at that point; on workouts with GPS, you can also point along the route on the map.',
  ],
  [
    'Select a range',
    'Touch and hold a chart, then drag, to select a range and see its totals; Clear removes it.',
    'Drag across a chart to select a range and see its totals; Clear removes it.',
  ],
  ['Laps and splits', 'Tap a lap or split to select it as a range.', 'Click a lap or split to select it as a range.'],
  [
    'Fullscreen map',
    'On workouts with GPS, tap the fullscreen button on the map to enlarge it; tap it again or go back to leave.',
    'On workouts with GPS, click the fullscreen button on the map to enlarge it; click it again or press Esc to leave.',
  ],
  [
    'Units and theme',
    'Change units and theme at the top of the page; this device remembers your choice.',
    'Change units and theme at the top of the page; this device remembers your choice.',
  ],
  [
    'Zones',
    'Zones use the max heart rate and threshold power from the FIT file when it has them; the sharer can set their own on the edit page.',
    'Zones use the max heart rate and threshold power from the FIT file when it has them; the sharer can set their own on the edit page.',
  ], // F-49
  [
    'Share a range',
    'Select a range, then tap "Copy link to this selection" to share it.',
    'Select a range, then click "Copy link to this selection" to share it.',
  ], // F-40 SR-8
  [
    'Pins',
    'On workouts with pins, tap a pin on the map or in the list to show it on the route and charts; the buttons on the map show or hide each type.',
    'On workouts with pins, click a pin on the map or in the list to show it on the route and charts; the buttons on the map show or hide each type.',
  ], // F-56 PN-13
  [
    'Summary numbers',
    'Tap ⓘ on the summary to see how its numbers are computed.',
    'Click ⓘ on the summary to see how its numbers are computed.',
  ], // F-58 OQ-252
];

export function openHowto(doc: Document): void {
  // One line per section: its name, then the touch and the mouse wording.
  openDialog(doc, 'howto-dlg', 'How to use this page', LINES.map(([name, touch, mouse]) => line(doc, name, ['coarse', touch], ['fine', mouse])));
}
