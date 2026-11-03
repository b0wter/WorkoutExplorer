// The theme script in the heads of index.html and edit.html (DS-4, user 3 Nov 2026): applies the
// stored choice before the first paint. Allowed by its hash in the CSP (ADR-032 item 3), so its
// bytes must be the same in both pages and match the hash in deploy/Caddyfile.
import { describe, expect, it } from 'vitest';
import { readWebFile } from '../../test-support/source.mjs';
import { PREF_KEYS } from '../lib/prefs';

const inline = (html: string): string[] => [...html.matchAll(/<script>([^<]*)<\/script>/g)].map((m) => m[1]);
const [script] = inline(readWebFile('index.html'));

/** Runs the script against a stand-in storage and root element; returns data-theme. */
function run(getItem: (k: string) => string | null): string | undefined {
  const html = { dataset: {} as Record<string, string> };
  new Function('localStorage', 'document', script)({ getItem }, { documentElement: html });
  return html.dataset.theme;
}

describe('theme head script', () => {
  it('is the same single script in both pages, with no template action', () => {
    expect(inline(readWebFile('index.html'))).toEqual([script]);
    expect(inline(readWebFile('edit.html'))).toEqual([script]);
    expect(script).not.toMatch(/\{\{|\/\/|\/\*/); // html/template would rewrite actions or comments
  });

  it('its sha256 is in the CSP that Caddy sends, and in the caddy check', async () => {
    const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(script));
    const hash = `'sha256-${btoa(String.fromCharCode(...new Uint8Array(digest)))}'`;
    expect(readWebFile('../../deploy/Caddyfile')).toContain(`script-src 'self' ${hash};`);
    expect(readWebFile('../server/cmd/workout-explorer/caddy_test.go')).toContain(`script-src 'self' ${hash};`);
  });

  it('sets data-theme only for a stored light or dark, from the prefs key', () => {
    const store = (v: string | null) => (k: string) => (k === PREF_KEYS.theme ? v : null);
    expect(run(store('dark'))).toBe('dark');
    expect(run(store('light'))).toBe('light');
    for (const v of [null, 'system', 'Dark', 'dark ', '']) expect(run(store(v))).toBeUndefined();
  });

  it('is silent when storage throws', () => {
    expect(
      run(() => {
        throw new Error('SecurityError');
      }),
    ).toBeUndefined();
  });
});
