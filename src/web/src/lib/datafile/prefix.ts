// The data file's metadata from its streamed prefix (F-54 LG-4, ADR-059 item 4). LAZY: the viewing
// page's language pass for a language other than English imports it; English never does.
import { checkHead } from './decode';
import { HEADER_SIZE } from './format';
import { parseMeta } from './meta';
import type { WorkoutMeta } from './model';

/** F-54 LG-4: the metadata from the file's first bytes, checked by rules 1 to 4 as in the full decode;
 *  null while `prefix` is too short for it, or when the file is refused. Never throws. */
export function decodeMeta(prefix: ArrayBuffer): WorkoutMeta | null {
  try {
    if (prefix.byteLength < HEADER_SIZE) return null;
    const head = checkHead(prefix, Infinity);
    if (!head.ok || prefix.byteLength < HEADER_SIZE + head.l) return null;
    const parsed = parseMeta(new Uint8Array(prefix, HEADER_SIZE, head.l));
    return parsed.ok ? parsed.meta : null;
  } catch {
    return null;
  }
}

/**
 * F-54 LG-4: the metadata from the streamed prefix of the same download (`response` is the caller's
 * clone; its branch is cancelled once the metadata is read, so it buffers no more than the prefix). Null
 * without a readable stream or for a refused prefix: the caller waits for the full decode instead.
 * Never rejects.
 */
export async function streamMeta(response: Response): Promise<WorkoutMeta | null> {
  try {
    const reader = response.body?.getReader();
    if (!reader) return null;
    const chunks: Uint8Array[] = [];
    let size = 0;
    for (;;) {
      const { done, value } = await reader.read();
      if (value) {
        chunks.push(value);
        size += value.length;
        const all = new Uint8Array(size);
        let at = 0;
        for (const c of chunks) all.set(c, (at += c.length) - c.length);
        const meta = decodeMeta(all.buffer);
        // The header is in, so the length is known: the metadata or a refusal ends the read.
        if (meta || (size >= 14 && size >= 14 + new DataView(all.buffer).getUint32(10, true))) {
          void reader.cancel();
          return meta;
        }
      }
      if (done) return null;
    }
  } catch {
    return null;
  }
}

