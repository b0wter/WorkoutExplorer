// The How to use dialog (F-39 HU-7; AP-22). LAZY: reached only through enhanceHowto's import().
// Plain DOM, one native modal <dialog>, built on first open and reused. Both wordings are in the
// markup; CSS picks one by the primary pointer, as the range hint does. showModal adds no history
// entry; Esc and the Android back gesture close it natively.

import './howto.css';

/** [section, touch, mouse]; the same lines for every workout (OQ-126). */
const LINES: [string, string, string][] = [
  [
    'Values at a point',
    'Touch a chart and slide sideways to see the values at that point; on workouts with GPS, you can also drag along the route on the map.',
    'Point at a chart to see the values at that point; on workouts with GPS, you can also point along the route on the map.',
  ],
  [
    'Select a range',
    'Touch and hold a chart, then drag, to select a range and see its totals; Clear removes it.',
    'Drag across a chart to select a range and see its totals; Clear removes it.',
  ],
  ['Laps and splits', 'Tap a lap or split to select it as a range.', 'Click a lap or split to select it as a range.'],
  [
    'Fullscreen map',
    'On workouts with GPS, tap the fullscreen button on the map to enlarge it; tap it again or go back to leave.',
    'On workouts with GPS, click the fullscreen button on the map to enlarge it; click it again or press Esc to leave.',
  ],
  [
    'Units and theme',
    'Change units and theme at the top of the page; this device remembers your choice.',
    'Change units and theme at the top of the page; this device remembers your choice.',
  ],
  [
    'Share a range',
    'Select a range, then tap "Copy link to this selection" to share it.',
    'Select a range, then click "Copy link to this selection" to share it.',
  ], // F-40 SR-8
];

export function openHowto(doc: Document): void {
  (doc.querySelector<HTMLDialogElement>('dialog.howto-dlg') ?? build(doc)).showModal();
}

function build(doc: Document): HTMLDialogElement {
  const d = doc.createElement('dialog');
  d.className = 'howto-dlg';
  d.setAttribute('aria-labelledby', 'howto-title');
  const body = doc.createElement('div');
  body.className = 'howto-body';
  const title = doc.createElement('h2');
  title.id = 'howto-title';
  title.textContent = 'How to use this page';
  // One line per section: its name, then the touch and the mouse wording.
  const lines = LINES.map(([name, touch, mouse]) => {
    const p = doc.createElement('p');
    for (const [tag, cls, text] of [['b', '', name + '. '], ['span', 'coarse', touch], ['span', 'fine', mouse]]) {
      const el = doc.createElement(tag);
      el.className = cls;
      el.textContent = text;
      p.appendChild(el);
    }
    return p;
  });
  const close = doc.createElement('button');
  close.type = 'button';
  close.textContent = 'Close';
  close.addEventListener('click', () => d.close());
  const actions = doc.createElement('p');
  actions.className = 'howto-actions';
  actions.appendChild(close);
  body.append(title, ...lines, actions);
  d.appendChild(body);
  // The dialog has no padding, so a click on the dialog itself is a click on the backdrop.
  d.addEventListener('click', (e) => {
    if (e.target === d) d.close();
  });
  doc.body.appendChild(d);
  return d;
}
