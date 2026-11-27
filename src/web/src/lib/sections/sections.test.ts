// F-42 MS-1 to MS-3: the edge cases no page sample has; Go TestSections holds the same table.
import { describe, expect, it } from 'vitest';
import type { Lap, Session } from '../datafile';
import { lapsBySection, legs, sectionRanges, sections } from '.';
import { legSport, ordered, sportIn } from './order';

const s = (sport: string, start = 0, start_index = 0): Session =>
  ({ sport, sub_sport: '', start, end: start, start_index, end_index: start_index, totals: {} });
const l = (start_index: number): Lap => ({ start: 0, end: 0, start_index, end_index: start_index, trigger: '', stroke: '', totals: {} });
/** Go TestSections' notation: name[lap numbers]. */
const names = (sessions: Session[], laps: Lap[]) => {
  const secs = sections({ sessions });
  const ids = new Map(laps.map((x, i) => [x, String(i + 1)]));
  return lapsBySection(secs, laps)
    .map((ls, k) => `${secs?.[k].name}[${ls.map((x) => ids.get(x)).join(' ')}]`)
    .join(' ');
};

describe('sections', () => {
  it('one sport, or no laps: no rows', () => {
    expect(names([s('cycling'), s('cycling', 9, 5)], [l(0)])).toBe('');
    expect(names([s('running'), s('cycling', 9, 5)], [])).toBe('');
  });
  it('start order, ties in file order, T numbers', () => {
    expect(names([s('running', 20, 8), s('transition', 10, 4), s('swimming', 0, 3), s('transition', 20, 8)], [l(0)]))
      .toBe('Swim[1] T1[] Run[] T2[]');
  });
  it('a lap out of order stays under the separator above; trailing sections', () => {
    expect(names([s('swimming'), s('cycling', 5, 5), s('running', 9, 9)], [l(6), l(1)])).toBe('Swim[] Ride[1 2] Run[]');
  });
  it('the first section starts at 0', () => {
    expect(sections({ sessions: [s('running', 0, 2), s('cycling', 5, 5)] })?.[0].start).toBe(0);
  });
});

describe('ranges and legs (MS-2, MS-6)', () => {
  // Swim 0, T1 4, Ride 4 (no samples: T1 ties), Run 8, T2 12 (past N = 10: no samples).
  const secs = sections({ sessions: [s('swimming', 0, 3), s('transition', 4, 4), s('cycling', 4, 4), s('running', 8, 8), s('transition', 12, 12)] });
  it('a section runs to the next one\'s start (shared sample) or N − 1, never past it', () => {
    expect(secs?.map((x) => x.name)).toEqual(['Swim', 'T1', 'Ride', 'Run', 'T2']);
    expect(sectionRanges(secs ?? [], 10)).toEqual([0, 4, 4, 4, 4, 8, 8, 9, 12, 9]);
  });
  it('only sections with samples are legs', () => {
    expect(legs(secs ?? [], 10).map((g) => [g.name, g.start, g.end])).toEqual([
      ['Swim', 0, 4],
      ['Ride', 4, 8],
      ['Run', 8, 9],
    ]);
  });
});

describe('ordered and sportIn (F-67 LP-10)', () => {
  const sessions = [s('swimming', 0, 3), s('transition', 4, 4), s('cycling', 4, 4), s('running', 8, 8)];
  const secs = ordered({ sessions }) ?? [];
  it('the MS-2 order without names; null for one sport; sections() builds on it', () => {
    expect(ordered({ sessions: [s('cycling'), s('cycling', 9, 5)] })).toBeNull();
    expect(secs).toEqual([
      { start: 0, sport: 'swimming' },
      { start: 4, sport: 'transition' },
      { start: 4, sport: 'cycling' },
      { start: 8, sport: 'running' },
    ]);
    expect(sections({ sessions })?.map((x) => [x.start, x.sport])).toEqual(secs.map((x) => [x.start, x.sport]));
  });
  it('legSport: the leg of the points, else the workout sport (YD-8)', () => {
    const meta = { sessions, sport: 'cycling' };
    expect([legSport(meta, 1), legSport(meta, 5), legSport(meta, 0, 6), legSport({ sessions: [s('cycling')], sport: 'cycling' }, 1)]).toEqual(['swimming', 'cycling', 'cycling', 'cycling']);
  });
  it('a sample: the last section whose start ≤ it; a range: that section while it ends by the next start, else ""', () => {
    expect([0, 3, 4, 7, 8, 99].map((i) => sportIn(secs, i))).toEqual(['swimming', 'swimming', 'cycling', 'cycling', 'running', 'running']);
    expect(sportIn(secs, 0, 4)).toBe('swimming'); // the shared boundary sample, as the leg heading's range
    expect(sportIn(secs, 0, 5)).toBe('');
    expect(sportIn(secs, 4, 8)).toBe('cycling');
    expect(sportIn(secs, 5, 9)).toBe('');
    expect(sportIn(secs, 8, 99)).toBe('running');
  });
});
