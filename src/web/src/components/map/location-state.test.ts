// F-62 LC-3, LC-8: the location state rules as a table (AC 43 to 47, 49).
import { describe, expect, it } from 'vitest';
import { LOC_OFF, locStep, type LocEvent, type LocState } from './location-state';

const STARTING: LocState = { phase: 'starting', following: false, said: false };
const FOLLOWING: LocState = { phase: 'on', following: true, said: false };
const FREE: LocState = { phase: 'on', following: false, said: false };
const SAID: LocState = { phase: 'on', following: false, said: true };

describe('locStep (F-62 LC-3)', () => {
  const rows: [string, LocState, LocEvent, LocState, boolean, 'denied' | 'failed' | null][] = [
    ['tap turns on, no camera move yet', LOC_OFF, 'tap', STARTING, false, null],
    ['first fix: on, following, centred', STARTING, 'fix', FOLLOWING, true, null],
    ['fix while following: centred', FOLLOWING, 'fix', FOLLOWING, true, null],
    ['fix while not following: never centred', FREE, 'fix', FREE, false, null],
    ['viewer move stops follow mode', FOLLOWING, 'move', FREE, false, null],
    ['follow tap: centred, following again', FREE, 'follow', FOLLOWING, true, null],
    ['follow tap while following: centred again', FOLLOWING, 'follow', FOLLOWING, true, null],
    ['follow before the first fix: nothing', STARTING, 'follow', STARTING, false, null],
    ['tap while on: off, camera untouched', FOLLOWING, 'tap', LOC_OFF, false, null],
    ['tap while starting: off', STARTING, 'tap', LOC_OFF, false, null],
    ['late fix after off: nothing', LOC_OFF, 'fix', LOC_OFF, false, null],
    ['late error after off: nothing', LOC_OFF, 'failed', LOC_OFF, false, null],
    ['follow while off: nothing', LOC_OFF, 'follow', LOC_OFF, false, null],
    ['denied before the first fix: off with the message', STARTING, 'denied', LOC_OFF, false, 'denied'],
    ['failed before the first fix: off with the message', STARTING, 'failed', LOC_OFF, false, 'failed'],
    ['denied after the first fix: off with the message', FOLLOWING, 'denied', LOC_OFF, false, 'denied'],
    ['failed after the first fix: stays on, message once', FREE, 'failed', SAID, false, 'failed'],
    ['failed again: no second message', SAID, 'failed', SAID, false, null],
    ['timeout before the first fix: off with the message', STARTING, 'timeout', LOC_OFF, false, 'failed'],
    ['timeout after the first fix (standing still): nothing', FOLLOWING, 'timeout', FOLLOWING, false, null],
    ['pagehide or teardown: off', FOLLOWING, 'off', LOC_OFF, false, null],
  ];
  for (const [name, from, e, to, centre, message] of rows) {
    it(name, () => expect(locStep(from, e)).toEqual({ state: to, centre, message }));
  }

  it('the camera moves only on the first fix, a follow tap and a fix while following (AC 47)', () => {
    const states = [LOC_OFF, STARTING, FOLLOWING, FREE, SAID, { ...FOLLOWING, said: true }];
    const events: LocEvent[] = ['tap', 'fix', 'follow', 'move', 'denied', 'failed', 'timeout', 'off'];
    for (const s of states)
      for (const e of events) {
        const allowed = (e === 'fix' && (s.phase === 'starting' || (s.phase === 'on' && s.following))) || (e === 'follow' && s.phase === 'on');
        expect(locStep(s, e).centre, `${JSON.stringify(s)} ${e}`).toBe(allowed);
      }
  });

  it('a new tap after a message may show it again (once per turning on)', () => {
    expect(locStep(SAID, 'tap').state).toEqual(LOC_OFF);
    expect(locStep(LOC_OFF, 'tap').state.said).toBe(false);
  });
});

describe('F-62 LC-10: no exit for the position, structurally', () => {
  const src = import.meta.glob(['./location.ts', './location-state.ts', './basemap.ts', '../../edit/*.ts', '!../../edit/*.test.ts'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>;
  const code = (text: string): string => text.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/.*$/gm, '$1');

  it('the two location modules use no network, storage, URL, history, messaging or console API and no lib/prefs', () => {
    for (const f of ['./location.ts', './location-state.ts']) {
      expect(src[f], f).toBeDefined();
      expect(code(src[f]), f).not.toMatch(
        /fetch|XMLHttpRequest|sendBeacon|WebSocket|EventSource|localStorage|sessionStorage|indexedDB|cookie|caches\b|history|location\.(href|hash|search|assign|replace)|postMessage|BroadcastChannel|console|lib\/prefs/,
      );
      expect(code(src[f]), f).not.toMatch(/^export (const|let|var) (?!LOC_OFF)/m); // no exported state
    }
  });

  it('the preview map (SH-12) and the owner glue never import a location module', () => {
    for (const [f, text] of Object.entries(src)) if (!/location/.test(f)) expect(text, f).not.toMatch(/['/]location(-state)?'/);
  });
});
