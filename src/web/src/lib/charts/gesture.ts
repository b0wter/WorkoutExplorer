// Gesture reducer: pointer input on a chart → cursor action (ADR-010 rev. 2; F-05
// design v1.4, "Interaction"; review decision 6 and the user's touch decisions). Pure:
// no DOM, no store.
//
// Touch: down places the cursor under the finger at once (P3 option B); moves and up
// set it; pointercancel (the browser took a scroll or a pinch) restores the cursor from
// before the touch. Each touch is classified once, 8 CSS px from its start: within 30°
// of horizontal is a scrub, anything else a pan (D-18). During a scrub, blocksScroll()
// lets MetricChart's touchmove listener stop the page from scrolling. Mouse and pen:
// down and move set the cursor; leaving keeps it (D-8, OQ-13). Non-primary pointers are
// ignored.
//
// F-17 (design R-5, gestures table): range selection. Mouse and pen: press, move past
// MOUSE_SLOP, release selects from the press sample to the pointer sample. Touch: hold within
// TOUCH_SLOP until the long-press input (LONG_PRESS_MS, timed by MetricChart), then drag. A click
// or a tap (up with the axis still open) outside the range clears it; nothing else clears. A
// cancel while selecting restores range and cursor. A drag that ends with a = b changes nothing.

/** CSS px from the touch-down point at which a touch is classified (D-18). */
export const TOUCH_SLOP = 8;
/** CSS px a pressed mouse or pen must move before the press becomes a range drag (F-17 R-5). */
export const MOUSE_SLOP = 4;
/** ms a touch must stay within TOUCH_SLOP before it selects (F-17 R-5; assumed, phone-checked). */
export const LONG_PRESS_MS = 400;

/** A sample index or no cursor; same shape as lib/cursor's CursorIndex (lib/charts may not import lib/cursor). */
export type GestureCursor = number | null;

/** 'longpress': the long-press timer of the touch `pointerId` fired. */
export type GestureInputKind = 'down' | 'move' | 'up' | 'cancel' | 'leave' | 'longpress';

export interface GestureInput {
  readonly kind: GestureInputKind;
  /** PointerEvent.pointerType: 'mouse', 'pen', 'touch' (anything other than 'touch' behaves as mouse). */
  readonly pointerType: string;
  readonly pointerId: number;
  readonly isPrimary: boolean;
  /** Sample index under the pointer, or −1 when unknown (no action is taken for −1). */
  readonly index: number;
  /** cursor.get() at the time of the event. */
  readonly current: GestureCursor;
  /** clientX, clientY in CSS px. */
  readonly x: number;
  readonly y: number;
  /** The selection at the time of the event (inclusive); −1 or absent for none. */
  readonly rangeA?: number;
  readonly rangeB?: number;
}

/** 'open' until the slop is reached; then fixed for the rest of the touch. */
export type TouchAxis = 'open' | 'scrub' | 'pan';

export type GestureState =
  | { readonly kind: 'idle' }
  | {
      readonly kind: 'touch';
      readonly pointerId: number;
      readonly before: GestureCursor;
      readonly x0: number;
      readonly y0: number;
      readonly axis: TouchAxis;
      /** Sample at touch-down: the range anchor if a long press follows (−1 unknown). */
      readonly anchor: number;
    }
  | {
      /** Mouse or pen button down, still within MOUSE_SLOP. */
      readonly kind: 'press';
      readonly pointerId: number;
      readonly before: GestureCursor;
      readonly x0: number;
      readonly y0: number;
      readonly anchor: number;
    }
  | {
      /** A range drag (mouse after the slop, touch after the long press). */
      readonly kind: 'select';
      readonly pointerId: number;
      /** A touch drag (blocks scrolling, suppresses the callout); false for mouse and pen. */
      readonly touch: boolean;
      readonly anchor: number;
      /** Cursor and range from before the drag, for cancel and a = b. */
      readonly before: GestureCursor;
      readonly beforeA: number;
      readonly beforeB: number;
    };

type SelectState = Extract<GestureState, { kind: 'select' }>;

/** A selection write: (a, b) inclusive, a = −1 clears. */
export interface RangeAction {
  readonly a: number;
  readonly b: number;
}

export type GestureAction = { readonly kind: 'none' } | { readonly kind: 'set'; readonly index: GestureCursor };

export interface GestureStep {
  readonly state: GestureState;
  readonly action: GestureAction;
  /** Present only when the selection changes. */
  readonly range?: RangeAction;
}

export const IDLE: GestureState = { kind: 'idle' };
const NONE: GestureAction = { kind: 'none' };
const CLEAR: RangeAction = { a: -1, b: -1 };

function setTo(index: number): GestureAction {
  return index >= 0 ? { kind: 'set', index } : NONE;
}

/** A click or tap at input.index: clears a range it is outside of. */
function tap(input: GestureInput): GestureStep {
  const a = input.rangeA ?? -1;
  const i = input.index;
  const outside = a >= 0 && i >= 0 && (i < a || i > (input.rangeB ?? -1));
  return outside ? { state: IDLE, action: setTo(i), range: CLEAR } : { state: IDLE, action: setTo(i) };
}

