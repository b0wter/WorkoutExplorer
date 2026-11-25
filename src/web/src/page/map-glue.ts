// The map's page-side glue (ADR-011 decision 7 T-2, F-75): the pin toggle (F-13 D-19), the fullscreen
// readout strip (F-33) and the splitter handle (F-13 D-17). Re-exported by components/map/map-view, so it
// travels in the map chunk: a workout without a route never downloads it, and start.ts reaches it through
// the already loaded map-view module when the map is ready. Its CSS is map-view.css's.

import { onLanguage } from '../lib/i18n';
import { t } from './texts';
import { MAX_SHARE, MIN_SHARE, layoutOf, type SplitterStorage } from './splitter';

export { mountStrip } from './strip.svelte';

/** Two taps within this time without a drag reset the ratio. */
const DOUBLE_MS = 350;
/** Movement below this many px is a tap, not a drag. */
const SLOP_PX = 4;

const PIN_SVG =
  '<svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><path fill="currentColor" d="M16 3H8v2l1 .5V11l-3 3v2h5v5l1 1 1-1v-5h5v-2l-3-3V5.5L16 5z"/></svg>';

/** The phone's pin toggle (F-13 design D-19, AP-18). Bundle-only: created when the map is ready,
 *  so it never exists without JavaScript, before the map or after a failed map. An icon button in
 *  MapLibre's top-right stack under fullscreen (own control group); without that stack (stubbed
 *  map) it is a plain child of #map-slot. It flips one class on the layout wrapper and
 *  aria-pressed; held in memory only. CSS shows it and applies the class below 1024 px only. */
export function addPinToggle(root: ParentNode, slot: HTMLElement): HTMLButtonElement | null {
  const layout = root.querySelector('.layout');
  if (!layout || slot.querySelector('.map-pin')) return null;
  const doc = slot.ownerDocument;
  const group = doc.createElement('div');
  group.className = 'maplibregl-ctrl maplibregl-ctrl-group map-pin-group';
  const button = doc.createElement('button');
  button.type = 'button';
  button.className = 'map-pin';
  const label = (): void => {
    button.title = t('pinMap');
    button.setAttribute('aria-label', t('pinMap'));
  };
  label();
  onLanguage(label);
  button.setAttribute('aria-pressed', 'false');
  button.innerHTML = PIN_SVG;
  button.addEventListener('click', () => {
    const on = layout.classList.toggle('pinned');
    button.setAttribute('aria-pressed', String(on));
  });
  group.appendChild(button);
  (slot.querySelector('.maplibregl-ctrl-top-right') ?? slot).appendChild(group);
  return button;
}

export function addSplitter(root: ParentNode, slot: HTMLElement, storage?: SplitterStorage): HTMLElement | null {
  const layout = root.querySelector<HTMLElement>('.layout');
  if (!layout || slot.querySelector('.splitter')) return null;
  const el = slot.ownerDocument.createElement('div');
  el.className = 'splitter';
  let id = -1;
  let x0 = 0;
  let share0 = 0;
  let width = 1;
  let moved = false;
  let lastTap = -Infinity;
  let frame = 0;
  let next = 0;

  el.addEventListener('pointerdown', (e) => {
    if (id !== -1 || e.button !== 0) return;
    e.preventDefault(); // no text selection, no emulated mouse events
    id = e.pointerId;
    el.setPointerCapture(id);
    width = layout.getBoundingClientRect().width || 1;
    // From the box as laid out, so a share clamped by the columns' minimum widths starts no dead zone.
    share0 = slot.getBoundingClientRect().width / width;
    x0 = e.clientX;
    moved = false;
  });
  el.addEventListener('pointermove', (e) => {
    if (e.pointerId !== id) return;
    const dx = e.clientX - x0;
    if (!moved && Math.abs(dx) < SLOP_PX) return;
    moved = true;
    next = Math.min(MAX_SHARE, Math.max(MIN_SHARE, share0 + dx / width)) * 100;
    if (frame === 0) {
      frame = requestAnimationFrame(() => {
        frame = 0;
        layout.style.setProperty('--map-w', next.toFixed(2) + '%');
      });
    }
  });
  const end = (e: PointerEvent): void => {
    if (e.pointerId !== id) return;
    id = -1;
    if (moved || e.type === 'pointercancel') {
      lastTap = -Infinity;
      if (moved) storage?.prefs.setSplit(layoutOf(storage), next);
    } else if (e.timeStamp - lastTap < DOUBLE_MS) {
      lastTap = -Infinity;
      layout.style.removeProperty('--map-w');
      storage?.prefs.clearSplit(layoutOf(storage));
    } else {
      lastTap = e.timeStamp;
    }
  };
  el.addEventListener('pointerup', end);
  el.addEventListener('pointercancel', end);
  slot.appendChild(el);
  return el;
}
