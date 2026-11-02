// Gesture reducer: every row of the F-05 design v1.4 gesture table, the direction
// rule and blocksScroll (ADR-010 rev. 2, D-7, D-18).

import { describe, expect, it } from 'vitest';
import {
  IDLE,
  LONG_PRESS_MS,
  MOUSE_SLOP,
  holdsTouch,
  TOUCH_SLOP,
  blocksScroll,
  step,
  type GestureInput,
  type GestureInputKind,
  type GestureState,
  type TouchAxis,
} from './index';

function input(kind: GestureInputKind, over: Partial<GestureInput> = {}): GestureInput {
  return { kind, pointerType: 'mouse', pointerId: 1, isPrimary: true, index: 10, current: 3, x: 100, y: 200, ...over };
}
const touch = (kind: GestureInputKind, over: Partial<GestureInput> = {}) =>
  input(kind, { pointerType: 'touch', pointerId: 7, ...over });

function touching(axis: TouchAxis = 'open', over: Partial<{ pointerId: number; before: number | null }> = {}): GestureState {
  return { kind: 'touch', pointerId: 7, before: 1, x0: 100, y0: 200, axis, anchor: 10, ...over };
}

/** Runs inputs through the reducer; returns the final state and every action. */
function run(inputs: GestureInput[], start: GestureState = IDLE) {
  let state = start;
  const actions = inputs.map((i) => {
    const r = step(state, i);
    state = r.state;
    return r.action;
  });
  return { state, actions };
}

/** Axis after a touch down at (100, 200) and one move by (dx, dy). */
function axisAfter(dx: number, dy: number): string {
  const r = run([touch('down', { x: 100, y: 200 }), touch('move', { x: 100 + dx, y: 200 + dy })]);
  return r.state.kind === 'touch' ? r.state.axis : 'idle';
}

describe('mouse and pen', () => {
  it.each(['mouse', 'pen'])('%s down sets the index and starts a press (F-17)', (pointerType) => {
    const r = step(IDLE, input('down', { pointerType, index: 4 }));
    expect(r.action).toEqual({ kind: 'set', index: 4 });
    expect(r.state).toEqual({ kind: 'press', pointerId: 1, before: 3, x0: 100, y0: 200, anchor: 4 });
  });

  it.each(['mouse', 'pen'])('%s move (hover or drag) sets the index', (pointerType) => {
    const r = step(IDLE, input('move', { pointerType, index: 12 }));
    expect(r.action).toEqual({ kind: 'set', index: 12 });
    expect(r.state.kind).toBe('idle');
  });

  it.each(['up', 'cancel', 'leave'] as const)('mouse %s does nothing: the cursor stays (D-8)', (kind) => {
    const r = step(IDLE, input(kind));
    expect(r.action).toEqual({ kind: 'none' });
    expect(r.state.kind).toBe('idle');
  });

  it.each(['up', 'cancel', 'leave'] as const)('pen %s does nothing', (kind) => {
    expect(step(IDLE, input(kind, { pointerType: 'pen' })).action).toEqual({ kind: 'none' });
  });

  it('an unknown index (−1) sets nothing', () => {
    expect(step(IDLE, input('move', { index: -1 })).action).toEqual({ kind: 'none' });
  });

  it('mouse hover moves never create a state, however far they go', () => {
    const r = run([input('move', { x: 0, y: 0 }), input('move', { x: 300, y: 0 }), input('move', { x: 0, y: 300 })]);
    expect(r.state).toBe(IDLE);
  });
});

