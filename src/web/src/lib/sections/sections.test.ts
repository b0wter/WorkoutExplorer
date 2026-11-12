// F-42 MS-1 to MS-3: the edge cases no page sample has; Go TestSections holds the same table.
import { describe, expect, it } from 'vitest';
import type { Lap, Session } from '../datafile';
import { lapSections, legs, sectionRanges, sections } from '.';

const s = (sport: string, start = 0, start_index = 0): Session =>
  ({ sport, sub_sport: '', start, end: start, start_index, end_index: start_index, totals: {} });
const l = (start_index: number): Lap => ({ start: 0, end: 0, start_index, end_index: start_index, trigger: '', totals: {} });
/** Go TestSections' notation: name[lap numbers]. */
const names = (sessions: Session[], laps: Lap[]) =>
  lapSections(sections({ sessions }), laps, laps.map((_, i) => [String(i + 1)]), 3)
    .map((x) => `${x.Name}[${x.Rows.map((r) => r[0]).join(' ')}]`)
    .join(' ');

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
  it('the span is the column count as a string; the first section starts at 0', () => {
    expect(lapSections(sections({ sessions: [s('running', 0, 2), s('cycling', 5, 5)] }), [l(0)], [['1']], 7)[0])
      .toEqual({ Name: 'Run', Span: '7', Rows: [['1']] });
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
