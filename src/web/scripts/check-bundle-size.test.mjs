import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { LIMIT_BYTES, limitFor, main, resolveEntries, withHtmlEntries, withinBudget } from './check-bundle-size.mjs';

// Shape of Vite's .vite/manifest.json: one entry, one static import,
// one dynamic import, one CSS file.
const fixture = {
  'index.html': {
    file: 'assets/index-AAAA.js',
    name: 'index',
    src: 'index.html',
    isEntry: true,
    imports: ['_shared-BBBB.js'],
    dynamicImports: ['node_modules/maplibre-gl/index.js'],
    css: ['assets/index-CCCC.css'],
  },
  '_shared-BBBB.js': {
    file: 'assets/shared-BBBB.js',
    name: 'shared',
  },
  'node_modules/maplibre-gl/index.js': {
    file: 'assets/maplibre-DDDD.js',
    name: 'maplibre',
    src: 'node_modules/maplibre-gl/index.js',
    isDynamicEntry: true,
    imports: ['_shared-BBBB.js'],
  },
};

describe('resolveEntries', () => {
  it('counts entry, static imports and CSS, but not dynamic imports', () => {
    expect(resolveEntries(fixture)).toEqual([
      {
        entry: 'index.html',
        files: ['assets/index-AAAA.js', 'assets/index-CCCC.css', 'assets/shared-BBBB.js'],
        dynamicFiles: ['assets/maplibre-DDDD.js'],
      },
    ]);
  });

  it('follows static imports transitively and counts shared chunks once', () => {
    const manifest = {
      'a.html': { file: 'a.js', isEntry: true, imports: ['_b', '_c'] },
      _b: { file: 'b.js', imports: ['_c'], css: ['b.css'] },
      _c: { file: 'c.js' },
    };
    expect(resolveEntries(manifest)[0].files).toEqual(['a.js', 'b.js', 'b.css', 'c.js']);
  });

  it('gives each entry its own set (shared chunks count in each)', () => {
    const manifest = {
      'a.html': { file: 'a.js', isEntry: true, imports: ['_s'] },
      'b.html': { file: 'b.js', isEntry: true, imports: ['_s'] },
      _s: { file: 's.js' },
    };
    expect(resolveEntries(manifest).map((e) => [e.entry, e.files])).toEqual([
      ['a.html', ['a.js', 's.js']],
      ['b.html', ['b.js', 's.js']],
    ]);
  });

  it('returns no entries for a manifest without isEntry', () => {
    expect(resolveEntries({ _x: { file: 'x.js' } })).toEqual([]);
  });

  it('rejects a reference to a missing chunk', () => {
    const manifest = { 'a.html': { file: 'a.js', isEntry: true, imports: ['_gone'] } };
    expect(() => resolveEntries(manifest)).toThrow(/unknown chunk _gone/);
  });
});

describe('withHtmlEntries (Vite 8: two pages, one module, F-19)', () => {
  const shared = {
    '_main-AAAA.js': { file: 'assets/main-AAAA.js', name: 'main', dynamicImports: ['src/edit.ts'], css: ['assets/main-CCCC.css'] },
    'src/edit.ts': { file: 'assets/edit-DDDD.js', isDynamicEntry: true, imports: ['_main-AAAA.js'] },
  };
  const page = '<script type="module" crossorigin src="/assets/main-AAAA.js"></script>';

  it('adds each page as an entry with its script chunk; both count the same set', () => {
    const m = withHtmlEntries(shared, { 'index.html': page, 'edit.html': page, 'generic.html': '<link rel="stylesheet">' });
    const entries = resolveEntries(m);
    expect(entries.map((e) => e.entry).sort()).toEqual(['edit.html', 'index.html']);
    for (const e of entries) {
      expect(e.files).toEqual(['assets/main-AAAA.js', 'assets/main-CCCC.css']);
      expect(e.dynamicFiles).toEqual(['assets/edit-DDDD.js']);
    }
  });

  it('keeps an entry Vite listed and rejects a script that is not in the manifest', () => {
    expect(withHtmlEntries(fixture, { 'index.html': page })).toEqual(fixture);
    expect(() => withHtmlEntries(shared, { 'index.html': '<script type="module" src="/assets/x.js">' })).toThrow(/x\.js/);
    expect(() => withHtmlEntries(shared, { 'index.html': page + page })).toThrow(/2 module scripts/);
  });
});

describe('limitFor', () => {
  it('gives the upload entry 8,000 bytes and every other entry the common limit (ADR-024)', () => {
    expect(limitFor('upload.html')).toBe(8000);
    expect(limitFor('index.html')).toBe(LIMIT_BYTES);
    expect(withinBudget(8000, limitFor('upload.html'))).toBe(true);
    expect(withinBudget(8001, limitFor('upload.html'))).toBe(false);
    expect(limitFor('report-delete.html')).toBe(4000);
    expect(limitFor('impressum.html')).toBe(2000); // F-38 IM-2
    expect(withinBudget(2001, limitFor('impressum.html'))).toBe(false);
  });
});

