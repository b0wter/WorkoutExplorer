// Types for golden.mjs (F-04 design, Tests).

export interface Golden {
  name: string;
  bin: ArrayBuffer;
  fixture: unknown;
}

/** Every golden pair, sorted by name; throws on an unpaired file, another entry or an empty folder. */
export function loadGolden(): Golden[];

/** F-65: the added-laps agreement cases (Go-built laps per golden and picked samples). */
export function loadAddedLaps(): { name: string; golden: string; at: number[]; laps: unknown[] }[];

/** F-66: the remove-auto-laps agreement cases, as loadAddedLaps. */
export function loadAutoLaps(): { name: string; golden: string; at: number[]; laps: unknown[] }[];
