// @vitest-environment jsdom
// Touch-quiet gate (F-06 design v1.3, Tests; ADR-013 rev. 1 decision 5; ADR-012). Fake timers
// for setTimeout, a manual requestAnimationFrame; no timing or touch behaviour is asserted,
// only which events keep whenQuiet() pending.
//
// jsdom has TouchEvent but no Touch, so `touches` cannot be filled: a plain Event with a
// `touches` property stands in (ADR-012 rev. 2). The gate reads only `touches.length`.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { QUIET_MS, createTouchQuiet, type TouchQuiet } from './touch-quiet';

const frames: FrameRequestCallback[] = [];
function frame(): void {
  const batch = frames.splice(0);
  for (const f of batch) f(0);
}

let quiet: TouchQuiet | null = null;

beforeEach(() => {
  vi.useFakeTimers();
  window.requestAnimationFrame = (f: FrameRequestCallback): number => {
    frames.push(f);
    return frames.length;
  };
});

afterEach(() => {
  quiet?.dispose();
  quiet = null;
  frames.length = 0;
  vi.useRealTimers();
  vi.restoreAllMocks();
});

/** A touch event on window with `touches` fingers still on the screen after it. */
function touch(type: 'touchstart' | 'touchend' | 'touchcancel', touches: number): void {
  const ev = new Event(type, { bubbles: true });
  Object.defineProperty(ev, 'touches', { value: { length: touches } });
  window.dispatchEvent(ev);
}

function pointer(type: string, o: Partial<PointerEventInit> = {}): void {
  window.dispatchEvent(new PointerEvent(type, { pointerType: 'touch', pointerId: 1, isPrimary: true, ...o }));
}

/** whenQuiet() with a flag that turns true when it resolves. */
function waiting(q: TouchQuiet): { done: () => boolean } {
  let done = false;
  void q.whenQuiet().then(() => {
    done = true;
  });
  return { done: () => done };
}

/** One animation frame, then the zero timeout after it, then pending promise callbacks. */
async function frameAndTimeout(): Promise<void> {
  frame();
  await vi.advanceTimersByTimeAsync(0);
}

