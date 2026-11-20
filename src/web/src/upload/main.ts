// Upload page entry (F-11 design U-1, U-2; ADR-024). Plain TypeScript: no Svelte. F-54: the page's
// language first (the head script holds the paint for at most 500 ms), then the page.
import './upload.css';
import { start } from './page';
import { addLangSelect } from './lang';
import { onLanguage } from '../lib/i18n';
import { applyMarkup, startLang } from '../lib/i18n/load';

const markup = (): void => applyMarkup(document);
onLanguage(markup);
void startLang(['upload'], document, undefined, markup).then(() => {
  addLangSelect(document, () => window.localStorage);
  start(document, window, {
    fetch: (input, init) => window.fetch(input, init),
    xhr: () => new XMLHttpRequest(),
    navigate: (url) => window.location.assign(url),
    origin: window.location.origin,
    statusWaitMs: 4000,
  });
});
