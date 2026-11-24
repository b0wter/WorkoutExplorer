// The page markup's texts as the edit page's components render them (F-54 LG-3, LG-4). The same
// keys mark the viewing page's template (index.html `data-t`), whose English must be equal.
import { texts } from '../../lib/i18n';

export const en = {
  copyLink: 'Copy link',
  howto: 'How to use this page',
  summaryInfo: 'How these numbers are computed',
  privacyNotice: 'Parts of the route, such as the start and end, may be hidden for privacy. Totals include them.',
  routeHidden: 'The entire route has been hidden by the uploader.',
  headLaps: 'Laps',
  headSplits: 'Splits',
  lapsNotePace: "* Time includes pauses; pace is the device's own average, usually over moving time only.",
  lapsNoteSpeed: "* Time includes pauses; speed is the device's own average, usually over moving time only.",
  about: 'About',
  report: 'Report',
};

export const t = texts(en);
