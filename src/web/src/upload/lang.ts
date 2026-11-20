// The upload page's language drop-down (F-54 LG-1, OQ-280): each language named in itself; a
// choice applies at once and is remembered, the one storage write of this page.
import { LANGS, LANG_NAMES, lang, type Lang } from '../lib/i18n';
import { setLang } from '../lib/i18n/load';
import { rememberLang, type KeyValueStore } from '../lib/prefs';
import { t } from './texts';

export function addLangSelect(doc: Document, storage: () => KeyValueStore): void {
  const slot = doc.getElementById('lang-slot');
  if (!slot) return;
  const select = doc.createElement('select');
  select.setAttribute('aria-label', t('uploadLanguage'));
  select.append(...LANGS.map((l, i) => new Option(LANG_NAMES[i], l)));
  select.value = lang();
  select.addEventListener('change', () => {
    rememberLang(storage, select.value);
    void setLang(select.value as Lang, doc).then(() => select.setAttribute('aria-label', t('uploadLanguage')));
  });
  slot.append(select);
}
