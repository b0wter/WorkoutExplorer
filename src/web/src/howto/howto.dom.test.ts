// @vitest-environment jsdom
// The How to use dialog (F-39 HU-7; AC 3, AC 4). jsdom has no showModal: a minimal stand-in.
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
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
});