describe('touch', () => {
  it('down sets the cursor under the finger and records before, x0, y0, axis open (P3 option B)', () => {
    const r = step(IDLE, touch('down', { index: 42, current: 5, x: 30, y: 40 }));
    expect(r.action).toEqual({ kind: 'set', index: 42 });
    expect(r.state).toEqual({ kind: 'touch', pointerId: 7, before: 5, x0: 30, y0: 40, axis: 'open', anchor: 42 });
  });

  it('down, move, move, up: sets on down, on each move and on up', () => {
    const r = run([
      touch('down', { index: 19 }),
      touch('move', { index: 20, x: 102 }),
      touch('move', { index: 21, x: 120 }),
      touch('up', { index: 22, x: 125 }),
    ]);
    expect(r.actions).toEqual([
      { kind: 'set', index: 19 },
      { kind: 'set', index: 20 },
      { kind: 'set', index: 21 },
      { kind: 'set', index: 22 },
    ]);
    expect(r.state.kind).toBe('idle');
  });

  it('a tap (down, up) places the cursor at the up index', () => {
    const r = run([touch('down', { index: 8 }), touch('up', { index: 9 })]);
    expect(r.actions[1]).toEqual({ kind: 'set', index: 9 });
  });

  it('down then cancel restores the cursor from before the touch', () => {
    const r = run([touch('down', { current: 5, index: 29 }), touch('move', { index: 30, current: 29 }), touch('cancel', { current: 30 })]);
    expect(r.actions[2]).toEqual({ kind: 'set', index: 5 });
    expect(r.state.kind).toBe('idle');
  });

  it('cancel restores null when there was no cursor before', () => {
    const r = run([touch('down', { current: null }), touch('move', { index: 30 }), touch('cancel', { current: 30 })]);
    expect(r.actions[2]).toEqual({ kind: 'set', index: null });
  });

  it('leave does nothing and keeps the state (fires after up or cancel)', () => {
    const t = touching('scrub');
    expect(step(t, touch('leave'))).toEqual({ state: t, action: { kind: 'none' } });
    expect(step(IDLE, touch('leave'))).toEqual({ state: IDLE, action: { kind: 'none' } });
  });

  it('ignores a second (non-primary) touch pointer throughout', () => {
    const second = { pointerId: 8, isPrimary: false };
    const r = run([
      touch('down', { current: 2, index: 39 }),
      touch('down', second),
      touch('move', { ...second, index: 40 }),
      touch('move', { index: 41 }),
      touch('up', { ...second, index: 42 }),
      touch('cancel', second),
      touch('up', { index: 43 }),
    ]);
    expect(r.actions).toEqual([
      { kind: 'set', index: 39 },
      { kind: 'none' },
      { kind: 'none' },
      { kind: 'set', index: 41 },
      { kind: 'none' },
      { kind: 'none' },
      { kind: 'set', index: 43 },
    ]);
    expect(r.state.kind).toBe('idle');
  });

  it('a primary down with a new pointerId replaces a stale touch: sets the cursor, axis open (v1.1, v1.4)', () => {
    const r = step(touching('scrub'), touch('down', { pointerId: 9, current: 30, index: 31, x: 5, y: 6 }));
    expect(r.action).toEqual({ kind: 'set', index: 31 });
    expect(r.state).toEqual({ kind: 'touch', pointerId: 9, before: 30, x0: 5, y0: 6, axis: 'open', anchor: 31 });
  });

  it('after the replacement, moves of the new ID set the cursor and moves of the old ID are ignored', () => {
    const r = run(
      [
        touch('down', { pointerId: 9, current: 30, index: 49 }),
        touch('move', { pointerId: 9, index: 50 }),
        touch('move', { pointerId: 7, index: 60 }),
        touch('up', { pointerId: 7, index: 61 }),
        touch('cancel', { pointerId: 7 }),
        touch('move', { pointerId: 9, index: 51 }),
        touch('cancel', { pointerId: 9, current: 51 }),
      ],
      touching('scrub'),
    );
    expect(r.actions).toEqual([
      { kind: 'set', index: 49 },
      { kind: 'set', index: 50 },
      { kind: 'none' },
      { kind: 'none' },
      { kind: 'none' },
      { kind: 'set', index: 51 },
      { kind: 'set', index: 30 },
    ]);
    expect(r.state.kind).toBe('idle');
  });

  it('ignores move, up and cancel without a touch in progress', () => {
    for (const kind of ['move', 'up', 'cancel'] as const) {
      expect(step(IDLE, touch(kind))).toEqual({ state: IDLE, action: { kind: 'none' } });
    }
  });

  it('an unknown index (−1) on down, move or up sets nothing', () => {
    const r = run([touch('down', { index: -1 }), touch('move', { index: -1 }), touch('up', { index: -1 })]);
    expect(r.actions).toEqual([{ kind: 'none' }, { kind: 'none' }, { kind: 'none' }]);
    expect(r.state.kind).toBe('idle');
  });
});

