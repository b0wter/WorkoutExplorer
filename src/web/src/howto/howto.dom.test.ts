// @vitest-environment jsdom
// The How to use dialog (F-39 HU-7; AC 3, AC 4). jsdom has no showModal: a minimal stand-in.
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
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
const texts = (cls: string): string[] => [...dialog().querySelectorAll('.howto-body > p:not(.howto-actions)')].map((p) => p.querySelector('b')!.textContent + p.querySelector('.' + cls)!.textContent);

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
      'Units and theme. Change units and theme at the top of the page; this device remembers your choice.',
    ]);
    expect(texts('fine')).toEqual([
      'Values at a point. Point at a chart to see the values at that point; on workouts with GPS, you can also point along the route on the map.',
      'Select a range. Drag across a chart to select a range and see its totals; Clear removes it.',
      'Laps and splits. Click a lap or split to select it as a range.',
      'Fullscreen map. On workouts with GPS, click the fullscreen button on the map to enlarge it; click it again or press Esc to leave.',
      'Units and theme. Change units and theme at the top of the page; this device remembers your choice.',
    ]);
  });

  it('closes with its button and a click on the backdrop, not inside; one dialog, reused', () => {
    openHowto(document);
    dialog().querySelector<HTMLElement>('.howto-body p')!.click();
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