function startSelect(pointerId: number, anchor: number, before: GestureCursor, input: GestureInput): SelectState {
  const touch = input.pointerType === 'touch';
  return { kind: 'select', pointerId, touch, anchor, before, beforeA: input.rangeA ?? -1, beforeB: input.rangeB ?? -1 };
}

/** The range from the anchor to input.index, live; a = b keeps the range from before the drag. */
function drag(state: SelectState, input: GestureInput, next: GestureState): GestureStep {
  const i = input.index;
  if (i < 0) return { state: next, action: NONE };
  const a = Math.min(state.anchor, i);
  const b = Math.max(state.anchor, i);
  return { state: next, action: setTo(i), range: a === b ? { a: state.beforeA, b: state.beforeB } : { a, b } };
}

function restore(state: SelectState): GestureStep {
  return { state: IDLE, action: { kind: 'set', index: state.before }, range: { a: state.beforeA, b: state.beforeB } };
}

/** A selecting pointer: move drags, up ends the drag, cancel restores, anything else is ignored. */
function selecting(state: SelectState, input: GestureInput): GestureStep {
  switch (input.kind) {
    case 'move':
      return drag(state, input, state);
    case 'up':
      return drag(state, input, IDLE);
    case 'cancel':
      return restore(state);
    default:
      return { state, action: NONE };
  }
}

function mouse(state: GestureState, input: GestureInput): GestureStep {
  if (input.kind === 'down') {
    if (input.index < 0) return { state: IDLE, action: NONE };
    return {
      state: { kind: 'press', pointerId: input.pointerId, before: input.current, x0: input.x, y0: input.y, anchor: input.index },
      action: setTo(input.index),
    };
  }
  const mine = (state.kind === 'press' || state.kind === 'select') && state.pointerId === input.pointerId;
  if (!mine) return { state: IDLE, action: input.kind === 'move' ? setTo(input.index) : NONE }; // hover; leave keeps (D-8)
  if (state.kind === 'select') return selecting(state, input);
  if (state.kind !== 'press') return { state, action: NONE };
  switch (input.kind) {
    case 'move': {
      const dx = input.x - state.x0;
      const dy = input.y - state.y0;
      if (dx * dx + dy * dy < MOUSE_SLOP * MOUSE_SLOP) return { state, action: setTo(input.index) };
      const sel = startSelect(state.pointerId, state.anchor, state.before, input);
      return drag(sel, input, sel);
    }
    case 'up':
      return tap(input);
    case 'cancel':
      return { state: IDLE, action: NONE };
    default:
      return { state, action: NONE };
  }
}

export function step(state: GestureState, input: GestureInput): GestureStep {
  if (!input.isPrimary) return { state, action: NONE };
  if (input.pointerType !== 'touch') return mouse(state, input);

  // Touch.
  if (input.kind === 'down') {
    // Also a primary down with a new ID while a touch is tracked: the old touch ended
    // without its up or cancel reaching us, and the new touch replaces it (v1.1), so a
    // chart never gets stuck ignoring touches or blocking scrolls. Non-primary returned above.
    return {
      state: { kind: 'touch', pointerId: input.pointerId, before: input.current, x0: input.x, y0: input.y, axis: 'open', anchor: input.index },
      action: setTo(input.index),
    };
  }
  if (input.kind === 'leave') return { state, action: NONE }; // fires after up or cancel
  if (state.kind === 'select' && state.pointerId === input.pointerId) return selecting(state, input);
  if (state.kind !== 'touch' || state.pointerId !== input.pointerId) return { state, action: NONE };

  switch (input.kind) {
    case 'longpress':
      if (state.axis !== 'open' || state.anchor < 0) return { state, action: NONE };
      return { state: startSelect(state.pointerId, state.anchor, state.before, input), action: NONE };
    case 'move': {
      if (state.axis !== 'open') return { state, action: setTo(input.index) };
      const dx = input.x - state.x0;
      const dy = input.y - state.y0;
      if (dx * dx + dy * dy < TOUCH_SLOP * TOUCH_SLOP) return { state, action: setTo(input.index) };
      // Within 30° of horizontal (tan² 30° = 1/3), boundary inclusive.
      const axis: TouchAxis = 3 * dy * dy <= dx * dx ? 'scrub' : 'pan';
      return { state: { ...state, axis }, action: setTo(input.index) };
    }
    case 'up':
      // A tap places the cursor (and may clear the range); the end of a scrub or pan only places it.
      return state.axis === 'open' ? tap(input) : { state: IDLE, action: setTo(input.index) };
    case 'cancel':
      return { state: IDLE, action: { kind: 'set', index: state.before } };
  }
}

/** True only while one finger is scrubbing or selecting (F-17 R-5), with touchCount 1.
 *  Called by the non-passive touchmove listener: no allocation. */
export function blocksScroll(state: GestureState, touchCount: number): boolean {
  return touchCount === 1 && ((state.kind === 'touch' && state.axis === 'scrub') || (state.kind === 'select' && state.touch));
}

/** True while a touch is held or selecting: the touch callout and context menu are suppressed (F-17 R-5). */
export function holdsTouch(state: GestureState): boolean {
  return state.kind === 'touch' || (state.kind === 'select' && state.touch);
}