describe('withinBudget', () => {
  it('uses a 60,000 byte limit, inclusive', () => {
    expect(LIMIT_BYTES).toBe(60000);
    expect(withinBudget(60000)).toBe(true);
    expect(withinBudget(60001)).toBe(false);
  });
});

// Deterministic, incompressible bytes (xorshift32 from a fixed seed).
function pseudoRandomBytes(n, seed = 0x2545f491) {
  const out = Buffer.alloc(n);
  let x = seed >>> 0;
  for (let i = 0; i < n; i++) {
    x ^= x << 13;
    x >>>= 0;
    x ^= x >>> 17;
    x ^= x << 5;
    x >>>= 0;
    out[i] = x & 0xff;
  }
  return out;
}

describe('main', () => {
  let dir;
  let log;
  let error;

  beforeEach(() => {
    dir = fs.mkdtempSync(path.join(os.tmpdir(), 'bundle-size-'));
    log = vi.spyOn(console, 'log').mockImplementation(() => {});
    error = vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
    fs.rmSync(dir, { recursive: true, force: true });
  });

  const writeManifest = (manifest) => {
    fs.mkdirSync(path.join(dir, '.vite'), { recursive: true });
    fs.writeFileSync(path.join(dir, '.vite', 'manifest.json'), JSON.stringify(manifest));
  };
  const writeFile = (name, content) => {
    fs.mkdirSync(path.dirname(path.join(dir, name)), { recursive: true });
    fs.writeFileSync(path.join(dir, name), content);
  };
  const logged = () => log.mock.calls.map((c) => c.join(' '));
  const totalLine = () => logged().filter((l) => l.startsWith('entry index.html: ')).at(-1);

  it('passes a small entry and prints the total line', () => {
    writeFile('assets/index.js', 'console.log("hi");\n');
    writeManifest({ 'index.html': { file: 'assets/index.js', isEntry: true } });

    expect(main(dir)).toBe(0);
    expect(totalLine()).toMatch(/^entry index\.html: \d+ \/ 60000 bytes gzip \(\d+ brotli\) PASS$/);
    expect(error).not.toHaveBeenCalled();
  });

  it('fails an entry over the limit', () => {
    writeFile('assets/index.js', pseudoRandomBytes(100_000));
    writeManifest({ 'index.html': { file: 'assets/index.js', isEntry: true } });

    expect(main(dir)).toBe(1);
    expect(totalLine()).toMatch(/^entry index\.html: \d+ \/ 60000 bytes gzip \(\d+ brotli\) FAIL$/);
  });

  it('does not count a large chunk reached only through a dynamic import', () => {
    writeFile('assets/index.js', 'import("./big.js");\n');
    writeFile('assets/big.js', pseudoRandomBytes(100_000));
    writeManifest({
      'index.html': { file: 'assets/index.js', isEntry: true, dynamicImports: ['big.js'] },
      'big.js': { file: 'assets/big.js', isDynamicEntry: true },
    });

    expect(main(dir)).toBe(0);
    expect(totalLine()).toMatch(/ PASS$/);
  });

  it('fails without a manifest', () => {
    expect(main(dir)).toBe(1);
    expect(error.mock.calls[0][0]).toMatch(/^bundle size: cannot read /);
  });

  it('fails when the manifest has no entry', () => {
    writeFile('assets/x.js', 'x');
    writeManifest({ _x: { file: 'assets/x.js' } });

    expect(main(dir)).toBe(1);
    expect(error.mock.calls[0][0]).toMatch(/^bundle size: no entry/);
  });

  it('fails when the activity page is not among the entries', () => {
    writeFile('assets/upload.js', 'x');
    writeManifest({ 'upload.html': { file: 'assets/upload.js', isEntry: true } });

    expect(main(dir)).toBe(1);
    expect(error.mock.calls[0][0]).toMatch(/^bundle size: no index\.html entry/);
  });

  it('counts index.html from its page when the manifest lists only a shared chunk', () => {
    writeFile('assets/main.js', 'console.log("hi");\n');
    writeFile('index.html', '<script type="module" crossorigin src="/assets/main.js"></script>');
    writeManifest({ '_main.js': { file: 'assets/main.js' } });

    expect(main(dir)).toBe(0);
    expect(totalLine()).toMatch(/ PASS$/);
  });

  it('fails cleanly when a file named in the manifest is missing', () => {
    writeManifest({ 'index.html': { file: 'assets/gone.js', isEntry: true } });

    let code;
    expect(() => {
      code = main(dir);
    }).not.toThrow();
    expect(code).toBe(1);
    expect(error).toHaveBeenCalledTimes(1);
    expect(error.mock.calls[0][0]).toMatch(/^bundle size: .*gone\.js/);
    expect(error.mock.calls[0][0]).not.toContain('\n');
  });
});
