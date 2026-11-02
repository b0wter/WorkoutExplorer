// Cursor store (ADR-008; F-05 design, Tests). The scheduler is a manual queue.

import { describe, expect, it } from 'vitest';
import { createCursorStore, type CursorIndex } from './index';

function manualScheduler() {
  const queue: (() => void)[] = [];
  return {
    schedule: (f: () => void) => {
      queue.push(f);
    },
    get scheduled() {
      return queue.length;
    },
    /** Runs every flush queued so far (one frame). */
    frame() {
      const batch = queue.splice(0, queue.length);
      for (const f of batch) f();
    },
  };
}

function setup(n = 10) {
  const s = manualScheduler();
  const store = createCursorStore(n, s.schedule);
  return { s, store };
}

function recorder() {
  const calls: CursorIndex[] = [];
  return { calls, listener: (i: CursorIndex) => calls.push(i) };
}

describe('createCursorStore', () => {
  it('exposes sampleCount and starts with null', () => {
    const { store } = setup(7);
    expect(store.sampleCount).toBe(7);
    expect(store.get()).toBeNull();
  });

  it('calls a new subscriber synchronously with null initially', () => {
    const { store } = setup();
    const r = recorder();
    store.subscribe(r.listener);
    expect(r.calls).toEqual([null]);
  });

  it('calls a new subscriber synchronously with the current value later', () => {
    const { s, store } = setup();
    store.set(3);
    s.frame();
    const r = recorder();
    store.subscribe(r.listener);
    expect(r.calls).toEqual([3]);
  });

  it('never calls a listener synchronously from set', () => {
    const { store } = setup();
    const r = recorder();
    store.subscribe(r.listener);
    store.set(4);
    expect(r.calls).toEqual([null]);
  });

  it('coalesces many sets into one schedule call and one delivery of the latest value', () => {
    const { s, store } = setup();
    const r = recorder();
    store.subscribe(r.listener);
    store.set(1);
    store.set(2);
    store.set(5);
    store.set(3);
    expect(s.scheduled).toBe(1);
    s.frame();
    expect(r.calls).toEqual([null, 3]);
    expect(s.scheduled).toBe(0);
  });

  it('schedules nothing when setting the current value', () => {
    const { s, store } = setup();
    store.set(null);
    expect(s.scheduled).toBe(0);
    store.set(2);
    s.frame();
    store.set(2);
    expect(s.scheduled).toBe(0);
  });

  it('delivers nothing when the value returns to the delivered one within a frame', () => {
    const { s, store } = setup();
    const r = recorder();
    store.set(4);
    s.frame();
    store.subscribe(r.listener);
    store.set(5);
    store.set(4);
    s.frame();
    expect(r.calls).toEqual([4]);
  });

  it('delivers null', () => {
    const { s, store } = setup();
    const r = recorder();
    store.set(2);
    s.frame();
    store.subscribe(r.listener);
    store.set(null);
    s.frame();
    expect(r.calls).toEqual([2, null]);
    expect(store.get()).toBeNull();
  });

  it.each([-1, 10, 1.5, Number.NaN, Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY])(
    'throws RangeError for %s',
    (bad) => {
      const { s, store } = setup(10);
      expect(() => store.set(bad)).toThrow(RangeError);
      expect(store.get()).toBeNull();
      expect(s.scheduled).toBe(0);
    },
  );

  it('accepts 0 and N−1', () => {
    const { store } = setup(10);
    expect(() => store.set(0)).not.toThrow();
    expect(() => store.set(9)).not.toThrow();
  });

  it('rejects an invalid sampleCount', () => {
    expect(() => createCursorStore(-1, () => {})).toThrow(RangeError);
    expect(() => createCursorStore(1.5, () => {})).toThrow(RangeError);
  });

  it('stops delivery after unsubscribe; unsubscribe is idempotent', () => {
    const { s, store } = setup();
    const r = recorder();
    const other = recorder();
    const off = store.subscribe(r.listener);
    store.subscribe(other.listener);
    off();
    off();
    store.set(1);
    s.frame();
    expect(r.calls).toEqual([null]);
    expect(other.calls).toEqual([null, 1]);
  });

  it('does not skip later listeners when one unsubscribes another during a flush', () => {
    const { s, store } = setup();
    const third = recorder();
    const calls: string[] = [];
    let offSecond: () => void = () => {};
    store.subscribe((i) => {
      calls.push(`first ${i}`);
      if (i === 1) offSecond();
    });
    offSecond = store.subscribe((i) => calls.push(`second ${i}`));
    store.subscribe(third.listener);
    store.set(1);
    s.frame();
    // third still gets this flush; second may or may not (design: "may still receive that flush").
    expect(third.calls).toEqual([null, 1]);
    store.set(2);
    s.frame();
    expect(calls).not.toContain('second 2');
    expect(third.calls).toEqual([null, 1, 2]);
  });

  it('gives a listener subscribed during a flush one initial call and no second call for that value', () => {
    const { s, store } = setup();
    const late = recorder();
    store.subscribe((i) => {
      if (i === 6) store.subscribe(late.listener);
    });
    store.set(6);
    s.frame();
    expect(late.calls).toEqual([6]);
    s.frame();
    expect(late.calls).toEqual([6]);
    store.set(7);
    s.frame();
    expect(late.calls).toEqual([6, 7]);
  });

  it('delivers in subscription order', () => {
    const { s, store } = setup();
    const order: string[] = [];
    store.subscribe((i) => order.push(`a${i}`));
    store.subscribe((i) => order.push(`b${i}`));
    store.subscribe((i) => order.push(`c${i}`));
    store.set(2);
    s.frame();
    expect(order).toEqual(['anull', 'bnull', 'cnull', 'a2', 'b2', 'c2']);
  });

  it('get returns the latest value before the flush', () => {
    const { s, store } = setup();
    store.set(8);
    expect(store.get()).toBe(8);
    store.set(1);
    expect(store.get()).toBe(1);
    s.frame();
    expect(store.get()).toBe(1);
  });

  it('a set from inside a listener is delivered in the next flush', () => {
    const { s, store } = setup();
    const r = recorder();
    store.subscribe((i) => {
      if (i === 1) store.set(2);
    });
    store.subscribe(r.listener);
    store.set(1);
    s.frame();
    expect(r.calls).toEqual([null, 1]);
    expect(s.scheduled).toBe(1);
    s.frame();
    expect(r.calls).toEqual([null, 1, 2]);
  });

  describe('v1.2 (QA-05-r1-2)', () => {
    it('a subscriber between set and flush gets get(), not the last delivered value', () => {
      const { s, store } = setup();
      store.set(3);
      s.frame();
      store.set(7);
      const r = recorder();
      store.subscribe(r.listener);
      expect(r.calls).toEqual([7]);
    });

    it('A unsubscribes itself during a flush: B and C still get that value', () => {
      const { s, store } = setup();
      const b = recorder();
      const c = recorder();
      const aCalls: CursorIndex[] = [];
      const offA = store.subscribe((i) => {
        aCalls.push(i);
        if (i === 1) offA();
      });
      store.subscribe(b.listener);
      store.subscribe(c.listener);
      store.set(1);
      s.frame();
      expect(b.calls).toEqual([null, 1]);
      expect(c.calls).toEqual([null, 1]);
      store.set(2);
      s.frame();
      expect(aCalls).toEqual([null, 1]);
      expect(b.calls).toEqual([null, 1, 2]);
    });

    // With splice on the live array, removing a listener BEFORE the current one shifts
    // C into the slot already visited; removing one after it shifts C into B's slot.
    it.each([
      ['B subscribed before A', true],
      ['B subscribed after A', false],
    ])('A unsubscribes B during a flush (%s): C still gets that value', (_label, bFirst) => {
      const { s, store } = setup();
      const c = recorder();
      let offB: () => void = () => {};
      const b = () => {};
      const a = (i: CursorIndex) => {
        if (i === 1) offB();
      };
      if (bFirst) {
        offB = store.subscribe(b);
        store.subscribe(a);
      } else {
        store.subscribe(a);
        offB = store.subscribe(b);
      }
      store.subscribe(c.listener);
      store.set(1);
      s.frame();
      expect(c.calls).toEqual([null, 1]);
    });
  });

  describe('listener errors (D-14)', () => {
    function setupWithReport() {
      const s = manualScheduler();
      const reported: unknown[] = [];
      const store = createCursorStore(10, s.schedule, (e) => reported.push(e));
      return { s, store, reported };
    }

    it('a throwing listener does not stop delivery; report gets the error once; A gets the next flush', () => {
      const { s, store, reported } = setupWithReport();
      const boom = new Error('boom');
      const aCalls: CursorIndex[] = [];
      store.subscribe((i) => {
        aCalls.push(i);
        if (i === 3) throw boom;
      });
      const b = recorder();
      const c = recorder();
      store.subscribe(b.listener);
      store.subscribe(c.listener);
      store.set(3);
      expect(() => s.frame()).not.toThrow();
      expect(b.calls).toEqual([null, 3]);
      expect(c.calls).toEqual([null, 3]);
      expect(reported).toEqual([boom]);
      store.set(4);
      s.frame();
      expect(aCalls).toEqual([null, 3, 4]);
      expect(b.calls).toEqual([null, 3, 4]);
      expect(reported).toEqual([boom]);
    });

    it('after A throws on 5, set(null) is delivered to A and B (QA-05-r2-3 d)', () => {
      const { s, store, reported } = setupWithReport();
      const aCalls: CursorIndex[] = [];
      store.subscribe((i) => {
        aCalls.push(i);
        if (i === 5) throw new Error('five');
      });
      const b = recorder();
      store.subscribe(b.listener);
      store.set(5);
      s.frame();
      store.set(null);
      s.frame();
      expect(aCalls).toEqual([null, 5, null]);
      expect(b.calls).toEqual([null, 5, null]);
      expect(reported).toHaveLength(1);
    });

    it('report is never called when no listener throws', () => {
      const { s, store, reported } = setupWithReport();
      store.subscribe(() => {});
      for (const i of [1, 2, null, 5]) {
        store.set(i);
        s.frame();
      }
      expect(reported).toEqual([]);
    });

    it('the initial call in subscribe is not wrapped', () => {
      const { store, reported } = setupWithReport();
      expect(() =>
        store.subscribe(() => {
          throw new Error('initial');
        }),
      ).toThrow('initial');
      expect(reported).toEqual([]);
    });

    it('the default report rethrows in a microtask', async () => {
      const s = manualScheduler();
      const store = createCursorStore(10, s.schedule);
      const boom = new Error('later');
      store.subscribe((i) => {
        if (i === 1) throw boom;
      });
      const original = globalThis.queueMicrotask;
      const queued: (() => void)[] = [];
      globalThis.queueMicrotask = (f: () => void) => {
        queued.push(f);
      };
      try {
        store.set(1);
        expect(() => s.frame()).not.toThrow();
      } finally {
        globalThis.queueMicrotask = original;
      }
      expect(queued).toHaveLength(1);
      expect(() => queued[0]()).toThrow(boom);
    });
  });
});
