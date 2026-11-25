// Uploads and publishes the shared read-only fixtures once; tests that change a workout upload their own (helpers.mjs upload()).
import fs from 'node:fs';
import { FIXTURES, fitPath } from './fixtures.mjs';

export default async function setup() {
  const base = process.env.WX_BASE;
  if (!base) throw new Error('WX_BASE is not set: run the suite with make ui-test');
  const ids = {};
  for (const name of FIXTURES) {
    const body = new FormData();
    body.append('file', new Blob([fs.readFileSync(fitPath(name))]), name + '.fit');
    const r = await fetch(base + '/api/workouts', { method: 'POST', body });
    if (!r.ok) throw new Error(`upload ${name}: ${r.status} ${await r.text()}`);
    const j = await r.json();
    const id = j.view_url.split('/').pop();
    const tok = j.edit_url.split('#')[1];
    const p = await fetch(`${base}/api/workouts/${id}/publish`, { method: 'POST', headers: { Authorization: 'Bearer ' + tok } });
    if (!p.ok) throw new Error(`publish ${name}: ${p.status}`);
    ids[name] = { id, tok };
  }
  process.env.WX_WORKOUTS = JSON.stringify(ids);
}
