import { describe, expect, it } from 'vitest';
import { fixLanguage } from './locale';

describe('fixLanguage', () => {
  it('falls back to en for an invalid tag such as en-US@posix', () => {
    const nav = { language: 'en-US@posix' };
    fixLanguage(nav);
    expect(nav.language).toBe('en');
    expect(() => new Intl.NumberFormat(nav.language)).not.toThrow();
  });
  it('keeps a valid tag', () => {
    const nav = { language: 'de-AT' };
    fixLanguage(nav);
    expect(nav.language).toBe('de-AT');
  });
});
