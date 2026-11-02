// DOM glue of the upload page (F-11 design U-5 to U-10). Everything the browser is asked to do
// comes in through `Env`, so jsdom tests can replace it. No cookies, no browser storage (U-10).
import { canPick, outcomeOf, parseStatus, percent, SENTENCES, tooLarge, type PageState } from '../lib/upload';

export interface Env {
  fetch(input: string, init?: RequestInit): Promise<Response>;
  xhr(): XMLHttpRequest;
  navigate(url: string): void;
  origin: string;
  /** Longest wait for `/api/status` before the page fails open (U-5). */
  statusWaitMs: number;
}

function el<T extends HTMLElement>(doc: Document, id: string): T {
  const e = doc.getElementById(id);
  if (!e) throw new Error(`missing #${id}`);
  return e as T;
}

/** Starts the page; the returned function removes the document and window listeners (tests). */
export function start(doc: Document, win: Window, env: Env): () => void {
  const page = el(doc, 'page');
  const notice = el(doc, 'notice');
  const drop = el(doc, 'drop');
  const input = el<HTMLInputElement>(doc, 'file');
  const progress = el(doc, 'progress');
  const message = el(doc, 'message');

  let state: PageState = 'checking';
  let generation = 0;

  function show(next: PageState, opts: { notice?: string; progress?: string; message?: string } = {}): void {
    state = next;
    page.dataset.state = next;
    input.disabled = !canPick(next);
    notice.textContent = opts.notice ?? '';
    progress.textContent = opts.progress ?? '';
    message.textContent = opts.message ?? '';
  }

  async function checkStatus(): Promise<void> {
    const gen = ++generation;
    show('checking');
    let open = true; // fails open: a broken status never blocks uploads (U-5)
    const ctl = new AbortController();
    const timer = win.setTimeout(() => ctl.abort(), env.statusWaitMs);
    try {
      const res = await env.fetch('/api/status', { cache: 'no-store', signal: ctl.signal });
      if (res.ok) open = parseStatus(await res.text()) !== false;
    } catch {
      // network error, timeout: stay open
    } finally {
      win.clearTimeout(timer);
    }
    if (gen !== generation) return;
    if (open) show('ready');
    else show('paused', { notice: SENTENCES.paused });
  }

  function pick(file: File | undefined): void {
    if (!file || !canPick(state)) return; // a second pick or drop while busy starts nothing
    if (tooLarge(file.size)) {
      show('ready', { message: SENTENCES.tooLarge });
      return;
    }
    send(file);
  }

  function send(file: File): void {
    const gen = generation;
    const xhr = env.xhr();
    show('uploading', { progress: 'Uploading… 0 %' });
    const fail = (text: string): void => {
      if (gen === generation) show('ready', { message: text });
    };
    xhr.open('POST', '/api/workouts');
    xhr.upload.onprogress = (e) => {
      if (gen === generation && state === 'uploading' && e.lengthComputable) {
        progress.textContent = `Uploading… ${percent(e.loaded, e.total)} %`;
      }
    };
    xhr.upload.onload = () => {
      if (gen === generation) show('processing', { progress: 'Reading the workout…' });
    };
    xhr.onload = () => {
      if (gen !== generation) return;
      const out = outcomeOf(xhr.status, xhr.responseText, env.origin);
      if (out.kind === 'navigate') env.navigate(out.url);
      else if (out.kind === 'paused') show('paused', { notice: out.message });
      else fail(out.message);
    };
    xhr.onerror = xhr.onabort = () => fail(SENTENCES.unreadable);
    const form = new FormData();
    form.append('file', file);
    xhr.send(form);
  }

  const onChange = () => {
    const file = input.files?.[0];
    input.value = ''; // the same file can be picked again after a refusal (U-6)
    pick(file);
  };
  input.addEventListener('change', onChange);

  // A file dropped anywhere never makes the browser open it (U-6); only the first file counts.
  const onDragOver = (e: Event) => {
    e.preventDefault();
    drop.classList.add('over');
  };
  const onDragLeave = () => drop.classList.remove('over');
  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    drop.classList.remove('over');
    pick(e.dataTransfer?.files?.[0]);
  };
  doc.addEventListener('dragover', onDragOver);
  doc.addEventListener('dragleave', onDragLeave);
  doc.addEventListener('drop', onDrop);

  // Back-forward cache: start again, status included (U-5).
  const onPageShow = (e: Event) => {
    if ((e as PageTransitionEvent).persisted) void checkStatus();
  };
  win.addEventListener('pageshow', onPageShow);

  void checkStatus();
  return () => {
    input.removeEventListener('change', onChange);
    doc.removeEventListener('dragover', onDragOver);
    doc.removeEventListener('dragleave', onDragLeave);
    doc.removeEventListener('drop', onDrop);
    win.removeEventListener('pageshow', onPageShow);
  };
}
