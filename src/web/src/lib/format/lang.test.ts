// F-54 LG-7, AC 49: numbers and dates follow the language; English stays as Go writes it; units never follow the language.
import { afterEach, describe, expect, it } from 'vitest';
import { langSource, resetI18n, type Lang } from '../i18n';
import { formatDistance, formatFixed, formatSpeedOrPace, formatTimeLeft, speedOrPaceUnit } from '.';
import { formatDate, formatLocalStart } from './dates';

const use = (l: Lang): void => langSource(() => l, () => {});
afterEach(resetI18n);
const at = Date.parse('2026-10-09T05:30:00Z');

describe('per language', () => {
  it('English: the fixed formats (byte-equal to Go)', () => {
    expect(formatFixed(12.345, 2)).toBe('12.35');
    expect(formatDate('2026-10-09T05:30:00Z', 7200)).toBe('9 Oct 2026');
    expect(formatTimeLeft(at + 6 * 86400e3, at)).toBe('Expires in 6 days');
  });

  it.each([
    ['de', '12,35', '9. Okt. 2026', 'Läuft in 6 Tagen ab'],
    ['es', '12,35', '9 oct 2026', 'Caduca dentro de 6 días'],
    ['fr', '12,35', '9 oct. 2026', 'Expire dans 6 jours'],
  ] as const)('%s: decimal comma, date order and month names, relative expiry', (l, fixed, date, left) => {
    use(l);
    expect(formatFixed(12.345, 2)).toBe(fixed);
    expect(formatFixed(-0.001, 1)).toBe('0,0');
    expect(formatFixed(1234, 0)).toBe('1234'); // no grouping, as in English
    expect(formatDate('2026-10-09T05:30:00Z', 7200)).toBe(date);
    expect(formatLocalStart('2026-10-09T05:30:00Z', { known: true, offset_seconds: 7200 })).toBe(date + ', 07:30');
    // Expiry texts come from the tables; without a loaded file they are English around the Intl part.
    expect(formatTimeLeft(at + 6 * 86400e3, at)).toContain(left.replace(/^(Läuft|Caduca|Expire) /, '').replace(/ ab$/, ''));
  });

  it('units stay with the unit system, not the language (OQ-227)', () => {
    use('de');
    expect(formatDistance(10_000, 'imperial')).toBe('6,21');
    expect(speedOrPaceUnit('cycling', 'imperial')).toBe('mph');
    expect(formatSpeedOrPace('running', 1000 / 300, 'metric')).toBe('5:00');
  });
});
