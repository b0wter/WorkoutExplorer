// Map gesture reducer: pointer input on the map → cursor action (ADR-015; F-06 design,
// "Route interaction"). Pure: no DOM, no store, no MapLibre.
//
// Mouse and pen: hovering within HOVER_RADIUS_PX of the route sets the cursor; dragging the
// map, leaving it or moving off the route leaves the cursor where it is (F-05 D-8).
// Touch: a primary touch that goes down within TOUCH_START_RADIUS_PX of the route scrubs
// for the rest of that touch (moves within TOUCH_DRAG_RADIUS_PX set the cursor); lifting
// keeps the cursor, pointercancel restores the value from before the touch, and a second
// finger hands the gesture to MapLibre. Touches that start elsewhere belong to the page and
// MapLibre (cooperative gestures).

/** CSS px. Assumed values, checked by the operator (F-06 MP2–MP5); defined only here. */
export const HOVER_RADIUS_PX = 12; // mouse, pen
export const TOUCH_START_RADIUS_PX = 20; // a touch must start this close to the route to scrub
export const TOUCH_DRAG_RADIUS_PX = 48; // during a scrub
/** Samples within this distance of the nearest one are tied; the earliest wins (decision 5). */
export const TIE_PX = 3;

/** A sample index or no cursor; same shape as lib/cursor's CursorIndex (lib/route may not import lib/cursor). */
export type RouteCursor = number | null;

export type RouteGestureKind = 'down' | 'move' | 'up' | 'cancel' | 'leave';

export interface RouteGestureInput {
  readonly kind: RouteGestureKind;
  /** PointerEvent.pointerType: 'mouse', 'pen', 'touch' (anything other than 'touch' behaves as mouse). */
  readonly pointerType: string;
  readonly pointerId: number;
  readonly isPrimary: boolean;
  /** PointerEvent.buttons. */
  readonly buttons: number;
  /** Sample index of the route hit within hitRadiusPx, or −1. */
  readonly hit: number;
  /** cursor.get() at the time of the event. */
  readonly current: RouteCursor;
}

export type RouteGestureState =
  | { readonly kind: 'idle' }
  | { readonly kind: 'scrub'; readonly pointerId: number; readonly before: RouteCursor };

export type RouteGestureAction = { readonly kind: 'none' } | { readonly kind: 'set'; readonly index: RouteCursor };

export interface RouteGestureStep {
  readonly state: RouteGestureState;
  readonly action: RouteGestureAction;
}

export const ROUTE_IDLE: RouteGestureState = { kind: 'idle' };
const NONE: RouteGestureAction = { kind: 'none' };

/** Hit radius in CSS px for the next input of this pointer type. A touch `down` always starts
 *  a new gesture (also over a stale scrub), so it gets the start radius; `kind` defaults to
 *  'move'. */
export function hitRadiusPx(state: RouteGestureState, pointerType: string, kind: RouteGestureKind = 'move'): number {
  if (pointerType !== 'touch') return HOVER_RADIUS_PX;
  return state.kind === 'scrub' && kind !== 'down' ? TOUCH_DRAG_RADIUS_PX : TOUCH_START_RADIUS_PX;
}

/** True exactly for the inputs whose reducer row reads `hit` (F-06 design v1.3, QA-06-r1-4):
 *  a mouse or pen move with no button pressed, a primary touch down, and a move of the
 *  scrubbing touch. For everything else the adapter passes hit = −1 without a hit test, so
 *  page scrolls over the map and mouse drags of the map cost no unproject and no grid query. */
export function wantsHit(
  state: RouteGestureState,
  kind: RouteGestureKind,
  pointerType: string,
  pointerId: number,
  isPrimary: boolean,
  buttons: number,
): boolean {
  if (pointerType !== 'touch') return kind === 'move' && buttons === 0;
  if (kind === 'down') return isPrimary;
  return kind === 'move' && state.kind === 'scrub' && pointerId === state.pointerId;
}

/** True only while one finger scrubs: the outer map element's touchmove listener then stops
 *  the page from scrolling and hides the move from MapLibre (ADR-015 decision 3). */
export function interceptsTouch(state: RouteGestureState, touchCount: number): boolean {
  return state.kind === 'scrub' && touchCount === 1;
}

export function routeStep(state: RouteGestureState, input: RouteGestureInput): RouteGestureStep {
  if (input.pointerType !== 'touch') {
    // Mouse and pen never scrub: only a hover (no button pressed) on the route sets the cursor.
    if (input.kind === 'move' && input.buttons === 0 && input.hit >= 0) {
      return { state: ROUTE_IDLE, action: { kind: 'set', index: input.hit } };
    }
    return { state: ROUTE_IDLE, action: NONE };
  }

  if (input.kind === 'down') {
    if (input.isPrimary) {
      // A new primary touch; replaces a stale scrub whose up or cancel was missed.
      if (input.hit < 0) return { state: ROUTE_IDLE, action: NONE };
      return {
        state: { kind: 'scrub', pointerId: input.pointerId, before: input.current },
        action: { kind: 'set', index: input.hit },
      };
    }
    // A second finger: the gesture goes to MapLibre (two-finger pan or pinch).
    return { state: ROUTE_IDLE, action: NONE };
  }

  if (state.kind !== 'scrub' || input.pointerId !== state.pointerId) return { state, action: NONE };

  switch (input.kind) {
    case 'move':
      return { state, action: input.hit >= 0 ? { kind: 'set', index: input.hit } : NONE };
    case 'up':
      return { state: ROUTE_IDLE, action: NONE }; // the cursor stays where the finger was
    case 'cancel':
      return { state: ROUTE_IDLE, action: { kind: 'set', index: state.before } };
    default:
      return { state, action: NONE }; // leave
  }
}
