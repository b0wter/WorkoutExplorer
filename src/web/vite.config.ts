// Vite + Vitest config. Contract: architect/F-01-project-skeleton-and-ci.md, "Vite config".
import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import type { Plugin } from 'vite';
import { langFiles, langMetas, readTranslations } from './scripts/lang-files.mjs';
import { GROUPS, PAGES, TRANSLATED } from './src/i18n/groups';

// Typed locally so @types/node does not leak Node globals into the app's type check (F-01 D-4).
declare const process: { env: Record<string, string | undefined>; cwd(): string };

// API address for the dev proxy (F-01 "Local development address"). The default
// must equal the Makefile's DEV_ADDR default and Go's defaultAddr in main.go.
const devAddr = process.env.DEV_ADDR || '127.0.0.1:8080';

// uPlot statics the app never uses (only uPlot.pxRatio is read; ADR-009, ADR-011 C1). Blanking their
// assignments lets the minifier drop the code. Each must occur exactly once and the file must be seen,
// so a uPlot upgrade that moves or rewrites them fails the build and the tests instead of breaking or
// silently not saving. Blanked with spaces: line and column positions stay the same.
const UPLOT_UNUSED = [
  'uPlot.rangeLog = rangeLog;',
  'uPlot.rangeAsinh = rangeAsinh;',
  'uPlot.join = join;',
  'uPlot.fmtDate = fmtDate;',
  'uPlot.tzDate  = tzDate;',
  'uPlot.sync = _sync;',
  '(paths.stepped = stepped);',
  '(paths.bars    = bars);',
  '(paths.spline  = monotoneCubic);',
];
function uplotTrim(): Plugin {
  let seen = false;
  return {
    name: 'wx-uplot-trim',
    transform(code, id) {
      if (!id.endsWith('/node_modules/uplot/dist/uPlot.esm.js')) return;
      seen = true;
      for (const p of UPLOT_UNUSED) {
        if (code.split(p).length !== 2) this.error(`wx-uplot-trim: "${p}" not found exactly once in ${id}; update UPLOT_UNUSED for this uPlot version`);
        code = code.replace(p, ' '.repeat(p.length));
      }
      return { code, map: null };
    },
    generateBundle() {
      if (!seen) this.error('wx-uplot-trim: uplot/dist/uPlot.esm.js was never transformed; did uPlot change its entry file?');
    },
  };
}

/** F-54 LG-2, LG-3: emits one content-hashed JSON file per text group and language into assets/,
 *  and writes the head elements naming a page's files before its <title> (the head script reads
 *  them). The dev server serves the same files from memory. */
function langFilesPlugin(): Plugin {
  const webRoot = process.cwd(); // npm runs Vite and Vitest in src/web
  const build = () => langFiles(GROUPS, readTranslations(webRoot, TRANSLATED));
  let out = build();
  return {
    name: 'wx-lang-files',
    buildStart() {
      out = build();
      for (const l of TRANSLATED) this.addWatchFile(`${webRoot}/src/i18n/${l}.json`);
    },
    transformIndexHtml(html, ctx) {
      const page = PAGES[ctx.filename.slice(ctx.filename.lastIndexOf('/') + 1)];
      if (!page) return html;
      if (html.split('<title>').length !== 2) throw new Error(`wx-lang-files: ${ctx.filename} has not exactly one <title>`);
      return html.replace('<title>', langMetas(page, out.urls) + '\n    <title>');
    },
    generateBundle() {
      for (const [fileName, source] of Object.entries(out.files)) this.emitFile({ type: 'asset', fileName, source });
    },
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const text = out.files[((req as { url?: string }).url ?? '').slice(1)];
        if (text === undefined) return next();
        (res as unknown as { setHeader(k: string, v: string): void; end(t: string): void }).setHeader('Content-Type', 'application/json');
        (res as unknown as { end(t: string): void }).end(text);
      });
    },
  };
}

/** LG-5, ADR-024 item 3: the upload and static entries keep their own copy of every lib module they
 *  import (the i18n core, the prefs key), so no chunk is shared with the activity entry. A module
 *  reached from them is resolved with a `?wx=<entry>` suffix, which makes it a module of its own. */
