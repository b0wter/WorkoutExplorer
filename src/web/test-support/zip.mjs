// Builds zip archives for the F-35 tests (architect/F-35-zip-upload.md, Verification), emulating
// the layouts of common tools: data descriptors with and without signature, __MACOSX/ entries,
// extra fields, stored entries. Plain JS so node:zlib needs no @types/node (F-01). Types: zip.d.mts.
// Synthetic only: archives are built in the test run from the synthetic corpus, never committed.

import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';

const CORPUS = path.resolve(import.meta.dirname, '../../server/testdata/corpus/synthetic');

/** Every synthetic FIT file of the corpus, sorted by name. */
export function syntheticFits() {
  const out = fs
    .readdirSync(CORPUS)
    .filter((f) => f.endsWith('.fit'))
    .sort()
    .map((name) => ({ name, bytes: new Uint8Array(fs.readFileSync(path.join(CORPUS, name))) }));
  if (out.length === 0) throw new Error(`no synthetic FIT files in ${CORPUS}`);
  return out;
}

export const crc32 = (b) => zlib.crc32(b) >>> 0;
export const deflate = (b) => new Uint8Array(zlib.deflateRawSync(b));

/** Raw deflate data that inflates to `mib` MiB of zeros, built from independent full-flushed pieces. */
export function bomb(mib) {
  const piece = zlib.deflateRawSync(Buffer.alloc(1 << 20), { finishFlush: zlib.constants.Z_FULL_FLUSH });
  return new Uint8Array(Buffer.concat([...Array(mib).fill(piece), Buffer.from([0x03, 0x00])]));
}

const enc = (s) => (typeof s === 'string' ? Buffer.from(s, 'latin1') : Buffer.from(s ?? []));

function header(size, fields) {
  const b = Buffer.alloc(size);
  for (const [at, bits, v] of fields) bits === 16 ? b.writeUInt16LE(v, at) : b.writeUInt32LE(v >>> 0, at);
  return b;
}

/**
 * One archive. Entry: { name, data?, method? (0 stored, 8 deflate default, others written as stored
 * bytes), flags?, descriptor? ('sig' | 'nosig'), extra?, packed? (data as written), crc?, size?,
 * packedSize? (central overrides), localName?, localMethod?, sameLocalAs? (index of an earlier entry
 * whose local header this central entry points at) }. Options: { comment? }.
 * Returns the bytes and the offsets of every structure, so tests can change single fields.
 */
export function zip(entries, opts = {}) {
  const parts = [];
  let at = 0;
  const push = (b) => {
    parts.push(b);
    at += b.length;
  };
  const locals = [];
  const central = [];
  for (const e of entries) {
    const name = enc(e.name);
    const data = Buffer.from(e.data ?? []);
    const method = e.method ?? 8;
    const packed = Buffer.from(e.packed ?? (method === 8 ? zlib.deflateRawSync(data) : data));
    const crc = e.crc ?? crc32(data);
    const size = e.size ?? data.length;
    const packedSize = e.packedSize ?? packed.length;
    const flags = (e.flags ?? 0) | (e.descriptor ? 8 : 0);
    const extra = enc(e.extra);
    let local;
    if (e.sameLocalAs !== undefined) local = locals[e.sameLocalAs];
    else {
      local = at;
      const lname = e.localName === undefined ? name : enc(e.localName);
      const d = e.descriptor;
      push(header(30, [[0, 32, 0x04034b50], [4, 16, 20], [6, 16, flags], [8, 16, e.localMethod ?? method],
        [14, 32, d ? 0 : crc], [18, 32, d ? 0 : packed.length], [22, 32, d ? 0 : data.length],
        [26, 16, lname.length], [28, 16, extra.length]]));
      push(lname);
      push(extra);
      push(packed);
      if (d === 'sig') push(header(16, [[0, 32, 0x08074b50], [4, 32, crc], [8, 32, packed.length], [12, 32, data.length]]));
      if (d === 'nosig') push(header(12, [[0, 32, crc], [4, 32, packed.length], [8, 32, data.length]]));
    }
    locals.push(local);
    central.push({ name, extra, flags, method, crc, size, packedSize, local });
  }
  const cd = at;
  const centrals = [];
  for (const c of central) {
    centrals.push(at);
    push(header(46, [[0, 32, 0x02014b50], [4, 16, 0x031e], [6, 16, 20], [8, 16, c.flags], [10, 16, c.method],
      [16, 32, c.crc], [20, 32, c.packedSize], [24, 32, c.size], [28, 16, c.name.length], [30, 16, c.extra.length],
      [42, 32, c.local]]));
    push(c.name);
    push(c.extra);
  }
  const end = at;
  const comment = enc(opts.comment);
  push(header(22, [[0, 32, 0x06054b50], [8, 16, entries.length], [10, 16, entries.length],
    [12, 32, end - cd], [16, 32, cd], [20, 16, comment.length]]));
  push(comment);
  return { bytes: new Uint8Array(Buffer.concat(parts)), cd, end, locals, centrals };
}