describe('direction rule (D-18)', () => {
  it('TOUCH_SLOP is 8 CSS px', () => {
    expect(TOUCH_SLOP).toBe(8);
  });

  it('moves inside the slop (dx² + dy² < 64) keep the axis open', () => {
    expect(axisAfter(7, 0)).toBe('open');
    expect(axisAfter(0, -7)).toBe('open');
    expect(axisAfter(5, 6)).toBe('open'); // 61
    expect(axisAfter(-5.6, 5.6)).toBe('open'); // 62.72
  });

  it.each([
    [8, 0, 'scrub'],
    [-8, 0, 'scrub'],
    [0, 8, 'pan'],
    [0, -8, 'pan'],
    [8, 4.61, 'scrub'],
    [8, 4.62, 'pan'],
    [-8, 4.61, 'scrub'],
    [-8, 4.62, 'pan'],
    [8, -4.61, 'scrub'],
    [8, -4.62, 'pan'],
    [-8, -4.61, 'scrub'],
    [-8, -4.62, 'pan'],
  ])('first move at (%s, %s) → %s', (dx, dy, axis) => {
    expect(axisAfter(dx, dy)).toBe(axis);
  });

  it('classifies on the first move that reaches the slop, after moves inside it', () => {
    const r = run([
      touch('down', { x: 100, y: 200 }),
      touch('move', { x: 103, y: 200 }),
      touch('move', { x: 106, y: 201 }),
      touch('move', { x: 109, y: 201 }),
    ]);
    expect(r.state.kind === 'touch' && r.state.axis).toBe('scrub');
  });

  it("once 'scrub', a later move straight down stays 'scrub'", () => {
    const r = run([
      touch('down', { x: 100, y: 200 }),
      touch('move', { x: 110, y: 200 }),
      touch('move', { x: 110, y: 400, index: 77 }),
    ]);
    expect(r.state.kind === 'touch' && r.state.axis).toBe('scrub');
    expect(r.actions[2]).toEqual({ kind: 'set', index: 77 });
  });

  it("once 'pan', a later horizontal move stays 'pan' and still sets the cursor", () => {
    const r = run([
      touch('down', { x: 100, y: 200 }),
      touch('move', { x: 100, y: 210 }),
      touch('move', { x: 400, y: 210, index: 88 }),
    ]);
    expect(r.state.kind === 'touch' && r.state.axis).toBe('pan');
    expect(r.actions[2]).toEqual({ kind: 'set', index: 88 });
  });
});

const selectState: GestureState = { kind: 'select', pointerId: 7, touch: true, anchor: 10, before: 1, beforeA: -1, beforeB: -1 };

describe('blocksScroll', () => {
  it('is true only for a touch with axis scrub, or a range drag, and exactly one touch', () => {
    expect(blocksScroll(touching('scrub'), 1)).toBe(true);
    expect(blocksScroll(selectState, 1)).toBe(true);
    expect(blocksScroll(selectState, 2)).toBe(false);
    expect(blocksScroll({ ...selectState, touch: false }, 1)).toBe(false); // a mouse drag
  });

  it.each([
    ['idle', IDLE, 1],
    ['open', touching('open'), 1],
    ['pan', touching('pan'), 1],
    ['scrub, 0 touches', touching('scrub'), 0],
    ['scrub, 2 touches', touching('scrub'), 2],
  ] as const)('is false for %s', (_label, state, count) => {
    expect(blocksScroll(state, count)).toBe(false);
  });
});

describe('non-primary pointers', () => {
  it.each(['down', 'move', 'up', 'cancel', 'leave'] as const)('ignores a non-primary %s', (kind) => {
    for (const state of [IDLE, touching('open'), touching('scrub')]) {
      for (const pointerType of ['mouse', 'pen', 'touch']) {
        const r = step(state, input(kind, { pointerType, pointerId: 7, isPrimary: false }));
        expect(r).toEqual({ state, action: { kind: 'none' } });
      }
    }
  });
});

