// @vitest-environment jsdom
// The map/content splitter (user decision 28 Oct 2026): drag writes a clamped --map-w on the
// layout wrapper, a tap does nothing, a double tap removes it, and it is added only once.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { addSplitter, restoreSplit, LANDSCAPE_QUERY, type SplitterStorage } from './splitter';
import { createViewerPrefs } from '../lib/prefs';
import { readWebFile } from '../../test-support/source.mjs';

function setup(storage?: SplitterStorage): { layout: HTMLElement; slot: HTMLElement; el: HTMLElement } {
  document.body.innerHTML = '<div class="page has-map"><div class="layout"><div id="map-slot" class="map-slot"></div></div></div>';
  const layout = document.querySelector<HTMLElement>('.layout')!;
  const slot = document.querySelector<HTMLElement>('#map-slot')!;
  layout.getBoundingClientRect = () => ({ width: 1000 }) as DOMRect;
  slot.getBoundingClientRect = () => ({ width: 500 }) as DOMRect;
  if (storage) restoreSplit(document, storage); // as the page glue does, before the load
  const el = addSplitter(document, slot, storage)!;
  return { layout, slot, el };
}

function fire(el: HTMLElement, type: string, clientX: number, o: PointerEventInit = {}): void {
  el.dispatchEvent(new PointerEvent(type, { bubbles: true, cancelable: true, clientX, pointerId: 1, isPrimary: true, button: 0, pointerType: 'touch', ...o }));
}

function drag(el: HTMLElement, from: number, to: number): void {
  fire(el, 'pointerdown', from);
  fire(el, 'pointermove', to);
  vi.runAllTimers();
  fire(el, 'pointerup', to);
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['requestAnimationFrame'] });
  HTMLElement.prototype.setPointerCapture = () => {};
});
afterEach(() => {
  vi.useRealTimers();
  document.body.innerHTML = '';
  localStorage.clear();
});

describe('addSplitter', () => {
  it('is added once, inside the map slot', () => {
    const { slot, el } = setup();
    expect(el.parentElement).toBe(slot);
    expect(addSplitter(document, slot)).toBeNull();
    expect(slot.querySelectorAll('.splitter')).toHaveLength(1);
  });

  it('sets the map share from the laid-out width plus the drag', () => {
    const { layout, el } = setup();
    drag(el, 500, 600);
    expect(layout.style.getPropertyValue('--map-w')).toBe('60.00%');
  });

  it('clamps the share to 30 to 70 %', () => {
    const { layout, el } = setup();
    drag(el, 500, 950);
    expect(layout.style.getPropertyValue('--map-w')).toBe('70.00%');
    drag(el, 500, 0);
    expect(layout.style.getPropertyValue('--map-w')).toBe('30.00%');
  });

  it('ignores a tap and movement below the slop', () => {
    const { layout, el } = setup();
    drag(el, 500, 502);
    expect(layout.style.getPropertyValue('--map-w')).toBe('');
  });

  it('resets on a double tap or double click, not after a drag', () => {
    const { layout, el } = setup();
    drag(el, 500, 600);
    fire(el, 'pointerdown', 600, { pointerType: 'mouse' });
    fire(el, 'pointerup', 600, { pointerType: 'mouse' });
    fire(el, 'pointerdown', 600, { pointerType: 'mouse' });
    fire(el, 'pointerup', 600, { pointerType: 'mouse' });
    expect(layout.style.getPropertyValue('--map-w')).toBe('');
    drag(el, 500, 600);
    fire(el, 'pointerdown', 600);
    fire(el, 'pointerup', 600);
    expect(layout.style.getPropertyValue('--map-w')).toBe('60.00%');
  });

  it('ignores other pointers and buttons', () => {
    const { layout, el } = setup();
    fire(el, 'pointerdown', 500, { button: 2, pointerType: 'mouse' });
    fire(el, 'pointermove', 700, { pointerType: 'mouse' });
    vi.runAllTimers();
    expect(layout.style.getPropertyValue('--map-w')).toBe('');
    fire(el, 'pointerdown', 500);
    fire(el, 'pointermove', 700, { pointerId: 2 });
    vi.runAllTimers();
    expect(layout.style.getPropertyValue('--map-w')).toBe('');
  });
});

describe('remembered split (AP-17, decision log 29 Oct 2026)', () => {
  // A stand-in for matchMedia(LANDSCAPE_QUERY) whose state the test flips.
  function media(): MediaQueryList & { flip(on: boolean): void } {
    let on = false;
    const listeners: (() => void)[] = [];
    return {
      get matches() {
        return on;
      },
      addEventListener: (_: string, f: () => void) => listeners.push(f),
      flip(v: boolean) {
        on = v;
        for (const f of listeners) f();
      },
    } as unknown as MediaQueryList & { flip(on: boolean): void };
  }
  const prefs = () => createViewerPrefs(() => localStorage);

  it('a drag stores the share for the current layout only; a reset clears it', () => {
    const landscape = media();
    const { layout, el } = setup({ prefs: prefs(), landscape });
    expect(localStorage.length).toBe(0);
    drag(el, 500, 600);
    expect(localStorage.getItem('wx.split.wide')).toBe('60.00');
    landscape.flip(true);
    expect(layout.style.getPropertyValue('--map-w')).toBe('');
    drag(el, 500, 450);
    expect(localStorage.getItem('wx.split.landscape')).toBe('45.00');
    landscape.flip(false);
    expect(layout.style.getPropertyValue('--map-w')).toBe('60.00%');
    fire(el, 'pointerdown', 600);
    fire(el, 'pointerup', 600);
    fire(el, 'pointerdown', 600);
    fire(el, 'pointerup', 600);
    expect(layout.style.getPropertyValue('--map-w')).toBe('');
    expect(localStorage.getItem('wx.split.wide')).toBeNull();
    expect(localStorage.getItem('wx.split.landscape')).toBe('45.00');
  });

  it('restores the stored share on start; out of range or unknown is ignored; a tap writes nothing', () => {
    localStorage.setItem('wx.split.wide', '35.5');
    expect(setup({ prefs: prefs(), landscape: null }).layout.style.getPropertyValue('--map-w')).toBe('35.50%');
    for (const v of ['80', '29.99', 'wide']) {
      localStorage.setItem('wx.split.wide', v);
      const { layout, el } = setup({ prefs: prefs(), landscape: null });
      expect(layout.style.getPropertyValue('--map-w'), v).toBe('');
      fire(el, 'pointerdown', 500);
      fire(el, 'pointerup', 500);
      expect(localStorage.getItem('wx.split.wide')).toBe(v);
    }
  });

  it('LANDSCAPE_QUERY is the query of page.css', () => {
    expect(readWebFile('src/page/page.css')).toContain('@media ' + LANDSCAPE_QUERY + ' {');
  });
});
