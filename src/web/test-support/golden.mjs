// Reads the golden data files of F-03 for the F-04 decoder tests
// (architect/F-04-workout-data-file-reader.md, Tests). Plain JS so node:fs
// needs no @types/node in the app's type check (F-01). Types: golden.d.mts.
//
// Synthetic fixtures only: nothing derived from a real FIT file is committed
// (ADR-004 item 7, rev. 3).

import fs from 'node:fs';
import path from 'node:path';

// A path string built without URL: under the jsdom test environment (ADR-012) the
// global URL is jsdom's, and node:fs rejects what it produces.
const DIR = path.resolve(import.meta.dirname, '../../server/testdata/datafile/synthetic');

/**
 * Every `<name>.bin` with its `<name>.json`, sorted by name. Throws on any
 * other entry, an unpaired file or an empty folder, so a test never passes
 * vacuously.
 *
 * @returns {{ name: string, bin: ArrayBuffer, fixture: unknown }[]}
 */
export function loadGolden() {
  const entries = fs.readdirSync(DIR).sort();
  const bins = new Set();
  const jsons = new Set();
  for (const entry of entries) {
    const m = /^(.+)\.(bin|json)$/.exec(entry);
    if (!m) throw new Error(`golden: unexpected entry ${entry} in ${DIR}`);
    (m[2] === 'bin' ? bins : jsons).add(m[1]);
  }
  for (const name of bins) if (!jsons.has(name)) throw new Error(`golden: ${name}.bin has no ${name}.json`);
  for (const name of jsons) if (!bins.has(name)) throw new Error(`golden: ${name}.json has no ${name}.bin`);
  if (bins.size === 0) throw new Error(`golden: no fixtures in ${DIR}`);

  return [...bins].sort().map((name) => {
    const buf = fs.readFileSync(path.join(DIR, `${name}.bin`));
    // A Buffer may sit inside a larger pooled ArrayBuffer; the decoder needs byte 0 of the file at offset 0.
    const bin = buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength);
    const fixture = JSON.parse(fs.readFileSync(path.join(DIR, `${name}.json`), 'utf8'));
    return { name, bin, fixture };
  });
}
