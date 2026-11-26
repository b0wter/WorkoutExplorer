// uPlot builds `new Intl.NumberFormat(navigator.language)` when its module loads, and that throws
// a RangeError for a tag such as `en-US@posix`, leaving every page blank. main.ts imports this first
// so the tag is valid before uPlot (or anything else) reads it. Our own Intl calls use lang(), which
// is always en, de, es or fr.
export function fixLanguage(nav: { language: string }): void {
  try {
    Intl.getCanonicalLocales(nav.language);
  } catch {
    Object.defineProperty(nav, 'language', { value: 'en', configurable: true });
  }
}
fixLanguage(navigator);
