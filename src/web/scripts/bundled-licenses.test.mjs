import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { addBundled } from './bundled-licenses.mjs';

function pkg(root, name, json, licence) {
  const d = join(root, 'node_modules', name);
  mkdirSync(d, { recursive: true });
  writeFileSync(join(d, 'package.json'), JSON.stringify({ name, ...json }));
  if (licence) writeFileSync(join(d, 'LICENSE'), licence);
}

describe('addBundled', () => {
  it('adds the transitive runtime tree once, with identifier and text', () => {
    const root = mkdtempSync(join(tmpdir(), 'bl-'));
    pkg(root, 'top', { version: '1.0.0', license: 'MIT', dependencies: { a: '1', b: '1' } }, 'top text');
    pkg(root, 'a', { version: '2.0.0', license: 'ISC', dependencies: { b: '1' }, devDependencies: { dev: '1' } }, 'a text');
    pkg(root, 'b', { version: '3.0.0', licenses: [{ type: 'MIT' }, { type: 'Apache-2.0' }] }, 'b text');
    const out = addBundled([{ name: 'top', version: '1.0.0', identifier: 'MIT', text: 'top text' }], root);
    expect(out.map((e) => `${e.name}@${e.version}`)).toEqual(['top@1.0.0', 'a@2.0.0', 'b@3.0.0']);
    expect(out[2]).toMatchObject({ identifier: 'MIT OR Apache-2.0', text: 'b text' });
  });
  it('leaves a missing licence text empty and fails on a missing dependency', () => {
    const root = mkdtempSync(join(tmpdir(), 'bl-'));
    pkg(root, 'top', { version: '1.0.0', dependencies: { a: '1' } });
    pkg(root, 'a', { version: '1.0.0', license: 'MIT' });
    expect(addBundled([{ name: 'top', version: '1.0.0' }], root)[1].text).toBe('');
    pkg(root, 'top', { version: '1.0.0', dependencies: { gone: '1' } });
    expect(() => addBundled([{ name: 'top', version: '1.0.0' }], root)).toThrow(/gone/);
  });
});
