// F-53 AC 57, MW-7: the viewing page's code neither reads nor writes the My workouts list. Walks the
// static import graph from the entry (main.ts) and checks who may import lib/mine or name its key.
import { describe, expect, it } from 'vitest';

const sources = import.meta.glob(['/src/**/*.ts', '/src/**/*.svelte', '!/src/**/*.test.ts', '!/src/**/testing/**'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>;
/** Paths relative to src/. */
const files = new Map(Object.entries(sources).map(([k, v]) => [k.slice('/src/'.length), v]));

const resolve = (from: string, spec: string): string | undefined => {
  if (!spec.startsWith('.')) return undefined;
  const parts = from.split('/').slice(0, -1);
  for (const p of spec.split('/')) p === '..' ? parts.pop() : p !== '.' && parts.push(p);
  const base = parts.join('/');
  return [base, base + '.ts', base + '/index.ts'].find((c) => files.has(c));
};
/** Static imports only: `import … from '…'`, `import '…'`, `export … from '…'`; never `import('…')`. */
const staticImports = (file: string): string[] =>
  [...files.get(file)!.matchAll(/^\s*(?:import|export)\s(?:[^'"]*?\sfrom\s)?['"]([^'"]+)['"]/gm)].map((m) => resolve(file, m[1])).filter((f): f is string => !!f);

describe('AC 57: the list is out of the viewing page’s reach', () => {
  it('the static graph from main.ts holds neither lib/mine, src/mine nor src/edit', () => {
    const seen = new Set<string>();
    const walk = (f: string): void => {
      if (seen.has(f)) return;
      seen.add(f);
      staticImports(f).forEach(walk);
    };
    walk('main.ts');
    expect(seen.size).toBeGreaterThan(20);
    expect(seen.has('page/start.ts')).toBe(true);
    for (const f of seen) expect(f, f).not.toMatch(/^(lib\/mine|mine|edit)\//);
  });

  it('only src/edit and src/mine import lib/mine; only lib/mine names the key', () => {
    const importers = [...files.keys()].filter((f) => staticImports(f).includes('lib/mine/index.ts'));
    expect(importers.sort()).toEqual(['edit/start.svelte.ts', 'mine/index.ts']);
    expect([...files].filter(([, code]) => code.includes('wx.workouts')).map(([f]) => f)).toEqual(['lib/mine/index.ts']);
  });

  it('main.ts reaches src/mine and src/edit only through import()', () => {
    const main = files.get('main.ts')!;
    expect(main).toContain("import('./mine')");
    expect(main).toContain("import('./edit/start.svelte')");
  });
});

describe('MW-5: stored text is text', () => {
  it('no HTML API, no new window, no target in src/mine or lib/mine', () => {
    for (const f of ['mine/index.ts', 'lib/mine/index.ts']) {
      expect(files.get(f), f).not.toMatch(/innerHTML|outerHTML|insertAdjacentHTML|setHTML|DOMParser|createContextualFragment|document\.write|window\.open|\.target\s*=[^=]|setAttribute/);
    }
  });
});
