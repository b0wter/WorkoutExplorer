// Map gesture reducer (F-06 design, "Route interaction", Tests; ADR-015).

import { describe, expect, it } from 'vitest';
import {
  HOVER_RADIUS_PX,
  ROUTE_IDLE,
  TOUCH_DRAG_RADIUS_PX,
  TOUCH_START_RADIUS_PX,
  hitRadiusPx,
  interceptsTouch,
  routeStep,
  wantsHit,
  type RouteGestureInput,
  type RouteGestureState,
} from './index';

function input(o: Partial<RouteGestureInput>): RouteGestureInput {
  return { kind: 'move', pointerType: 'touch', pointerId: 1, isPrimary: true, buttons: 0, hit: -1, current: null, ...o };
}
const scrub = (pointerId: number, before: number | null): RouteGestureState => ({ kind: 'scrub', pointerId, before });
const NONE = { kind: 'none' };
const set = (index: number | null) => ({ kind: 'set', index });

describe('routeStep: mouse and pen', () => {
  for (const pointerType of ['mouse', 'pen']) {
    it(`${pointerType}: hover on the route sets the cursor and stays idle`, () => {
      expect(routeStep(ROUTE_IDLE, input({ pointerType, hit: 42 }))).toEqual({ state: ROUTE_IDLE, action: set(42) });
    });

    it(`${pointerType}: everything else does nothing and stays idle`, () => {
      const cases: Partial<RouteGestureInput>[] = [
        { kind: 'move', hit: -1 }, // off the route
        { kind: 'move', hit: 42, buttons: 1 }, // dragging the map
        { kind: 'down', hit: 42 },
        { kind: 'up', hit: 42 },
        { kind: 'leave', hit: 42 },
        { kind: 'cancel', hit: 42 },
      ];
      for (const c of cases) {
        expect(routeStep(ROUTE_IDLE, input({ pointerType, ...c }))).toEqual({ state: ROUTE_IDLE, action: NONE });
      }
    });
  }

  it('mouse never enters scrub, also from a stale scrub state', () => {
    expect(routeStep(scrub(3, 7), input({ pointerType: 'mouse', kind: 'down', hit: 5 })).state).toBe(ROUTE_IDLE);
    expect(routeStep(scrub(3, 7), input({ pointerType: 'mouse', kind: 'move', hit: 5 }))).toEqual({
      state: ROUTE_IDLE,
      action: set(5),
    });
  });
});

