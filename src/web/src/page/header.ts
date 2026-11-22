// The viewing page's header in the page language (F-54 LG-4, LG-10). LAZY: loaded by enhanceUnits
// only once the page is not English, so English pages never carry dates, sport names or the device line.
import type { WorkoutMeta } from '../lib/datafile';
import { formatDate, formatLocalStart, sportName } from '../lib/format/dates';
import { deviceLine } from '../lib/page/device';
import { enhanceExpiry } from './enhance';

/** F-54 LG-4, LG-10: the start line, a default title (sport and date; a written or suggested title
 *  never), the expiry, the device line and the multisport leg rows, in the page language. */
export function header(root: ParentNode, meta: WorkoutMeta): void {
  const writes: [Node, string][] = [];
  const when = root.querySelector('.head .when');
  const h1 = root.querySelector('.head h1');
  if (when && h1) {
    const [sport, start = ''] = (when.textContent ?? '').split(' · ');
    const zone = meta.time_zone;
    const date = formatDate(meta.start_time, zone.known ? zone.offset_seconds : 0);
    if (h1.textContent === sport + ', ' + start.split(', ')[0]) {
      const doc = h1.ownerDocument;
      writes.push([h1, sportName(meta.sport) + ', ' + date]);
      if (doc.title === h1.textContent) writes.push([doc.querySelector('title') ?? h1, sportName(meta.sport) + ', ' + date]);
    }
    writes.push([when, sportName(meta.sport) + ' · ' + formatLocalStart(meta.start_time, zone)]);
  }
  const device = root.querySelector('.device');
  if (device) writes.push([device, deviceLine(meta.device)]);
  const legs = [...meta.sessions].sort((a, b) => a.start - b.start);
  let k = 0;
  const names = legs.map((s) => (s.sport === 'transition' ? 'T' + ++k : sportName(s.sport)));
  const rows = root.querySelectorAll('.laps-legs:not(.splits) h3'); // F-64 LL-3: the laps view's leg headings
  if (rows.length === names.length) rows.forEach((th, i) => writes.push([th, names[i]]));
  enhanceExpiry(root, Date.now());
  for (const [node, text] of writes) node.textContent = text;
}
