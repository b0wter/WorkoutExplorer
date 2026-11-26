import fs from 'node:fs';

// Synthetic files only: the parser corpus (src/server/testdata/corpus/synthetic) and the suite's own (corpusgen -e2e).
const CORPUS = '/repo/src/server/testdata/corpus/synthetic';
const EXTRA = '/repo/build/ui-test/fit';

export const FIXTURES = ['ride-30min', 'run-30min', 'indoor-no-gps', 'pool-swim', 'pool-swim-25yd', 'triathlon', 'ride-12h', 'gps-dropout', 'interval-run', 'spike-ride', 'glitch-ride', 'ride-lap-triggers'];

export function fitPath(name) {
  const p = [CORPUS, EXTRA].map((d) => `${d}/${name}.fit`).find((f) => fs.existsSync(f));
  if (!p) throw new Error('no fixture ' + name);
  return p;
}
