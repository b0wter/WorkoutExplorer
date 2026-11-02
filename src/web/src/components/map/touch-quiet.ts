// Touch-quiet gate (F-06 design v1.3, "Loading and start-up"; ADR-013 rev. 1 decision 5). STATIC.
//
// MapLibre's start-up holds the main thread for a while. A touch that starts on a chart
// during that time waits for it (the chart's non-passive touchmove, F-05 D-18), so the first
// scroll feels stuck. whenQuiet() lets the map start only in a moment without fingers on the
// screen and none lifted in the last QUIET_MS, then right after a frame. It cannot stop a
// touch that starts during a long task; the start-up measurement bounds that.
//
// Fingers, not pointers (QA-06-r1-2): browsers send pointercancel as soon as a scroll starts,
// while the finger is still down and scrolling, which is exactly the touch to wait for. The
// count is `touches.length` of the latest touchstart, touchend or touchcancel, so it corrects
// itself on every touch event and a missed end can never block the map for good. Mouse and
// pen fire no touch events and never delay anything. No time reads: the QUIET_MS cool-down
// is a timer, restarted whenever the count reaches 0.

/** Quiet time after the last finger lifted, in ms. */
export const QUIET_MS = 250;

export interface TouchQuiet {
  /** Resolves in a touch-quiet moment, right after a frame. Stays pending after dispose(). */
  whenQuiet(): Promise<void>;
  /** Removes every listener and timer. */
  dispose(): void;
}

export function createTouchQuiet(win: Window): TouchQuiet {
  /** Fingers on the screen, from the latest touch event. */
  let fingers = 0;
  /** Cool-down timer after the count reached 0; 0 = none. */
  let cooling = 0;
  /** A frame + timeout is scheduled for the waiters. */
  let scheduled = false;
  let disposed = false;
  let waiters: (() => void)[] = [];

  const quiet = (): boolean => fingers === 0 && cooling === 0;

  const release = (): void => {
    if (disposed || scheduled || waiters.length === 0 || !quiet()) return;
    scheduled = true;
    win.requestAnimationFrame(() => {
      win.setTimeout(() => {
        scheduled = false;
        if (disposed) return;
        if (!quiet()) return; // a finger came down meanwhile: wait for the next quiet moment
        const ready = waiters;
        waiters = [];
        for (const resolve of ready) resolve();
      }, 0);
    });
  };

  const onTouch = (e: TouchEvent): void => {
    fingers = e.touches.length;
    if (cooling !== 0) win.clearTimeout(cooling);
    cooling = 0;
    if (fingers > 0) return;
    // The last finger lifted (touchend or touchcancel): the cool-down starts now.
    cooling = win.setTimeout(() => {
      cooling = 0;
      release();
    }, QUIET_MS);
  };
  const onVisibility = (): void => {
    if (win.document.visibilityState !== 'hidden') return;
    fingers = 0;
    release();
  };

  const options: AddEventListenerOptions = { passive: true, capture: true };
  win.addEventListener('touchstart', onTouch, options);
  win.addEventListener('touchend', onTouch, options);
  win.addEventListener('touchcancel', onTouch, options);
  win.document.addEventListener('visibilitychange', onVisibility, options);

  return {
    whenQuiet(): Promise<void> {
      return new Promise((resolve) => {
        waiters.push(resolve);
        release();
      });
    },
    dispose(): void {
      if (disposed) return;
      disposed = true;
      win.removeEventListener('touchstart', onTouch, options);
      win.removeEventListener('touchend', onTouch, options);
      win.removeEventListener('touchcancel', onTouch, options);
      win.document.removeEventListener('visibilitychange', onVisibility, options);
      if (cooling !== 0) win.clearTimeout(cooling);
      cooling = 0;
      waiters = [];
    },
  };
}
