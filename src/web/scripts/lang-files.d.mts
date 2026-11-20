export const LANG_FILE_LIMIT: number;
export function readTranslations(webRoot: string, langs: readonly string[]): Record<string, Record<string, Record<string, unknown>>>;
export function langFiles(
  groups: Readonly<Record<string, readonly string[]>>,
  translations: Record<string, Record<string, Record<string, unknown>>>,
): { files: Record<string, string>; urls: Record<string, Record<string, string>> };
export function langMetas(page: { groups: readonly string[]; hold: boolean }, urls: Record<string, Record<string, string>>): string;
export function gzipSize(text: string): number;
