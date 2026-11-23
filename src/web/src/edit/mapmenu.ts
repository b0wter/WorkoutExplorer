// Right-click menu on the map of the edit page (F-65 AL-9, F-56 PN-12): the "+" items (Add lap,
// Add pin) on the existing cursor or range, exactly as the "+" buttons act. Desktop only: a touch
// long-press leaves the browser's own menu. Owner chunk; the item states and rules are the "+"
// menu's (PlusItem), nothing is duplicated here.

import type { CursorStore } from '../lib/cursor';
import type { SelectionStore } from '../lib/selection';
import type { PlusItem } from './plus';

export interface MapMenuEnv {
  /** The element holding the map (listener and positioning parent). */
  readonly host: HTMLElement;
  readonly cursor: CursorStore;
  readonly selection: SelectionStore;
  readonly items: readonly PlusItem[];
}

export function startMapMenu(env: MapMenuEnv): void {
  const { host, cursor, selection, items } = env;
  const doc = host.ownerDocument;
  let pointer = '';
  let menu: HTMLElement | null = null;
  let from: HTMLElement | null = null;

  const close = (focus: boolean): void => {
    if (!menu) return;
    menu.remove();
    menu = null;
    for (const t of ['pointerdown', 'wheel'] as const) doc.removeEventListener(t, away, true);
    if (focus) from?.focus?.();
  };
  const away = (e: Event): void => {
    if (menu && !menu.contains(e.target as Node)) close(false);
  };

  host.addEventListener('pointerdown', (e) => (pointer = e.pointerType), true);
  host.addEventListener('contextmenu', (e) => {
    const target = e.target as HTMLElement;
    if (pointer !== 'mouse' || target.tagName !== 'CANVAS') return; // not on a marker or control
    e.preventDefault();
    close(false);
    from = target;
    const usable = selection.start() >= 0 || (cursor.get() ?? -1) >= 0; // as "+" is disabled without

    const m = doc.createElement('div');
    m.className = 'map-menu';
    m.setAttribute('role', 'menu');
    const buttons: HTMLButtonElement[] = [];
    for (const item of items) {
      const s = item.state();
      if (s.hidden) continue;
      const b = doc.createElement('button');
      b.type = 'button';
      b.className = 'edit-button plus-item';
      b.setAttribute('role', 'menuitem');
      b.setAttribute('aria-disabled', String(s.off || !usable));
      b.textContent = item.label();
      m.append(b);
      if (s.off && s.why) {
        const why = doc.createElement('span');
        why.className = 'edit-small';
        why.id = 'wx-map-menu-why-' + buttons.length;
        why.textContent = s.why;
        b.setAttribute('aria-describedby', why.id);
        m.append(why);
      }
      b.addEventListener('click', () => {
        if (!usable || item.state().off) return;
        close(true);
        item.run();
      });
      buttons.push(b);
    }
    if (!buttons.length) return;
    m.addEventListener('keydown', (k) => {
      const at = buttons.indexOf(doc.activeElement as HTMLButtonElement);
      if (k.key === 'Escape') {
        k.stopPropagation(); // not the fullscreen map's Esc
        close(true);
      } else if (k.key === 'ArrowDown' || k.key === 'ArrowUp') {
        k.preventDefault();
        buttons[(at + (k.key === 'ArrowDown' ? 1 : buttons.length - 1)) % buttons.length].focus();
      }
    });
    const r = host.getBoundingClientRect();
    host.append(m);
    // kept inside the map box
    m.style.left = Math.max(0, Math.min(e.clientX - r.left, r.width - m.offsetWidth)) + 'px';
    m.style.top = Math.max(0, Math.min(e.clientY - r.top, r.height - m.offsetHeight)) + 'px';
    menu = m;
    doc.addEventListener('pointerdown', away, true);
    doc.addEventListener('wheel', away, true);
    buttons[0].focus();
  });
}
