// The range panel's mount (F-17 R-9, F-40 SR-7), a lazy chunk since ADR-011 decision 7 T-1: started
// from the charts' onready, so a range set before it lands (range link, lap tap) shows when it does.
import { mount } from 'svelte';
import RangePanel from '../components/range/RangePanel.svelte';
import { rangeFragment } from '../lib/rangelink';
import type { SelectionStore } from '../lib/selection';
import type { WorkoutData } from '../lib/datafile';
import { copier, copyField, type ClipboardNav } from './enhance';
import type { Look } from './map-props.svelte';

/** The range panel (F-17 R-9): mounted after the charts' first paint into #range-slot. The slot
 *  is hidden while there is no range; only the two transitions touch it, never a frame of a drag. */
export function mountRangePanel(root: ParentNode, data: WorkoutData, selection: SelectionStore, look: Look, viewUrl: string | null, nav: ClipboardNav): void {
  const slot = root.querySelector<HTMLElement>('#range-slot');
  if (!slot) return;
  let shown: boolean | null = null;
  selection.subscribe((a) => {
    const on = a >= 0;
    if (on === shown) return;
    shown = on;
    slot.hidden = !on;
  });
  mount(RangePanel, {
    target: slot,
    props: {
      data,
      selection,
      get units() {
        return look.units;
      },
      get view() {
        return look.view;
      },
      oncopy: viewUrl ? rangeCopy(viewUrl, data, selection, nav) : undefined,
    },
  });
}

/** "Copy link to this selection" (F-40 SR-7): the viewing link plus the current range's fragment,
 *  through Copy link's routine. The fallback field never shows another range than the current one. */
function rangeCopy(viewUrl: string, data: WorkoutData, selection: SelectionStore, nav: ClipboardNav): (button: HTMLButtonElement) => void {
  const url = (): string => viewUrl + '#' + rangeFragment(data.time, selection.start(), selection.end());
  let field: HTMLInputElement | null = null;
  selection.subscribe(() => {
    field?.remove();
    field = null;
  });
  let copy: ((url: string) => void) | null = null;
  return (button) => {
    copy ??= copier(button, nav, (u) => {
      if (selection.start() >= 0 && u === url() && button.parentElement) field = copyField(button.parentElement, u);
    });
    copy(url());
  };
}

