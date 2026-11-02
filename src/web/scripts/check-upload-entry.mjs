// Upload page build checks (F-11 design, Verification AC 8; ADR-024 item 3).
// 1. The upload entry's counted files use no cookie or browser-storage API.
// 2. The counted sets of the two entries are disjoint (no shared chunk).
// Only node:fs and node:path. Run by `npm run size`, after the build.

import fs from 'node:fs';
import path from 'node:path';
import { readPages, resolveEntries, withHtmlEntries } from './check-bundle-size.mjs';

export const FORBIDDEN = /document\.cookie|localStorage|sessionStorage|indexedDB|\bcaches\b|serviceWorker|cookieStore/;

/** Pure: problems found for a manifest; `read(file)` returns a counted file's text. */
export function problems(manifest, read) {
  const entries = resolveEntries(manifest);
  const upload = entries.find((e) => e.entry === 'upload.html');
  if (!upload) return ['no upload.html entry in the manifest'];
  const out = [];
  for (const f of upload.files) {
    if (f.endsWith('.js') && FORBIDDEN.test(read(f))) out.push(`${f}: cookie or storage API`);
  }
  for (const e of entries) {
    if (e === upload) continue;
    for (const f of e.files) if (upload.files.includes(f)) out.push(`${f}: shared by upload.html and ${e.entry}`);
  }
  return out;
}

if (process.argv[1] && fs.realpathSync(process.argv[1]) === import.meta.filename) {
  const buildDir = path.resolve(import.meta.dirname, '..', '..', '..', 'build', 'web');
  const manifest = JSON.parse(fs.readFileSync(path.join(buildDir, '.vite', 'manifest.json'), 'utf8'));
  const found = problems(withHtmlEntries(manifest, readPages(buildDir)), (f) => fs.readFileSync(path.join(buildDir, f), 'utf8'));
  for (const p of found) console.error(`upload entry: ${p}`);
  if (found.length === 0) console.log('upload entry: no storage API, no shared chunk PASS');
  process.exitCode = found.length === 0 ? 0 : 1;
}
