// Takes the one root FIT file out of a zip archive (F-35 Z-1 to Z-10; ADR-040). Pure: no imports,
// no text, never throws; names are compared as bytes and never decoded.

/** Why an archive is refused; the keys of the matching sentences in lib/upload. */
export type ZipReason = 'noFit' | 'several' | 'fitTooLarge' | 'damaged';
export type ZipResult = { ok: true; chunks: Uint8Array[] } | { ok: false; reason: ZipReason };

/** Compressed bytes fed to the inflater at a time: one slice inflates to at most about 4 MB (Z-8). */
export const SLICE = 4096;
const YIELD_EVERY = 64; // slices between two macrotask yields, so the tab stays responsive

const LOCAL = 0x04034b50;
const CENTRAL = 0x02014b50;
const END = 0x06054b50;
const ZIP64_LOCATOR = 0x07064b50;
const SPLIT = 0x08074b50;

function sig(b: Uint8Array): number {
  return b.length < 4 ? -1 : (b[0] | (b[1] << 8) | (b[2] << 16) | (b[3] << 24)) >>> 0;
}

/** Z-3: an archive is a file whose first four bytes are a zip signature; nothing else is searched. */
export function isZip(head: Uint8Array): boolean {
  const s = sig(head);
  return s === LOCAL || s === END || s === SPLIT;
}

/** Z-9: the browser can inflate deflate-raw. */
export function canInflate(): boolean {
  try {
    new DecompressionStream('deflate-raw');
    return true;
  } catch {
    return false;
  }
}

/** Z-7: no `/` or `\` byte, not starting with `._` (AppleDouble, OQ-109), last four bytes `.fit` in any ASCII case. */
function isRootFit(name: Uint8Array): boolean {
  const n = name.length;
  if (n < 4 || name.includes(0x2f) || name.includes(0x5c) || (name[0] === 0x2e && name[1] === 0x5f)) return false;
  return name[n - 4] === 0x2e && (name[n - 3] | 0x20) === 0x66 && (name[n - 2] | 0x20) === 0x69 && (name[n - 1] | 0x20) === 0x74;
}

const damaged: ZipResult = { ok: false, reason: 'damaged' };

/** Extracts the root FIT entry of `zip`, producing at most `limit` bytes. */
export async function extractFit(zip: Uint8Array<ArrayBuffer>, limit: number): Promise<ZipResult> {
  try {
    return await extract(zip, limit);
  } catch {
    return damaged;
  }
}