describe('routeStep: touch', () => {
  it('primary down on the route: sets the cursor, scrub with before = current', () => {
    expect(routeStep(ROUTE_IDLE, input({ kind: 'down', pointerId: 4, hit: 10, current: 99 }))).toEqual({
      state: scrub(4, 99),
      action: set(10),
    });
    expect(routeStep(ROUTE_IDLE, input({ kind: 'down', pointerId: 4, hit: 10, current: null })).state).toEqual(scrub(4, null));
  });

  it('primary down off the route: nothing, idle (the touch belongs to the page and MapLibre)', () => {
    expect(routeStep(ROUTE_IDLE, input({ kind: 'down', hit: -1, current: 5 }))).toEqual({ state: ROUTE_IDLE, action: NONE });
  });

  it('move of the scrubbing pointer: sets on a hit, keeps the cursor without one', () => {
    const s = scrub(4, 1);
    expect(routeStep(s, input({ pointerId: 4, hit: 11 }))).toEqual({ state: s, action: set(11) });
    expect(routeStep(s, input({ pointerId: 4, hit: -1 }))).toEqual({ state: s, action: NONE });
  });

  it('up of the scrubbing pointer: the cursor stays, idle', () => {
    expect(routeStep(scrub(4, 1), input({ kind: 'up', pointerId: 4, hit: 12 }))).toEqual({ state: ROUTE_IDLE, action: NONE });
  });

  it('cancel restores before, including null', () => {
    expect(routeStep(scrub(4, 1), input({ kind: 'cancel', pointerId: 4 }))).toEqual({ state: ROUTE_IDLE, action: set(1) });
    expect(routeStep(scrub(4, null), input({ kind: 'cancel', pointerId: 4 }))).toEqual({
      state: ROUTE_IDLE,
      action: set(null),
    });
  });

  it('a second finger (non-primary down) ends the scrub without an action', () => {
    expect(routeStep(scrub(4, 1), input({ kind: 'down', pointerId: 5, isPrimary: false, hit: 3 }))).toEqual({
      state: ROUTE_IDLE,
      action: NONE,
    });
    expect(routeStep(ROUTE_IDLE, input({ kind: 'down', pointerId: 5, isPrimary: false, hit: 3 }))).toEqual({
      state: ROUTE_IDLE,
      action: NONE,
    });
  });

  it('a primary down with another pointerId over a stale scrub acts as a primary down in idle', () => {
    expect(routeStep(scrub(4, 1), input({ kind: 'down', pointerId: 9, hit: 20, current: 15 }))).toEqual({
      state: scrub(9, 15),
      action: set(20),
    });
    expect(routeStep(scrub(4, 1), input({ kind: 'down', pointerId: 9, hit: -1, current: 15 }))).toEqual({
      state: ROUTE_IDLE,
      action: NONE,
    });
  });

  it('anything else: nothing, state unchanged', () => {
    const s = scrub(4, 1);
    const cases: [RouteGestureState, Partial<RouteGestureInput>][] = [
      [s, { kind: 'move', pointerId: 5, hit: 3 }], // other pointer
      [s, { kind: 'up', pointerId: 5 }],
      [s, { kind: 'cancel', pointerId: 5 }],
      [s, { kind: 'leave', pointerId: 4 }],
      [ROUTE_IDLE, { kind: 'move', hit: 3 }], // a touch move without a scrub
      [ROUTE_IDLE, { kind: 'up', hit: 3 }],
      [ROUTE_IDLE, { kind: 'cancel' }],
      [ROUTE_IDLE, { kind: 'leave' }],
    ];
    for (const [state, c] of cases) expect(routeStep(state, input(c))).toEqual({ state, action: NONE });
  });
});

describe('hitRadiusPx', () => {
  it('per state, pointer type and input kind', () => {
    expect(hitRadiusPx(ROUTE_IDLE, 'mouse')).toBe(HOVER_RADIUS_PX);
    expect(hitRadiusPx(ROUTE_IDLE, 'pen')).toBe(HOVER_RADIUS_PX);
    expect(hitRadiusPx(scrub(1, null), 'mouse')).toBe(HOVER_RADIUS_PX);
    expect(hitRadiusPx(ROUTE_IDLE, 'touch')).toBe(TOUCH_START_RADIUS_PX);
    expect(hitRadiusPx(ROUTE_IDLE, 'touch', 'down')).toBe(TOUCH_START_RADIUS_PX);
    expect(hitRadiusPx(scrub(1, null), 'touch')).toBe(TOUCH_DRAG_RADIUS_PX);
    expect(hitRadiusPx(scrub(1, null), 'touch', 'move')).toBe(TOUCH_DRAG_RADIUS_PX);
    expect(hitRadiusPx(scrub(1, null), 'touch', 'down')).toBe(TOUCH_START_RADIUS_PX); // a new touch over a stale scrub
    expect([HOVER_RADIUS_PX, TOUCH_START_RADIUS_PX, TOUCH_DRAG_RADIUS_PX]).toEqual([12, 20, 48]);
  });
});

describe('hitRadiusPx: kind (v1.2 confirmation)', () => {
  it('a touch down in scrub state gets 20 px, a touch move in scrub 48 px, and the default kind is move', () => {
    const s = scrub(1, 5);
    expect(hitRadiusPx(s, 'touch', 'down')).toBe(20);
    expect(hitRadiusPx(s, 'touch', 'move')).toBe(48);
    expect(hitRadiusPx(s, 'touch')).toBe(hitRadiusPx(s, 'touch', 'move'));
    expect(hitRadiusPx(ROUTE_IDLE, 'touch')).toBe(hitRadiusPx(ROUTE_IDLE, 'touch', 'move'));
    expect(hitRadiusPx(ROUTE_IDLE, 'mouse')).toBe(hitRadiusPx(ROUTE_IDLE, 'mouse', 'move'));
  });
});

