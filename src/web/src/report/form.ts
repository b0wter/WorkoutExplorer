// The report form (F-28 RF-3 to RF-8; INF-2). LAZY: reached only through enhanceReport's import().
// Plain DOM, no Svelte (RF-3): one native <dialog>, built on first open and reused. Nothing about a
// report outlives the page: no storage, no console output of the message (RF-6).

import './report.css';
import { cleanText } from '../lib/text';

const MAX = 1000;
const FALLBACK = 'The report could not be sent. Please try again later.';
/** Statuses whose JSON `message` is shown as is (RF-7). */
const SPOKEN = [400, 404, 429, 500];

const HTML = `<form class="report-form">
<h2>Report this workout</h2>
<fieldset><legend>Reason</legend>
<label><input type="radio" name="reason" value="spam" required> Spam</label>
<label><input type="radio" name="reason" value="illegal_content"> Illegal content</label>
<label><input type="radio" name="reason" value="personal_data"> Someone else's personal data</label>
<label><input type="radio" name="reason" value="other"> Other</label>
</fieldset>
<label for="report-msg">Message (optional)</label>
<textarea id="report-msg" name="message" maxlength="${MAX}" rows="4" aria-describedby="report-count report-note"></textarea>
<p class="report-count" id="report-count">0 / ${MAX}</p>
<p class="report-note" id="report-note">Please do not include contact details. Reports get no reply.</p>
<p class="report-error" role="alert"></p>
<p class="report-actions"><button type="submit">Send report</button> <button type="button">Cancel</button></p>
</form>
<div class="report-done" role="status" hidden><p>Thank you. The report has been sent.</p><p class="report-actions"><button type="button">Close</button></p></div>`;

/** Opens the form for the workout `publicId`; the entered text survives a close (RF-8). */
export function openReport(doc: Document, publicId: string): void {
  (doc.querySelector<HTMLDialogElement>('dialog.report') ?? build(doc, publicId)).showModal();
}

function build(doc: Document, publicId: string): HTMLDialogElement {
  const d = doc.createElement('dialog');
  d.className = 'report';
  d.setAttribute('aria-label', 'Report this workout');
  d.innerHTML = HTML;
  const form = d.querySelector('form')!;
  const done = d.querySelector<HTMLElement>('.report-done')!;
  const message = form.querySelector('textarea')!;
  const count = form.querySelector('.report-count')!;
  const error = form.querySelector('.report-error')!;
  const [send, cancel] = form.querySelectorAll('button');
  const counter = (): void => {
    count.textContent = `${message.value.length} / ${MAX}`; // UTF-16 units, as maxlength (RF-4)
  };
  message.addEventListener('input', counter);
  cancel.addEventListener('click', () => d.close());
  done.querySelector('button')!.addEventListener('click', () => d.close());
  // After a confirmation, the next open starts with an empty form (RF-7).
  d.addEventListener('close', () => {
    if (done.hidden) return;
    done.hidden = true;
    form.hidden = false;
  });
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    const reason = (form.elements.namedItem('reason') as RadioNodeList).value;
    if (!reason) return; // `required` already stops the submit
    send.disabled = true;
    send.textContent = 'Sending…';
    error.textContent = '';
    void post(publicId, reason, cleanText(message.value, true)).then((text) => {
      send.disabled = false;
      send.textContent = 'Send report';
      if (text !== null) {
        error.textContent = text; // the reason and the text stay (AC 5)
        return;
      }
      form.reset();
      counter();
      form.hidden = true;
      done.hidden = false;
    });
  });
  doc.body.appendChild(d);
  return d;
}

/** Sends one report; null on success, else the sentence to show (RF-5, RF-7). Never rejects. */
async function post(publicId: string, reason: string, message: string): Promise<string | null> {
  try {
    const res = await fetch('/api/reports', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ public_id: publicId, reason, message }),
    });
    if (res.status === 204) return null;
    if (SPOKEN.includes(res.status)) {
      const body: unknown = await res.json();
      const text = (body as { message?: unknown } | null)?.message;
      if (typeof text === 'string') return text;
    }
  } catch {
    // network failure or a body that is not JSON: the fallback
  }
  return FALLBACK;
}
