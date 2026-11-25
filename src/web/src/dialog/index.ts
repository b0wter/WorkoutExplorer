// The info dialogs' shared shell (F-58 SE-6, from F-39 HU-7): one native modal <dialog> per kind,
// built on first open and refilled on every open (units and saved settings may have changed).
// LAZY: imported only by the lazy How to use and summary chunks. showModal adds no history entry;
// Esc and the Android back gesture close it natively.

import './dialog.css';
import { t } from './texts';

/** Opens the dialog of class `cls` with a title and the given lines, then a header (title, × top right). With an
 *  `anchor`, at 1024 px and wider (F-13 D-17), its top-right corner sits on the anchor's, inside
 *  the viewport (OQ-258); narrower, centred. `actions` replace the × as a bottom row of the scrolling body (the pin dialog). */
export function openDialog(doc: Document, cls: string, title: string, lines: readonly Node[], anchor?: Element, actions?: readonly Node[]): HTMLDialogElement {
  const d = doc.querySelector<HTMLDialogElement>('dialog.' + cls) ?? build(doc, cls);
  const h = doc.createElement('h2');
  h.id = cls + '-title';
  h.textContent = title;
  const head = doc.createElement('div');
  head.className = 'dlg-head';
  head.append(h);
  let bar: HTMLParagraphElement | null = null;
  if (actions) {
    bar = doc.createElement('p');
    bar.className = 'dlg-actions';
    bar.append(...actions);
  } else {
    // First in focus order: showModal focuses it without scrolling anything. The header never scrolls.
    const x = head.appendChild(doc.createElement('button'));
    x.type = 'button';
    x.className = 'dlg-x';
    x.textContent = '×';
    x.setAttribute('aria-label', t('closeDialog'));
    x.addEventListener('click', () => d.close());
  }
  const body = d.querySelector('.dlg-body')!;
  body.replaceChildren(...lines, ...(bar ? [bar] : []));
  d.firstElementChild!.replaceWith(head);
  d.removeAttribute('style');
  d.showModal();
  body.scrollTop = 0;
  const win = doc.defaultView;
  if (anchor && win?.matchMedia?.('(min-width: 1024px)').matches) {
    const r = anchor.getBoundingClientRect();
    const left = Math.max(8, Math.min(r.right - d.offsetWidth, doc.documentElement.clientWidth - d.offsetWidth - 8));
    const top = Math.max(8, Math.min(r.top, win.innerHeight - d.offsetHeight - 8));
    d.style.cssText = `margin:0;inset:${top}px auto auto ${left}px`;
  }
  return d;
}

function build(doc: Document, cls: string): HTMLDialogElement {
  const d = doc.createElement('dialog');
  d.className = 'dlg ' + cls;
  d.setAttribute('aria-labelledby', cls + '-title');
  const body = doc.createElement('div');
  body.className = 'dlg-body';
  d.append(doc.createElement('div'), body); // the header is put in place on every open
  // The dialog has no padding, so a click on the dialog itself is a click on the backdrop. Only when the press began
  // there too: a text selection dragged out of a field ends on the backdrop and must not close.
  let down: EventTarget | null = d;
  d.addEventListener('pointerdown', (e) => (down = e.target));
  d.addEventListener('click', (e) => {
    if (e.target === d && down === d) d.close();
    down = d;
  });
  doc.body.appendChild(d);
  return d;
}

/** One line: a bold name, then text spans as [class, text] ('' for no class); empty texts are left out. */
export function line(doc: Document, name: string, ...parts: readonly (readonly [string, string])[]): HTMLParagraphElement {
  const p = doc.createElement('p');
  const b = p.appendChild(doc.createElement('b'));
  b.textContent = name + '. ';
  for (const [cls, text] of parts) {
    if (!text) continue;
    const s = p.appendChild(doc.createElement('span'));
    if (cls) s.className = cls;
    s.textContent = text;
  }
  return p;
}
