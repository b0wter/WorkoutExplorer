// Adds the runtime dependency tree of every package in Vite's license.json (F-29 AB-5).
// Vite only sees module boundaries, so packages a dependency ships pre-bundled (MapLibre)
// are invisible to it. Over-including a tree-shaken package is fine; missing one is not.
// A package without identifier or text is written with it empty: the Go generator fails on that.
// Usage: node scripts/bundled-licenses.mjs <license.json> [node_modules parent dir]
import { existsSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

function find(name, from) { // node resolution: nearest node_modules/<name> going up
  for (let d = from; ; d = dirname(d)) {
    const dir = join(d, 'node_modules', name);
    if (existsSync(join(dir, 'package.json'))) return dir;
    if (dirname(d) === d) throw new Error(`dependency ${name} (from ${from}) is not installed`);
  }
}

function identifier(j) {
  if (typeof j.license === 'string') return j.license;
  if (j.license?.type) return j.license.type;
  return Array.isArray(j.licenses) ? j.licenses.map((l) => l.type ?? l).join(' OR ') : '';
}

export function addBundled(entries, root) {
  const out = [...entries];
  const seen = new Set(out.map((e) => `${e.name}@${e.version}`));
  const walk = (dir) => {
    const j = JSON.parse(readFileSync(join(dir, 'package.json'), 'utf8'));
    for (const dep of Object.keys(j.dependencies ?? {})) {
      const d = find(dep, dir);
      const dj = JSON.parse(readFileSync(join(d, 'package.json'), 'utf8'));
      const key = `${dj.name}@${dj.version}`;
      if (seen.has(key)) continue;
      seen.add(key);
      const file = readdirSync(d).find((f) => /^(licen[cs]e|copying)/i.test(f));
      out.push({ name: dj.name, version: dj.version, identifier: identifier(dj), text: file ? readFileSync(join(d, file), 'utf8') : '' });
      walk(d);
    }
  };
  for (const e of entries) walk(find(e.name, root));
  return out;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const [file, root = process.cwd()] = process.argv.slice(2);
  if (!file) { console.error('usage: bundled-licenses.mjs <license.json> [root]'); process.exit(2); }
  const entries = JSON.parse(readFileSync(file, 'utf8'));
  const all = addBundled(entries, root);
  writeFileSync(file, JSON.stringify(all, null, 2));
  console.log(`bundled-licenses: ${all.length - entries.length} added to ${entries.length}`);
}
