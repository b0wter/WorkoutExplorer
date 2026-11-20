// F-54 LG-4: the metadata from the data file's streamed prefix, checked as in the full decode.
import { describe, expect, it } from 'vitest';
import { loadGolden } from '../../../test-support/golden.mjs';
import { decodeDataFile } from './decode';
import { decodeMeta, streamMeta } from './prefix';

const golden = loadGolden().map((g) => ({ name: g.name, bytes: new Uint8Array(g.bin) }));

/** A response whose body streams `bytes` in chunks of `size`; `read` counts the chunks taken. */
function chunked(bytes: Uint8Array, size: number, read = { n: 0 }): Response {
  let at = 0;
  const body = new ReadableStream<Uint8Array>({
    pull(c) {
      if (at >= bytes.length) return c.close();
      read.n++;
      c.enqueue(bytes.slice(at, (at += size)));
    },
  });
  return new Response(body);
}

describe('decodeMeta', () => {
  it.each(golden.map((g) => [g.name, g] as const))('%s: the same metadata as the full decode, from the prefix alone', (_, g) => {
    const full = decodeDataFile(g.bytes.slice().buffer);
    if (!full.ok) throw new Error(full.error.message);
    const l = new DataView(g.bytes.buffer, g.bytes.byteOffset).getUint32(10, true);
    expect(decodeMeta(g.bytes.slice(0, 14 + l).buffer)).toEqual(full.data.meta);
    expect(decodeMeta(g.bytes.slice(0, 14 + l - 1).buffer)).toBeNull(); // one byte short
    expect(decodeMeta(g.bytes.slice(0, 10).buffer)).toBeNull();
  });

  it('refuses what the full decode refuses: bad magic, another version, no samples', () => {
    const g = golden[0].bytes;
    for (const [at, v] of [[0, 0x58], [4, 99], [6, 0]] as const) {
      const b = g.slice();
      if (at === 6) b.set([0, 0, 0, 0], 6);
      else b[at] = v;
      expect(decodeMeta(b.buffer)).toBeNull();
    }
  });
});

describe('streamMeta', () => {
  it('reads only the prefix and cancels the rest', async () => {
    const g = golden.reduce((a, b) => (b.bytes.length > a.bytes.length ? b : a));
    const read = { n: 0 };
    const meta = await streamMeta(chunked(g.bytes, 512, read));
    const full = decodeDataFile(g.bytes.slice().buffer);
    expect(full.ok && meta).toEqual(full.ok && full.data.meta);
    expect(read.n).toBeLessThan(Math.ceil(g.bytes.length / 512));
  });

  it('null for a refused prefix, a short file or no body; never rejects', async () => {
    expect(await streamMeta(chunked(new TextEncoder().encode('not a data file at all, really'), 7))).toBeNull();
    expect(await streamMeta(chunked(golden[0].bytes.slice(0, 20), 4))).toBeNull();
    expect(await streamMeta(new Response(null))).toBeNull();
    const broken = new Response(new ReadableStream({ pull: (c) => c.error(new Error('net')) }));
    expect(await streamMeta(broken)).toBeNull();
  });
});