async function extract(zip: Uint8Array<ArrayBuffer>, limit: number): Promise<ZipResult> {
  const n = zip.length;
  const v = new DataView(zip.buffer, zip.byteOffset, n);
  const u16 = (p: number) => v.getUint16(p, true);
  const u32 = (p: number) => v.getUint32(p, true);
  if (n < 22 || sig(zip) === SPLIT) return damaged;

  // Z-4: the end record, searched backwards; its comment must reach exactly the end of the file.
  let end = -1;
  for (let p = n - 22; p >= 0 && p >= n - 22 - 0xffff; p--) {
    if (u32(p) === END && p + 22 + u16(p + 20) === n) {
      end = p;
      break;
    }
  }
  if (end < 0) return damaged;
  const onDisk = u16(end + 8);
  const total = u16(end + 10);
  const cdSize = u32(end + 12);
  const cdStart = u32(end + 16);
  // Z-6: Zip64 and split archives, before any entry is looked at.
  if (end >= 20 && u32(end - 20) === ZIP64_LOCATOR) return damaged;
  if (total === 0xffff || onDisk === 0xffff || cdSize === 0xffffffff || cdStart === 0xffffffff) return damaged;
  if (u16(end + 4) !== 0 || u16(end + 6) !== 0 || onDisk !== total) return damaged;
  if (cdStart + cdSize !== end) return damaged;

  // Z-4, Z-6, Z-7: walk every central header; count root FIT entries, keep the first.
  let fit = -1;
  let fits = 0;
  let p = cdStart;
  for (let i = 0; i < total; i++) {
    if (p + 46 > end || u32(p) !== CENTRAL) return damaged;
    const nameLen = u16(p + 28);
    const next = p + 46 + nameLen + u16(p + 30) + u16(p + 32);
    if (next > end) return damaged;
    if (u32(p + 20) === 0xffffffff || u32(p + 24) === 0xffffffff || u32(p + 42) === 0xffffffff || u16(p + 34) !== 0) return damaged;
    if (isRootFit(zip.subarray(p + 46, p + 46 + nameLen))) {
      fits++;
      if (fit < 0) fit = p;
    }
    p = next;
  }
  if (p !== end) return damaged;
  if (fits === 0) return { ok: false, reason: 'noFit' };
  if (fits > 1) return { ok: false, reason: 'several' };

  // The FIT entry: central facts only (Z-5).
  const flags = u16(fit + 8);
  const method = u16(fit + 10);
  const crc = u32(fit + 16);
  const packed = u32(fit + 20);
  const size = u32(fit + 24);
  const nameLen = u16(fit + 28);
  const local = u32(fit + 42);
  if (flags & 0x41 || (method !== 0 && method !== 8)) return damaged;
  if (size > limit) return { ok: false, reason: 'fitTooLarge' };

  // The local header must agree on signature, method and name; it only locates the data.
  if (local + 30 + nameLen > cdStart || u32(local) !== LOCAL || u16(local + 8) !== method || u16(local + 26) !== nameLen) return damaged;
  for (let i = 0; i < nameLen; i++) if (zip[local + 30 + i] !== zip[fit + 46 + i]) return damaged;
  const from = local + 30 + nameLen + u16(local + 28);
  const to = from + packed;
  if (to > cdStart) return damaged;

  let chunks: Uint8Array[];
  let got = 0;
  let sum = 0xffffffff;
  if (method === 0) {
    if (packed !== size) return damaged;
    chunks = [zip.subarray(from, to)];
    got = packed;
    sum = crc32(chunks[0], sum);
  } else {
    // Z-8: slices pulled one at a time; output counted as it arrives, never trusting `size`.
    chunks = [];
    let at = from;
    let slices = 0;
    const source = new ReadableStream<Uint8Array<ArrayBuffer>>(
      {
        async pull(c) {
          if (at >= to) return c.close();
          if (++slices % YIELD_EVERY === 0) await new Promise((r) => setTimeout(r, 0));
          c.enqueue(zip.subarray(at, (at = Math.min(at + SLICE, to))));
        },
      },
      { highWaterMark: 0 },
    );
    const reader = source.pipeThrough(new DecompressionStream('deflate-raw')).getReader();
    for (;;) {
      const r = await reader.read(); // a stream error (truncated or invalid data) is caught as damaged
      if (r.done) break;
      got += r.value.length;
      if (got > limit) {
        reader.cancel().catch(() => {});
        return { ok: false, reason: 'fitTooLarge' };
      }
      sum = crc32(r.value, sum);
      chunks.push(r.value);
    }
  }
  if (got !== size || (sum ^ 0xffffffff) >>> 0 !== crc) return damaged;
  // Z-10: no nested archive ever leaves the browser.
  const head = new Uint8Array(4);
  let k = 0;
  for (const c of chunks) for (let i = 0; i < c.length && k < 4; i++) head[k++] = c[i];
  return k === 4 && isZip(head) ? damaged : { ok: true, chunks };
}

let table: Int32Array | undefined;

/** CRC-32 (IEEE) running state: start with 0xffffffff, finish with `^ 0xffffffff`. */
function crc32(b: Uint8Array, crc: number): number {
  if (!table) {
    table = new Int32Array(256);
    for (let i = 0; i < 256; i++) {
      let c = i;
      for (let j = 0; j < 8; j++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
      table[i] = c;
    }
  }
  for (let i = 0; i < b.length; i++) crc = table[(crc ^ b[i]) & 0xff] ^ (crc >>> 8);
  return crc;
}
