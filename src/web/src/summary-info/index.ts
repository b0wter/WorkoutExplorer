// "About these numbers" (F-58; AP-24). LAZY: reached only through enhanceSummaryInfo's import().
// Content is rebuilt on every open (SE-6). Select and format, never compute (SE-7): values are the
// summary's, through lib/format and the page's units. All texts sit in this file (F-54, OQ-260).
//
// PRIVACY (SE-1, AC 39): on the viewing page every input is in `Facts`, which holds nothing that
// depends on hiding (no streams, positions, route state or title). The hiding topic is on every
// workout and, without `owner`, has no example: neither may depend on anything here.

import { line, openDialog } from '../dialog';
import type { SummaryItem, summaryItems } from '../lib/page';
import { formatFixed, toSpeed, type Units } from '../lib/format';
import { formatClock, formatRadius } from '../lib/format/dates';
import type { Summary, TimeZone } from '../lib/datafile';

/** The public facts (SE-1): the metadata's sport, summary, sessions' sports and lengths, and
 *  whether the page starts its zones section. */
export interface Facts {
  readonly sport: string;
  readonly summary: Summary;
  readonly sessions: readonly { readonly sport: string }[];
  readonly lengths: readonly unknown[];
  readonly zones: boolean;
  /** The edit page's extras (SE-4), from the saved settings; never on the viewing page. */
  readonly owner?: OwnerFacts;
}

export interface OwnerFacts {
  /** Start and end radius in metres; 0 = not hidden. */
  readonly radii: readonly [number, number];
  /** RFC 3339 of the first and last remaining sample; null = that end not trimmed. */
  readonly trimAt: readonly [string | null, string | null];
  readonly zone: TimeZone | undefined;
  /** The draft's (untrimmed) summary: what the trim dropped is what it has and the strip lacks. */
  readonly untrimmed: Summary;
}

const TRIM_ENDS: Record<string, string> = { start: 'at the start', end: 'at the end', both: 'at the start and the end' };

const cell = (i: SummaryItem | undefined): string => (i ? i.Value + (i.Unit ? ' ' + i.Unit : '') : '');

/** `cells` is lib/page's summaryItems, passed in: imported here, it would split lib/format out of the entry. */
/** Opens over the button's strip; the trim string is the button's data attribute (SE-2). */
export function openSummaryInfo(button: HTMLElement, f: Facts, units: Units, cells: typeof summaryItems): void {
  const doc = button.ownerDocument;
  const trim = button.dataset.trim ?? '';
  const items = cells(f.sport, f.summary, units);
  const get = (k: string): SummaryItem | undefined => items.find((i) => i.Key === k);
  const ex = (text: string): readonly [string, string] => ['ex', text && 'This workout: ' + text + '.'];
  const speed = (kmh: number): string => formatFixed(toSpeed(kmh / 3.6, units), 1) + (units === 'metric' ? ' km/h' : ' mph');
  const transitions = f.sessions.filter((s) => s.sport === 'transition').length;
  const lines: Node[] = [];
  const add = (name: string, text: string, example = ''): void => void lines.push(line(doc, name, ['', text], ex(example)));

  const [dist, moving, elapsed] = [cell(get('distance')), cell(get('moving_time')), cell(get('elapsed_time'))];
  add('Totals', 'The values the device recorded for the whole workout; where it recorded none, the server computed them from the samples.', [dist, elapsed].filter(Boolean).join(' in '));
  add(
    'Moving time',
    `Taken from the device's pauses where it recorded them, otherwise the time faster than ${speed(1)} on a ride or ${speed(0.5)} in other sports; pauses count as stopped.`,
    moving && (elapsed ? `${moving} moving of ${elapsed} elapsed` : moving),
  );
  add(
    'Elapsed time',
    'From the start to the last record, including pauses' + (transitions ? ', transitions and gaps between legs.' : f.sessions.length > 1 ? ' and gaps between legs.' : '.'),
    elapsed && transitions ? `${elapsed}, including ${transitions} transition${transitions > 1 ? 's' : ''}` : '',
  );
  const avg = items.find((i) => /^(Avg|Max) /.test(i.Label));
  if (avg)
    add(
      'Averages and maximums',
      'The device\'s values for the whole workout, or the server\'s where the device recorded none.' +
        (transitions ? ' Heart rate includes the transitions; speed, cadence and power do not.' : ''),
      avg.Label.toLowerCase() + ' ' + cell(avg) + (avg.Max ? ', max ' + avg.Max + (avg.Unit ? ' ' + avg.Unit : '') : ''),
    );
  add('Range panel and laps', 'Computed from the recorded samples, so they can differ slightly from the device\'s totals.');
  const o = f.owner;
  if (TRIM_ENDS[trim]) {
    let example = 'trimmed ' + TRIM_ENDS[trim] + ', totals recomputed';
    if (o) {
      const off = o.zone?.known ? o.zone.offset_seconds : 0;
      const utc = o.zone?.known ? '' : ' UTC';
      const at = o.trimAt.map((t, i) => t && (i ? 'ends ' : 'starts ') + formatClock(t, off) + utc).filter(Boolean);
      const kept = new Set(items.map((i) => i.Key));
      // An empty strip means the trimmed values are unavailable (TR-9 read failure), not dropped (QA-58-r1-3).
      const dropped = items.length ? cells(f.sport, o.untrimmed, units).filter((i) => !kept.has(i.Key)).map((i) => i.Label.toLowerCase()) : [];
      example = `trimmed (now ${at.join(', ')})` + (items.length ? `; recomputed: ${items.map((i) => i.Label.toLowerCase()).join(', ')}` : '') + (dropped.length ? `; dropped: ${dropped.join(', ')}` : '');
    }
    add('Trim', 'After a trim, the totals are recomputed for the remaining part; calories are dropped.', example);
  }
  // Every workout, both pages (SE-3); the example only with the owner's facts (OQ-253).
  const radius = (m: number, end: string): string => (m ? `${end} hidden within ${formatRadius(m, units)}` : `${end} not hidden`);
  add('Hidden start and end', 'The sharer may hide the route near the start and end; that changes the map, never the totals.', o ? radius(o.radii[0], 'start') + ', ' + radius(o.radii[1], 'end') : '');
  if (get('pool') || get('lengths') || f.lengths.length)
    add('Pool swims', 'Lengths count the active lengths only; rests are not lengths.', [get('lengths') && cell(get('lengths')) + ' lengths', get('pool') && 'in a ' + cell(get('pool')) + ' pool'].filter(Boolean).join(' '));
  if (f.zones) add('Zones', 'The line under the zones says where their limits come from: the device, an estimate, or the sharer\'s own values.');
  openDialog(doc, 'summary-dlg', 'About these numbers', lines, button.parentElement ?? undefined);
}
