// @vitest-environment jsdom
// The How to use dialog (F-39 HU-7; AC 3, AC 4). jsdom has no showModal: a minimal stand-in.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
// @ts-expect-error no @types/node here; vitest runs in node
import { readFileSync } from 'node:fs';
import { openHowto } from './index';

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function (this: HTMLDialogElement) {
    this.open = true;
  };
  HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
    this.open = false;
  };
});
afterEach(() => {
  document.body.innerHTML = '';
});

const dialog = (): HTMLDialogElement => document.querySelector('dialog.howto-dlg')!;
const texts = (cls: string): string[] => [...dialog().querySelectorAll('.dlg-body > p:not(.dlg-actions)')].map((p) => p.querySelector('b')!.textContent + p.querySelector('.' + cls)!.textContent);

describe('openHowto', () => {
  it('shows the title and one line per section in both wordings', () => {
    openHowto(document);
    expect(dialog().open).toBe(true);
    expect(dialog().querySelector('h2')?.textContent).toBe('How to use this page');
    expect(texts('coarse')).toEqual([
      'Values at a point. Touch a chart and slide sideways to see the values at that point; on workouts with GPS, you can also drag along the route on the map.',
      'Select a range. Touch and hold a chart, then drag, to select a range and see its totals; Clear removes it.',
      'Laps and splits. Tap a lap or split to select it as a range.',
      'Fullscreen map. On workouts with GPS, tap the fullscreen button on the map to enlarge it; tap it again or go back to leave.',
      'Units, theme and language. Change units, theme and language at the top of the page; this device remembers your choice.',
      'Zones. Zones use the max heart rate and threshold power from the FIT file when it has them; the sharer can set their own on the edit page.',
      'Share a range. Select a range, then tap "Copy link to this selection" to share it.',
      'Pins. On workouts with pins, tap a pin on the map or in the list to show it on the route and charts; the buttons on the map show or hide each type.',
      'Add a lap or pin. For sharers: on the edit page, select a point or range, then tap "+ Lap" or "+ Pin" (or "+" on the map).',
      'Summary numbers. The "How these numbers are computed" bar, under the "How to use this page" bar, explains the summary\'s numbers.',
    ]);
    expect(texts('fine')).toEqual([
      'Values at a point. Point at a chart to see the values at that point; on workouts with GPS, you can also point along the route on the map.',
      'Select a range. Drag across a chart to select a range and see its totals; Clear removes it.',
      'Laps and splits. Click a lap or split to select it as a range.',
      'Fullscreen map. On workouts with GPS, click the fullscreen button on the map to enlarge it; click it again or press Esc to leave.',
      'Units, theme and language. Change units, theme and language at the top of the page; this device remembers your choice.',
      'Zones. Zones use the max heart rate and threshold power from the FIT file when it has them; the sharer can set their own on the edit page.',
      'Share a range. Select a range, then click "Copy link to this selection" to share it.',
      'Pins. On workouts with pins, click a pin on the map or in the list to show it on the route and charts; the buttons on the map show or hide each type.',
      'Add a lap or pin. For sharers: on the edit page, select a point or range, then right-click the map or use "+", to add a lap boundary or pin.',
      'Summary numbers. The "How these numbers are computed" bar, under the "How to use this page" bar, explains the summary\'s numbers.',
    ]);
  });

  it('closes with its button and a click on the backdrop, not inside; one dialog, reused', () => {
    openHowto(document);
    dialog().querySelector<HTMLElement>('.dlg-body p')!.click();
    expect(dialog().open).toBe(true);
    dialog().click(); // the dialog has no padding: its own box is the backdrop
    expect(dialog().open).toBe(false);
    openHowto(document);
    expect(dialog().open).toBe(true);
    dialog().querySelector('button')!.click();
    expect(dialog().open).toBe(false);
    expect(document.querySelectorAll('dialog').length).toBe(1);
  });
});

