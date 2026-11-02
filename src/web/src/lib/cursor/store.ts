// The shared cursor store (ADR-008; F-05 design, "Cursor store").
// Framework-free: holds a sample index or null, delivers the latest value once
// per scheduled flush (production: one animation frame). No Svelte, no DOM.

/** A sample index into the WorkoutData arrays, or no cursor. */
export type CursorIndex = number | null;
export type CursorListener = (index: CursorIndex) => void;
/** Production: (f) => requestAnimationFrame(() => f()). Tests: a manual queue. */
export type Schedule = (flush: () => void) => void;
/** Receives an error thrown by a listener during delivery (D-14). */
export type ReportError = (error: unknown) => void;

/** Default: rethrow in a microtask, so the error reaches the console and window.onerror unchanged. */
const rethrowLater: ReportError = (error) => {
  queueMicrotask(() => {
    throw error;
  });
};

export interface CursorStore {
  readonly sampleCount: number;
  /** Latest value passed to set (may be newer than the last one delivered). */
  get(): CursorIndex;
  /** index: integer with 0 ≤ index < sampleCount, or null. Same value as get(): no-op.
   *  Anything else throws RangeError (a caller bug). Never calls listeners synchronously. */
  set(index: CursorIndex): void;
  /** Calls listener once, synchronously, with get(); then once per flush in which the
   *  delivered value changed. Returns the unsubscribe function. */
  subscribe(listener: CursorListener): () => void;
}

export function createCursorStore(
  sampleCount: number,
  schedule: Schedule,
  report: ReportError = rethrowLater,
): CursorStore {
  if (!Number.isInteger(sampleCount) || sampleCount < 0) {
    throw new RangeError(`cursor: sampleCount must be a non-negative integer, got ${sampleCount}`);
  }

  let latest: CursorIndex = null;
  let delivered: CursorIndex = null;
  let pending = false;
  // Copy-on-write: subscribe/unsubscribe replace the array; a flush iterates
  // the array it started with.
  let listeners: readonly CursorListener[] = [];

  // Hot path (D-10): no allocation, no closures created here. Each listener is
  // isolated (D-14, ADR-008 rev. 1): a throw is reported, delivery continues with
  // the next listener, and the value still counts as delivered. Listeners must
  // not throw anyway (review rule).
  const flush = (): void => {
    pending = false;
    const value = latest;
    if (value === delivered) return;
    delivered = value;
    const list = listeners;
    for (let k = 0; k < list.length; k++) {
      try {
        list[k](value);
      } catch (error) {
        report(error);
      }
    }
  };

  return {
    sampleCount,
    get: () => latest,
    set(index: CursorIndex): void {
      if (index !== null && !(Number.isInteger(index) && index >= 0 && index < sampleCount)) {
        throw new RangeError(`cursor: index must be an integer in [0, ${sampleCount}) or null, got ${index}`);
      }
      if (index === latest) return;
      latest = index;
      if (!pending) {
        pending = true;
        schedule(flush);
      }
    },
    subscribe(listener: CursorListener): () => void {
      listeners = [...listeners, listener];
      // Not wrapped: a throw here reaches the subscriber's caller directly.
      listener(latest);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        const at = listeners.indexOf(listener);
        if (at >= 0) listeners = [...listeners.slice(0, at), ...listeners.slice(at + 1)];
      };
    },
  };
}
