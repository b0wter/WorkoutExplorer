// Reads and writes the committed sample page view models (F-13 design D-3):
// src/server/testdata/page/<name>.json, the contract with F-15's Go builder and the input of
// protoserve's sample pages. Plain JS so node:fs and node:crypto need no @types/node (F-01).
// Synthetic only (ADR-004).

import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

const DIR = path.resolve(import.meta.dirname, '../../server/testdata/page');

/** Sample names present on disk, sorted. */
export function listSamples() {
  if (!fs.existsSync(DIR)) return [];
  return fs
    .readdirSync(DIR)
    .filter((f) => f.endsWith('.json'))
    .map((f) => f.slice(0, -'.json'.length))
    .sort();
}

/** The committed text of a sample, or null. */
export function readSample(name) {
  const file = path.join(DIR, `${name}.json`);
  return fs.existsSync(file) ? fs.readFileSync(file, 'utf8') : null;
}

export function writeSample(name, text) {
  fs.mkdirSync(DIR, { recursive: true });
  fs.writeFileSync(path.join(DIR, `${name}.json`), text);
}

export function removeSample(name) {
  fs.rmSync(path.join(DIR, `${name}.json`));
}

/** True when the samples are to be rewritten: `UPDATE_SAMPLES=1 npm test`. */
export function updating() {
  return process.env.UPDATE_SAMPLES === '1';
}

/** First 16 hex digits of the SHA-256 of a data file: the {hash} of data.{hash}.bin. */
export function fileHash(bytes) {
  return crypto.createHash('sha256').update(new Uint8Array(bytes)).digest('hex').slice(0, 16);
}
