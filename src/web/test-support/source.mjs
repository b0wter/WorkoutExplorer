// Reads a source file of src/web as text, for tests that check the files themselves (CSS that
// Vitest would otherwise blank out). Plain JS so node:fs needs no @types/node (F-01).
import fs from 'node:fs';
import path from 'node:path';

const ROOT = path.resolve(import.meta.dirname, '..');

/** @param {string} rel path relative to src/web */
export function readWebFile(rel) {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8');
}
