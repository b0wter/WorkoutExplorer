// Vite + Vitest config. Contract: architect/F-01-project-skeleton-and-ci.md, "Vite config".
import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import type { Plugin } from 'vite';

// Typed locally so @types/node does not leak Node globals into the app's type check (F-01 D-4).
declare const process: { env: Record<string, string | undefined> };

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

export default defineConfig({
  plugins: [
    svelte(),
    uplotTrim(),
    // Dev only: every /edit/* and /published/* is edit.html, as Caddy serves it (F-19 E-1, F-20 C-1).
    // The fragment never reaches here.
    {
      name: 'wx-edit-route',
      configureServer(server) {
        server.middlewares.use((req, _res, next) => {
          const r = req as { url?: string }; // no Node types here (F-01 D-4)
          if (r.url?.startsWith('/edit/') || r.url?.startsWith('/published/')) r.url = '/edit.html';
          else if (r.url === '/report-delete') r.url = '/report-delete.html'; // F-37 DK-11
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
    rollupOptions: { input: { index: 'index.html', edit: 'edit.html', upload: 'upload.html', generic: 'generic.html', about: 'about.html', 'report-delete': 'report-delete.html', impressum: 'impressum.html' } },
  },
  test: {
    environment: 'node',
    // uPlot goes through Vite's plugins in tests too, so the chart tests run the trimmed build (wx-uplot-trim).
    server: { deps: { inline: ['uplot'] } },
  },
});
