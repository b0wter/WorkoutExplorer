// Language files (F-54 LG-2, LG-3): one content-hashed JSON file per text group and language, and
// the head elements naming them. Used by vite.config.ts (`wx-lang-files`) and the tests. Plain JS,
// so node:fs and node:crypto need no @types/node (F-01). Only node built-ins.

import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';

/** The served limit per language file (LG-3): 5 KB gzip. */
export const LANG_FILE_LIMIT = 5120;

/** gzip -9 size, as ADR-002 counts. */
export const gzipSize = (text) => zlib.gzipSync(text, { level: 9 }).length;

/** Reads src/i18n/{lang}.json for every translated language. */
export function readTranslations(webRoot, langs) {
  return Object.fromEntries(langs.map((l) => [l, JSON.parse(fs.readFileSync(path.join(webRoot, 'src', 'i18n', l + '.json'), 'utf8'))]));
}

/**
 * Pure: the files and their URLs. `translations[lang][table]` is a table's texts; a group's file is
 * the flat union of its tables. Returns `{ files: {fileName: text}, urls: {group: {lang: url}} }`.
 */
export function langFiles(groups, translations) {
  const files = {};
  const urls = {};
  for (const [group, tables] of Object.entries(groups)) {
    urls[group] = {};
    for (const [lang, t] of Object.entries(translations)) {
      const text = JSON.stringify(Object.assign({}, ...tables.map((name) => t[name] ?? {})));
      const hash = crypto.createHash('sha256').update(text).digest('base64url').slice(0, 8);
      const fileName = `assets/lang-${group}-${lang}-${hash}.json`;
      files[fileName] = text;
      urls[group][lang] = '/' + fileName;
    }
  }
  return { files, urls };
}

/** Pure: the head elements of one page, before its <title> (the head script reads them). */
export function langMetas(page, urls) {
  return page.groups
    .map((g, i) => {
      const hold = page.hold && i === 0 ? ' data-hold' : '';
      const data = Object.entries(urls[g]).map(([l, u]) => ` data-${l}="${u}"`).join('');
      return `<meta name="wx-lang-${g}"${hold}${data} />`;
    })
    .join('\n    ');
}
