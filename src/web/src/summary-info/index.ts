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
import { lang } from '../lib/i18n';
import { t } from './texts';

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

const TRIM_ENDS: Record<string, 'trimmedStart' | 'trimmedEnd' | 'trimmedBoth'> = { start: 'trimmedStart', end: 'trimmedEnd', both: 'trimmedBoth' };
/** The pairs' keys (lib/page's avg/max cells), by Key: the labels are in the page language. */
const PAIRS = ['speed', 'heart_rate', 'power', 'cadence', 'temperature'];

const cell = (i: SummaryItem | undefined): string => (i ? i.Value + (i.Unit ? ' ' + i.Unit : '') : '');

/** `cells` is lib/page's summaryItems, passed in: imported here, it would split lib/format out of the entry. */
/** Opens over the button's strip; the trim string is the button's data attribute (SE-2). */
export function openSummaryInfo(button: HTMLElement, f: Facts, units: Units, cells: typeof summaryItems): void {
  const doc = button.ownerDocument;
  const trim = button.dataset.trim ?? '';
  const items = cells(f.sport, f.summary, units);
  const get = (k: string): SummaryItem | undefined => items.find((i) => i.Key === k);
  const ex = (text: string): readonly [string, string] => ['ex', text && t('example', { x: text })];
  const speed = (kmh: number): string => formatFixed(toSpeed(kmh / 3.6, units), 1) + (units === 'metric' ? ' km/h' : ' mph');
  // A label inside the example's sentence: lower case, but German nouns keep their capital.
  const low = (label: string): string => (lang() === 'de' ? label : label.toLowerCase());
  const transitions = f.sessions.filter((s) => s.sport === 'transition').length;
  const lines: Node[] = [];
  const add = (name: string, text: string, example = ''): void => void lines.push(line(doc, name, ['', text], ex(example)));

  const [dist, moving, elapsed] = [cell(get('distance')), cell(get('moving_time')), cell(get('elapsed_time'))];
  add(t('totalsName'), t('totalsText'), dist && elapsed ? t('distIn', { d: dist, t: elapsed }) : dist || elapsed);
  add(t('movingName'), t('movingText', { ride: speed(1), other: speed(0.5) }), moving && (elapsed ? t('movingOf', { moving, elapsed }) : moving));
  add(
    t('elapsedName'),
    t(transitions ? 'elapsedTransitions' : f.sessions.length > 1 ? 'elapsedLegs' : 'elapsedText'),
    elapsed && transitions ? t('elapsedIncl', { elapsed, n: transitions }) : '',
  );
  const avg = items.find((i) => PAIRS.includes(i.Key));
  if (avg)
    add(
      t('avgName'),
      t(transitions ? 'avgTextTransitions' : 'avgText'),
      avg.Max ? t('avgMaxExample', { label: low(avg.Label), value: cell(avg), max: avg.Max + (avg.Unit ? ' ' + avg.Unit : '') }) : t('avgExample', { label: low(avg.Label), value: cell(avg) }),
    );
  add(t('rangeLapsName'), t('rangeLapsText'));
  const o = f.owner;
  if (TRIM_ENDS[trim]) {
    let example = t(TRIM_ENDS[trim]);
    if (o) {
      const off = o.zone?.known ? o.zone.offset_seconds : 0;
      const utc = o.zone?.known ? '' : ' UTC';
      const at = o.trimAt.map((x, i) => x && t(i ? 'endsAt' : 'startsAt', { t: formatClock(x, off) + utc })).filter(Boolean);
      const kept = new Set(items.map((i) => i.Key));
      const dropped = items.length ? cells(f.sport, o.untrimmed, units).filter((i) => !kept.has(i.Key)).map((i) => low(i.Label)) : [];
      example =
        t('trimmedNow', { at: at.join(', ') }) +
        (items.length ? t('recomputed', { list: items.map((i) => low(i.Label)).join(', ') }) : '') +
        (dropped.length ? t('dropped', { list: dropped.join(', ') }) : '');
    }
    add(t('trimName'), t('trimText'), example);
  }
  const radius = (m: number, end: 0 | 1): string =>
    m ? t(end ? 'endHiddenWithin' : 'startHiddenWithin', { r: formatRadius(m, units) }) : t(end ? 'endNotHidden' : 'startNotHidden');
  add(t('hiddenName'), t('hiddenText'), o ? radius(o.radii[0], 0) + ', ' + radius(o.radii[1], 1) : '');
  const lengths = get('lengths');
  const pool = get('pool');
  if (pool || lengths || f.lengths.length)
    add(t('poolName'), t('poolText'), [lengths && t('poolLengths', { n: cell(lengths) }), pool && t('poolIn', { p: cell(pool) })].filter(Boolean).join(' '));
  if (f.zones) add(t('zonesInfoName'), t('zonesInfoText'));
  openDialog(doc, 'summary-dlg', t('numbersTitle'), lines, button.parentElement ?? undefined);
}
