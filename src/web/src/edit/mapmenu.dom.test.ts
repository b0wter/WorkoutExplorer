// @vitest-environment jsdom
// The map's right-click menu (F-65 AL-9, F-56 PN-12): desktop mouse only, near the route only.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createCursorStore } from '../lib/cursor';
import { createSelectionStore } from '../lib/selection';
import { startMapMenu } from './mapmenu';
import type { PlusItem } from './plus';

let host: HTMLElement;
let canvas: HTMLCanvasElement;
let off: boolean;
const run = vi.fn();
const cursor = createCursorStore(100, (f) => f());
const selection = createSelectionStore(100, (f) => f());

const items: PlusItem[] = [
  { label: () => 'Add lap', short: () => '+ Lap', state: () => ({ off, why: off ? 'Already a lap here' : '' }), run: () => run('lap') },
  { label: () => 'Add pin', short: () => '+ Pin', state: () => ({ off: false, why: '' }), run: () => run('pin') },
];
const down = (type: string): void => void canvas.dispatchEvent(Object.assign(new Event('pointerdown', { bubbles: true }), { pointerType: type }));
const right = (): boolean => canvas.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: 5, clientY: 5 }));
const menu = (): HTMLElement | null => host.querySelector('.map-menu');
const entries = (): HTMLElement[] => [...host.querySelectorAll<HTMLElement>('[role=menuitem]')];

beforeEach(() => {
  document.body.innerHTML = '<div id="map-slot"><div class="route-map"><canvas></canvas></div></div>';
  host = document.getElementById('map-slot')!;
  canvas = host.querySelector('canvas')!;
  cursor.set(42);
  off = false;
  run.mockClear();
  selection.clear();
  startMapMenu({ host, cursor, selection, items, where: () => 'at 4.2 km' });
});

describe('map right-click menu', () => {
  it('opens on a mouse right-click, acting on the existing cursor; first entry focused', () => {
    down('mouse');
    expect(right()).toBe(false); // default prevented
    expect(entries().map((e) => [e.firstChild!.textContent, e.querySelector('.map-menu-why')?.textContent])).toEqual([['Add lap', 'at 4.2 km'], ['Add pin', 'at 4.2 km']]); // where it will act
    expect(menu()!.parentElement).toBe(host.querySelector('.route-map')); // inside the box that goes fullscreen
    expect(menu()!.getAttribute('role')).toBe('menu');
    expect(entries().map((e) => e.getAttribute('aria-disabled'))).toEqual(['false', 'false']);
    expect(document.activeElement).toBe(entries()[0]);
  });

  it('leaves the browser menu for touch and off the canvas', () => {
    down('touch');
    expect(right()).toBe(true);
    down('mouse');
    host.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true }));
    expect(menu()).toBeNull();
  });

  it('without a cursor or range every entry is disabled and does nothing', () => {
    cursor.set(null);
    down('mouse');
    right();
    expect(entries().map((e) => e.getAttribute('aria-disabled'))).toEqual(['true', 'true']);
    expect(menu()!.querySelector('.map-menu-why')).toBeNull(); // no reason without a target
    entries()[1].click();
    expect(run).not.toHaveBeenCalled();
  });

  it('shows a refusal as a disabled entry with its reason, and does nothing on it', () => {
    off = true;
    down('mouse');
    right();
    const lap = entries()[0];
    expect(lap.getAttribute('aria-disabled')).toBe('true');
    expect(lap.querySelector('.map-menu-why')!.textContent).toBe('Already a lap here');
    lap.click();
    expect(run).not.toHaveBeenCalled();
    expect(menu()).not.toBeNull();
  });

  it('choosing an entry runs it and closes', () => {
    down('mouse');
    right();
    entries()[1].click();
    expect(run).toHaveBeenCalledWith('pin');
    expect(menu()).toBeNull();
  });

  it('arrow keys move, Esc closes with focus back on the map', () => {
    canvas.tabIndex = 0;
    down('mouse');
    right();
    menu()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    expect(document.activeElement).toBe(entries()[1]);
    menu()!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(menu()).toBeNull();
    expect(document.activeElement).toBe(canvas);
  });

  it('closes on a click outside', () => {
    down('mouse');
    right();
    down('mouse');
    expect(menu()).toBeNull();
  });
});
