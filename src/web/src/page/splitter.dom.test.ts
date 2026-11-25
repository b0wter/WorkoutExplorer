// @vitest-environment jsdom
// The map/content splitter (user decision 28 Oct 2026): drag writes a clamped --map-w on the
// layout wrapper, a tap does nothing, a double tap removes it, and it is added only once.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { restoreSplit, LANDSCAPE_QUERY, type SplitterStorage } from './splitter';
import { addSplitter } from './map-glue';
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

// F-73 RS-2, RS-3: the head script's first-paint split and the bundle's handover.
describe('head script split', () => {
  const script = /<script>([^<]*)<\/script>/.exec(readWebFile('index.html'))![1];
  const headValue = (stored: string | null, landscape: boolean): string | null => {
    document.documentElement.style.removeProperty('--map-w');
    const store = { getItem: (k: string) => (k === (landscape ? 'wx.split.landscape' : 'wx.split.wide') ? stored : null) };
    new Function('localStorage', 'document', 'navigator', 'window', 'fetch', 'setTimeout', 'matchMedia', script)(store, document, { languages: ['en'] }, {}, () => {}, () => {}, () => ({ matches: landscape }));
    return document.documentElement.style.getPropertyValue('--map-w') || null;
  };
  const bundleValue = (stored: string | null, landscape: boolean): string | null => {
    document.documentElement.style.removeProperty('--map-w');
    const prefs = createViewerPrefs(() => ({ getItem: () => stored, setItem: () => {}, removeItem: () => {} }));
    const { layout } = setup({ prefs, landscape: { matches: landscape, addEventListener: () => {} } as unknown as MediaQueryList });
    return layout.style.getPropertyValue('--map-w') || null;
  };

  it('sets the same number as the bundle for the same stored texts, in both layouts', () => {
    for (const landscape of [false, true])
      for (const v of ['20', '29.99', '30', '60', '70', '70.01', '80', 'abc', '50%', '1e2', '', '45.5', null])
        expect(parseFloat(headValue(v, landscape) ?? 'NaN'), `${v} ${landscape}`).toEqual(parseFloat(bundleValue(v, landscape) ?? 'NaN'));
  });

  it('hands over to .layout: the value leaves the root, a reset leaves no value anywhere', () => {
    document.documentElement.style.setProperty('--map-w', '60%');
    const store: Record<string, string> = { 'wx.split.wide': '60' };
    const prefs = createViewerPrefs(() => ({ getItem: (k) => store[k] ?? null, setItem: (k, v) => void (store[k] = v), removeItem: (k) => void delete store[k] }));
    const { layout, el } = setup({ prefs, landscape: null });
    expect(layout.style.getPropertyValue('--map-w')).toBe('60.00%');
    expect(document.documentElement.style.getPropertyValue('--map-w')).toBe('');
    fire(el, 'pointerdown', 10, { pointerType: 'mouse' });
    fire(el, 'pointerup', 10, { pointerType: 'mouse' });
    fire(el, 'pointerdown', 10, { pointerType: 'mouse' });
    fire(el, 'pointerup', 10, { pointerType: 'mouse' });
    expect(layout.style.getPropertyValue('--map-w')).toBe('');
    expect(document.documentElement.style.getPropertyValue('--map-w')).toBe('');
  });

  it('rotating into a layout with nothing stored leaves no --map-w anywhere (QA-73-r1-2)', () => {
    document.documentElement.style.setProperty('--map-w', '60%');
    const store: Record<string, string> = { 'wx.split.wide': '60' };
    const prefs = createViewerPrefs(() => ({ getItem: (k) => store[k] ?? null, setItem: () => {}, removeItem: () => {} }));
    let change = (): void => {};
    const landscape = { matches: false, addEventListener: (_: string, f: () => void) => (change = f) } as unknown as MediaQueryList;
    const { layout } = setup({ prefs, landscape });
    expect(layout.style.getPropertyValue('--map-w')).toBe('60.00%');
    (landscape as { matches: boolean }).matches = true;
    change();
    expect(layout.style.getPropertyValue('--map-w')).toBe('');
    expect(document.documentElement.style.getPropertyValue('--map-w')).toBe('');
  });
});
