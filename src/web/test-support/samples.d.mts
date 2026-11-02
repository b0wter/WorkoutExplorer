export function listSamples(): string[];
export function readSample(name: string): string | null;
export function writeSample(name: string, text: string): void;
export function removeSample(name: string): void;
export function updating(): boolean;
export function fileHash(bytes: ArrayBuffer): string;