describe('createTouchQuiet', () => {
  it('resolves after one frame and a zero timeout when nothing happened', async () => {
    quiet = createTouchQuiet(window);
    const w = waiting(quiet);
    await Promise.resolve();
    expect(w.done()).toBe(false);
    frame();
    await Promise.resolve();
    expect(w.done()).toBe(false); // the timeout after the frame is still pending
    await vi.advanceTimersByTimeAsync(0);
    expect(w.done()).toBe(true);
  });

  it('stays pending after touchstart; resolves only QUIET_MS after the touchend that brings the count to 0', async () => {
    quiet = createTouchQuiet(window);
    touch('touchstart', 1);
    const w = waiting(quiet);
    await vi.advanceTimersByTimeAsync(10_000);
    await frameAndTimeout();
    expect(w.done()).toBe(false);

    touch('touchend', 0);
    await vi.advanceTimersByTimeAsync(QUIET_MS - 1);
    await frameAndTimeout();
    expect(w.done()).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(w.done()).toBe(false); // the frame comes first
    await frameAndTimeout();
    expect(w.done()).toBe(true);
    expect(QUIET_MS).toBe(250);
  });

  it('a pointercancel (the browser took the touch for a scroll) does not open the gate', async () => {
    quiet = createTouchQuiet(window);
    pointer('pointerdown');
    touch('touchstart', 1);
    const w = waiting(quiet);
    pointer('pointercancel'); // scroll started, the finger is still down
    await vi.advanceTimersByTimeAsync(10_000);
    await frameAndTimeout();
    expect(w.done()).toBe(false);
    touch('touchend', 0); // the physical lift
    await vi.advanceTimersByTimeAsync(QUIET_MS);
    await frameAndTimeout();
    expect(w.done()).toBe(true);
  });

  it('a touchend with one finger still down keeps it pending', async () => {
    quiet = createTouchQuiet(window);
    touch('touchstart', 1);
    touch('touchstart', 2);
    touch('touchend', 1);
    const w = waiting(quiet);
    await vi.advanceTimersByTimeAsync(10_000);
    await frameAndTimeout();
    expect(w.done()).toBe(false);
    touch('touchend', 0);
    await vi.advanceTimersByTimeAsync(QUIET_MS);
    await frameAndTimeout();
    expect(w.done()).toBe(true);
  });

  it('touchcancel that brings the count to 0 ends the touch like touchend', async () => {
    quiet = createTouchQuiet(window);
    touch('touchstart', 1);
    const w = waiting(quiet);
    touch('touchcancel', 0);
    await vi.advanceTimersByTimeAsync(QUIET_MS - 1);
    await frameAndTimeout();
    expect(w.done()).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    await frameAndTimeout();
    expect(w.done()).toBe(true);
  });

  it('a later touchstart with a lower count corrects a missed end', async () => {
    quiet = createTouchQuiet(window);
    touch('touchstart', 1);
    touch('touchstart', 2); // both ends are never seen
    const w = waiting(quiet);
    touch('touchstart', 1); // a new single finger: the count is 1, not 3
    touch('touchend', 0);
    await vi.advanceTimersByTimeAsync(QUIET_MS);
    await frameAndTimeout();
    expect(w.done()).toBe(true);
  });

  it('a new finger during the cool-down restarts the wait', async () => {
    quiet = createTouchQuiet(window);
    touch('touchstart', 1);
    touch('touchend', 0);
    const w = waiting(quiet);
    await vi.advanceTimersByTimeAsync(200);
    touch('touchstart', 1);
    touch('touchend', 0);
    await vi.advanceTimersByTimeAsync(200);
    await frameAndTimeout();
    expect(w.done()).toBe(false);
    await vi.advanceTimersByTimeAsync(50);
    await frameAndTimeout();
    expect(w.done()).toBe(true);
  });

  it('a finger that comes down between the frame and the timeout keeps it pending', async () => {
    quiet = createTouchQuiet(window);
    const w = waiting(quiet);
    frame();
    touch('touchstart', 1);
    await vi.advanceTimersByTimeAsync(0);
    expect(w.done()).toBe(false);
    touch('touchend', 0);
    await vi.advanceTimersByTimeAsync(QUIET_MS);
    await frameAndTimeout();
    expect(w.done()).toBe(true);
  });

  it('pointer events of any type never delay', async () => {
    quiet = createTouchQuiet(window);
    pointer('pointerdown', { pointerType: 'touch' });
    pointer('pointerdown', { pointerType: 'mouse', pointerId: 2 });
    pointer('pointerdown', { pointerType: 'pen', pointerId: 3 });
    const w = waiting(quiet);
    await frameAndTimeout();
    expect(w.done()).toBe(true);
    pointer('pointerup', { pointerType: 'touch' });
    pointer('pointercancel', { pointerType: 'touch', pointerId: 4 });
    const w2 = waiting(quiet);
    await frameAndTimeout();
    expect(w2.done()).toBe(true); // no cool-down from pointer events either
  });

  it('visibilitychange to hidden sets the count to 0', async () => {
    quiet = createTouchQuiet(window);
    touch('touchstart', 1);
    const w = waiting(quiet);
    await frameAndTimeout();
    expect(w.done()).toBe(false);
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
    document.dispatchEvent(new Event('visibilitychange'));
    await frameAndTimeout();
    expect(w.done()).toBe(false); // visible: nothing changes
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden');
    document.dispatchEvent(new Event('visibilitychange'));
    await frameAndTimeout();
    expect(w.done()).toBe(true);
  });

  it('dispose removes every listener with the same target, type, listener and capture flag', () => {
    type Call = { target: EventTarget; type: string; fn: unknown; capture: boolean; passive: boolean | undefined };
    const added: Call[] = [];
    const removed: Call[] = [];
    const record = (list: Call[], target: EventTarget, type: string, fn: unknown, opts?: boolean | AddEventListenerOptions) =>
      list.push({
        target,
        type,
        fn,
        capture: typeof opts === 'boolean' ? opts : !!opts?.capture,
        passive: typeof opts === 'object' ? opts.passive : undefined,
      });
    // jsdom defines add/removeEventListener on window itself, and EventTarget.prototype for the document.
    for (const target of [window, document]) {
      const add = target.addEventListener.bind(target);
      const remove = target.removeEventListener.bind(target);
      vi.spyOn(target, 'addEventListener').mockImplementation(
        (type: string, fn: EventListenerOrEventListenerObject, opts?: boolean | AddEventListenerOptions) => {
          record(added, target, type, fn, opts);
          add(type, fn, opts);
        },
      );
      vi.spyOn(target, 'removeEventListener').mockImplementation(
        (type: string, fn: EventListenerOrEventListenerObject, opts?: boolean | EventListenerOptions) => {
          record(removed, target, type, fn, opts);
          remove(type, fn, opts);
        },
      );
    }

    const q = createTouchQuiet(window);
    expect(added.map((c) => c.type).sort()).toEqual(['touchcancel', 'touchend', 'touchstart', 'visibilitychange']);
    expect(added.every((c) => c.capture && c.passive === true)).toBe(true);
    expect(added.filter((c) => c.type.startsWith('touch')).every((c) => c.target === window)).toBe(true);
    q.dispose();
    for (const a of added) {
      const match = removed.some((r) => r.target === a.target && r.type === a.type && r.fn === a.fn && r.capture === a.capture);
      expect(match, `${a.type} removed with capture ${a.capture}`).toBe(true);
    }
  });

  it('after dispose, nothing resolves and touches are no longer tracked', async () => {
    quiet = createTouchQuiet(window);
    const w = waiting(quiet);
    quiet.dispose();
    touch('touchstart', 1);
    await frameAndTimeout();
    expect(w.done()).toBe(false);
  });
});