describe('wantsHit (v1.3, QA-06-r1-4)', () => {
  const s = scrub(4, 1);
  it('true for a mouse or pen move with no button pressed, in any state', () => {
    for (const state of [ROUTE_IDLE, s]) {
      expect(wantsHit(state, 'move', 'mouse', 1, true, 0)).toBe(true);
      expect(wantsHit(state, 'move', 'pen', 1, true, 0)).toBe(true);
    }
  });

  it('true for a primary touch down, in idle and over a (stale) scrub', () => {
    expect(wantsHit(ROUTE_IDLE, 'down', 'touch', 7, true, 1)).toBe(true);
    expect(wantsHit(s, 'down', 'touch', 7, true, 1)).toBe(true);
  });

  it('true for a move of the scrubbing touch', () => {
    expect(wantsHit(s, 'move', 'touch', 4, true, 1)).toBe(true);
  });

  it('false for every other input', () => {
    const cases: [RouteGestureState, Parameters<typeof wantsHit>[1], string, number, boolean, number][] = [
      [ROUTE_IDLE, 'move', 'touch', 4, true, 1], // touch move in idle (a page scroll over the map)
      [s, 'move', 'touch', 5, true, 1], // touch move of another pointer
      [s, 'move', 'touch', 5, false, 1],
      [ROUTE_IDLE, 'move', 'mouse', 1, true, 1], // mouse drag of the map
      [ROUTE_IDLE, 'move', 'pen', 1, true, 2],
      [ROUTE_IDLE, 'down', 'mouse', 1, true, 1],
      [ROUTE_IDLE, 'down', 'pen', 1, true, 1],
      [s, 'down', 'touch', 5, false, 1], // non-primary down (second finger)
      [ROUTE_IDLE, 'down', 'touch', 5, false, 1],
    ];
    for (const kind of ['up', 'cancel', 'leave'] as const) {
      for (const type of ['mouse', 'pen', 'touch']) {
        cases.push([ROUTE_IDLE, kind, type, 4, true, 0], [s, kind, type, 4, true, 0]);
      }
    }
    for (const c of cases) expect(wantsHit(...c), JSON.stringify(c)).toBe(false);
  });

  it('agrees with routeStep: whenever wantsHit is false, a hit changes nothing', () => {
    const kinds = ['down', 'move', 'up', 'cancel', 'leave'] as const;
    for (const state of [ROUTE_IDLE, s]) {
      for (const kind of kinds) {
        for (const pointerType of ['mouse', 'pen', 'touch']) {
          for (const pointerId of [4, 5]) {
            for (const isPrimary of [true, false]) {
              for (const buttons of [0, 1]) {
                if (wantsHit(state, kind, pointerType, pointerId, isPrimary, buttons)) continue;
                const base = { kind, pointerType, pointerId, isPrimary, buttons, current: 9 };
                expect(routeStep(state, input({ ...base, hit: 3 }))).toEqual(routeStep(state, input({ ...base, hit: -1 })));
              }
            }
          }
        }
      }
    }
  });
});

describe('interceptsTouch', () => {
  it('true only for a scrub with exactly one touch', () => {
    expect(interceptsTouch(scrub(1, null), 1)).toBe(true);
    expect(interceptsTouch(scrub(1, null), 0)).toBe(false);
    expect(interceptsTouch(scrub(1, null), 2)).toBe(false);
    expect(interceptsTouch(ROUTE_IDLE, 1)).toBe(false);
    expect(interceptsTouch(ROUTE_IDLE, 0)).toBe(false);
  });
});
