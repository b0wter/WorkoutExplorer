// lib/zip against built archives (F-35 design, Verification; ADR-040). Synthetic only: archives are
// built here from the synthetic corpus by test-support/zip.mjs, never committed.
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { bomb, crc32, deflate, syntheticFits, zip, type Entry } from '../../../test-support/zip.mjs';
import { canInflate, extractFit, isZip, SLICE, type ZipResult } from '.';

const LIMIT = 20_000_000;
const FIT = syntheticFits()[0].bytes;
const one = (e: Partial<Entry> = {}) => zip([{ name: 'ride.fit', data: FIT, ...e }]);
function join(r: ZipResult): Uint8Array | string {
  if (!r.ok) return r.reason;
  const out = new Uint8Array(r.chunks.reduce((n, c) => n + c.length, 0));
  r.chunks.reduce((at, c) => (out.set(c, at), at + c.length), 0);
  return out;
}
const latin1 = (s: string) => Uint8Array.from(s, (c) => c.charCodeAt(0));
const run = async (b: Uint8Array<ArrayBuffer>, limit = LIMIT) => join(await extractFit(b, limit));
// toEqual on large typed arrays takes seconds; a plain comparison gives the same answer.
const same = (got: Uint8Array | string, want: Uint8Array) =>
  typeof got !== 'string' && got.length === want.length && got.every((x, i) => x === want[i]);
const set16 = (b: Uint8Array, at: number, v: number) => new DataView(b.buffer).setUint16(at, v, true);
const set32 = (b: Uint8Array, at: number, v: number) => new DataView(b.buffer).setUint32(at, v >>> 0, true);

// Counts every DecompressionStream made, the bytes it produced and whether it was cancelled.
const Real = globalThis.DecompressionStream;
let made = 0;
let produced = 0;
let cancelled = false;
class Spy {
  writable: WritableStream;
  readable: ReadableStream;
  constructor(format: CompressionFormat) {
    made++;
    const d = new Real(format);
    const r = d.readable.getReader();
    this.writable = d.writable;
    this.readable = new ReadableStream(
      {
        async pull(c) {
          const x = await r.read();
          if (x.done) return c.close();
          produced += x.value.length;
          c.enqueue(x.value);
        },
        cancel(why) {
          cancelled = true;
          return r.cancel(why);
        },
      },
      { highWaterMark: 0 },
    );
  }
}
beforeEach(() => {
  made = produced = 0;
  cancelled = false;
  globalThis.DecompressionStream = Spy as unknown as typeof DecompressionStream;
});
afterEach(() => {
  globalThis.DecompressionStream = Real;
});

describe('detection (Z-3)', () => {
  it('never takes a synthetic FIT file or a short file for an archive', () => {
    for (const f of syntheticFits()) expect(isZip(f.bytes.subarray(0, 4)), f.name).toBe(false);
    for (const s of ['', 'P', 'PK', 'PK\x03']) expect(isZip(latin1(s))).toBe(false);
  });
  it('takes the three zip signatures', () => {
    for (const s of ['PK\x03\x04', 'PK\x05\x06', 'PK\x07\x08']) expect(isZip(latin1(s))).toBe(true);
    expect(isZip(latin1('PK\x01\x02'))).toBe(false);
  });
});

describe('browser support (Z-9)', () => {
  it('needs DecompressionStream with deflate-raw', () => {
    expect(canInflate()).toBe(true);
    // @ts-expect-error removing the global, as an old browser
    delete globalThis.DecompressionStream;
    expect(canInflate()).toBe(false);
    globalThis.DecompressionStream = class {
      constructor(f: string) {
        if (f === 'deflate-raw') throw new TypeError('unsupported');
      }
    } as unknown as typeof DecompressionStream;
    expect(canInflate()).toBe(false);
  });
});