describe('close ×', () => {
  it('has a fixed header with title and ×; the body opens at the top; × closes; no bottom button', () => {
    openHowto(document);
    const d = dialog();
    const head = d.querySelector('.dlg-head')!;
    const body = d.querySelector<HTMLElement>('.dlg-body')!;
    expect(d.firstElementChild).toBe(head);
    expect(head.nextElementSibling).toBe(body);
    expect(head.firstElementChild!.tagName).toBe('H2');
    expect(head.querySelector('h2')!.textContent).toBe('How to use this page');
    const x = head.querySelector<HTMLButtonElement>('button.dlg-x')!;
    expect(x.getAttribute('aria-label')).toBe('Close');
    expect(body.querySelector('h2, button, .dlg-actions')).toBeNull();
    expect(d.querySelectorAll('button')).toHaveLength(1);
    body.scrollTop = 50;
    openHowto(document);
    expect(body.scrollTop).toBe(0);
    x.click();
    expect(d.open).toBe(false);
  });
});

// jsdom lays nothing out: check the rules that keep a tall dialog scrolling inside itself, not the page.
describe('dialog scrolling (info dialogs, Report)', () => {
  it.each([
    ['../dialog/dialog.css', 'dlg'],
    ['../report/report.css', 'report'],
  ])('%s scrolls inside the dialog and locks the page', (file, cls) => {
    const css = (readFileSync(new URL(file, import.meta.url), 'utf8') as string).replace(/\/\*[\s\S]*?\*\//g, '');
    const own = css.match(new RegExp(`\\.${cls} \\{([^}]*)\\}`))![1];
    expect(own).toMatch(/max-height: calc\(100dvh - 16px\);/);
    expect(own).toMatch(/overflow-y: auto;/);
    expect(own).toMatch(/overscroll-behavior: contain;/);
    const lock = css.match(new RegExp(`html:has\\(dialog\\.${cls}\\[open\\]\\) \\{([^}]*)\\}`))![1];
    expect(lock).toMatch(/overflow: hidden;/);
    expect(lock).toMatch(/scrollbar-gutter: stable;/);
  });

  it('header is fixed, only the body scrolls, overscroll contained, fits the viewport', () => {
    const file = '../dialog/dialog.css'; // a variable: vite would turn a literal into an asset URL
    const css = (readFileSync(new URL(file, import.meta.url), 'utf8') as string).replace(/\/\*[\s\S]*?\*\//g, '');
    const rule = (sel: string): string => css.match(new RegExp(sel.replace(/[[\]().]/g, '\\$&') + ' \\{([^}]*)\\}'))![1];
    expect(rule('.dlg[open]')).toMatch(/display: flex;[\s\S]*flex-direction: column;/);
    expect(rule('.dlg-head')).toMatch(/flex: none;/);
    expect(rule('.dlg-head')).toMatch(/border-bottom: 1px solid var\(--line\);/);
    expect(rule('.dlg-body')).toMatch(/flex: 1 1 auto;/);
    expect(rule('.dlg-body')).toMatch(/min-height: 0;/);
    expect(rule('.dlg-body')).toMatch(/overflow-y: auto;/);
    expect(rule('.dlg-body')).toMatch(/overscroll-behavior: contain;/);
    expect(css).toMatch(/@media \(pointer: coarse\) \{\s*\.dlg-x \{\s*min-width: 44px;\s*height: 44px;/);
  });
});

describe('page lock while open (iOS Safari)', () => {
  let scrollTo: ReturnType<typeof vi.fn>;
  beforeEach(() => {
    HTMLDialogElement.prototype.close = function (this: HTMLDialogElement) {
      this.open = false;
      this.dispatchEvent(new Event('close'));
    };
    Object.defineProperty(window, 'scrollY', { value: 340, configurable: true });
    scrollTo = vi.fn();
    window.scrollTo = scrollTo as unknown as typeof window.scrollTo;
  });
  const locked = (): void => {
    const st = document.body.style;
    expect([st.position, st.top, st.left, st.right, st.width]).toEqual(['fixed', '-340px', '0px', '0px', '100%']);
  };
  const unlocked = (): void => {
    expect(document.body.getAttribute('style') ?? '').toBe('');
    expect(scrollTo).toHaveBeenCalledWith({ left: 0, top: 340, behavior: 'instant' });
  };
  it('pins the body at -scrollY and restores on ×', () => {
    openHowto(document);
    locked();
    dialog().querySelector<HTMLButtonElement>('.dlg-x')!.click();
    unlocked();
  });
  it('restores on Esc (close event)', () => {
    openHowto(document);
    locked();
    dialog().open = false;
    dialog().dispatchEvent(new Event('close'));
    unlocked();
  });
  it('restores on backdrop click', () => {
    openHowto(document);
    locked();
    dialog().click();
    unlocked();
  });
});