function entryCopies(): Plugin {
  return {
    name: 'wx-entry-copy',
    enforce: 'pre',
    async resolveId(source, importer, options) {
      if (!importer) return;
      const tag = /[?&]wx=(\w+)/.exec(importer)?.[1] ?? /\/src\/(upload|static)\//.exec(importer)?.[1];
      if (!tag) return;
      const r = await this.resolve(source, importer.split('?')[0], { ...options, skipSelf: true });
      if (!r || r.external || !/\/src\/lib\/.*\.ts$/.test(r.id)) return r;
      return r.id + '?wx=' + tag;
    },
  };
}

export default defineConfig({
  plugins: [
    svelte(),
    uplotTrim(),
    langFilesPlugin(),
    entryCopies(),
    // Dev only: every /edit/* and /published/* is edit.html, as Caddy serves it (F-19 E-1, F-20 C-1).
    // The fragment never reaches here.
    {
      name: 'wx-edit-route',
      configureServer(server) {
        server.middlewares.use((req, _res, next) => {
          const r = req as { url?: string }; // no Node types here (F-01 D-4)
          if (r.url?.startsWith('/edit/') || r.url?.startsWith('/published/')) r.url = '/edit.html';
          else if (r.url === '/report-delete') r.url = '/report-delete.html'; // F-37 DK-11
          else if (r.url === '/my-workouts') r.url = '/my-workouts.html'; // F-53 MW-1
          next();
        });
      },
    },
  ],
  // Component smoke tests (*.dom.test.ts, ADR-012) need Svelte's client build, not its
  // server build: the browser condition, under Vitest only. Never affects `vite build`.
  // mainFields: tests load uPlot's ESM file, the one the build trims (wx-uplot-trim), not its CJS main.
  resolve: process.env.VITEST ? { conditions: ['browser'], mainFields: ['module'] } : undefined,
  server: {
    proxy: {
      '/api': `http://${devAddr}`,
    },
    // The dev server may serve this folder and the synthetic golden data files
    // (for the bench page, bench/decode.html), nothing else outside src/web.
    // architect/F-04-workout-data-file-reader.md, "Decode timing".
    fs: {
      allow: ['.', '../server/testdata/datafile/synthetic'],
    },
  },
  // Pre-bundled at dev-server start, so the first import('./map-view') on bench/map.html does
  // not make Vite re-optimise and reload the page (F-06 design, "Dev page"). Dev only.
  optimizeDeps: {
    include: ['maplibre-gl'],
  },
  build: {
    outDir: '../../build/web',
    emptyOutDir: true,
    // Read by scripts/check-bundle-size.mjs (ADR-002).
    manifest: true,
    // Attribution list input (F-29 AB-5): per bundled package name, version, SPDX id and text. Not served.
    license: { fileName: '.vite/license.json' },
    // Six entries (F-29 AB-1: the About page, CSS only): the activity page, the edit page (F-19 E-1, the same entry module), the upload
    // page (ADR-024), the static generic page (F-15 P-12, CSS only) and the delete link's page (F-37 DK-11, plain TypeScript as the
    // upload page). Relative to the project root.
    // No module-preload polyfill: it would become a chunk both entries share (ADR-024 item 3).
    modulePreload: { polyfill: false },
    // One script for the activity page (F-54): index.html, edit.html and my-workouts.html share
    // main.ts, so Rolldown would put modules that the entry and lazy chunks share (lib/i18n, lib/format,
    // the text tables) into extra chunks, each a second module script. Every module the three load
    // up front stays in the one main chunk; the other entries (own copies, wx-entry-copy) and lazy chunks are untouched.
    rollupOptions: {
      input: { index: 'index.html', edit: 'edit.html', upload: 'upload.html', generic: 'generic.html', about: 'about.html', 'report-delete': 'report-delete.html', 'my-workouts': 'my-workouts.html', impressum: 'impressum.html' },
      output: {
        codeSplitting: {
          groups: [{ name: 'main', tags: ['$initial'], test: (id: string) => !/\.html($|\?)|[?&]wx=|[\\/]src[\\/](upload|static|report-delete|impressum|about|generic|theme)[\\/]/.test(id) }],
        },
      },
    },
  },
  test: {
    environment: 'node',
    // uPlot goes through Vite's plugins in tests too, so the chart tests run the trimmed build (wx-uplot-trim).
    server: { deps: { inline: ['uplot'] } },
  },
});
