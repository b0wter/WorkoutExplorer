// Public surface of lib/cursor (F-05 design, "Cursor store"; ADR-008).
// Imports nothing. App code imports this folder only through this file.

export { createCursorStore } from './store';
export type { CursorIndex, CursorListener, CursorStore, ReportError, Schedule } from './store';
export { nearestIndex } from './nearest';