describe('accepted archives (Z-5, Z-7)', () => {
  it('every synthetic FIT file comes out byte-identical, deflated and stored', async () => {
    for (const f of syntheticFits()) {
      expect(same(await run(one({ data: f.bytes }).bytes), f.bytes), f.name).toBe(true);
      expect(same(await run(one({ data: f.bytes, method: 0 }).bytes), f.bytes), f.name).toBe(true);
    }
  }, 30_000);
  it.each([
    ['Linux zip', [{ name: 'ride.fit', data: FIT, extra: 'UT\x05\x00\x03abcd' }]],
    ['Finder: data descriptor with signature, __MACOSX', [
      { name: 'ride.fit', data: FIT, descriptor: 'sig' },
      { name: '__MACOSX/', data: new Uint8Array(), method: 0 },
      { name: '__MACOSX/._ride.fit', data: new Uint8Array(200), descriptor: 'sig' },
    ]],
    ['Explorer: data descriptor without signature', [{ name: 'ride.fit', data: FIT, descriptor: 'nosig' }]],
    ['zip -0', [{ name: 'ride.fit', data: FIT, method: 0 }]],
    ['upper case .FIT', [{ name: 'RIDE.FIT', data: FIT }]],
    ['ignored neighbours', [
      { name: 'notes.txt', data: new Uint8Array(10) },
      { name: 'readme.pdf', data: new Uint8Array(10) },
      { name: 'sub/other.fit', data: new Uint8Array(10) },
      { name: 'sub\\other.fit', data: new Uint8Array(10) },
      { name: 'Ride.Fit', data: FIT },
    ]],
    ['AppleDouble ._ride.fit next to ride.fit (OQ-109)', [
      { name: '._ride.fit', data: new Uint8Array(200) },
      { name: 'ride.fit', data: FIT },
      { name: '._RIDE.FIT', data: new Uint8Array(200) },
    ]],
    ['encrypted notes.txt and bzip2 readme.pdf', [
      { name: 'notes.txt', data: new Uint8Array(10), flags: 1, method: 0 },
      { name: 'readme.pdf', data: new Uint8Array(10), method: 12 },
      { name: 'ride.fit', data: FIT },
    ]],
  ] as [string, Entry[]][])('%s', async (_n, entries) => {
    expect(same(await run(zip(entries).bytes), FIT)).toBe(true);
    expect(made).toBeLessThanOrEqual(1);
  });
  it('a comment of 65,535 bytes', async () => {
    expect(same(await run(zip([{ name: 'ride.fit', data: FIT }], { comment: 'x'.repeat(0xffff) }).bytes), FIT)).toBe(true);
  });
});

describe('selection (Z-7)', () => {
  it.each([
    ['empty archive', []],
    ['FIT files only in folders', [{ name: 'a/ride.fit' }, { name: 'a\\ride.fit' }, { name: '__MACOSX/._ride.fit' }]],
    ['directory entries and other names', [{ name: 'ride.fit/' }, { name: 'ride.fit.txt' }, { name: 'fit' }, { name: '' }]],
    ['only AppleDouble names (OQ-109)', [{ name: '._ride.fit' }, { name: '._.fit' }]],
  ] as [string, Entry[]][])('%s: no FIT file', async (_n, entries) => {
    expect(await run(zip(entries).bytes)).toBe('noFit');
    expect(made).toBe(0);
  });
  it.each([
    ['two root FIT files', [{ name: 'a.fit' }, { name: 'B.FIT' }]],
    ['one name twice', [{ name: 'ride.fit' }, { name: 'ride.fit' }]],
    ['a name starting with one dot or with _ is not AppleDouble', [{ name: '.ride.fit' }, { name: '_ride.fit' }]],
    ['a damaged second FIT entry', [{ name: 'a.fit' }, { name: 'b.fit', method: 99, flags: 1 }]],
  ] as [string, Entry[]][])('%s: more than one', async (_n, entries) => {
    expect(await run(zip(entries).bytes)).toBe('several');
    expect(made).toBe(0);
  });
  it('the empty archive starts with the end record and is detected', () => {
    expect(isZip(zip([]).bytes)).toBe(true);
  });
  it('the name `.fit` alone is a root FIT file', async () => {
    expect(same(await run(zip([{ name: '.fit', data: FIT }]).bytes), FIT)).toBe(true);
  });
});

describe('refused FIT entries (Z-7)', () => {
  it.each([
    ['encrypted', { flags: 1 }],
    ['strong encryption', { flags: 0x41 }],
    ['bzip2', { method: 12 }],
    ['deflate64', { method: 9 }],
  ] as [string, Partial<Entry>][])('%s', async (_n, e) => {
    expect(await run(one(e).bytes)).toBe('damaged');
    expect(made).toBe(0);
  });
});

