// Types for golden.mjs (F-04 design, Tests).

export interface Golden {
  name: string;
  bin: ArrayBuffer;
  fixture: unknown;
}

/** Every golden pair, sorted by name; throws on an unpaired file, another entry or an empty folder. */
export function loadGolden(): Golden[];