describe('range selection (F-17 R-5)', () => {
  const R = { rangeA: 20, rangeB: 30 };
  const steps = (inputs: GestureInput[], start: GestureState = IDLE) => {
    let state = start;
    return inputs.map((i) => {
      const r = step(state, i);
      state = r.state;
      return r;
    });
  };

  it('constants: 4 px mouse slop, 400 ms long press', () => {
    expect(MOUSE_SLOP).toBe(4);
    expect(LONG_PRESS_MS).toBe(400);
  });

  it('mouse: press, move past the slop, release selects from the press sample, live, cursor following', () => {
    const r = steps([
      input('down', { index: 40, x: 100 }),
      input('move', { index: 41, x: 103 }), // inside the slop: cursor only
      input('move', { index: 50, x: 120 }),
      input('move', { index: 35, x: 80 }),
      input('up', { index: 36, x: 81 }),
    ]);
    expect(r[1].range).toBeUndefined();
    expect(r[1].action).toEqual({ kind: 'set', index: 41 });
    expect(r[2].range).toEqual({ a: 40, b: 50 });
    expect(r[2].state.kind).toBe('select');
    expect(r[3].range).toEqual({ a: 35, b: 40 });
    expect(r[3].action).toEqual({ kind: 'set', index: 35 });
    expect(r[4].range).toEqual({ a: 36, b: 40 });
    expect(r[4].state).toBe(IDLE);
  });

  it('a drag back to the press sample (a = b) keeps the range from before', () => {
    const r = steps([
      input('down', { index: 40, x: 100, ...R }),
      input('move', { index: 50, x: 120, ...R }),
      input('up', { index: 40, x: 100, rangeA: 40, rangeB: 50 }),
    ]);
    expect(r[2].range).toEqual({ a: 20, b: 30 });
  });

  it('mouse click outside the range clears it on up, never on down; inside keeps it', () => {
    const out = steps([input('down', { index: 50, ...R }), input('up', { index: 50, ...R })]);
    expect(out[0].range).toBeUndefined();
    expect(out[1].range).toEqual({ a: -1, b: -1 });
    expect(out[1].action).toEqual({ kind: 'set', index: 50 });
    for (const i of [20, 25, 30]) {
      const inside = steps([input('down', { index: i, ...R }), input('up', { index: i, ...R })]);
      expect(inside[1].range).toBeUndefined();
    }
    expect(steps([input('down', { index: 50 }), input('up', { index: 50 })])[1].range).toBeUndefined(); // no range
  });

  it('mouse hover and leave never touch the range', () => {
    for (const kind of ['move', 'leave', 'up'] as const) expect(step(IDLE, input(kind, { index: 99, ...R })).range).toBeUndefined();
  });

  it('cancel while selecting restores range and cursor from before (mouse and touch)', () => {
    const m = steps([input('down', { index: 40, current: 3, ...R }), input('move', { index: 60, x: 150, ...R }), input('cancel', { rangeA: 40, rangeB: 60 })]);
    expect(m[2]).toEqual({ state: IDLE, action: { kind: 'set', index: 3 }, range: { a: 20, b: 30 } });
    const t = steps([
      touch('down', { index: 40, current: 5, ...R }),
      touch('longpress', { index: -1, ...R }),
      touch('move', { index: 60, x: 150, ...R }),
      touch('cancel', { rangeA: 40, rangeB: 60 }),
    ]);
    expect(t[3]).toEqual({ state: IDLE, action: { kind: 'set', index: 5 }, range: { a: 20, b: 30 } });
  });

  it('touch: long press, then drag selects from the touch-down sample; scroll blocked while selecting', () => {
    const r = steps([
      touch('down', { index: 40, x: 100, y: 200 }),
      touch('move', { index: 41, x: 103, y: 202 }),
      touch('longpress', { index: -1 }),
      touch('move', { index: 70, x: 200, y: 260 }),
      touch('up', { index: 72, x: 205, y: 260 }),
    ]);
    expect(r[2].state.kind).toBe('select');
    expect(r[2].action).toEqual({ kind: 'none' });
    expect(blocksScroll(r[2].state, 1)).toBe(true); // before any movement
    expect(r[3].range).toEqual({ a: 40, b: 70 });
    expect(r[4].range).toEqual({ a: 40, b: 72 });
    expect(r[4].state).toBe(IDLE);
  });

  it('touch: a long press after a scrub or pan, or of another pointer, does nothing', () => {
    for (const axis of ['scrub', 'pan'] as const) {
      const r = step(touching(axis), touch('longpress'));
      expect(r.state.kind).toBe('touch');
      expect(r.range).toBeUndefined();
    }
    expect(step(touching('open'), touch('longpress', { pointerId: 9 })).state.kind).toBe('touch');
    expect(step(IDLE, touch('longpress')).state).toBe(IDLE);
  });

  it('AC 9: a scrub with a range leaves the range unchanged', () => {
    const r = steps([
      touch('down', { index: 40, x: 100, ...R }),
      touch('move', { index: 60, x: 150, ...R }),
      touch('move', { index: 70, x: 180, ...R }),
      touch('up', { index: 70, x: 180, ...R }),
    ]);
    for (const s of r) expect(s.range).toBeUndefined();
    expect(r.map((s) => s.action)).toEqual([40, 60, 70, 70].map((index) => ({ kind: 'set', index })));
  });

  it('a pan (scroll) that starts on a chart and is cancelled never clears', () => {
    const r = steps([touch('down', { index: 90, ...R }), touch('move', { index: 90, y: 260, ...R }), touch('cancel', R)]);
    for (const s of r) expect(s.range).toBeUndefined();
  });

  it('touch tap outside the range clears it; inside, or after a scrub, it stays', () => {
    expect(steps([touch('down', { index: 90, ...R }), touch('up', { index: 90, ...R })])[1].range).toEqual({ a: -1, b: -1 });
    expect(steps([touch('down', { index: 25, ...R }), touch('up', { index: 25, ...R })])[1].range).toBeUndefined();
  });

  it('holdsTouch: while a touch is held or selecting', () => {
    expect(holdsTouch(touching('open'))).toBe(true);
    expect(holdsTouch(selectState)).toBe(true);
    expect(holdsTouch(IDLE)).toBe(false);
  });
});