describe('Zip64 and split archives are refused whatever the entries (Z-6)', () => {
  const fields: [string, (z: ReturnType<typeof one>) => void][] = [
    ['entry count 0xFFFF', (z) => (set16(z.bytes, z.end + 8, 0xffff), set16(z.bytes, z.end + 10, 0xffff))],
    ['directory size all ones', (z) => set32(z.bytes, z.end + 12, 0xffffffff)],
    ['directory offset all ones', (z) => set32(z.bytes, z.end + 16, 0xffffffff)],
    ['central compressed size all ones', (z) => set32(z.bytes, z.centrals[0] + 20, 0xffffffff)],
    ['central size all ones', (z) => set32(z.bytes, z.centrals[0] + 24, 0xffffffff)],
    ['central offset all ones', (z) => set32(z.bytes, z.centrals[0] + 42, 0xffffffff)],
    ['disk number 1', (z) => set16(z.bytes, z.end + 4, 1)],
    ['directory disk 1', (z) => set16(z.bytes, z.end + 6, 1)],
    ['entries on this disk differ from total', (z) => set16(z.bytes, z.end + 8, 0)],
    ['entry on disk 1', (z) => set16(z.bytes, z.centrals[0] + 34, 1)],
  ];
  it.each(fields)('%s', async (_n, change) => {
    const z = one();
    change(z);
    expect(await run(z.bytes)).toBe('damaged');
  });
  it('a Zip64 end locator right before the end record', async () => {
    const z = zip([{ name: 'ride.fit', data: FIT }, { name: 'pad', data: new Uint8Array(), method: 0, extra: new Uint8Array(20) }]);
    set32(z.bytes, z.end - 20, 0x07064b50);
    expect(await run(z.bytes)).toBe('damaged');
  });
  it('Zip64 on a non-FIT entry refuses the archive too', async () => {
    const z = zip([{ name: 'ride.fit', data: FIT }, { name: 'big.bin' }]);
    set32(z.bytes, z.centrals[1] + 24, 0xffffffff);
    expect(await run(z.bytes)).toBe('damaged');
  });
  it('the split marker at the start', async () => {
    const b = one().bytes;
    set32(b, 0, 0x08074b50);
    expect(await run(b)).toBe('damaged');
  });
});

describe('structure (Z-4, Z-5): every failure is a result, never an exception', () => {
  const small = zip([{ name: 'a.txt', data: new Uint8Array(20) }, { name: 'ride.fit', data: FIT.subarray(0, 200) }], { comment: 'hi' }).bytes;
  it('the small archive itself is accepted', async () => {
    expect(same(await run(small), FIT.subarray(0, 200))).toBe(true);
  });
  it('truncation at every length', async () => {
    for (let n = 0; n < small.length; n++) expect(await run(small.slice(0, n)), `${n}`).toBe('damaged');
  });
  it('a trailing byte, a padded directory, a fake end record inside the comment', async () => {
    const pad = new Uint8Array(small.length + 1);
    pad.set(small);
    expect(await run(pad)).toBe('damaged');
    const z = one();
    set32(z.bytes, z.end + 12, z.end - z.cd - 1);
    expect(await run(z.bytes)).toBe('damaged');
    const fake = new Uint8Array(22);
    set32(fake, 0, 0x06054b50); // claims no entries, comment length 0: it is the first candidate from the end
    expect(await run(zip([{ name: 'ride.fit', data: FIT }], { comment: fake }).bytes)).toBe('damaged');
  });
  it.each([
    ['one entry more than written', (z: ReturnType<typeof one>) => (set16(z.bytes, z.end + 8, 3), set16(z.bytes, z.end + 10, 3))],
    ['one entry less than written', (z: ReturnType<typeof one>) => (set16(z.bytes, z.end + 8, 1), set16(z.bytes, z.end + 10, 1))],
    ['directory offset out of range', (z: ReturnType<typeof one>) => set32(z.bytes, z.end + 16, 0x7fffffff)],
    ['local offset out of range', (z: ReturnType<typeof one>) => set32(z.bytes, z.centrals[1] + 42, 0x7ffffff0)],
    ['local offset into the directory', (z: ReturnType<typeof one>) => set32(z.bytes, z.centrals[1] + 42, z.cd)],
    ['name length past the directory', (z: ReturnType<typeof one>) => set16(z.bytes, z.centrals[1] + 28, 0xffff)],
    ['bad central signature', (z: ReturnType<typeof one>) => set32(z.bytes, z.centrals[1], 0)],
    ['bad local signature', (z: ReturnType<typeof one>) => set32(z.bytes, z.locals[1], 0x02014b50)],
    ['compressed size past the directory', (z: ReturnType<typeof one>) => set32(z.bytes, z.centrals[1] + 20, z.cd)],
  ])('%s', async (_n, change) => {
    const z = zip([{ name: 'a.txt' }, { name: 'ride.fit', data: FIT }]) as ReturnType<typeof one>;
    change(z);
    expect(await run(z.bytes)).toBe('damaged');
  });
  it('a few thousand random byte changes give only result values', async () => {
    let seed = 42;
    const rnd = () => ((seed = (Math.imul(seed, 1103515245) + 12345) >>> 0) / 2 ** 32);
    const seen = new Set<string>();
    for (let i = 0; i < 3000; i++) {
      const b = small.slice();
      for (let k = 1 + Math.floor(rnd() * 3); k > 0; k--) b[Math.floor(rnd() * b.length)] = Math.floor(rnd() * 256);
      const r = await run(b); // a throw fails the test
      if (typeof r !== 'string') expect(same(r, FIT.subarray(0, 200))).toBe(true);
      seen.add(typeof r === 'string' ? r : 'ok');
    }
    expect(seen.has('damaged')).toBe(true);
  }, 60_000);
});

