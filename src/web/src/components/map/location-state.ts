// F-62 LC-3, LC-8: the pure rules of "show my location" (Node tests in location-state.test.ts).
// The camera moves only where a step says `centre`: the first fix after a tap, a follow tap and
// a fix while following. Everything impure is ./location.

export type Phase = 'off' | 'starting' | 'on';

export interface LocState {
  readonly phase: Phase;
  readonly following: boolean;
  /** An error message was shown since location was turned on (LC-8: once per turning on). */
  readonly said: boolean;
}

/**
 * tap: the location button; fix: a position; follow: the follow button; move: a viewer's pan,
 * drag or zoom; denied: permission denied; failed: no position, timeout, insecure context or no
 * API; off: page closed (pagehide) or map torn down.
 */
export type LocEvent = 'tap' | 'fix' | 'follow' | 'move' | 'denied' | 'failed' | 'off';

export interface LocStep {
  readonly state: LocState;
  readonly centre: boolean;
  readonly message: 'denied' | 'failed' | null;
}

export const LOC_OFF: LocState = { phase: 'off', following: false, said: false };

const step = (state: LocState, centre = false, message: LocStep['message'] = null): LocStep => ({ state, centre, message });

export function locStep(s: LocState, e: LocEvent): LocStep {
  if (e === 'off') return step(LOC_OFF);
  if (e === 'tap') return step(s.phase === 'off' ? { phase: 'starting', following: false, said: false } : LOC_OFF);
  if (s.phase === 'off') return step(s); // a late callback after turning off
  switch (e) {
    case 'fix':
      return s.phase === 'starting' ? step({ ...s, phase: 'on', following: true }, true) : step(s, s.following);
    case 'follow':
      return s.phase === 'on' ? step({ ...s, following: true }, true) : step(s);
    case 'move':
      return step({ ...s, following: false });
    case 'denied':
      return step(LOC_OFF, false, 'denied');
    case 'failed':
      if (s.phase === 'starting') return step(LOC_OFF, false, 'failed');
      return s.said ? step(s) : step({ ...s, said: true }, false, 'failed');
  }
}
