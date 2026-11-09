// App bundle budget check (ADR-002).
//
// For each entry of the Vite build manifest, counts the entry chunk, every
// chunk it imports statically (transitively) and their CSS, gzips each file at
// level 9 and compares the sum with the budget. Chunks reached only through a
// dynamic import() are listed for information and not counted.
//
// Only node:fs, node:zlib and node:path (ADR-002, decision 6).
// Raising LIMIT_BYTES requires a superseding ADR (ADR-002, decision 7).

import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';

export const LIMIT_BYTES = 60000;

/** Lower ceilings for single entries, by manifest key (ADR-024 item 4: upload page 8,000; F-37 DK-11: delete link page 4,000; F-38 IM-2: Impressum page 2,000). */
export const ENTRY_LIMITS = { 'upload.html': 8000, 'report-delete.html': 4000, 'impressum.html': 2000 };

/** The ceiling that applies to one entry. */
export function limitFor(entry) {
  return ENTRY_LIMITS[entry] ?? LIMIT_BYTES;
}

/**
 * Pure: resolves which files count against each entry's budget.
 *
 * @param {Record<string, {file: string, isEntry?: boolean, imports?: string[],
 *   dynamicImports?: string[], css?: string[]}>} manifest Vite build manifest
 * @returns {{entry: string, files: string[], dynamicFiles: string[]}[]}
 *   per entry (manifest key): counted files and, separately, files reached only
 *   through dynamic imports; paths relative to the build output directory
 */
export function resolveEntries(manifest) {
  const result = [];
  for (const [key, chunk] of Object.entries(manifest)) {
    if (!chunk.isEntry) continue;

    const files = [];
    const staticKeys = new Set();
    const pendingDynamic = [];
    const walkStatic = (k) => {
      if (staticKeys.has(k)) return;
      const c = manifest[k];
      if (!c) throw new Error(`manifest: ${key} references unknown chunk ${k}`);
      staticKeys.add(k);
      pushUnique(files, c.file);
      for (const css of c.css ?? []) pushUnique(files, css);
      for (const imp of c.imports ?? []) walkStatic(imp);
      for (const dyn of c.dynamicImports ?? []) pendingDynamic.push(dyn);
    };
    walkStatic(key);

    // Everything reachable from a dynamic import that is not already counted.
    const dynamicFiles = [];
    const seen = new Set(staticKeys);
    while (pendingDynamic.length > 0) {
      const k = pendingDynamic.pop();
      if (seen.has(k)) continue;
      seen.add(k);
      const c = manifest[k];
      if (!c) throw new Error(`manifest: ${key} references unknown chunk ${k}`);
      for (const f of [c.file, ...(c.css ?? [])]) {
        if (!files.includes(f)) pushUnique(dynamicFiles, f);
      }
      pendingDynamic.push(...(c.imports ?? []), ...(c.dynamicImports ?? []));
    }

    result.push({ entry: key, files, dynamicFiles });
  }
  return result;
}

/**
 * Pure: Vite 8 drops an HTML entry from the manifest when two HTML files load the same module
 * (index.html and edit.html, F-19 E-1): the module becomes a plain shared chunk. Adds each such
 * page back as an entry, keyed by its file name, from its module script's chunk.
 *
 * @param manifest Vite build manifest
 * @param {Record<string, string>} pages HTML file name (relative to the build directory) → text
 */
export function withHtmlEntries(manifest, pages) {
  const out = { ...manifest };
  for (const [name, html] of Object.entries(pages)) {
    if (out[name]?.isEntry) continue;
    const src = html.match(/<script type="module"[^>]*\ssrc="\/([^"]+)"/)?.[1];
    if (!src) continue;
    const key = Object.keys(manifest).find((k) => manifest[k].file === src);
    if (!key) throw new Error(`manifest: ${name} loads ${src}, which is not in the manifest`);
    out[name] = { ...manifest[key], isEntry: true };
  }
  return out;
}

/** Reads the build's top-level HTML files: name → text. */
export function readPages(buildDir) {
  const pages = {};
  for (const f of fs.readdirSync(buildDir)) if (f.endsWith('.html')) pages[f] = fs.readFileSync(path.join(buildDir, f), 'utf8');
  return pages;
}

/** Pure: true when a gzip total fits the budget (limit inclusive). */
export function withinBudget(gzipBytes, limit = LIMIT_BYTES) {
  return gzipBytes <= limit;
}

function pushUnique(list, item) {
  if (!list.includes(item)) list.push(item);
}

function sizes(file) {
  const raw = fs.readFileSync(file);
  return {
    raw: raw.length,
    gzip: zlib.gzipSync(raw, { level: 9 }).length,
    brotli: zlib.brotliCompressSync(raw).length,
  };
}

function line(label, s) {
  return `  ${label}: ${s.raw} raw, ${s.gzip} gzip, ${s.brotli} brotli`;
}

/** Runs the check against a build directory; returns the process exit code. */
export function main(buildDir) {
  const manifestPath = path.join(buildDir, '.vite', 'manifest.json');
  let manifest;
  try {
    manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
  } catch (e) {
    console.error(`bundle size: cannot read ${manifestPath}: ${e.message}`);
    return 1;
  }

  let entries;
  try {
    entries = resolveEntries(withHtmlEntries(manifest, readPages(buildDir)));
  } catch (e) {
    console.error(`bundle size: ${e.message}`);
    return 1;
  }
  if (entries.length === 0) {
    console.error(`bundle size: no entry in ${manifestPath}`);
    return 1;
  }
  // The activity page is the budget's reason; never pass without counting it.
  if (!entries.some((e) => e.entry === 'index.html')) {
    console.error(`bundle size: no index.html entry in ${manifestPath}`);
    return 1;
  }

  try {
    return report(buildDir, entries);
  } catch (e) {
    console.error(`bundle size: ${e.message}`);
    return 1;
  }
}

function report(buildDir, entries) {
  let failed = false;
  for (const { entry, files, dynamicFiles } of entries) {
    let gzip = 0;
    let brotli = 0;
    console.log(`entry ${entry}, counted:`);
    for (const f of files) {
      const s = sizes(path.join(buildDir, f));
      gzip += s.gzip;
      brotli += s.brotli;
      console.log(line(f, s));
    }
    if (dynamicFiles.length > 0) {
      console.log(`entry ${entry}, dynamic imports (not counted):`);
      for (const f of dynamicFiles) console.log(line(f, sizes(path.join(buildDir, f))));
    }
    const limit = limitFor(entry);
    const ok = withinBudget(gzip, limit);
    if (!ok) failed = true;
    console.log(
      `entry ${entry}: ${gzip} / ${limit} bytes gzip (${brotli} brotli) ${ok ? 'PASS' : 'FAIL'}`,
    );
  }
  return failed ? 1 : 0;
}

// Run only when executed directly, not when imported by the test. realpath so a
// symlinked script path still matches import.meta.filename (QA-01-r1-2).
if (process.argv[1] && fs.realpathSync(process.argv[1]) === import.meta.filename) {
  const buildDir = path.resolve(import.meta.dirname, '..', '..', '..', 'build', 'web');
  process.exitCode = main(buildDir);
}