describe('only the FIT entry is inflated, once (Z-5)', () => {
  it('overlapping entries pointing at one local header', async () => {
    const z = zip([{ name: 'ride.fit', data: FIT }, { name: 'notes.txt', data: FIT, sameLocalAs: 0 }, { name: 'more.txt', data: FIT, sameLocalAs: 0 }]);
    expect(same(await run(z.bytes), FIT)).toBe(true);
    expect(made).toBe(1);
  });
  it('two root FIT names on one local header are more than one', async () => {
    const z = zip([{ name: 'ride.fit', data: FIT }, { name: 'ride.fit', data: FIT, sameLocalAs: 0 }]);
    expect(await run(z.bytes)).toBe('several');
    expect(made).toBe(0);
  });
});

describe('output limit (Z-8)', () => {
  const big = (n: number) => new Uint8Array(n).fill(1);
  it('exactly 20,000,000 bytes are accepted; 20,000,001 declared are refused before inflating', async () => {
    const r = await extractFit(one({ data: big(LIMIT) }).bytes, LIMIT);
    expect(r.ok && r.chunks.reduce((s, c) => s + c.length, 0)).toBe(LIMIT);
    made = 0;
    expect(await run(one({ data: big(LIMIT + 1) }).bytes)).toBe('fitTooLarge');
    expect(made).toBe(0);
    expect(await run(one({ data: big(LIMIT + 1), method: 0 }).bytes)).toBe('fitTooLarge');
  });
  it('20,000,001 bytes declared as 20,000,000 are refused by the bytes produced', async () => {
    expect(await run(one({ data: big(LIMIT + 1), size: LIMIT }).bytes)).toBe('fitTooLarge');
    expect(cancelled).toBe(true);
  });
  it('a 1 GiB bomb declaring 1,000 bytes stops at the limit plus one slice, stream cancelled', async () => {
    const t = Date.now();
    const z = one({ packed: bomb(1024), size: 1000, crc: 0 });
    expect(await run(z.bytes)).toBe('fitTooLarge');
    expect(produced).toBeGreaterThan(LIMIT);
    expect(produced).toBeLessThanOrEqual(LIMIT + SLICE * 1032);
    expect(cancelled).toBe(true);
    expect(Date.now() - t).toBeLessThan(5000);
  });
});

describe('lying sizes and data are damaged (Z-5, Z-8)', () => {
  it.each([
    ['declared length too small', { size: FIT.length - 1 }],
    ['declared length too large', { size: FIT.length + 1 }],
    ['wrong CRC-32', { crc: crc32(FIT) ^ 1 }],
    ['truncated deflate data', { packed: deflate(FIT).subarray(0, 100) }],
    ['invalid deflate data', { packed: new Uint8Array(64).fill(0xff) }],
    ['stored with unequal sizes', { method: 0, packedSize: FIT.length - 1 }],
    ['local name differs', { localName: 'ride.fiT' }],
    ['local name longer', { localName: 'ride.fit2' }],
    ['local method differs', { localMethod: 0 }],
  ] as [string, Partial<Entry>][])('%s', async (_n, e) => {
    expect(await run(one(e).bytes)).toBe('damaged');
  });
  it('a root .fit entry that holds a zip is refused, deflated or stored (Z-10, OQ-111)', async () => {
    const inner = one().bytes;
    expect(await run(one({ data: inner }).bytes)).toBe('damaged');
    expect(await run(one({ data: inner, method: 0 }).bytes)).toBe('damaged');
  });
});

describe('many entries (Z-4)', () => {
  // 65,535 is the Zip64 count marker (Z-6), so 65,534 is the most a plain archive holds.
  it('65,534 entries with one root FIT file in well under a second', async () => {
    const entries: Entry[] = Array.from({ length: 0xfffd }, (_, i) => ({ name: `d/${i}.fit`, method: 0 }));
    entries.push({ name: 'ride.fit', data: FIT });
    const b = zip(entries).bytes;
    const t = performance.now();
    expect(same(await run(b), FIT)).toBe(true);
    expect(performance.now() - t).toBeLessThan(500);
  });
});
