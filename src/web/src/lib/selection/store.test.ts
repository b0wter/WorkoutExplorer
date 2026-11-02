// Selection store (F-17 R-1): the cursor store's rules for a range of two indices.
import { describe, expect, it } from 'vitest';
import { createSelectionStore } from './index';

function setup(n = 10) {
  const queue: (() => void)[] = [];
  const reported: unknown[] = [];
  const store = createSelectionStore(n, (f) => queue.push(f), (e) => reported.push(e));
  const frame = () => queue.splice(0).forEach((f) => f());
  const calls: [number, number][] = [];
  return { store, frame, queue, reported, calls, listener: (a: number, b: number) => calls.push([a, b]) };
}

describe('createSelectionStore', () => {
  it('starts empty and calls a new subscriber synchronously', () => {
    const s = setup();
    expect([s.store.start(), s.store.end()]).toEqual([-1, -1]);
    s.store.subscribe(s.listener);
    expect(s.calls).toEqual([[-1, -1]]);
  });

  it('coalesces writes to one delivery per frame, latest value, only on change', () => {
    const s = setup();
    s.store.subscribe(s.listener);
    s.store.set(1, 3);
    s.store.set(2, 5);
    expect(s.calls).toHaveLength(1);
    expect(s.queue).toHaveLength(1);
    s.frame();
    expect(s.calls).toEqual([[-1, -1], [2, 5]]);
    s.store.set(1, 3);
    s.store.set(2, 5);
    s.frame();
    expect(s.calls).toHaveLength(2);
    s.store.clear();
    s.frame();
    expect(s.calls.at(-1)).toEqual([-1, -1]);
    expect([s.store.start(), s.store.end()]).toEqual([-1, -1]);
  });

  it.each([
    [3, 3],
    [4, 2],
    [-1, 2],
    [0, 10],
    [0.5, 2],
    [Number.NaN, 2],
  ])('rejects (%s, %s) with RangeError', (a, b) => {
    expect(() => setup().store.set(a, b)).toThrow(RangeError);
  });

  it('a throwing listener is reported and the others still get the value', () => {
    const s = setup();
    s.store.subscribe((a) => {
      if (a >= 0) throw new Error('boom');
    });
    s.store.subscribe(s.listener);
    s.store.set(0, 9);
    s.frame();
    expect(s.reported).toHaveLength(1);
    expect(s.calls.at(-1)).toEqual([0, 9]);
  });

  it('unsubscribe stops delivery', () => {
    const s = setup();
    const off = s.store.subscribe(s.listener);
    off();
    off();
    s.store.set(0, 1);
    s.frame();
    expect(s.calls).toEqual([[-1, -1]]);
  });
});
