// The How to use dialog's table (F-54 LG-3; F-39 HU-7, AP-22): per topic its name, the touch and the mouse wording.
import { texts } from '../lib/i18n';

export const en = {
  howtoTitle: 'How to use this page',
  pointName: 'Values at a point',
  pointTouch: 'Touch a chart and slide sideways to see the values at that point; on workouts with GPS, you can also drag along the route on the map.',
  pointMouse: 'Point at a chart to see the values at that point; on workouts with GPS, you can also point along the route on the map.',
  rangeName: 'Select a range',
  rangeTouch: 'Touch and hold a chart, then drag, to select a range and see its totals; Clear removes it.',
  rangeMouse: 'Drag across a chart to select a range and see its totals; Clear removes it.',
  lapsName: 'Laps and splits',
  lapsTouch: 'Tap a lap or split to select it as a range.',
  lapsMouse: 'Click a lap or split to select it as a range.',
  fullscreenName: 'Fullscreen map',
  fullscreenTouch: 'On workouts with GPS, tap the fullscreen button on the map to enlarge it; tap it again or go back to leave.',
  fullscreenMouse: 'On workouts with GPS, click the fullscreen button on the map to enlarge it; click it again or press Esc to leave.',
  unitsName: 'Units, theme and language',
  unitsText: 'Change units, theme and language at the top of the page; this device remembers your choice.',
  zonesName: 'Zones',
  zonesText: 'Zones use the max heart rate and threshold power from the FIT file when it has them; the sharer can set their own on the edit page.',
  shareName: 'Share a range',
  shareTouch: 'Select a range, then tap "Copy link to this selection" to share it.',
  shareMouse: 'Select a range, then click "Copy link to this selection" to share it.',
  pinsName: 'Pins',
  pinsTouch: 'On workouts with pins, tap a pin on the map or in the list to show it on the route and charts; the buttons on the map show or hide each type.',
  pinsMouse: 'On workouts with pins, click a pin on the map or in the list to show it on the route and charts; the buttons on the map show or hide each type.',
  editName: 'Add a lap or pin',
  editTouch: 'For sharers: on the edit page, select a point or range, then tap "+" to add a lap or pin.',
  editMouse: 'For sharers: on the edit page, select a point or range, then right-click the map or use "+", to add a lap boundary or pin.',
  numbersName: 'Summary numbers',
  numbersText: 'The "How these numbers are computed" bar, under the "How to use this page" bar, explains the summary\'s numbers.',
};

export const t = texts(en);
