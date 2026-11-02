// The selection store (F-17 design R-1): a range of two sample indices a < b, both inclusive, or
// none. Same rules as the cursor store (ADR-008 items 2 to 5): framework-free, an invalid value
// throws RangeError, delivery coalesced to one flush per scheduled frame, a synchronous first call
// on subscribe, listener errors caught per listener. Neither a write nor a flush allocates: the
// range is held and delivered as two numbers, a = b = −1 for none.

/** Receives the range, or (−1, −1) for none. Must not write the store. */
export type SelectionListener = (a: number, b: number) => void;

export interface SelectionStore {
  readonly sampleCount: number;
  /** Latest range start (inclusive), −1 for none. */
  start(): number;
  /** Latest range end (inclusive), −1 for none. */
  end(): number;
  /** Integers 0 ≤ a < b < sampleCount, else RangeError. Never calls listeners synchronously. */
  set(a: number, b: number): void;
  clear(): void;
  /** Calls listener once, synchronously, with the current range; then once per flush in which
   *  the delivered range changed. Returns the unsubscribe function. */
  subscribe(listener: SelectionListener): () => void;
}

const rethrowLater = (error: unknown): void => {
  queueMicrotask(() => {
    throw error;
  });
};

export function createSelectionStore(
  sampleCount: number,
  schedule: (flush: () => void) => void,
  report: (error: unknown) => void = rethrowLater,
): SelectionStore {
  let a = -1;
  let b = -1;
  let da = -1;
  let db = -1;
  let pending = false;
  let listeners: readonly SelectionListener[] = [];

  const flush = (): void => {
    pending = false;
    if (a === da && b === db) return;
    da = a;
    db = b;
    const list = listeners;
    for (let k = 0; k < list.length; k++) {
      try {
        list[k](da, db);
      } catch (error) {
        report(error);
      }
    }
  };
  const write = (na: number, nb: number): void => {
    if (na === a && nb === b) return;
    a = na;
    b = nb;
    if (!pending) {
      pending = true;
      schedule(flush);
    }
  };

  return {
    sampleCount,
    start: () => a,
    end: () => b,
    set(na, nb) {
      if (!(Number.isInteger(na) && Number.isInteger(nb) && na >= 0 && na < nb && nb < sampleCount)) {
        throw new RangeError(`selection: need integers 0 ≤ a < b < ${sampleCount}, got ${na}, ${nb}`);
      }
      write(na, nb);
    },
    clear: () => write(-1, -1),
    subscribe(listener) {
      listeners = [...listeners, listener];
      listener(a, b);
      let active = true;
      return () => {
        if (!active) return;
        active = false;
        listeners = listeners.filter((l) => l !== listener);
      };
    },
  };
}
