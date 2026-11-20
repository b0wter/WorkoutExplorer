// The How to use dialog (F-39 HU-7; AP-22). LAZY: reached only through enhanceHowto's import().
// The frame is the shared shell (src/dialog, F-58 SE-6). Both wordings are in the markup; CSS picks
// one by the primary pointer, as the range hint does.

import { line, openDialog } from '../dialog';
import { t, type en } from './texts';

type K = keyof typeof en;

/** [section, touch, mouse] keys; the same lines for every workout (OQ-126). */
const LINES: (readonly [K, K, K])[] = [
  ['pointName', 'pointTouch', 'pointMouse'],
  ['rangeName', 'rangeTouch', 'rangeMouse'],
  ['lapsName', 'lapsTouch', 'lapsMouse'],
  ['fullscreenName', 'fullscreenTouch', 'fullscreenMouse'],
  ['unitsName', 'unitsText', 'unitsText'],
  ['zonesName', 'zonesText', 'zonesText'], // F-49
  ['shareName', 'shareTouch', 'shareMouse'], // F-40 SR-8
  ['pinsName', 'pinsTouch', 'pinsMouse'], // F-56 PN-13
  ['numbersName', 'numbersTouch', 'numbersMouse'], // F-58 OQ-252
];

export function openHowto(doc: Document): void {
  // One line per section: its name, then the touch and the mouse wording.
  openDialog(doc, 'howto-dlg', t('howtoTitle'), LINES.map(([name, touch, mouse]) => line(doc, t(name), ['coarse', t(touch)], ['fine', t(mouse)])));
}
