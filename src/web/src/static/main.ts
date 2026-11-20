// The static entry (F-54 LG-5): the About and generic pages' fixed texts in the page language, by
// their `data-t` keys. Plain TypeScript with its own i18n core (wx-entry-copy). No drop-down
// (OQ-280); the head script holds the paint for at most 500 ms. The operator's part, the licence
// list and the Impressum carry no key and stay as written.
import { applyMarkup, startLang } from '../lib/i18n/load';

void startLang(['static'], document, undefined, () => applyMarkup(document));
