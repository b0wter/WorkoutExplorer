// Dev-only decode timing for F-04 AC 5 (F-04 design v1.1, "Decode timing"; ADR-007 rev. 1).
// Times decodeDataFile on the in-memory ArrayBuffer of the synthetic 12-hour
// golden file. Per page load: exactly one cold run (the first decodeDataFile
// call in this page), then 20 warm runs. Cold values are kept in sessionStorage
// across reloads; the trigger is the median of 5 cold runs. Not the network,
// not brotli. Never part of the production build; nothing in src/ imports it.

import { decodeDataFile } from '../src/lib/datafile';
import url from '../../server/testdata/datafile/synthetic/ride-12h.bin?url';

const WARM_RUNS = 20;
const STORAGE_KEY = 'f04-bench-cold';
const MEDIAN_OF = 5;

const button = document.getElementById('run') as HTMLButtonElement;
const reset = document.getElementById('reset') as HTMLButtonElement;
const out = document.getElementById('out') as HTMLPreElement;

let lastResult: string[] = [];

/** Stored cold values; empty when storage is unavailable or holds something else. */
function readCold(): number[] {
  try {
    const parsed: unknown = JSON.parse(sessionStorage.getItem(STORAGE_KEY) ?? '[]');
    return Array.isArray(parsed) ? parsed.filter((v): v is number => typeof v === 'number') : [];
  } catch {
    return [];
  }
}

function appendCold(ms: number): void {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify([...readCold(), ms]));
  } catch {
    // No storage: the page still shows this load's values.
  }
}

function clearCold(): void {
  try {
    sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // Nothing to clear.
  }
}

function median(values: number[]): number {
  const s = [...values].sort((a, b) => a - b);
  const m = Math.floor(s.length / 2);
  return s.length % 2 === 1 ? s[m] : (s[m - 1] + s[m]) / 2;
}

function show(head: string[]): void {
  const cold = readCold();
  const lines = [...head, '', `stored cold ms (${cold.length}): ${cold.map((v) => v.toFixed(1)).join(', ') || '-'}`];
  if (cold.length >= MEDIAN_OF) {
    const last = cold.slice(-MEDIAN_OF);
    lines.push(
      `cold median ms (last ${MEDIAN_OF}): ${median(last).toFixed(1)}`,
      `cold min–max ms (last ${MEDIAN_OF}): ${Math.min(...last).toFixed(1)}–${Math.max(...last).toFixed(1)}`,
    );
  }
  lines.push(`userAgent: ${navigator.userAgent}`);
  out.textContent = lines.join('\n');
}

reset.addEventListener('click', () => {
  clearCold();
  show(lastResult);
});

async function main(): Promise<void> {
  const response = await fetch(url);
  if (!response.ok) throw new Error(`HTTP ${response.status} for ${url}`);
  const buffer = await response.arrayBuffer();
  button.textContent = 'Run';
  button.disabled = false;
  lastResult = [`Loaded ${buffer.byteLength} bytes. Press Run (once per page load).`];
  show(lastResult);

  button.addEventListener(
    'click',
    () => {
      button.disabled = true;
      // The first decodeDataFile call in this page: the cold run.
      const t0 = performance.now();
      const cold = decodeDataFile(buffer);
      const coldMs = performance.now() - t0;
      appendCold(coldMs);

      let total = 0;
      let max = 0;
      let ok = cold.ok;
      for (let i = 0; i < WARM_RUNS; i++) {
        const s = performance.now();
        const r = decodeDataFile(buffer);
        const ms = performance.now() - s;
        total += ms;
        if (ms > max) max = ms;
        ok = ok && r.ok;
      }

      const data = cold.ok ? cold.data : undefined;
      const streams = data ? 1 + (data.position ? 2 : 0) + Object.keys(data.metrics).length : 0;
      lastResult = [
        `ok: ${ok}${cold.ok ? '' : ` (${cold.error.message})`}`,
        `N: ${data?.sampleCount ?? '-'}`,
        `streams decoded: ${streams}`,
        `cold ms (this load): ${coldMs.toFixed(1)}`,
        `warm mean ms (${WARM_RUNS} runs): ${(total / WARM_RUNS).toFixed(2)}`,
        `warm max ms: ${max.toFixed(1)}`,
      ];
      button.textContent = 'Reload the page for the next cold run';
      show(lastResult);
    },
    { once: true },
  );
}

main().catch((e: unknown) => show([`error: ${e instanceof Error ? e.message : String(e)}`]));
