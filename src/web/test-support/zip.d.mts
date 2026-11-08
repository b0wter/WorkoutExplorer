// Types for zip.mjs (F-35 tests).

export interface Entry {
  name: string | Uint8Array;
  data?: Uint8Array;
  method?: number;
  flags?: number;
  descriptor?: 'sig' | 'nosig';
  extra?: string | Uint8Array;
  packed?: Uint8Array;
  crc?: number;
  size?: number;
  packedSize?: number;
  localName?: string | Uint8Array;
  localMethod?: number;
  sameLocalAs?: number;
}

export interface Built {
  bytes: Uint8Array<ArrayBuffer>;
  cd: number;
  end: number;
  locals: number[];
  centrals: number[];
}

export function syntheticFits(): { name: string; bytes: Uint8Array<ArrayBuffer> }[];
export function crc32(b: Uint8Array): number;
export function deflate(b: Uint8Array): Uint8Array<ArrayBuffer>;
export function bomb(mib: number): Uint8Array<ArrayBuffer>;
export function zip(entries: Entry[], opts?: { comment?: string | Uint8Array }): Built;
