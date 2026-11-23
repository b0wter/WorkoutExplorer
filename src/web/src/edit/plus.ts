// The "+" control of the edit page (F-65 AL-9; F-56 PN-12): a disclosure, not an ARIA menu. A
// button with aria-expanded, followed in the DOM by its item buttons, so Tab reaches them; a
// disabled item is aria-disabled with its one-line reason linked by aria-describedby. The list is
// an overlay (CSS), so opening never changes the layout. Item states are computed on open and,
// while open, on every refresh; nothing runs per cursor flush while closed.

import { t } from './texts';

export interface PlusItem {
  label(): string;
  /** The one-column button text. */
  short(): string;
  /** hidden: not offered; off: disabled with `why` (may be ''). */
  state(): { readonly hidden?: boolean; readonly off: boolean; readonly why: string };
  run(): void;
}

export interface Plus {
  readonly group: HTMLElement;
  readonly button: HTMLButtonElement;
  /** `usable`: a cursor or range is set ("+" is disabled without); open item states follow. */
  refresh(usable: boolean): void;
  label(): void;
  /** One column: no "+", the items sit open in a row. Two columns: the "+" disclosure. */
  setFlat(flat: boolean): void;
}

let seq = 0;
// Items are always open and "+" is hidden in flat mode (the one-column layout).

export function createPlus(doc: Document, items: readonly PlusItem[]): Plus {
  const make = <K extends keyof HTMLElementTagNameMap>(tag: K, cls: string): HTMLElementTagNameMap[K] => {
    const e = doc.createElement(tag);
    e.className = cls;
    return e;
  };
  let flat = false;
  const group = make('div', 'wx-plus');
  const button = group.appendChild(make('button', 'edit-button plus-button'));
  button.type = 'button';
  button.textContent = '+';
  button.setAttribute('aria-expanded', 'false');
  const menu = group.appendChild(make('div', 'plus-menu'));
  menu.hidden = true;
  const rows = items.map((item) => {
    const row = menu.appendChild(make('div', 'plus-row'));
    const b = row.appendChild(make('button', 'edit-button plus-item'));
    b.type = 'button';
    const why = row.appendChild(make('span', 'edit-small'));
    why.id = 'wx-plus-why-' + ++seq;
    b.addEventListener('click', () => {
      if (item.state().off) return paint(); // the state now, not the one painted on open
      close(false);
      item.run();
      if (!flat) button.focus(); // the list hid the focused item: back to "+", as Esc does
    });
    return { item, row, b, why };
  });
  const paint = (): void => {
    for (const { item, row, b, why } of rows) {
      const s = item.state();
      row.hidden = !!s.hidden;
      b.setAttribute('aria-disabled', String(s.off));
      b.disabled = flat && button.disabled; // flat: no cursor or range, as the disabled "+"
      why.textContent = s.off ? s.why : '';
      why.title = why.textContent; // flat: one ellipsized line, the full text here
      if (why.textContent) b.setAttribute('aria-describedby', why.id);
      else b.removeAttribute('aria-describedby');
    }
  };
  const outside = (e: Event): void => {
    if (!group.contains(e.target as Node)) close(false);
  };
  const open = (): void => {
    paint();
    menu.hidden = false;
    button.setAttribute('aria-expanded', 'true');
    doc.addEventListener('pointerdown', outside, true);
  };
  function close(focus: boolean): void {
    if (flat || menu.hidden) return;
    menu.hidden = true;
    button.setAttribute('aria-expanded', 'false');
    doc.removeEventListener('pointerdown', outside, true);
    if (focus) button.focus();
  }
  button.addEventListener('click', () => (menu.hidden ? !button.disabled && open() : close(false)));
  group.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && !flat && !menu.hidden) {
      e.stopPropagation(); // not the fullscreen map's Esc
      close(true);
    }
  });
  group.addEventListener('focusout', (e) => {
    const to = (e as FocusEvent).relatedTarget as Node | null;
    if (to && !group.contains(to)) close(false);
  });
  const label = (): void => {
    button.setAttribute('aria-label', t('plus'));
    button.title = t('plus');
    for (const { item, b } of rows) {
      // flat: the short "+ Lap" with the full name for assistive tech
      b.textContent = flat ? item.short() : item.label();
      if (flat) b.setAttribute('aria-label', item.label());
      else b.removeAttribute('aria-label');
    }
    if (!menu.hidden) paint();
  };
  label();
  return {
    group,
    button,
    refresh: (usable) => {
      button.disabled = !usable;
      if (!usable) close(false);
      if (!menu.hidden) paint();
    },
    label,
    setFlat: (f) => {
      doc.removeEventListener('pointerdown', outside, true);
      flat = f;
      group.classList.toggle('flat', f);
      button.hidden = f;
      button.setAttribute('aria-expanded', 'false');
      menu.hidden = !f;
      label();
      if (f) paint();
    },
  };
}
